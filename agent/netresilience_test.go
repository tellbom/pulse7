package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

func streamTestClient(t *testing.T, body string) (*openai.Client, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, body)
	}))
	cfg := openai.DefaultConfig("test-key")
	cfg.BaseURL = srv.URL
	return openai.NewClientWithConfig(cfg), srv.Close
}

func runStreamFixture(t *testing.T, body string) (*streamSink, error) {
	t.Helper()
	client, closeServer := streamTestClient(t, body)
	defer closeServer()
	cfg := &config{
		model: "test-model", llmFirstChunkTimeout: time.Second,
		llmIdleTimeout: time.Second,
	}
	sink := newStreamSink()
	err := llmStreamOnce(context.Background(), client, cfg,
		openai.ChatCompletionRequest{Model: cfg.model, Stream: true}, sink)
	return sink, err
}

func TestStreamRejectsTextTruncatedByUnexpectedEOF(t *testing.T) {
	sink, err := runStreamFixture(t,
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\"},\"finish_reason\":null}]}\n\n")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "unexpected eof") {
		t.Fatalf("stream error = %v, want unexpected EOF", err)
	}
	if sink.content.String() != "partial" {
		t.Fatalf("diagnostic sink content = %q", sink.content.String())
	}
	if !retryableLLMError(err) {
		t.Fatalf("unexpected EOF must use the existing retry path: %v", err)
	}
}

func TestStreamRejectsTruncatedToolArguments(t *testing.T) {
	body := "data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call-1\",\"type\":\"function\",\"function\":{\"name\":\"read\",\"arguments\":\"{\\\"path\\\":\"}}]},\"finish_reason\":null}]}\n\n"
	sink, err := runStreamFixture(t, body)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "unexpected eof") {
		t.Fatalf("stream error = %v, want unexpected EOF", err)
	}
	if len(sink.calls()) != 1 || sink.calls()[0].Function.Arguments != `{"path":` {
		t.Fatalf("tool fragment was not reproduced: %+v", sink.calls())
	}
}

func TestStreamRejectsCompleteToolArgumentsWithoutTerminalMarker(t *testing.T) {
	body := "data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call-1\",\"type\":\"function\",\"function\":{\"name\":\"read\",\"arguments\":\"{\\\"path\\\":\\\"a.txt\\\"}\"}}]},\"finish_reason\":null}]}\n\n"
	sink, err := runStreamFixture(t, body)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "unexpected eof") {
		t.Fatalf("stream error = %v, want unexpected EOF", err)
	}
	if len(sink.calls()) != 1 || sink.calls()[0].Function.Arguments != `{"path":"a.txt"}` {
		t.Fatalf("complete unterminated tool call was not reproduced: %+v", sink.calls())
	}
}

func TestStreamAcceptsExplicitStopAndToolCallsTermination(t *testing.T) {
	t.Run("stop", func(t *testing.T) {
		body := "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"complete\"},\"finish_reason\":null}]}\n\n" +
			"data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
			"data: [DONE]\n\n"
		sink, err := runStreamFixture(t, body)
		if err != nil || sink.content.String() != "complete" || len(sink.calls()) != 0 {
			t.Fatalf("normal stop: content=%q calls=%d error=%v", sink.content.String(), len(sink.calls()), err)
		}
	})

	t.Run("tool_calls", func(t *testing.T) {
		body := "data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call-1\",\"type\":\"function\",\"function\":{\"name\":\"read\",\"arguments\":\"{\\\"path\\\":\\\"a.txt\\\"}\"}}]},\"finish_reason\":null}]}\n\n" +
			"data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n" +
			"data: [DONE]\n\n"
		sink, err := runStreamFixture(t, body)
		calls := sink.calls()
		if err != nil || len(calls) != 1 || calls[0].Function.Name != "read" || calls[0].Function.Arguments != `{"path":"a.txt"}` {
			t.Fatalf("normal tool_calls: calls=%+v error=%v", calls, err)
		}
	})
}

func TestStreamReportsLengthTerminationAsTruncationWithoutRetry(t *testing.T) {
	body := "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"cut at limit\"},\"finish_reason\":\"length\"}]}\n\n" +
		"data: [DONE]\n\n"
	_, err := runStreamFixture(t, body)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "length") {
		t.Fatalf("length termination error = %v", err)
	}
	if retryableLLMError(err) {
		t.Fatalf("length termination must not be retried: %v", err)
	}
}

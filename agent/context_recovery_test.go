package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	openai "github.com/sashabaranov/go-openai"
)

func TestCompressionThresholdIsSixtyFivePercent(t *testing.T) {
	if compressThreshold != 0.65 {
		t.Fatalf("compress threshold = %.2f, want 0.65", compressThreshold)
	}
}

func TestShortHistoryOverThresholdFallsBackToTruncation(t *testing.T) {
	oldCfg := curCfg
	curCfg = nil
	t.Cleanup(func() { curCfg = oldCfg })

	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "system"},
		{Role: openai.ChatMessageRoleUser, Content: strings.Repeat("old", 500)},
		{Role: openai.ChatMessageRoleAssistant, Content: strings.Repeat("answer", 200)},
		{Role: openai.ChatMessageRoleUser, Content: "CURRENT_TASK"},
	}
	before := contextChars(msgs)
	if err := maybeCompressContext(context.Background(), nil, &config{maxCtx: 1_000}, nil, &msgs); err != nil {
		t.Fatal(err)
	}
	if after := contextChars(msgs); after >= before || after > int(1_000*compressThreshold) {
		t.Fatalf("short-history fallback chars: before=%d after=%d", before, after)
	}
	if len(msgs) != 2 || msgs[0].Role != openai.ChatMessageRoleSystem || msgs[1].Content != "CURRENT_TASK" {
		t.Fatalf("protected messages after fallback = %#v", msgs)
	}
}

func TestAgentMarkdownTruncatesAtRuneBoundary(t *testing.T) {
	workspace := t.TempDir()
	body := strings.Repeat("a", (8<<10)-1) + "界" + "tail"
	if err := os.WriteFile(filepath.Join(workspace, "AGENT.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := loadAgentMd(workspace)
	if !utf8.ValidString(got) {
		t.Fatal("truncated AGENT.md is not valid UTF-8")
	}
	if strings.ContainsRune(got, utf8.RuneError) || strings.Contains(got, "界") {
		t.Fatalf("AGENT.md truncation split or retained partial boundary rune: %q", got[len(got)-80:])
	}
}

func TestContextLengthErrorEquivalentForms(t *testing.T) {
	cases := []error{
		&openai.APIError{Code: "context_length_exceeded", Message: "too long", HTTPStatusCode: 400},
		&openai.APIError{Type: "invalid_request_error", Message: "This model's maximum context length is 8192 tokens", HTTPStatusCode: 400},
		&openai.RequestError{HTTPStatusCode: 400, Body: []byte(`{"error":"context window exceeded"}`)},
	}
	for _, err := range cases {
		if !contextLengthExceededError(err) {
			t.Fatalf("context error not recognized: %T %v", err, err)
		}
	}
	if contextLengthExceededError(&openai.APIError{Code: "invalid_api_key", Message: "bad key", HTTPStatusCode: 401}) {
		t.Fatal("authentication error was misclassified as context overflow")
	}
}

func overflowHistory() []openai.ChatCompletionMessage {
	msgs := []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: "system"}, {Role: openai.ChatMessageRoleUser, Content: "old task"}}
	for i := 1; i <= 4; i++ {
		msgs = append(msgs, protocolRound(fmt.Sprintf("call-%d", i), `{"path":"a.txt"}`, strings.Repeat("result", 50))...)
	}
	return append(msgs, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "current request"})
}

func runOverflowTurn(t *testing.T, client *openai.Client) (string, error) {
	t.Helper()
	cfg := &config{
		model: "test", maxCtx: 1_000_000, maxRounds: 1,
		llmCompressTimeout: time.Second, llmFirstChunkTimeout: time.Second, llmIdleTimeout: time.Second,
	}
	msgs := overflowHistory()
	oldSession, oldCfg := sess, curCfg
	sess = newSession(filepath.Join(t.TempDir(), "session.jsonl"), t.TempDir())
	curCfg = nil
	// Recovery indexes read the persisted history, as production pushMsg does.
	for _, m := range msgs {
		if err := sess.record(m); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		if err := sess.Close(); err != nil {
			t.Errorf("close test session: %v", err)
		}
		sess = oldSession
		curCfg = oldCfg
		resetInterrupt()
	})
	return streamTurn(client, &Registry{tools: map[string]Tool{}}, cfg, &msgs, &turnStats{})
}

func TestContextOverflowCompressesOnceAndRetriesCurrentRequest(t *testing.T) {
	streamRequests := 0
	compressionRequests := 0
	var temperatures []float32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var body openai.ChatCompletionRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		temperatures = append(temperatures, body.Temperature)
		if len(body.Messages) == 1 && strings.Contains(body.Messages[0].Content, "结构化摘要") {
			compressionRequests++
			writeSummaryFixture(w, "older work summarized")
			return
		}

		streamRequests++
		if streamRequests == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":{"message":"maximum context length exceeded","type":"invalid_request_error","code":"context_length_exceeded"}}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"RECOVERED\"},\"finish_reason\":null}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	clientCfg := openai.DefaultConfig("test")
	clientCfg.BaseURL = srv.URL
	client := openai.NewClientWithConfig(clientCfg)
	answer, err := runOverflowTurn(t, client)
	if err != nil || answer != "RECOVERED" {
		t.Fatalf("recovered turn answer=%q error=%v", answer, err)
	}
	if streamRequests != 2 || compressionRequests != 1 {
		t.Fatalf("requests: stream=%d compression=%d, want 2 and 1", streamRequests, compressionRequests)
	}
	for i, temperature := range temperatures {
		if temperature != codingTemperature {
			t.Fatalf("request %d temperature=%v, want %v", i+1, temperature, codingTemperature)
		}
	}
}

func TestContextOverflowStopsAfterOneEmergencyRetry(t *testing.T) {
	streamRequests := 0
	compressionRequests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var body openai.ChatCompletionRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if len(body.Messages) == 1 && strings.Contains(body.Messages[0].Content, "结构化摘要") {
			compressionRequests++
			writeSummaryFixture(w, "older work summarized")
			return
		}
		streamRequests++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":{"message":"maximum context length exceeded","type":"invalid_request_error","code":"context_length_exceeded"}}`)
	}))
	defer srv.Close()

	clientCfg := openai.DefaultConfig("test")
	clientCfg.BaseURL = srv.URL
	_, err := runOverflowTurn(t, openai.NewClientWithConfig(clientCfg))
	if err == nil || !strings.Contains(err.Error(), "单次重试仍然上下文超限") {
		t.Fatalf("overflow retry error = %v", err)
	}
	if streamRequests != 2 || compressionRequests != 1 {
		t.Fatalf("requests: stream=%d compression=%d, want exactly 2 and 1", streamRequests, compressionRequests)
	}
}

func TestEmergencyCompressionFailureIsExplicit(t *testing.T) {
	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: strings.Repeat("system", 100)},
		{Role: openai.ChatMessageRoleUser, Content: strings.Repeat("current", 100)},
	}
	err := emergencyCompressContext(context.Background(), nil, &config{maxCtx: 1_000_000}, nil, &msgs)
	if err == nil || !strings.Contains(err.Error(), "未能缩小请求") {
		t.Fatalf("emergency compression error = %v", err)
	}
}

func TestEmergencyCompressionAuditIsMarked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeSummaryFixture(w, "older work summarized")
	}))
	defer srv.Close()
	clientCfg := openai.DefaultConfig("test")
	clientCfg.BaseURL = srv.URL

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data", "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &config{model: "test", maxCtx: 1_000_000, llmCompressTimeout: time.Second, exeDir: root}
	oldCfg := curCfg
	curCfg = cfg
	t.Cleanup(func() { curCfg = oldCfg })
	msgs := overflowHistory()
	if err := emergencyCompressContext(context.Background(), openai.NewClientWithConfig(clientCfg), cfg, nil, &msgs); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "data", "sessions", "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(b))), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["emergency"] != true || entry["method"] != compressionSummary {
		t.Fatalf("emergency audit entry = %#v", entry)
	}
}

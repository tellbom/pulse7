package main

import (
	"context"
	"encoding/json"
	openai "github.com/sashabaranov/go-openai"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestF1LatestToolEvidenceSurvives(t *testing.T) {
	for _, count := range []int{0, 1, 2} {
		msgs := []openai.ChatCompletionMessage{{Role: "system", Content: "system"}, {Role: "user", Content: "task"}}
		for i := 0; i < count; i++ {
			msgs = append(msgs, protocolRound(string(rune('a'+i)), strings.Repeat("a", 300), strings.Repeat("r", 15000))...)
		}
		err := maybeCompressContext(context.Background(), nil, &config{maxCtx: 12000}, nil, &msgs)
		if err != nil {
			t.Fatal(err)
		}
		if count > 0 {
			if len(msgs) != 4 || msgs[2].ToolCalls[0].ID != string(rune('a'+count-1)) {
				t.Fatalf("count=%d latest group lost: %#v", count, msgs)
			}
			if !strings.Contains(msgs[3].Content, "original_bytes=15000") || !strings.Contains(msgs[3].Content, "discarded_bytes=") {
				t.Fatalf("missing byte accounting: %s", msgs[3].Content)
			}
			assertValidToolProtocol(t, msgs)
		}
		if requestContextChars(msgs, nil) > 7800 {
			t.Fatal("over budget")
		}
	}
}
func TestF1MinimumBudgetNeverSendsRequest(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; w.WriteHeader(500) }))
	defer srv.Close()
	cc := openai.DefaultConfig("test")
	cc.BaseURL = srv.URL
	cfg := &config{maxCtx: 100, maxRounds: 1, model: "test", llmCompressTimeout: time.Second}
	msgs := append([]openai.ChatCompletionMessage{{Role: "system", Content: "system"}, {Role: "user", Content: "task"}}, protocolRound("keep-id", strings.Repeat("a", 400), strings.Repeat("r", 15000))...)
	_, err := streamTurn(openai.NewClientWithConfig(cc), &Registry{tools: map[string]Tool{}}, cfg, &msgs, &turnStats{})
	if err == nil || requests != 0 {
		t.Fatalf("err=%v requests=%d", err, requests)
	}
	if len(msgs) != 4 || msgs[2].ToolCalls[0].ID != "keep-id" || msgs[2].ToolCalls[0].Function.Name != "write" {
		t.Fatal("protected group lost")
	}
	if !strings.Contains(msgs[3].Content, "discarded_bytes=14488") {
		t.Fatal("result not reduced to 512 bytes")
	}
	if !strings.Contains(msgs[2].ToolCalls[0].Function.Arguments, "discarded_bytes=336") {
		t.Fatal("arguments not reduced to 64 bytes")
	}
	if !json.Valid([]byte(msgs[2].ToolCalls[0].Function.Arguments)) {
		t.Fatal("invalid argument summary JSON")
	}
}

func TestF1ResultReductionPrecedesArguments(t *testing.T) {
	args := strings.Repeat("a", 300)
	msgs := append([]openai.ChatCompletionMessage{{Role: "system", Content: "system"}, {Role: "user", Content: "task"}}, protocolRound("keep", args, strings.Repeat("r", 15000))...)
	truncateContext(&msgs, 1400)
	if msgs[2].ToolCalls[0].Function.Arguments != args {
		t.Fatal("arguments reduced before necessary")
	}
	if !strings.Contains(msgs[3].Content, "discarded_bytes=14488") {
		t.Fatal("result minimum not applied first")
	}
}

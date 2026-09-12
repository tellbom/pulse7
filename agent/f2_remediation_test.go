package main

import (
	"context"
	"fmt"
	openai "github.com/sashabaranov/go-openai"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestF2ConsumerDiscardsFailedAttempts(t *testing.T) {
	for _, mode := range []string{"retry-success", "final-failure", "success"} {
		t.Run(mode, func(t *testing.T) {
			protocol, human := captureEvents(t)
			requests := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.Header().Set("Content-Type", "text/event-stream")
				success := mode == "success" || (mode == "retry-success" && requests == 2)
				text := "FAILED_ATTEMPT"
				if success {
					text = "FINAL_ANSWER"
				}
				fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"%s\"}}]}\n\n", text)
				if success {
					fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
				}
			}))
			defer srv.Close()
			cc := openai.DefaultConfig("test")
			cc.BaseURL = srv.URL
			cfg := &config{llmMaxRetries: 1, llmFirstChunkTimeout: time.Second, llmIdleTimeout: time.Second}
			answer, _, err := roundStream(context.Background(), openai.NewClientWithConfig(cc), cfg, openai.ChatCompletionRequest{Model: "test", Stream: true})
			if err != nil {
				emitTurnResult("error", err)
			} else {
				emitTurnResult("success", nil)
			}
			texts := map[float64]string{}
			accepted := ""
			failed := false
			for _, event := range parseEventLines(t, protocol.String()) {
				d := event.Data.(map[string]interface{})
				id, _ := d["attempt"].(float64)
				if event.Type == "assistant_delta" {
					texts[id] += d["delta"].(string)
				}
				if event.Type == "assistant_attempt" {
					if d["status"] == "discard" {
						delete(texts, id)
					}
					if d["status"] == "complete" {
						accepted += texts[id]
						delete(texts, id)
					}
				}
				if event.Type == "turn_result" && d["status"] == "error" {
					failed = true
				}
			}
			if mode == "final-failure" {
				if err == nil || !failed || len(texts) != 0 || accepted != "" {
					t.Fatalf("failed leftovers: %s", protocol.String())
				}
			} else if err != nil || accepted != answer || accepted != "FINAL_ANSWER" {
				t.Fatalf("consumer=%q answer=%q err=%v events=%s", accepted, answer, err, protocol.String())
			}
			if mode != "success" && (!strings.Contains(human.String(), "[重试]") || !strings.Contains(human.String(), "unexpected EOF")) {
				t.Fatal("retry reason invisible")
			}
		})
	}
}

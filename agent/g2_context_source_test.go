package main

import (
	"fmt"
	openai "github.com/sashabaranov/go-openai"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestContextFailureReportsActualSource(t *testing.T) {
	for _, local := range []bool{true, false} {
		t.Run(fmt.Sprint("local=", local), func(t *testing.T) {
			requests := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(400)
				fmt.Fprint(w, `{"error":{"message":"maximum context length exceeded","code":"context_length_exceeded"}}`)
			}))
			defer srv.Close()
			cc := openai.DefaultConfig("test")
			cc.BaseURL = srv.URL
			cfg := &config{model: "test", maxCtx: 1000000, maxRounds: 1, llmCompressTimeout: time.Second, llmFirstChunkTimeout: time.Second, llmIdleTimeout: time.Second}
			if local {
				cfg.maxCtx = 100
			}
			msgs := []openai.ChatCompletionMessage{{Role: "system", Content: strings.Repeat("system", 100)}, {Role: "user", Content: strings.Repeat("current", 100)}}
			oldCfg := curCfg
			curCfg = nil
			defer func() { curCfg = oldCfg; resetInterrupt() }()
			_, err := streamTurn(openai.NewClientWithConfig(cc), &Registry{tools: map[string]Tool{}}, cfg, &msgs, nil)
			t.Logf("CLI error: %v; endpoint_requests=%d", err, requests)
			if err == nil {
				t.Fatal("expected explicit overflow error")
			}
			if local {
				if requests != 0 || strings.Contains(err.Error(), "端点报告") || !strings.Contains(err.Error(), "本地预算不足") || !strings.Contains(err.Error(), "retained_bytes=") || !strings.Contains(err.Error(), "budget_bytes=") {
					t.Fatalf("wrong local source: %v, requests=%d", err, requests)
				}
			} else if requests != 1 || !strings.Contains(err.Error(), "端点报告上下文超限") {
				t.Fatalf("wrong endpoint source: %v requests=%d", err, requests)
			}
		})
	}
}

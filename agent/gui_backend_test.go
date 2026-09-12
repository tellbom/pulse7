package main

import (
	"context"
	"encoding/json"
	"fmt"
	openai "github.com/sashabaranov/go-openai"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGUIHistoryToolOutcome(t *testing.T) {
	s := newSession(filepath.Join(t.TempDir(), "sess-history.jsonl"), t.TempDir())
	defer s.Close()
	cases := []struct {
		name, result string
		ok           bool
	}{
		{"ls", "", true}, {"write", "error: checkpoint failed", false},
		{"shell", "exitcode=1\nfailed", false},
		{"read", "[无法按文本读取 binary]", false},
		{"write", "error: hard_link_impact_unknown: multiple names", false},
	}
	for i, c := range cases {
		call := openai.ToolCall{ID: fmt.Sprint(i), Function: openai.FunctionCall{Name: c.name, Arguments: `{}`}}
		outcome := toolOutcomeFor(call, c.result)
		if outcome.OK != c.ok {
			t.Fatalf("%s: %+v", c.name, outcome)
		}
		if err := s.record(openai.ChatCompletionMessage{Role: "tool", ToolCallID: call.ID, Content: c.result}, outcome); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.record(openai.ChatCompletionMessage{Role: "tool", ToolCallID: "legacy", Content: "error: legacy"}); err != nil {
		t.Fatal(err)
	}
	rows, err := readAPISessionMessages(s.path)
	if err != nil {
		t.Fatal(err)
	}
	for i, raw := range rows {
		var record sessionMessageRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			t.Fatal(err)
		}
		if i == len(cases) {
			if record.ToolOutcome != nil {
				t.Fatal("legacy must remain unknown")
			}
			continue
		}
		if record.ToolOutcome == nil || record.ToolOutcome.OK != cases[i].ok {
			t.Fatalf("history: %s", raw)
		}
		wire, _ := json.Marshal(record.ChatCompletionMessage)
		if strings.Contains(string(wire), "toolOutcome") {
			t.Fatal("metadata leaked to model")
		}
		if i == 4 && record.ToolOutcome.ErrorCode != "hard_link_impact_unknown" {
			t.Fatal("missing capability error")
		}
	}
}

func TestGUIReasoningStreamAndPersistence(t *testing.T) {
	protocol, _ := captureEvents(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"inspect first\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"answer\"}}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()
	cc := openai.DefaultConfig("test")
	cc.BaseURL = srv.URL
	msg, err := roundStreamMessage(context.Background(), openai.NewClientWithConfig(cc), &config{llmFirstChunkTimeout: time.Second, llmIdleTimeout: time.Second}, openai.ChatCompletionRequest{Model: "test", Stream: true})
	if err != nil || msg.Content != "answer" || msg.ReasoningContent != "inspect first" {
		t.Fatalf("%+v %v", msg, err)
	}
	if !strings.Contains(protocol.String(), `"type":"assistant_reasoning_delta"`) {
		t.Fatal(protocol.String())
	}
	s := newSession(filepath.Join(t.TempDir(), "sess-reasoning.jsonl"), t.TempDir())
	defer s.Close()
	if err := s.record(msg); err != nil {
		t.Fatal(err)
	}
	rows, err := readAPISessionMessages(s.path)
	if err != nil || len(rows) != 1 || !strings.Contains(string(rows[0]), `"reasoning_content":"inspect first"`) {
		t.Fatalf("%s %v", rows, err)
	}
	sink := newStreamSink()
	sink.finishReason = openai.FinishReasonStop
	sink.onChunk(openai.ChatCompletionStreamResponse{Choices: []openai.ChatCompletionStreamChoice{{Delta: openai.ChatCompletionStreamChoiceDelta{ReasoningContent: "late"}}}})
	if _, err := sink.terminalResult(); err == nil {
		t.Fatal("reasoning after terminal must fail")
	}
}

func TestGUIContextDefault(t *testing.T) {
	if defaultAgentConfig().MaxCtx != 256000 {
		t.Fatal("default budget drift")
	}
	if requestContextChars([]openai.ChatCompletionMessage{{Role: "user", Content: "中文"}}, nil) <= len("中文") {
		t.Fatal("budget must include JSON framing")
	}
}

func TestGUITurnRetryToolHistory(t *testing.T) {
	protocol, _ := captureEvents(t)
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		requests++
		w.Header().Set("Content-Type", "text/event-stream")
		switch requests {
		case 1:
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"discard this\"}}]}\n\n")
		case 2:
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"inspect\",\"content\":\"checking tool\",\"tool_calls\":[{\"index\":0,\"id\":\"tc\",\"type\":\"function\",\"function\":{\"name\":\"missing_tool\",\"arguments\":\"{}\"}}]}}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n")
		default:
			if len(req.Messages) < 2 {
				t.Error("missing context")
			} else {
				a := req.Messages[len(req.Messages)-2]
				if a.Content != "checking tool" || a.ReasoningContent != "inspect" {
					t.Errorf("assistant lost: %+v", a)
				}
			}
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"done\"}}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
		}
	}))
	defer srv.Close()
	cc := openai.DefaultConfig("test")
	cc.BaseURL = srv.URL
	oldSession, oldCfg := sess, curCfg
	sess = newSession(filepath.Join(t.TempDir(), "sess-turn.jsonl"), t.TempDir())
	curCfg = nil
	defer func() { sess.Close(); sess = oldSession; curCfg = oldCfg; resetInterrupt() }()
	cfg := &config{model: "test", maxCtx: defaultMaxContextBytes, maxRounds: 2, llmMaxRetries: 1, llmFirstChunkTimeout: time.Second, llmIdleTimeout: time.Second}
	msgs := []openai.ChatCompletionMessage{{Role: "user", Content: "test"}}
	answer, err := streamTurn(openai.NewClientWithConfig(cc), &Registry{tools: map[string]Tool{}}, cfg, &msgs, nil)
	if err != nil || answer != "done" || requests != 3 {
		t.Fatalf("%s %v requests=%d", answer, err, requests)
	}
	rows, err := readAPISessionMessages(sess.path)
	if err != nil || len(rows) != 3 {
		t.Fatalf("%s %v", rows, err)
	}
	if strings.Contains(fmt.Sprint(rows), "discard this") {
		t.Fatal("failed reasoning persisted")
	}
	var tool sessionMessageRecord
	if err := json.Unmarshal(rows[1], &tool); err != nil {
		t.Fatal(err)
	}
	if tool.ToolOutcome == nil || tool.ToolOutcome.OK {
		t.Fatalf("failed tool history: %s", rows[1])
	}
	if !strings.Contains(protocol.String(), `"status":"discard"`) {
		t.Fatal("missing discard")
	}
}

func TestGUIRepeatedWorkspaceSelection(t *testing.T) {
	a := testAPI(t)
	t.Setenv("USERPROFILE", t.TempDir())
	roots := []string{t.TempDir(), t.TempDir()}
	for i, root := range roots {
		path := projectConfigPath(root)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(fmt.Sprintf(`{"max_ctx":%d}`, 100000+i)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 6; i++ {
		path := roots[i%2]
		body, _ := json.Marshal(map[string]string{"path": path})
		a.selected = "old"
		a.messages = []openai.ChatCompletionMessage{{Role: "user", Content: "old workspace"}}
		w := callAPI(a, "PUT", "/api/workspace", string(body), true)
		if w.Code != 200 || a.cfg.workspace != path || a.selected != "" || len(a.messages) != 0 || a.cfg.maxCtx != 100000+i%2 {
			t.Fatalf("switch %d: %d %s", i, w.Code, w.Body)
		}
	}
	before := a.cfg.workspace
	a.busy = true
	body, _ := json.Marshal(map[string]string{"path": roots[0]})
	if w := callAPI(a, "PUT", "/api/workspace", string(body), true); w.Code != 409 || a.cfg.workspace != before {
		t.Fatal("busy switch mutated workspace")
	}
	a.busy = false
}

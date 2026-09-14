package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	openai "github.com/sashabaranov/go-openai"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLargeContentRoundtripAndIntegrity(t *testing.T) {
	s := newSession(filepath.Join(t.TempDir(), "sess-large.jsonl"), t.TempDir())
	defer s.Close()
	original := strings.Repeat("日志abc\n", 1400000)
	m := openai.ChatCompletionMessage{Role: "tool", ToolCallID: "c", Content: original}
	projected, err := s.recordProjected(m, &toolOutcome{OK: false, Summary: "failed command"})
	if err != nil {
		t.Fatal(err)
	}
	if len(projected.Content) > largeContentThreshold || m.Content != original {
		t.Fatal("projection or original changed")
	}
	rows, err := readAPISessionMessages(s.path)
	if err != nil {
		t.Fatal(err)
	}
	var row sessionMessageRecord
	if err := json.Unmarshal(rows[0], &row); err != nil {
		t.Fatal(err)
	}
	ref := row.LargeContent["content"]
	if ref == "" || row.ToolOutcome == nil || row.ToolOutcome.OK {
		t.Fatal("lost reference/outcome")
	}
	var restored strings.Builder
	for offset := int64(0); offset < int64(len(original)); {
		page, total, next, err := readLargeContent(s.path, ref, offset, 32767)
		if err != nil || next <= offset || total != int64(len(original)) {
			t.Fatalf("%d %d %v", offset, next, err)
		}
		restored.WriteString(page)
		offset = next
	}
	if restored.String() != original {
		t.Fatal("page bytes lost or duplicated")
	}
	if _, _, _, err := readLargeContent(filepath.Join(t.TempDir(), "sess-other.jsonl"), ref, 0, 32); err == nil {
		t.Fatal("cross-session reference accepted")
	}
	if _, _, _, err := readLargeContent(s.path, ref, 1, 32); err == nil {
		t.Fatal("mid rune accepted")
	}
	path, _, _, _ := parseLargeContentRef(s.path, ref)
	if err := os.WriteFile(path, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := readLargeContent(s.path, ref, 0, 32); err == nil {
		t.Fatal("changed attachment accepted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSession(s.path); err == nil {
		t.Fatal("resume silently accepted missing attachment")
	}
}

func TestLargeArgumentsExecuteUnmodifiedAndResumeProjected(t *testing.T) {
	captureEvents(t)
	root := t.TempDir()
	args := mustJSON(t, map[string]string{"payload": strings.Repeat("x", 1100000)})
	requests := 0
	executed := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		requests++
		w.Header().Set("Content-Type", "text/event-stream")
		delta := map[string]interface{}{}
		finish := "stop"
		if requests == 1 {
			delta = map[string]interface{}{"content": "running fixture", "reasoning_content": strings.Repeat("thinking ", 12000), "tool_calls": []interface{}{map[string]interface{}{"index": 0, "id": "big", "type": "function", "function": map[string]string{"name": "fixture", "arguments": args}}}}
			finish = "tool_calls"
		} else {
			for _, m := range req.Messages {
				if len(m.Content) > largeContentThreshold || len(m.ReasoningContent) > largeContentThreshold {
					t.Error("full data reinjected")
				}
				for _, c := range m.ToolCalls {
					if len(c.Function.Arguments) > largeContentThreshold {
						t.Error("full arguments reinjected")
					}
				}
			}
			delta["content"] = "done"
		}
		b, _ := json.Marshal(map[string]interface{}{"choices": []interface{}{map[string]interface{}{"delta": delta}}})
		fmt.Fprintf(w, "data: %s\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"%s\"}]}\n\ndata: [DONE]\n\n", b, finish)
	}))
	defer srv.Close()
	cc := openai.DefaultConfig("fixture")
	cc.BaseURL = srv.URL
	old, oldCfg := sess, curCfg
	sess = newSession(filepath.Join(root, "sess-run.jsonl"), root)
	curCfg = nil
	defer func() { sess.Close(); sess = old; curCfg = oldCfg; resetInterrupt() }()
	p := defaultPermissionConfig()
	p.Profile = "open"
	reg := NewRegistry(&Policy{Workspace: root}, nil, filepath.Join(root, "audit.jsonl"), filepath.Join(root, "manifest.jsonl"), false, false, false, strings.NewReader(""), root, root, "run", p)
	reg.register(openai.Tool{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "fixture"}}, func(got string) (string, error) {
		executed = got == args
		return strings.Repeat("result ", 170000), nil
	})
	msgs := []openai.ChatCompletionMessage{{Role: "user", Content: "test"}}
	cfg := &config{workspace: root, maxCtx: 256000, maxRounds: 2, model: "test", llmFirstChunkTimeout: time.Second, llmIdleTimeout: time.Second}
	answer, err := streamTurn(openai.NewClientWithConfig(cc), reg, cfg, &msgs, nil)
	if err != nil || !executed || answer != "done" {
		t.Fatalf("executed=%v answer=%s err=%v", executed, answer, err)
	}
	loaded, err := loadSession(sess.path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded[0].ToolCalls[0].Function.Arguments) >= largeContentThreshold {
		t.Fatal("resume hydrated big arguments")
	}
	full, err := loadSessionFullRecords(sess.path)
	if err != nil {
		t.Fatal(err)
	}
	if full[0].Content != "running fixture" || full[0].ToolCalls[0].Function.Arguments != args || full[1].ToolOutcome == nil || !full[1].ToolOutcome.OK {
		t.Fatal("full recovery lost original or outcome")
	}
}

func TestLargeFileUTF8AndGBKPages(t *testing.T) {
	for _, encoding := range []string{"utf8", "gbk"} {
		t.Run(encoding, func(t *testing.T) {
			reg, root := newTestRegistry(t)
			original := strings.Repeat("日志中文abc", 1100000)
			raw := []byte(original)
			if encoding == "gbk" {
				raw = gbkBytes(t, original)
			}
			path := put(t, root, "large.log", raw)
			var joined strings.Builder
			nextRE := regexp.MustCompile(`next_byte_offset=(\d+)`)
			for offset := int64(0); offset < int64(len(raw)); {
				out, err := reg.toolRead(mustJSON(t, map[string]interface{}{"path": path, "byte_offset": offset, "byte_limit": 32767}))
				if err != nil {
					t.Fatal(err)
				}
				parts := strings.SplitN(out, "\n", 2)
				m := nextRE.FindStringSubmatch(parts[0])
				if len(m) != 2 {
					t.Fatal(parts[0])
				}
				next, _ := strconv.ParseInt(m[1], 10, 64)
				if next <= offset {
					t.Fatal("no progress")
				}
				joined.WriteString(parts[1])
				offset = next
			}
			if joined.String() != original {
				t.Fatalf("encoding %s lost data", encoding)
			}
			if got, err := reg.toolRead(mustJSON(t, map[string]interface{}{"path": path})); err != nil || len(got) > 65536 {
				t.Fatalf("unbounded first line: %d %v", len(got), err)
			}
		})
	}
}

func TestLargeAPIIndexedPagesAndAttachmentRange(t *testing.T) {
	a := testAPI(t)
	dir := filepath.Join(a.cfg.exeDir, "data", "sessions")
	os.MkdirAll(dir, 0700)
	s := newSession(filepath.Join(dir, "sess-api-large.jsonl"), a.cfg.workspace)
	defer s.Close()
	if err := s.record(openai.ChatCompletionMessage{Role: "user", Content: "hello"}); err != nil {
		t.Fatal(err)
	}
	original := strings.Repeat("中文", 200000)
	if err := s.record(openai.ChatCompletionMessage{Role: "tool", ToolCallID: "big", Content: original}, &toolOutcome{OK: true}); err != nil {
		t.Fatal(err)
	}
	w := callAPI(a, "GET", "/api/sessions/api-large/messages?offset=1&limit=1", "", true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "largeContent") || w.Body.Len() > 200000 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	w = callAPI(a, "GET", "/api/tool-result?ref=session:api-large%23tool:big&offset=0&limit=99", "", true)
	var response struct {
		Result string `json:"result"`
		More   bool   `json:"hasMore"`
		Next   int    `json:"nextOffset"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || !response.More || response.Result != original[:response.Next] {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	idx, err := indexAPIHistory(s.path)
	if err != nil {
		t.Fatal(err)
	}
	again, err := indexAPIHistory(s.path)
	if err != nil || idx != again {
		t.Fatal("unchanged history reindexed")
	}
	// CRLF record positions must remain correct as well.
	data, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "sess-crlf.jsonl")
	os.WriteFile(path, bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n")), 0600)
	rows, count, err := apiPagedMessages(path, 1, 1)
	if err != nil || count != 2 || len(rows) != 1 || !json.Valid(rows[0]) {
		t.Fatalf("CRLF index %d %v", count, err)
	}
}

func TestLargeExternalizationFailureDoesNotCommitReference(t *testing.T) {
	root := t.TempDir()
	s := newSession(filepath.Join(root, "sess-denied.jsonl"), root)
	defer s.Close()
	if err := os.WriteFile(s.path+".content", []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := s.recordProjected(openai.ChatCompletionMessage{Role: "tool", Content: strings.Repeat("x", 100000)})
	if err == nil || s.n != 0 {
		t.Fatal("failed storage marked committed")
	}
	rows, err := readAPISessionMessages(s.path)
	if err != nil || len(rows) != 0 {
		t.Fatal("dangling ref written")
	}
}

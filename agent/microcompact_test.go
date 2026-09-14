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

func writeSummaryFixture(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "text/event-stream")
	b, _ := json.Marshal(map[string]interface{}{"choices": []interface{}{map[string]interface{}{"index": 0, "delta": map[string]string{"content": text}}}})
	fmt.Fprintf(w, "data: %s\n\ndata: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n", b)
}

func microFixture(t *testing.T) []openai.ChatCompletionMessage {
	t.Helper()
	old, oldCfg := sess, curCfg
	root := t.TempDir()
	sess = newSession(filepath.Join(root, "sess-micro.jsonl"), root)
	curCfg = nil
	t.Cleanup(func() { sess.Close(); sess = old; curCfg = oldCfg; resetInterrupt() })
	msgs := []openai.ChatCompletionMessage{{Role: "system", Content: "system"}, {Role: "user", Content: "preserve current task"}}
	for i := 0; i < 14; i++ {
		msgs = append(msgs, protocolRound(fmt.Sprintf("micro-%d", i), fmt.Sprintf(`{"path":"controller-%d.go"}`, i), strings.Repeat(fmt.Sprintf("FACT-%d ", i), 1500))...)
		msgs[len(msgs)-2].ToolCalls[0].Function.Name = "read"
	}
	for _, m := range msgs {
		if err := sess.record(m); err != nil {
			t.Fatal(err)
		}
	}
	return msgs
}

func TestMicroRetainsRecentAndOriginalAndIsIdempotent(t *testing.T) {
	msgs := microFixture(t)
	before := contextChars(msgs)
	got, n := microCompact(msgs, 8)
	if n != 6 || contextChars(got) >= before {
		t.Fatalf("changed=%d", n)
	}
	assertValidToolProtocol(t, got)
	if strings.HasPrefix(msgs[3].Content, microMarker) || !strings.HasPrefix(got[3].Content, microMarker) {
		t.Fatal("original mutated or old result not projected")
	}
	again, n := microCompact(got, 8)
	if n != 0 || contextChars(again) != contextChars(got) {
		t.Fatal("not idempotent")
	}
	record, err := apiFindToolResult(sess.path, "micro-0")
	if err != nil || !strings.HasPrefix(record.Content, "FACT-0") {
		t.Fatal("history overwritten")
	}
	files, _ := os.ReadDir(sess.path + ".content")
	_, n = microCompact(msgs, 8)
	files2, _ := os.ReadDir(sess.path + ".content")
	if n != 6 || len(files) != len(files2) {
		t.Fatal("resume duplicated attachments")
	}
}

func TestMicroAvoidsSummaryRequest(t *testing.T) {
	msgs := microFixture(t)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		t.Error("unnecessary summary")
		writeSummaryFixture(w, "summary")
	}))
	defer srv.Close()
	cc := openai.DefaultConfig("test")
	cc.BaseURL = srv.URL
	cfg := &config{maxCtx: 180000, microKeepRecent: 8, llmCompressTimeout: time.Second}
	if requestContextChars(msgs, nil) <= int(float64(cfg.maxCtx)*compressThreshold) {
		t.Fatal("fixture below threshold")
	}
	if err := maybeCompressContext(context.Background(), openai.NewClientWithConfig(cc), cfg, nil, &msgs); err != nil {
		t.Fatal(err)
	}
	if calls != 0 || requestContextChars(msgs, nil) > int(float64(cfg.maxCtx)*compressThreshold) {
		t.Fatal("micro did not fit")
	}
}

func TestMicroSummarySeesOriginalAndUsesOnlyStreaming(t *testing.T) {
	msgs := microFixture(t)
	constraint := compactIndexMarker + "\n用户引用索引标记并要求保留此约束"
	msgs = append(msgs, openai.ChatCompletionMessage{Role: "user", Content: constraint})
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body openai.ChatCompletionRequest
		json.NewDecoder(r.Body).Decode(&body)
		if !body.Stream || len(body.Tools) != 0 || !strings.Contains(body.Messages[0].Content, "FACT-0 FACT-0") || !strings.Contains(body.Messages[0].Content, "当前工作") {
			t.Error("summary missing originals/structure or not streaming")
		}
		writeSummaryFixture(w, "已确认事实：controller-0.go。当前工作：迁移；未验证。")
	}))
	defer srv.Close()
	cc := openai.DefaultConfig("test")
	cc.BaseURL = srv.URL
	if err := maybeCompressContext(context.Background(), openai.NewClientWithConfig(cc), &config{maxCtx: 70000, llmCompressTimeout: time.Second}, nil, &msgs); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
	assertValidToolProtocol(t, msgs)
	if !strings.HasPrefix(msgs[0].Content, compactIndexMarker) {
		t.Fatal("recovery index missing")
	}
	parts := strings.Split(msgs[0].Content, "\"")
	if len(parts) < 3 {
		t.Fatal("index reference missing")
	}
	index, _, _, err := readLargeContent(sess.path, parts[1], 0, largeContentMaxReadBytes)
	if err != nil || !strings.Contains(index, "controller-10.go") {
		t.Fatal("summary lost non-micro file index", err)
	}
	found := false
	for _, m := range msgs {
		if m.Role == "user" && m.Content == constraint {
			found = true
		}
	}
	if !found {
		t.Fatal("user quoted marker was removed")
	}
}

func TestMicroAttachmentFailureLeavesContent(t *testing.T) {
	msgs := microFixture(t)
	os.WriteFile(sess.path+".content", []byte("blocked"), 0600)
	got, n := microCompact(msgs, 8)
	if n != 0 || contextChars(got) != contextChars(msgs) {
		t.Fatal("lost unsaved results")
	}
}

func TestCompressionStreamRejectsBrokenAndCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n")
	}))
	defer srv.Close()
	cc := openai.DefaultConfig("test")
	cc.BaseURL = srv.URL
	client := openai.NewClientWithConfig(cc)
	cfg := &config{maxCtx: 10000}
	if _, err := streamCompression(context.Background(), client, cfg, openai.ChatCompletionRequest{}); err == nil {
		t.Fatal("accepted broken stream")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := streamCompression(ctx, client, cfg, openai.ChatCompletionRequest{}); err == nil {
		t.Fatal("ignored cancellation")
	}
}

func TestMicroLatestParallelGroupAndWhitelist(t *testing.T) {
	msgs := microFixture(t)
	// Merge the last ten assistant groups into one valid parallel call group.
	start := len(msgs) - 20
	group := openai.ChatCompletionMessage{Role: "assistant"}
	results := []openai.ChatCompletionMessage{}
	for i := start; i < len(msgs); i += 2 {
		group.ToolCalls = append(group.ToolCalls, msgs[i].ToolCalls...)
		results = append(results, msgs[i+1])
	}
	msgs = append(append(msgs[:start], group), results...)
	got, n := microCompact(msgs, 1)
	if n != 4 {
		t.Fatalf("protected group changed: %d", n)
	}
	assertValidToolProtocol(t, got)
	for i := start + 1; i < len(got); i++ {
		if strings.HasPrefix(got[i].Content, microMarker) {
			t.Fatal("latest parallel group cleared")
		}
	}
	msgs[2].ToolCalls[0].Function.Name = "checkpoint"
	got, _ = microCompact(msgs, 1)
	if got[3].Content != msgs[3].Content {
		t.Fatal("non whitelist changed")
	}
}

func TestMicroConfigPersistsAndValidates(t *testing.T) {
	a := testAPI(t)
	t.Setenv("USERPROFILE", t.TempDir())
	w := callAPI(a, "PUT", "/api/config", `{"micro_keep_recent":12}`, true)
	if w.Code != 200 || a.cfg.microKeepRecent != 12 || !strings.Contains(w.Body.String(), `"micro_keep_recent":12`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	doc, _, err := readConfigDocument(globalConfigPath(os.Getenv("USERPROFILE")))
	if err != nil || string(doc["micro_keep_recent"]) != "12" {
		t.Fatal("not persisted")
	}
	w = callAPI(a, "PUT", "/api/config", `{"micro_keep_recent":0}`, true)
	if w.Code != 400 || a.cfg.microKeepRecent != 12 {
		t.Fatal("invalid config applied")
	}
}

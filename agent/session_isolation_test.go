package main

import (
	"bytes"
	"encoding/json"
	openai "github.com/sashabaranov/go-openai"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSONLineWriteReadBoundary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "boundary.jsonl")
	base := map[string]string{"role": "user", "content": ""}
	encoded, _ := json.Marshal(base)
	base["content"] = strings.Repeat("x", maxSessionRecordBytes-len(encoded)-1)
	if err := appendJSONLine(path, base); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != maxSessionRecordBytes {
		t.Fatalf("bytes=%d", len(before))
	}
	if rows, err := readAPISessionMessages(path); err != nil || len(rows) != 1 {
		t.Fatalf("boundary unreadable: %v", err)
	}
	base["content"] += "x"
	if err := appendJSONLine(path, base); err == nil {
		t.Fatal("oversize accepted")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("oversize modified existing file")
	}
	missing := filepath.Join(t.TempDir(), "missing.jsonl")
	if err := appendJSONLine(missing, base); err == nil {
		t.Fatal("oversize accepted")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("oversize created file")
	}
	// Session records use writeJSONLine directly, not appendJSONLine.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := writeJSONLine(f, base); err == nil {
		t.Fatal("direct writer accepted oversize")
	}
	after, _ = os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("direct writer changed file")
	}
	// JSON escaping, not source character count, determines the limit.
	if _, err := encodeJSONLine(map[string]string{"content": strings.Repeat("\x00", maxSessionRecordBytes/5)}); err == nil {
		t.Fatal("escaped size ignored")
	}
}

func TestSessionListIsolatesUnreadableFiles(t *testing.T) {
	a := testAPI(t)
	dir := filepath.Join(a.cfg.exeDirStore(), "data", "sessions")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	good := newSession(filepath.Join(dir, "sess-good.jsonl"), a.cfg.workspace)
	if err := good.record(openai.ChatCompletionMessage{Role: "user", Content: "healthy"}); err != nil {
		t.Fatal(err)
	}
	good.Close()
	bad := map[string][]byte{
		"sess-large.jsonl":   append([]byte(`{"role":"user","content":"`), append(bytes.Repeat([]byte("x"), 10900000), []byte("\"}\n")...)...),
		"sess-invalid.jsonl": []byte("{invalid}\n"),
		"sess-partial.jsonl": []byte(`{"role":"user","content":"unfinished"}`),
	}
	for name, data := range bad {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	w := callAPI(a, "GET", "/api/sessions", "", true)
	var response struct {
		Sessions []struct {
			SessionID string `json:"sessionId"`
		} `json:"sessions"`
		Errors []struct {
			FileName string `json:"fileName"`
			Code     string `json:"code"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || len(response.Sessions) != 1 || response.Sessions[0].SessionID != "good" || len(response.Errors) != len(bad) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	for _, problem := range response.Errors {
		if _, ok := bad[problem.FileName]; !ok || problem.Code != "session_unreadable" {
			t.Fatalf("unexpected problem %+v", problem)
		}
	}
	for name, data := range bad {
		after, _ := os.ReadFile(filepath.Join(dir, name))
		if !bytes.Equal(data, after) {
			t.Fatal("list modified source")
		}
	}
	if w := callAPI(a, "GET", "/api/sessions/large/messages", "", true); w.Code == 200 {
		t.Fatal("direct read must still reject")
	}
	if w := callAPI(a, "GET", "/api/sessions/good/messages", "", true); w.Code != 200 {
		t.Fatalf("good history: %d", w.Code)
	}
}

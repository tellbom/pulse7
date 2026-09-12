package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestSessionWriteFailureIsReturned(t *testing.T) {
	s := newSession(t.TempDir(), t.TempDir())
	err := s.record(openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "not saved"})
	if !errors.Is(err, errSessionStorage) {
		t.Fatalf("record error = %v, want session storage failure", err)
	}
	if s.n != 0 {
		t.Fatalf("session count = %d after failed write, want 0", s.n)
	}
}

func TestClearBoundaryReplacesHistoryOnResume(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sess-clear.jsonl")
	s := newSession(path, t.TempDir())
	if err := s.record(openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: "old system"}); err != nil {
		t.Fatal(err)
	}
	if err := s.record(openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "old task"}); err != nil {
		t.Fatal(err)
	}
	if err := s.recordClear("new system with AGENT rules"); err != nil {
		t.Fatal(err)
	}
	if err := s.record(openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "new task"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	msgs, err := loadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[0].Role != openai.ChatMessageRoleSystem ||
		msgs[0].Content != "new system with AGENT rules" || msgs[1].Content != "new task" {
		t.Fatalf("messages after clear = %#v", msgs)
	}
}

func TestLoadSessionRejectsOversizedRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sess-large.jsonl")
	meta := []byte("{\"role\":\"_meta\",\"workspace\":\"C:\\\\work\",\"task_id\":\"large\"}\n")
	record := append([]byte("{\"role\":\"user\",\"content\":\""), bytes.Repeat([]byte("x"), maxSessionRecordBytes)...)
	record = append(record, []byte("\"}\n")...)
	if err := os.WriteFile(path, append(meta, record...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSession(path); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("loadSession error = %v, want explicit size error", err)
	}
}

func TestLoadSessionDistinguishesCorruptTruncatedAndEmpty(t *testing.T) {
	dir := t.TempDir()
	corrupt := filepath.Join(dir, "sess-corrupt.jsonl")
	if err := os.WriteFile(corrupt, []byte("{\"role\":\"_meta\",\"workspace\":\"x\"}\nnot-json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSession(corrupt); err == nil || !strings.Contains(err.Error(), "invalid session record") {
		t.Fatalf("corrupt error = %v", err)
	}

	truncated := filepath.Join(dir, "sess-truncated.jsonl")
	if err := os.WriteFile(truncated, []byte("{\"role\":\"_meta\",\"workspace\":\"x\"}\n{\"role\":\"user\"}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSession(truncated); err == nil || !strings.Contains(err.Error(), "truncated") {
		t.Fatalf("truncated error = %v", err)
	}

	empty := filepath.Join(dir, "sess-empty.jsonl")
	if err := os.WriteFile(empty, []byte("{\"role\":\"_meta\",\"workspace\":\"x\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	msgs, err := loadSession(empty)
	if err != nil || len(msgs) != 0 {
		t.Fatalf("legal empty session messages=%#v error=%v", msgs, err)
	}
}

func TestManifestWriteFailureIsReturnedAfterMutation(t *testing.T) {
	r, _, workspace, _ := newPermissionTestRegistry(t, "standard", nil, "")
	r.man.path = t.TempDir()
	got := r.Execute("write", `{"path":"changed.txt","content":"written"}`)
	if !strings.Contains(got, "manifest persistence failed") {
		t.Fatalf("Execute(write) = %q, want manifest persistence error", got)
	}
	b, err := os.ReadFile(filepath.Join(workspace, "changed.txt"))
	if err != nil || string(b) != "written" {
		t.Fatalf("mutation result bytes=%q error=%v", b, err)
	}
}

func TestAuditWriteFailureIsReturned(t *testing.T) {
	r, _, _, _ := newPermissionTestRegistry(t, "standard", nil, "")
	r.auditPath = t.TempDir()
	if err := r.audit("read", `{"path":"x"}`, "ok"); err == nil {
		t.Fatal("audit unexpectedly succeeded with directory as file path")
	}
}

func TestStreamTurnPreservesInterruptForCaller(t *testing.T) {
	r, _, _, _ := newPermissionTestRegistry(t, "standard", nil, "")
	oldSession := sess
	sess = newSession(filepath.Join(t.TempDir(), "sess-interrupt.jsonl"), t.TempDir())
	defer func() {
		sess = oldSession
		resetInterrupt()
	}()
	atomic.StoreInt32(&interruptFlag, 1)

	var msgs []openai.ChatCompletionMessage
	_, err := streamTurn(nil, r, &config{maxRounds: 1}, &msgs, &turnStats{})
	if !errors.Is(err, errInterrupted) {
		t.Fatalf("streamTurn error = %v, want interrupted", err)
	}
	if !interrupted() {
		t.Fatal("streamTurn cleared interrupt before caller handled it")
	}
}

func TestConfirmationWaitIsCancelledByInterrupt(t *testing.T) {
	r, _, _, _ := newPermissionTestRegistry(t, "strict", nil, "")
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	r.input = newLineInput(reader)
	atomic.StoreInt32(&interruptFlag, 1)
	defer resetInterrupt()

	err := r.authorize("shell", `{"command":"echo should-not-run"}`)
	if !errors.Is(err, errInterrupted) {
		t.Fatalf("authorize error = %v, want interrupted", err)
	}
}

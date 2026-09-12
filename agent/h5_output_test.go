package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestH5OutputEventDoesNotRequireTaskOutputCall(t *testing.T) {
	protocol, _ := captureEvents(t)
	path := filepath.Join(t.TempDir(), "task.output")
	if err := os.WriteFile(path, []byte("first\n"), 0600); err != nil {
		t.Fatal(err)
	}
	proc := &fakeManagedProcess{done: make(chan processWaitResult, 1)}
	task := &backgroundTask{ID: "output-event", OutputPath: path, Status: "running", proc: proc}
	m := newBackgroundTaskManager(&jobObjectRunner{}, t.TempDir(), t.TempDir())
	finished := make(chan struct{})
	go func() { m.observe(task); close(finished) }()
	time.Sleep(250 * time.Millisecond)
	if err := os.WriteFile(path, []byte("first\nsecond\n"), 0600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(250 * time.Millisecond)
	proc.done <- processWaitResult{exitCode: 0}
	<-finished
	if !strings.Contains(protocol.String(), `"action":"output"`) || !strings.Contains(protocol.String(), `"nextOffset":13`) {
		t.Fatalf("no unsolicited output notification: %s", protocol.String())
	}
}

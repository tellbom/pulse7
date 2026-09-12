package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuditShellCommandsIgnoresPermissionEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	data := "{\"ts\":\"one\",\"task\":\"t1\",\"tool\":\"shell\",\"args\":\"{\\\"command\\\":\\\"echo ok\\\"}\"}\n" +
		"{\"ts\":\"two\",\"task\":\"t1\",\"tool\":\"shell\",\"event\":\"permission\",\"decision\":\"allow\"}\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	commands, err := auditShellCommands(path, "t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 1 || commands[0] != "  1. echo ok  (one)" {
		t.Fatalf("commands=%#v", commands)
	}
}

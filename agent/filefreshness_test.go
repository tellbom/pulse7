package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newFreshnessTestRegistry(t *testing.T) (*Registry, string) {
	t.Helper()
	workspace := t.TempDir()
	r := &Registry{
		policy: &Policy{Workspace: workspace}, workspace: workspace,
		readFiles: map[string]fileReadState{}, man: &manifest{path: filepath.Join(t.TempDir(), "manifest.jsonl")},
	}
	return r, workspace
}

func TestEditRejectsFileModifiedAfterRead(t *testing.T) {
	r, workspace := newFreshnessTestRegistry(t)
	path := filepath.Join(workspace, "state.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := r.toolRead(`{"path":"state.txt"}`); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("changed outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := r.toolEdit(`{"path":"state.txt","old_string":"changed outside","new_string":"agent"}`)
	if err == nil || !strings.Contains(err.Error(), "文件已被外部修改") || !strings.Contains(err.Error(), "重新 read") {
		t.Fatalf("edit error = %v", err)
	}
	if b, readErr := os.ReadFile(path); readErr != nil || string(b) != "changed outside\n" {
		t.Fatalf("external content changed: bytes=%q err=%v", b, readErr)
	}
}

func TestEditSucceedsWhenReadFileIsUnchanged(t *testing.T) {
	r, workspace := newFreshnessTestRegistry(t)
	path := filepath.Join(workspace, "state.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := r.toolRead(`{"path":"state.txt"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.toolEdit(`{"path":"state.txt","old_string":"before","new_string":"after"}`); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "after\n" {
		t.Fatalf("edited content = %q err=%v", b, err)
	}
}

func TestEditRequiresPriorRead(t *testing.T) {
	r, workspace := newFreshnessTestRegistry(t)
	path := filepath.Join(workspace, "state.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := r.toolEdit(`{"path":"state.txt","old_string":"before","new_string":"after"}`)
	if err == nil || !strings.Contains(err.Error(), "尚未 read") || !strings.Contains(err.Error(), "先 read") {
		t.Fatalf("edit error = %v", err)
	}
}

func TestWriteCreatesNewFileWithoutPriorRead(t *testing.T) {
	r, workspace := newFreshnessTestRegistry(t)
	if _, err := r.toolWrite(`{"path":"new.txt","content":"created\n"}`); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(workspace, "new.txt")); err != nil || string(b) != "created\n" {
		t.Fatalf("new content = %q err=%v", b, err)
	}
}

func TestRereadAfterExternalChangeAllowsEdit(t *testing.T) {
	r, workspace := newFreshnessTestRegistry(t)
	path := filepath.Join(workspace, "state.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := r.toolRead(`{"path":"state.txt"}`); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("external\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := r.toolRead(`{"path":"state.txt"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.toolEdit(`{"path":"state.txt","old_string":"external","new_string":"accepted"}`); err != nil {
		t.Fatal(err)
	}
}

func TestOverwriteRequiresFreshReadByTimeAndSize(t *testing.T) {
	r, workspace := newFreshnessTestRegistry(t)
	path := filepath.Join(workspace, "state.txt")
	if err := os.WriteFile(path, []byte("same-size\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := r.toolWrite(`{"path":"state.txt","content":"blocked\n"}`); err == nil || !strings.Contains(err.Error(), "尚未 read") {
		t.Fatalf("overwrite without read error = %v", err)
	}
	if _, err := r.toolRead(`{"path":"state.txt"}`); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	if _, err := r.toolWrite(`{"path":"state.txt","content":"blocked\n"}`); err == nil || !strings.Contains(err.Error(), "外部修改") {
		t.Fatalf("overwrite after mtime change error = %v", err)
	}
}

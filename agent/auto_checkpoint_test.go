package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func enableRegistryExecutionForCheckpointTest(t *testing.T, r *Registry) {
	t.Helper()
	r.tools = map[string]Tool{}
	r.permissions = permissionConfig{Profile: "open"}
	r.auditPath = filepath.Join(t.TempDir(), "audit.jsonl")
	r.tools["write"] = Tool{Fn: r.toolWrite}
}

func TestFirstModelMutationCreatesAutomaticCheckpoint(t *testing.T) {
	r, g := newRollbackTestRegistry(t)
	commitTestFiles(t, g, map[string]string{"state.txt": "baseline\n"})
	enableRegistryExecutionForCheckpointTest(t, r)
	mustReadBeforeMutation(t, r, "state.txt")

	got := r.Execute("write", `{"path":"state.txt","content":"changed\n"}`)
	if strings.HasPrefix(got, "error:") {
		t.Fatalf("Execute(write) = %q", got)
	}
	entries := readCheckpointMetadata(t, g)
	if len(entries) != 1 || entries[0].Kind != "auto" {
		t.Fatalf("checkpoint metadata = %+v, want one auto checkpoint", entries)
	}
	if _, err := r.toolRollback(`{"To":1}`); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(g.workspace, "state.txt")); err != nil || string(b) != "baseline\n" {
		t.Fatalf("automatic checkpoint did not preserve pre-write state: bytes=%q err=%v", b, err)
	}
}

func TestModelAndAutomaticCheckpointsAreDistinct(t *testing.T) {
	r, g := newRollbackTestRegistry(t)
	commitTestFiles(t, g, map[string]string{"state.txt": "baseline\n"})
	enableRegistryExecutionForCheckpointTest(t, r)
	mustReadBeforeMutation(t, r, "state.txt")

	if _, err := r.toolCheckpoint(`{}`); err != nil {
		t.Fatal(err)
	}
	if got := r.Execute("write", `{"path":"state.txt","content":"changed\n"}`); strings.HasPrefix(got, "error:") {
		t.Fatalf("Execute(write) = %q", got)
	}
	entries := readCheckpointMetadata(t, g)
	if len(entries) != 2 || entries[0].Kind != "model" || entries[1].Kind != "auto" {
		t.Fatalf("checkpoint kinds = %+v, want model then auto", entries)
	}
}

func TestAutomaticCheckpointRepeatsAfterTenSuccessfulMutations(t *testing.T) {
	r, g := newRollbackTestRegistry(t)
	commitTestFiles(t, g, map[string]string{"state.txt": "baseline\n"})
	enableRegistryExecutionForCheckpointTest(t, r)
	mustReadBeforeMutation(t, r, "state.txt")

	for i := 1; i <= 11; i++ {
		args := fmt.Sprintf(`{"path":"state.txt","content":"version-%d\n"}`, i)
		if got := r.Execute("write", args); strings.HasPrefix(got, "error:") {
			t.Fatalf("mutation %d = %q", i, got)
		}
	}
	entries := readCheckpointMetadata(t, g)
	if len(entries) != 2 || entries[0].Kind != "auto" || entries[1].Kind != "auto" {
		t.Fatalf("checkpoint metadata = %+v, want two auto checkpoints", entries)
	}
}

func TestDeniedMutationDoesNotCreateAutomaticCheckpoint(t *testing.T) {
	r, g := newRollbackTestRegistry(t)
	commitTestFiles(t, g, map[string]string{"state.txt": "baseline\n"})
	enableRegistryExecutionForCheckpointTest(t, r)
	r.permissions.Rules = []permissionRule{{Tool: "write", Pattern: "*", Action: "deny"}}

	if got := r.Execute("write", `{"path":"state.txt","content":"changed\n"}`); !strings.HasPrefix(got, "error:") {
		t.Fatalf("Execute(write) = %q, want denial", got)
	}
	if _, err := os.Stat(checkpointMetadataTestPath(g)); !os.IsNotExist(err) {
		t.Fatalf("denied mutation created checkpoint metadata: %v", err)
	}
}

func TestAutomaticCheckpointFailureStopsMutation(t *testing.T) {
	r, _, workspace, _ := newPermissionTestRegistry(t, "open", nil, "")
	r.autoCheckpointDone = false
	got := r.Execute("write", `{"path":"state.txt","content":"changed\n"}`)
	if !strings.Contains(got, "automatic checkpoint failed") {
		t.Fatalf("Execute(write) = %q, want checkpoint failure", got)
	}
	if _, err := os.Stat(filepath.Join(workspace, "state.txt")); !os.IsNotExist(err) {
		t.Fatalf("write ran after checkpoint failure: %v", err)
	}
}

func readCheckpointMetadata(t *testing.T, g *gitOps) []checkpointMetadata {
	t.Helper()
	f, err := os.Open(checkpointMetadataTestPath(g))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var entries []checkpointMetadata
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var entry checkpointMetadata
		if err := json.Unmarshal(sc.Bytes(), &entry); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return entries
}

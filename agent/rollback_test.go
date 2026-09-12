package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newRollbackTestRegistry(t *testing.T) (*Registry, *gitOps) {
	t.Helper()
	gitExe, err := exec.LookPath("git.exe")
	if err != nil {
		t.Skip("git.exe is required for checkpoint tests")
	}
	workspace := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	g := &gitOps{
		gitExe: gitExe, workspace: workspace, mode: 1, taskID: "task-a",
		indexDir: filepath.Join(t.TempDir(), "indexes"),
	}
	if err := os.MkdirAll(g.indexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := g.gitRun("", "init"); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	r := &Registry{
		policy:    &Policy{Workspace: workspace},
		workspace: workspace,
		taskID:    "task-a",
		man:       &manifest{path: filepath.Join(t.TempDir(), "manifest.jsonl")},
		git:       g,
	}
	return r, g
}

func checkpointMetadataTestPath(g *gitOps) string {
	return filepath.Join(g.indexDir, "checkpoint-metadata.jsonl")
}

func captureTestOutput(t *testing.T, fn func()) string {
	t.Helper()
	var captured bytes.Buffer
	old := teeOut
	teeOut = &captured
	defer func() { teeOut = old }()
	fn()
	return captured.String()
}

func commitTestFiles(t *testing.T, g *gitOps, files map[string]string) {
	t.Helper()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(g.workspace, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := g.gitRun("", "add", "--", "."); err != nil {
		t.Fatalf("git add: %v: %s", err, out)
	}
	if out, err := g.gitRun("", "commit", "-m", "baseline"); err != nil {
		t.Fatalf("git commit: %v: %s", err, out)
	}
}

func TestRollbackPreservesIndexAndUnrelatedWorkspaceFiles(t *testing.T) {
	// A regression in the rollback implementation would either add --staged
	// again or restore the whole tree instead of the Agent-recorded paths.
	r, g := newRollbackTestRegistry(t)
	commitTestFiles(t, g, map[string]string{
		"agent.txt":     "base\n",
		"user-only.txt": "original\n",
	})

	if err := os.WriteFile(filepath.Join(g.workspace, "agent.txt"), []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := g.gitRun("", "add", "--", "agent.txt"); err != nil {
		t.Fatalf("stage fixture: %v: %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(g.workspace, "agent.txt"), []byte("unstaged-at-checkpoint\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Checkpoint(); err != nil {
		t.Fatal(err)
	}
	indexBefore, err := g.gitRun("", "diff", "--cached", "--binary")
	if err != nil {
		t.Fatal(err)
	}

	mustReadBeforeMutation(t, r, "agent.txt")
	if _, err := r.toolWrite(`{"path":"agent.txt","content":"agent change\n"}`); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(g.workspace, "user-only.txt"), []byte("user change after checkpoint\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := r.toolRollback(`{"To":1}`); err != nil {
		t.Fatal(err)
	}

	indexAfter, err := g.gitRun("", "diff", "--cached", "--binary")
	if err != nil {
		t.Fatal(err)
	}
	if indexAfter != indexBefore {
		t.Fatalf("rollback changed the user's index\nbefore:\n%s\nafter:\n%s", indexBefore, indexAfter)
	}
	got, err := os.ReadFile(filepath.Join(g.workspace, "agent.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "unstaged-at-checkpoint\n" {
		t.Fatalf("agent path = %q, want checkpoint bytes", got)
	}
	got, err = os.ReadFile(filepath.Join(g.workspace, "user-only.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "user change after checkpoint\n" {
		t.Fatalf("rollback overwrote an unrelated user file: %q", got)
	}
}

func TestRollbackRefusesFileChangedByUserAfterAgentWrite(t *testing.T) {
	// Removing the post-write fingerprint check must make this test fail:
	// the user's bytes would be silently replaced by checkpoint content.
	r, g := newRollbackTestRegistry(t)
	commitTestFiles(t, g, map[string]string{"shared.txt": "base\n"})
	if _, err := g.Checkpoint(); err != nil {
		t.Fatal(err)
	}
	mustReadBeforeMutation(t, r, "shared.txt")
	if _, err := r.toolWrite(`{"path":"shared.txt","content":"agent change\n"}`); err != nil {
		t.Fatal(err)
	}
	userBytes := []byte("agent change\nuser appended\n")
	if err := os.WriteFile(filepath.Join(g.workspace, "shared.txt"), userBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := r.toolRollback(`{"To":1}`)
	if err == nil || !strings.Contains(err.Error(), "changed after Agent write") {
		t.Fatalf("rollback error = %v, want an explicit user-change conflict", err)
	}
	got, readErr := os.ReadFile(filepath.Join(g.workspace, "shared.txt"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != string(userBytes) {
		t.Fatalf("conflicting user bytes changed: got %q want %q", got, userBytes)
	}
}

func TestRollbackDeletesOnlySafeAgentFilesAbsentFromTarget(t *testing.T) {
	t.Run("file already contained in target is retained", func(t *testing.T) {
		r, g := newRollbackTestRegistry(t)
		commitTestFiles(t, g, map[string]string{"baseline.txt": "base\n"})
		if _, err := r.toolWrite(`{"path":"included.txt","content":"included in checkpoint\n"}`); err != nil {
			t.Fatal(err)
		}
		if _, err := g.Checkpoint(); err != nil {
			t.Fatal(err)
		}

		if _, err := r.toolRollback(`{"To":1}`); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(g.workspace, "included.txt"))
		if err != nil {
			t.Fatalf("file present in target was removed: %v", err)
		}
		if string(got) != "included in checkpoint\n" {
			t.Fatalf("included file bytes = %q", got)
		}
	})

	t.Run("unchanged file created after target is removed", func(t *testing.T) {
		r, g := newRollbackTestRegistry(t)
		commitTestFiles(t, g, map[string]string{"baseline.txt": "base\n"})
		if _, err := g.Checkpoint(); err != nil {
			t.Fatal(err)
		}
		created := filepath.Join(g.workspace, "created-after.txt")
		if _, err := r.toolWrite(`{"path":"created-after.txt","content":"agent bytes\n"}`); err != nil {
			t.Fatal(err)
		}

		if _, err := r.toolRollback(`{"To":1}`); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(created); !os.IsNotExist(err) {
			t.Fatalf("post-target Agent file still exists; stat error = %v", err)
		}
	})

	t.Run("user-modified created file is retained as conflict", func(t *testing.T) {
		r, g := newRollbackTestRegistry(t)
		commitTestFiles(t, g, map[string]string{"baseline.txt": "base\n"})
		if _, err := g.Checkpoint(); err != nil {
			t.Fatal(err)
		}
		created := filepath.Join(g.workspace, "created-after.txt")
		if _, err := r.toolWrite(`{"path":"created-after.txt","content":"agent bytes\n"}`); err != nil {
			t.Fatal(err)
		}
		userBytes := []byte("agent bytes\nuser bytes\n")
		if err := os.WriteFile(created, userBytes, 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := r.toolRollback(`{"To":1}`)
		if err == nil || !strings.Contains(err.Error(), "changed after Agent write") {
			t.Fatalf("rollback error = %v, want created-file conflict", err)
		}
		got, readErr := os.ReadFile(created)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if string(got) != string(userBytes) {
			t.Fatalf("user-modified file changed: got %q want %q", got, userBytes)
		}
	})

	t.Run("file path replaced by directory is never recursively deleted", func(t *testing.T) {
		r, g := newRollbackTestRegistry(t)
		commitTestFiles(t, g, map[string]string{"baseline.txt": "base\n"})
		if _, err := g.Checkpoint(); err != nil {
			t.Fatal(err)
		}
		claimed := filepath.Join(g.workspace, "claimed")
		if _, err := r.toolWrite(`{"path":"claimed","content":"agent file\n"}`); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(claimed); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(claimed, 0o755); err != nil {
			t.Fatal(err)
		}
		sentinel := filepath.Join(claimed, "user-sentinel.txt")
		if err := os.WriteFile(sentinel, []byte("keep\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := r.toolRollback(`{"To":1}`)
		if err == nil || !strings.Contains(err.Error(), "changed after Agent write") {
			t.Fatalf("rollback error = %v, want path-identity conflict", err)
		}
		got, readErr := os.ReadFile(sentinel)
		if readErr != nil || string(got) != "keep\n" {
			t.Fatalf("replacement directory was damaged: bytes=%q error=%v", got, readErr)
		}
	})
}

func TestRollbackLatestUsesNumericSequenceForCurrentTask(t *testing.T) {
	r, g := newRollbackTestRegistry(t)
	commitTestFiles(t, g, map[string]string{"state.txt": "baseline\n"})
	state := filepath.Join(g.workspace, "state.txt")
	for seq := 1; seq <= 12; seq++ {
		content := []byte(fmt.Sprintf("checkpoint-%d\n", seq))
		if err := os.WriteFile(state, content, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := g.Checkpoint(); err != nil {
			t.Fatal(err)
		}
	}
	mustReadBeforeMutation(t, r, "state.txt")
	if _, err := r.toolWrite(`{"path":"state.txt","content":"after latest\n"}`); err != nil {
		t.Fatal(err)
	}

	out, err := r.toolRollback(`{}`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(state)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "checkpoint-12\n" || !strings.Contains(out, "/12") {
		t.Fatalf("latest rollback selected the wrong sequence: bytes=%q output=%q", got, out)
	}
}

func TestRollbackLatestDoesNotCrossTaskBoundary(t *testing.T) {
	rA, gA := newRollbackTestRegistry(t)
	commitTestFiles(t, gA, map[string]string{"state.txt": "baseline\n"})
	state := filepath.Join(gA.workspace, "state.txt")
	if err := os.WriteFile(state, []byte("task-a-checkpoint\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := gA.Checkpoint(); err != nil {
		t.Fatal(err)
	}

	gB := &gitOps{
		gitExe: gA.gitExe, workspace: gA.workspace, mode: 1, taskID: "task-b",
		indexDir: filepath.Join(t.TempDir(), "indexes"),
	}
	if err := os.MkdirAll(gB.indexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(state, []byte("task-b-checkpoint\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := gB.Checkpoint(); err != nil {
		t.Fatal(err)
	}
	mustReadBeforeMutation(t, rA, "state.txt")
	if _, err := rA.toolWrite(`{"path":"state.txt","content":"task-a later change\n"}`); err != nil {
		t.Fatal(err)
	}

	if _, err := rA.toolRollback(`{}`); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(state)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "task-a-checkpoint\n" {
		t.Fatalf("task-a rollback selected another task: %q", got)
	}
}

func TestRollbackLatestIsNotOverriddenByLegacyNamespace(t *testing.T) {
	r, g := newRollbackTestRegistry(t)
	commitTestFiles(t, g, map[string]string{"state.txt": "baseline\n"})
	state := filepath.Join(g.workspace, "state.txt")
	var firstCommit string
	for seq := 1; seq <= 12; seq++ {
		if err := os.WriteFile(state, []byte(fmt.Sprintf("checkpoint-%d\n", seq)), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := g.Checkpoint(); err != nil {
			t.Fatal(err)
		}
		if seq == 1 {
			var err error
			firstCommit, err = g.gitRun("", "rev-parse", g.ref(1))
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	legacy := refPrefixOld + g.taskID + "/1"
	if out, err := g.gitRun("", "update-ref", legacy, firstCommit); err != nil {
		t.Fatalf("create legacy ref: %v: %s", err, out)
	}
	mustReadBeforeMutation(t, r, "state.txt")
	if _, err := r.toolWrite(`{"path":"state.txt","content":"after latest\n"}`); err != nil {
		t.Fatal(err)
	}

	out, err := r.toolRollback(`{}`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(state)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "checkpoint-12\n" || strings.Contains(out, refPrefixOld) {
		t.Fatalf("legacy namespace overrode latest primary checkpoint: bytes=%q output=%q", got, out)
	}
}

func TestCheckpointPersistsWorkspaceTaskSequenceTimeAndCommit(t *testing.T) {
	_, g := newRollbackTestRegistry(t)
	commitTestFiles(t, g, map[string]string{"state.txt": "baseline\n"})
	if _, err := g.Checkpoint(); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(checkpointMetadataTestPath(g))
	if err != nil {
		t.Fatalf("open checkpoint metadata: %v", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		t.Fatalf("checkpoint metadata is empty: %v", sc.Err())
	}
	var got struct {
		Workspace string    `json:"workspace"`
		TaskID    string    `json:"task_id"`
		Kind      string    `json:"kind"`
		Seq       int       `json:"seq"`
		CreatedAt time.Time `json:"created_at"`
		Commit    string    `json:"commit"`
		Tree      string    `json:"tree"`
		Ref       string    `json:"ref"`
	}
	if err := json.Unmarshal(sc.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Workspace != filepath.Clean(g.workspace) || got.TaskID != g.taskID || got.Kind != "model" || got.Seq != 1 {
		t.Fatalf("metadata identity = %+v", got)
	}
	if got.CreatedAt.IsZero() || got.Commit == "" || got.Tree == "" || got.Ref != g.ref(1) {
		t.Fatalf("metadata checkpoint fields = %+v", got)
	}
}

func TestCheckpointSequenceContinuesAfterProcessRestart(t *testing.T) {
	_, g1 := newRollbackTestRegistry(t)
	commitTestFiles(t, g1, map[string]string{"state.txt": "baseline\n"})
	for seq := 1; seq <= 2; seq++ {
		if err := os.WriteFile(filepath.Join(g1.workspace, "state.txt"), []byte(fmt.Sprintf("checkpoint-%d\n", seq)), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := g1.Checkpoint(); err != nil {
			t.Fatal(err)
		}
	}
	g2 := &gitOps{
		gitExe: g1.gitExe, workspace: g1.workspace, mode: g1.mode,
		taskID: g1.taskID, indexDir: g1.indexDir,
	}
	if err := os.WriteFile(filepath.Join(g2.workspace, "state.txt"), []byte("checkpoint-3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := g2.Checkpoint()
	if err != nil {
		t.Fatal(err)
	}
	if g2.seq != 3 || !strings.Contains(out, g2.taskID+"/3") {
		t.Fatalf("restarted checkpoint did not continue at 3: seq=%d output=%q", g2.seq, out)
	}
}

func TestRollbackAnnouncesSequenceTimeAndCommitBeforeRestore(t *testing.T) {
	r, g := newRollbackTestRegistry(t)
	commitTestFiles(t, g, map[string]string{"state.txt": "baseline\n"})
	if _, err := g.Checkpoint(); err != nil {
		t.Fatal(err)
	}
	commit, err := g.gitRun("", "rev-parse", g.ref(1))
	if err != nil {
		t.Fatal(err)
	}
	mustReadBeforeMutation(t, r, "state.txt")
	if _, err := r.toolWrite(`{"path":"state.txt","content":"agent change\n"}`); err != nil {
		t.Fatal(err)
	}
	var rollbackErr error
	stdout := captureTestOutput(t, func() {
		_, rollbackErr = r.toolRollback(`{"To":1}`)
	})
	if rollbackErr != nil {
		t.Fatal(rollbackErr)
	}
	if !strings.Contains(stdout, "checkpoint task-a/1") || !strings.Contains(stdout, short(commit)) || !strings.Contains(stdout, "created_at=") {
		t.Fatalf("rollback target announcement missing identity fields: %q", stdout)
	}
}

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

func writeResumeFixture(t *testing.T, path, workspace, taskID, manifestPath string, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	meta := map[string]string{
		"role": "_meta", "workspace": workspace, "task_id": taskID,
		"manifest_path": manifestPath,
	}
	b, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := json.Marshal(openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(append(b, '\n'), append(msg, '\n')...), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func TestResolveResumeLatestFiltersCurrentWorkspace(t *testing.T) {
	exeDir := t.TempDir()
	sessions := filepath.Join(exeDir, "data", "sessions")
	workspaceA := filepath.Join(t.TempDir(), "workspace-a")
	workspaceB := filepath.Join(t.TempDir(), "workspace-b")
	pathA := filepath.Join(sessions, "sess-task-a.jsonl")
	pathB := filepath.Join(sessions, "sess-task-b.jsonl")
	writeResumeFixture(t, pathA, workspaceA, "task-a", filepath.Join(sessions, "manifest-task-a.jsonl"), time.Now().Add(-time.Hour))
	writeResumeFixture(t, pathB, workspaceB, "task-b", filepath.Join(sessions, "manifest-task-b.jsonl"), time.Now())
	cfg := &config{exeDir: exeDir, workspace: workspaceA}

	got := resolveResume(cfg, "latest")
	if !sameResolvedPath(got, pathA) {
		t.Fatalf("latest for workspace A = %q, want %q", got, pathA)
	}
}

func TestSessionIDOmitsJSONLExtension(t *testing.T) {
	s := newSession(filepath.Join(t.TempDir(), "sess-task-123.jsonl"), t.TempDir())
	if got := s.id(); got != "task-123" {
		t.Fatalf("session id = %q, want task-123", got)
	}
}

func TestSessionMetadataPersistsTaskAndManifestIdentity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sess-task-123.jsonl")
	workspace := t.TempDir()
	s := newSession(path, workspace)
	s.record(openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "hello"})
	s.Close()

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		t.Fatalf("missing session metadata: %v", sc.Err())
	}
	var meta struct {
		Workspace    string `json:"workspace"`
		TaskID       string `json:"task_id"`
		ManifestPath string `json:"manifest_path"`
	}
	if err := json.Unmarshal(sc.Bytes(), &meta); err != nil {
		t.Fatal(err)
	}
	wantManifest := filepath.Join(dir, "manifest-task-123.jsonl")
	if meta.Workspace != workspace || meta.TaskID != "task-123" || !sameResolvedPath(meta.ManifestPath, wantManifest) {
		t.Fatalf("session metadata = %+v, want workspace=%q task=task-123 manifest=%q", meta, workspace, wantManifest)
	}
}

func TestPrepareResumeRejectsWorkspaceMismatchWithoutMigration(t *testing.T) {
	exeDir := t.TempDir()
	sessions := filepath.Join(exeDir, "data", "sessions")
	originalWorkspace := filepath.Join(t.TempDir(), "original")
	currentWorkspace := filepath.Join(t.TempDir(), "current")
	manifest := filepath.Join(sessions, "manifest-task-a.jsonl")
	target := filepath.Join(sessions, "sess-task-a.jsonl")
	writeResumeFixture(t, target, originalWorkspace, "task-a", manifest, time.Now())
	cfg := &config{exeDir: exeDir, workspace: currentWorkspace, resumePath: target}

	_, err := prepareResume(cfg, "new-task")
	if err == nil || !strings.Contains(err.Error(), "workspace mismatch") ||
		!strings.Contains(err.Error(), originalWorkspace) || !strings.Contains(err.Error(), currentWorkspace) {
		t.Fatalf("prepareResume error = %v, want explicit workspace mismatch", err)
	}
}

func TestPrepareResumeSameWorkspaceBindsOriginalIdentity(t *testing.T) {
	exeDir := t.TempDir()
	sessions := filepath.Join(exeDir, "data", "sessions")
	workspace := filepath.Join(t.TempDir(), "workspace")
	manifest := filepath.Join(sessions, "manifest-task-a.jsonl")
	target := filepath.Join(sessions, "sess-task-a.jsonl")
	writeResumeFixture(t, target, workspace, "task-a", manifest, time.Now())
	cfg := &config{exeDir: exeDir, workspace: workspace, resumePath: target}

	got, err := prepareResume(cfg, "new-task")
	if err != nil {
		t.Fatal(err)
	}
	if !sameResolvedPath(got.Workspace, workspace) || got.TaskID != "task-a" ||
		!sameResolvedPath(got.ManifestPath, manifest) || !sameResolvedPath(got.SessionPath, target) || got.Migrated {
		t.Fatalf("same-workspace resume identity = %+v", got)
	}
}

func TestPrepareResumeMigrationUsesNewIdentityAndDoesNotAppendOldSession(t *testing.T) {
	exeDir := t.TempDir()
	sessions := filepath.Join(exeDir, "data", "sessions")
	originalWorkspace := filepath.Join(t.TempDir(), "original")
	currentWorkspace := filepath.Join(t.TempDir(), "current")
	target := filepath.Join(sessions, "sess-task-a.jsonl")
	writeResumeFixture(t, target, originalWorkspace, "task-a", filepath.Join(sessions, "manifest-task-a.jsonl"), time.Now())
	cfg := &config{
		exeDir: exeDir, workspace: currentWorkspace, resumePath: target,
		migrateResumeWorkspace: true,
	}

	got, err := prepareResume(cfg, "new-task")
	if err != nil {
		t.Fatal(err)
	}
	wantManifest := filepath.Join(sessions, "manifest-new-task.jsonl")
	if !sameResolvedPath(got.Workspace, currentWorkspace) || got.TaskID != "new-task" ||
		!sameResolvedPath(got.ManifestPath, wantManifest) || got.SessionPath != "" || !got.Migrated ||
		!sameResolvedPath(got.ResumeTarget, target) {
		t.Fatalf("migrated resume identity = %+v", got)
	}
}

func TestLoadPreparedMessagesMigrationCopiesHistoryWithoutChangingSource(t *testing.T) {
	exeDir := t.TempDir()
	sessions := filepath.Join(exeDir, "data", "sessions")
	target := filepath.Join(sessions, "sess-task-a.jsonl")
	writeResumeFixture(t, target, t.TempDir(), "task-a", filepath.Join(sessions, "manifest-task-a.jsonl"), time.Now())
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config{
		exeDir: exeDir, workspace: t.TempDir(), resumePath: target,
		migrateResumeWorkspace: true,
	}
	plan, err := prepareResume(cfg, "task-b")
	if err != nil {
		t.Fatal(err)
	}
	applyResumePreparation(cfg, plan)
	destination := openSessionFor(cfg, plan.TaskID)
	msgs, err := loadPreparedMessages(plan, destination)
	if err != nil {
		t.Fatal(err)
	}
	destination.Close()
	if len(msgs) != 1 || msgs[0].Content != "fixture" {
		t.Fatalf("migrated messages = %#v", msgs)
	}
	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("migration appended to or changed the source session")
	}
	wantDestination := filepath.Join(sessions, "sess-task-b.jsonl")
	if sameResolvedPath(target, destination.file()) || !sameResolvedPath(destination.file(), wantDestination) {
		t.Fatalf("migration destination = %q, want new session %q", destination.file(), wantDestination)
	}
	copied, err := loadSession(wantDestination)
	if err != nil {
		t.Fatal(err)
	}
	if len(copied) != 1 || copied[0].Content != "fixture" {
		t.Fatalf("new session history = %#v", copied)
	}
}

func TestPrepareResumeLegacyMetadataDerivesTaskAndManifestIdentity(t *testing.T) {
	exeDir := t.TempDir()
	sessions := filepath.Join(exeDir, "data", "sessions")
	target := filepath.Join(sessions, "sess-legacy-task.jsonl")
	workspace := t.TempDir()
	legacy := []byte(`{"role":"_meta","workspace":` + mustJSONQuote(t, workspace) + `}` + "\n" +
		`{"role":"user","content":"fixture"}` + "\n")
	if err := os.MkdirAll(sessions, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, legacy, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config{exeDir: exeDir, workspace: workspace, resumePath: target}

	got, err := prepareResume(cfg, "new-task")
	if err != nil {
		t.Fatal(err)
	}
	wantManifest := filepath.Join(sessions, "manifest-legacy-task.jsonl")
	if got.TaskID != "legacy-task" || !sameResolvedPath(got.Workspace, workspace) ||
		!sameResolvedPath(got.ManifestPath, wantManifest) || !sameResolvedPath(got.SessionPath, target) {
		t.Fatalf("legacy resume identity = %+v", got)
	}
}

func TestPrepareResumeRejectsTaskIdentityMismatch(t *testing.T) {
	exeDir := t.TempDir()
	sessions := filepath.Join(exeDir, "data", "sessions")
	target := filepath.Join(sessions, "sess-task-a.jsonl")
	writeResumeFixture(t, target, t.TempDir(), "different-task", filepath.Join(sessions, "manifest-different-task.jsonl"), time.Now())
	cfg := &config{exeDir: exeDir, workspace: t.TempDir(), resumePath: target}

	_, err := prepareResume(cfg, "new-task")
	if err == nil || !strings.Contains(err.Error(), "task identity mismatch") {
		t.Fatalf("prepareResume error = %v, want task identity mismatch", err)
	}
}

func mustJSONQuote(t *testing.T, value string) string {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func requireF4Allowed(t *testing.T, name string, call func() (string, error)) {
	t.Helper()
	got, err := call()
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if got == "" {
		t.Fatalf("%s returned empty result", name)
	}
}
func requireF4UnknownImpact(t *testing.T, name string, call func() (string, error)) {
	t.Helper()
	_, err := call()
	if err == nil || !strings.Contains(err.Error(), "hard_link_impact_unknown") || !strings.Contains(err.Error(), "无法确定") || strings.Contains(err.Error(), "outside workspace") {
		t.Fatalf("%s: %v", name, err)
	}
}
func f4WriteEdit(t *testing.T, r *Registry, path string) {
	t.Helper()
	if got := r.Execute("write", mustJSON(t, map[string]string{"path": path, "content": "before"})); strings.HasPrefix(got, "error:") {
		t.Fatal(got)
	}
	mustReadBeforeMutation(t, r, path)
	if got := r.Execute("edit", mustJSON(t, map[string]string{"path": path, "old_string": "before", "new_string": "after"})); strings.HasPrefix(got, "error:") {
		t.Fatal(got)
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "after" {
		t.Fatalf("content=%q err=%v", b, err)
	}
}
func assertF4Visibility(t *testing.T, r *Registry, protocol string, human *bytes.Buffer, path string, count int) {
	t.Helper()
	seen := 0
	for _, e := range parseEventLines(t, protocol) {
		if e.Type == "outside_workspace_write" {
			d := e.Data.(map[string]interface{})
			if d["resolvedPath"] == path {
				seen++
				if d["checkpointCovered"] != false || d["rollbackCovered"] != false {
					t.Fatal("false checkpoint coverage")
				}
			}
		}
	}
	if seen != count {
		t.Fatalf("events=%d want=%d for %s: %s", seen, count, path, protocol)
	}
	b, err := os.ReadFile(r.auditPath)
	if err != nil {
		t.Fatal(err)
	}
	seen = 0
	for _, line := range bytes.Split(bytes.TrimSpace(b), []byte("\n")) {
		var d map[string]interface{}
		if err := json.Unmarshal(line, &d); err != nil {
			t.Fatal(err)
		}
		if d["event"] == "outside_workspace_write" && d["resolvedPath"] == path {
			seen++
			if d["checkpointCovered"] != false || d["rollbackCovered"] != false {
				t.Fatal("audit coverage")
			}
		}
	}
	if seen != count {
		t.Fatalf("audit count=%d for %s: %s", seen, path, b)
	}
	human.Reset()
	// Actual terminal rendering from persisted audit, with no in-memory write list.
	printTaskEnd(&Registry{auditPath: r.auditPath, taskID: r.taskID}, false, taskEndState{status: "已完成"})
	if !strings.Contains(human.String(), path) || !strings.Contains(human.String(), "不在 checkpoint 覆盖范围内") || !strings.Contains(human.String(), "无法通过 rollback 回退") {
		t.Fatalf("summary missing: %s", human.String())
	}
	t.Logf("three surfaces path=%s\nevents=%s\naudit=%s\nsummary=%s", path, protocol, b, human.String())
}
func TestF4DefaultOpenOutsideWriteVisible(t *testing.T) {
	r, _, ws, _ := newPermissionTestRegistry(t, "open", nil, "")
	p, h := captureEvents(t)
	path := filepath.Join(filepath.Dir(ws), "outside.txt")
	f4WriteEdit(t, r, path)
	assertF4Visibility(t, r, p.String(), h, path, 2)
}
func TestF4ExplicitDenyStillBlocksOutsideWrites(t *testing.T) {
	r, _, ws, _ := newPermissionTestRegistry(t, "open", []permissionRule{{Tool: "*", Pattern: "*outside.txt", Action: "allow"}, {Tool: "write", Pattern: "*outside.txt", Action: "deny"}, {Tool: "edit", Pattern: "*outside.txt", Action: "deny"}}, "")
	path := filepath.Join(filepath.Dir(ws), "outside.txt")
	if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"write", "edit"} {
		got := r.Execute(tool, mustJSON(t, map[string]string{"path": path, "content": "new", "old_string": "old", "new_string": "new"}))
		if !strings.Contains(got, "denied by permission rule") {
			t.Fatal(got)
		}
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "old" {
		t.Fatalf("denied file changed: %s %v", b, err)
	}
}
func TestF4StrictAndStandardAskForOutsideWrite(t *testing.T) {
	for _, profile := range []string{"strict", "standard"} {
		t.Run(profile, func(t *testing.T) {
			r, _, ws, _ := newPermissionTestRegistry(t, profile, nil, "n\n")
			p, _ := captureEvents(t)
			path := filepath.Join(filepath.Dir(ws), "outside.txt")
			got := r.Execute("write", mustJSON(t, map[string]string{"path": path, "content": "new"}))
			if !strings.Contains(got, "denied by user") {
				t.Fatal(got)
			}
			if !strings.Contains(p.String(), "permission_request") {
				t.Fatal("no ask")
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("denied file created")
			}
		})
	}
}
func TestF4JunctionExecuteAndIndirectGitProtection(t *testing.T) {
	r, _, ws, _ := newPermissionTestRegistry(t, "open", nil, "")
	outside := filepath.Join(filepath.Dir(ws), "external")
	if err := os.MkdirAll(outside, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(ws, "link")
	if out, err := exec.Command("cmd.exe", "/c", "mklink", "/J", link, outside).CombinedOutput(); err != nil {
		t.Fatalf("junction: %v %s", err, out)
	}
	r.autoCheckpointDone = false // real Execute must not try to checkpoint an external write.
	p, h := captureEvents(t)
	path := filepath.Join(link, "created.txt")
	f4WriteEdit(t, r, path)
	assertF4Visibility(t, r, p.String(), h, filepath.Join(outside, "created.txt"), 2)
	if _, err := os.Stat(r.man.path); !os.IsNotExist(err) {
		t.Fatalf("external write entered rollback manifest: %v", err)
	}
	gitDir := filepath.Join(outside, ".git")
	if err := os.MkdirAll(gitDir, 0700); err != nil {
		t.Fatal(err)
	}
	protected := filepath.Join(gitDir, "config")
	if err := os.WriteFile(protected, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	gitLink := filepath.Join(ws, "metadata")
	if out, err := exec.Command("cmd.exe", "/c", "mklink", "/J", gitLink, gitDir).CombinedOutput(); err != nil {
		t.Fatalf("junction: %v %s", err, out)
	}
	hard := filepath.Join(ws, "alias.txt")
	if err := os.Link(protected, hard); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{filepath.Join(gitLink, "config"), hard} {
		for _, tool := range []string{"write", "edit"} {
			got := r.Execute(tool, mustJSON(t, map[string]string{"path": target, "content": "changed", "old_string": "original", "new_string": "changed"}))
			if !strings.HasPrefix(got, "error:") {
				t.Fatalf("indirect git accepted: %s", got)
			}
		}
	}
	b, err := os.ReadFile(protected)
	if err != nil || string(b) != "original" {
		t.Fatalf("git mutated: %q %v", b, err)
	}
}
func TestF4ResolvedPathAndReadContent(t *testing.T) {
	r, ws, outside := newBoundaryTestRegistry(t)
	path := filepath.Join(outside, "source.txt")
	if err := os.WriteFile(path, []byte("EXTERNAL_CONTENT"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(ws, "link")
	if out, err := exec.Command("cmd.exe", "/c", "mklink", "/J", link, outside).CombinedOutput(); err != nil {
		t.Fatalf("junction: %v %s", err, out)
	}
	hard := filepath.Join(ws, "hard.txt")
	if err := os.Link(path, hard); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{path, filepath.Join(link, "source.txt"), "../outside/source.txt", hard} {
		got, err := r.toolRead(mustJSON(t, map[string]string{"path": target}))
		if err != nil || !strings.Contains(got, "EXTERNAL_CONTENT") {
			t.Fatalf("read %s: %q %v", target, got, err)
		}
	}
	for _, target := range []string{path, filepath.Join(link, "source.txt"), "../outside/source.txt"} {
		got, err := r.absPath(target)
		if err != nil || !sameResolvedPath(got, path) {
			t.Fatalf("resolve %s=%s %v", target, got, err)
		}
	}
	for _, tool := range []struct {
		name string
		call func(string) (string, error)
	}{{"ls", r.toolLs}, {"tree", r.toolTree}, {"grep", r.toolGrep}, {"glob", r.toolGlob}} {
		args := mustJSON(t, map[string]string{"path": outside, "pattern": "EXTERNAL_CONTENT"})
		if tool.name == "glob" {
			args = mustJSON(t, map[string]string{"pattern": filepath.Join(outside, "*.txt")})
		}
		got, err := tool.call(args)
		if err != nil || !strings.Contains(got, "source.txt") {
			t.Fatalf("%s: %q %v", tool.name, got, err)
		}
	}
}
func TestF4HardLinkExecuteReportsCapabilityBeforeCheckpoint(t *testing.T) {
	r, _, ws, _ := newPermissionTestRegistry(t, "open", nil, "")
	r.autoCheckpointDone = false
	original := filepath.Join(filepath.Dir(ws), "outside.txt")
	if err := os.WriteFile(original, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(ws, "alias.txt")
	if err := os.Link(original, alias); err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"write", "edit"} {
		got := r.Execute(tool, mustJSON(t, map[string]string{"path": alias, "content": "new", "old_string": "old", "new_string": "new"}))
		if !strings.Contains(got, "hard_link_impact_unknown") {
			t.Fatalf("wrong capability error: %s", got)
		}
	}
}

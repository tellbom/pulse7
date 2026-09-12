package main

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type boundaryTestRunner struct{ ran bool }

func (r *boundaryTestRunner) Run(string) (string, int, error) { r.ran = true; return "", 0, nil }
func (r *boundaryTestRunner) Mode() string                    { return "test" }
func (r *boundaryTestRunner) Interrupt()                      {}

func newBoundaryTestRegistry(t *testing.T) (*Registry, string, string) {
	t.Helper()
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	r := &Registry{
		policy: &Policy{Workspace: workspace}, workspace: workspace,
		man: &manifest{path: filepath.Join(root, "manifest.jsonl")},
	}
	return r, workspace, outside
}

func requireBoundaryDenied(t *testing.T, name string, call func() (string, error)) {
	t.Helper()
	_, err := call()
	if err == nil {
		t.Fatalf("%s unexpectedly crossed the workspace boundary", name)
	}
	lower := strings.ToLower(err.Error())
	if !strings.Contains(lower, "outside workspace") && !strings.Contains(lower, "reparse") && !strings.Contains(lower, "hard link") && !strings.Contains(lower, "protected") {
		t.Fatalf("%s error does not identify a boundary denial: %v", name, err)
	}
}

func TestAllFileToolsAllowResolvedOutsideTraversalAndJunction(t *testing.T) {
	r, workspace, outside := newBoundaryTestRegistry(t)
	r.auditPath = filepath.Join(filepath.Dir(workspace), "audit.jsonl")
	protocol, human := captureEvents(t)
	sentinel := filepath.Join(outside, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("outside sentinel\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(workspace, "link")
	if out, err := exec.Command("cmd.exe", "/c", "mklink", "/J", link, outside).CombinedOutput(); err != nil {
		t.Fatalf("create junction fixture: %v: %s", err, out)
	}

	requireF4Allowed(t, "read junction", func() (string, error) {
		return r.toolRead(`{"path":"link/sentinel.txt"}`)
	})
	requireF4Allowed(t, "ls junction", func() (string, error) {
		return r.toolLs(`{"path":"link"}`)
	})
	requireF4Allowed(t, "tree junction", func() (string, error) {
		return r.toolTree(`{"path":"link"}`)
	})
	requireF4Allowed(t, "tree nested junction", func() (string, error) {
		return r.toolTree(`{}`)
	})
	requireF4Allowed(t, "grep junction", func() (string, error) {
		return r.toolGrep(`{"pattern":"sentinel","path":"link"}`)
	})
	requireF4Allowed(t, "grep nested junction", func() (string, error) {
		return r.toolGrep(`{"pattern":"sentinel"}`)
	})
	requireF4Allowed(t, "glob traversal", func() (string, error) {
		return r.toolGlob(`{"pattern":"../outside/*"}`)
	})
	requireF4Allowed(t, "glob junction", func() (string, error) {
		return r.toolGlob(`{"pattern":"link/*"}`)
	})
	requireF4Allowed(t, "glob wildcard junction", func() (string, error) {
		return r.toolGlob(`{"pattern":"*/*"}`)
	})
	requireF4Allowed(t, "write junction", func() (string, error) {
		return r.toolWrite(`{"path":"link/new.txt","content":"external created"}`)
	})
	requireF4Allowed(t, "edit junction", func() (string, error) {
		return r.toolEdit(`{"path":"link/sentinel.txt","old_string":"outside","new_string":"changed"}`)
	})
	if data, err := os.ReadFile(filepath.Join(outside, "new.txt")); err != nil || string(data) != "external created" {
		t.Fatalf("write created an outside file: %v", err)
	}
	got, err := os.ReadFile(sentinel)
	if err != nil || string(got) != "changed sentinel\n" {
		t.Fatalf("outside sentinel changed: bytes=%q error=%v", got, err)
	}
	assertF4Visibility(t, r, protocol.String(), human, filepath.Join(outside, "new.txt"), 1)
	assertF4Visibility(t, r, protocol.String(), human, sentinel, 1)
}

func TestCheckpointRejectsWorkspaceJunctionEscape(t *testing.T) {
	r, g := newRollbackTestRegistry(t)
	commitTestFiles(t, g, map[string]string{"state.txt": "baseline\n"})
	outside := filepath.Join(filepath.Dir(g.workspace), "checkpoint-outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "sentinel.txt"), []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(g.workspace, "outside-link")
	if out, err := exec.Command("cmd.exe", "/c", "mklink", "/J", link, outside).CombinedOutput(); err != nil {
		t.Fatalf("create junction fixture: %v: %s", err, out)
	}

	_, err := r.toolCheckpoint(`{}`)
	if err == nil || (!strings.Contains(strings.ToLower(err.Error()), "outside workspace") && !strings.Contains(strings.ToLower(err.Error()), "reparse")) {
		t.Fatalf("checkpoint error = %v, want final-path boundary denial", err)
	}
}

func TestHardLinkReadAllowedAndWritesRejectUnknownImpact(t *testing.T) {
	r, workspace, outside := newBoundaryTestRegistry(t)
	original := filepath.Join(outside, "original.txt")
	if err := os.WriteFile(original, []byte("outside bytes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(workspace, "alias.txt")
	if err := os.Link(original, alias); err != nil {
		t.Fatalf("create hard-link fixture: %v", err)
	}

	requireF4Allowed(t, "read hard link", func() (string, error) {
		return r.toolRead(`{"path":"alias.txt"}`)
	})
	requireF4UnknownImpact(t, "write hard link", func() (string, error) {
		return r.toolWrite(`{"path":"alias.txt","content":"changed"}`)
	})
	requireF4UnknownImpact(t, "edit hard link", func() (string, error) {
		return r.toolEdit(`{"path":"alias.txt","old_string":"outside","new_string":"changed"}`)
	})
	got, err := os.ReadFile(original)
	if err != nil || string(got) != "outside bytes\n" {
		t.Fatalf("outside hard-link target changed: bytes=%q error=%v", got, err)
	}
}

func TestAllFileToolsProtectGitMetadata(t *testing.T) {
	r, workspace, _ := newBoundaryTestRegistry(t)
	gitDir := filepath.Join(workspace, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	checks := []struct {
		name string
		call func() (string, error)
	}{
		{"read", func() (string, error) { return r.toolRead(`{"path":".git/config"}`) }},
		{"ls", func() (string, error) { return r.toolLs(`{"path":".git"}`) }},
		{"tree", func() (string, error) { return r.toolTree(`{"path":".git"}`) }},
		{"grep", func() (string, error) { return r.toolGrep(`{"pattern":"secret","path":".git"}`) }},
		{"glob", func() (string, error) { return r.toolGlob(`{"pattern":".git/*"}`) }},
		{"write", func() (string, error) { return r.toolWrite(`{"path":".git/config","content":"changed"}`) }},
		{"edit", func() (string, error) {
			return r.toolEdit(`{"path":".git/config","old_string":"secret","new_string":"changed"}`)
		}},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) { requireBoundaryDenied(t, check.name, check.call) })
	}
}

func TestReadOnlyModeDeniesEveryPotentiallyMutatingTool(t *testing.T) {
	r, workspace, _ := newBoundaryTestRegistry(t)
	r.readOnly = true
	r.yolo = true
	r.runner = &boundaryTestRunner{}
	if err := os.WriteFile(filepath.Join(workspace, "existing.txt"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	checks := []struct {
		name string
		call func() (string, error)
	}{
		{"write", func() (string, error) { return r.toolWrite(`{"path":"new.txt","content":"new"}`) }},
		{"edit", func() (string, error) {
			return r.toolEdit(`{"path":"existing.txt","old_string":"old","new_string":"new"}`)
		}},
		{"rollback", func() (string, error) { return r.toolRollback(`{}`) }},
		{"shell", func() (string, error) { return r.toolShell(`{"command":"echo mutation"}`) }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			_, err := check.call()
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), "read-only") {
				t.Fatalf("%s error = %v, want read-only denial", check.name, err)
			}
		})
	}
	if got, err := os.ReadFile(filepath.Join(workspace, "existing.txt")); err != nil || string(got) != "old\n" {
		t.Fatalf("read-only edit changed file: bytes=%q error=%v", got, err)
	}
	if _, err := os.Lstat(filepath.Join(workspace, "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("read-only write created file: %v", err)
	}
	if r.runner.(*boundaryTestRunner).ran {
		t.Fatal("read-only shell reached the runner")
	}
}

func TestApplyConfigEnablesReadOnlyUnlessFlagOverridesIt(t *testing.T) {
	var cfg config
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.BoolVar(&cfg.readOnly, "read-only", false, "")
	ac := defaultAgentConfig()
	ac.ReadOnly = true
	applyConfigToFlags(&cfg, ac, fs)
	if !cfg.readOnly {
		t.Fatal("read_only from agent.json was not applied")
	}

	var overridden config
	overrideFS := flag.NewFlagSet("test-override", flag.ContinueOnError)
	overrideFS.BoolVar(&overridden.readOnly, "read-only", true, "")
	if err := overrideFS.Parse([]string{"--read-only=false"}); err != nil {
		t.Fatal(err)
	}
	applyConfigToFlags(&overridden, ac, overrideFS)
	if overridden.readOnly {
		t.Fatal("explicit --read-only=false did not override agent.json")
	}
}

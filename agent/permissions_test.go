package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newPermissionTestRegistry(t *testing.T, profile string, rules []permissionRule, input string) (*Registry, *boundaryTestRunner, string, string) {
	t.Helper()
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &boundaryTestRunner{}
	auditPath := filepath.Join(root, "audit.jsonl")
	r := NewRegistry(
		&Policy{Workspace: workspace}, runner, auditPath, filepath.Join(root, "manifest.jsonl"),
		false, false, false, strings.NewReader(input), root, workspace, "permission-test",
		permissionConfig{Profile: profile, Rules: rules},
	)
	// These tests isolate permission behavior; automatic-checkpoint behavior
	// has its own integration suite backed by a real temporary Git repository.
	r.autoCheckpointDone = true
	return r, runner, workspace, auditPath
}

func TestStrictProfileAsksBeforeModelWrite(t *testing.T) {
	r, _, workspace, _ := newPermissionTestRegistry(t, "strict", nil, "n\n")
	got := r.Execute("write", `{"path":"denied.txt","content":"no"}`)
	if !strings.Contains(got, "denied by user") {
		t.Fatalf("Execute(write) = %q, want user denial", got)
	}
	if _, err := os.Stat(filepath.Join(workspace, "denied.txt")); !os.IsNotExist(err) {
		t.Fatalf("strict denial still created file: %v", err)
	}
}

func TestStandardProfileAutoAllowsWorkspaceWriteAndAuditsIt(t *testing.T) {
	r, _, workspace, auditPath := newPermissionTestRegistry(t, "standard", nil, "")
	var visible bytes.Buffer
	oldOut := teeOut
	teeOut = &visible
	t.Cleanup(func() { teeOut = oldOut })

	got := r.Execute("write", `{"path":"allowed.txt","content":"yes"}`)
	if strings.HasPrefix(got, "error:") {
		t.Fatalf("Execute(write) = %q", got)
	}
	if b, err := os.ReadFile(filepath.Join(workspace, "allowed.txt")); err != nil || string(b) != "yes" {
		t.Fatalf("auto-allowed write bytes=%q err=%v", b, err)
	}
	if !strings.Contains(visible.String(), "[permission] AUTO-ALLOWED") || !strings.Contains(visible.String(), "write") {
		t.Fatalf("automatic decision was not visible: %q", visible.String())
	}
	entries := readPermissionAudit(t, auditPath)
	if len(entries) == 0 || entries[0]["event"] != "permission" || entries[0]["decision"] != "allow" {
		t.Fatalf("permission audit entry missing: %#v", entries)
	}
}

func TestStandardProfileStillAsksForShell(t *testing.T) {
	r, runner, _, _ := newPermissionTestRegistry(t, "standard", nil, "n\n")
	got := r.Execute("shell", `{"command":"echo should-not-run"}`)
	if !strings.Contains(got, "denied by user") {
		t.Fatalf("Execute(shell) = %q, want user denial", got)
	}
	if runner.ran {
		t.Fatal("standard shell denial reached runner")
	}
}

func TestExplicitAllowOverridesStrictAsk(t *testing.T) {
	r, runner, _, _ := newPermissionTestRegistry(t, "strict", []permissionRule{
		{Tool: "shell", Pattern: "go test *", Action: "allow"},
	}, "")
	got := r.Execute("shell", `{"command":"go test ./..."}`)
	if strings.HasPrefix(got, "error:") {
		t.Fatalf("Execute(shell) = %q", got)
	}
	if !runner.ran {
		t.Fatal("matching allow rule did not reach runner")
	}
}

func TestDenyRuleWinsInOpenProfileAndOverAllow(t *testing.T) {
	r, runner, workspace, _ := newPermissionTestRegistry(t, "open", []permissionRule{
		{Tool: "shell", Pattern: "*blocked*", Action: "allow"},
		{Tool: "shell", Pattern: "*blocked*", Action: "deny"},
		{Tool: "write", Pattern: "*/secret.txt", Action: "deny"},
	}, "")
	got := r.Execute("shell", `{"command":"echo blocked"}`)
	if !strings.Contains(got, "denied by permission rule") {
		t.Fatalf("Execute(shell) = %q, want rule denial", got)
	}
	if runner.ran {
		t.Fatal("denied model shell reached runner")
	}
	got = r.Execute("write", `{"path":"secret.txt","content":"no"}`)
	if !strings.Contains(got, "denied by permission rule") {
		t.Fatalf("Execute(write) = %q, want rule denial", got)
	}
	if _, err := os.Stat(filepath.Join(workspace, "secret.txt")); !os.IsNotExist(err) {
		t.Fatalf("denied model write created file: %v", err)
	}
}

func TestLoadPermissionConfigRejectsInvalidInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permissions.json")
	if err := os.WriteFile(path, []byte(`{"profile":"open","rules":[{"tool":"shell","pattern":"*","action":"maybe"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPermissionConfig(path); err == nil || !strings.Contains(err.Error(), "action") {
		t.Fatalf("loadPermissionConfig error = %v, want invalid action", err)
	}
}

func TestDefaultPermissionProfileIsOpen(t *testing.T) {
	cfg := defaultPermissionConfig()
	if cfg.Profile != "open" {
		t.Fatalf("default profile = %q, want open", cfg.Profile)
	}

	loaded, err := loadPermissionConfig(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Profile != "open" {
		t.Fatalf("missing config profile = %q, want open", loaded.Profile)
	}

	templatePath := filepath.Join(t.TempDir(), "permissions.json")
	if err := writePermissionConfigTemplate(templatePath); err != nil {
		t.Fatal(err)
	}
	template, err := loadPermissionConfig(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	if template.Profile != "open" {
		t.Fatalf("generated template profile = %q, want open", template.Profile)
	}
}

func TestDefaultProfileAutoAllowsWriteEditAndShellWithAudit(t *testing.T) {
	r, runner, workspace, auditPath := newPermissionTestRegistry(t, defaultPermissionConfig().Profile, nil, "")

	if got := r.Execute("write", `{"path":"state.txt","content":"before"}`); strings.HasPrefix(got, "error:") {
		t.Fatalf("default write = %q", got)
	}
	mustReadBeforeMutation(t, r, "state.txt")
	if got := r.Execute("edit", `{"path":"state.txt","old_string":"before","new_string":"after"}`); strings.HasPrefix(got, "error:") {
		t.Fatalf("default edit = %q", got)
	}
	if got := r.Execute("shell", `{"command":"echo allowed"}`); strings.HasPrefix(got, "error:") {
		t.Fatalf("default shell = %q", got)
	}
	if !runner.ran {
		t.Fatal("default shell did not reach runner")
	}
	if b, err := os.ReadFile(filepath.Join(workspace, "state.txt")); err != nil || string(b) != "after" {
		t.Fatalf("default mutation bytes=%q err=%v", b, err)
	}

	entries := readPermissionAudit(t, auditPath)
	for _, tool := range []string{"write", "edit", "shell"} {
		found := false
		for _, entry := range entries {
			if entry["tool"] == tool && entry["decision"] == "allow" && entry["source"] == "preset:open" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing default auto-allow audit for %s: %#v", tool, entries)
		}
	}
}

func TestOutsideWritesAllowRulesAndGitProtection(t *testing.T) {
	for _, profile := range []string{"strict", "standard", "open"} {

		t.Run(profile+"/outside", func(t *testing.T) {
			r, _, workspace, _ := newPermissionTestRegistry(t, profile, []permissionRule{{Tool: "write", Pattern: "*", Action: "allow"}, {Tool: "edit", Pattern: "*", Action: "allow"}}, "")
			protocol, human := captureEvents(t)
			path := filepath.Join(filepath.Dir(workspace), "outside.txt")
			f4WriteEdit(t, r, path)
			assertF4Visibility(t, r, protocol.String(), human, path, 2)
		})

		t.Run(profile+"/git", func(t *testing.T) {
			r, _, workspace, auditPath := newPermissionTestRegistry(t, profile, []permissionRule{
				{Tool: "write", Pattern: "*", Action: "allow"},
				{Tool: "edit", Pattern: "*", Action: "allow"},
			}, "y\n")
			for tool, args := range map[string]string{
				"write": `{"path":".git/config","content":"no"}`,
				"edit":  `{"path":".git/config","old_string":"old","new_string":"new"}`,
			} {
				got := r.Execute(tool, args)
				if !strings.Contains(got, "hard boundary") || !strings.Contains(got, "protected .git") {
					t.Fatalf(".git %s = %q, want hard-boundary denial", tool, got)
				}
			}
			if _, err := os.Stat(filepath.Join(workspace, ".git", "config")); !os.IsNotExist(err) {
				t.Fatalf(".git write created file: %v", err)
			}
			entries := readPermissionAudit(t, auditPath)
			if len(entries) != 2 {
				t.Fatalf(".git denial audit = %#v", entries)
			}
			for _, entry := range entries {
				if entry["decision"] != "deny" || entry["source"] != "hard-boundary:git-write" {
					t.Fatalf(".git denial audit = %#v", entries)
				}
			}
		})
	}
}

func readPermissionAudit(t *testing.T, path string) []map[string]interface{} {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var entries []map[string]interface{}
	for _, line := range bytes.Split(bytes.TrimSpace(b), []byte("\n")) {
		var entry map[string]interface{}
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("decode audit line %q: %v", line, err)
		}
		if entry["event"] == "permission" {
			entries = append(entries, entry)
		}
	}
	return entries
}

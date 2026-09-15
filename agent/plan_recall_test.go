package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanModeLifecycleAndIsolation(t *testing.T) {
	r, runner, ws, _ := newPermissionTestRegistry(t, "open", nil, "")
	if got := r.Execute("enter_plan_mode", `{}`); strings.HasPrefix(got, "error:") {
		t.Fatal(got)
	}
	// A rejected write must fail before automatic checkpoint creation.
	r.autoCheckpointDone = false
	for _, call := range []struct{ name, args string }{
		{"write", `{"path":"sub/forbidden.txt","content":"x"}`},
		{"edit", `{"path":"other.txt","old_string":"x","new_string":"y"}`},
		{"shell", `{"command":"echo x > bypass.txt"}`},
		{"rollback", `{}`},
	} {
		if got := r.Execute(call.name, call.args); !strings.HasPrefix(got, "error: plan_mode:") {
			t.Fatalf("%s: %s", call.name, got)
		}
	}
	if runner.ran {
		t.Fatal("shell executed")
	}
	if _, err := os.Stat(filepath.Join(ws, "sub")); !os.IsNotExist(err) {
		t.Fatal("rejected write created directory")
	}
	if got := r.Execute("exit_plan_mode", `{}`); !strings.HasPrefix(got, "error: plan_mode:") {
		t.Fatal(got)
	}
	r.autoCheckpointDone = true
	if got := r.Execute("write", `{"path":"PLAN.md","content":"uncertain facts"}`); strings.HasPrefix(got, "error:") {
		t.Fatal(got)
	}
	if got := r.Execute("read", `{"path":"PLAN.md"}`); !strings.Contains(got, "uncertain facts") {
		t.Fatal(got)
	}
	if got := r.Execute("edit", `{"path":"PLAN.md","old_string":"uncertain facts","new_string":"revised notes"}`); strings.HasPrefix(got, "error:") {
		t.Fatal(got)
	}
	// A new registry with the same session manifest restores the restriction.
	resumed := NewRegistry(r.policy, runner, r.auditPath, r.manPath, false, false, false, strings.NewReader(""), r.exeDir, ws, r.taskID, r.permissions)
	resumed.autoCheckpointDone = true
	if err := resumed.checkPlanTool("write", `{"path":"other.txt"}`); err == nil {
		t.Fatal("resume lost stage")
	}
	other := &Registry{workspace: ws, policy: r.policy, manPath: filepath.Join(t.TempDir(), "other.jsonl")}
	if err := other.checkPlanTool("write", `{"path":"other.txt"}`); err != nil {
		t.Fatal("mode leaked between sessions", err)
	}
	if got := resumed.Execute("exit_plan_mode", `{}`); strings.HasPrefix(got, "error:") {
		t.Fatal(got)
	}
	if got := r.Execute("write", `{"path":"implementation.txt","content":"x"}`); strings.HasPrefix(got, "error:") {
		t.Fatal(got)
	}
}

func TestPlanModePreservesReadOnlyAndFreshnessAndDeny(t *testing.T) {
	r, _, ws, _ := newPermissionTestRegistry(t, "open", nil, "")
	plan := filepath.Join(ws, "PLAN.md")
	if err := os.WriteFile(plan, []byte("user notes"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := r.Execute("enter_plan_mode", `{}`); strings.HasPrefix(got, "error:") {
		t.Fatal(got)
	}
	if got := r.Execute("write", `{"path":"PLAN.md","content":"overwrite"}`); !strings.HasPrefix(got, "error:") {
		t.Fatal("freshness bypass", got)
	}
	r.readOnly = true
	if got := r.Execute("write", `{"path":"PLAN.md","content":"overwrite"}`); !strings.Contains(got, "read-only") {
		t.Fatal(got)
	}
	if got := r.Execute("read", `{"path":"PLAN.md"}`); !strings.Contains(got, "user notes") {
		t.Fatal(got)
	}
	if got := r.Execute("exit_plan_mode", `{}`); strings.HasPrefix(got, "error:") {
		t.Fatal(got)
	}
	if !r.readOnly {
		t.Fatal("exit disabled read-only")
	}
	r.readOnly = false
	r.permissions.Rules = []permissionRule{{Tool: "write", Pattern: "*", Action: "deny"}}
	r.Execute("enter_plan_mode", `{}`)
	if got := r.Execute("write", `{"path":"PLAN.md","content":"overwrite"}`); !strings.Contains(got, "denied by permission rule") {
		t.Fatal(got)
	}
	if b, _ := os.ReadFile(plan); string(b) != "user notes" {
		t.Fatal("existing notes changed")
	}
}

func TestPlanModeStateErrorsAndPathResolution(t *testing.T) {
	r, _, ws, _ := newPermissionTestRegistry(t, "open", nil, "")
	for _, args := range []string{`{"plan_path":"../outside.md"}`, `{"plan_path":"."}`, `{"plan_path":".git/PLAN.md"}`} {
		if got := r.Execute("enter_plan_mode", args); !strings.HasPrefix(got, "error:") {
			t.Fatal(got)
		}
	}
	r.Execute("enter_plan_mode", `{}`)
	if err := r.checkPlanTool("write", `{"path":"sub/../PLAN.md"}`); err != nil {
		t.Fatal(err)
	}
	if err := r.checkPlanTool("write", `{"path":"PLAN.md/../other.md"}`); err == nil {
		t.Fatal("path escape")
	}
	if err := os.WriteFile(r.planJournal(), []byte("broken\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := r.Execute("write", `{"path":"PLAN.md","content":"x"}`); !strings.Contains(got, "plan_mode_state") {
		t.Fatal(got)
	}
	if err := os.WriteFile(filepath.Join(ws, "source.txt"), []byte("read remains available"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := r.Execute("read", `{"path":"source.txt"}`); !strings.Contains(got, "read remains available") {
		t.Fatal(got)
	}
}

func TestRecallPageCompactionKeepsOriginAndRange(t *testing.T) {
	msgs := microFixture(t)
	ref, err := compactAttachment(strings.Repeat("source snapshot ", 6000))
	if err != nil {
		t.Fatal(err)
	}
	r := &Registry{policy: &Policy{Workspace: sess.workspace}, workspace: sess.workspace}
	offset := int64(200)
	page, err := r.readContentRef(ref, &offset, 8192)
	if err != nil {
		t.Fatal(err)
	}
	args := fmt.Sprintf(`{"content_ref":%q,"byte_offset":200,"byte_limit":8192}`, ref)
	pair := protocolRound("recall-page", args, page)
	pair[0].ToolCalls[0].Function.Name = "read"
	for _, m := range pair {
		if err := sess.record(m); err != nil {
			t.Fatal(err)
		}
	}
	// Place this page outside recent protection; keep a complete newest pair.
	msgs = append(msgs[:2], append(pair, msgs[2:]...)...)
	got, n := microCompact(msgs, 8)
	if n == 0 {
		t.Fatal("no compaction")
	}
	if !strings.Contains(got[3].Content, `"content_ref":"`+ref+`"`) || !strings.Contains(got[3].Content, `"recall_page":{"byte_limit":8192,"byte_offset":200,"next_byte_offset":8392}`) {
		t.Fatal(got[3].Content)
	}
	if msgs[3].Content != page {
		t.Fatal("original page changed")
	}
	index, err := compactRecoveryIndex(msgs)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(index, "lc1.") {
		t.Fatal(index)
	}
	row, err := apiFindToolResult(sess.path, "recall-page")
	if err != nil {
		t.Fatal(err)
	}
	gotRef, rng, err := compactResultReference(row, pair[0].ToolCalls[0])
	if err != nil || gotRef != ref || rng["byte_offset"] != 200 {
		t.Fatalf("%s %v %v", gotRef, rng, err)
	}
	// Recalling the same range remains allowed and yields the same page.
	for i := 0; i < 3; i++ {
		again, err := r.toolRead(args)
		if err != nil || again != page {
			t.Fatal(err)
		}
	}
}

func TestRecallRejectsMixedAddressingAndRecordsFacts(t *testing.T) {
	microFixture(t)
	r, _, _, audit := newPermissionTestRegistry(t, "open", nil, "")
	ref, err := compactAttachment(strings.Repeat("abcd", 1000))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.toolRead(fmt.Sprintf(`{"content_ref":%q}`, ref)); err == nil {
		t.Fatal("cross-workspace recall was allowed")
	}
	r.workspace = sess.workspace
	r.policy = &Policy{Workspace: sess.workspace}
	for _, key := range []string{`"path":"ignored"`, `"offset":0`, `"limit":0`} {
		if _, err := r.toolRead(fmt.Sprintf(`{"content_ref":%q,%s}`, ref, key)); err == nil {
			t.Fatal("silently ignored", key)
		}
	}
	for i := 0; i < 2; i++ {
		if _, err := r.toolRead(fmt.Sprintf(`{"content_ref":%q,"byte_limit":1024}`, ref)); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(audit)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 {
		t.Fatal(string(b))
	}
	for _, line := range lines {
		var row map[string]interface{}
		json.Unmarshal([]byte(line), &row)
		if row["kind"] != "lc1" || row["source_bytes"] != float64(1024) || row["page_sha256"] == "" {
			t.Fatal(row)
		}
	}
}

func TestPlanModeJunctionCannotRedirectPlan(t *testing.T) {
	r, _, ws, _ := newPermissionTestRegistry(t, "open", nil, "")
	inside := filepath.Join(ws, "notes")
	outside := t.TempDir()
	if err := os.MkdirAll(inside, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(ws, "alias")
	if output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", link, inside).CombinedOutput(); err != nil {
		t.Fatalf("junction: %v %s", err, output)
	}
	if got := r.Execute("enter_plan_mode", `{"plan_path":"alias/PLAN.md"}`); strings.HasPrefix(got, "error:") {
		t.Fatal(got)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", link, outside).CombinedOutput(); err != nil {
		t.Fatalf("junction: %v %s", err, output)
	}
	if got := r.Execute("write", `{"path":"alias/PLAN.md","content":"no"}`); !strings.HasPrefix(got, "error: plan_mode:") {
		t.Fatal(got)
	}
	if _, err := os.Stat(filepath.Join(outside, "PLAN.md")); !os.IsNotExist(err) {
		t.Fatal("redirected write")
	}
}

func TestPlanReadOnlyCannotEnterUnsavablePlan(t *testing.T) {
	r, _, ws, _ := newPermissionTestRegistry(t, "open", nil, "")
	r.readOnly = true
	if got := r.Execute("enter_plan_mode", `{}`); !strings.Contains(got, "read-only") {
		t.Fatal(got)
	}
	state, err := r.loadPlanMode()
	if err != nil || state.Active {
		t.Fatal("stuck in unsavable mode", err)
	}
	if _, err := os.Stat(filepath.Join(ws, "PLAN.md")); !os.IsNotExist(err) {
		t.Fatal("created notes in read-only")
	}
}

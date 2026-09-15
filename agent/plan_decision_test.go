package main

import (
	openai "github.com/sashabaranov/go-openai"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func decisionFixture(t *testing.T) (*Registry, string) {
	t.Helper()
	r, _, ws, _ := newPermissionTestRegistry(t, "open", nil, "")
	if _, err := r.toolEnterPlanMode(`{}`); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "PLAN.md"), []byte("Known facts; remaining unknowns"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := r.toolAskPlanningDecision(`{"question":"Which delivery format is required?"}`); err != nil {
		t.Fatal(err)
	}
	d, err := r.pendingPlanningDecision()
	if err != nil || d == nil {
		t.Fatalf("%v %v", d, err)
	}
	return r, d.ID
}

func TestPlanningDecisionReplyIsNotApproval(t *testing.T) {
	r, id := decisionFixture(t)
	if r.validateDecisionAnswer("stale") == nil {
		t.Fatal("accepted stale reply")
	}
	if r.recordDecisionAnswer(id, "", "unknown") == nil {
		t.Fatal("accepted unpersisted reply")
	}
	if err := r.checkPlanTool("read", `{"path":"PLAN.md"}`); err == nil {
		t.Fatal("pending decision did not pause remaining tools")
	}
	if err := r.recordDecisionAnswer(id, "persisted-user-uuid", "I don't know; what do you suggest?"); err != nil {
		t.Fatal(err)
	}
	state, err := r.loadPlanMode()
	if err != nil {
		t.Fatal(err)
	}
	if !state.Active || state.Decision.AwaitingReply || state.Decision.Cancelled || state.Decision.ReplyUUID != "persisted-user-uuid" {
		t.Fatalf("%+v", state)
	}
	if r.validateDecisionAnswer(id) == nil {
		t.Fatal("duplicate answer accepted")
	}
	if err := r.checkPlanTool("write", `{"path":"implementation.go"}`); err == nil {
		t.Fatal("reply unlocked implementation")
	}
	other := &Registry{manPath: filepath.Join(t.TempDir(), "other.jsonl")}
	if other.validateDecisionAnswer(id) == nil {
		t.Fatal("decision crossed session")
	}
}

func TestPlanningAttachmentRebuiltAndUserTextUnchanged(t *testing.T) {
	r, id := decisionFixture(t)
	text := planContextMarker + " user literal"
	msgs := []openai.ChatCompletionMessage{{Role: "system", Content: "base"}, {Role: "user", Content: text}}
	for i := 0; i < 3; i++ {
		if err := r.projectPlanContext(&msgs); err != nil {
			t.Fatal(err)
		}
	}
	if len(msgs) != 3 || msgs[2].Content != text || !strings.Contains(msgs[0].Content, id) {
		t.Fatalf("%+v", msgs)
	}
	if err := r.recordDecisionAnswer(id, "uuid", "unknown"); err != nil {
		t.Fatal(err)
	}
	// Simulate a summary replacing request history; facts are regenerated from the journal.
	msgs = []openai.ChatCompletionMessage{{Role: "user", Content: "summary"}}
	resumed := &Registry{manPath: r.manPath}
	if err := resumed.projectPlanContext(&msgs); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msgs[0].Content, `"awaitingReply":false`) || !strings.Contains(msgs[0].Content, `"decisionResolution":"not_judged"`) {
		t.Fatal(msgs[0].Content)
	}
}

func TestPlanningUserExitCancelsWithoutAnswer(t *testing.T) {
	r, id := decisionFixture(t)
	r.readOnly = true
	if _, err := r.toolExitPlanMode(`{}`); err != nil {
		t.Fatal(err)
	}
	s, err := r.loadPlanMode()
	if err != nil {
		t.Fatal(err)
	}
	if s.Active || !s.Decision.Cancelled || s.Decision.AwaitingReply || s.Decision.ReplyUUID != "" || !r.readOnly {
		t.Fatalf("%+v", s)
	}
	if r.validateDecisionAnswer(id) == nil {
		t.Fatal("cancelled decision accepted answer")
	}
	if _, err := r.toolEnterPlanMode(`{}`); err != nil {
		t.Fatal(err)
	}
	s, err = r.loadPlanMode()
	if err != nil || !s.Active || s.Decision != nil {
		t.Fatalf("new planning retained an old decision: %+v %v", s, err)
	}
}

func TestPlanningAPIRejectsWrongReplyAndBusyExit(t *testing.T) {
	a := testAPI(t)
	r, id := decisionFixture(t)
	a.reg = r
	a.selected = "current"
	// Avoid closing the isolated permission-test runner through API session teardown.
	defer func() { a.reg = nil }()
	for _, body := range []string{`{"sessionId":"other","decisionId":"` + id + `","answer":"yes"}`, `{"sessionId":"current","decisionId":"stale","answer":"yes"}`} {
		w := callAPI(a, "POST", "/api/answer", body, true)
		if w.Code != 409 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
	}
	if len(a.messages) != 0 {
		t.Fatal("invalid reply appended")
	}
	w := callAPI(a, "GET", "/api/plan", "", true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), id) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	a.busy = true
	w = callAPI(a, "POST", "/api/plan/exit", `{"sessionId":"current"}`, true)
	if w.Code != 409 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	a.busy = false
	w = callAPI(a, "POST", "/api/plan/exit", `{"sessionId":"other"}`, true)
	if w.Code != 409 {
		t.Fatal(w.Code)
	}
	w = callAPI(a, "POST", "/api/plan/exit", `{"sessionId":"current"}`, true)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

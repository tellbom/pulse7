package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func captureEvents(t *testing.T) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var protocol bytes.Buffer
	var human bytes.Buffer
	oldEvents, oldOut := runtimeEvents, teeOut
	runtimeEvents = newEventBus(true, &protocol)
	teeOut = &human
	t.Cleanup(func() {
		runtimeEvents = oldEvents
		teeOut = oldOut
	})
	return &protocol, &human
}

func parseEventLines(t *testing.T, content string) []runtimeEvent {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	events := make([]runtimeEvent, 0, len(lines))
	for i, line := range lines {
		var event runtimeEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("protocol line %d is not JSON: %q: %v", i+1, line, err)
		}
		if event.Type == "" || event.Data == nil {
			t.Fatalf("protocol line %d missing envelope fields: %s", i+1, line)
		}
		events = append(events, event)
	}
	return events
}

func TestStreamJSONEveryStdoutLineIsAnEventAndHumanTextIsSeparate(t *testing.T) {
	protocol, human := captureEvents(t)
	emitRuntimeEvent("session_init", sessionInitEvent{
		SessionID: "s1", Workspace: `C:\work`, Model: "m", Tools: []string{"read"},
		Skills:        []skillInfo{{Name: "release", Description: "release work", Path: `.pulse7\skills\release\SKILL.md`}},
		ContextBudget: 12000,
	})
	emitRuntimeEvent("assistant_delta", assistantDeltaEvent{Delta: "hello\n"})
	emitRuntimeEvent("tool_call", toolCallEvent{ID: "call-1", Name: "read", Args: json.RawMessage(`{"path":"a.txt"}`)})
	emitRuntimeEvent("tool_result", toolResultEvent{
		ID: "call-1", OK: true, Summary: "read a.txt（1 行）", ResultRef: "session:s1#tool:call-1",
		Name: "read", Args: `{"path":"a.txt"}`, Result: "secret full result",
	})
	emitRuntimeEvent("permission_request", permissionRequestEvent{Tool: "shell", Args: json.RawMessage(`{"command":"dir"}`), Target: "dir"})
	emitRuntimeEvent("permission_response", permissionResponseEvent{Tool: "shell", Decision: "allow", Source: "preset:strict:user", Requested: true, Target: "dir"})
	emitRuntimeEvent("context_state", contextStateEvent{UsedTokens: 100, Budget: 1000, PercentLeft: 90, WarningLevel: "normal"})
	emitRuntimeEvent("compaction", compactionEvent{Reason: "threshold", BeforeTokens: 700, AfterTokens: 400, Summary: "summary", Method: compressionSummary, Removed: 4})
	emitRuntimeEvent("skill_loaded", skillLoadedEvent{Name: "release", Path: `.pulse7\skills\release\SKILL.md`})
	emitRuntimeEvent("background_task", backgroundTaskEvent{Action: "start", TaskID: "t1", PID: intPointer(42), Command: "dir", Detached: boolPointer(false)})
	emitRuntimeEvent("background_task", backgroundTaskEvent{Action: "output", TaskID: "t1", Status: "running", ExitCode: intPointer(-1), Offset: 0, NextOffset: 4})
	emitRuntimeEvent("background_task", backgroundTaskEvent{Action: "end", TaskID: "t1", PID: intPointer(42), Status: "exited", ExitCode: intPointer(0)})
	emitRuntimeEvent("heartbeat", heartbeatEvent{WaitedSeconds: 15})
	emitRuntimeEvent("turn_result", turnResultEvent{Status: "success"})
	flushAssistantEvents()

	events := parseEventLines(t, protocol.String())
	wantTypes := []string{
		"session_init", "assistant_delta", "tool_call", "tool_result", "permission_request",
		"permission_response", "context_state", "compaction", "skill_loaded", "background_task",
		"background_task", "background_task", "heartbeat", "turn_result",
	}
	if len(events) != len(wantTypes) {
		t.Fatalf("event count = %d, want %d\n%s", len(events), len(wantTypes), protocol.String())
	}
	for i, want := range wantTypes {
		if events[i].Type != want {
			t.Fatalf("event %d type = %q, want %q", i, events[i].Type, want)
		}
	}
	if strings.Contains(protocol.String(), "secret full result") || strings.Contains(protocol.String(), "[permission]") {
		t.Fatalf("protocol stdout leaked human/full-result text: %s", protocol.String())
	}
	for _, want := range []string{"| hello", "-> read a.txt", "[OK] read a.txt", "[permission] ASK shell dir", "[上下文已压缩：4 条消息 → 摘要]", "[task] STARTED", "[task] FINISHED", "[等待模型响应... 15s]"} {
		if !strings.Contains(human.String(), want) {
			t.Fatalf("human stderr missing %q:\n%s", want, human.String())
		}
	}
}

func TestStreamJSONDoesNotChangeStreamResult(t *testing.T) {
	protocol, human := captureEvents(t)
	body := "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"same result\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: [DONE]\n\n"
	sink, err := runStreamFixture(t, body)
	if err != nil || sink.content.String() != "same result" {
		t.Fatalf("stream result = %q error=%v", sink.content.String(), err)
	}
	events := parseEventLines(t, protocol.String())
	if len(events) != 1 || events[0].Type != "assistant_delta" {
		t.Fatalf("stream events = %#v\n%s", events, protocol.String())
	}
	if !strings.Contains(human.String(), "| same result") {
		t.Fatalf("human output = %q", human.String())
	}
}

func TestTurnResultErrorShape(t *testing.T) {
	protocol, _ := captureEvents(t)
	emitTurnResult("error", errors.New("endpoint failed"))
	lines := parseEventLines(t, protocol.String())
	if len(lines) != 1 || lines[0].Type != "turn_result" {
		t.Fatalf("turn result lines = %#v", lines)
	}
	encoded, err := json.Marshal(lines[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), "endpoint failed") {
		t.Fatalf("turn result data = %s", encoded)
	}
}

func TestSessionInitUsesEmptySkillsArray(t *testing.T) {
	protocol, _ := captureEvents(t)
	workspace := t.TempDir()
	emitSessionInit(&config{workspace: workspace, model: "m", maxCtx: 4000}, &Registry{tools: map[string]Tool{}}, "s1")
	if !strings.Contains(protocol.String(), `"skills":[]`) {
		t.Fatalf("session_init skills must be an array: %s", protocol.String())
	}
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

func protocolRound(id, arguments, result string) []openai.ChatCompletionMessage {
	return []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{{
			ID: id, Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{Name: "write", Arguments: arguments},
		}}},
		{Role: openai.ChatMessageRoleTool, ToolCallID: id, Content: result},
	}
}

func TestContextAccountingIncludesToolArgumentsAndSchemas(t *testing.T) {
	arguments := strings.Repeat("x", 20_000)
	msgs := protocolRound("call-large", arguments, "ok")
	legacy := 0
	for _, msg := range msgs {
		legacy += len(msg.Role) + len(msg.Content) + len(msg.Name) + len(msg.ToolCallID)
	}
	got := contextChars(msgs)
	t.Logf("20KB tool argument accounting: legacy=%d current=%d", legacy, got)
	if got < len(arguments) {
		t.Fatalf("contextChars = %d, want at least %d tool-argument chars", got, len(arguments))
	}
	tools := []openai.Tool{{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{
		Name: "large_schema", Description: strings.Repeat("schema", 2_000),
	}}}
	if requestChars := requestContextChars(msgs, tools); requestChars <= got+10_000 {
		t.Fatalf("requestContextChars = %d, tool schema was not counted", requestChars)
	}
}

func TestTruncateContextRemovesWholeToolProtocolGroups(t *testing.T) {
	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "system"},
		{Role: openai.ChatMessageRoleUser, Content: "current task constraints"},
	}
	for i := 1; i <= 100; i++ {
		msgs = append(msgs, protocolRound(fmt.Sprintf("call-%d", i), strings.Repeat("a", 200), strings.Repeat("r", 200))...)
	}
	truncateContext(&msgs, 8_000)
	if contextChars(msgs) > 8_000 {
		t.Fatalf("context remains over budget: %d", contextChars(msgs))
	}
	if msgs[0].Role != openai.ChatMessageRoleSystem || msgs[1].Role != openai.ChatMessageRoleUser {
		t.Fatalf("protected system/current-user messages were removed: %+v", msgs[:2])
	}
	assertValidToolProtocol(t, msgs)
}

func TestCompressionPromptKeepsUserConstraintAfterByte500(t *testing.T) {
	tail := "MUST_KEEP_USER_TAIL"
	userText := strings.Repeat("前", 700) + tail
	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		captured, _ = io.ReadAll(req.Body)
		writeSummaryFixture(w, "summary")
	}))
	defer srv.Close()
	ocfg := openai.DefaultConfig("test")
	ocfg.BaseURL = srv.URL
	client := openai.NewClientWithConfig(ocfg)
	cfg := &config{model: "test", maxCtx: 1_000, llmCompressTimeout: time.Second}
	msgs := []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: "system"}, {Role: openai.ChatMessageRoleUser, Content: userText}}
	for i := 1; i <= 4; i++ {
		msgs = append(msgs, protocolRound(fmt.Sprintf("call-%d", i), `{"path":"a"}`, "ok")...)
	}
	maybeCompressContext(context.Background(), client, cfg, nil, &msgs)
	if !strings.Contains(string(captured), tail) {
		t.Fatalf("compression request lost user tail after byte 500: %s", captured)
	}
}

func assertValidToolProtocol(t *testing.T, msgs []openai.ChatCompletionMessage) {
	t.Helper()
	pending := map[string]bool{}
	for i, msg := range msgs {
		if msg.Role == openai.ChatMessageRoleAssistant && len(msg.ToolCalls) > 0 {
			if len(pending) != 0 {
				t.Fatalf("message %d starts tool calls before prior results: %+v", i, pending)
			}
			for _, call := range msg.ToolCalls {
				pending[call.ID] = true
			}
			continue
		}
		if msg.Role == openai.ChatMessageRoleTool {
			if !pending[msg.ToolCallID] {
				t.Fatalf("message %d is orphan tool result %q", i, msg.ToolCallID)
			}
			delete(pending, msg.ToolCallID)
			continue
		}
		if len(pending) != 0 {
			t.Fatalf("message %d interrupts pending tool results: %+v", i, pending)
		}
	}
	if b, _ := json.Marshal(pending); len(pending) != 0 {
		t.Fatalf("tool calls missing results: %s", b)
	}
}

package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	openai "github.com/sashabaranov/go-openai"
)

const (
	outputFormatText       = "text"
	outputFormatStreamJSON = "stream-json"
)

type runtimeEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type processModeEvent struct {
	CleanupScope         string `json:"cleanupScope"`
	DescendantsMayEscape bool   `json:"descendantsMayEscape"`
	Action               string `json:"action"`
	Mode                 string `json:"mode"`
	Restricted           bool   `json:"restricted"`
	CleanupGuaranteed    bool   `json:"cleanupGuaranteed"`
	Reason               string `json:"reason,omitempty"`
	Notice               string `json:"notice,omitempty"`
}

type processWarningEvent struct {
	Kind      string `json:"kind"`
	Current   int    `json:"current"`
	Threshold int    `json:"threshold"`
	TaskID    string `json:"taskId,omitempty"`
	Blocked   bool   `json:"blocked"`
}

type sessionInitEvent struct {
	SessionID     string      `json:"sessionId"`
	Workspace     string      `json:"workspace"`
	Model         string      `json:"model"`
	Tools         []string    `json:"tools"`
	Skills        []skillInfo `json:"skills"`
	ContextBudget int         `json:"contextBudget"`
}

type assistantDeltaEvent struct {
	Delta   string `json:"delta"`
	Attempt uint64 `json:"attempt,omitempty"`
}

// Provider-supplied reasoning stays separate from the assistant answer.
type assistantReasoningDeltaEvent struct {
	Delta   string `json:"delta"`
	Attempt uint64 `json:"attempt"`
}

type assistantAttemptEvent struct {
	Attempt uint64 `json:"attempt"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
}

type toolCallEvent struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type toolResultEvent struct {
	ErrorCode string `json:"errorCode,omitempty"`
	ID        string `json:"id"`
	OK        bool   `json:"ok"`
	Summary   string `json:"summary"`
	ResultRef string `json:"resultRef"`
	Name      string `json:"-"`
	Args      string `json:"-"`
	Result    string `json:"-"`
}

type permissionRequestEvent struct {
	RequestID string          `json:"requestId,omitempty"`
	Tool      string          `json:"tool"`
	Args      json.RawMessage `json:"args"`
	Target    string          `json:"target"`
}

type permissionResponseEvent struct {
	RequestID string `json:"requestId,omitempty"`
	Tool      string `json:"tool"`
	Decision  string `json:"decision"`
	Source    string `json:"source"`
	Requested bool   `json:"requested"`
	Target    string `json:"target"`
}

type contextStateEvent struct {
	UsedTokens   int     `json:"usedTokens"`
	Budget       int     `json:"budget"`
	PercentLeft  float64 `json:"percentLeft"`
	WarningLevel string  `json:"warningLevel"`
}

type compactionEvent struct {
	OriginalBytes        *int      `json:"originalBytes,omitempty"`
	DiscardedBytes       *int      `json:"discardedBytes,omitempty"`
	KeptToolCallIDs      *[]string `json:"keptToolCallIds,omitempty"`
	DiscardedToolCallIDs *[]string `json:"discardedToolCallIds,omitempty"`
	Reason               string    `json:"reason"`
	BeforeTokens         int       `json:"beforeTokens"`
	AfterTokens          int       `json:"afterTokens"`
	Emergency            bool      `json:"emergency"`
	Summary              string    `json:"summary"`
	Method               string    `json:"method"`
	Removed              int       `json:"removedMessages"`
}

type skillLoadedEvent struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type backgroundTaskEvent struct {
	Error      string `json:"error,omitempty"`
	Action     string `json:"action"`
	TaskID     string `json:"taskId"`
	PID        *int   `json:"pid,omitempty"`
	Command    string `json:"command,omitempty"`
	Detached   *bool  `json:"detached,omitempty"`
	Status     string `json:"status,omitempty"`
	ExitCode   *int   `json:"exitCode,omitempty"`
	Offset     int    `json:"offset,omitempty"`
	NextOffset int    `json:"nextOffset,omitempty"`
}

type heartbeatEvent struct {
	WaitedSeconds int `json:"waitedSeconds"`
}

type turnResultEvent struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// eventConsumer is one output of the same runtime stream. Callbacks execute
// in emission order under the bus lock and must not re-enter the bus.
type eventConsumer struct {
	emit           func(runtimeEvent)
	flushAssistant func()
	resetAssistant func()
}

type eventBus struct {
	mu        sync.Mutex
	attempt   uint64
	consumers []eventConsumer
}

func (b *eventBus) emit(event runtimeEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, consumer := range b.consumers {
		consumer.emit(event)
	}
}

func (b *eventBus) flushAssistant() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, consumer := range b.consumers {
		if consumer.flushAssistant != nil {
			consumer.flushAssistant()
		}
	}
}

func (b *eventBus) resetAssistant() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, consumer := range b.consumers {
		if consumer.resetAssistant != nil {
			consumer.resetAssistant()
		}
	}
}

func intPointer(value int) *int    { return &value }
func boolPointer(value bool) *bool { return &value }

func emitRuntimeEvent(kind string, data interface{}) {
	runtimeEvents.emit(runtimeEvent{Type: kind, Data: data})
}

func emitSessionInit(cfg *config, reg *Registry, sessionID string) {
	definitions := reg.Definitions()
	tools := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		tools = append(tools, definition.Function.Name)
	}
	catalog := discoverSkills(cfg.workspace)
	skills := catalog.Skills
	if skills == nil {
		skills = []skillInfo{}
	}
	emitRuntimeEvent("session_init", sessionInitEvent{
		SessionID: sessionID, Workspace: cfg.workspace, Model: cfg.model,
		Tools: tools, Skills: skills, ContextBudget: cfg.maxCtx / 4,
	})
}

func emitTurnResult(status string, err error) {
	message := ""
	if err != nil {
		message = err.Error()
	}
	emitRuntimeEvent("turn_result", turnResultEvent{Status: status, Error: message})
}

func emitContextState(msgs []openai.ChatCompletionMessage, tools []openai.Tool, cfg *config) {
	used := requestContextChars(msgs, tools) / 4
	budget := cfg.maxCtx / 4
	percentLeft := 0.0
	if budget > 0 {
		percentLeft = float64(budget-used) * 100 / float64(budget)
		if percentLeft < 0 {
			percentLeft = 0
		}
	}
	warning := "normal"
	if percentLeft <= 0 {
		warning = "critical"
	} else if percentLeft <= (1-compressThreshold)*100 {
		warning = "warning"
	}
	emitRuntimeEvent("context_state", contextStateEvent{
		UsedTokens: used, Budget: budget, PercentLeft: percentLeft, WarningLevel: warning,
	})
}

func emitToolCall(call openai.ToolCall) {
	emitRuntimeEvent("tool_call", toolCallEvent{
		ID: call.ID, Name: call.Function.Name, Args: rawEventArgs(call.Function.Arguments),
	})
}

type toolOutcome struct {
	OK        bool   `json:"ok"`
	Summary   string `json:"summary"`
	ErrorCode string `json:"errorCode,omitempty"`
}

func toolOutcomeFor(call openai.ToolCall, result string) *toolOutcome {
	summary, failed, _ := resultSummary(call.Function.Name, call.Function.Arguments, result)
	code := ""
	if strings.HasPrefix(result, "error: hard_link_impact_unknown:") {
		code = "hard_link_impact_unknown"
	}
	return &toolOutcome{OK: !failed, Summary: summary, ErrorCode: code}
}

func emitToolResult(call openai.ToolCall, result string) {
	outcome := toolOutcomeFor(call, result)
	ref := "tool:" + call.ID
	if sess != nil {
		ref = fmt.Sprintf("session:%s#tool:%s", sess.id(), call.ID)
	}
	emitRuntimeEvent("tool_result", toolResultEvent{ErrorCode: outcome.ErrorCode,
		ID: call.ID, OK: outcome.OK, Summary: outcome.Summary, ResultRef: ref,
		Name: call.Function.Name, Args: call.Function.Arguments, Result: result,
	})
}

func emitCompaction(beforeChars, afterChars int, emergency bool, method string, removed int, summary string) {
	reason := "threshold"
	if emergency {
		reason = "context_length_exceeded"
	}
	emitRuntimeEvent("compaction", compactionEvent{
		Reason: reason, BeforeTokens: beforeChars / 4, AfterTokens: afterChars / 4,
		Emergency: emergency, Summary: summary, Method: method, Removed: removed,
	})
}

func flushAssistantEvents() {
	runtimeEvents.flushAssistant()
}

func resetAssistantEvents() {
	runtimeEvents.resetAssistant()
}

func rawEventArgs(args string) json.RawMessage {
	if json.Valid([]byte(args)) {
		return json.RawMessage(args)
	}
	return json.RawMessage("null")
}

func (b *eventBus) nextAttempt() uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.attempt++
	return b.attempt
}

type outsideWorkspaceWriteEvent struct {
	Tool              string `json:"tool"`
	RequestedPath     string `json:"requestedPath"`
	ResolvedPath      string `json:"resolvedPath"`
	CheckpointCovered bool   `json:"checkpointCovered"`
	RollbackCovered   bool   `json:"rollbackCovered"`
}

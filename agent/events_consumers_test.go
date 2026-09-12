package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestEventConsumersPreserveOrderWithoutCLI(t *testing.T) {
	var first, second, controls []string
	b := &eventBus{consumers: []eventConsumer{
		{emit: func(e runtimeEvent) { first = append(first, e.Type) }, resetAssistant: func() { controls = append(controls, "reset") }, flushAssistant: func() { controls = append(controls, "flush") }},
		{emit: func(e runtimeEvent) { second = append(second, e.Type) }},
	}}
	b.emit(runtimeEvent{Type: "assistant_attempt", Data: assistantAttemptEvent{Attempt: 1, Status: "start"}})
	b.emit(runtimeEvent{Type: "assistant_delta", Data: assistantDeltaEvent{Attempt: 1, Delta: "discard me"}})
	b.resetAssistant()
	b.emit(runtimeEvent{Type: "assistant_attempt", Data: assistantAttemptEvent{Attempt: 1, Status: "discard"}})
	b.flushAssistant()
	want := "assistant_attempt,assistant_delta,assistant_attempt"
	if strings.Join(first, ",") != want || strings.Join(second, ",") != want || strings.Join(controls, ",") != "reset,flush" {
		t.Fatalf("first=%v second=%v controls=%v", first, second, controls)
	}
}

type failedProtocolWriter struct{}

func (failedProtocolWriter) Write([]byte) (int, error) {
	return 0, errors.New("protocol output failed")
}
func TestEventConsumerWriteFailureIsNotSilenced(t *testing.T) {
	defer func() {
		failure := recover()
		if failure == nil || !strings.Contains(fmt.Sprint(failure), "protocol output failed") {
			t.Fatalf("failure=%v", failure)
		}
	}()
	newEventBus(true, failedProtocolWriter{}).emit(runtimeEvent{Type: "turn_result", Data: turnResultEvent{Status: "success"}})
}

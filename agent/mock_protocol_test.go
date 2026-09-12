package main

import (
	"net/http/httptest"
	"testing"
)

func TestMockTextCompletionHasExplicitStop(t *testing.T) {
	recorder := httptest.NewRecorder()
	sseContent(recorder, recorder, "complete")
	finishMockStream(recorder, recorder, false)

	sink, err := runStreamFixture(t, recorder.Body.String())
	if err != nil || sink.content.String() != "complete" {
		t.Fatalf("mock text stream content=%q err=%v\n%s", sink.content.String(), err, recorder.Body.String())
	}
}

func TestMockToolCompletionKeepsToolCallsTerminal(t *testing.T) {
	recorder := httptest.NewRecorder()
	emitCalls(recorder, recorder, []mockCall{{idx: 0, id: "call-1", name: "read", ar: `{"path":"a.txt"}`}})
	finishMockStream(recorder, recorder, true)

	sink, err := runStreamFixture(t, recorder.Body.String())
	if err != nil || len(sink.calls()) != 1 {
		t.Fatalf("mock tool stream calls=%+v err=%v\n%s", sink.calls(), err, recorder.Body.String())
	}
}

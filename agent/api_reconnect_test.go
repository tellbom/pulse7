package main

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestReconnectBusyRuntimeAndHistory(t *testing.T) {
	a := testAPI(t)
	a.selected = "live"
	a.busy = true
	w := callAPI(a, "GET", "/api/runtime", "", true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"sessionId":"live"`) || !strings.Contains(w.Body.String(), `"state":"running"`) {
		t.Fatal(w.Code, w.Body)
	}
	if callAPI(a, "GET", "/api/runtime", "", false).Code != 401 {
		t.Fatal("auth bypass")
	}
	if callAPI(a, "POST", "/api/sessions/resume", `{"sessionId":"other"}`, true).Code != 409 {
		t.Fatal("busy resume allowed")
	}
	path, err := a.sessionFile("history")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err = appendJSONLine(path, map[string]string{"role": "user", "content": "old history"}); err != nil {
		t.Fatal(err)
	}
	w = callAPI(a, "GET", "/api/sessions/history/messages?limit=1", "", true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "old history") {
		t.Fatal(w.Code, w.Body)
	}
	if a.selected != "live" || !a.busy {
		t.Fatal("preview mutated execution")
	}
}

func TestReconnectCursorBoundsAndSessionAttribution(t *testing.T) {
	a := testAPI(t)
	a.publish(runtimeEvent{Type: "session_init", Data: map[string]string{"sessionId": "first"}})
	a.publish(runtimeEvent{Type: "assistant_delta", Data: map[string]string{"delta": "partial"}})
	req := httptest.NewRequest("GET", "/api/events?after="+a.streamID+":1", nil)
	rows, err := a.replayAfter(req)
	if err != nil || len(rows) != 1 {
		t.Fatal(len(rows), err)
	}
	var event map[string]interface{}
	json.Unmarshal(rows[0], &event)
	if event["sessionId"] != "first" || event["seq"] != float64(2) {
		t.Fatal(event)
	}
	out := httptest.NewRecorder()
	if !writeAPIEvent(out, out, rows[0]) || !strings.Contains(out.Body.String(), "id: "+a.streamID+":2") {
		t.Fatal(out.Body)
	}
	for _, cursor := range []string{"old:1", a.streamID + ":99", a.streamID + ":bad"} {
		if _, err = a.replayAfter(httptest.NewRequest("GET", "/api/events?after="+cursor, nil)); err == nil {
			t.Fatal(cursor)
		}
	}
	for i := 0; i < 2050; i++ {
		a.publish(runtimeEvent{Type: "test", Data: i})
	}
	if len(a.replay) > 2048 {
		t.Fatal("unbounded replay")
	}
	if _, err = a.replayAfter(req); err == nil {
		t.Fatal("expired cursor silently accepted")
	}
	a.publish(runtimeEvent{Type: "large", Data: strings.Repeat("x", 4*1024*1024)})
	if len(a.replay) != 0 || a.replayBytes != 0 {
		t.Fatal("oversize event retained")
	}
}

func TestReconnectHistoryConcurrentAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	if err := appendJSONLine(path, map[string]string{"role": "user", "content": "first"}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 30; i++ {
			if err := appendJSONLine(path, map[string]string{"role": "assistant", "content": fmt.Sprint(i)}); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	for i := 0; i < 30; i++ {
		if _, _, err := apiPagedMessages(path, 0, 10); err != nil {
			t.Error(err)
		}
	}
	wg.Wait()
}

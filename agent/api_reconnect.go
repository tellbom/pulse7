package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Called with apiServer.mu held; publishers only take streamMu.
func (a *apiServer) runtimeView(w http.ResponseWriter, r *http.Request) {
	a.streamMu.Lock()
	defer a.streamMu.Unlock()
	state := "idle"
	if a.busy {
		state = "running"
	} else if a.waitingAnswer {
		state = "need_answer"
	}
	apiJSON(w, 200, map[string]interface{}{
		"sessionId": a.selected, "workspace": a.cfg.workspace, "state": state, "busy": a.busy,
		"streamId": a.streamID, "cursor": a.eventSeq, "oldestCursor": a.eventSeq - uint64(len(a.replay)) + 1,
		"turnActive": a.turnActive, "turnStartCursor": a.turnStartCursor, "turnHistoryCount": a.turnHistoryCount,
	})
}

// Validate and snapshot backlog under streamMu, before registering the live
// subscriber. Events cannot fall between replay and subscription.
func (a *apiServer) replayAfter(r *http.Request) ([][]byte, error) {
	cursor := r.Header.Get("Last-Event-ID")
	if q := r.URL.Query().Get("after"); q != "" {
		cursor = q
	}
	if cursor == "" {
		return nil, nil
	} // Existing clients retain live-only behavior.
	parts := strings.Split(cursor, ":")
	if len(parts) != 2 || parts[0] != a.streamID {
		return nil, errors.New("stream changed; refresh runtime and persisted history")
	}
	seq, err := strconv.ParseUint(parts[1], 10, 64)
	oldest := a.eventSeq - uint64(len(a.replay))
	if err != nil || seq < oldest || seq > a.eventSeq {
		return nil, errors.New("cursor unavailable; refresh runtime and persisted history")
	}
	return append([][]byte(nil), a.replay[int(seq-oldest):]...), nil
}

func writeAPIEvent(w http.ResponseWriter, f http.Flusher, b []byte) bool {
	var e struct {
		Type     string `json:"type"`
		StreamID string `json:"streamId"`
		Seq      uint64 `json:"seq"`
	}
	if json.Unmarshal(b, &e) != nil {
		return false
	}
	if _, err := fmt.Fprintf(w, "id: %s:%d\nevent: %s\ndata: %s\n\n", e.StreamID, e.Seq, e.Type, b); err != nil {
		return false
	}
	f.Flush()
	return true
}

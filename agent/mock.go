package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// Embedded mock OpenAI endpoint for smoke tests (loopback HTTP only).
// Triggers (user message content):
//   "M1-SMOKE"  -> one round: read + get_time + shell(redirect-fix regression)
//   "M2-FILES"  -> one round: write + edit + grep + glob + ls
//   "M2-GIT"    -> phased: checkpoint -> write -> rollback -> final
//   tool result -> final answer echoing results
//   otherwise   -> plain streaming text
func runMock(seconds int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req openai.ChatCompletionRequest
		json.Unmarshal(body, &req)
		var lastRole, lastContent string
		nTools := 0
		hasM2Git, hasM3 := false, false
		for _, m := range req.Messages {
			if m.Role == openai.ChatMessageRoleTool {
				nTools++
			}
			if m.Role == openai.ChatMessageRoleUser {
				if strings.Contains(m.Content, "M2-GIT") {
					hasM2Git = true
				}
				if strings.Contains(m.Content, "M3-SMOKE") {
					hasM3 = true
				}
			}
		}
		if n := len(req.Messages); n > 0 {
			lastRole = string(req.Messages[n-1].Role)
			lastContent = req.Messages[n-1].Content
		}
		fmt.Printf("[mock] tools=%d last_role=%s nToolMsgs=%d\n", len(req.Tools), lastRole, nTools)

		fl, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "no flusher", 500)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")

		switch {
		case len(req.Tools) > 0 && strings.Contains(lastContent, "T2-ROLLBACK"):
			// cross-session rollback probe: only calls rollback (no checkpoint
			// in this run) so it must resolve a PREVIOUS session's ref
			emitCalls(w, fl, []mockCall{{0, "t2-rb", "rollback", `{}`}})
		case len(req.Tools) > 0 && hasM3:
			// relative paths so the same trigger works on any --workspace
			switch nTools {
			case 0: // start: read + write (2 results -> next nTools=2)
				emitCalls(w, fl, []mockCall{
					{0, "m3-read", "read", `{"path": "note.txt"}`},
					{1, "m3-write", "write", `{"path": "m3/probe.txt", "content": "M3-PROBE-LINE"}`},
				})
			case 2: // checkpoint
				emitCalls(w, fl, []mockCall{{0, "m3-ckpt", "checkpoint", `{}`}})
			case 3: // shell
				emitCalls(w, fl, []mockCall{{0, "m3-shell", "shell",
					`{"command": "echo M3-SHELL-OK > m3shell.txt"}`}})
			case 4: // rollback
				emitCalls(w, fl, []mockCall{{0, "m3-rb", "rollback", `{}`}})
			default:
				sseContent(w, fl, "MOCK-FINAL: M3 suite complete.")
			}
		case len(req.Tools) > 0 && hasM2Git:
			switch nTools {
			case 0:
				emitCalls(w, fl, []mockCall{{0, "call-ckpt", "checkpoint", `{}`}})
			case 1:
				emitCalls(w, fl, []mockCall{{0, "call-write2", "write",
					`{"path": "C:\\Users\\user\\ws\\m2extra.txt", "content": "EXTRA-CONTENT"}`}})
			case 2:
				emitCalls(w, fl, []mockCall{{0, "call-rb", "rollback", `{}`}})
			default:
				sseContent(w, fl, "MOCK-FINAL: git sequence complete.")
			}
		case lastRole == string(openai.ChatMessageRoleTool):
			var parts []string
			for _, m := range req.Messages {
				if m.Role == openai.ChatMessageRoleTool {
					c := strings.ReplaceAll(m.Content, "\n", " ")
					if len(c) > 150 {
						c = c[:150]
					}
					parts = append(parts, "<"+c+">")
				}
			}
			final := "MOCK-FINAL: received " + fmt.Sprint(len(parts)) + " tool results: " + strings.Join(parts, " ")
			for _, p := range []string{final[:len(final)/3], final[len(final)/3 : 2*len(final)/3], final[2*len(final)/3:]} {
				sseContent(w, fl, p)
			}
		case len(req.Tools) > 0 && strings.Contains(lastContent, "M1-SMOKE"):
			emitCalls(w, fl, []mockCall{
				{0, "call-read", "read", `{"path": "C:\\Users\\user\\ws\\note.txt"}`},
				{1, "call-time", "get_time", `{}`},
				{2, "call-shell", "shell", `{"command": "echo REDIRECT-FIX > C:\\Users\\user\\ws\\redirect-test.txt"}`},
			})
		case len(req.Tools) > 0 && strings.Contains(lastContent, "M2-FILES"):
			emitCalls(w, fl, []mockCall{
				{0, "call-write", "write", `{"path": "C:\\Users\\user\\ws\\m2\\note2.txt", "content": "M2-LINE-1"}`},
				{1, "call-edit", "edit", `{"path": "C:\\Users\\user\\ws\\m2\\note2.txt", "old_string": "M2-LINE-1", "new_string": "M2-LINE-1-EDITED"}`},
				{2, "call-grep", "grep", `{"pattern": "EDITED"}`},
				{3, "call-glob", "glob", `{"pattern": "m2/*.txt"}`},
				{4, "call-ls", "ls", `{"path": "m2"}`},
			})
		case len(req.Tools) > 0 && strings.Contains(lastContent, "M2-GIT"):
			// unreachable: handled by hasM2Git branch above; kept for safety
			sseContent(w, fl, "MOCK-FINAL: git sequence complete.")
		default:
			for _, p := range []string{"Hello ", "from ", "mock M2."} {
				sseContent(w, fl, p)
			}
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		fl.Flush()
	})
	fmt.Println("[mock] listening 127.0.0.1:8080")
	s := &http.Server{Addr: "127.0.0.1:8080", Handler: mux}
	go func() {
		time.Sleep(time.Duration(seconds) * time.Second)
		s.Close()
	}()
	if err := s.ListenAndServe(); err != nil && err.Error() != "http: Server closed" {
		fmt.Println("[mock] err:", err)
	}
	fmt.Println("[mock] window over")
}

type mockCall struct {
	idx          int
	id, name, ar string
}

func emitCalls(w http.ResponseWriter, fl http.Flusher, calls []mockCall) {
	for _, c := range calls {
		delta := map[string]interface{}{"tool_calls": []interface{}{map[string]interface{}{
			"index": c.idx, "id": c.id, "type": "function",
			"function": map[string]interface{}{"name": c.name, "arguments": c.ar},
		}}}
		writeSSE(w, fl, delta, nil)
	}
	writeSSE(w, fl, map[string]interface{}{}, "tool_calls")
}

func sseContent(w http.ResponseWriter, fl http.Flusher, s string) {
	writeSSE(w, fl, map[string]interface{}{"content": s}, nil)
}

func writeSSE(w http.ResponseWriter, fl http.Flusher, delta map[string]interface{}, finish interface{}) {
	obj := map[string]interface{}{
		"id": "mock-m2", "object": "chat.completion.chunk", "created": time.Now().Unix(),
		"choices": []interface{}{map[string]interface{}{
			"index": 0, "delta": delta, "finish_reason": finish,
		}},
	}
	b, _ := json.Marshal(obj)
	fmt.Fprintf(w, "data: %s\n\n", b)
	fl.Flush()
}

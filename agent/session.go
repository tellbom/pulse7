package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	openai "github.com/sashabaranov/go-openai"
)

// session: append-only .jsonl record of the whole conversation; --resume
// reloads it as context. No database by design.
type session struct {
	f *os.File
}

func openSession(path string) (*session, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &session{f: f}, nil
}

func (s *session) record(m openai.ChatCompletionMessage) {
	if s == nil {
		return
	}
	b, err := json.Marshal(m)
	if err == nil {
		s.f.Write(append(b, '\n'))
	}
}

func (s *session) Close() {
	if s != nil {
		s.f.Close()
	}
}

func loadSession(path string) ([]openai.ChatCompletionMessage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var msgs []openai.ChatCompletionMessage
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		var m openai.ChatCompletionMessage
		if err := json.Unmarshal(sc.Bytes(), &m); err == nil {
			msgs = append(msgs, m)
		}
	}
	msgs, n := pairToolCalls(msgs)
	if n > 0 {
		fmt.Printf("[resume] patched %d interrupted tool call(s) with synthetic results\n", n)
	}
	return msgs, nil
}

// pairToolCalls: after an interrupted run the session tail may contain an
// assistant tool_call without its result, which would make the next API
// request invalid. Append a synthetic tool result right after each unanswered
// call; no crash-window guessing, no auto-retry — judgment stays with the
// LLM and the user.
func pairToolCalls(msgs []openai.ChatCompletionMessage) ([]openai.ChatCompletionMessage, int) {
	answered := map[string]bool{}
	for _, m := range msgs {
		if m.Role == openai.ChatMessageRoleTool {
			answered[m.ToolCallID] = true
		}
	}
	const note = "上一次工具调用因程序中断而未完成，无法确定是否已执行。\n" +
		"请先用只读工具（read / ls / grep）检查当前实际状态，再决定下一步。\n" +
		"若需要重新执行命令或修改文件，请用日常语言向用户说明情况并征求同意，不要直接重试。"
	added := 0
	var out []openai.ChatCompletionMessage
	for _, m := range msgs {
		out = append(out, m)
		if m.Role == openai.ChatMessageRoleAssistant {
			for _, c := range m.ToolCalls {
				if !answered[c.ID] {
					out = append(out, openai.ChatCompletionMessage{
						Role: openai.ChatMessageRoleTool, ToolCallID: c.ID, Content: note,
					})
					added++
				}
			}
		}
	}
	return out, added
}

// manifest: thin task change log (created/modified) written by write/edit.
// Used by rollback to remove only agent-created leftovers — never `git clean`.
type manifest struct {
	path string
}

func (m *manifest) record(op, p string) {
	if m == nil {
		return
	}
	f, err := os.OpenFile(m.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	b, _ := json.Marshal(map[string]string{"op": op, "path": p})
	f.Write(append(b, '\n'))
}

// CreatedPaths returns agent-created paths still on disk.
func (m *manifest) CreatedPaths() []string {
	var out []string
	f, err := os.Open(m.path)
	if err != nil {
		return nil
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e struct{ Op, Path string }
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.Op == "created" {
			if _, err := os.Stat(e.Path); err == nil {
				out = append(out, e.Path)
			}
		}
	}
	return out
}

func (m *manifest) RemoveCreated() int {
	n := 0
	for _, p := range m.CreatedPaths() {
		if os.RemoveAll(p) == nil {
			n++
		}
	}
	return n
}

var _ = fmt.Sprintf

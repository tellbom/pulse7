package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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

// writeMeta stamps a first meta line (workspace) on a FRESH session file so
// --list can show where each session worked (M4-T5).
func (s *session) writeMeta(workspace string) {
	if s == nil {
		return
	}
	if fi, err := s.f.Stat(); err == nil && fi.Size() > 0 {
		return
	}
	if b, err := json.Marshal(map[string]string{"role": "_meta", "workspace": workspace}); err == nil {
		s.f.Write(append(b, '\n'))
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
		if err := json.Unmarshal(sc.Bytes(), &m); err == nil && m.Role != "" && m.Role != "_meta" {
			msgs = append(msgs, m)
		}
	}
	msgs, n := pairToolCalls(msgs)
	if n > 0 {
		fmt.Printf("[resume] patched %d interrupted tool call(s) with synthetic results\n", n)
	}
	return msgs, nil
}

// pairToolCalls: an interrupted run can leave an assistant tool_call without its
// result; append a synthetic tool result so the next API request is valid. No
// window guessing, no auto-retry — judgment stays with the LLM and the user.
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
						Role: openai.ChatMessageRoleTool, ToolCallID: c.ID, Content: note})
					added++
				}
			}
		}
	}
	return out, added
}

// endOfTaskSummary: shell side effects are not covered by git rollback; at
// task end the user gets the list of shell commands this task executed, plus
// a warning when the run stopped at the round cap without a final answer.
func endOfTaskSummary(r *Registry, maxed bool) {
	if r == nil {
		return
	}
	if f, err := os.Open(r.auditPath); err == nil {
		var cmds []string
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			var e struct{ Task, Tool, Args, Ts string }
			var a struct{ Command string }
			if json.Unmarshal(sc.Bytes(), &e) == nil && e.Task == r.taskID && e.Tool == "shell" {
				cmd := e.Args
				if json.Unmarshal([]byte(e.Args), &a) == nil && a.Command != "" {
					cmd = a.Command
				}
				cmds = append(cmds, fmt.Sprintf("  %d. %s  (%s)", len(cmds)+1, cmd, e.Ts))
			}
		}
		f.Close()
		if len(cmds) > 0 {
			fmt.Println("本次任务执行的 shell 命令（不可通过 rollback 回退）：")
			for _, c := range cmds {
				fmt.Println(c)
			}
			fmt.Println("文件改动已存 checkpoint，可 rollback；以上命令的外部影响不可回退。")
		}
	}
	if maxed {
		fmt.Println("[警告] 本次任务达到最大轮次上限后停止，任务很可能未完成。")
	}
}

// manifest: thin task change log (created/modified) written by write/edit.
// Used by rollback to remove only agent-created leftovers — never `git clean`.
type manifest struct {
	path string
}

// sessionInfo is one --list row (M4-T5).
type sessionInfo struct {
	path      string
	mtime     time.Time
	workspace string
	firstUser string
	count     int
}

func listSessions(dir string, limit int) []sessionInfo {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var infos []sessionInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "sess-") || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		fi, err := e.Info()
		if err != nil {
			continue
		}
		info := sessionInfo{path: p, mtime: fi.ModTime()}
		if f, err := os.Open(p); err == nil {
			sc := bufio.NewScanner(f)
			sc.Buffer(make([]byte, 64*1024), 1024*1024)
			for sc.Scan() {
				var m struct {
					Role      string `json:"role"`
					Content   string `json:"content"`
					Workspace string `json:"workspace"`
				}
				if json.Unmarshal(sc.Bytes(), &m) != nil || m.Role == "" || m.Role == "_meta" {
					if m.Role == "_meta" {
						info.workspace = m.Workspace
					}
					continue
				}
				if m.Role == openai.ChatMessageRoleUser && info.firstUser == "" {
					r := []rune(m.Content)
					if len(r) > 60 {
						r = r[:60]
					}
					info.firstUser = string(r)
				}
				info.count++
			}
			f.Close()
		}
		infos = append(infos, info)
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].mtime.After(infos[j].mtime) })
	if len(infos) > limit {
		infos = infos[:limit]
	}
	return infos
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

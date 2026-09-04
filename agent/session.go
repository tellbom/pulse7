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
	return msgs, nil
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

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

type apiMessageSpan struct {
	offset int64
	size   int
}
type apiHistoryIndex struct {
	info  os.FileInfo
	spans []apiMessageSpan
	first string
}

var apiHistoryIndexes = struct {
	sync.Mutex
	entries map[string]*apiHistoryIndex
}{entries: make(map[string]*apiHistoryIndex)}

// Index record positions once per file version; never retain all message bodies.
// Existing malformed/oversize files stay explicit errors, never silently repaired.
func indexAPIHistory(path string) (*apiHistoryIndex, error) {
	journalIOMu.RLock()
	defer journalIOMu.RUnlock()
	return indexAPIHistoryUnlocked(path)
}
func indexAPIHistoryUnlocked(path string) (*apiHistoryIndex, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	apiHistoryIndexes.Lock()
	cached := apiHistoryIndexes.entries[path]
	apiHistoryIndexes.Unlock()
	if cached != nil && unchangedFile(cached.info, info) {
		return cached, nil
	}
	if err := requireCompleteJSONL(path); err != nil {
		return nil, err
	}
	idx := &apiHistoryIndex{info: info}
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 64*1024), maxSessionRecordBytes)
	var pos int64
	line := 0
	for scan.Scan() {
		line++
		raw := scan.Bytes()
		var env struct {
			Role    string `json:"role"`
			System  string `json:"system"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, fmt.Errorf("invalid session JSON at line %d", line)
		}
		switch env.Role {
		case "_meta":
			if line != 1 {
				return nil, errors.New("invalid metadata position")
			}
		case "_clear":
			if env.System == "" {
				return nil, errors.New("invalid clear boundary")
			}
		case "system", "user", "assistant", "tool":
			idx.spans = append(idx.spans, apiMessageSpan{pos, len(raw)})
			if idx.first == "" && env.Role == "user" {
				idx.first = apiPreview(env.Content, 512)
			}
		default:
			return nil, errors.New("invalid session role")
		}
		// Scanner removes CR in CRLF. Track the on-disk newline width.
		pos += int64(len(raw))
		var tail [2]byte
		n, _ := f.ReadAt(tail[:], pos)
		if n > 0 && tail[0] == '\r' {
			pos++
		}
		pos++
	}
	if err := scan.Err(); err != nil {
		return nil, err
	}
	after, err := f.Stat()
	if err != nil || !unchangedFile(info, after) {
		return nil, errors.New("session changed during indexing; retry")
	}
	apiHistoryIndexes.Lock()
	if len(apiHistoryIndexes.entries) >= 128 {
		apiHistoryIndexes.entries = make(map[string]*apiHistoryIndex)
	}
	apiHistoryIndexes.entries[path] = idx
	apiHistoryIndexes.Unlock()
	return idx, nil
}

func apiSessionSummary(path string) (int, string, error) {
	idx, err := indexAPIHistory(path)
	if err != nil {
		return 0, "", err
	}
	return len(idx.spans), idx.first, nil
}
func apiPreview(s string, max int) string {
	if len(s) <= max {
		return s
	}
	for max > 0 && !utf8.RuneStart(s[max]) {
		max--
	}
	return s[:max] + "…"
}
func apiReadSpan(f *os.File, span apiMessageSpan) (json.RawMessage, error) {
	b := make([]byte, span.size)
	_, err := f.ReadAt(b, span.offset)
	return b, err
}
func apiPagedMessages(path string, offset, limit int) ([]json.RawMessage, int, error) {
	journalIOMu.RLock()
	defer journalIOMu.RUnlock()
	idx, err := indexAPIHistoryUnlocked(path)
	if err != nil {
		return nil, 0, err
	}
	count := len(idx.spans)
	if offset < 0 || offset > count {
		return nil, count, errors.New("offset outside history")
	}
	end := offset + limit
	if end > count {
		end = count
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	rows := make([]json.RawMessage, 0, end-offset)
	for _, span := range idx.spans[offset:end] {
		raw, err := apiReadSpan(f, span)
		if err != nil {
			return nil, 0, err
		}
		rows = append(rows, raw)
	}
	info, err := f.Stat()
	if err != nil || !unchangedFile(idx.info, info) {
		return nil, 0, errors.New("session changed during reading; retry")
	}
	return rows, count, nil
}
func apiFindToolResult(path, call string) (sessionMessageRecord, error) {
	idx, err := indexAPIHistory(path)
	if err != nil {
		return sessionMessageRecord{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return sessionMessageRecord{}, err
	}
	defer f.Close()
	for _, span := range idx.spans {
		raw, err := apiReadSpan(f, span)
		if err != nil {
			return sessionMessageRecord{}, err
		}
		var row sessionMessageRecord
		if err := json.Unmarshal(raw, &row); err != nil {
			return row, err
		}
		if row.Role == "tool" && row.ToolCallID == call {
			return row, nil
		}
	}
	return sessionMessageRecord{}, os.ErrNotExist
}
func apiResultRange(s string, offset, limit int) (string, int, bool, error) {
	if offset < 0 || offset > len(s) || limit < 1 || limit > largeContentMaxReadBytes {
		return "", 0, false, errors.New("result range out of bounds")
	}
	if offset < len(s) && !utf8.RuneStart(s[offset]) {
		return "", 0, false, errors.New("offset is not a UTF-8 boundary")
	}
	end := offset + limit
	if end > len(s) {
		end = len(s)
	}
	for end > offset && end < len(s) && !utf8.RuneStart(s[end]) {
		end--
	}
	if end == offset && offset < len(s) {
		return "", 0, false, errors.New("limit splits character")
	}
	return s[offset:end], end, end < len(s), nil
}
func apiResultPagination(rawOffset, rawLimit string) (int, int, error) {
	offset, limit := 0, largeContentPreviewBytes
	var err error
	if rawOffset != "" {
		offset, err = strconv.Atoi(rawOffset)
		if err != nil {
			return 0, 0, err
		}
	}
	if rawLimit != "" {
		limit, err = strconv.Atoi(rawLimit)
		if err != nil {
			return 0, 0, err
		}
	}
	if offset < 0 || limit < 1 || limit > largeContentMaxReadBytes {
		return 0, 0, errors.New("invalid result range")
	}
	return offset, limit, nil
}

// PUT /api/config writes only the user layer. Do not serialize this request
// into the session, audit, events or diagnostic output.
type apiConfigUpdate struct {
	BaseURL *string `json:"base_url"`
	Model   *string `json:"model"`
	APIKey  *string `json:"api_key"`

	MaxCtx                  *int    `json:"max_ctx"`
	MaxRounds               *int    `json:"max_rounds"`
	SkillCatalogBudgetBytes *int    `json:"skill_catalog_budget_bytes"`
	MicroKeepRecent         *int    `json:"micro_keep_recent"`
	ShellTimeoutSec         *int    `json:"shell_timeout_sec"`
	MemoryLimitMB           *int    `json:"memory_limit_mb"`
	ProcessWarnThreshold    *int    `json:"process_warn_threshold"`
	BgTaskMaxOutputMB       *int    `json:"background_task_max_output_mb"`
	BgTaskWarnCount         *int    `json:"background_task_warn_count"`
	BgTaskWarnSec           *int    `json:"background_task_warn_sec"`
	CleanupOnExit           *bool   `json:"cleanup_on_exit"`
	ReadOnly                *bool   `json:"read_only"`
	LLMFirstChunkTimeoutSec *int    `json:"llm_first_chunk_timeout_sec"`
	LLMIdleTimeoutSec       *int    `json:"llm_idle_timeout_sec"`
	LLMMaxRetries           *int    `json:"llm_max_retries"`
	LLMCompressTimeoutSec   *int    `json:"llm_compress_timeout_sec"`
	SandboxPreference       *string `json:"sandbox_preference"`
}

// provided 生成"仅含本次提交字段"的键值映射，saveAPIConfig 据此增量合并落盘。
func (u apiConfigUpdate) provided() map[string]interface{} {
	m := map[string]interface{}{}
	if u.BaseURL != nil {
		m["base_url"] = *u.BaseURL
	}
	if u.Model != nil {
		m["model"] = *u.Model
	}
	if u.APIKey != nil {
		m["api_key"] = *u.APIKey
	}
	for k, v := range map[string]interface{}{
		"max_ctx":                       u.MaxCtx,
		"max_rounds":                    u.MaxRounds,
		"micro_keep_recent":             u.MicroKeepRecent,
		"skill_catalog_budget_bytes":    u.SkillCatalogBudgetBytes,
		"shell_timeout_sec":             u.ShellTimeoutSec,
		"memory_limit_mb":               u.MemoryLimitMB,
		"process_warn_threshold":        u.ProcessWarnThreshold,
		"background_task_max_output_mb": u.BgTaskMaxOutputMB,
		"background_task_warn_count":    u.BgTaskWarnCount,
		"background_task_warn_sec":      u.BgTaskWarnSec,
		"cleanup_on_exit":               u.CleanupOnExit,
		"read_only":                     u.ReadOnly,
		"llm_first_chunk_timeout_sec":   u.LLMFirstChunkTimeoutSec,
		"llm_idle_timeout_sec":          u.LLMIdleTimeoutSec,
		"llm_max_retries":               u.LLMMaxRetries,
		"llm_compress_timeout_sec":      u.LLMCompressTimeoutSec,
		"sandbox_preference":            u.SandboxPreference,
	} {
		switch p := v.(type) {
		case *int:
			if p != nil {
				m[k] = *p
			}
		case *bool:
			if p != nil {
				m[k] = *p
			}
		case *string:
			if p != nil {
				m[k] = *p
			}
		}
	}
	return m
}

func validateAPIConnection(base, model string) error {
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("base_url must be an HTTP(S) endpoint without credentials, query or fragment")
	}
	if strings.TrimSpace(model) == "" {
		return errors.New("model is required")
	}
	return nil
}

func saveAPIConfig(path string, update apiConfigUpdate) error {
	doc, _, err := readConfigDocument(path)
	if err != nil {
		return errors.New("could not read global configuration")
	}
	if doc == nil {
		doc = map[string]json.RawMessage{}
	}
	for k, v := range update.provided() {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		doc[k] = b
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return errors.New("could not encode global configuration")
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return errors.New("could not create global configuration directory")
	}
	f, err := os.CreateTemp(filepath.Dir(path), "config-*.tmp")
	if err != nil {
		return errors.New("could not create global configuration temporary file")
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(append(b, '\n')); err != nil {
		f.Close()
		return errors.New("could not write global configuration")
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return errors.New("could not sync global configuration")
	}
	if err = f.Close(); err != nil {
		return errors.New("could not close global configuration")
	}
	if err = os.Rename(name, path); err != nil {
		return errors.New("could not replace global configuration")
	}
	return nil
}

func apiSessionPath(dir, id string) (string, error) {
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, `/\:`) || strings.ContainsRune(id, 0) {
		return "", errors.New("invalid session id")
	}
	path := filepath.Join(dir, id+".jsonl")
	// session.id strips the standard sess- prefix; custom --session filenames
	// remain supported. Ambiguous identities are errors, not guessed matches.
	prefixed := filepath.Join(dir, "sess-"+id+".jsonl")
	_, plainErr := os.Stat(path)
	_, prefixedErr := os.Stat(prefixed)
	if plainErr == nil && prefixedErr == nil {
		return "", errors.New("ambiguous session id")
	}
	if os.IsNotExist(plainErr) && prefixedErr == nil {
		path = prefixed
	}

	root, _, _, err := resolveExistingPrefix(dir)
	if err != nil {
		return "", err
	}
	resolved, _, _, err := resolveExistingPrefix(path)
	if err != nil {
		return "", err
	}
	if requirePathWithin(root, resolved) != nil {
		return "", errors.New("session resolves outside session directory")
	}
	return resolved, nil
}

// Read persisted messages, not loadSession's synthesized tool results. The
// pagination endpoint must not repair or modify history as a read side effect.
func readAPISessionMessages(path string) ([]json.RawMessage, error) {
	if err := requireCompleteJSONL(path); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rows := []json.RawMessage{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), maxSessionRecordBytes)
	line := 0
	for sc.Scan() {
		line++
		var record struct {
			Role      string          `json:"role"`
			System    string          `json:"system"`
			ToolCalls json.RawMessage `json:"tool_calls"`
		}
		if json.Unmarshal(sc.Bytes(), &record) != nil || record.Role == "" {
			return nil, fmt.Errorf("invalid session record at line %d", line)
		}
		if record.Role == "_meta" {
			if line != 1 {
				return nil, errors.New("invalid session metadata position")
			}
			continue
		}
		if record.Role == "_clear" {
			if record.System == "" {
				return nil, errors.New("invalid session clear boundary")
			}
			continue
		}
		if record.Role != "system" && record.Role != "user" && record.Role != "assistant" && record.Role != "tool" {
			return nil, fmt.Errorf("invalid session role at line %d", line)
		}
		var valid sessionMessageRecord
		if err := json.Unmarshal(sc.Bytes(), &valid); err != nil {
			return nil, fmt.Errorf("invalid session message at line %d", line)
		}
		rows = append(rows, append(json.RawMessage{}, sc.Bytes()...))
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

func decodeAPIJSON(body io.Reader, v interface{}) error {
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return errors.New("invalid JSON request")
	}
	var tail interface{}
	if err := dec.Decode(&tail); err != io.EOF {
		return errors.New("request must contain one JSON value")
	}
	return nil
}

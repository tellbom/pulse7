package main

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// session: append-only .jsonl record of the whole conversation; --resume
// reloads it as context. No database by design.
// T5 (slow-network): creation is LAZY - the file only materializes when the
// first conversation record is written, so a run that dies before any message
// lands (hard kill during connect/queueing) leaves no empty session file
// behind for --list to confuse with real progress.
type session struct {
	f            *os.File
	path         string
	workspace    string
	taskID       string
	manifestPath string
	lastUUID     string
	n            int // conversation records written (meta line not counted)
}

const maxSessionRecordBytes = 1 << 20
const sessionSchemaVersion = 1

var errSessionStorage = errors.New("session storage failure")

func newSession(path, workspace string) *session {
	taskID := sessionTaskID(path)
	return &session{
		path: path, workspace: workspace, taskID: taskID,
		manifestPath: filepath.Join(filepath.Dir(path), "manifest-"+taskID+".jsonl"),
	}
}

type sessionMetadata struct {
	Role         string `json:"role"`
	Workspace    string `json:"workspace"`
	TaskID       string `json:"task_id"`
	ManifestPath string `json:"manifest_path"`
}

type sessionMessageRecord struct {
	ToolOutcome *toolOutcome `json:"toolOutcome,omitempty"`
	UUID        string       `json:"uuid"`
	ParentUUID  *string      `json:"parentUuid"`
	Timestamp   string       `json:"timestamp"`
	SessionID   string       `json:"sessionId"`
	Cwd         string       `json:"cwd"`
	Version     int          `json:"version"`
	openai.ChatCompletionMessage
}

func (r sessionMessageRecord) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(r.ChatCompletionMessage)
	if err != nil {
		return nil, err
	}
	var fields map[string]interface{}
	if err := json.Unmarshal(b, &fields); err != nil {
		return nil, err
	}
	fields["uuid"] = r.UUID
	fields["parentUuid"] = r.ParentUUID
	fields["timestamp"] = r.Timestamp
	fields["sessionId"] = r.SessionID
	fields["cwd"] = r.Cwd
	fields["version"] = r.Version
	if r.ToolOutcome != nil {
		fields["toolOutcome"] = r.ToolOutcome
	}
	return json.Marshal(fields)
}

func (r *sessionMessageRecord) UnmarshalJSON(b []byte) error {
	var metadata struct {
		ToolOutcome *toolOutcome `json:"toolOutcome,omitempty"`
		UUID        string       `json:"uuid"`
		ParentUUID  *string      `json:"parentUuid"`
		Timestamp   string       `json:"timestamp"`
		SessionID   string       `json:"sessionId"`
		Cwd         string       `json:"cwd"`
		Version     int          `json:"version"`
	}
	if err := json.Unmarshal(b, &metadata); err != nil {
		return err
	}
	var message openai.ChatCompletionMessage
	if err := json.Unmarshal(b, &message); err != nil {
		return err
	}
	r.UUID = metadata.UUID
	r.ParentUUID = metadata.ParentUUID
	r.Timestamp = metadata.Timestamp
	r.SessionID = metadata.SessionID
	r.Cwd = metadata.Cwd
	r.Version = metadata.Version
	r.ToolOutcome = metadata.ToolOutcome
	r.ChatCompletionMessage = message
	return nil
}

func sessionTaskID(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return strings.TrimPrefix(base, "sess-")
}

func writeJSONLine(f osFileWriter, value interface{}) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	n, err := f.Write(b)
	if err != nil {
		return err
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	return f.Sync()
}

func appendJSONLine(path string, value interface{}) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if err := writeJSONLine(f, value); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

type osFileWriter interface {
	Write([]byte) (int, error)
	Sync() error
}

// open materializes the file (append mode) and stamps the workspace meta
// line on a fresh file so --list can show where each session worked.
func (s *session) open() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	if fi.Size() == 0 {
		if err := writeJSONLine(f, sessionMetadata{
			Role: "_meta", Workspace: s.workspace, TaskID: s.taskID, ManifestPath: s.manifestPath,
		}); err != nil {
			f.Close()
			return fmt.Errorf("write session metadata: %w", err)
		}
	} else {
		lastUUID, err := lastSessionMessageUUID(s.path)
		if err != nil {
			f.Close()
			return err
		}
		s.lastUUID = lastUUID
	}
	s.f = f
	return nil
}

// file returns the backing path ("" until open).
func (s *session) file() string {
	if s == nil {
		return ""
	}
	return s.path
}

// id returns the session id used by --resume (filename stem, sess- prefix
// stripped; resolveResume accepts both shapes).
func (s *session) id() string {
	base := filepath.Base(s.file())
	return strings.TrimSuffix(strings.TrimPrefix(base, "sess-"), filepath.Ext(base))
}

func (s *session) record(m openai.ChatCompletionMessage, outcome ...*toolOutcome) error {
	if s == nil {
		return fmt.Errorf("%w: session is not initialized", errSessionStorage)
	}
	if s.f == nil {
		if err := s.open(); err != nil {
			return fmt.Errorf("%w: open session: %v", errSessionStorage, err)
		}
	}
	uuid, err := newMessageUUID()
	if err != nil {
		return fmt.Errorf("%w: create message uuid: %v", errSessionStorage, err)
	}
	cwd, err := filepath.Abs(s.workspace)
	if err != nil {
		return fmt.Errorf("%w: resolve session cwd: %v", errSessionStorage, err)
	}
	var parent *string
	if s.lastUUID != "" {
		value := s.lastUUID
		parent = &value
	}
	record := sessionMessageRecord{
		UUID: uuid, ParentUUID: parent, Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		SessionID: s.id(), Cwd: cwd, Version: sessionSchemaVersion, ChatCompletionMessage: m,
	}
	if m.Role == openai.ChatMessageRoleTool && len(outcome) > 0 {
		record.ToolOutcome = outcome[0]
	}
	if err := writeJSONLine(s.f, record); err != nil {
		return fmt.Errorf("%w: write session record: %v", errSessionStorage, err)
	}
	s.lastUUID = uuid
	s.n++
	return nil
}

func newMessageUUID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16]), nil
}

func lastSessionMessageUUID(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	lastUUID := ""
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), maxSessionRecordBytes)
	line := 0
	for scanner.Scan() {
		line++
		var record struct {
			Role string `json:"role"`
			UUID string `json:"uuid"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return "", fmt.Errorf("invalid session record at line %d: %w", line, err)
		}
		if record.Role != "_meta" && record.Role != "_clear" {
			lastUUID = record.UUID
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("session record exceeds %d-byte limit or could not be read: %w", maxSessionRecordBytes, err)
	}
	return lastUUID, nil
}

type sessionClearRecord struct {
	Role   string `json:"role"`
	System string `json:"system"`
	Time   string `json:"time"`
}

func (s *session) recordClear(system string) error {
	if s == nil {
		return fmt.Errorf("%w: session is not initialized", errSessionStorage)
	}
	if s.f == nil {
		if err := s.open(); err != nil {
			return fmt.Errorf("%w: open session: %v", errSessionStorage, err)
		}
	}
	if err := writeJSONLine(s.f, sessionClearRecord{
		Role: "_clear", System: system, Time: time.Now().Format(time.RFC3339),
	}); err != nil {
		return fmt.Errorf("%w: write session clear boundary: %v", errSessionStorage, err)
	}
	s.n++
	return nil
}

func (s *session) Close() error {
	if s != nil && s.f != nil {
		err := s.f.Close()
		s.f = nil
		return err
	}
	return nil
}

func requireCompleteJSONL(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	if fi.Size() == 0 {
		return nil
	}
	if _, err := f.Seek(-1, io.SeekEnd); err != nil {
		return err
	}
	var last [1]byte
	if _, err := io.ReadFull(f, last[:]); err != nil {
		return err
	}
	if last[0] != '\n' {
		return errors.New("session is truncated: final record has no newline")
	}
	return nil
}

func loadSession(path string) ([]openai.ChatCompletionMessage, error) {
	if err := requireCompleteJSONL(path); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var msgs []openai.ChatCompletionMessage
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), maxSessionRecordBytes)
	line := 0
	for sc.Scan() {
		line++
		var envelope struct {
			Role   string `json:"role"`
			System string `json:"system"`
		}
		if err := json.Unmarshal(sc.Bytes(), &envelope); err != nil {
			return nil, fmt.Errorf("invalid session record at line %d: %w", line, err)
		}
		if envelope.Role == "_meta" {
			if line != 1 {
				return nil, fmt.Errorf("invalid session metadata position at line %d", line)
			}
			continue
		}
		if envelope.Role == "_clear" {
			if envelope.System == "" {
				return nil, fmt.Errorf("invalid clear boundary at line %d: missing system prompt", line)
			}
			msgs = []openai.ChatCompletionMessage{{
				Role: openai.ChatMessageRoleSystem, Content: envelope.System,
			}}
			continue
		}
		var m openai.ChatCompletionMessage
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			return nil, fmt.Errorf("invalid session message at line %d: %w", line, err)
		}
		if m.Role == "" {
			return nil, fmt.Errorf("invalid session message at line %d: missing role", line)
		}
		msgs = append(msgs, m)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("session record exceeds %d-byte limit or could not be read: %w", maxSessionRecordBytes, err)
	}
	msgs, n := pairToolCalls(msgs)
	if n > 0 {
		out("[resume] patched %d interrupted tool call(s) with synthetic results\n", n)
	}
	return msgs, nil
}

func loadSessionMetadata(path string) (sessionMetadata, error) {
	if err := requireCompleteJSONL(path); err != nil {
		return sessionMetadata{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return sessionMetadata{}, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), maxSessionRecordBytes)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return sessionMetadata{}, err
		}
		return sessionMetadata{}, errors.New("session has no metadata record")
	}
	var meta sessionMetadata
	if err := json.Unmarshal(sc.Bytes(), &meta); err != nil {
		return sessionMetadata{}, fmt.Errorf("invalid session metadata: %w", err)
	}
	if meta.Role != "_meta" || strings.TrimSpace(meta.Workspace) == "" {
		return sessionMetadata{}, errors.New("session is missing its workspace metadata")
	}
	fileTaskID := sessionTaskID(path)
	if meta.TaskID == "" {
		meta.TaskID = fileTaskID // backward compatibility for pre-A6 sessions
	}
	if meta.TaskID != fileTaskID {
		return sessionMetadata{}, fmt.Errorf("session task identity mismatch: metadata=%s filename=%s", meta.TaskID, fileTaskID)
	}
	return meta, nil
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

// taskEndState (T3 output-layering): one-line status + stats for the
// terminal block. status is one of: 已完成 / 需要回答 / 出错中止 / 已中止.
type taskEndState struct {
	status  string
	rounds  int
	elapsed time.Duration
}

// printTaskEnd: the single terminal block users scan for. Merges the
// historic endOfTaskSummary (shell side-effect list + checkpoint note)
// so nothing is printed twice.
func printTaskEnd(r *Registry, maxed bool, st taskEndState) {
	out("========================================\n")
	out("任务结束：%s（共 %d 轮，耗时 %v）\n", st.status, st.rounds, st.elapsed.Round(time.Second))
	endOfTaskSummaryLines(r)
	if maxed {
		out("[警告] 本次任务达到最大轮次上限后停止，任务很可能未完成。\n")
	}
	out("========================================\n")
}

// endOfTaskSummary: shell side effects are not covered by git rollback; at
// task end the user gets the list of shell commands this task executed, plus
// a warning when the run stopped at the round cap without a final answer.
func endOfTaskSummary(r *Registry, maxed bool) {
	if r == nil {
		return
	}
	endOfTaskSummaryLines(r)
	if maxed {
		outln("[警告] 本次任务达到最大轮次上限后停止，任务很可能未完成。")
	}
}

func endOfTaskSummaryLines(r *Registry) {
	if r == nil {
		return
	}
	paths, err := auditOutsideWrites(r.auditPath, r.taskID)
	if err != nil {
		outln("[存储错误] 无法读取工作区外写入审计：", err)
		return
	}
	if len(paths) > 0 {
		outln("本次任务在工作区外写入的文件：")
		for i, path := range paths {
			out("  %d. %s\n", i+1, path)
		}
		outln("以上不在 checkpoint 覆盖范围内，无法通过 rollback 回退。")
	}
	cmds, err := auditShellCommands(r.auditPath, r.taskID)
	if err != nil {
		outln("[存储错误] 无法读取 shell 审计记录：", err)
		return
	}
	if len(cmds) > 0 {
		outln("本次任务执行的 shell 命令（不可通过 rollback 回退）：")
		for _, c := range cmds {
			outln(c)
		}
		outln("文件改动已存 checkpoint，可 rollback；以上命令的外部影响不可回退。")
	}
}

func auditShellCommands(auditPath, taskID string) ([]string, error) {
	f, err := os.Open(auditPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cmds []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e struct{ Task, Tool, Args, Ts, Event string }
		var a struct{ Command string }
		if json.Unmarshal(sc.Bytes(), &e) != nil || e.Event != "" || e.Task != taskID || e.Tool != "shell" {
			continue
		}
		cmd := e.Args
		if json.Unmarshal([]byte(e.Args), &a) == nil && a.Command != "" {
			cmd = a.Command
		}
		cmds = append(cmds, fmt.Sprintf("  %d. %s  (%s)", len(cmds)+1, cmd, e.Ts))
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return cmds, nil
}

// manifest: thin task change log (created/modified) written by write/edit.
// Used by rollback to remove only agent-created leftovers — never `git clean`.
type manifest struct {
	path string
}

type manifestEntry struct {
	Op              string `json:"op"`
	Path            string `json:"path"`
	AfterCheckpoint int    `json:"after_checkpoint"`
	SHA256          string `json:"sha256"`
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
	if limit > 0 && len(infos) > limit {
		infos = infos[:limit]
	}
	return infos
}

func (m *manifest) record(op, p string, afterCheckpoint int) error {
	if m == nil {
		return errors.New("manifest is not initialized")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(b)
	entry := manifestEntry{
		Op: op, Path: p, AfterCheckpoint: afterCheckpoint,
		SHA256: fmt.Sprintf("%x", sum[:]),
	}
	f, err := os.OpenFile(m.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if err := writeJSONLine(f, entry); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// ChangesAfter returns the last Agent write to every path changed after the
// target checkpoint. Target-tree classification and current-file validation
// happen before rollback in toolRollback.
func (m *manifest) ChangesAfter(targetSeq int) ([]manifestEntry, error) {
	if m == nil {
		return nil, nil
	}
	f, err := os.Open(m.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	latest := map[string]manifestEntry{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e manifestEntry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			return nil, fmt.Errorf("invalid manifest entry: %w", err)
		}
		if (e.Op == "modified" || e.Op == "created") && e.AfterCheckpoint >= targetSeq {
			latest[e.Path] = e
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(latest))
	for p := range latest {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	entries := make([]manifestEntry, 0, len(paths))
	for _, p := range paths {
		entries = append(entries, latest[p])
	}
	return entries, nil
}

func manifestEntryMatchesFile(e manifestEntry) error {
	fi, err := os.Lstat(e.Path)
	if err != nil {
		return fmt.Errorf("rollback conflict: %s changed after Agent write (%v)", e.Path, err)
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("rollback conflict: %s changed after Agent write (expected a regular file, found %s)", e.Path, fi.Mode())
	}
	b, err := os.ReadFile(e.Path)
	if err != nil {
		return fmt.Errorf("rollback conflict: %s changed after Agent write (%v)", e.Path, err)
	}
	sum := sha256.Sum256(b)
	if e.SHA256 == "" || !strings.EqualFold(e.SHA256, fmt.Sprintf("%x", sum[:])) {
		return fmt.Errorf("rollback conflict: %s changed after Agent write", e.Path)
	}
	return nil
}

func verifyRollbackPath(g *gitOps, target, path string, targetHasPath bool) error {
	if !targetHasPath {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			return fmt.Errorf("rollback verification failed: %s should be absent (stat error: %v)", path, err)
		}
		return nil
	}
	want, err := g.targetBlob(target, path)
	if err != nil {
		return err
	}
	got, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("rollback verification failed: read %s: %w", path, err)
	}
	if !strings.EqualFold(fileSHA256(got), fileSHA256(want)) {
		return fmt.Errorf("rollback verification failed: %s does not match target tree", path)
	}
	return nil
}

func fileSHA256(b []byte) string {
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:])
}

var _ = fmt.Sprintf

func auditOutsideWrites(path, task string) ([]string, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	paths := []string{}
	seen := map[string]bool{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), maxSessionRecordBytes)
	for sc.Scan() {
		var e struct {
			Event string `json:"event"`
			Task  string `json:"task"`
			Path  string `json:"resolvedPath"`
		}
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			return nil, err
		}
		if e.Event == "outside_workspace_write" && e.Task == task && !seen[fileReadStateKey(e.Path)] {
			paths = append(paths, e.Path)
			seen[fileReadStateKey(e.Path)] = true
		}
	}
	return paths, sc.Err()
}

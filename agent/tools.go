package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// Policy: workspace path allowlist.
type Policy struct {
	Workspace string
}

func (p *Policy) Check(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(p.Workspace, abs)
	if err != nil {
		return fmt.Errorf("path %s outside workspace %s", path, p.Workspace)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %s is outside workspace %s", path, p.Workspace)
	}
	return nil
}

type Tool struct {
	Def openai.Tool
	Fn  func(args string) (string, error)
}

type Registry struct {
	tools     map[string]Tool
	order     []string
	policy    *Policy
	runner    sandboxRunner
	auditPath string
	manPath   string
	yolo      bool
	execMode  bool
	stdin     io.Reader
	exeDir    string
	workspace string
	taskID    string
	man       *manifest
	git       *gitOps
}

func NewRegistry(policy *Policy, runner sandboxRunner, auditPath, manPath string,
	yolo, execMode bool, stdin io.Reader, exeDir, workspace, taskID string) *Registry {
	r := &Registry{
		tools: map[string]Tool{}, policy: policy, runner: runner,
		auditPath: auditPath, manPath: manPath, yolo: yolo, execMode: execMode,
		stdin: stdin, exeDir: exeDir, workspace: workspace, taskID: taskID,
		man: &manifest{path: manPath},
	}
	r.register(openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name: "read", Description: "Read a text file inside the workspace. Long files are paginated: the result is annotated with the line range and total, and names the next offset while lines remain - page through large files with offset/limit instead of re-reading the whole file.",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"path":   map[string]interface{}{"type": "string"},
				"offset": map[string]interface{}{"type": "integer", "description": "1-based start line (default 1)"},
				"limit":  map[string]interface{}{"type": "integer", "description": "number of lines to read (default: auto, ~4KB window)"},
			}, "required": []string{"path"}},
		},
	}, r.toolRead)
	r.register(openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name: "get_time", Description: "Get the local time of this Windows machine. No arguments.",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		},
	}, r.toolGetTime)
	r.register(openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name: "shell", Description: "Run a cmd.exe command (sandboxed). Working dir is the workspace. Returns exit code and output.",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"command": map[string]interface{}{"type": "string"},
			}, "required": []string{"command"}},
		},
	}, r.toolShell)
	r.register(openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name: "write", Description: "Create or overwrite a text file inside the workspace. ONLY write what the user explicitly asked for - do not create extra files, restructure, or 'improve' beyond the request.",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"path":    map[string]interface{}{"type": "string"},
				"content": map[string]interface{}{"type": "string"},
			}, "required": []string{"path", "content"}},
		},
	}, r.toolWrite)
	r.register(openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name: "edit", Description: "Replace an exact unique string in a file. Fails if not found or ambiguous. ONLY make the change the user asked for - no opportunistic refactors or renames.",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"path": map[string]interface{}{"type": "string"},
				"old_string": map[string]interface{}{"type": "string"},
				"new_string": map[string]interface{}{"type": "string"},
			}, "required": []string{"path", "old_string", "new_string"}},
		},
	}, r.toolEdit)
	r.register(openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name: "tree", Description: "Show project directory structure as a tree. Use this FIRST to understand project layout instead of calling ls repeatedly. Directories show child count. Skips .git/node_modules/vendor/dist/build/target/__pycache__/.venv/bin/obj and hidden dirs.",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"path":       map[string]interface{}{"type": "string", "description": "starting directory (default: workspace root)"},
				"max_depth":  map[string]interface{}{"type": "integer", "description": "max directory depth (default: 3)"},
				"max_entries": map[string]interface{}{"type": "integer", "description": "max lines of output (default: 300)"},
			}},
		},
	}, r.toolTree)
	r.register(openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name: "grep", Description: "Search text across workspace files using ripgrep (fast) or Go fallback. Supports regex, case sensitivity, and file type filter. Returns file:line matches.",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"pattern": map[string]interface{}{"type": "string"},
				"path":    map[string]interface{}{"type": "string", "description": "optional sub path"},
				"glob":    map[string]interface{}{"type": "string", "description": "file glob filter, e.g. *.cs"},
			}, "required": []string{"pattern"}},
		},
	}, r.toolGrep)
	r.register(openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name: "glob", Description: "List files matching a glob pattern relative to the workspace.",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"pattern": map[string]interface{}{"type": "string"},
			}, "required": []string{"pattern"}},
		},
	}, r.toolGlob)
	r.register(openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name: "ls", Description: "List directory entries.",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"path": map[string]interface{}{"type": "string"},
			}},
		},
	}, r.toolLs)
	r.register(openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name: "checkpoint", Description: "Snapshot the workspace via bundled git (private refs, user history untouched).",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		},
	}, r.toolCheckpoint)
	r.register(openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name: "rollback", Description: "Restore the workspace to the latest (or given) checkpoint; removes agent-created leftovers.",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"to": map[string]interface{}{"type": "integer", "description": "checkpoint sequence, default latest"},
			}},
		},
	}, r.toolRollback)
	return r
}

func (r *Registry) register(def openai.Tool, fn func(string) (string, error)) {
	r.tools[def.Function.Name] = Tool{Def: def, Fn: fn}
	r.order = append(r.order, def.Function.Name)
}

func (r *Registry) Definitions() []openai.Tool {
	out := make([]openai.Tool, 0, len(r.order))
	for _, n := range r.order {
		out = append(out, r.tools[n].Def)
	}
	return out
}

func (r *Registry) Execute(name, argsJSON string) string {
	t, ok := r.tools[name]
	var res string
	if !ok {
		res = "error: unknown tool " + name
	} else {
		out, err := t.Fn(argsJSON)
		if err != nil {
			res = "error: " + err.Error()
		} else {
			res = out
		}
	}
	r.audit(name, argsJSON, res)
	return res
}

func (r *Registry) audit(tool, args, res string) {
	f, err := os.OpenFile(r.auditPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	entry := map[string]interface{}{
		"ts": time.Now().Format(time.RFC3339), "task": r.taskID, "tool": tool, "args": args,
		"ok": !strings.HasPrefix(res, "error:"), "result_bytes": len(res),
	}
	b, _ := json.Marshal(entry)
	f.Write(append(b, '\n'))
}

// absPath resolves tool paths: relative paths are workspace-rooted, then
// checked against the allowlist.
func (r *Registry) absPath(p string) (string, error) {
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.workspace, p)
	}
	if err := r.policy.Check(p); err != nil {
		return "", err
	}
	return filepath.Abs(p)
}

func (r *Registry) toolRead(argsJSON string) (string, error) {
	var a struct {
		Path   string `json:"path"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", err
	}
	abs, err := r.absPath(a.Path)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	return readPage(string(b), a.Offset, a.Limit), nil
}

// readPage: line-based pagination for the read tool (encoding-pagination T3).
// offset is 1-based; limit<=0 means "auto" (as many lines as the historic
// 4096-byte cap allows). Output is annotated with the line range and the
// total, and while lines remain it names the next offset to continue with.
// An offset beyond the end yields an explicit note, never an error.
func readPage(content string, offset, limit int) string {
	if content == "" {
		return "[empty file]\n"
	}
	lines := strings.Split(content, "\n")
	// A trailing "\n" produces one phantom empty element; it is not a line.
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	total := len(lines)
	auto := limit <= 0
	if offset <= 0 {
		offset = 1
	}
	if offset > total {
		return fmt.Sprintf("[file has %d line(s); offset %d is beyond the end]\n", total, offset)
	}
	n := total - offset + 1 // lines available from offset
	if auto {
		// historic default: from offset, take as many lines as fit 4096 bytes
		size := 0
		n = 0
		for i := offset - 1; i < total; i++ {
			if size+len(lines[i])+1 > 4096 && n > 0 {
				break
			}
			size += len(lines[i]) + 1
			n++
		}
	} else if limit > maxReadLines {
		limit = maxReadLines
	}
	if !auto && limit < n {
		n = limit
	}
	last := offset + n - 1 // 1-based inclusive last line
	var sb strings.Builder
	fmt.Fprintf(&sb, "[第 %d-%d 行，共 %d 行]\n", offset, last, total)
	for i := offset - 1; i <= last-1; i++ {
		sb.WriteString(lines[i])
		sb.WriteByte('\n')
	}
	if last < total {
		fmt.Fprintf(&sb, "[...还有 %d 行未读；继续读取请传 offset=%d]\n", total-last, last+1)
	}
	return sb.String()
}

// maxReadLines caps an explicit limit so one call cannot dump a huge file
// into the context; the annotation tells the model how to page further.
const maxReadLines = 2000

func (r *Registry) toolGetTime(string) (string, error) {
	return time.Now().Format(time.RFC3339), nil
}

func (r *Registry) toolShell(argsJSON string) (string, error) {
	var a struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", err
	}
	if a.Command == "" {
		return "", errors.New("empty command")
	}
	if blocked, why := gitWriteBlocked(a.Command); blocked {
		return "", errors.New(why)
	}
	if !r.confirm(a.Command) {
		return "", errors.New("denied by user (confirmation)")
	}
	out, ec, err := r.runner.Run(a.Command)
	if err != nil {
		return fmt.Sprintf("exitcode=%d\n[sandbox=%s]\n%s\nsandbox-error: %v", ec, r.runner.Mode(), out, err), nil
	}
	return fmt.Sprintf("exitcode=%d\n[sandbox=%s]\n%s", ec, r.runner.Mode(), out), nil
}

// diffSummary renders a bounded line diff (M4-T2): common prefix/suffix are
// aligned, the changed middle gets +/- markers with 2 context lines on each
// side, output is capped at cap lines with an overflow notice.
func diffSummary(oldS, newS string, cap int) string {
	norm := func(s string) []string { return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") }
	o, n := norm(oldS), norm(newS)
	pre := 0
	for pre < len(o) && pre < len(n) && o[pre] == n[pre] {
		pre++
	}
	suf := 0
	for suf < len(o)-pre && suf < len(n)-pre && o[len(o)-1-suf] == n[len(n)-1-suf] {
		suf++
	}
	lo := pre - 2
	if lo < 0 {
		lo = 0
	}
	hiN := len(n) - suf + 2
	if hiN > len(n) {
		hiN = len(n)
	}
	var b []string
	if lo > 0 {
		b = append(b, fmt.Sprintf("  (前略 %d 行未变)", lo))
	}
	for i := lo; i < pre; i++ {
		b = append(b, "  "+o[i])
	}
	for i := pre; i < len(o)-suf; i++ {
		b = append(b, "- "+o[i])
	}
	for i := pre; i < len(n)-suf; i++ {
		b = append(b, "+ "+n[i])
	}
	for i := len(n) - suf; i < hiN; i++ {
		b = append(b, "  "+n[i])
	}
	overflow := 0
	if len(b) > cap {
		overflow = len(b) - cap
		b = b[:cap]
	}
	out := strings.Join(b, "\n")
	if overflow > 0 {
		out += fmt.Sprintf("\n... 另有 %d 行改动未显示", overflow)
	}
	return out
}

func (r *Registry) toolWrite(argsJSON string) (string, error) {
	var a struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", err
	}
	abs, err := r.absPath(a.Path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	_, statErr := os.Stat(abs)
	existed := statErr == nil
	var oldText string
	if existed {
		if b, err := os.ReadFile(abs); err == nil {
			oldText = string(b)
		}
	}
	if err := os.WriteFile(abs, []byte(a.Content), 0644); err != nil {
		return "", err
	}
	if existed {
		r.man.record("modified", abs)
	} else {
		r.man.record("created", abs)
	}
	if !existed {
		lines := strings.Count(a.Content, "\n") + 1
		preview := strings.Split(strings.TrimRight(a.Content, "\n"), "\n")
		if len(preview) > 5 {
			preview = append(preview[:5], "...")
		}
		return fmt.Sprintf("created %s (%d 行)\n预览:\n%s", abs, lines, strings.Join(preview, "\n")), nil
	}
	if oldText != a.Content {
		return fmt.Sprintf("modified %s\n%s", abs, diffSummary(oldText, a.Content, 40)), nil
	}
	return fmt.Sprintf("modified %s (内容未变化)", abs), nil
}

func (r *Registry) toolEdit(argsJSON string) (string, error) {
	var a struct {
		Path      string `json:"path"`
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", err
	}
	if a.OldString == "" {
		return "", errors.New("old_string is empty")
	}
	abs, err := r.absPath(a.Path)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	original := string(b)
	// T1 (final-fix): normalize line endings for matching, but PRESERVE the
	// file's original style on write-back. Windows files are CRLF; the model
	// sends \n. Matching \n against \r\n fails, causing re-edit loops.
	normFile := strings.ReplaceAll(original, "\r\n", "\n")
	normOld := strings.ReplaceAll(a.OldString, "\r\n", "\n")
	normNew := strings.ReplaceAll(a.NewString, "\r\n", "\n")
	switch strings.Count(normFile, normOld) {
	case 0:
		return "", errors.New("old_string not found")
	}
	if strings.Count(normFile, normOld) > 1 {
		return "", errors.New("old_string is ambiguous (multiple occurrences)")
	}
	// do the replacement in normalized space, then restore original style
	replaced := strings.Replace(normFile, normOld, normNew, 1)
	result := replaced
	if strings.Contains(original, "\r\n") {
		result = strings.ReplaceAll(replaced, "\n", "\r\n")
	}
	note := ""
	if strings.Contains(original, "\r\n") && strings.Contains(original, "\n") &&
		!strings.HasSuffix(original, "\n") {
		note = "\n(注：文件含混合换行符，已按 CRLF 风格写回)"
	}
	if err := os.WriteFile(abs, []byte(result), 0644); err != nil {
		return "", err
	}
	r.man.record("modified", abs)
	return fmt.Sprintf("replaced 1 occurrence in %s\n%s%s", abs, diffSummary(original, result, 40), note), nil
}

// toolGrep moved to grep.go (T2 retrieval: ripgrep with Go fallback)

func (r *Registry) toolGlob(argsJSON string) (string, error) {
	var a struct {
		Pattern string `json:"pattern"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", err
	}
	matches, _ := filepath.Glob(filepath.Join(r.workspace, a.Pattern))
	if len(matches) > 200 {
		matches = matches[:200]
	}
	if len(matches) == 0 {
		return "no matches", nil
	}
	return strings.Join(matches, "\n"), nil
}

func (r *Registry) toolLs(argsJSON string) (string, error) {
	var a struct {
		Path string `json:"path"`
	}
	json.Unmarshal([]byte(argsJSON), &a)
	root := r.workspace
	if a.Path != "" {
		p, err := r.absPath(a.Path)
		if err != nil {
			return "", err
		}
		root = p
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	var out []string
	for i, e := range entries {
		if i >= 200 {
			break
		}
		if e.IsDir() {
			out = append(out, "d "+e.Name())
		} else {
			fi, _ := e.Info()
			out = append(out, fmt.Sprintf("- %s (%d B)", e.Name(), fi.Size()))
		}
	}
	return strings.Join(out, "\n"), nil
}

func (r *Registry) ensureGit() (*gitOps, error) {
	if r.git == nil {
		g, err := newGitOps(r.exeDir, r.workspace, r.taskID)
		if err != nil {
			return nil, err
		}
		r.git = g
	}
	return r.git, nil
}

func (r *Registry) toolCheckpoint(string) (string, error) {
	g, err := r.ensureGit()
	if err != nil {
		return "", err
	}
	return g.Checkpoint()
}

func (r *Registry) toolRollback(argsJSON string) (string, error) {
	var a struct{ To int }
	json.Unmarshal([]byte(argsJSON), &a)
	g, err := r.ensureGit()
	if err != nil {
		return "", err
	}
	res, err := g.Rollback(a.To)
	if err != nil {
		return "", err
	}
	removed := r.man.RemoveCreated()
	return fmt.Sprintf("%s; manifest cleanup removed %d agent-created files", res, removed), nil
}

// gitWriteBlocked rejects git MUTATING subcommands through the shell tool
// (they bypass the private-ref checkpoint design; a push is unrecoverable).
// Read-only subcommands pass. Covers "git -C <path> commit" and
// "git --git-dir=... push"; compound lines split on &, | and newlines.
var gitWriteSubs = map[string]bool{"commit": true, "push": true, "reset": true, "checkout": true,
	"merge": true, "rebase": true, "cherry-pick": true, "clean": true, "stash": true}

const gitBlockedMsg = "该 git 写操作已被 pulse7 禁止（会绕过 checkpoint 保护，push 后无法回退）。请用日常语言向用户说明，由用户自行在终端执行。"

func gitWriteBlocked(cmdline string) (bool, string) {
	for _, part := range strings.FieldsFunc(strings.ToLower(cmdline),
		func(r rune) bool { return r == '&' || r == '|' || r == '\n' || r == '\r' }) {
		f := strings.Fields(part)
		for i := 0; i < len(f); i++ {
			tok := f[i]
			if tok != "git" && !strings.HasSuffix(tok, `\git.exe`) && !strings.HasSuffix(tok, "/git") && tok != "git.exe" {
				continue
			}
			j := i + 1
			for j < len(f) { // skip global options before the subcommand
				a := f[j]
				if a == "-c" || a == "-C" || a == "--git-dir" || a == "--work-tree" || a == "--namespace" {
					j++ // value follows on the next token
				} else if !strings.HasPrefix(a, "-") {
					break
				}
				j++
			}
			if j < len(f) && gitWriteSubs[f[j]] {
				return true, gitBlockedMsg + "（被拦截: git " + f[j] + "）"
			}
		}
	}
	return false, ""
}

func (r *Registry) confirm(command string) bool {
	if r.yolo {
		fmt.Printf("[confirm] AUTO-ALLOWED (--yolo): %s\n", command)
		return true
	}
	if r.execMode {
		fmt.Printf("[confirm] DENIED (exec mode requires --yolo for shell): %s\n", command)
		return false
	}
	fmt.Printf("[confirm] run shell: %s\n[y/N]? ", command)
	var line string
	fmt.Fscanln(r.stdin, &line)
	return strings.EqualFold(strings.TrimSpace(line), "y")
}

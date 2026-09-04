package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
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
			Name: "read", Description: "Read a text file inside the workspace.",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"path": map[string]interface{}{"type": "string"},
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
			Name: "write", Description: "Create or overwrite a text file inside the workspace.",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"path":    map[string]interface{}{"type": "string"},
				"content": map[string]interface{}{"type": "string"},
			}, "required": []string{"path", "content"}},
		},
	}, r.toolWrite)
	r.register(openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name: "edit", Description: "Replace an exact unique string in a file. Fails if not found or ambiguous.",
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
			Name: "grep", Description: "Regex search across workspace files. Returns file:line matches.",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"pattern": map[string]interface{}{"type": "string"},
				"path":    map[string]interface{}{"type": "string", "description": "optional sub path"},
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
		Path string `json:"path"`
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
	if len(b) > 4096 {
		b = b[:4096]
	}
	return string(b), nil
}

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
	if err := os.WriteFile(abs, []byte(a.Content), 0644); err != nil {
		return "", err
	}
	op := "modified"
	if !existed {
		op = "created"
		r.man.record("created", abs)
	} else {
		r.man.record("modified", abs)
	}
	return fmt.Sprintf("%s %s (%d bytes)", op, abs, len(a.Content)), nil
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
	switch strings.Count(string(b), a.OldString) {
	case 0:
		return "", errors.New("old_string not found")
	}
	if strings.Count(string(b), a.OldString) > 1 {
		return "", errors.New("old_string is ambiguous (multiple occurrences)")
	}
	nb := strings.Replace(string(b), a.OldString, a.NewString, 1)
	if err := os.WriteFile(abs, []byte(nb), 0644); err != nil {
		return "", err
	}
	r.man.record("modified", abs)
	return fmt.Sprintf("replaced 1 occurrence in %s", abs), nil
}

func (r *Registry) toolGrep(argsJSON string) (string, error) {
	var a struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", err
	}
	re, err := regexp.Compile(a.Pattern)
	if err != nil {
		return "", fmt.Errorf("bad regex: %v", err)
	}
	root := r.workspace
	if a.Path != "" {
		root, err = r.absPath(a.Path)
		if err != nil {
			return "", err
		}
	}
	var lines []string
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() && d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || len(lines) >= 50 {
			return nil
		}
		if fi, _ := d.Info(); fi != nil && fi.Size() > 1024*1024 {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(r.workspace, p)
		for i, line := range strings.Split(string(b), "\n") {
			if re.MatchString(line) {
				lines = append(lines, fmt.Sprintf("%s:%d: %s", rel, i+1, strings.TrimSpace(line)))
				if len(lines) >= 50 {
					break
				}
			}
		}
		return nil
	})
	if len(lines) == 0 {
		return "no matches", nil
	}
	return strings.Join(lines, "\n"), nil
}

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

// gitWriteBlocked rejects git MUTATING subcommands through the shell tool so
// the private-ref checkpoint design cannot be bypassed (a push is simply
// unrecoverable). Read-only subcommands pass. Covers global args like
// "git -C <path> commit" and "git --git-dir=... push"; compound lines are
// split on &, | and newlines.
var gitWriteSubs = map[string]bool{
	"commit": true, "push": true, "reset": true, "checkout": true, "merge": true,
	"rebase": true, "cherry-pick": true, "clean": true, "stash": true,
}

const gitBlockedMsg = "该 git 写操作已被 win7-agent 禁止（会绕过 checkpoint 保护，push 后无法回退）。请用日常语言向用户说明，由用户自行在终端执行。"

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
					j += 2
					continue
				}
				if strings.HasPrefix(a, "-") {
					j++
					continue
				}
				break
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

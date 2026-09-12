// pulse7: agent loop (formerly win7-agent) + files + git checkpoints + auto-degrading sandbox.
// Build: GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	openai "github.com/sashabaranov/go-openai"
)

type config struct {
	baseURL, apiKey, model, workspace string
	startExe, box, sandboxRoot        string
	yolo                              bool
	readOnly                          bool
	shellTimeout                      time.Duration
	maxCtx                            int
	maxRounds                         int
	processWarnThreshold              int
	backgroundTaskMaxOutputMB         int
	backgroundTaskWarnCount           int
	backgroundTaskWarnSec             int
	llmFirstChunkTimeout              time.Duration
	llmIdleTimeout                    time.Duration
	llmMaxRetries                     int
	llmCompressTimeout                time.Duration
	execMode                          bool
	sessionPath, resumePath           string
	manifestPath                      string
	migrateResumeWorkspace            bool
	sandboxPreference                 string
	memLimitMB                        int
	cleanupOnExit                     bool
	exeDir                            string
	listSessions                      bool
	promptFile                        string
	outputFormat                      string
}

type resumePreparation struct {
	ResumeTarget string
	SessionPath  string
	Workspace    string
	TaskID       string
	ManifestPath string
	Migrated     bool
}

func (c *config) exeDirStore() string {
	if c.exeDir != "" {
		return c.exeDir
	}
	exe, _ := os.Executable()
	return filepath.Dir(exe)
}

var sess *session
var curRunner sandboxRunner
var curCfg *config
var curRegistry *Registry
var exitCleanupOnce sync.Once

// teeOut: all user-visible output goes here (stdout + agent.log when active).
// stdout goes through consoleWriter: WriteConsoleW on a real console (correct
// under any codepage, file-encoding T-repl), raw UTF-8 when redirected.
var teeOut io.Writer = newConsoleWriter()

func out(format string, a ...interface{}) { fmt.Fprintf(teeOut, format, a...) }
func outln(a ...interface{})              { fmt.Fprintln(teeOut, a...) }
func outPrint(s string)                   { fmt.Fprint(teeOut, s) }

// errMaxRounds: the turn stopped at the round cap without a final answer.
var errMaxRounds = errors.New("round limit reached without a final answer")

func maxRoundsError(limit int) error {
	return fmt.Errorf("%w: %d rounds", errMaxRounds, limit)
}

// M4-T1 interrupt machinery: first Ctrl-C = controlled stop (kill children,
// complete the session, print the summary); second Ctrl-C = immediate exit.
var (
	interruptFlag int32
	turnCancel    context.CancelFunc
	turnCancelMu  sync.Mutex
)

var errInterrupted = errors.New("interrupted by user")

func watchInterrupt() {
	sig := make(chan os.Signal, 4)
	signal.Notify(sig, os.Interrupt)
	go func() {
		var lastCtrl time.Time
		for range sig {
			// T4 (output-layering): in REPL a second Ctrl-C within 3s exits;
			// a single press only aborts the running turn. exec keeps its
			// rc-0.4 semantics exactly: first press = graceful stop + exit
			// 130, ANY second press = immediate exit.
			if curCfg != nil && curCfg.execMode {
				if atomic.AddInt32(&interruptFlag, 1) >= 2 {
					exitWith(130, "INTERRUPTED-FORCE", "second Ctrl-C: immediate exit (exec mode)")
				}
			} else if time.Since(lastCtrl) < 3*time.Second {
				exitWith(130, "INTERRUPTED-FORCE", "second Ctrl-C within 3s: immediate exit")
			}
			lastCtrl = time.Now()
			atomic.AddInt32(&interruptFlag, 1)
			turnCancelMu.Lock()
			cancel := turnCancel
			turnCancelMu.Unlock()
			if cancel == nil {
				outln("\n[当前没有正在执行的任务（再按一次退出，或 /exit）]")
				continue
			}
			outln("\n[收到中断，正在停止当前轮（3 秒内再按一次立即退出；REPL 下会话保留）…]")
			interruptActiveManagedProcess()
			if curRunner != nil {
				curRunner.Interrupt()
			}
			cancel()
		}
	}()
}

// resetInterrupt clears the Ctrl-C latch after a turn ends (REPL continues
// with a fresh latch; a stale count must not turn the NEXT single press into
// a force-exit).
func resetInterrupt() { atomic.StoreInt32(&interruptFlag, 0) }

func interrupted() bool { return atomic.LoadInt32(&interruptFlag) > 0 }

// finalizeInterrupted completes the session: unanswered tool_calls get a
// synthetic result so --resume works, then the T4 summary is printed.
func finalizeInterrupted(msgs *[]openai.ChatCompletionMessage, reg *Registry, endSt taskEndState) error {
	const note = "用户中断，该调用未完成，无法确定是否已执行。\n" +
		"请先用只读工具（read / ls / grep）检查当前实际状态，再决定下一步；若需重试请先向用户说明。"
	answered := map[string]bool{}
	for _, m := range *msgs {
		if m.Role == openai.ChatMessageRoleTool {
			answered[m.ToolCallID] = true
		}
	}
	patched := 0
	for _, m := range *msgs {
		if m.Role != openai.ChatMessageRoleAssistant {
			continue
		}
		for _, c := range m.ToolCalls {
			if !answered[c.ID] {
				if err := pushMsg(msgs, openai.ChatCompletionMessage{
					Role: openai.ChatMessageRoleTool, ToolCallID: c.ID, Content: note,
				}); err != nil {
					return err
				}
				patched++
			}
		}
	}
	if patched > 0 {
		out("[中断] 已为 %d 个未完成的工具调用补写结果\n", patched)
	}
	endOfTaskSummary(reg, false)
	if sess != nil && sess.file() != "" {
		out("=== 已中断；续跑命令：pulse7.exe --resume %q \"继续\" ===\n", sess.id())
	} else {
		outln("=== 已中断；可用 --resume 继续本会话 ===")
	}
	return nil
}

// exitWith (T2 file-encoding): every process exit writes a terminal record
// to stdout AND data\logs\agent.log before os.Exit. A batch harness checks
// for this record (or a nonzero EXITCODE echo) — "process vanished with no
// output" can never again masquerade as a pass.
func exitWith(code int, kind, detail string) {
	performExitCleanup()
	line := fmt.Sprintf("=== EXIT %s code=%d %s ===", kind, code, time.Now().Format(time.RFC3339))
	if detail != "" {
		line += "\n" + detail
	}
	outln(line)
	if exe, err := os.Executable(); err == nil {
		if f, err := os.OpenFile(filepath.Join(filepath.Dir(exe), "data", "logs", "agent.log"),
			os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			f.WriteString(line + "\n")
			f.Close()
		}
	}
	os.Exit(code)
}

func performExitCleanup() {
	exitCleanupOnce.Do(func() {
		if curRegistry != nil && curRegistry.tasks != nil {
			curRegistry.tasks.exitSummary()
		}
		if curRunner != nil && curCfg != nil {
			sessionEndCleanup(curRunner, curCfg)
		}
		if curRegistry != nil && curRegistry.tasks != nil {
			curRegistry.tasks.waitHarvested()
		}
	})
}

func main() {
	// T2 (file-encoding): no exit may be silent. Panics on the main
	// goroutine land here with the stack in agent.log; os.Exit paths all go
	// through exitWith which writes the same terminal record first.
	defer func() {
		if p := recover(); p != nil {
			exitWith(2, "PANIC", fmt.Sprintf("%v\n%s", p, debug.Stack()))
		}
	}()
	cfg := &config{}
	flag.StringVar(&cfg.baseURL, "base-url", "http://127.0.0.1:8080/v1", "OpenAI-compatible base URL")
	flag.StringVar(&cfg.apiKey, "api-key", "dummy", "API key")
	flag.StringVar(&cfg.model, "model", "mock-model", "model name")
	flag.StringVar(&cfg.workspace, "workspace", ".", "workspace root (path allowlist)")
	flag.StringVar(&cfg.startExe, "start-exe", `C:\Program Files\Sandboxie\Start.exe`, "Sandboxie Start.exe")
	flag.StringVar(&cfg.box, "box", "Win7Agent", "Sandboxie box name")
	flag.StringVar(&cfg.sandboxRoot, "sandbox-root", `C:\Sandbox`, "Sandboxie container root")
	flag.BoolVar(&cfg.yolo, "yolo", false, "skip interactive confirmation")
	flag.BoolVar(&cfg.readOnly, "read-only", false, "deny shell, write, edit and rollback in code")
	flag.DurationVar(&cfg.shellTimeout, "shell-timeout", 120*time.Second, "shell tool timeout")
	flag.IntVar(&cfg.maxCtx, "max-ctx", defaultMaxContextBytes, "context budget in serialized UTF-8 bytes (messages + tools)")
	flag.IntVar(&cfg.maxRounds, "max-rounds", 100, "maximum tool-call rounds before stopping without a final answer")
	flag.IntVar(&cfg.processWarnThreshold, "process-warn-threshold", defaultProcessWarnThreshold, "warn above this session process count")
	flag.IntVar(&cfg.backgroundTaskMaxOutputMB, "background-task-max-output-mb", defaultBackgroundTaskMaxOutputMB, "hard output cap per background task")
	flag.IntVar(&cfg.backgroundTaskWarnCount, "background-task-warn-count", defaultBackgroundTaskWarnCount, "warn above this concurrent background task count")
	flag.IntVar(&cfg.backgroundTaskWarnSec, "background-task-warn-sec", defaultBackgroundTaskWarnSec, "warn after a background task runs this many seconds")
	flag.DurationVar(&cfg.llmFirstChunkTimeout, "llm-first-chunk-timeout", 300*time.Second,
		"LLM watchdog: max wait from request start to the first streamed chunk (queueing)")
	flag.DurationVar(&cfg.llmIdleTimeout, "llm-idle-timeout", 120*time.Second,
		"LLM watchdog: max gap between streamed chunks, reset on every chunk")
	flag.IntVar(&cfg.llmMaxRetries, "llm-max-retries", 2,
		"retries for retryable LLM failures (backoff 5s/15s)")
	flag.DurationVar(&cfg.llmCompressTimeout, "llm-compress-timeout", 180*time.Second,
		"timeout budget of the context-compression summarize call")
	flag.StringVar(&cfg.sessionPath, "session", "", "session .jsonl path (default auto)")
	flag.StringVar(&cfg.resumePath, "resume", "", "resume from a session .jsonl")
	flag.BoolVar(&cfg.migrateResumeWorkspace, "migrate-resume-workspace", false,
		"fork resumed history into a new session bound to the current workspace")
	flag.BoolVar(&cfg.listSessions, "list", false, "list recent sessions (time/workspace/first message/count)")
	flag.StringVar(&cfg.promptFile, "prompt-file", "", "read the task prompt from this UTF-8 file (recommended for Chinese/long prompts)")
	flag.StringVar(&cfg.sandboxPreference, "sandbox-preference", "auto", "auto | sandboxie | jobobject")
	flag.IntVar(&cfg.memLimitMB, "memory-limit-mb", 2048, "JobObject memory cap in MB")
	flag.BoolVar(&cfg.cleanupOnExit, "cleanup-on-exit", true, "terminate + clear agent sandbox box on exit")
	flag.StringVar(&cfg.outputFormat, "output-format", outputFormatText, "text | stream-json")
	flag.Parse()
	if err := configureEventOutput(cfg.outputFormat); err != nil {
		exitWith(2, "USAGE", err.Error())
	}
	if cfg.outputFormat == outputFormatStreamJSON {
		teeOut = newConsoleErrorWriter()
	}

	// N5: global config, then project config, then explicit flags.
	if exe, err := os.Executable(); err == nil {
		cfg.exeDir = filepath.Dir(exe)
	}
	ac, configWarnings, err := loadLayeredAgentConfig(cfg.workspace, flagWasSet(flag.CommandLine, "workspace"))
	if err != nil {
		exitWith(1, "CONFIG-ERROR", err.Error())
	}
	for _, warning := range configWarnings {
		out("[警告] %s\n", warning)
	}
	applyConfigToFlags(cfg, ac, flag.CommandLine)
	args := flag.Args()
	if len(args) > 0 && args[0] == "task-worker" {
		os.Exit(runTaskWorker(args[1:]))
	}
	watchInterrupt()

	if cfg.listSessions {
		dir := filepath.Join(cfg.exeDirStore(), "data", "sessions")
		out("%-20s %-28s %4s  %s\n", "TIME", "WORKSPACE", "MSG", "FIRST MESSAGE")
		for _, si := range listSessions(dir, 20) {
			out("%-20s %-28s %4d  %s\n",
				si.mtime.Format("01-02 15:04:05"),
				func() string {
					w := si.workspace
					if len(w) > 28 {
						w = "..." + w[len(w)-25:]
					}
					return w
				}(),
				si.count, si.firstUser)
			_ = si.path
		}
		return
	}

	sub := "repl"
	if len(args) > 0 {
		sub = args[0]
	}
	if cfg.outputFormat == outputFormatStreamJSON && sub != "exec" {
		exitWith(2, "USAGE", "--output-format stream-json is only supported by exec")
	}

	// T3 (PreRC02): tee stdout to data\logs\agent.log — on Win7 /it sessions,
	// spawning sandboxed shell children can corrupt the Go process's console
	// handles, making subsequent fmt.Print output vanish from `>>` redirects.
	// The log file is unaffected and is the durable record.
	if sub == "exec" || sub == "repl" {
		logDir := filepath.Join(cfg.exeDirStore(), "data", "logs")
		os.MkdirAll(logDir, 0o755)
		if lf, err := os.OpenFile(filepath.Join(logDir, "agent.log"),
			os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			humanConsole := io.Writer(newConsoleWriter())
			if cfg.outputFormat == outputFormatStreamJSON {
				humanConsole = newConsoleErrorWriter()
			}
			teeOut = io.MultiWriter(humanConsole, lf)
			logOnly = lf
			fmt.Fprintf(lf, "\n=== session start %s ===\n", time.Now().Format(time.RFC3339))
		}
	}

	switch sub {
	case "mock":
		secs := 180
		if len(args) > 1 {
			fmt.Sscanf(args[1], "%d", &secs)
		}
		runMock(secs)
	case "exec":
		prompt := strings.Join(args[1:], " ")
		if pf := cfg.promptFile; pf != "" {
			if prompt != "" {
				exitWith(2, "USAGE", "EXEC-ERROR: --prompt-file and a positional prompt are mutually exclusive")
			}
			b, err := os.ReadFile(pf)
			if err != nil {
				exitWith(2, "USAGE", fmt.Sprintf("EXEC-ERROR: cannot read --prompt-file: %v", err))
			}
			// UTF-8 BOM (optional) is tolerated, then surrounding whitespace.
			prompt = strings.TrimPrefix(string(b), string(rune(0xFEFF)))
			prompt = strings.TrimSpace(prompt)
			if prompt == "" {
				exitWith(2, "USAGE", "EXEC-ERROR: --prompt-file is empty: "+pf)
			}
		}
		if prompt == "" {
			exitWith(2, "USAGE", `usage: pulse7 exec "task ..."  |  pulse7 exec --prompt-file <path>`)
		}
		cfg.execMode = true
		runExec(cfg, prompt)
	case "repl":
		runRepl(cfg)
	case "serve":
		if err := runServe(cfg); err != nil {
			exitWith(1, "HTTP-ERROR", err.Error())
		}
	case "doctor":
		doctorCmd(cfg)
	case "init":
		home, err := os.UserHomeDir()
		if err != nil {
			exitWith(1, "INIT-ERROR", fmt.Sprintf("resolve user profile: %v", err))
		}
		globalPath := globalConfigPath(home)
		if err := writeAgentConfigTemplate(globalPath); err != nil {
			exitWith(1, "INIT-ERROR", fmt.Sprintf("%v", err))
		}
		if err := writePermissionConfigTemplate(permissionsPath(cfg.exeDirStore())); err != nil {
			exitWith(1, "INIT-ERROR", fmt.Sprintf("%v", err))
		}
		outln("config ensured:", globalPath)
		outln("permissions ensured:", permissionsPath(cfg.exeDirStore()))
	default:
		exitWith(2, "USAGE", "usage: pulse7 [repl | exec \"task\" | mock <sec>] [flags]")
	}
}

func newClient(cfg *config) *openai.Client {
	ocfg := openai.DefaultConfig(cfg.apiKey)
	ocfg.BaseURL = cfg.baseURL
	return openai.NewClientWithConfig(ocfg)
}

func setupEnv(cfg *config, taskID string) (*Registry, error) {
	if err := validateBackgroundTaskConfig(cfg.backgroundTaskMaxOutputMB, cfg.backgroundTaskWarnCount,
		cfg.backgroundTaskWarnSec, cfg.processWarnThreshold); err != nil {
		return nil, err
	}
	ws, err := filepath.Abs(cfg.workspace)
	if err != nil {
		return nil, err
	}
	cfg.workspace = ws
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)

	runner, reason, _ := selectSandboxMode(cfg, ws, false)
	if reason != "" {
		out("sandbox: %s (auto-degraded: %s) - no system patch required\n", runner.Mode(), reason)
	} else {
		out("sandbox: %s (box=%s)\n", runner.Mode(), cfg.box)
	}
	curRunner, curCfg = runner, cfg
	if starter, ok := runner.(interface{ StartSession() error }); ok {
		if err := starter.StartSession(); err != nil {
			return nil, fmt.Errorf("start session Job: %w", err)
		}
	}
	purgeStaleRunDirs(homeDir(), time.Hour)
	purgeStaleIndexTemps(filepath.Join(cfg.exeDirStore(), "data", "sessions"), time.Hour)

	auditPath := filepath.Join(exeDir, "data", "sessions", "audit.jsonl")
	manPath := cfg.manifestPath
	if manPath == "" {
		manPath = filepath.Join(exeDir, "data", "sessions", "manifest-"+taskID+".jsonl")
	}
	if err := os.MkdirAll(filepath.Dir(auditPath), 0o755); err != nil {
		return nil, fmt.Errorf("create session audit directory: %w", err)
	}
	permissions, err := loadPermissionConfig(permissionsPath(exeDir))
	if err != nil {
		return nil, fmt.Errorf("load permissions: %w", err)
	}
	r := NewRegistry(&Policy{Workspace: ws}, runner, auditPath, manPath,
		cfg.yolo, cfg.readOnly, cfg.execMode, os.Stdin, exeDir, ws, taskID, permissions)
	r.tasks.configure(cfg.backgroundTaskMaxOutputMB, cfg.backgroundTaskWarnCount,
		cfg.backgroundTaskWarnSec, cfg.processWarnThreshold, cfg.shellTimeout)
	curRegistry = r
	if j, ok := runner.(*jobObjectRunner); ok {
		if err := j.processes.loadDetached(r.tasks.dir); err != nil {
			return nil, fmt.Errorf("load detached history: %w", err)
		}
		if notice := j.processes.previousNotice(); notice != "" {
			outln(notice)
			emitRuntimeEvent("detached_history", map[string]interface{}{"verified": false, "notice": notice})
		}
	}
	return r, nil
}

// openSessionFor returns a LAZY session (T5): the file appears only when the
// first conversation record is written. With --resume set, it appends to the
// ORIGINAL session file instead of forking a fresh one per exec, so --list
// and retry guidance keep pointing at one stable id.
func openSessionFor(cfg *config, taskID string) *session {
	path := cfg.sessionPath
	if path == "" {
		path = filepath.Join(cfg.exeDirStore(), "data", "sessions", "sess-"+taskID+".jsonl")
	}
	s := newSession(path, cfg.workspace)
	if cfg.manifestPath != "" {
		s.manifestPath = cfg.manifestPath
	}
	return s
}

// resolveResume maps --resume values (M4-T5): a path or a session id
// (filename stem) or "latest"; empty returns empty (no resume).
func resolveResume(cfg *config, v string) string {
	if v == "" || v == "latest" {
		if v == "" {
			return ""
		}
		infos := listSessions(filepath.Join(cfg.exeDirStore(), "data", "sessions"), 0)
		workspace, _ := filepath.Abs(cfg.workspace)
		for _, info := range infos {
			candidate, _ := filepath.Abs(info.workspace)
			if sameResolvedPath(candidate, workspace) {
				return info.path
			}
		}
		return ""
	}
	if _, err := os.Stat(v); err == nil {
		return v
	}
	dir := filepath.Join(cfg.exeDirStore(), "data", "sessions")
	for _, cand := range []string{
		filepath.Join(dir, v+".jsonl"),
		filepath.Join(dir, "sess-"+v+".jsonl"),
	} {
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	return v // let loadSession produce the error message
}

func prepareResume(cfg *config, generatedTaskID string) (resumePreparation, error) {
	workspace, err := filepath.Abs(cfg.workspace)
	if err != nil {
		return resumePreparation{}, err
	}
	if cfg.sessionPath != "" {
		generatedTaskID = sessionTaskID(cfg.sessionPath)
	}
	base := resumePreparation{
		SessionPath: cfg.sessionPath,
		Workspace:   workspace, TaskID: generatedTaskID,
		ManifestPath: filepath.Join(cfg.exeDirStore(), "data", "sessions", "manifest-"+generatedTaskID+".jsonl"),
	}
	if cfg.resumePath == "" {
		return base, nil
	}
	target := resolveResume(cfg, cfg.resumePath)
	if target == "" {
		return resumePreparation{}, fmt.Errorf("no resumable session found for workspace %s", workspace)
	}
	meta, err := loadSessionMetadata(target)
	if err != nil {
		return resumePreparation{}, err
	}
	base.ResumeTarget = target
	if !sameResolvedPath(meta.Workspace, workspace) && !cfg.migrateResumeWorkspace {
		return resumePreparation{}, fmt.Errorf("resume workspace mismatch: session=%s current=%s; use --migrate-resume-workspace to copy history into the current workspace", meta.Workspace, workspace)
	}
	if cfg.migrateResumeWorkspace {
		base.Migrated = true
		if base.SessionPath != "" && sameResolvedPath(base.SessionPath, target) {
			return resumePreparation{}, errors.New("migrated resume must use a new session path")
		}
		return base, nil
	}
	originalWorkspace, err := filepath.Abs(meta.Workspace)
	if err != nil {
		return resumePreparation{}, err
	}
	wantManifest := filepath.Join(cfg.exeDirStore(), "data", "sessions", "manifest-"+meta.TaskID+".jsonl")
	manifestPath := meta.ManifestPath
	if manifestPath == "" {
		manifestPath = wantManifest
	}
	manifestPath, err = filepath.Abs(manifestPath)
	if err != nil {
		return resumePreparation{}, err
	}
	if !sameResolvedPath(manifestPath, wantManifest) {
		return resumePreparation{}, fmt.Errorf("session manifest identity mismatch: metadata=%s expected=%s", manifestPath, wantManifest)
	}
	return resumePreparation{
		ResumeTarget: target, SessionPath: target, Workspace: originalWorkspace,
		TaskID: meta.TaskID, ManifestPath: manifestPath,
	}, nil
}

func applyResumePreparation(cfg *config, plan resumePreparation) {
	cfg.workspace = plan.Workspace
	cfg.sessionPath = plan.SessionPath
	cfg.manifestPath = plan.ManifestPath
}

func loadPreparedMessages(plan resumePreparation, destination *session) ([]openai.ChatCompletionMessage, error) {
	if plan.ResumeTarget == "" {
		return nil, nil
	}
	msgs, err := loadSession(plan.ResumeTarget)
	if err != nil {
		return nil, err
	}
	if plan.Migrated {
		for _, msg := range msgs {
			if err := destination.record(msg); err != nil {
				return nil, err
			}
		}
	}
	return msgs, nil
}

// pushMsg appends to the conversation and records it in the session file.
func pushMsg(msgs *[]openai.ChatCompletionMessage, m openai.ChatCompletionMessage, outcome ...*toolOutcome) error {
	if err := sess.record(m, outcome...); err != nil {
		return err
	}
	*msgs = append(*msgs, m)
	return nil
}

func newTaskID() string {
	now := time.Now()
	return fmt.Sprintf("t%s-%03d", now.Format("0102-150405"), now.Nanosecond()/1e6)
}

// loadAgentMd reads workspace AGENT.md as project conventions for the system
// prompt (M4-T3). Hard cap 8KB with an explicit truncation warning.
// baseSystemPrompt is the always-on behavior prompt (prompt-tune round):
// vague requirements -> inspect first, then ask the user ONE concrete
// question instead of inferring scope; write/edit touch only what was asked.
func baseSystemPrompt() string {
	return "你是运行在用户工作区里的编程助手。行为准则：\n" +
		"1. 如果任务描述不明确（范围、目标或验收标准看不清），先用只读工具（read / ls / grep）了解现状，" +
		"然后向用户提一个具体的问题，等回答后再动手；不要自行推断需求范围。\n" +
		"2. 只做用户明确要求的改动；没有要求的事情（重构、重命名、移动文件、建目录）即使看起来更好也不要做。\n" +
		"3. 工具会在首次修改前自动建立 checkpoint；仅在用户明确要求额外快照时手动 checkpoint。最后简要说明改了什么。" +
		"改动后如需验证，运行程序或测试（例如 python x.py）。" +
		"不要用 type / more / findstr 等命令回读文件来确认内容——工具返回的 diff 已经是准确的。"
}
func loadAgentMd(ws string) string {
	b, err := os.ReadFile(filepath.Join(ws, "AGENT.md"))
	if err != nil || len(b) == 0 {
		return ""
	}
	const max = 8 << 10
	truncated := false
	if len(b) > max {
		cut := max
		for cut > 0 && !utf8.RuneStart(b[cut]) {
			cut--
		}
		b = b[:cut]
		truncated = true
	}
	s := "以下是本项目的约定（来自工作区 AGENT.md），必须遵守：\n" + string(b)
	if truncated {
		outln("[警告] AGENT.md 超过 8KB，已截断后注入")
		s += "\n（AGENT.md 过长，以上为截断内容）"
	}
	return s
}

func systemMessage(cfg *config) openai.ChatCompletionMessage {
	system := baseSystemPrompt()
	if curRegistry != nil {
		if j, ok := curRegistry.runner.(*jobObjectRunner); ok && j.processes != nil {
			system += "\n" + j.processes.previousNotice()
		}
	}
	if project := loadAgentMd(cfg.workspace); project != "" {
		system += "\n\n" + project
		out("[AGENT.md] 已注入项目约定（%d 字节）\n", len(project))
	}
	catalog := discoverSkills(cfg.workspace)
	printSkillWarnings(catalog)
	if skills := skillsSystemBlock(catalog.Skills); skills != "" {
		system += "\n\n" + skills
		out("[skills] 已注入 %d 条技能元数据\n", len(catalog.Skills))
	}
	return openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: system}
}

func runExec(cfg *config, prompt string) {
	// TEST-ONLY (T2 verification): inject a panic to prove the recover
	// wrapper writes the stack to agent.log instead of dying silently.
	if os.Getenv("PULSE7_PANIC_TEST") != "" {
		panic("PULSE7_PANIC_TEST injected")
	}
	plan, err := prepareResume(cfg, newTaskID())
	if err != nil {
		exitWith(1, "RESUME-ERROR", fmt.Sprintf("%v", err))
	}
	applyResumePreparation(cfg, plan)
	taskID := plan.TaskID
	client := newClient(cfg)
	reg, err := setupEnv(cfg, taskID)
	if err != nil {
		exitWith(1, "SETUP-ERROR", fmt.Sprintf("%v", err))
	}
	sess = openSessionFor(cfg, taskID)
	emitSessionInit(cfg, reg, sess.id())
	defer sess.Close()
	defer performExitCleanup()
	msgs, err := loadPreparedMessages(plan, sess)
	if err != nil {
		if errors.Is(err, errSessionStorage) {
			exitWith(1, "STORAGE-ERROR", err.Error())
		}
		exitWith(1, "RESUME-ERROR", fmt.Sprintf("%v", err))
	}
	if plan.ResumeTarget != "" {
		if plan.Migrated {
			out("migrated %d messages from %s into new session %s (workspace=%s)\n",
				len(msgs), plan.ResumeTarget, sess.id(), cfg.workspace)
		} else {
			out("resumed %d messages from %s (workspace=%s task=%s)\n",
				len(msgs), plan.ResumeTarget, cfg.workspace, taskID)
		}
	}
	outln("=== pulse7 exec (headless) ===")
	if len(msgs) == 0 {
		if err := pushMsg(&msgs, systemMessage(cfg)); err != nil {
			exitWith(1, "STORAGE-ERROR", fmt.Sprintf("persist system message: %v", err))
		}
	}
	if err := pushMsg(&msgs, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: prompt}); err != nil {
		exitWith(1, "STORAGE-ERROR", fmt.Sprintf("persist user message: %v", err))
	}
	taskStart := time.Now()
	var stats turnStats
	_, err = streamTurn(client, reg, cfg, &msgs, &stats)
	endSt := taskEndState{rounds: stats.rounds, elapsed: time.Since(taskStart)}
	if errors.Is(err, errInterrupted) {
		emitTurnResult("interrupted", err)
		if finalErr := finalizeInterrupted(&msgs, reg, endSt); finalErr != nil {
			exitWith(1, "STORAGE-ERROR", finalErr.Error())
		}
		resetInterrupt()
		exitWith(130, "INTERRUPTED", "")
	}
	resetInterrupt()
	if errors.Is(err, errSessionStorage) {
		emitTurnResult("error", err)
		printTaskEnd(reg, false, taskEndState{status: "存储失败", rounds: endSt.rounds, elapsed: endSt.elapsed})
		exitWith(1, "STORAGE-ERROR", "对话进度未完整保存："+err.Error())
	}
	// T5: distinguish "ends with a question for the user" from clean completion
	if msgsLen := len(msgs); msgsLen > 0 && msgs[msgsLen-1].Role == openai.ChatMessageRoleAssistant {
		c := msgs[msgsLen-1].Content
		if strings.Contains(c, "？") || strings.Contains(c, "?") {
			if strings.Contains(c, "具体") || strings.Contains(c, "指什么") ||
				strings.Contains(c, "哪些") || strings.Contains(c, "希望") ||
				strings.Contains(c, "还是") || strings.Contains(c, "请告诉我") {
				out("----------------------------------------\n")
				out("需要你回答后才能继续：\n%s\n", c)
				if sess != nil && sess.n > 0 {
					out("\n回答后续跑：\n  pulse7.exe --resume %q \"你的回答\"\n", sess.id())
				}
				out("----------------------------------------\n")
				endOfTaskSummary(reg, false)
				emitTurnResult("need_answer", nil)
				exitWith(2, "AWAIT-USER-ANSWER", "")
			}
		}
	}
	endSt.status = "已完成"
	if err != nil {
		endSt.status = "出错中止"
	}
	if err == nil {
		emitTurnResult("success", nil)
	} else if errors.Is(err, errMaxRounds) {
		emitTurnResult("max_rounds", err)
	} else {
		emitTurnResult("error", err)
	}
	printTaskEnd(reg, errors.Is(err, errMaxRounds), endSt)
	if err != nil {
		detail := fmt.Sprintf("EXEC-ERROR: %v", err)
		// T5: hand the user a copy-paste resume command with the concrete
		// session id, or say explicitly that nothing was saved.
		if sess != nil && sess.n > 0 {
			detail += fmt.Sprintf("\n已完成的进度已保存。续跑命令：\n  pulse7.exe --resume %q %q", sess.id(), prompt)
		} else {
			detail += "\n本次运行没有保存任何对话进度（未生成 session 文件）。"
		}
		exitWith(1, "EXEC-ERROR", detail)
	}
	outln("=== EXEC-DONE ===")
	// T2 (file-encoding): uniform terminal record for harness verification.
	exitWith(0, "DONE", "")
}

func runRepl(cfg *config) {
	plan, err := prepareResume(cfg, newTaskID())
	if err != nil {
		exitWith(1, "RESUME-ERROR", fmt.Sprintf("%v", err))
	}
	applyResumePreparation(cfg, plan)
	taskID := plan.TaskID
	client := newClient(cfg)
	reg, err := setupEnv(cfg, taskID)
	if err != nil {
		exitWith(1, "SETUP-ERROR", fmt.Sprintf("%v", err))
	}
	sess = openSessionFor(cfg, taskID)
	emitSessionInit(cfg, reg, sess.id())
	defer sess.Close()
	defer performExitCleanup()
	msgs, err := loadPreparedMessages(plan, sess)
	if err != nil {
		if errors.Is(err, errSessionStorage) {
			exitWith(1, "STORAGE-ERROR", err.Error())
		}
		exitWith(1, "RESUME-ERROR", fmt.Sprintf("%v", err))
	}
	if plan.ResumeTarget != "" {
		if plan.Migrated {
			out("migrated %d messages from %s into new session %s (workspace=%s)\n",
				len(msgs), plan.ResumeTarget, sess.id(), cfg.workspace)
		} else {
			out("resumed %d messages from %s (workspace=%s task=%s)\n",
				len(msgs), plan.ResumeTarget, cfg.workspace, taskID)
		}
	}
	outln("pulse7 REPL (model:", cfg.model, "workspace:", reg.policy.Workspace, ")")
	if len(msgs) == 0 {
		if err := pushMsg(&msgs, systemMessage(cfg)); err != nil {
			exitWith(1, "STORAGE-ERROR", err.Error())
		}
	}
	outln("命令：/help /list /skills /tasks /clear /exit（Ctrl-C 中止当前轮，会话保留）")
	for {
		outPrint("> ")
		line, readErr := reg.input.read(nil)
		if errors.Is(readErr, io.EOF) {
			return
		}
		if readErr != nil {
			exitWith(1, "INPUT-ERROR", readErr.Error())
		}
		switch {
		case line == "":
			continue
		case line == "/exit" || line == "/quit":
			return
		case line == "/clear":
			system := systemMessage(cfg)
			if err := sess.recordClear(system.Content); err != nil {
				outln("ERROR: 上下文未清空：", err)
				continue
			}
			msgs = []openai.ChatCompletionMessage{system}
			outln("[上下文已清空并记录会话分界标记，可以开始新任务]")
			continue
		case line == "/help":
			outln("可用命令：")
			outln("  /help  显示本帮助")
			outln("  /list  列出历史会话")
			outln("  /skills 列出当前可用 skills")
			outln("  /tasks 列出本会话后台任务")
			outln("  /clear 清空当前上下文，开始新任务（不删会话文件）")
			outln("  /exit  退出（等同 /quit）")
			outln("直接输入其他内容即作为任务发给模型。")
			continue
		case line == "/list":
			dir := filepath.Join(cfg.exeDirStore(), "data", "sessions")
			outln("历史会话（最近 20 条）：")
			for _, si := range listSessions(dir, 20) {
				w := si.workspace
				if len(w) > 28 {
					w = "..." + w[len(w)-25:]
				}
				out("%-20s %-28s %4d  %s\n", si.mtime.Format("01-02 15:04:05"), w, si.count, si.firstUser)
			}
			continue
		case line == "/skills":
			catalog := discoverSkills(cfg.workspace)
			printSkillWarnings(catalog)
			outln(skillsList(catalog))
			continue
		case line == "/tasks":
			outln(reg.tasks.list())
			continue
		case strings.HasPrefix(line, "/tasks kill "):
			outln(reg.Execute("task_kill", strings.TrimSpace(strings.TrimPrefix(line, "/tasks kill "))))
			continue
		case strings.HasPrefix(line, "/"):
			outln("[未知命令 " + line + "——/help 查看可用命令；如需作为任务发送请去掉开头的 /]")
			continue
		}
		resetInterrupt()
		if err := pushMsg(&msgs, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: line}); err != nil {
			exitWith(1, "STORAGE-ERROR", err.Error())
		}
		turnStart := time.Now()
		var stats turnStats
		_, stErr := streamTurn(client, reg, cfg, &msgs, &stats)
		endSt := taskEndState{rounds: stats.rounds, elapsed: time.Since(turnStart)}
		if errors.Is(stErr, errInterrupted) {
			emitTurnResult("interrupted", stErr)
			if finalErr := finalizeInterrupted(&msgs, reg, endSt); finalErr != nil {
				exitWith(1, "STORAGE-ERROR", finalErr.Error())
			}
			resetInterrupt()
			outln("[已中止当前轮，会话保留。可以直接输入新的指示。]")
			continue
		}
		resetInterrupt()
		if errors.Is(stErr, errSessionStorage) {
			emitTurnResult("error", stErr)
			exitWith(1, "STORAGE-ERROR", "对话进度未完整保存："+stErr.Error())
		}
		if stErr != nil {
			if errors.Is(stErr, errMaxRounds) {
				emitTurnResult("max_rounds", stErr)
			} else {
				emitTurnResult("error", stErr)
			}
			printTaskEnd(reg, errors.Is(stErr, errMaxRounds), taskEndState{status: "出错中止", rounds: endSt.rounds, elapsed: endSt.elapsed})
			outln("ERROR:", stErr)
			continue
		}
		emitTurnResult("success", nil)
		printTaskEnd(reg, false, taskEndState{status: "已完成", rounds: endSt.rounds, elapsed: endSt.elapsed})
	}
}

// streamTurn runs one agent turn: stream reply; execute tool calls; feed results; loop until final answer.
// turnStats (T3): per-turn counters surfaced in the terminal block.
type turnStats struct {
	rounds int
}

func streamTurn(client *openai.Client, reg *Registry, cfg *config, msgs *[]openai.ChatCompletionMessage, stats *turnStats) (string, error) {
	if j, ok := reg.runner.(*jobObjectRunner); ok {
		j.emitMode("state")
		if j.processes != nil {
			if notice := j.processes.previousNotice(); notice != "" {
				found := false
				for _, message := range *msgs {
					if strings.Contains(message.Content, notice) {
						found = true
						break
					}
				}
				if !found {
					if err := pushMsg(msgs, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: notice}); err != nil {
						return "", err
					}
				}
			}
		}
	}
	// T1 (slow-network): no whole-turn time budget. The turn context carries
	// only the interrupt cancellation; every LLM request is guarded by the
	// first-chunk / idle watchdogs in llmStreamOnce instead. A stream that
	// keeps producing chunks is never killed for being slow.
	ctx, cancel := context.WithCancel(context.Background())
	turnCancelMu.Lock()
	turnCancel = cancel
	turnCancelMu.Unlock()
	defer func() {
		turnCancelMu.Lock()
		turnCancel = nil
		turnCancelMu.Unlock()
		cancel()
	}()
	tools := reg.Definitions()
	for round := 0; round < cfg.maxRounds; round++ {
		if interrupted() {
			return "", errInterrupted
		}
		roundStart := time.Now()
		if stats != nil {
			stats.rounds = round + 1
		}
		out("[第 %d 轮 / 上限 %d]\n", round+1, cfg.maxRounds)
		emitContextState(*msgs, tools, cfg)
		prepErr := maybeCompressContext(ctx, client, cfg, tools, msgs)
		if prepErr != nil && !errors.Is(prepErr, errLocalContextBudget) {
			return "", fmt.Errorf("%w: persist context-compression audit: %v", errSessionStorage, prepErr)
		}
		req := openai.ChatCompletionRequest{
			Model:       cfg.model,
			Messages:    *msgs,
			Tools:       tools,
			Stream:      true,
			Temperature: codingTemperature,
		}
		var response openai.ChatCompletionMessage
		var calls []openai.ToolCall
		err := prepErr
		if err == nil {
			response, err = roundStreamMessage(ctx, client, cfg, req)
			calls = response.ToolCalls
		}
		if errors.Is(err, errLocalContextBudget) || contextLengthExceededError(err) {
			if compressErr := emergencyCompressContext(ctx, client, cfg, tools, msgs); compressErr != nil {
				if errors.Is(err, errLocalContextBudget) {
					return "", fmt.Errorf("本地预算不足（%v），紧急压缩失败：%w", err, compressErr)
				}
				return "", fmt.Errorf("端点报告上下文超限，紧急压缩失败：%w", compressErr)
			}
			req.Messages = *msgs
			response, err = roundStreamMessage(ctx, client, cfg, req)
			calls = response.ToolCalls
			if contextLengthExceededError(err) {
				return "", fmt.Errorf("紧急压缩后单次重试仍然上下文超限：%w", err)
			}
		}
		if err != nil {
			if interrupted() {
				return "", errInterrupted
			}
			return "", err
		}
		content := response.Content
		if len(calls) == 0 {
			// M4-T0: persist the final assistant answer so the session file
			// distinguishes convergence from cap-stop and --resume sees it.
			if err := pushMsg(msgs, response); err != nil {
				return "", err
			}
			out("[第 %d 轮完成，耗时 %v]\n", round+1, time.Since(roundStart).Round(time.Second))
			// T2 (output-layering): the answer already streamed with the
			// narrative prefix; frame the clean text so it is unmissable.
			frameAnswer(content)
			return content, nil
		}
		if err := pushMsg(msgs, response); err != nil {
			return "", err
		}
		for _, c := range calls {
			if interrupted() {
				return "", errInterrupted
			}
			emitToolCall(c)
			res := reg.Execute(c.Function.Name, c.Function.Arguments)
			emitToolResult(c, res)
			if !strings.HasPrefix(res, "error:") {
				if skill, ok := loadedSkillForRead(cfg.workspace, c.Function.Name, c.Function.Arguments); ok {
					emitRuntimeEvent("skill_loaded", skillLoadedEvent{Name: skill.Name, Path: skill.Path})
				}
			}
			if err := pushMsg(msgs, openai.ChatCompletionMessage{
				Role: openai.ChatMessageRoleTool, ToolCallID: c.ID, Content: res,
			}, toolOutcomeFor(c, res)); err != nil {
				return "", err
			}
			if os.Getenv("PULSE7_PANIC_AFTER_TOOL") != "" {
				panic("PULSE7_PANIC_AFTER_TOOL injected")
			}
		}
		out("[第 %d 轮完成，耗时 %v]\n", round+1, time.Since(roundStart).Round(time.Second))
	}
	return "", maxRoundsError(cfg.maxRounds)
}

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return os.Getenv("USERPROFILE")
}

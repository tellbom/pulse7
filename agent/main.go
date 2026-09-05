// win7-agent M2: agent loop + files + git checkpoints + auto-degrading sandbox.
// Build: GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

type config struct {
	baseURL, apiKey, model, workspace string
	startExe, box, sandboxRoot       string
	yolo                             bool
	shellTimeout                     time.Duration
	maxCtx                           int
	execMode                         bool
	sessionPath, resumePath          string
	sandboxPreference                string
	memLimitMB                       int
	cleanupOnExit                    bool
	exeDir                           string
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

// errMaxRounds: the turn stopped at the round cap without a final answer.
var errMaxRounds = errors.New("too many tool rounds")

// M4-T1 interrupt machinery: first Ctrl-C = controlled stop (kill children,
// complete the session, print the summary); second Ctrl-C = immediate exit.
var (
	interruptFlag int32
	turnCancel    context.CancelFunc
)

var errInterrupted = errors.New("interrupted by user")

func watchInterrupt() {
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, os.Interrupt)
	go func() {
		for range sig {
			if atomic.AddInt32(&interruptFlag, 1) >= 2 {
				fmt.Println("\n[强制退出]")
				os.Exit(130)
			}
			fmt.Println("\n[收到中断，正在停止（再按一次立即退出）…]")
			if curRunner != nil {
				curRunner.Interrupt()
			}
			if turnCancel != nil {
				turnCancel()
			}
		}
	}()
}

func interrupted() bool { return atomic.LoadInt32(&interruptFlag) > 0 }

// finalizeInterrupted completes the session: unanswered tool_calls get a
// synthetic result so --resume works, then the T4 summary is printed.
func finalizeInterrupted(msgs *[]openai.ChatCompletionMessage, reg *Registry) {
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
				pushMsg(msgs, openai.ChatCompletionMessage{
					Role: openai.ChatMessageRoleTool, ToolCallID: c.ID, Content: note})
				patched++
			}
		}
	}
	if patched > 0 {
		fmt.Printf("[中断] 已为 %d 个未完成的工具调用补写结果\n", patched)
	}
	endOfTaskSummary(reg, false)
	fmt.Println("=== 已中断；可用 --resume 继续本会话 ===")
}

func main() {
	cfg := &config{}
	flag.StringVar(&cfg.baseURL, "base-url", "http://127.0.0.1:8080/v1", "OpenAI-compatible base URL")
	flag.StringVar(&cfg.apiKey, "api-key", "dummy", "API key")
	flag.StringVar(&cfg.model, "model", "mock-model", "model name")
	flag.StringVar(&cfg.workspace, "workspace", ".", "workspace root (path allowlist)")
	flag.StringVar(&cfg.startExe, "start-exe", `C:\Program Files\Sandboxie\Start.exe`, "Sandboxie Start.exe")
	flag.StringVar(&cfg.box, "box", "Win7Agent", "Sandboxie box name")
	flag.StringVar(&cfg.sandboxRoot, "sandbox-root", `C:\Sandbox`, "Sandboxie container root")
	flag.BoolVar(&cfg.yolo, "yolo", false, "skip interactive confirmation")
	flag.DurationVar(&cfg.shellTimeout, "shell-timeout", 120*time.Second, "shell tool timeout")
	flag.IntVar(&cfg.maxCtx, "max-ctx", 48000, "max context chars before truncation")
	flag.StringVar(&cfg.sessionPath, "session", "", "session .jsonl path (default auto)")
	flag.StringVar(&cfg.resumePath, "resume", "", "resume from a session .jsonl")
	flag.StringVar(&cfg.sandboxPreference, "sandbox-preference", "auto", "auto | sandboxie | jobobject")
	flag.IntVar(&cfg.memLimitMB, "memory-limit-mb", 2048, "JobObject memory cap in MB")
	flag.BoolVar(&cfg.cleanupOnExit, "cleanup-on-exit", true, "terminate + clear agent sandbox box on exit")
	flag.Parse()

	// M3-E: config\agent.json fills any field not set explicitly by flags.
	if exe, err := os.Executable(); err == nil {
		cfg.exeDir = filepath.Dir(exe)
	}
	applyConfigToFlags(cfg, loadAgentConfig(configPath(cfg.exeDirStore())), flag.CommandLine)
	watchInterrupt()

	args := flag.Args()
	sub := "repl"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "mock":
		secs := 180
		if len(args) > 1 {
			fmt.Sscanf(args[1], "%d", &secs)
		}
		runMock(secs)
	case "exec":
		if len(args) < 2 {
			fmt.Println("usage: win7-agent exec \"task ...\"")
			os.Exit(2)
		}
		cfg.execMode = true
		runExec(cfg, strings.Join(args[1:], " "))
	case "repl":
		runRepl(cfg)
	case "doctor":
		doctorCmd(cfg)
	case "init":
		// M3-E/F: ensure config template exists (never overwrites).
		if err := writeAgentConfigTemplate(configPath(cfg.exeDirStore())); err != nil {
			fmt.Println("INIT-ERROR:", err)
			os.Exit(1)
		}
		fmt.Println("config ensured:", configPath(cfg.exeDirStore()))
	default:
		fmt.Println("usage: win7-agent [repl | exec \"task\" | mock <sec>] [flags]")
		os.Exit(2)
	}
}

func newClient(cfg *config) *openai.Client {
	ocfg := openai.DefaultConfig(cfg.apiKey)
	ocfg.BaseURL = cfg.baseURL
	return openai.NewClientWithConfig(ocfg)
}

func setupEnv(cfg *config, taskID string) (*Registry, error) {
	ws, err := filepath.Abs(cfg.workspace)
	if err != nil {
		return nil, err
	}
	cfg.workspace = ws
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)

	runner, reason, _ := selectSandboxMode(cfg, ws, false)
	if reason != "" {
		fmt.Printf("sandbox: %s (auto-degraded: %s) - no system patch required\n", runner.Mode(), reason)
	} else {
		fmt.Printf("sandbox: %s (box=%s)\n", runner.Mode(), cfg.box)
	}
	curRunner, curCfg = runner, cfg
	purgeStaleRunDirs(homeDir(), time.Hour)
	purgeStaleIndexTemps(filepath.Join(cfg.exeDirStore(), "data", "sessions"), time.Hour)

	auditPath := filepath.Join(exeDir, "data", "sessions", "audit.jsonl")
	manPath := filepath.Join(exeDir, "data", "sessions", "manifest-"+taskID+".jsonl")
	os.MkdirAll(filepath.Dir(auditPath), 0o755)
	return NewRegistry(&Policy{Workspace: ws}, runner, auditPath, manPath,
		cfg.yolo, cfg.execMode, os.Stdin, exeDir, ws, taskID), nil
}

func openSessionFor(cfg *config, taskID string) (*session, error) {
	path := cfg.sessionPath
	if path == "" {
		exe, _ := os.Executable()
		path = filepath.Join(filepath.Dir(exe), "data", "sessions", "sess-"+taskID+".jsonl")
	}
	return openSession(path)
}

// pushMsg appends to the conversation and records it in the session file.
func pushMsg(msgs *[]openai.ChatCompletionMessage, m openai.ChatCompletionMessage) {
	sess.record(m)
	*msgs = append(*msgs, m)
}

func newTaskID() string {
	now := time.Now()
	return fmt.Sprintf("t%s-%03d", now.Format("0102-150405"), now.Nanosecond()/1e6)
}

// loadAgentMd reads workspace AGENT.md as project conventions for the system
// prompt (M4-T3). Hard cap 8KB with an explicit truncation warning.
func loadAgentMd(ws string) string {
	b, err := os.ReadFile(filepath.Join(ws, "AGENT.md"))
	if err != nil || len(b) == 0 {
		return ""
	}
	const max = 8 << 10
	truncated := false
	if len(b) > max {
		b = b[:max]
		truncated = true
	}
	s := "以下是本项目的约定（来自工作区 AGENT.md），必须遵守：\n" + string(b)
	if truncated {
		fmt.Println("[警告] AGENT.md 超过 8KB，已截断后注入")
		s += "\n（AGENT.md 过长，以上为截断内容）"
	}
	return s
}

func runExec(cfg *config, prompt string) {
	taskID := newTaskID()
	client := newClient(cfg)
	reg, err := setupEnv(cfg, taskID)
	if err != nil {
		fmt.Println("SETUP-ERROR:", err)
		os.Exit(1)
	}
	s, err := openSessionFor(cfg, taskID)
	if err == nil {
		sess = s
		defer sess.Close()
	}
	defer sessionEndCleanup(curRunner, curCfg)
	var msgs []openai.ChatCompletionMessage
	if cfg.resumePath != "" {
		msgs, err = loadSession(cfg.resumePath)
		if err != nil {
			fmt.Println("RESUME-ERROR:", err)
			os.Exit(1)
		}
		fmt.Printf("resumed %d messages from %s\n", len(msgs), cfg.resumePath)
	}
	fmt.Println("=== win7-agent exec (headless) ===")
	if len(msgs) == 0 {
		if s := loadAgentMd(cfg.workspace); s != "" {
			pushMsg(&msgs, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: s})
			fmt.Printf("[AGENT.md] 已注入项目约定（%d 字节）\n", len(s))
		}
	}
	pushMsg(&msgs, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: prompt})
	_, err = streamTurn(client, reg, cfg, &msgs)
	if interrupted() {
		finalizeInterrupted(&msgs, reg)
		os.Exit(130)
	}
	endOfTaskSummary(reg, errors.Is(err, errMaxRounds))
	if err != nil {
		fmt.Println("EXEC-ERROR:", err)
		os.Exit(1)
	}
	fmt.Println("=== EXEC-DONE ===")
}

func runRepl(cfg *config) {
	taskID := newTaskID()
	client := newClient(cfg)
	reg, err := setupEnv(cfg, taskID)
	if err != nil {
		fmt.Println("SETUP-ERROR:", err)
		os.Exit(1)
	}
	s, err := openSessionFor(cfg, taskID)
	if err == nil {
		sess = s
		defer sess.Close()
	}
	defer sessionEndCleanup(curRunner, curCfg)
	var msgs []openai.ChatCompletionMessage
	if cfg.resumePath != "" {
		msgs, err = loadSession(cfg.resumePath)
		if err != nil {
			fmt.Println("RESUME-ERROR:", err)
			os.Exit(1)
		}
		fmt.Printf("resumed %d messages\n", len(msgs))
	}
	fmt.Println("win7-agent M2 REPL (model:", cfg.model, "workspace:", reg.policy.Workspace, ")")
	if len(msgs) == 0 {
		if s := loadAgentMd(cfg.workspace); s != "" {
			pushMsg(&msgs, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: s})
			fmt.Printf("[AGENT.md] 已注入项目约定（%d 字节）\n", len(s))
		}
	}
	fmt.Println("commands: /exit /clear")
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 64*1024), 256*1024)
	for {
		fmt.Print("> ")
		if !sc.Scan() {
			break
		}
		line := strings.TrimSpace(sc.Text())
		switch line {
		case "":
			continue
		case "/exit", "/quit":
			return
		case "/clear":
			msgs = nil
			fmt.Println("[context cleared]")
			continue
		}
		pushMsg(&msgs, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: line})
		_, stErr := streamTurn(client, reg, cfg, &msgs)
		if interrupted() {
			finalizeInterrupted(&msgs, reg)
			continue
		}
		endOfTaskSummary(reg, errors.Is(stErr, errMaxRounds))
		if stErr != nil {
			fmt.Println("ERROR:", stErr)
		}
	}
}

// streamTurn runs one agent turn: stream reply; execute tool calls; feed results; loop until final answer.
func streamTurn(client *openai.Client, reg *Registry, cfg *config, msgs *[]openai.ChatCompletionMessage) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	turnCancel = cancel
	defer func() { turnCancel = nil; cancel() }()
	for round := 0; round < 30; round++ {
		if interrupted() {
			return "", errInterrupted
		}
		maybeCompressContext(ctx, client, cfg, msgs)
		maybeCompressContext(ctx, client, cfg, msgs)
		req := openai.ChatCompletionRequest{
			Model:    cfg.model,
			Messages: *msgs,
			Tools:    reg.Definitions(),
			Stream:   true,
		}
		stream, err := client.CreateChatCompletionStream(ctx, req)
		if err != nil {
			return "", err
		}
		var content strings.Builder
		toolAcc := map[int]*openai.ToolCall{}
		var order []int
		for {
			chunk, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				stream.Close()
				if interrupted() {
					return "", errInterrupted
				}
				return "", err
			}
			if len(chunk.Choices) == 0 {
				continue
			}
			d := chunk.Choices[0].Delta
			if d.Content != "" {
				fmt.Print(d.Content)
				content.WriteString(d.Content)
			}
			for _, tc := range d.ToolCalls {
				idx := 0
				if tc.Index != nil {
					idx = *tc.Index
				}
				e, ok := toolAcc[idx]
				if !ok {
					e = &openai.ToolCall{ID: tc.ID, Type: openai.ToolTypeFunction, Function: openai.FunctionCall{}}
					toolAcc[idx] = e
					order = append(order, idx)
				}
				if tc.ID != "" {
					e.ID = tc.ID
				}
				if tc.Function.Name != "" {
					e.Function.Name = tc.Function.Name
				}
				e.Function.Arguments += tc.Function.Arguments
			}
		}
		stream.Close()
		if len(toolAcc) == 0 {
			fmt.Println()
			// M4-T0: persist the final assistant answer so the session file
			// distinguishes convergence from cap-stop and --resume sees it.
			pushMsg(msgs, openai.ChatCompletionMessage{
				Role: openai.ChatMessageRoleAssistant, Content: content.String(),
			})
			return content.String(), nil
		}
		calls := make([]openai.ToolCall, 0, len(order))
		for _, i := range order {
			c := *toolAcc[i]
			c.Index = nil
			calls = append(calls, c)
		}
		pushMsg(msgs, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, ToolCalls: calls})
		for _, c := range calls {
			if interrupted() {
				return "", errInterrupted
			}
			fmt.Printf("\n[tool] %s(%s)\n", c.Function.Name, c.Function.Arguments)
			res := reg.Execute(c.Function.Name, c.Function.Arguments)
			preview := strings.ReplaceAll(res, "\n", " | ")
			if len(preview) > 300 {
				preview = preview[:300] + " ..."
			}
			fmt.Printf("[tool-result] %s\n", preview)
			pushMsg(msgs, openai.ChatCompletionMessage{
				Role: openai.ChatMessageRoleTool, ToolCallID: c.ID, Content: res,
			})
		}
	}
	return "", errMaxRounds
}

// truncateContext: minimal context manager — drop oldest middle messages over the char budget.
func truncateContext(msgs *[]openai.ChatCompletionMessage, maxChars int) {
	total := 0
	for _, m := range *msgs {
		total += len(m.Content)
	}
	for total > maxChars && len(*msgs) > 2 {
		total -= len((*msgs)[1].Content)
		*msgs = append((*msgs)[:1], (*msgs)[2:]...)
	}
}

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return os.Getenv("USERPROFILE")
}

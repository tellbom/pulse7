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
	"path/filepath"
	"strings"
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
	pushMsg(&msgs, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: prompt})
	_, err = streamTurn(client, reg, cfg, &msgs)
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
		endOfTaskSummary(reg, errors.Is(stErr, errMaxRounds))
		if stErr != nil {
			fmt.Println("ERROR:", stErr)
		}
	}
}

// streamTurn runs one agent turn: stream reply; execute tool calls; feed results; loop until final answer.
func streamTurn(client *openai.Client, reg *Registry, cfg *config, msgs *[]openai.ChatCompletionMessage) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	// M4-T0: cap raised 8->30; RC0.1's toy tasks burned all 8 rounds doing
	// useful work (fix verified) and got cut off before the final answer.
	for round := 0; round < 30; round++ {
		truncateContext(msgs, cfg.maxCtx)
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

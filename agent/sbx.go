package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// sbxRunner: Sandboxie adapter. Runs commands inside the box via Start.exe.
// stdout/stderr/exit-code are carried by wrapper files written inside the box
// and read back from the container view (M0.5 findings, see freeze manifest).
type sbxRunner struct {
	StartExe    string
	Box         string
	SandboxRoot string
	Home        string
	Workspace   string
	Timeout     time.Duration

	curCmd *exec.Cmd // current Start.exe host process while a command runs
}

func (s *sbxRunner) Mode() string { return "Sandboxie" }

// Interrupt stops the current execution: kill the Start.exe host we spawned
// and terminate the whole dedicated box (M4-T1 Ctrl-C path).
func (s *sbxRunner) Interrupt() {
	if c := s.curCmd; c != nil && c.Process != nil {
		c.Process.Kill()
	}
	exec.Command(s.StartExe, "/box:"+s.Box, "/terminate").Run()
}

func (s *sbxRunner) Run(command string) (string, int, error) {
	rf, err := buildRunFiles(s.Home, s.Workspace, command)
	if err != nil {
		return "", -1, err
	}
	defer os.RemoveAll(rf.dir)
	// M3-C: kill box stragglers after every run (dedicated agent box).
	defer afterShellCleanup(s)

	ctx, cancel := context.WithTimeout(context.Background(), s.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.StartExe, "/silent", "/box:"+s.Box, "/wait", "cmd.exe", "/c", rf.batPath)
	cmd.SysProcAttr = sysProcHidden()
	s.curCmd = cmd
	defer func() { s.curCmd = nil }()
	consoleOut, runErr := cmd.CombinedOutput()
	timedOut := ctx.Err() == context.DeadlineExceeded
	if timedOut {
		exec.Command(s.StartExe, "/box:"+s.Box, "/terminate").Run()
		time.Sleep(800 * time.Millisecond)
	}

	ctnDir := filepath.Join(s.SandboxRoot, userName(), s.Box, "user", "current", ".pulse7", "run", rf.id)
	out, ec := readResult(ctnDir)

	if timedOut {
		return out + "\n[TIMEOUT: box terminated]", ec, nil
	}
	if runErr != nil && ec == -1 {
		return fmt.Sprintf("start.exe failed: %v\nconsole: %s", runErr, consoleOut), -1, runErr
	}
	if ec == -1 && len(out) == 0 {
		// T2: wrapper produced no output AND no exit code — the sandbox
		// blocked the command from starting. Tell the model explicitly so
		// it stops retrying the same command.
		return "[沙盒阻止] 该命令在 Sandboxie 沙盒内无法启动（wrapper 无输出，exitcode=-1）。" +
			"这是环境限制，不是命令本身的错误——重试同一命令不会成功。" +
			"可考虑：改用其他方式验证；或向用户说明需要为该程序配置沙盒例外。", -1, nil
	}
	return out, ec, nil
}

func sysProcHidden() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true}
}

func userName() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		if i := strings.LastIndex(u.Username, `\`); i >= 0 {
			return u.Username[i+1:]
		}
		return u.Username
	}
	return os.Getenv("USERNAME")
}

var _ = strconv.Itoa

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
}

func (s *sbxRunner) Mode() string { return "Sandboxie" }

func (s *sbxRunner) Run(command string) (string, int, error) {
	rf, err := buildRunFiles(s.Home, s.Workspace, command)
	if err != nil {
		return "", -1, err
	}
	defer os.RemoveAll(rf.dir)

	ctx, cancel := context.WithTimeout(context.Background(), s.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.StartExe, "/silent", "/box:"+s.Box, "/wait", "cmd.exe", "/c", rf.batPath)
	cmd.SysProcAttr = sysProcHidden()
	consoleOut, runErr := cmd.CombinedOutput()
	timedOut := ctx.Err() == context.DeadlineExceeded
	if timedOut {
		exec.Command(s.StartExe, "/box:"+s.Box, "/terminate").Run()
		time.Sleep(800 * time.Millisecond)
	}

	ctnDir := filepath.Join(s.SandboxRoot, userName(), s.Box, "user", "current", ".win7-agent", "run", rf.id)
	out, ec := readResult(ctnDir)

	if timedOut {
		return out + "\n[TIMEOUT: box terminated]", ec, nil
	}
	if runErr != nil && ec == -1 {
		return fmt.Sprintf("start.exe failed: %v\nconsole: %s", runErr, consoleOut), -1, runErr
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

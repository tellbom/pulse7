package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// sandboxRunner abstracts the shell-execution sandbox.
// Exactly two runtime outcomes: Sandboxie | JobObject(auto-degraded).
type sandboxRunner interface {
	Run(command string) (string, int, error)
	Mode() string
	// Interrupt kills whatever this runner is currently executing (M4-T1).
	Interrupt()
}

// probeStartExe validates the REAL production shell path end-to-end: it runs
// a trivial command through the same wrapper mechanism the shell tool uses
// and requires its output to appear in the CONTAINER view. Any failure mode
// (service dead, driver dead, silent unsandboxed fallback, hangs) leaves the
// container empty and degrades to JobObject. Exit codes lie; side effects do not.
func probeStartExe(s *sbxRunner) bool {
	rf, err := buildRunFiles(s.Home, s.Workspace, "echo PROBE-OK")
	if err != nil {
		probeDebug = "buildRunFiles: " + err.Error()
		return false
	}
	defer os.RemoveAll(rf.dir)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.StartExe, "/silent", "/box:"+s.Box,
		"/wait", "cmd.exe", "/c", rf.batPath)
	cmd.SysProcAttr = sysProcHidden()
	runErr := cmd.Run()
	ctnDir := filepath.Join(s.SandboxRoot, userName(), s.Box,
		"user", "current", ".win7-agent", "run", rf.id)
	out, ec := readResult(ctnDir)
	probeDebug = fmt.Sprintf("bat=%s runErr=%v ctxErr=%v ec=%d out=%q ctn=%s",
		rf.batPath, runErr, ctx.Err(), ec, out, ctnDir)
	return ctx.Err() == nil && runErr == nil && ec == 0 && strings.Contains(out, "PROBE-OK")
}

// probeDebug carries the last probe internals for doctor/support output.
var probeDebug string

func jobRunner(cfg *config, ws string) *jobObjectRunner {
	mb := uint64(0)
	if cfg.memLimitMB > 0 {
		mb = uint64(cfg.memLimitMB)
	} else {
		mb = 2048
	}
	return &jobObjectRunner{
		Workspace: ws, Home: homeDir(), Timeout: cfg.shellTimeout, MemLimitMB: mb,
	}
}

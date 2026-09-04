package main

import (
	"context"
	"os"
	"os/exec"
	"time"
)

// sandboxRunner abstracts the shell-execution sandbox.
// Selection is automatic and degradation is silent-by-default:
// Sandboxie when its service/driver is healthy, JobObject otherwise.
// System patching (KB4474419) is a DEV/TEST-only validation condition and is
// NEVER a production prerequisite.
type sandboxRunner interface {
	Run(command string) (string, int, error)
	Mode() string
}

func detectSandbox(cfg *config, ws string) (sandboxRunner, string) {
	s := &sbxRunner{
		StartExe:    cfg.startExe,
		Box:         cfg.box,
		SandboxRoot: cfg.sandboxRoot,
		Home:        homeDir(),
		Workspace:   ws,
		Timeout:     cfg.shellTimeout,
	}
	if _, err := os.Stat(cfg.startExe); err != nil {
		return jobRunner(cfg, ws), "Sandboxie Start.exe not installed"
	}
	if !probeStartExe(s) {
		return jobRunner(cfg, ws), "Sandboxie service/driver unavailable (auto-degraded, no system patch required)"
	}
	return s, ""
}

func jobRunner(cfg *config, ws string) *jobObjectRunner {
	return &jobObjectRunner{
		Workspace: ws, Home: homeDir(), Timeout: cfg.shellTimeout, MemLimitMB: 2048,
	}
}

// probeStartExe performs a cheap, dialog-suppressed functional probe.
func probeStartExe(s *sbxRunner) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.StartExe, "/silent", "/box:"+s.Box, "/listpids")
	cmd.SysProcAttr = sysProcHidden()
	return ctx.Err() == nil && cmd.Run() == nil
}

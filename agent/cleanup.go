package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func quietExec(name string, args ...string) {
	if name == "" {
		return
	}
	exec.Command(name, args...).Run()
}

// Cleanup design (revised RC 0.3.2):
//
//   per shell run    -> wrapper temp dir removed (defer in runners) ONLY.
//                       NO /terminate here — the per-shell /terminate was
//                       killing the box before the next command could start,
//                       causing ~86% intermittent shell failures (finding #1).
//   session end      -> terminate box + delete_sandbox_silent (config-gated)
//   startup          -> purge stale wrapper dirs (>1h) left by crashed runs
//
// Process leak risk: commands that daemonize (rare in a coding agent's
// usage) could linger in the box until session end. The session-end
// /terminate catches them. For extreme cases, /box /listpids can detect
// accumulation, but per-shell termination caused more harm than good.
func afterShellCleanup(runner sandboxRunner) {
	// Intentionally empty: see comment above. Wrapper dir cleanup happens
	// in the runner's defer os.RemoveAll(rf.dir), which is sufficient.
}

func sessionEndCleanup(runner sandboxRunner, cfg *config) {
	if s, ok := runner.(*sbxRunner); ok && cfg.cleanupOnExit {
		quietExec(s.StartExe, "/box:"+s.Box, "/terminate")
		quietExec(s.StartExe, "/box:"+s.Box, "delete_sandbox_silent")
	}
	purgeStaleRunDirs(homeDir(), time.Hour)
}

// purgeStaleRunDirs: remove leftover wrapper staging dirs older than maxAge.
func purgeStaleRunDirs(home string, maxAge time.Duration) {
	runDir := filepath.Join(home, ".pulse7", "run")
	entries, err := os.ReadDir(runDir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !e.IsDir() {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.RemoveAll(filepath.Join(runDir, e.Name()))
		}
	}
}

// purgeStaleIndexTemps: checkpoint temp indexes (index-*.tmp) under
// data\sessions have no other cleanup path; drop those older than maxAge
// (fresh ones may belong to a still-running session).
func purgeStaleIndexTemps(sessionsDir string, maxAge time.Duration) {
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "index-") || !strings.HasSuffix(e.Name(), ".tmp") {
			continue
		}
		if info, err := e.Info(); err == nil && info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(sessionsDir, e.Name()))
		}
	}
}

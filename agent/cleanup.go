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

// M3-C: automatic cleanup. Product rule: the agent controls its own garbage;
// user workspaces are never touched.
//
//   per shell run    -> wrapper temp dir removed (defer in runners) + box terminate
//   session end      -> terminate box + delete_sandbox_silent (config-gated)
//   startup          -> purge stale wrapper dirs (>1h) left by crashed runs
func afterShellCleanup(runner sandboxRunner) {
	s, ok := runner.(*sbxRunner)
	if !ok {
		return
	}
	// dedicated agent box: terminating after each run kills stragglers
	// (long sleepers, orphaned children) without side effects.
	quietExec(s.StartExe, "/box:"+s.Box, "/terminate")
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
	runDir := filepath.Join(home, ".win7-agent", "run")
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

// purgeStaleIndexTemps: checkpoint temp indexes (index-<task>-<seq>.tmp)
// under data\sessions have no other cleanup path; drop those older than
// maxAge (fresh ones belong to possibly-still-running sessions).
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

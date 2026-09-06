package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"
)

var (
	procGetCurrentProcessId  = k32.NewProc("GetCurrentProcessId")
	procProcessIdToSessionId = k32.NewProc("ProcessIdToSessionId")
)

func currentSessionID() uint32 {
	pid, _, _ := procGetCurrentProcessId.Call()
	var sid uint32
	r1, _, _ := procProcessIdToSessionId.Call(pid, uintptr(unsafe.Pointer(&sid)))
	if r1 == 0 {
		return 0
	}
	return sid
}

// M3-A environment detection. Produces exactly one of two runtime outcomes:
// Sandboxie | JobObject(auto-degraded). Never a third "half-usable" state.
// The final arbiter is a REAL run probe — Start.exe existing is not enough
// (measured: SbieSvc down with driver loaded can wedge process creation).
func selectSandboxMode(cfg *config, ws string, verbose bool) (sandboxRunner, string, []string) {
	var r []string
	add := func(format string, a ...interface{}) {
		r = append(r, fmt.Sprintf(format, a...))
	}

	session := os.Getenv("SESSIONNAME")
	sid := currentSessionID()
	// SESSIONNAME is unreliable across task-scheduler contexts; the session ID
	// is authoritative: 0 = service/headless (no desktop), >0 = console/RDP.
	interactive := sid != 0
	add("session: id=%d SESSIONNAME=%q interactive=%v", sid, session, interactive)

	osv := osVersion()
	add("os: %s arch=%s", osv, archDesc())

	gitExe := filepath.Join(cfg.exeDirStore(), "runtime", "git", "cmd", "git.exe")
	if _, err := os.Stat(gitExe); err == nil {
		out, err := exec.Command(gitExe, "--version").Output()
		add("mingit: %s (%v)", strings.TrimSpace(string(out)), err)
	} else {
		add("mingit: MISSING at %s (checkpoint/rollback will error)", gitExe)
	}

	startExe := cfg.startExe
	_, statErr := os.Stat(startExe)
	add("start.exe: %s present=%v", startExe, statErr == nil)
	add("sbiesvc/sbiedrv: %s", serviceStates())

	job := func(reason string) (sandboxRunner, string, []string) {
		add("probe result: %s -> JobObject (auto-degraded)", reason)
		add("probe detail: %s", probeDebug)
		return jobRunner(cfg, ws), reason, r
	}

	switch cfg.sandboxPreference {
	case "jobobject":
		return job("preference=jobobject")
	case "sandboxie":
		// explicit preference still cannot force a broken sandbox: degrade honestly
	}

	if !interactive {
		return job("no interactive desktop session (Sandboxie requires a desktop session; JobObject is the headless default)")
	}
	if statErr != nil {
		return job("Sandboxie Start.exe not installed")
	}
	if ok := probeStartExeRunner(startExe, cfg.box, homeDir(), ws, cfg.sandboxRoot); !ok {
		return job("Sandboxie service/driver unavailable (real-run probe failed; no system patch required or requested)")
	}
	if err := ensureSandboxieConfig(startExe, cfg.box, ws); err != nil {
		add("sandboxie config ensure: %v (continuing with existing config)", err)
	} else {
		add("sandboxie config: box=%s OpenFilePath=%s (reloaded)", cfg.box, ws)
	}
	add("probe result: OK -> Sandboxie")
	return &sbxRunner{
		StartExe: startExe, Box: cfg.box, SandboxRoot: cfg.sandboxRoot,
		Home: homeDir(), Workspace: ws, Timeout: cfg.shellTimeout,
	}, "", r
}

// probeStartExeRunner: cheap wrapper building a fully-populated runner for
// the probe (Home/Workspace/SandboxRoot are required: the probe uses the
// production wrapper-file mechanism).
func probeStartExeRunner(startExe, box, home, ws, sandboxRoot string) bool {
	return probeStartExe(&sbxRunner{
		StartExe: startExe, Box: box, Home: home, Workspace: ws, SandboxRoot: sandboxRoot,
	})
}

func osVersion() string {
	out, err := exec.Command("cmd", "/c", "ver").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func archDesc() string {
	if os.Getenv("PROCESSOR_ARCHITEW6432") != "" {
		return "x64 (WOW64)"
	}
	return os.Getenv("PROCESSOR_ARCHITECTURE")
}

func serviceStates() string {
	get := func(name string) string {
		out, err := exec.Command("sc", "query", name).CombinedOutput()
		if err != nil {
			return name + "=absent"
		}
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "STATE") {
				return name + "=" + strings.TrimSpace(strings.SplitN(strings.TrimSpace(line), ":", 2)[1])
			}
		}
		return name + "=unknown"
	}
	return get("SbieSvc") + " " + get("SbieDrv")
}

// doctorCmd: full environment report for support/install logs.
func doctorCmd(cfg *config) {
	fmt.Println("=== pulse7 doctor ===")
	ws, _ := filepath.Abs(cfg.workspace)
	fmt.Printf("exe-dir: %s\n", cfg.exeDirStore())
	cfgDir := filepath.Dir(configPath(cfg.exeDirStore()))
	probe := filepath.Join(cfgDir, "..", "data")
	if err := os.MkdirAll(probe, 0o755); err != nil {
		fmt.Printf("config/data writable: FAIL (%v)\n", err)
	} else {
		fmt.Printf("config/data writable: OK (%s)\n", probe)
	}
	fmt.Printf("config file: %s (exists=%v)\n", configPath(cfg.exeDirStore()), fileExists(configPath(cfg.exeDirStore())))
	fmt.Printf("workspace: %s\n", ws)
	if fi, err := os.Stat(filepath.Join(ws, "AGENT.md")); err == nil {
		fmt.Printf("agent.md: detected (%d bytes, injected into system prompt)\n", fi.Size())
	} else {
		fmt.Println("agent.md: not found (optional project conventions file)")
	}

	runner, reason, report := selectSandboxMode(cfg, ws, true)
	for _, line := range report {
		fmt.Println(line)
	}
	if reason == "" {
		fmt.Printf("MODE: Sandboxie (box=%s)\n", cfg.box)
	} else {
		fmt.Printf("MODE: JobObject (auto-degraded: %s) - no system patch required\n", reason)
	}
	_ = runner
	fmt.Println("=== pulse7 doctor done ===")
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

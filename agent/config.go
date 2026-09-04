package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"time"
)

// agentConfig: user-facing configuration (config\agent.json).
// Only user-level concerns are exposed; Sandboxie internals stay hidden.
type agentConfig struct {
	BaseURL            string `json:"base_url"`
	APIKey             string `json:"api_key"`
	Model              string `json:"model"`
	Workspace          string `json:"workspace"`
	Box                string `json:"box"`
	StartExe           string `json:"start_exe"`
	SandboxRoot        string `json:"sandbox_root"`
	SandboxPreference  string `json:"sandbox_preference"` // auto | sandboxie | jobobject
	YOLO               bool   `json:"yolo"`
	ShellTimeoutSec    int    `json:"shell_timeout_sec"`
	MemoryLimitMB      int    `json:"memory_limit_mb"`
	MaxCtx             int    `json:"max_ctx"`
	CleanupOnExit      bool   `json:"cleanup_on_exit"`
}

func defaultAgentConfig() agentConfig {
	return agentConfig{
		BaseURL:           "http://127.0.0.1:8080/v1",
		APIKey:            "dummy",
		Model:             "mock-model",
		Workspace:         ".",
		Box:               "Win7Agent",
		StartExe:          `C:\Program Files\Sandboxie\Start.exe`,
		SandboxRoot:       `C:\Sandbox`,
		SandboxPreference: "auto",
		YOLO:              false,
		ShellTimeoutSec:   120,
		MemoryLimitMB:     2048,
		MaxCtx:            48000,
		CleanupOnExit:     true,
	}
}

func configPath(exeDir string) string {
	return filepath.Join(exeDir, "config", "agent.json")
}

func loadAgentConfig(path string) agentConfig {
	ac := defaultAgentConfig()
	b, err := os.ReadFile(path)
	if err != nil {
		return ac
	}
	json.Unmarshal(b, &ac)
	return ac
}

func writeAgentConfigTemplate(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil // never overwrite user config
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(defaultAgentConfig(), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0644)
}

// applyConfigToFlags: config file values fill fields whose flag was NOT set
// explicitly on the command line. Precedence: flag > config > default.
func applyConfigToFlags(cfg *config, ac agentConfig, fs *flag.FlagSet) {
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	use := func(name string, apply func()) {
		if !set[name] {
			apply()
		}
	}
	use("base-url", func() { cfg.baseURL = ac.BaseURL })
	use("api-key", func() { cfg.apiKey = ac.APIKey })
	use("model", func() { cfg.model = ac.Model })
	use("workspace", func() { cfg.workspace = ac.Workspace })
	use("box", func() { cfg.box = ac.Box })
	use("start-exe", func() { cfg.startExe = ac.StartExe })
	use("sandbox-root", func() { cfg.sandboxRoot = ac.SandboxRoot })
	use("sandbox-preference", func() { cfg.sandboxPreference = ac.SandboxPreference })
	use("yolo", func() { cfg.yolo = ac.YOLO })
	use("shell-timeout", func() { if ac.ShellTimeoutSec > 0 { cfg.shellTimeout = time.Duration(ac.ShellTimeoutSec) * time.Second } })
	use("memory-limit-mb", func() { cfg.memLimitMB = ac.MemoryLimitMB })
	use("max-ctx", func() { if ac.MaxCtx > 0 { cfg.maxCtx = ac.MaxCtx } })
	use("cleanup-on-exit", func() { cfg.cleanupOnExit = ac.CleanupOnExit })
}

var _ = os.Getenv

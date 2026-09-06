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
	// T1 (slow-network): watchdog timeouts in seconds. First-chunk covers
	// queueing before the first data block; idle covers the gap between
	// chunks and is reset by every chunk (slow streams are never killed).
	LLMFirstChunkTimeoutSec int `json:"llm_first_chunk_timeout_sec"`
	LLMIdleTimeoutSec       int `json:"llm_idle_timeout_sec"`
	// T2 (slow-network): retry count for retryable LLM failures (backoff
	// 5s / 15s). Non-retryable errors (4xx, balance-exhausted, idle timeout)
	// never consume retries.
	LLMMaxRetries int `json:"llm_max_retries"`
	// T3 (slow-network): the context-compression summarize call runs under
	// its own timeout so it cannot starve (or be starved by) the main loop.
	LLMCompressTimeoutSec int `json:"llm_compress_timeout_sec"`
}

func defaultAgentConfig() agentConfig {
	return agentConfig{
		BaseURL:           "http://127.0.0.1:8080/v1",
		APIKey:            "dummy",
		Model:             "mock-model",
		Workspace:         ".",
		Box:               "Win7Agent", // Sandboxie box name kept for compat; changing would orphan existing boxes
		StartExe:          `C:\Program Files\Sandboxie\Start.exe`,
		SandboxRoot:       `C:\Sandbox`,
		SandboxPreference: "auto",
		YOLO:              false,
		ShellTimeoutSec:   120,
		MemoryLimitMB:     2048,
		MaxCtx:            48000,
		CleanupOnExit:     true,
		// T1 defaults: generous first-chunk (queueing on a slow intranet can
		// take minutes) and a 2-minute idle gap before a stream is judged dead.
		LLMFirstChunkTimeoutSec: 300,
		LLMIdleTimeoutSec:       120,
		LLMMaxRetries:           2,
		LLMCompressTimeoutSec:   180,
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
	// JSON has no comments - ship the field docs as adjacent "_doc_" keys
	// (unknown keys are ignored on load).
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return os.WriteFile(path, append(b, '\n'), 0644)
	}
	for k, v := range map[string]string{
		"_doc_llm_first_chunk_timeout_sec": "首字节超时(秒)：请求发出到收到第一个数据块的等待上限。内网高峰排队慢可调大；超时后会自动重试",
		"_doc_llm_idle_timeout_sec":         "空闲超时(秒)：流式响应中相邻数据块的最大间隔，每收到数据即重置。持续吐字再慢也不会被掐断；只有长时间无任何数据才判定卡死",
		"_doc_llm_max_retries":              "LLM 网络失败自动重试次数（退避 5 秒/15 秒）。连接失败、5xx、429、首字节超时会重试；鉴权/余额错误不重试",
		"_doc_llm_compress_timeout_sec":     "上下文压缩调用的独立超时(秒)，不影响主对话",
	} {
		m[k] = v
	}
	b, err = json.MarshalIndent(m, "", "  ")
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
	use("api-key", func() {
		cfg.apiKey = ac.APIKey
		if cfg.apiKey == "" || cfg.apiKey == "REPLACE_ME" || cfg.apiKey == "dummy" {
			cfg.apiKey = os.Getenv("PULSE7_API_KEY") // pulse7 primary
			if cfg.apiKey == "" {
				cfg.apiKey = os.Getenv("WIN7_AGENT_API_KEY") // legacy fallback
			}
		}
	})
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
	use("llm-first-chunk-timeout", func() {
		if ac.LLMFirstChunkTimeoutSec > 0 {
			cfg.llmFirstChunkTimeout = time.Duration(ac.LLMFirstChunkTimeoutSec) * time.Second
		}
	})
	use("llm-idle-timeout", func() {
		if ac.LLMIdleTimeoutSec > 0 {
			cfg.llmIdleTimeout = time.Duration(ac.LLMIdleTimeoutSec) * time.Second
		}
	})
	use("llm-max-retries", func() {
		if ac.LLMMaxRetries > 0 {
			cfg.llmMaxRetries = ac.LLMMaxRetries
		}
	})
	use("llm-compress-timeout", func() {
		if ac.LLMCompressTimeoutSec > 0 {
			cfg.llmCompressTimeout = time.Duration(ac.LLMCompressTimeoutSec) * time.Second
		}
	})
}

var _ = os.Getenv

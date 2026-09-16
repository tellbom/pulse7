package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// agentConfig: user-facing configuration shared by global and project layers.
// Only user-level concerns are exposed; Sandboxie internals stay hidden.
type agentConfig struct {
	BaseURL                   string `json:"base_url"`
	APIKey                    string `json:"api_key"`
	Model                     string `json:"model"`
	Workspace                 string `json:"workspace"`
	Box                       string `json:"box"`
	StartExe                  string `json:"start_exe"`
	SandboxRoot               string `json:"sandbox_root"`
	SandboxPreference         string `json:"sandbox_preference"` // auto | sandboxie | jobobject
	YOLO                      bool   `json:"yolo"`
	ReadOnly                  bool   `json:"read_only"`
	ShellTimeoutSec           int    `json:"shell_timeout_sec"`
	MemoryLimitMB             int    `json:"memory_limit_mb"`
	MaxCtx                    int    `json:"max_ctx"`
	MaxRounds                 int    `json:"max_rounds"`
	SkillCatalogBudgetBytes   int    `json:"skill_catalog_budget_bytes"`
	MicroKeepRecent           int    `json:"micro_keep_recent"`
	ProcessWarnThreshold      int    `json:"process_warn_threshold"`
	BackgroundTaskMaxOutputMB int    `json:"background_task_max_output_mb"`
	BackgroundTaskWarnCount   int    `json:"background_task_warn_count"`
	BackgroundTaskWarnSec     int    `json:"background_task_warn_sec"`
	CleanupOnExit             bool   `json:"cleanup_on_exit"`
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

const defaultMaxContextBytes = 256000

func defaultAgentConfig() agentConfig {
	return agentConfig{
		BaseURL:                   "http://127.0.0.1:8080/v1",
		APIKey:                    "dummy",
		Model:                     "mock-model",
		Workspace:                 ".",
		Box:                       "Win7Agent", // Sandboxie box name kept for compat; changing would orphan existing boxes
		StartExe:                  `C:\Program Files\Sandboxie\Start.exe`,
		SandboxRoot:               `C:\Sandbox`,
		SandboxPreference:         "auto",
		YOLO:                      false,
		ReadOnly:                  false,
		ShellTimeoutSec:           120,
		MemoryLimitMB:             2048,
		MaxCtx:                    defaultMaxContextBytes,
		MaxRounds:                 100,
		MicroKeepRecent:           defaultMicroKeepRecent,
		SkillCatalogBudgetBytes:   defaultSkillCatalogBudgetBytes,
		ProcessWarnThreshold:      defaultProcessWarnThreshold,
		BackgroundTaskMaxOutputMB: defaultBackgroundTaskMaxOutputMB,
		BackgroundTaskWarnCount:   defaultBackgroundTaskWarnCount,
		BackgroundTaskWarnSec:     defaultBackgroundTaskWarnSec,
		CleanupOnExit:             true,
		// T1 defaults: generous first-chunk (queueing on a slow intranet can
		// take minutes) and a 2-minute idle gap before a stream is judged dead.
		LLMFirstChunkTimeoutSec: 300,
		LLMIdleTimeoutSec:       120,
		LLMMaxRetries:           2,
		LLMCompressTimeoutSec:   180,
	}
}

func globalConfigPath(home string) string {
	return filepath.Join(home, ".pulse7", "config.json")
}

func projectConfigPath(workspace string) string {
	return filepath.Join(workspace, ".pulse7", "config.json")
}

func flagWasSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}

func readConfigDocument(path string) (map[string]json.RawMessage, bool, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read config %s: %w", path, err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, true, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return doc, true, nil
}

func mergeAgentConfig(base agentConfig, overlay map[string]json.RawMessage) (agentConfig, error) {
	b, err := json.Marshal(base)
	if err != nil {
		return agentConfig{}, err
	}
	var merged map[string]json.RawMessage
	if err := json.Unmarshal(b, &merged); err != nil {
		return agentConfig{}, err
	}
	for key, value := range overlay {
		merged[key] = value
	}
	b, err = json.Marshal(merged)
	if err != nil {
		return agentConfig{}, err
	}
	var result agentConfig
	if err := json.Unmarshal(b, &result); err != nil {
		return agentConfig{}, err
	}
	return result, nil
}

func loadLayeredAgentConfig(flagWorkspace string, workspaceFlagSet bool) (agentConfig, []string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return agentConfig{}, nil, fmt.Errorf("resolve user profile: %w", err)
	}
	config := defaultAgentConfig()
	globalPath := globalConfigPath(home)
	global, _, err := readConfigDocument(globalPath)
	if err != nil {
		return agentConfig{}, nil, err
	}
	if global != nil {
		config, err = mergeAgentConfig(config, global)
		if err != nil {
			return agentConfig{}, nil, fmt.Errorf("merge global config %s: %w", globalPath, err)
		}
	}
	discoveryWorkspace := config.Workspace
	if workspaceFlagSet {
		discoveryWorkspace = flagWorkspace
	}
	discoveryWorkspace, err = filepath.Abs(discoveryWorkspace)
	if err != nil {
		return agentConfig{}, nil, fmt.Errorf("resolve project workspace: %w", err)
	}
	projectPath := projectConfigPath(discoveryWorkspace)
	project, _, err := readConfigDocument(projectPath)
	if err != nil {
		return agentConfig{}, nil, err
	}
	var warnings []string
	if project != nil {
		hadAPIKey := false
		for key := range project {
			if strings.EqualFold(key, "api_key") {
				delete(project, key)
				hadAPIKey = true
			}
		}
		if hadAPIKey {
			warnings = append(warnings, fmt.Sprintf("项目配置 %s 中的 api_key 已忽略；密钥只能放在全局配置或环境变量中", projectPath))
		}
		config, err = mergeAgentConfig(config, project)
		if err != nil {
			return agentConfig{}, nil, fmt.Errorf("merge project config %s: %w", projectPath, err)
		}
	}
	return config, warnings, nil
}

func writeProjectAgentConfig(path string, content []byte) error {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(content, &doc); err != nil {
		return fmt.Errorf("invalid project config: %w", err)
	}
	for key := range doc {
		if strings.EqualFold(key, "api_key") {
			return fmt.Errorf("项目配置禁止写入 api_key；请写入全局配置或使用环境变量")
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	normalized, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(normalized, '\n'), 0o644)
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
		"_doc_llm_first_chunk_timeout_sec":   "首字节超时(秒)：请求发出到收到第一个数据块的等待上限。内网高峰排队慢可调大；超时后会自动重试",
		"_doc_llm_idle_timeout_sec":          "空闲超时(秒)：流式响应中相邻数据块的最大间隔，每收到数据即重置。持续吐字再慢也不会被掐断；只有长时间无任何数据才判定卡死",
		"_doc_llm_max_retries":               "LLM 网络失败自动重试次数（退避 5 秒/15 秒）。连接失败、5xx、429、首字节超时会重试；鉴权/余额错误不重试",
		"_doc_llm_compress_timeout_sec":      "上下文压缩调用的独立超时(秒)，不影响主对话",
		"_doc_read_only":                     "只读模式：代码层拒绝 shell、write、edit、rollback；不依赖模型提示词",
		"_doc_max_rounds":                    "单次任务的工具调用轮次上限，默认 100；触顶表示未得到最终答复，不会宣称任务完成",
		"_doc_skill_catalog_budget_bytes":    "技能目录预算：默认 8192 序列化 UTF-8 字节，范围 1024–1048576；正文按需读取，下个任务生效",
		"_doc_micro_keep_recent":             "本地微压缩保留最近工具结果数，默认 8，范围 1–1000；最新完整工具组始终保留",
		"_doc_process_warn_threshold":        "会话 Job 内进程数告警阈值，默认 50；只告警，不阻止进程",
		"_doc_background_task_max_output_mb": "单个后台任务输出硬上限，默认 50MB；超限截断并标注",
		"_doc_background_task_warn_count":    "并发后台任务告警阈值，默认 5；只告警，不阻止任务",
		"_doc_background_task_warn_sec":      "后台任务运行时长告警秒数，默认 3600；只告警，不终止任务",
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
	use("read-only", func() { cfg.readOnly = ac.ReadOnly })
	use("shell-timeout", func() {
		if ac.ShellTimeoutSec > 0 {
			cfg.shellTimeout = time.Duration(ac.ShellTimeoutSec) * time.Second
		}
	})
	use("memory-limit-mb", func() { cfg.memLimitMB = ac.MemoryLimitMB })
	use("max-ctx", func() {
		if ac.MaxCtx > 0 {
			cfg.maxCtx = ac.MaxCtx
		}
	})
	use("max-rounds", func() {
		if ac.MaxRounds > 0 {
			cfg.maxRounds = ac.MaxRounds
		}
	})
	use("skill-catalog-budget-bytes", func() { cfg.skillCatalogBudgetBytes = ac.SkillCatalogBudgetBytes })
	use("micro-keep-recent", func() {
		cfg.microKeepRecent = ac.MicroKeepRecent
	})
	use("process-warn-threshold", func() { cfg.processWarnThreshold = ac.ProcessWarnThreshold })
	use("background-task-max-output-mb", func() { cfg.backgroundTaskMaxOutputMB = ac.BackgroundTaskMaxOutputMB })
	use("background-task-warn-count", func() { cfg.backgroundTaskWarnCount = ac.BackgroundTaskWarnCount })
	use("background-task-warn-sec", func() { cfg.backgroundTaskWarnSec = ac.BackgroundTaskWarnSec })
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
		if ac.LLMMaxRetries >= 0 {
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

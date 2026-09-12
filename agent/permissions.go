package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type permissionRule struct {
	Tool    string `json:"tool"`
	Pattern string `json:"pattern"`
	Action  string `json:"action"`
}

type permissionConfig struct {
	Profile string           `json:"profile"`
	Rules   []permissionRule `json:"rules"`
}

type permissionDecision struct {
	Action string
	Source string
}

func defaultPermissionConfig() permissionConfig {
	return permissionConfig{Profile: "open", Rules: []permissionRule{}}
}

func permissionsPath(exeDir string) string {
	return filepath.Join(exeDir, "config", "permissions.json")
}

func loadPermissionConfig(path string) (permissionConfig, error) {
	cfg := defaultPermissionConfig()
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return permissionConfig{}, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return permissionConfig{}, fmt.Errorf("parse permissions config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return permissionConfig{}, err
	}
	return cfg, nil
}

func (cfg permissionConfig) validate() error {
	switch cfg.Profile {
	case "strict", "standard", "open":
	default:
		return fmt.Errorf("invalid permission profile %q (want strict, standard, or open)", cfg.Profile)
	}
	for i, rule := range cfg.Rules {
		if strings.TrimSpace(rule.Tool) == "" {
			return fmt.Errorf("permissions rule %d: tool is required", i+1)
		}
		if rule.Pattern == "" {
			return fmt.Errorf("permissions rule %d: pattern is required", i+1)
		}
		switch rule.Action {
		case "allow", "ask", "deny":
		default:
			return fmt.Errorf("permissions rule %d: invalid action %q", i+1, rule.Action)
		}
	}
	return nil
}

func writePermissionConfigTemplate(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	template := map[string]interface{}{
		"_doc_profile": "strict=write/edit/shell/rollback ask; standard=workspace write/edit allow and shell/rollback ask; open=allow. deny rules always win",
		"_doc_rules":   "rules are checked in file order; deny always wins. '*' matches any text including path separators; '?' matches one character; matching is case-insensitive",
		"profile":      "open",
		"rules":        []permissionRule{},
	}
	b, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func (cfg permissionConfig) decide(tool, target string) permissionDecision {
	var first *permissionDecision
	for i, rule := range cfg.Rules {
		if !wildcardMatch(rule.Tool, tool) || !wildcardMatch(rule.Pattern, target) {
			continue
		}
		decision := permissionDecision{Action: rule.Action, Source: fmt.Sprintf("rule[%d]", i+1)}
		if rule.Action == "deny" {
			return decision
		}
		if first == nil {
			first = &decision
		}
	}
	if first != nil {
		return *first
	}

	mutating := tool == "write" || tool == "edit" || tool == "shell" || tool == "rollback" || tool == "task_kill"
	if !mutating {
		return permissionDecision{Action: "allow", Source: "preset:" + cfg.Profile}
	}
	switch cfg.Profile {
	case "strict":
		return permissionDecision{Action: "ask", Source: "preset:strict"}
	case "standard":
		if tool == "write" || tool == "edit" {
			return permissionDecision{Action: "allow", Source: "preset:standard"}
		}
		return permissionDecision{Action: "ask", Source: "preset:standard"}
	case "open":
		return permissionDecision{Action: "allow", Source: "preset:open"}
	default:
		panic("permission config was not validated")
	}
}

// wildcardMatch uses a small glob language: '*' matches any sequence
// (including path separators) and '?' matches one rune. Matching is
// case-insensitive because pulse7 targets Windows 7.
func wildcardMatch(pattern, value string) bool {
	var b strings.Builder
	b.WriteString("(?i)^")
	for _, ch := range strings.ReplaceAll(pattern, `\`, "/") {
		switch ch {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteByte('.')
		default:
			b.WriteString(regexp.QuoteMeta(string(ch)))
		}
	}
	b.WriteByte('$')
	return regexp.MustCompile(b.String()).MatchString(strings.ReplaceAll(value, `\`, "/"))
}

func (r *Registry) permissionTarget(tool, argsJSON string) (string, error) {
	switch tool {
	case "write", "edit", "read":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", err
		}
		if !filepath.IsAbs(args.Path) {
			args.Path = filepath.Join(r.workspace, args.Path)
		}
		return filepath.Abs(args.Path)
	case "tree", "ls", "grep":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", err
		}
		if args.Path == "" && (tool == "tree" || tool == "ls" || tool == "grep") {
			args.Path = "."
		}
		if args.Path == "" {
			return "", nil
		}
		return r.absPath(args.Path)
	case "glob":
		var args struct {
			Pattern string `json:"pattern"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", err
		}
		if filepath.IsAbs(args.Pattern) {
			return filepath.Clean(args.Pattern), nil
		}
		return filepath.Join(r.workspace, args.Pattern), nil
	case "shell":
		var args struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", err
		}
		return args.Command, nil
	case "task_output", "task_kill":
		var args struct {
			TaskID  string               `json:"task_id"`
			Process *processIdentityArgs `json:"process"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", err
		}
		if args.Process != nil {
			return fmt.Sprintf("process:%d:%s:%s", args.Process.PID, args.Process.CreationTime, args.Process.ImagePath), nil
		}
		return args.TaskID, nil
	default:
		return "", nil
	}
}

func (r *Registry) authorize(tool, argsJSON string) error {
	target, err := r.permissionTarget(tool, argsJSON)
	if err != nil {
		return err
	}
	if tool == "write" || tool == "edit" {
		var hardBoundarySource, hardBoundaryMessage string
		for _, part := range strings.Split(filepath.Clean(target), string(filepath.Separator)) {
			if strings.EqualFold(part, ".git") {
				hardBoundarySource = "hard-boundary:git-write"
				hardBoundaryMessage = fmt.Sprintf("hard boundary denies direct write to protected .git path: %s", target)
				break
			}
		}
		if hardBoundarySource != "" {
			decision := permissionDecision{Action: "deny", Source: hardBoundarySource}
			if err := r.auditPermission(tool, target, decision); err != nil {
				return fmt.Errorf("permission audit failed: %w", err)
			}
			emitRuntimeEvent("permission_response", permissionResponseEvent{
				Tool: tool, Decision: decision.Action, Source: decision.Source, Target: target,
			})
			return errors.New(hardBoundaryMessage)
		}
	}
	if r.readOnly && (tool == "write" || tool == "edit" || tool == "shell" || tool == "rollback" || tool == "task_kill") {
		decision := permissionDecision{Action: "deny", Source: "read-only"}
		if err := r.auditPermission(tool, target, decision); err != nil {
			return fmt.Errorf("permission audit failed: %w", err)
		}
		emitRuntimeEvent("permission_response", permissionResponseEvent{
			Tool: tool, Decision: decision.Action, Source: decision.Source, Target: target,
		})
		return fmt.Errorf("read-only mode denies %s", tool)
	}
	if tool == "write" || tool == "edit" {
		target, err = r.absPath(target)
		if err != nil {
			return err
		}
	}
	r.permissionMu.RLock()
	decision := r.permissions.decide(tool, target)
	r.permissionMu.RUnlock()
	if decision.Source == "preset:standard" && (tool == "write" || tool == "edit") {
		root, _, _, err := resolveExistingPrefix(r.workspace)
		if err != nil {
			return err
		}
		if requirePathWithin(root, target) != nil {
			decision.Action = "ask"
		}
	}

	requestID := ""
	asked := decision.Action == "ask"
	if decision.Action == "ask" {
		if r.execMode {
			decision.Action = "deny"
			decision.Source += ":exec-no-prompt"
		} else {
			var line string
			var readErr error
			if r.confirmPermission != nil {
				line, requestID, readErr = r.confirmPermission(tool, argsJSON, target)
			} else {
				emitRuntimeEvent("permission_request", permissionRequestEvent{Tool: tool, Args: rawEventArgs(argsJSON), Target: target})
				line, readErr = r.input.read(interrupted)
			}
			if readErr != nil {
				return readErr
			}

			if strings.EqualFold(strings.TrimSpace(line), "y") {
				decision.Action = "allow"
				decision.Source += ":user"
			} else {
				decision.Action = "deny"
				decision.Source += ":user"
			}
		}
	}
	if err := r.auditPermission(tool, target, decision); err != nil {
		return fmt.Errorf("permission audit failed: %w", err)
	}
	emitRuntimeEvent("permission_response", permissionResponseEvent{
		Tool: tool, Decision: decision.Action, Source: decision.Source, Requested: asked && !r.execMode, Target: target, RequestID: requestID,
	})
	if decision.Action == "deny" {
		if strings.HasPrefix(decision.Source, "rule[") {
			return fmt.Errorf("denied by permission rule (%s)", decision.Source)
		}
		return errors.New("denied by user (confirmation)")
	}
	return nil
}

func (r *Registry) auditPermission(tool, target string, decision permissionDecision) error {
	f, err := os.OpenFile(r.auditPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	entry := map[string]interface{}{
		"ts": time.Now().Format(time.RFC3339), "task": r.taskID, "event": "permission",
		"tool": tool, "target": target, "decision": decision.Action, "source": decision.Source,
	}
	b, err := json.Marshal(entry)
	if err == nil {
		_, err = f.Write(append(b, '\n'))
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

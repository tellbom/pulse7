package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	openai "github.com/sashabaranov/go-openai"
)

type planModeState struct {
	Decision *planningDecision `json:"decision,omitempty"`
	Active   bool              `json:"active"`
	Path     string            `json:"path"`
}

// The session's manifest identity also scopes the mode journal across resume.
func (r *Registry) planJournal() string { return r.manPath + ".plan.jsonl" }

func (r *Registry) loadPlanMode() (planModeState, error) {
	var state planModeState
	if r.manPath == "" {
		return state, nil
	}
	f, err := os.Open(r.planJournal())
	if os.IsNotExist(err) {
		return state, nil
	}
	if err != nil {
		return state, fmt.Errorf("plan_mode_state: %w", err)
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 4096), maxSessionRecordBytes)
	for s.Scan() {
		// Each journal line is a full snapshot; omitted optional fields clear.
		state = planModeState{}
		if err := json.Unmarshal(s.Bytes(), &state); err != nil {
			return state, fmt.Errorf("plan_mode_state: %w", err)
		}
	}
	if err := s.Err(); err != nil {
		return state, fmt.Errorf("plan_mode_state: %w", err)
	}
	if state.Active && !filepath.IsAbs(state.Path) {
		return state, fmt.Errorf("plan_mode_state: invalid plan path")
	}
	return state, nil
}

func (r *Registry) registerPlanTools() {
	r.registerDecisionTool()
	for _, item := range []struct {
		name, description string
		fn                func(string) (string, error)
	}{
		{"enter_plan_mode", "Optionally enter planning. Only write/edit to plan_path (default PLAN.md) are allowed; shell/rollback are unavailable. Read tools remain available. Existing processes are not stopped. Notes are yours to organize and revise. Read-only mode still forbids notes writes.", r.toolEnterPlanMode},
		{"exit_plan_mode", "Leave planning after the plan file exists on disk. This checks file presence, not plan quality or task completion. No approval step; ordinary read-only and other checks remain active.", r.toolExitPlanMode},
	} {
		properties := map[string]interface{}{}
		if item.name == "enter_plan_mode" {
			properties["plan_path"] = map[string]interface{}{"type": "string", "description": "workspace-relative notes path, default PLAN.md; entering never overwrites an existing file"}
		}
		r.register(openai.Tool{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: item.name, Description: item.description, Parameters: map[string]interface{}{"type": "object", "properties": properties}}}, item.fn)
	}
}

func (r *Registry) toolEnterPlanMode(args string) (string, error) {
	state, err := r.loadPlanMode()
	if err != nil {
		return "", err
	}
	if state.Active {
		return fmt.Sprintf("plan_mode active: write/edit only %s. Save notes, then exit_plan_mode({}). Read tools remain available; shell/rollback unavailable.", state.Path), nil
	}
	var a struct {
		Path string `json:"plan_path"`
	}
	if err := json.Unmarshal([]byte(args), &a); err != nil {
		return "", err
	}
	if a.Path == "" {
		a.Path = "PLAN.md"
	}
	p, err := r.absPath(a.Path)
	if err != nil {
		return "", err
	}
	root, _, _, err := resolveExistingPrefix(r.workspace)
	if err != nil {
		return "", err
	}
	if err := requirePathWithin(root, p); err != nil {
		return "", err
	}
	info, statErr := os.Stat(p)
	if statErr != nil && !os.IsNotExist(statErr) {
		return "", statErr
	}
	if statErr == nil && !info.Mode().IsRegular() {
		return "", fmt.Errorf("plan_mode: plan must be a regular file")
	}
	if r.readOnly && os.IsNotExist(statErr) {
		return "", fmt.Errorf("plan_mode: read-only mode cannot create notes; choose an existing plan file or continue with read tools without entering planning")
	}
	if r.manPath == "" {
		return "", fmt.Errorf("plan_mode_state: session manifest is required")
	}
	if err := appendJSONLine(r.planJournal(), planModeState{Active: true, Path: p}); err != nil {
		return "", err
	}
	return fmt.Sprintf("plan_mode entered: write/edit only %s. Read tools remain available; shell/rollback unavailable. Do not bypass the stage via commands. Existing processes are unaffected. Save or revise notes, then exit_plan_mode({}). Read-only mode still forbids writing notes. Unknown facts may remain unknown.", p), nil
}

func (r *Registry) toolExitPlanMode(string) (string, error) {
	state, err := r.loadPlanMode()
	if err != nil {
		return "", err
	}
	if !state.Active {
		return "plan_mode inactive", nil
	}
	if err := r.revalidatePath(state.Path); err != nil {
		return "", err
	}
	f, err := os.Open(state.Path)
	if err != nil {
		return "", fmt.Errorf("plan_mode: save the plan file before exit: %w", err)
	}
	defer f.Close()
	if err := r.policy.ValidateOpenRead(f, state.Path); err != nil {
		return "", err
	}
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("plan_mode: plan must be a regular file")
	}
	state.Active = false
	if state.Decision != nil && state.Decision.AwaitingReply {
		state.Decision.AwaitingReply = false
		state.Decision.Cancelled = true
	}
	if err := r.savePlanState(state); err != nil {
		return "", err
	}
	return "plan_mode exited: plan file exists; contents and task completion have not been judged. Normal tool checks apply.", nil
}

func (r *Registry) checkPlanTool(tool, args string) error {
	if pending, err := r.pendingPlanningDecision(); err != nil {
		if tool == "write" || tool == "edit" || tool == "shell" || tool == "rollback" {
			return err
		}
	} else if pending != nil {
		return fmt.Errorf("plan_decision_pending: reply to decision %s before more tool execution", pending.ID)
	}
	if tool != "write" && tool != "edit" && tool != "shell" && tool != "rollback" {
		return nil
	}
	state, err := r.loadPlanMode()
	if err != nil {
		return err
	}
	if !state.Active {
		return nil
	}
	if tool == "write" || tool == "edit" {
		var a struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(args), &a); err != nil {
			return err
		}
		p, err := r.absPath(a.Path)
		if err != nil {
			return err
		}
		if sameResolvedPath(p, state.Path) {
			return r.revalidatePath(state.Path)
		}
	}
	return fmt.Errorf("plan_mode: %s blocked in planning; write/edit only %s. Use read/grep/ls/tree/glob to explore. Save the plan file, then exit_plan_mode({}) to restore normal tools", tool, state.Path)
}

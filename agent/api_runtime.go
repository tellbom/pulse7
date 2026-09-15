package main

import (
	"encoding/json"
	"errors"
	"fmt"
	openai "github.com/sashabaranov/go-openai"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (a *apiServer) hasRunningTasks() bool {
	if a.reg == nil {
		return false
	}
	a.reg.tasks.mu.Lock()
	defer a.reg.tasks.mu.Unlock()
	return a.reg.tasks.runningCountLocked() > 0
}
func (a *apiServer) closeSession() {
	if a.reg != nil {
		a.reg.tasks.exitSummary()
		sessionEndCleanup(a.reg.runner, a.cfg)
		a.reg.tasks.waitHarvested()
	}
	if sess != nil {
		sess.Close()
		sess = nil
	}
	a.reg = nil
	a.messages = nil
	curRegistry = nil
	curRunner = nil
}

// POST /api/sessions/new —— 真正关闭当前会话：下一条消息创建新 sessionId。
// 前端新建会话必须先调用本接口，成功后才清屏；失败保留页面（session-new-context 诊断）。
func (a *apiServer) newSession(w http.ResponseWriter, r *http.Request) {
	if !a.idle(w) {
		return
	}
	if a.hasRunningTasks() {
		apiError(w, 409, "background_running", "stop background tasks before changing session")
		return
	}
	a.closeSession()
	a.cfg.sessionPath = ""
	a.cfg.migrateResumeWorkspace = false
	a.selected = ""
	a.waitingAnswer = false
	apiJSON(w, 200, map[string]interface{}{"ok": true})
}

func (a *apiServer) resume(w http.ResponseWriter, r *http.Request) {
	if !a.idle(w) {
		return
	}
	if a.hasRunningTasks() {
		apiError(w, 409, "background_running", "stop background tasks before changing session")
		return
	}
	var input struct {
		SessionID string `json:"sessionId"`
	}
	if err := decodeAPIJSON(r.Body, &input); err != nil {
		apiBad(w, err)
		return
	}
	path, err := a.sessionFile(input.SessionID)
	if err != nil {
		apiBad(w, err)
		return
	}
	meta, err := loadSessionMetadata(path)
	if err != nil {
		apiReadError(w, err)
		return
	}
	ws, err := filepath.Abs(a.cfg.workspace)
	if err != nil {
		apiBad(w, err)
		return
	}
	if !sameResolvedPath(meta.Workspace, ws) {
		apiError(w, 409, "workspace_mismatch", "select the session workspace first")
		return
	}
	if _, err := readAPISessionMessages(path); err != nil {
		apiReadError(w, err)
		return
	}
	a.closeSession()
	a.cfg.sessionPath = ""
	a.cfg.migrateResumeWorkspace = false
	a.selected = input.SessionID
	a.waitingAnswer = false
	apiJSON(w, 200, map[string]string{"sessionId": input.SessionID, "cwd": meta.Workspace})
}
func (a *apiServer) workspace(w http.ResponseWriter, r *http.Request) {
	if !a.idle(w) {
		return
	}
	if a.hasRunningTasks() {
		apiError(w, 409, "background_running", "stop background tasks before changing workspace")
		return
	}
	var input struct {
		Path string `json:"path"`
	}
	if err := decodeAPIJSON(r.Body, &input); err != nil {
		apiBad(w, err)
		return
	}
	path, err := filepath.Abs(input.Path)
	if input.Path == "" {
		err = errors.New("workspace is required")
	}
	if err == nil {
		var info os.FileInfo
		info, err = os.Stat(path)
		if err == nil && !info.IsDir() {
			err = errors.New("workspace is not a directory")
		}
	}
	if err != nil {
		apiBad(w, err)
		return
	}
	// Respect project settings on selection; HTTP connection fields explicitly
	// saved by the user remain the active connection for this process.
	ac, _, err := loadLayeredAgentConfig(path, true)
	if err != nil {
		apiError(w, 500, "config_error", "could not load workspace configuration")
		return
	}
	a.closeSession()
	a.cfg.workspace = path
	a.cfg.readOnly = ac.ReadOnly
	a.cfg.maxCtx = ac.MaxCtx
	a.cfg.maxRounds = ac.MaxRounds
	a.selected = ""
	a.waitingAnswer = false
	apiJSON(w, 200, map[string]string{"workspace": path})
}
func (a *apiServer) ensureSession() error {
	if a.reg != nil {
		return nil
	}
	id := newTaskID()
	cfg := a.cfg
	cfg.execMode = false
	if a.selected != "" {
		path, err := a.sessionFile(a.selected)
		if err != nil {
			return err
		}
		cfg.resumePath = path
	} else {
		cfg.resumePath = ""
		cfg.sessionPath = ""
	}
	plan, err := prepareResume(cfg, id)
	if err != nil {
		return err
	}
	applyResumePreparation(cfg, plan)
	reg, err := setupEnv(cfg, plan.TaskID)
	if err != nil {
		return err
	}
	reg.confirmPermission = a.requestPermission
	a.reg = reg
	// Do not let a failed initialization make the next request reuse a
	// half-open session (including after a workspace change).
	initialized := false
	defer func() {
		if !initialized {
			a.closeSession()
		}
	}()
	sess = openSessionFor(cfg, plan.TaskID)
	a.selected = sess.id()
	a.messages, err = loadPreparedMessages(plan, sess)
	if err != nil {
		return err
	}
	if len(a.messages) == 0 {
		if err = pushMsg(&a.messages, systemMessage(cfg)); err != nil {
			return err
		}
	}
	emitSessionInit(cfg, reg, a.selected)
	initialized = true
	return nil
}
func apiNeedsAnswer(content string) bool {
	if !strings.ContainsAny(content, "?？") {
		return false
	}
	for _, word := range []string{"具体", "指什么", "哪些", "希望", "还是", "请告诉我"} {
		if strings.Contains(content, word) {
			return true
		}
	}
	return false
}
func (a *apiServer) startTurn(w http.ResponseWriter, r *http.Request) {
	if !a.idle(w) {
		return
	}
	var input struct {
		Prompt     string `json:"prompt"`
		Answer     string `json:"answer"`
		DecisionID string `json:"decisionId"`
		SessionID  string `json:"sessionId"`
	}
	if err := decodeAPIJSON(r.Body, &input); err != nil {
		apiBad(w, err)
		return
	}
	prompt := input.Prompt
	if r.URL.Path == "/api/answer" {
		if input.SessionID != a.selected || input.Answer == "" {
			apiError(w, 409, "session_mismatch", "answer must identify the current session")
			return
		}
		prompt = input.Answer
	}
	if strings.TrimSpace(prompt) == "" {
		apiBad(w, errors.New("prompt required"))
		return
	}
	if err := a.ensureSession(); err != nil {
		apiError(w, 500, "session_error", a.safeError(err))
		return
	}
	pending, err := a.reg.pendingPlanningDecision()
	if err != nil {
		apiError(w, 500, "plan_mode_state", a.safeError(err))
		return
	}
	if pending != nil || input.DecisionID != "" {
		if r.URL.Path != "/api/answer" {
			apiError(w, 409, "plan_decision_pending", "reply using /api/answer with sessionId and decisionId")
			return
		}
		if err := a.reg.validateDecisionAnswer(input.DecisionID); err != nil {
			apiError(w, 409, "decision_mismatch", err.Error())
			return
		}
	}
	resetInterrupt()
	if err := pushMsg(&a.messages, openai.ChatCompletionMessage{Role: "user", Content: prompt}); err != nil {
		apiError(w, 500, "storage_error", a.safeError(err))
		return
	}
	if input.DecisionID != "" {
		if err := a.reg.recordDecisionAnswer(input.DecisionID, sess.lastUUID, prompt); err != nil {
			apiError(w, 500, "storage_error", a.safeError(err))
			return
		}
	}
	a.busy = true
	a.waitingAnswer = false
	apiJSON(w, 202, map[string]string{"sessionId": a.selected})
	go a.runTurn()
}
func (a *apiServer) runTurn() {
	started := time.Now()
	var stats turnStats
	status := "success"
	var turnErr error
	defer func() {
		if failure := recover(); failure != nil {
			status = "error"
			turnErr = fmt.Errorf("execution panic: %v", failure)
		}
		if turnErr != nil {
			turnErr = errors.New(a.safeError(turnErr))
		}
		printTaskEnd(a.reg, false, taskEndState{status: status, rounds: stats.rounds, elapsed: time.Since(started)})
		a.mu.Lock()
		a.busy = false
		a.waitingAnswer = status == "need_answer"
		resetInterrupt()
		emitTurnResult(status, turnErr)
		a.mu.Unlock()
	}()
	content, err := streamTurn(newClient(a.cfg), a.reg, a.cfg, &a.messages, &stats)
	turnErr = err
	if errors.Is(err, errInterrupted) {
		status = "interrupted"
		if e := finalizeInterrupted(&a.messages, a.reg, taskEndState{}); e != nil {
			turnErr = e
			status = "error"
		}
	} else if errors.Is(err, errPlanningDecision) {
		status = "need_answer"
		turnErr = nil
	} else if errors.Is(err, errMaxRounds) {
		status = "max_rounds"
	} else if err != nil {
		status = "error"
	} else if apiNeedsAnswer(content) {
		status = "need_answer"
	}
}

func (a *apiServer) requestPermission(tool, args, target string) (string, string, error) {
	a.confirmMu.Lock()
	a.nextConfirm++
	id := fmt.Sprint(a.nextConfirm)
	ch := make(chan string, 1)
	a.confirmID = id
	a.confirmAnswer = ch
	a.confirmMu.Unlock()
	defer func() { a.confirmMu.Lock(); a.confirmID = ""; a.confirmAnswer = nil; a.confirmMu.Unlock() }()
	emitRuntimeEvent("permission_request", permissionRequestEvent{Tool: tool, Args: rawEventArgs(args), Target: target, RequestID: id})
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case line := <-ch:
			return line, id, nil
		case <-ticker.C:
			if interrupted() {
				return "", id, errInterrupted
			}
		}
	}
}
func (a *apiServer) answerPermission(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RequestID string `json:"requestId"`
		Decision  string `json:"decision"`
	}
	if err := decodeAPIJSON(r.Body, &input); err != nil {
		apiBad(w, err)
		return
	}
	if input.Decision != "allow" && input.Decision != "deny" {
		apiBad(w, errors.New("decision must be allow or deny"))
		return
	}
	a.confirmMu.Lock()
	defer a.confirmMu.Unlock()
	if input.RequestID == "" || input.RequestID != a.confirmID || a.confirmAnswer == nil || interrupted() {
		apiError(w, 409, "no_confirmation", "confirmation expired")
		return
	}
	line := "n"
	if input.Decision == "allow" {
		line = "y"
	}
	a.confirmAnswer <- line
	a.confirmID = ""
	apiJSON(w, 200, map[string]bool{"accepted": true})
}
func (a *apiServer) permissions(w http.ResponseWriter, r *http.Request) {
	var p permissionConfig
	if a.reg != nil {
		a.reg.permissionMu.RLock()
		p = a.reg.permissions
		a.reg.permissionMu.RUnlock()
	} else {
		var err error
		p, err = loadPermissionConfig(permissionsPath(a.cfg.exeDirStore()))
		if err != nil {
			apiError(w, 500, "permission_config_error", "could not load permissions")
			return
		}
	}
	if r.Method == "PUT" {
		var input struct {
			Profile string `json:"profile"`
		}
		if err := decodeAPIJSON(r.Body, &input); err != nil {
			apiBad(w, err)
			return
		}
		p.Profile = input.Profile
		if err := p.validate(); err != nil {
			apiBad(w, err)
			return
		}
		if a.reg == nil {
			if err := a.ensureSession(); err != nil {
				apiError(w, 500, "session_error", a.safeError(err))
				return
			}
		}
		a.reg.permissionMu.Lock()
		a.reg.permissions.Profile = p.Profile
		a.reg.permissionMu.Unlock()
	}
	apiJSON(w, 200, map[string]interface{}{"profile": p.Profile, "rules": p.Rules, "readOnly": a.cfg.readOnly})
}
func (a *apiServer) executeTool(w http.ResponseWriter, r *http.Request, name string) {
	if !a.idle(w) {
		return
	}
	if err := a.ensureSession(); err != nil {
		apiError(w, 500, "session_error", a.safeError(err))
		return
	}
	var input json.RawMessage
	if err := decodeAPIJSON(r.Body, &input); err != nil {
		apiBad(w, err)
		return
	}
	if name == "rollback" {
		var v struct {
			To int `json:"to"`
		}
		if err := decodeAPIJSON(strings.NewReader(string(input)), &v); err != nil || v.To <= 0 {
			apiBad(w, errors.New("positive checkpoint sequence required"))
			return
		}
	}
	a.busy = true
	resetInterrupt()
	a.mu.Unlock()
	result := a.reg.Execute(name, string(input))
	a.mu.Lock()
	a.busy = false
	if strings.HasPrefix(result, "error:") {
		code, status := "tool_error", 500
		if strings.Contains(result, "hard_link_impact_unknown") {
			code, status = "hard_link_impact_unknown", 422
		} else if strings.Contains(result, "denied") || strings.Contains(result, "read-only") {
			code, status = "permission_denied", 403
		}
		apiError(w, status, code, result)
		return
	}
	apiJSON(w, 200, map[string]string{"result": result})
}

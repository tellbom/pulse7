package main

import "net/http"

func (a *apiServer) planView(w http.ResponseWriter, r *http.Request) {
	if !a.idle(w) {
		return
	}
	reg := a.reg
	if reg == nil && a.selected != "" {
		path, err := a.sessionFile(a.selected)
		if err != nil {
			apiBad(w, err)
			return
		}
		cfg := *a.cfg
		cfg.resumePath = path
		cfg.migrateResumeWorkspace = false
		prepared, err := prepareResume(&cfg, "")
		if err != nil {
			apiError(w, 409, "session_error", a.safeError(err))
			return
		}
		reg = &Registry{manPath: prepared.ManifestPath}
	}
	if reg == nil {
		apiJSON(w, 200, map[string]interface{}{"sessionId": a.selected, "initialized": false, "state": nil})
		return
	}
	state, err := reg.loadPlanMode()
	if err != nil {
		apiError(w, 500, "plan_mode_state", a.safeError(err))
		return
	}
	apiJSON(w, 200, map[string]interface{}{"sessionId": a.selected, "initialized": true, "state": state})
}

func (a *apiServer) exitPlan(w http.ResponseWriter, r *http.Request) {
	if !a.idle(w) {
		return
	}
	var input struct {
		SessionID string `json:"sessionId"`
	}
	if err := decodeAPIJSON(r.Body, &input); err != nil {
		apiBad(w, err)
		return
	}
	if input.SessionID == "" || input.SessionID != a.selected {
		apiError(w, 409, "session_mismatch", "identify the current session")
		return
	}
	if err := a.ensureSession(); err != nil {
		apiError(w, 500, "session_error", a.safeError(err))
		return
	}
	// An explicit user exit can cancel a pending question. It does not answer it,
	// approve a plan's contents, disable read-only, or start an LLM request.
	result, err := a.reg.toolExitPlanMode("{}")
	if err != nil {
		apiError(w, 409, "plan_mode", a.safeError(err))
		return
	}
	if err := a.reg.audit("exit_plan_mode", "{}", result); err != nil {
		apiError(w, 500, "storage_error", a.safeError(err))
		return
	}
	a.waitingAnswer = false
	state, err := a.reg.loadPlanMode()
	if err != nil {
		apiError(w, 500, "plan_mode_state", a.safeError(err))
		return
	}
	emitRuntimeEvent("plan_state", map[string]interface{}{"sessionId": a.selected, "state": state, "source": "user"})
	apiJSON(w, 200, map[string]interface{}{"sessionId": a.selected, "state": state})
}

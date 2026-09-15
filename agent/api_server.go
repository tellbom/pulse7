package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

type apiServer struct {
	mu               sync.Mutex
	cfg              *config
	reg              *Registry
	messages         []openai.ChatCompletionMessage
	busy             bool
	waitingAnswer    bool
	selected         string
	token            string
	listener         net.Listener
	server           *http.Server
	streamMu         sync.Mutex
	subscribers      map[chan []byte]struct{}
	confirmMu        sync.Mutex
	confirmID        string
	confirmAnswer    chan string
	nextConfirm      uint64
	streamID         string
	streamSession    string
	eventSeq         uint64
	replay           [][]byte
	replayBytes      int
	turnActive       bool
	turnHistoryCount int
	turnStartCursor  uint64
}

func newAPIServer(cfg *config) (*apiServer, error) {
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return nil, err
	}
	streamID, err := newMessageUUID()
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp4", "0.0.0.0:0")
	if err != nil {
		return nil, err
	}
	a := &apiServer{cfg: cfg, token: hex.EncodeToString(secret[:]), streamID: streamID, listener: listener, subscribers: map[chan []byte]struct{}{}}
	a.server = &http.Server{Handler: a}
	return a, nil
}

func runServe(cfg *config) error {
	a, err := newAPIServer(cfg)
	if err != nil {
		return err
	}
	defer a.Close()
	// Only this mode replaces the runtime composition; CLI setup is untouched.
	runtimeEvents = &eventBus{consumers: []eventConsumer{{emit: a.publish}}}
	out("HTTP listening: http://%s (all IPv4 interfaces; token enabled)\n", a.listener.Addr())
	if cfg.openWeb {
		url := fmt.Sprintf("http://127.0.0.1:%d/", a.listener.Addr().(*net.TCPAddr).Port)
		if err := openWebBrowser(url); err != nil {
			out("Could not open browser: %v; open %s manually\n", err, url)
			showStartupError("浏览器未能打开，请手工访问：" + url)
		}
	}
	return a.server.Serve(a.listener)
}

func (a *apiServer) Close() {
	a.server.Close()
	a.streamMu.Lock()
	for ch := range a.subscribers {
		close(ch)
		delete(a.subscribers, ch)
	}
	a.streamMu.Unlock()

}

func (a *apiServer) publish(event runtimeEvent) {
	a.streamMu.Lock()
	defer a.streamMu.Unlock()
	if event.Type == "session_init" {
		raw, _ := json.Marshal(event.Data)
		var init struct {
			SessionID string `json:"sessionId"`
		}
		if json.Unmarshal(raw, &init) == nil {
			a.streamSession = init.SessionID
		}
	}
	a.eventSeq++
	b, err := json.Marshal(struct {
		Type      string      `json:"type"`
		Data      interface{} `json:"data"`
		SessionID string      `json:"sessionId"`
		StreamID  string      `json:"streamId"`
		Seq       uint64      `json:"seq"`
	}{event.Type, event.Data, a.streamSession, a.streamID, a.eventSeq})
	if err != nil {
		panic(err)
	}
	a.replay = append(a.replay, b)
	a.replayBytes += len(b)
	for len(a.replay) > 2048 || a.replayBytes > 4*1024*1024 {
		a.replayBytes -= len(a.replay[0])
		a.replay[0] = nil
		a.replay = a.replay[1:]
	}
	for ch := range a.subscribers {
		select {
		case ch <- b:
		default:
			close(ch)
			delete(a.subscribers, ch)
		}
	}
}

func apiJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
func apiError(w http.ResponseWriter, status int, code, message string) {
	apiJSON(w, status, map[string]interface{}{"error": map[string]string{"code": code, "message": message}})
}
func apiBad(w http.ResponseWriter, err error) { apiError(w, 400, "invalid_request", err.Error()) }
func (a *apiServer) safeError(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if a.cfg.apiKey != "" {
		s = strings.ReplaceAll(s, a.cfg.apiKey, "[redacted]")
	}
	if a.token != "" {
		s = strings.ReplaceAll(s, a.token, "[redacted]")
	}
	return s
}
func (a *apiServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
	// The document is the same-origin bootstrap; no token in URLs or console.
	if r.URL.Path == "/" && r.Method == http.MethodGet {
		data, err := fs.ReadFile(webFS(), "web/index.html")
		if err != nil {
			apiError(w, 500, "static_error", "embedded page unavailable")
			return
		}
		page := strings.Replace(string(data), "__PULSE7_TOKEN__", a.token, 1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprint(w, page)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/assets/") && r.Method == http.MethodGet {
		data, err := fs.ReadFile(webFS(), "web"+r.URL.Path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(r.URL.Path)))
		w.Header().Set("Cache-Control", "no-store")
		w.Write(data)
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	expected := "Bearer " + a.token
	if a.token == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte(expected)) != 1 {
		apiError(w, 401, "unauthorized", "invalid token")
		return
	}
	if r.Method == "GET" && r.URL.Path == "/api/events" {
		a.events(w, r)
		return
	}
	if r.Method == "POST" && r.URL.Path == "/api/permission" {
		a.answerPermission(w, r)
		return
	}
	if r.Method == "POST" && r.URL.Path == "/api/interrupt" {
		a.mu.Lock()
		busy := a.busy
		a.mu.Unlock()
		if !busy {
			apiError(w, 409, "not_running", "no active operation")
			return
		}
		atomic.StoreInt32(&interruptFlag, 1)
		turnCancelMu.Lock()
		cancel := turnCancel
		turnCancelMu.Unlock()
		if cancel != nil {
			cancel()
		}
		interruptActiveManagedProcess()
		if curRunner != nil {
			curRunner.Interrupt()
		}
		apiJSON(w, 200, map[string]bool{"requested": true})
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	switch r.Method + " " + r.URL.Path {
	case "GET /api/runtime":
		a.runtimeView(w, r)
	case "GET /api/plan":
		a.planView(w, r)
	case "POST /api/plan/exit":
		a.exitPlan(w, r)
	case "GET /api/listener":
		apiJSON(w, 200, map[string]interface{}{"address": a.listener.Addr().(*net.TCPAddr).IP.String(), "port": a.listener.Addr().(*net.TCPAddr).Port, "listening": true, "tokenValid": a.token != ""})
	case "GET /api/config":
		apiJSON(w, 200, a.configView())
	case "PUT /api/config":
		a.putConfig(w, r)
	case "POST /api/connection-test":
		a.connectionTest(w, r)
	case "GET /api/sessions":
		a.sessions(w, r)
	case "POST /api/sessions/resume":
		a.resume(w, r)
	case "POST /api/sessions/new":
		a.newSession(w, r)
	case "PUT /api/workspace":
		a.workspace(w, r)
	case "POST /api/turns", "POST /api/answer":
		a.startTurn(w, r)
	case "GET /api/permissions", "PUT /api/permissions":
		a.permissions(w, r)
	case "GET /api/tasks":
		a.tasks(w, r)
	case "POST /api/tasks/kill":
		a.executeTool(w, r, "task_kill")
	case "GET /api/checkpoints":
		a.checkpoints(w, r)
	case "POST /api/rollback":
		a.executeTool(w, r, "rollback")
	case "GET /api/tool-result":
		a.toolResult(w, r)
	default:
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if r.Method == "GET" && len(parts) == 4 && parts[1] == "sessions" && parts[3] == "messages" {
			a.sessionMessages(w, r, parts[2])
			return
		}
		if r.Method == "GET" && len(parts) == 4 && parts[1] == "tasks" && parts[3] == "output" {
			a.taskOutput(w, r, parts[2])
			return
		}
		apiError(w, 404, "not_found", "unknown operation")
	}
}

func (a *apiServer) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		apiError(w, 500, "stream_unavailable", "SSE unavailable")
		return
	}
	ch := make(chan []byte, 256)
	a.streamMu.Lock()
	replay, err := a.replayAfter(r)
	if err != nil {
		a.streamMu.Unlock()
		apiError(w, 409, "event_cursor_expired", err.Error())
		return
	}
	a.subscribers[ch] = struct{}{}
	a.streamMu.Unlock()
	defer func() {
		a.streamMu.Lock()
		if _, ok := a.subscribers[ch]; ok {
			delete(a.subscribers, ch)
			close(ch)
		}
		a.streamMu.Unlock()
	}()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(200)
	flusher.Flush()
	for _, b := range replay {
		if !writeAPIEvent(w, flusher, b) {
			return
		}
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case b, ok := <-ch:
			if !ok {
				return
			}
			if !writeAPIEvent(w, flusher, b) {
				return
			}
		}
	}
}

func (a *apiServer) configView() map[string]interface{} {
	// Build from the effective runtime values, never from a key-bearing map.
	c := a.cfg
	return map[string]interface{}{"base_url": c.baseURL, "model": c.model, "workspace": c.workspace, "apiKeyConfigured": c.apiKey != "", "read_only": c.readOnly, "max_ctx": c.maxCtx, "max_rounds": c.maxRounds, "micro_keep_recent": c.microKeepRecent, "sandbox_preference": c.sandboxPreference, "shell_timeout_sec": int(c.shellTimeout / time.Second), "memory_limit_mb": c.memLimitMB, "process_warn_threshold": c.processWarnThreshold, "background_task_max_output_mb": c.backgroundTaskMaxOutputMB, "background_task_warn_count": c.backgroundTaskWarnCount, "background_task_warn_sec": c.backgroundTaskWarnSec, "cleanup_on_exit": c.cleanupOnExit, "box": c.box, "start_exe": c.startExe, "sandbox_root": c.sandboxRoot, "yolo": c.yolo, "llm_first_chunk_timeout_sec": int(c.llmFirstChunkTimeout / time.Second), "llm_idle_timeout_sec": int(c.llmIdleTimeout / time.Second), "llm_max_retries": c.llmMaxRetries, "llm_compress_timeout_sec": int(c.llmCompressTimeout / time.Second)}
}
func (a *apiServer) idle(w http.ResponseWriter) bool {
	if a.busy {
		apiError(w, 409, "busy", "operation in progress")
		return false
	}
	return true
}
func (a *apiServer) putConfig(w http.ResponseWriter, r *http.Request) {
	if !a.idle(w) {
		return
	}
	var update apiConfigUpdate
	if err := decodeAPIJSON(r.Body, &update); err != nil {
		apiBad(w, err)
		return
	}
	base, model := a.cfg.baseURL, a.cfg.model
	if update.BaseURL != nil {
		base = *update.BaseURL
	}
	if update.Model != nil {
		model = *update.Model
	}
	if err := validateAPIConnection(base, model); err != nil {
		apiBad(w, err)
		return
	}
	if err := validateRuntimeParams(&update); err != nil {
		apiBad(w, err)
		return
	}
	home, err := os.UserHomeDir()
	if err == nil {
		err = saveAPIConfig(globalConfigPath(home), update)
	}
	if err != nil {
		apiError(w, 500, "config_write_failed", "could not persist global configuration")
		return
	}
	a.cfg.baseURL, a.cfg.model = base, model
	if update.APIKey != nil {
		a.cfg.apiKey = *update.APIKey
	}
	applyRuntimeParams(a.cfg, &update)
	view := a.configView()
	saved := update.provided()
	delete(saved, "api_key")
	restart := []string{}
	for _, key := range []string{"shell_timeout_sec", "memory_limit_mb", "process_warn_threshold", "background_task_max_output_mb", "background_task_warn_count", "background_task_warn_sec", "cleanup_on_exit", "read_only", "sandbox_preference"} {
		if _, ok := saved[key]; ok {
			restart = append(restart, key)
		}
	}
	view["saved"] = saved
	view["restartRequired"] = len(restart) > 0
	view["restartRequiredFields"] = restart
	apiJSON(w, 200, view)
}

// Apply only turn-local settings; runner/registry settings require restart.
func applyRuntimeParams(cfg *config, u *apiConfigUpdate) {
	if u.MaxCtx != nil {
		cfg.maxCtx = *u.MaxCtx
	}
	if u.MaxRounds != nil {
		cfg.maxRounds = *u.MaxRounds
	}
	if u.MicroKeepRecent != nil {
		cfg.microKeepRecent = *u.MicroKeepRecent
	}
	if u.LLMFirstChunkTimeoutSec != nil {
		cfg.llmFirstChunkTimeout = time.Duration(*u.LLMFirstChunkTimeoutSec) * time.Second
	}
	if u.LLMIdleTimeoutSec != nil {
		cfg.llmIdleTimeout = time.Duration(*u.LLMIdleTimeoutSec) * time.Second
	}
	if u.LLMMaxRetries != nil {
		cfg.llmMaxRetries = *u.LLMMaxRetries
	}
	if u.LLMCompressTimeoutSec != nil {
		cfg.llmCompressTimeout = time.Duration(*u.LLMCompressTimeoutSec) * time.Second
	}
}

// validateRuntimeParams 只校验本次提交的数值字段，范围宽松但拒绝零/负值与未知枚举。
func validateRuntimeParams(u *apiConfigUpdate) error {
	for _, c := range []struct {
		v  *int
		n  string
		lo int
		hi int
	}{
		{u.MaxCtx, "max_ctx", 16000, 32000000},
		{u.MaxRounds, "max_rounds", 1, 10000},
		{u.MicroKeepRecent, "micro_keep_recent", 1, 1000},
		{u.ShellTimeoutSec, "shell_timeout_sec", 5, 86400},
		{u.MemoryLimitMB, "memory_limit_mb", 64, 4095},
		{u.ProcessWarnThreshold, "process_warn_threshold", 0, 100000},
		{u.BgTaskMaxOutputMB, "background_task_max_output_mb", 1, 4096},
		{u.BgTaskWarnCount, "background_task_warn_count", 0, 1000},
		{u.BgTaskWarnSec, "background_task_warn_sec", 0, 864000},
		{u.LLMFirstChunkTimeoutSec, "llm_first_chunk_timeout_sec", 5, 3600},
		{u.LLMIdleTimeoutSec, "llm_idle_timeout_sec", 5, 3600},
		{u.LLMMaxRetries, "llm_max_retries", 0, 10},
		{u.LLMCompressTimeoutSec, "llm_compress_timeout_sec", 5, 3600},
	} {
		if c.v != nil && (*c.v < c.lo || *c.v > c.hi) {
			return fmt.Errorf("%s must be between %d and %d", c.n, c.lo, c.hi)
		}
	}
	if u.SandboxPreference != nil && *u.SandboxPreference != "auto" && *u.SandboxPreference != "sandboxie" && *u.SandboxPreference != "jobobject" {
		return errors.New("sandbox_preference must be auto, sandboxie or jobobject")
	}
	return nil
}

// GET /api/fs/dirs?path=... — 目录浏览（一层），供前端工作区选择器；仅返回目录名，不含文件内容。
func (a *apiServer) fsDirs(w http.ResponseWriter, r *http.Request) {
	type ent struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	p := r.URL.Query().Get("path")
	if p == "" {
		drives := []ent{}
		for c := 'A'; c <= 'Z'; c++ {
			d := string(c) + `:\`
			if fi, err := os.Stat(d); err == nil && fi.IsDir() {
				drives = append(drives, ent{d, d})
			}
		}
		apiJSON(w, 200, map[string]interface{}{"path": "", "parent": "", "dirs": drives})
		return
	}
	ap, err := filepath.Abs(p)
	if err != nil {
		apiBad(w, err)
		return
	}
	es, err := os.ReadDir(ap)
	if err != nil {
		apiError(w, 400, "unreadable_dir", err.Error())
		return
	}
	dirs := []ent{}
	for _, e := range es {
		if e.IsDir() && !strings.HasPrefix(e.Name(), "$") {
			dirs = append(dirs, ent{e.Name(), filepath.Join(ap, e.Name())})
		}
	}
	parent := filepath.Dir(ap)
	if parent == ap {
		parent = ""
	}
	apiJSON(w, 200, map[string]interface{}{"path": ap, "parent": parent, "dirs": dirs})
}
func (a *apiServer) connectionTest(w http.ResponseWriter, r *http.Request) {
	var input struct {
		BaseURL string  `json:"base_url"`
		Model   string  `json:"model"`
		APIKey  *string `json:"api_key"`
	}
	if err := decodeAPIJSON(r.Body, &input); err != nil {
		apiBad(w, err)
		return
	}
	if err := validateAPIConnection(input.BaseURL, input.Model); err != nil {
		apiBad(w, err)
		return
	}
	cfg := *a.cfg
	cfg.baseURL, cfg.model = input.BaseURL, input.Model
	if input.APIKey != nil {
		cfg.apiKey = *input.APIKey
	}
	started := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), cfg.llmFirstChunkTimeout)
	defer cancel()
	a.mu.Unlock()
	_, err := newClient(&cfg).CreateChatCompletion(ctx, openai.ChatCompletionRequest{Model: cfg.model, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "Reply OK."}}, MaxTokens: 8})
	a.mu.Lock()
	if err != nil {
		message := a.safeError(err)
		if cfg.apiKey != "" {
			message = strings.ReplaceAll(message, cfg.apiKey, "[redacted]")
		}
		apiError(w, 502, "connection_failed", message)
		return
	}
	apiJSON(w, 200, map[string]interface{}{"ok": true, "model": cfg.model, "elapsedMs": time.Since(started).Milliseconds()})
}

func apiPagination(r *http.Request, defaultLimit, maxLimit int) (int, int, error) {
	offset, limit := 0, defaultLimit
	var err error
	if s := r.URL.Query().Get("offset"); s != "" {
		offset, err = strconv.Atoi(s)
		if err != nil {
			return 0, 0, errors.New("invalid offset")
		}
	}
	if s := r.URL.Query().Get("limit"); s != "" {
		limit, err = strconv.Atoi(s)
		if err != nil {
			return 0, 0, errors.New("invalid limit")
		}
	}
	if offset < 0 || limit < 1 || limit > maxLimit {
		return 0, 0, errors.New("offset or limit out of range")
	}
	return offset, limit, nil
}
func (a *apiServer) sessionFile(id string) (string, error) {
	return apiSessionPath(filepath.Join(a.cfg.exeDirStore(), "data", "sessions"), id)
}
func (a *apiServer) readHistory(id string) ([]json.RawMessage, error) {
	path, err := a.sessionFile(id)
	if err != nil {
		return nil, err
	}
	return readAPISessionMessages(path)
}
func apiReadError(w http.ResponseWriter, err error) {
	if os.IsNotExist(err) {
		apiError(w, 404, "not_found", "session not found")
	} else {
		apiError(w, 500, "storage_error", "could not read session")
	}
}
func (a *apiServer) sessionMessages(w http.ResponseWriter, r *http.Request, id string) {
	offset, limit, err := apiPagination(r, 100, 1000)
	if err != nil {
		apiBad(w, err)
		return
	}
	path, err := a.sessionFile(id)
	if err != nil {
		apiReadError(w, err)
		return
	}
	index, err := indexAPIHistory(path)
	if err != nil {
		apiReadError(w, err)
		return
	}
	if offset > len(index.spans) {
		apiBad(w, errors.New("offset outside history"))
		return
	}
	rows, count, err := apiPagedMessages(path, offset, limit)
	if err != nil {
		apiReadError(w, err)
		return
	}
	if offset > count {
		apiBad(w, errors.New("offset outside history"))
		return
	}
	end := offset + len(rows)
	apiJSON(w, 200, map[string]interface{}{"sessionId": id, "messages": rows, "offset": offset, "nextOffset": end, "hasMore": end < count})
}
func (a *apiServer) sessions(w http.ResponseWriter, r *http.Request) {
	dir := filepath.Join(a.cfg.exeDirStore(), "data", "sessions")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		apiJSON(w, 200, map[string]interface{}{"sessions": []interface{}{}, "errors": []interface{}{}})
		return
	}
	if err != nil {
		apiReadError(w, err)
		return
	}
	list := []interface{}{}
	problems := []interface{}{}
	addProblem := func(id, filename, stage string) {
		// Do not expose raw parse errors or record contents (which can contain
		// user data). Leave the source untouched and make the omission visible.
		problems = append(problems, map[string]string{"sessionId": id, "fileName": filename,
			"code": "session_unreadable", "stage": stage, "message": "Session could not be listed; source file was not modified."})
	}
	for _, e := range entries {
		if e.IsDir() || (e.Name() == "audit.jsonl" || e.Name() == "checkpoint-metadata.jsonl" || strings.HasPrefix(e.Name(), "manifest-")) || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		id := sessionTaskID(e.Name())
		path, err := a.sessionFile(id)
		if err != nil {
			addProblem(id, e.Name(), "messages")
			continue
		}
		count, first, err := apiSessionSummary(path)
		if err != nil {
			addProblem(id, e.Name(), "messages")
			continue
		}
		meta, err := loadSessionMetadata(path)
		if err != nil {
			addProblem(id, e.Name(), "metadata")
			continue
		}
		info, err := e.Info()
		if err != nil {
			addProblem(id, e.Name(), "stat")
			continue
		}
		list = append(list, map[string]interface{}{"sessionId": id, "cwd": meta.Workspace, "updatedAt": info.ModTime(), "messageCount": count, "firstUser": first})
	}
	apiJSON(w, 200, map[string]interface{}{"sessions": list, "errors": problems})
}
func (a *apiServer) toolResult(w http.ResponseWriter, r *http.Request) {
	ref := r.URL.Query().Get("ref")
	id, call := a.selected, ""
	if value := r.URL.Query().Get("sessionId"); value != "" {
		id = value
	}
	if strings.HasPrefix(ref, "session:") {
		parts := strings.SplitN(strings.TrimPrefix(ref, "session:"), "#tool:", 2)
		if len(parts) == 2 {
			id, call = parts[0], parts[1]
		}
	} else if strings.HasPrefix(ref, "tool:") {
		call = strings.TrimPrefix(ref, "tool:")
	}
	if id == "" || (call == "" && !strings.HasPrefix(ref, "lc1.")) {
		apiBad(w, errors.New("invalid result reference"))
		return
	}
	offset, limit, err := apiResultPagination(r.URL.Query().Get("offset"), r.URL.Query().Get("limit"))
	if err != nil {
		apiBad(w, err)
		return
	}
	path, err := a.sessionFile(id)
	if err != nil {
		apiReadError(w, err)
		return
	}
	if _, err := loadSessionMetadata(path); err != nil {
		apiReadError(w, err)
		return
	}
	contentRef := ref
	var message sessionMessageRecord
	if call != "" {
		message, err = apiFindToolResult(path, call)
		if err != nil {
			apiReadError(w, err)
			return
		}
		contentRef = message.LargeContent["content"]
	}
	var result string
	var total, next int64
	if contentRef != "" {
		result, total, next, err = readLargeContent(path, contentRef, int64(offset), int64(limit))
	} else {
		var n int
		result, n, _, err = apiResultRange(message.Content, offset, limit)
		next = int64(n)
		total = int64(len(message.Content))
	}
	if err != nil {
		apiError(w, 400, "content_unavailable", a.safeError(err))
		return
	}
	apiJSON(w, 200, map[string]interface{}{"id": call, "result": result, "offset": offset, "nextOffset": next, "totalBytes": total, "hasMore": next < total, "contentRef": contentRef})
}

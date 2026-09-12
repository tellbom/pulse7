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
	mu            sync.Mutex
	cfg           *config
	reg           *Registry
	messages      []openai.ChatCompletionMessage
	busy          bool
	waitingAnswer bool
	selected      string
	token         string
	listener      net.Listener
	server        *http.Server
	streamMu      sync.Mutex
	subscribers   map[chan []byte]struct{}
	confirmMu     sync.Mutex
	confirmID     string
	confirmAnswer chan string
	nextConfirm   uint64
}

func newAPIServer(cfg *config) (*apiServer, error) {
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	a := &apiServer{cfg: cfg, token: hex.EncodeToString(secret[:]), listener: listener, subscribers: map[chan []byte]struct{}{}}
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
	out("HTTP listening: http://%s (token valid; loopback only)\n", a.listener.Addr())
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
	b, err := json.Marshal(event)
	if err != nil {
		panic(err)
	}
	a.streamMu.Lock()
	defer a.streamMu.Unlock()
	for ch := range a.subscribers {
		select {
		case ch <- b:
		default:
			// A disconnected/slow consumer is closed visibly, never silently fed an
			// incomplete stream. There is no replay queue or execution retry.
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
	case "GET /api/listener":
		apiJSON(w, 200, map[string]interface{}{"address": "127.0.0.1", "port": a.listener.Addr().(*net.TCPAddr).Port, "listening": true, "tokenValid": a.token != ""})
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
	for {
		select {
		case <-r.Context().Done():
			return
		case b, ok := <-ch:
			if !ok {
				return
			}
			var envelope runtimeEvent
			if err := json.Unmarshal(b, &envelope); err != nil {
				return
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", envelope.Type, b); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (a *apiServer) configView() map[string]interface{} {
	// Build from the effective runtime values, never from a key-bearing map.
	c := a.cfg
	return map[string]interface{}{"base_url": c.baseURL, "model": c.model, "workspace": c.workspace, "apiKeyConfigured": c.apiKey != "", "read_only": c.readOnly, "max_ctx": c.maxCtx, "max_rounds": c.maxRounds, "sandbox_preference": c.sandboxPreference, "shell_timeout_sec": int(c.shellTimeout / time.Second), "memory_limit_mb": c.memLimitMB, "process_warn_threshold": c.processWarnThreshold, "background_task_max_output_mb": c.backgroundTaskMaxOutputMB, "background_task_warn_count": c.backgroundTaskWarnCount, "background_task_warn_sec": c.backgroundTaskWarnSec, "cleanup_on_exit": c.cleanupOnExit, "box": c.box, "start_exe": c.startExe, "sandbox_root": c.sandboxRoot, "yolo": c.yolo, "llm_first_chunk_timeout_sec": int(c.llmFirstChunkTimeout / time.Second), "llm_idle_timeout_sec": int(c.llmIdleTimeout / time.Second), "llm_max_retries": c.llmMaxRetries, "llm_compress_timeout_sec": int(c.llmCompressTimeout / time.Second)}
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
	apiJSON(w, 200, a.configView())
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
	rows, err := a.readHistory(id)
	if err != nil {
		apiReadError(w, err)
		return
	}
	if offset > len(rows) {
		apiBad(w, errors.New("offset outside history"))
		return
	}
	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	apiJSON(w, 200, map[string]interface{}{"sessionId": id, "messages": rows[offset:end], "offset": offset, "nextOffset": end, "hasMore": end < len(rows)})
}
func (a *apiServer) sessions(w http.ResponseWriter, r *http.Request) {
	dir := filepath.Join(a.cfg.exeDirStore(), "data", "sessions")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		apiJSON(w, 200, map[string]interface{}{"sessions": []interface{}{}})
		return
	}
	if err != nil {
		apiReadError(w, err)
		return
	}
	list := []interface{}{}
	for _, e := range entries {
		if e.IsDir() || (e.Name() == "audit.jsonl" || e.Name() == "checkpoint-metadata.jsonl" || strings.HasPrefix(e.Name(), "manifest-")) || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		id := sessionTaskID(e.Name())
		rows, err := a.readHistory(id)
		if err != nil {
			apiReadError(w, err)
			return
		}
		path, _ := a.sessionFile(id)
		meta, err := loadSessionMetadata(path)
		if err != nil {
			apiReadError(w, err)
			return
		}
		info, err := e.Info()
		if err != nil {
			apiReadError(w, err)
			return
		}
		first := ""
		for _, raw := range rows {
			var m openai.ChatCompletionMessage
			json.Unmarshal(raw, &m)
			if m.Role == "user" {
				first = m.Content
				break
			}
		}
		list = append(list, map[string]interface{}{"sessionId": id, "cwd": meta.Workspace, "updatedAt": info.ModTime(), "messageCount": len(rows), "firstUser": first})
	}
	apiJSON(w, 200, map[string]interface{}{"sessions": list})
}
func (a *apiServer) toolResult(w http.ResponseWriter, r *http.Request) {
	ref := r.URL.Query().Get("ref")
	id := a.selected
	call := ""
	if strings.HasPrefix(ref, "session:") {
		parts := strings.SplitN(strings.TrimPrefix(ref, "session:"), "#tool:", 2)
		if len(parts) == 2 {
			id, call = parts[0], parts[1]
		}
	} else if strings.HasPrefix(ref, "tool:") {
		call = strings.TrimPrefix(ref, "tool:")
	}
	if id == "" || call == "" {
		apiBad(w, errors.New("invalid result reference"))
		return
	}
	rows, err := a.readHistory(id)
	if err != nil {
		apiReadError(w, err)
		return
	}
	for _, raw := range rows {
		var m openai.ChatCompletionMessage
		json.Unmarshal(raw, &m)
		if m.Role == "tool" && m.ToolCallID == call {
			apiJSON(w, 200, map[string]string{"id": call, "result": m.Content})
			return
		}
	}
	apiError(w, 404, "not_found", "tool result not found")
}

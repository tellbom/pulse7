package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func testAPI(t *testing.T) *apiServer {
	t.Helper()
	root := t.TempDir()
	cfg := &config{exeDir: root, workspace: root, baseURL: "http://127.0.0.1:8080/v1", model: "m", apiKey: "test-secret-never-return", llmFirstChunkTimeout: time.Second}
	a, err := newAPIServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	return a
}
func callAPI(a *apiServer, method, path, body string, auth bool) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if auth {
		r.Header.Set("Authorization", "Bearer "+a.token)
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}
func TestAPIAuthenticationAndConfigRedaction(t *testing.T) {
	a := testAPI(t)
	for _, path := range []string{"/api/config", "/api/listener", "/api/sessions", "/api/events"} {
		w := callAPI(a, "GET", path, "", false)
		if w.Code != 401 {
			t.Fatalf("%s status=%d", path, w.Code)
		}
	}
	w := callAPI(a, "GET", "/api/config", "", true)
	if w.Code != 200 || strings.Contains(w.Body.String(), a.cfg.apiKey) || !strings.Contains(w.Body.String(), "apiKeyConfigured") {
		t.Fatalf("config=%s", w.Body)
	}
	if a.listener.Addr().(*net.TCPAddr).IP.String() != "0.0.0.0" || a.listener.Addr().(*net.TCPAddr).Port == 0 {
		t.Fatal(a.listener.Addr())
	}
}
func TestAPIEmbeddedAssetsHaveBrowserMIMETypes(t *testing.T) {
	a := testAPI(t)
	w := callAPI(a, "GET", "/", "", false)
	if w.Code != 200 || strings.Contains(w.Body.String(), "__PULSE7_TOKEN__") {
		t.Fatal("embedded bootstrap did not render")
	}
	assets := regexp.MustCompile(`(?:src|href)="\./(assets/[^" ]+)"`).FindAllStringSubmatch(w.Body.String(), -1)
	if len(assets) < 2 {
		t.Fatal("missing embedded JS/CSS references")
	}
	for _, match := range assets {
		asset := callAPI(a, "GET", "/"+match[1], "", false)
		kind := asset.Header().Get("Content-Type")
		if asset.Code != 200 || (strings.HasSuffix(match[1], ".js") && !strings.Contains(kind, "javascript")) || (strings.HasSuffix(match[1], ".css") && !strings.Contains(kind, "text/css")) {
			t.Fatalf("asset %s: status %d type %s", match[1], asset.Code, kind)
		}
	}
}
func TestAPIConfigPersistsOnlyGlobalLayer(t *testing.T) {
	a := testAPI(t)
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	project := projectConfigPath(a.cfg.workspace)
	os.MkdirAll(filepath.Dir(project), 0700)
	original := []byte("{\"model\":\"project\"}")
	os.WriteFile(project, original, 0600)
	global := globalConfigPath(home)
	os.MkdirAll(filepath.Dir(global), 0700)
	os.WriteFile(global, []byte("{\"max_rounds\":77}"), 0600)
	w := callAPI(a, "PUT", "/api/config", `{"base_url":"https://example.test/v1","model":"saved","api_key":"secret-new"}`, true)
	if w.Code != 200 || strings.Contains(w.Body.String(), "secret-new") {
		t.Fatalf("save=%d %s", w.Code, w.Body)
	}
	doc, _, err := readConfigDocument(global)
	if err != nil || string(doc["max_rounds"]) != "77" || string(doc["model"]) != `"saved"` {
		t.Fatalf("doc=%v err=%v", doc, err)
	}
	got, _ := os.ReadFile(project)
	if !bytes.Equal(got, original) {
		t.Fatal("project config changed")
	}
	for _, body := range []string{`{"unknown":1}`, `{"base_url":"file:///C:/secret"}`, `{"model":""}`} {
		if w := callAPI(a, "PUT", "/api/config", body, true); w.Code != 400 {
			t.Fatalf("invalid request status=%d", w.Code)
		}
	}
	a.busy = true
	if w := callAPI(a, "PUT", "/api/config", `{"model":"other"}`, true); w.Code != 409 {
		t.Fatal(w.Code)
	}
}
func TestAPISessionPaginationPreservesToolCallsAndBytes(t *testing.T) {
	a := testAPI(t)
	dir := filepath.Join(a.cfg.exeDir, "data", "sessions")
	os.MkdirAll(dir, 0700)
	data := []byte("{\"role\":\"_meta\",\"workspace\":\"C:\\\\work\"}\n" + `{"uuid":"one","role":"assistant","tool_calls":[{"id":"c1","type":"function","function":{"name":"read","arguments":"{\"path\":\"x\"}"}}]}` + "\n" + `{"uuid":"two","role":"tool","tool_call_id":"c1","content":"full result"}` + "\n")
	path := filepath.Join(dir, "sess-example.jsonl")
	os.WriteFile(path, data, 0600)
	w := callAPI(a, "GET", "/api/sessions/sess-example/messages?offset=0&limit=1", "", true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "tool_calls") || !strings.Contains(w.Body.String(), `"hasMore":true`) {
		t.Fatalf("page=%d %s", w.Code, w.Body)
	}
	w = callAPI(a, "GET", "/api/sessions/sess-example/messages?offset=1&limit=1", "", true)
	if !strings.Contains(w.Body.String(), "tool_call_id") {
		t.Fatal(w.Body)
	}
	canonical := callAPI(a, "GET", "/api/sessions/example/messages?limit=1", "", true)
	if canonical.Code != 200 {
		t.Fatalf("canonical session id failed: %d", canonical.Code)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, data) {
		t.Fatal("history modified")
	}
	w = callAPI(a, "GET", "/api/tool-result?ref=session:sess-example%23tool:c1", "", true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "full result") {
		t.Fatal(w.Body)
	}
	for _, query := range []string{"offset=-1", "limit=1001", "offset=3"} {
		if w := callAPI(a, "GET", "/api/sessions/sess-example/messages?"+query, "", true); w.Code != 400 {
			t.Fatalf("%s: %d", query, w.Code)
		}
	}
	if _, err := apiSessionPath(dir, "../outside"); err == nil {
		t.Fatal("traversal accepted")
	}
	os.WriteFile(path, append(data, []byte("broken\n")...), 0600)
	if w := callAPI(a, "GET", "/api/sessions/sess-example/messages", "", true); w.Code != 500 {
		t.Fatal(w.Code)
	}
}
func TestAPISSESameEnvelopeAndListenerRelease(t *testing.T) {
	a := testAPI(t)
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(a.listener.Addr().(*net.TCPAddr).Port))
	go a.server.Serve(a.listener)
	req, _ := http.NewRequest("GET", "http://"+address+"/api/events", nil)
	req.Header.Set("Authorization", "Bearer "+a.token)
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	a.publish(runtimeEvent{Type: "turn_result", Data: turnResultEvent{Status: "success"}})
	reader := bufio.NewReader(response.Body)
	var frame strings.Builder
	var payload string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		frame.WriteString(line)
		if strings.HasPrefix(line, "data: ") {
			payload = strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		}
		if line == "\n" {
			break
		}
	}
	var envelope struct {
		Type      string          `json:"type"`
		Data      turnResultEvent `json:"data"`
		SessionID string          `json:"sessionId"`
		StreamID  string          `json:"streamId"`
		Seq       uint64          `json:"seq"`
	}
	if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Type != "turn_result" || envelope.Data.Status != "success" || envelope.StreamID != a.streamID || envelope.Seq != 1 || envelope.SessionID != "" || !strings.Contains(frame.String(), "event: turn_result") || !strings.Contains(frame.String(), "id: "+a.streamID+":1") {
		t.Fatal(frame.String())
	}
	a.Close()
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	listener.Close()
}
func TestAPIPermissionRejectsStaleAndDuplicateAnswers(t *testing.T) {
	a := testAPI(t)
	a.confirmID = "2"
	a.confirmAnswer = make(chan string, 1)
	for _, id := range []string{"1", ""} {
		w := callAPI(a, "POST", "/api/permission", `{"requestId":"`+id+`","decision":"allow"}`, true)
		if w.Code != 409 {
			t.Fatal(w.Code)
		}
	}
	w := callAPI(a, "POST", "/api/permission", `{"requestId":"2","decision":"deny"}`, true)
	if w.Code != 200 || <-a.confirmAnswer != "n" {
		t.Fatal(w.Code)
	}
	if w := callAPI(a, "POST", "/api/permission", `{"requestId":"2","decision":"allow"}`, true); w.Code != 409 {
		t.Fatal(w.Code)
	}
}

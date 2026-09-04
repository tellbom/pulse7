// Gate C/D: go-openai + mock OpenAI endpoint + TLS verification.
//
// poc-net serve <seconds>              run mock HTTP :8080 + HTTPS :8443 (writes ca.pem/cert.pem/key.pem)
// poc-net client <baseURL> [ca.pem]    full client chain via go-openai (no InsecureSkipVerify)
// poc-net client-noca <baseURL>        negative TLS test: must FAIL cert verification
package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: poc-net serve <sec> | client <baseURL> [ca.pem] | client-noca <baseURL>")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "serve":
		sec := 120
		if len(os.Args) > 2 {
			sec, _ = strconv.Atoi(os.Args[2])
		}
		serve(sec)
	case "client":
		ca := ""
		if len(os.Args) > 3 {
			ca = os.Args[3]
		}
		client(os.Args[2], ca)
	case "client-noca":
		clientNoCA(os.Args[2])
	default:
		fmt.Println("unknown subcommand:", os.Args[1])
		os.Exit(2)
	}
}

// ---------- mock server ----------

func serve(seconds int) {
	caTLS, caPEM := genCert()
	writeFile("ca.pem", caPEM.cert)
	writeFile("cert.pem", caPEM.leaf)
	writeFile("key.pem", caPEM.key)
	fmt.Println("[srv] cert material written: ca.pem cert.pem key.pem")

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", mockHandler)

	go func() {
		s := &http.Server{Addr: "127.0.0.1:8080", Handler: mux}
		fmt.Println("[srv] HTTP listening 127.0.0.1:8080")
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Println("[srv] http err:", err)
		}
	}()

	ts := &http.Server{
		Addr:    "127.0.0.1:8443",
		Handler: mux,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{caTLS},
			MinVersion:   tls.VersionTLS12, // Gate D: TLS 1.2+ only
		},
	}
	go func() {
		fmt.Println("[srv] HTTPS(TLS1.2+) listening 127.0.0.1:8443")
		if err := ts.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Println("[srv] https err:", err)
		}
	}()

	time.Sleep(time.Duration(seconds) * time.Second)
	fmt.Println("[srv] serve window over")
}

func mockHandler(w http.ResponseWriter, r *http.Request) {
	proto := "http"
	vers := "none"
	if r.TLS != nil {
		proto = "https"
		vers = tlsVersionName(r.TLS.Version)
	}
	body, _ := io.ReadAll(r.Body)
	var req openai.ChatCompletionRequest
	json.Unmarshal(body, &req)
	lastRole := ""
	lastContent := ""
	if n := len(req.Messages); n > 0 {
		lastRole = string(req.Messages[n-1].Role)
		lastContent = req.Messages[n-1].Content
	}
	fmt.Printf("[srv] %s %s proto=%s tls=%s stream=%v tools=%d last_role=%s\n",
		r.Method, r.URL.Path, proto, vers, req.Stream, len(req.Tools), lastRole)

	if req.Stream {
		fl, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "no flusher", 500)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if len(req.Tools) > 0 && lastRole == string(openai.ChatMessageRoleUser) {
			sse(w, fl, toolCallChunk("call_1", "get_time", "{}", false))
			sse(w, fl, toolCallChunk("", "", "", true))
			sseDone(w, fl)
			return
		}
		for _, piece := range []string{"Hello ", "from ", "mock. \u4f60\u597dWIN7"} {
			sse(w, fl, contentChunk(piece))
		}
		sseDone(w, fl)
		return
	}

	resp := map[string]interface{}{
		"id":      "mock-resp-1",
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   req.Model,
		"choices": []interface{}{map[string]interface{}{
			"index":         0,
			"finish_reason": "stop",
			"message": map[string]interface{}{"role": "assistant", "content": "Hello from mock. \u4f60\u597dWIN7 non-stream"},
		}},
	}
	if len(req.Tools) > 0 && lastRole == string(openai.ChatMessageRoleUser) {
		resp["choices"] = []interface{}{map[string]interface{}{
			"index":         0,
			"finish_reason": "tool_calls",
			"message": map[string]interface{}{
				"role": "assistant",
				"tool_calls": []interface{}{map[string]interface{}{
					"id":   "call_1",
					"type": "function",
					"function": map[string]interface{}{
						"name": "get_time", "arguments": "{}",
					},
				}},
			},
		}}
	} else if lastRole == string(openai.ChatMessageRoleTool) {
		resp["choices"] = []interface{}{map[string]interface{}{
			"index":         0,
			"finish_reason": "stop",
			"message":       map[string]interface{}{"role": "assistant", "content": "FINAL_ACK:" + lastContent},
		}}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func sse(w http.ResponseWriter, fl http.Flusher, obj map[string]interface{}) {
	b, _ := json.Marshal(obj)
	fmt.Fprintf(w, "data: %s\n\n", b)
	fl.Flush()
}

func sseDone(w http.ResponseWriter, fl http.Flusher) {
	fmt.Fprint(w, "data: [DONE]\n\n")
	fl.Flush()
}

func contentChunk(piece string) map[string]interface{} {
	return map[string]interface{}{
		"id": "mock-chunk", "object": "chat.completion.chunk",
		"choices": []interface{}{map[string]interface{}{ //nolint
			"index": 0, "delta": map[string]interface{}{"content": piece},
		}},
	}
}

func toolCallChunk(id, name, args string, finish bool) map[string]interface{} {
	delta := map[string]interface{}{}
	if id != "" {
		delta["tool_calls"] = []interface{}{map[string]interface{}{
			"index": 0, "id": id, "type": "function",
			"function": map[string]interface{}{"name": name, "arguments": args},
		}}
	}
	fin := ""
	if finish {
		fin = "tool_calls"
	}
	return map[string]interface{}{
		"id": "mock-chunk", "object": "chat.completion.chunk",
		"choices": []interface{}{map[string]interface{}{
			"index": 0, "delta": delta, "finish_reason": fin,
		}},
	}
}

func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS10:
		return "TLS1.0"
	case tls.VersionTLS11:
		return "TLS1.1"
	case tls.VersionTLS12:
		return "TLS1.2"
	case tls.VersionTLS13:
		return "TLS1.3"
	}
	return fmt.Sprintf("0x%04x", v)
}

// ---------- client ----------

func newClient(baseURL, caPath string) *openai.Client {
	cfg := openai.DefaultConfig("poc-dummy-key")
	cfg.BaseURL = baseURL
	if caPath != "" {
		pemBytes := mustRead(caPath)
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pemBytes) {
			fmt.Println("FATAL: cannot load CA pem:", caPath)
			os.Exit(1)
		}
		// Gate D rule: real verification via RootCAs. InsecureSkipVerify is FORBIDDEN and not used.
		cfg.HTTPClient = &http.Client{Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
		}}
	}
	return openai.NewClientWithConfig(cfg)
}

func client(baseURL, caPath string) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	c := newClient(baseURL, caPath)
	failed := false
	check := func(mark string, err error) {
		if err != nil {
			fmt.Printf("MARK %s-FAIL: %v\n", mark, err)
			failed = true
		} else {
			fmt.Printf("MARK %s-ok\n", mark)
		}
	}

	// phase 1: plain JSON response
	resp, err := c.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    "mock-model",
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "say hi"}},
	})
	if err == nil {
		fmt.Println("[phase1] content:", resp.Choices[0].Message.Content)
	}
	check("phase1-json", err)

	// phase 2: SSE streaming, multiple deltas, [DONE]
	stream, err := c.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:    "mock-model",
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "stream please"}},
		Stream:   true,
	})
	if err != nil {
		check("phase2-sse", err)
		os.Exit(1)
	}
	var full strings.Builder
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			fmt.Println("[phase2] stream ended ([DONE])")
			break
		}
		if err != nil {
			check("phase2-sse", err)
			os.Exit(1)
		}
		d := chunk.Choices[0].Delta.Content
		if d != "" {
			fmt.Printf("[phase2][delta] %q\n", d)
			full.WriteString(d)
		}
	}
	stream.Close()
	fmt.Printf("[phase2] full=%q contains_unicode_marker=%v\n", full.String(), strings.Contains(full.String(), "\u4f60\u597dWIN7"))
	if !strings.Contains(full.String(), "\u4f60\u597dWIN7") {
		check("phase2-unicode", errors.New("unicode marker missing"))
	} else {
		check("phase2-unicode", nil)
	}
	check("phase2-sse", nil)

	// phase 3: tool call + tool result round trip
	req := openai.ChatCompletionRequest{
		Model:    "mock-model",
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "what time is it"}},
		Tools: []openai.Tool{{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name: "get_time", Description: "returns local time", Parameters: map[string]interface{}{},
			},
		}},
	}
	tresp, err := c.CreateChatCompletion(ctx, req)
	check("phase3-toolcall", err)
	if err == nil && len(tresp.Choices[0].Message.ToolCalls) == 0 {
		check("phase3-toolcall", errors.New("no tool_calls in response"))
	}
	if err == nil && len(tresp.Choices[0].Message.ToolCalls) > 0 {
		tc := tresp.Choices[0].Message.ToolCalls[0]
		fmt.Printf("[phase3] tool_call id=%s name=%s args=%s\n", tc.ID, tc.Function.Name, tc.Function.Arguments)
		toolResult := "LOCAL_TIME=" + time.Now().Format(time.RFC3339)
		fmt.Println("[phase3] local tool executed:", toolResult)
		req.Messages = append(req.Messages, tresp.Choices[0].Message,
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleTool, ToolCallID: tc.ID, Content: toolResult})
		fresp, err2 := c.CreateChatCompletion(ctx, req)
		if err2 == nil {
			fmt.Println("[phase3] final:", fresp.Choices[0].Message.Content)
			if !strings.HasPrefix(fresp.Choices[0].Message.Content, "FINAL_ACK:LOCAL_TIME=") {
				check("phase3-roundtrip", errors.New("final answer did not echo tool result"))
			}
		}
		check("phase3-roundtrip", err2)
	}

	if failed {
		fmt.Println("RESULT: FAIL")
		os.Exit(1)
	}
	fmt.Println("RESULT: PASS")
}

func clientNoCA(baseURL string) {
	c := newClient(baseURL, "") // system roots only -> self-signed mock must FAIL
	_, err := c.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model:    "mock-model",
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "hi"}},
	})
	if err == nil {
		fmt.Println("MARK noca-FAIL: connection succeeded without CA (verification not enforced?)")
		os.Exit(1)
	}
	fmt.Printf("MARK noca-ok (expected failure): %v\n", err)
}

// ---------- self-signed CA + leaf ----------

type pemMat struct {
	cert, leaf, key []byte
}

func genCert() (tls.Certificate, pemMat) {
	caKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Win7Agent POC CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	caDER, _ := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	caCert, _ := x509.ParseCertificate(caDER)

	leafKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, _ := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)

	srv, _ := tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}),
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(leafKey)}))
	return srv, pemMat{
		cert: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}),
		leaf: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}),
		key:  pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(leafKey)}),
	}
}

func writeFile(name string, b []byte) {
	if err := os.WriteFile(name, b, 0644); err != nil {
		fmt.Println("FATAL write", name, err)
		os.Exit(1)
	}
}

func mustRead(name string) []byte {
	b, err := os.ReadFile(name)
	if err != nil {
		fmt.Println("FATAL read", name, err)
		os.Exit(1)
	}
	return b
}

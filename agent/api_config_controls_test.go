package main

import (
	"encoding/json"
	"flag"
	"os"
	"strings"
	"testing"
)

func TestAPIConfigOmittedFieldsAndRestart(t *testing.T) {
	a := testAPI(t)
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	path := globalConfigPath(home)
	key := "stored-secret"
	if err := saveAPIConfig(path, apiConfigUpdate{APIKey: &key}); err != nil {
		t.Fatal(err)
	}
	w := callAPI(a, "PUT", "/api/config", `{"max_ctx":512000,"max_rounds":200,"llm_max_retries":0,"read_only":true,"memory_limit_mb":512}`, true)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if a.cfg.maxCtx != 512000 || a.cfg.maxRounds != 200 || a.cfg.llmMaxRetries != 0 {
		t.Fatal("turn settings not applied")
	}
	if a.cfg.readOnly || a.cfg.memLimitMB == 512 {
		t.Fatal("restart settings incorrectly hot applied")
	}
	var response struct {
		Restart bool     `json:"restartRequired"`
		Fields  []string `json:"restartRequiredFields"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Restart || len(response.Fields) != 2 {
		t.Fatalf("%s", w.Body)
	}
	doc, _, err := readConfigDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(doc["api_key"]) != `"stored-secret"` || string(doc["read_only"]) != "true" {
		t.Fatal("persistence lost values")
	}
	for k, v := range doc {
		if string(v) == "null" {
			t.Fatalf("omitted field written null: %s", k)
		}
	}
	if strings.Contains(w.Body.String(), key) {
		t.Fatal("secret returned")
	}
}

func TestAPIConfigInvalidOrBusyDoesNotMutate(t *testing.T) {
	a := testAPI(t)
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	for _, body := range []string{`{"max_ctx":-1}`, `{"memory_limit_mb":1048576}`, `{"llm_idle_timeout_sec":2147483647}`, `{"sandbox_preference":"bad"}`, `{"workspace":"C:/"}`, `{"maxSessionRecordBytes":99999999}`, `{"max_ctx":"large"}`} {
		w := callAPI(a, "PUT", "/api/config", body, true)
		if w.Code != 400 {
			t.Fatalf("%s: %d", body, w.Code)
		}
	}
	if _, err := os.Stat(globalConfigPath(home)); !os.IsNotExist(err) {
		t.Fatal("invalid update created config")
	}
	a.busy = true
	w := callAPI(a, "PUT", "/api/config", `{"max_ctx":512000}`, true)
	if w.Code != 409 || a.cfg.maxCtx != 0 {
		t.Fatal("busy mutation")
	}
	a.busy = false
}

func TestAPIConfigWriteFailureDoesNotApply(t *testing.T) {
	a := testAPI(t)
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	// A directory cannot be replaced with the config JSON file.
	if err := os.MkdirAll(globalConfigPath(home), 0700); err != nil {
		t.Fatal(err)
	}
	w := callAPI(a, "PUT", "/api/config", `{"max_ctx":512000}`, true)
	if w.Code != 500 || a.cfg.maxCtx != 0 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestConfigZeroRetriesSurvivesRestart(t *testing.T) {
	cfg := &config{llmMaxRetries: 2}
	ac := defaultAgentConfig()
	ac.LLMMaxRetries = 0
	applyConfigToFlags(cfg, ac, flag.NewFlagSet("test", flag.ContinueOnError))
	if cfg.llmMaxRetries != 0 {
		t.Fatal("zero retry setting lost")
	}
}

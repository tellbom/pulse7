package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

func TestNewSessionMessagesContainSchemaFieldsAndParentChain(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(t.TempDir(), "sess-schema.jsonl")
	s := newSession(path, workspace)
	if err := s.record(openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "first"}); err != nil {
		t.Fatal(err)
	}
	if err := s.record(openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "second"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var records []sessionMessageRecord
	for line := 0; scanner.Scan(); line++ {
		if line == 0 {
			continue
		}
		var record sessionMessageRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("record count = %d, want 2", len(records))
	}
	for i, record := range records {
		if record.UUID == "" || record.SessionID != "schema" || record.Cwd != workspace || record.Version != sessionSchemaVersion {
			t.Fatalf("record %d schema fields = %+v", i, record)
		}
		if _, err := time.Parse(time.RFC3339Nano, record.Timestamp); err != nil {
			t.Fatalf("record %d timestamp = %q: %v", i, record.Timestamp, err)
		}
	}
	if records[0].ParentUUID != nil {
		t.Fatalf("first parentUuid = %v, want null", *records[0].ParentUUID)
	}
	if records[1].ParentUUID == nil || *records[1].ParentUUID != records[0].UUID {
		t.Fatalf("second parentUuid = %v, want %q", records[1].ParentUUID, records[0].UUID)
	}
}

func TestNewRecordContinuesParentChainAfterResume(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(t.TempDir(), "sess-resumed-schema.jsonl")
	s := newSession(path, workspace)
	if err := s.record(openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "before"}); err != nil {
		t.Fatal(err)
	}
	firstUUID := s.lastUUID
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	resumed := newSession(path, workspace)
	if err := resumed.record(openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "after"}); err != nil {
		t.Fatal(err)
	}
	if err := resumed.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	var last sessionMessageRecord
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &last); err != nil {
		t.Fatal(err)
	}
	if last.ParentUUID == nil || *last.ParentUUID != firstUUID {
		t.Fatalf("resumed parentUuid = %v, want %q", last.ParentUUID, firstUUID)
	}
}

func TestLegacySessionMessagesStillLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sess-legacy.jsonl")
	workspace := t.TempDir()
	legacy := `{"role":"_meta","workspace":` + mustJSONQuote(t, workspace) + `}` + "\n" +
		`{"role":"user","content":"legacy message"}` + "\n"
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	msgs, err := loadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].Role != openai.ChatMessageRoleUser || msgs[0].Content != "legacy message" {
		t.Fatalf("legacy messages = %#v", msgs)
	}
}

func TestLayeredConfigPriorityAndProjectAPIKeyWarning(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	t.Setenv("USERPROFILE", home)
	writeJSONFixture(t, globalConfigPath(home), map[string]interface{}{
		"workspace":  workspace,
		"model":      "global-model",
		"api_key":    "global-key",
		"max_rounds": 30,
	})
	writeJSONFixture(t, projectConfigPath(workspace), map[string]interface{}{
		"model":      "project-model",
		"api_key":    "project-key-must-be-ignored",
		"max_rounds": 40,
	})

	ac, warnings, err := loadLayeredAgentConfig(".", false)
	if err != nil {
		t.Fatal(err)
	}
	if ac.Model != "project-model" || ac.APIKey != "global-key" || ac.MaxRounds != 40 || ac.Workspace != workspace {
		t.Fatalf("layered config = %+v", ac)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "api_key") || !strings.Contains(warnings[0], "忽略") {
		t.Fatalf("warnings = %#v", warnings)
	}

	var cfg config
	fs := flag.NewFlagSet("priority", flag.ContinueOnError)
	fs.StringVar(&cfg.model, "model", "default", "")
	if err := fs.Parse([]string{"--model", "flag-model"}); err != nil {
		t.Fatal(err)
	}
	applyConfigToFlags(&cfg, ac, fs)
	if cfg.model != "flag-model" {
		t.Fatalf("flag model = %q, want flag-model", cfg.model)
	}
}

func TestExplicitWorkspaceSelectsProjectConfig(t *testing.T) {
	home := t.TempDir()
	globalWorkspace := t.TempDir()
	flagWorkspace := t.TempDir()
	t.Setenv("USERPROFILE", home)
	writeJSONFixture(t, globalConfigPath(home), map[string]interface{}{
		"workspace": globalWorkspace,
		"model":     "global-model",
	})
	writeJSONFixture(t, projectConfigPath(globalWorkspace), map[string]interface{}{"model": "wrong-project"})
	writeJSONFixture(t, projectConfigPath(flagWorkspace), map[string]interface{}{"model": "selected-project"})

	ac, warnings, err := loadLayeredAgentConfig(flagWorkspace, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 || ac.Model != "selected-project" {
		t.Fatalf("config = %+v warnings=%#v", ac, warnings)
	}
}

func TestProjectConfigWriteRejectsAPIKey(t *testing.T) {
	path := projectConfigPath(t.TempDir())
	err := writeProjectAgentConfig(path, []byte(`{"model":"ok","api_key":"must-not-write"}`))
	if err == nil || !strings.Contains(err.Error(), "api_key") || !strings.Contains(err.Error(), "项目配置") {
		t.Fatalf("writeProjectAgentConfig error = %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("project config should not be written: %v", statErr)
	}
}

func writeJSONFixture(t *testing.T, path string, value interface{}) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

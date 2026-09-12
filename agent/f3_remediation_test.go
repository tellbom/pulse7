package main

import (
	openai "github.com/sashabaranov/go-openai"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestF3CustomSessionRoundTrip(t *testing.T) {
	for _, name := range []string{"custom.jsonl", ""} {
		t.Run(name, func(t *testing.T) {
			cfg := &config{exeDir: t.TempDir(), workspace: t.TempDir()}
			if name != "" {
				cfg.sessionPath = filepath.Join(t.TempDir(), name)
			}
			plan, err := prepareResume(cfg, "generated-task")
			if err != nil {
				t.Fatal(err)
			}
			applyResumePreparation(cfg, plan)
			s := openSessionFor(cfg, plan.TaskID)
			if err := s.record(openai.ChatCompletionMessage{Role: "user", Content: "first"}); err != nil {
				t.Fatal(err)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			meta, err := loadSessionMetadata(s.path)
			if err != nil {
				t.Fatal(err)
			}
			if meta.TaskID != plan.TaskID || meta.ManifestPath != plan.ManifestPath {
				t.Fatalf("identity: %+v %+v", meta, plan)
			}
			cfg.resumePath = s.path
			resumed, err := prepareResume(cfg, "unused")
			if err != nil {
				t.Fatal(err)
			}
			applyResumePreparation(cfg, resumed)
			s = openSessionFor(cfg, resumed.TaskID)
			if err := s.record(openai.ChatCompletionMessage{Role: "assistant", Content: "second"}); err != nil {
				t.Fatal(err)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			msgs, err := loadSession(s.path)
			if err != nil || len(msgs) != 2 {
				t.Fatalf("msgs=%v err=%v", msgs, err)
			}
			if _, err := prepareResume(cfg, "unused-again"); err != nil {
				t.Fatal(err)
			}
			cfg.workspace = t.TempDir()
			_, err = prepareResume(cfg, "unused")
			if err == nil || !strings.Contains(err.Error(), meta.Workspace) || !strings.Contains(err.Error(), cfg.workspace) {
				t.Fatalf("workspace error=%v", err)
			}
		})
	}
}
func TestF3DirectCustomCreationReloads(t *testing.T) {
	cfg := &config{exeDir: t.TempDir(), workspace: t.TempDir(), sessionPath: filepath.Join(t.TempDir(), "custom.jsonl")}
	s := openSessionFor(cfg, "generated-task")
	if err := s.record(openai.ChatCompletionMessage{Role: "user", Content: "first"}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if _, err := loadSessionMetadata(s.path); err != nil {
		t.Fatal(err)
	}
}
func TestF3LegacyIdentityMismatchStillRejected(t *testing.T) {
	p := filepath.Join(t.TempDir(), "custom.jsonl")
	if err := os.WriteFile(p, []byte("{\"role\":\"_meta\",\"workspace\":\"C:\\\\work\",\"task_id\":\"generated\"}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := loadSessionMetadata(p)
	if err == nil || !strings.Contains(err.Error(), "metadata=generated") || !strings.Contains(err.Error(), "filename=custom") {
		t.Fatalf("error=%v", err)
	}
}

package main

import (
	"flag"
	"strings"
	"testing"
)

func TestDefaultAndConfiguredRoundLimit(t *testing.T) {
	ac := defaultAgentConfig()
	if ac.MaxRounds != 100 {
		t.Fatalf("default max rounds = %d, want 100", ac.MaxRounds)
	}
	ac.MaxRounds = 137
	var cfg config
	fs := flag.NewFlagSet("round-limit", flag.ContinueOnError)
	fs.IntVar(&cfg.maxRounds, "max-rounds", 100, "")
	applyConfigToFlags(&cfg, ac, fs)
	if cfg.maxRounds != 137 {
		t.Fatalf("configured max rounds = %d, want 137", cfg.maxRounds)
	}
}

func TestRoundLimitErrorDoesNotClaimCompletion(t *testing.T) {
	err := maxRoundsError(100)
	if !strings.Contains(err.Error(), "100") || !strings.Contains(err.Error(), "without a final answer") {
		t.Fatalf("round-limit error is ambiguous: %v", err)
	}
}

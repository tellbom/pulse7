package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	openai "github.com/sashabaranov/go-openai"
)

func TestSkillCatalogBudgetModesAndFullIndex(t *testing.T) {
	for _, tc := range []struct {
		name          string
		count, budget int
		mode          string
	}{
		{"full", 1, 8192, "full"},
		{"shortened", 1, 2048, "shortened"},
		{"names", 10, 2048, "names"},
		{"index", 30, 1024, "index"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config{exeDir: t.TempDir(), skillCatalogBudgetBytes: tc.budget}
			catalog := skillCatalog{}
			for i := 0; i < tc.count; i++ {
				catalog.Skills = append(catalog.Skills, skillInfo{Name: string(rune('a' + i)), Path: "C:/skills/example/SKILL.md", Scope: "global", Description: strings.Repeat("说明😀\"\\", 100)})
			}
			listing, state, err := buildSkillListing(cfg, catalog)
			if err != nil {
				t.Fatal(err)
			}
			if state.Mode != tc.mode || state.ListingBytes > tc.budget || state.ListingBytes != skillListingBytes(listing) || !utf8.ValidString(listing) {
				t.Fatalf("state=%+v listing=%s", state, listing)
			}
			if state.IndexPath != "" {
				b, err := os.ReadFile(state.IndexPath)
				if err != nil {
					t.Fatal(err)
				}
				lines := strings.Split(strings.TrimSpace(string(b)), "\n")
				if len(lines) != tc.count {
					t.Fatal("index lost skills")
				}
				for _, line := range lines {
					var skill skillInfo
					if err = json.Unmarshal([]byte(line), &skill); err != nil {
						t.Fatal(err)
					}
					if skill.Description != catalog.Skills[0].Description {
						t.Fatal("index lost description")
					}
				}
			}
		})
	}
}

func TestSkillCatalogRefreshReplacesAndRemovesWithoutTouchingHistory(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	workspace := filepath.Join(root, "workspace")
	t.Setenv("USERPROFILE", home)
	path := writeSkillFixture(t, workspace, "review", "review", "old purpose", "PRIVATE_BODY")
	cfg := &config{workspace: workspace, exeDir: root}
	base := "base instructions"
	msgs := []openai.ChatCompletionMessage{{Role: "system", Content: base + "\n\n" + legacySkillHeading + "- legacy (old): old"}, {Role: "user", Content: "task"}, {Role: "tool", Content: "historical evidence"}}
	if err := refreshSkillCatalog(cfg, &msgs); err != nil {
		t.Fatal(err)
	}
	writeSkillFixture(t, workspace, "review", "review", "new purpose", "NEW_PRIVATE_BODY")
	if !strings.Contains(msgs[1].Content, "old purpose") {
		t.Fatal("active snapshot changed without a refresh")
	}
	if err := refreshSkillCatalog(cfg, &msgs); err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 5 || msgs[0].Content != base || !strings.Contains(msgs[1].Content, "new purpose") || strings.Contains(msgs[1].Content, "PRIVATE_BODY") || msgs[4].Content != "historical evidence" || !strings.HasPrefix(msgs[2].Content, skillInstallMarker) {
		t.Fatalf("refresh=%+v", msgs)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := refreshSkillCatalog(cfg, &msgs); err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 4 || msgs[0].Content != base || msgs[3].Content != "historical evidence" || !strings.HasPrefix(msgs[1].Content, skillInstallMarker) {
		t.Fatalf("delete=%+v", msgs)
	}
	// Another workspace must not inherit the previous workspace's catalog.
	writeSkillFixture(t, home, "global", "global", "global purpose", "BODY")
	cfg.workspace = filepath.Join(root, "other")
	if err := refreshSkillCatalog(cfg, &msgs); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msgs[1].Content, "global purpose") || strings.Contains(msgs[1].Content, "new purpose") {
		t.Fatal("workspace isolation")
	}
}

func TestSkillWorkspaceOverridesGlobalByName(t *testing.T) {
	workspace, home := t.TempDir(), t.TempDir()
	writeSkillFixture(t, workspace, "local", "review", "workspace version", "body")
	writeSkillFixture(t, home, "personal", "review", "global version", "body")
	catalog := scanSkills(workspace, home)
	if len(catalog.Skills) != 1 || catalog.Skills[0].Scope != "workspace" || catalog.Skills[0].Description != "workspace version" || len(catalog.Warnings) != 1 {
		t.Fatalf("catalog=%+v", catalog)
	}
}

func TestSkillCatalogIndexWriteFailureIsReported(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "data"), []byte("block directory"), 0600); err != nil {
		t.Fatal(err)
	}
	_, _, err := buildSkillListing(&config{exeDir: root, skillCatalogBudgetBytes: 1024}, skillCatalog{Skills: []skillInfo{{Name: "x", Path: "x", Description: strings.Repeat("x", 9000)}}})
	if err == nil || !strings.Contains(err.Error(), "skill catalog index") {
		t.Fatalf("err=%v", err)
	}
}

func TestSkillBudgetAPIAndPersistence(t *testing.T) {
	a := testAPI(t)
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	w := callAPI(a, "GET", "/api/config", "", true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"skill_catalog_budget_bytes":8192`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	w = callAPI(a, "PUT", "/api/config", `{"skill_catalog_budget_bytes":16384}`, true)
	if w.Code != 200 || a.cfg.skillCatalogBudgetBytes != 16384 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	doc, _, err := readConfigDocument(globalConfigPath(home))
	if err != nil {
		t.Fatal(err)
	}
	if string(doc["skill_catalog_budget_bytes"]) != "16384" {
		t.Fatal("budget not persisted")
	}
	ac := defaultAgentConfig()
	b, _ := json.Marshal(doc)
	if err = json.Unmarshal(b, &ac); err != nil {
		t.Fatal(err)
	}
	if ac.SkillCatalogBudgetBytes != 16384 {
		t.Fatal("budget not reloaded")
	}
	restarted := &config{}
	applyConfigToFlags(restarted, ac, flag.NewFlagSet("restart", flag.ContinueOnError))
	if restarted.skillCatalogBudgetBytes != 16384 {
		t.Fatal("restart did not apply budget")
	}
	for _, input := range []string{`{"skill_catalog_budget_bytes":0}`, `{"skill_catalog_budget_bytes":1023}`, `{"skill_catalog_budget_bytes":1048577}`} {
		w = callAPI(a, "PUT", "/api/config", input, true)
		if w.Code != 400 || a.cfg.skillCatalogBudgetBytes != 16384 {
			t.Fatalf("invalid changed budget: %d %s", w.Code, w.Body)
		}
	}
	a.busy = true
	w = callAPI(a, "PUT", "/api/config", `{"skill_catalog_budget_bytes":8192}`, true)
	if w.Code != 409 || a.cfg.skillCatalogBudgetBytes != 16384 {
		t.Fatal("busy config changed")
	}
}

func TestSkillIndexUsesExistingReadPagination(t *testing.T) {
	r, _, workspace, _ := newPermissionTestRegistry(t, "open", nil, "")
	catalog := skillCatalog{}
	for i := 0; i < 30; i++ {
		catalog.Skills = append(catalog.Skills, skillInfo{Name: fmt.Sprintf("skill-%02d", i), Path: "C:/skills/example/SKILL.md", Description: "complete metadata"})
	}
	_, state, err := buildSkillListing(&config{exeDir: workspace, skillCatalogBudgetBytes: 1024}, catalog)
	if err != nil {
		t.Fatal(err)
	}
	got := r.Execute("read", mustJSON(t, map[string]interface{}{"path": state.IndexPath, "offset": 21, "limit": 1}))
	if !strings.Contains(got, "skill-20") || strings.Contains(got, "skill-19") || strings.Contains(got, "skill-21") {
		t.Fatalf("index page: %s", got)
	}
}

func TestSkillDescriptionBudgetKeepsUTF8AndMarksOmission(t *testing.T) {
	value := strings.Repeat("中😀文", 30)
	for limit := 3; limit < 70; limit++ {
		got := shortenSkillDescription(value, limit)
		if !utf8.ValidString(got) || len(got) > limit || !strings.HasSuffix(got, "…") {
			t.Fatalf("limit=%d got=%q", limit, got)
		}
	}
}

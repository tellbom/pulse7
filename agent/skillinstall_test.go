package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestSkillInstallationRootsPresentWhenEmptyAndRefreshOnResume(t *testing.T) {
	home, workspace := t.TempDir(), t.TempDir()
	t.Setenv("USERPROFILE", home)
	cfg := &config{workspace: workspace}
	msgs := []openai.ChatCompletionMessage{{Role: "system", Content: "base"}, {Role: "system", Content: skillInstallMarker + "stale roots"}, {Role: "user", Content: "install a skill"}}
	for i := 0; i < 2; i++ {
		if _, err := refreshSkillCatalog(cfg, &msgs); err != nil {
			t.Fatal(err)
		}
		if len(msgs) != 3 || msgs[0].Content != "base" || msgs[2].Content != "install a skill" {
			t.Fatalf("messages: %#v", msgs)
		}
		policy := msgs[1].Content
		if !strings.HasPrefix(policy, skillInstallMarker) || strings.Contains(policy, "stale roots") {
			t.Fatal("missing/refreshed policy")
		}
		var roots []skillRoot
		if err := json.Unmarshal([]byte(strings.Split(strings.TrimPrefix(policy, skillInstallMarker), "\n")[0]), &roots); err != nil {
			t.Fatal(err)
		}
		want := skillRoots(cfg.workspace, home)
		if len(roots) != 2 || roots[0] != want[0] || roots[1] != want[1] {
			t.Fatalf("roots: %#v", roots)
		}
		cfg.workspace = t.TempDir()
	}
}

func TestSkillDiscoveryContinuesWithoutHomeDirectory(t *testing.T) {
	workspace := t.TempDir()
	t.Setenv("USERPROFILE", "")
	if _, err := os.UserHomeDir(); err == nil {
		t.Skip("home directory still resolvable on this platform")
	}
	writeSkillFixture(t, workspace, "release", "发布流程", "发布时使用", "BODY")
	catalog := discoverSkills(workspace)
	if len(catalog.Skills) != 1 || catalog.Skills[0].Scope != "workspace" {
		t.Fatalf("workspace skills lost: %#v", catalog)
	}
	if len(catalog.Roots) != 1 || catalog.Roots[0].Scope != "workspace" {
		t.Fatalf("roots: %#v", catalog.Roots)
	}
	if len(catalog.Warnings) != 1 || !strings.Contains(catalog.Warnings[0], "个人目录") {
		t.Fatalf("warnings: %#v", catalog.Warnings)
	}
	cfg := &config{workspace: workspace, exeDir: t.TempDir()}
	msgs := []openai.ChatCompletionMessage{{Role: "system", Content: "base"}, {Role: "user", Content: "task"}}
	if _, err := refreshSkillCatalog(cfg, &msgs); err != nil {
		t.Fatalf("turn failed on home directory: %v", err)
	}
	var roots []skillRoot
	if err := json.Unmarshal([]byte(strings.Split(strings.TrimPrefix(msgs[2].Content, skillInstallMarker), "\n")[0]), &roots); err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 || roots[0] != catalog.Roots[0] || !strings.Contains(msgs[1].Content, "发布流程") {
		t.Fatalf("messages: %#v", msgs)
	}
}

func TestSkillDownloadsOutsideRootsAreNotInstalled(t *testing.T) {
	workspace, home := t.TempDir(), t.TempDir()
	download := filepath.Join(workspace, "skills", "community--review")
	if err := os.MkdirAll(download, 0700); err != nil {
		t.Fatal(err)
	}
	content := []byte("---\nname: review\ndescription: review code\n---\nBODY_SECRET\n")
	if err := os.WriteFile(filepath.Join(download, "SKILL.md"), content, 0600); err != nil {
		t.Fatal(err)
	}
	if c := scanSkills(workspace, home); len(c.Skills) != 0 {
		t.Fatalf("download registered: %#v", c)
	}
	// Mimic a provider-neutral complete package copy into the approved root.
	installed := filepath.Join(skillRoots(workspace, home)[0].Path, "community--review")
	if err := os.MkdirAll(filepath.Join(installed, "references"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installed, "SKILL.md"), content, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installed, "references", "rules.md"), []byte("RESOURCE_SECRET"), 0600); err != nil {
		t.Fatal(err)
	}
	c := scanSkills(workspace, home)
	if len(c.Skills) != 1 || c.Skills[0].Name != "review" || c.Skills[0].Scope != "workspace" {
		t.Fatalf("installed: %#v", c)
	}
	listing, _, err := buildSkillListing(&config{workspace: workspace}, c)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(listing, "BODY_SECRET") || strings.Contains(listing, "RESOURCE_SECRET") {
		t.Fatal("body/resource injected")
	}
	// Both download and supporting resource remain in place, without implicit moves.
	if _, err := os.Stat(filepath.Join(download, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(installed, "references", "rules.md")); err != nil {
		t.Fatal(err)
	}
}

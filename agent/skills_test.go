package main

import (
	openai "github.com/sashabaranov/go-openai"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func writeSkillFixture(t *testing.T, root, dir, name, description, body string) string {
	t.Helper()
	path := filepath.Join(root, ".pulse7", "skills", dir, skillMarkdownFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: " + description + "\n---\n" + body
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSkillsMetadataIsInjectedWithoutBody(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	projectPath := writeSkillFixture(t, workspace, "release", "发布流程", "发布、打包和出版本时使用。", "PROJECT_BODY_SECRET")
	personalPath := writeSkillFixture(t, home, "database", "数据库迁移", "执行数据库迁移和回滚时使用。", "PERSONAL_BODY_SECRET")
	t.Setenv("USERPROFILE", home)

	catalog := scanSkills(workspace, home)
	if len(catalog.Skills) != 2 || len(catalog.Warnings) != 0 {
		t.Fatalf("catalog = %#v", catalog)
	}
	block := skillsSystemBlock(catalog.Skills)
	for _, want := range []string{"发布流程", "发布、打包和出版本时使用。", filepath.Join(".pulse7", "skills", "release", "SKILL.md"), "数据库迁移", personalPath} {
		if !strings.Contains(block, want) {
			t.Fatalf("skills block missing %q: %s", want, block)
		}
	}
	for _, body := range []string{"PROJECT_BODY_SECRET", "PERSONAL_BODY_SECRET"} {
		if strings.Contains(block, body) {
			t.Fatalf("skills block leaked body %q: %s", body, block)
		}
	}
	cfg := &config{workspace: workspace, exeDir: root}
	msgs := []openai.ChatCompletionMessage{systemMessage(cfg)}
	if err := refreshSkillCatalog(cfg, &msgs); err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 || !strings.Contains(msgs[1].Content, "发布流程") || strings.Contains(msgs[1].Content, "PROJECT_BODY_SECRET") || strings.Contains(msgs[1].Content, "PERSONAL_BODY_SECRET") || !strings.HasPrefix(msgs[2].Content, skillInstallMarker) {
		t.Fatalf("metadata-only projection failed: %#v", msgs)
	}
	if filepath.IsAbs(catalog.Skills[0].Path) || catalog.Skills[0].Path == projectPath {
		t.Fatalf("project skill path should be workspace-relative: %q", catalog.Skills[0].Path)
	}
}

func TestMalformedSkillsAreSkippedWithWarnings(t *testing.T) {
	workspace := t.TempDir()
	home := t.TempDir()
	badDir := filepath.Join(workspace, ".pulse7", "skills", "bad")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badDir, skillMarkdownFileName), []byte("name: no-frontmatter\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeSkillFixture(t, workspace, "missing-description", "缺描述", "", "body")

	catalog := scanSkills(workspace, home)
	if len(catalog.Skills) != 0 || len(catalog.Warnings) != 2 {
		t.Fatalf("malformed catalog = %#v", catalog)
	}
	for _, warning := range catalog.Warnings {
		if !strings.Contains(warning, "frontmatter") {
			t.Fatalf("malformed warning = %q", warning)
		}
	}
}

func TestSkillFrontmatterIsYAML(t *testing.T) {
	for _, tc := range []struct {
		label, content, name, description string
	}{
		{"comments, blank lines, quotes and extra keys", "---\r\n# publisher comment\r\n\r\nname: \"review\"\r\ndescription: 'Review code, then report.'\r\nversion: 1\r\ntags:\r\n  - go\r\n  - windows\r\n---\r\nBODY", "review", "Review code, then report."},
		{"multi-line folded description", "---\nname: release\ndescription: >\n  Use when publishing\n  a new build.\n---\n", "release", "Use when publishing a new build."},
		{"nested keys are ignored", "---\nname: db\nmetadata:\n  author: someone\ndescription: migrate\n---\n", "db", "migrate"},
	} {
		name, description, err := parseSkillFrontmatter(tc.content)
		if err != nil || name != tc.name || description != tc.description {
			t.Fatalf("%s: name=%q description=%q err=%v", tc.label, name, description, err)
		}
	}
	for _, tc := range []struct{ label, content, reason string }{
		{"no leading marker", "name: x\ndescription: y\n", "缺失"},
		{"unclosed", "---\nname: x\ndescription: y\n", "未闭合"},
		{"invalid yaml", "---\nname: [\ndescription: y\n---\n", "无法解析"},
		{"empty description", "---\nname: x\ndescription: \"\"\n---\n", "非空"},
	} {
		if _, _, err := parseSkillFrontmatter(tc.content); err == nil || !strings.Contains(err.Error(), tc.reason) {
			t.Fatalf("%s: err=%v", tc.label, err)
		}
	}
}

func TestSkillLoadedJudgmentUsesRootsNotDiscovery(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	home := filepath.Join(root, "home")
	good := writeSkillFixture(t, workspace, "community--release", "发布流程", "发布时使用", "BODY")
	globalGood := writeSkillFixture(t, home, "db", "数据库", "迁移时使用", "BODY")
	badDir := filepath.Join(workspace, ".pulse7", "skills", "broken")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(badDir, skillMarkdownFileName)
	if err := os.WriteFile(bad, []byte("no frontmatter"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog := scanSkills(workspace, home)
	if len(catalog.Skills) != 2 || len(catalog.Warnings) != 1 {
		t.Fatalf("catalog = %#v", catalog)
	}
	read := func(path string) (skillInfo, bool) {
		return loadedSkillForRead(catalog, workspace, "read", mustJSON(t, map[string]string{"path": path}))
	}
	if skill, ok := read(good); !ok || skill.Name != "发布流程" || skill.Path != filepath.Join(".pulse7", "skills", "community--release", "SKILL.md") {
		t.Fatalf("discovered workspace skill: %#v %v", skill, ok)
	}
	if skill, ok := read(filepath.Join(".pulse7", "skills", "community--release", "skill.md")); !ok || skill.Name != "发布流程" {
		t.Fatalf("relative, case-insensitive read: %#v %v", skill, ok)
	}
	if skill, ok := read(globalGood); !ok || skill.Name != "数据库" || skill.Scope != "global" || skill.Path != globalGood {
		t.Fatalf("discovered global skill: %#v %v", skill, ok)
	}
	// Skipped by discovery, still a package read under the workspace root.
	if skill, ok := read(bad); !ok || skill.Name != "broken" || skill.Scope != "workspace" || skill.Path != filepath.Join(".pulse7", "skills", "broken", "SKILL.md") {
		t.Fatalf("skipped package read: %#v %v", skill, ok)
	}
	for _, path := range []string{
		filepath.Join(workspace, "skills", "community--release", "SKILL.md"),
		filepath.Join(workspace, ".pulse7", "skills", "SKILL.md"),
		filepath.Join(workspace, ".pulse7", "skills", "community--release", "references", "SKILL.md"),
		filepath.Join(workspace, ".pulse7", "skills", "community--release", "README.md"),
		filepath.Join(root, ".pulse7", "skills", "x", "SKILL.md"),
	} {
		if skill, ok := read(path); ok {
			t.Fatalf("%s registered as %#v", path, skill)
		}
	}
	if _, ok := loadedSkillForRead(catalog, workspace, "grep", mustJSON(t, map[string]string{"path": good})); ok {
		t.Fatal("non-read tool registered a skill")
	}
}

func TestSkillsKeepCompleteMetadataBeyondTwenty(t *testing.T) {
	workspace, home := t.TempDir(), t.TempDir()
	description := strings.Repeat("界", 421)
	for i := 0; i < 25; i++ {
		writeSkillFixture(t, workspace, "skill-"+strconv.Itoa(i), "技能"+strconv.Itoa(i), description, "BODY_NOT_IN_CATALOG")
	}
	catalog := scanSkills(workspace, home)
	if len(catalog.Skills) != 25 || len(catalog.Warnings) != 0 {
		t.Fatalf("catalog: %#v", catalog)
	}
	for _, skill := range catalog.Skills {
		if skill.Description != description || skill.Scope != "workspace" {
			t.Fatalf("metadata lost: %#v", skill)
		}
	}
}

func TestReadAllowsPersonalSkillAndOtherOutsideFiles(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	t.Setenv("USERPROFILE", home)
	r, _, workspace, _ := newPermissionTestRegistry(t, "open", nil, "")
	personalPath := writeSkillFixture(t, home, "release", "发布", "发布时使用", "PERSONAL_SKILL_BODY")
	got := r.Execute("read", mustJSON(t, map[string]string{"path": personalPath}))
	if strings.HasPrefix(got, "error:") || !strings.Contains(got, "PERSONAL_SKILL_BODY") {
		t.Fatalf("personal skill read = %q", got)
	}

	other := filepath.Join(filepath.Dir(workspace), "outside.txt")
	if err := os.WriteFile(other, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	got = r.Execute("read", mustJSON(t, map[string]string{"path": other}))
	if strings.HasPrefix(got, "error:") || !strings.Contains(got, "outside") {
		t.Fatalf("unrelated outside read = %q, want outside file content", got)
	}
}

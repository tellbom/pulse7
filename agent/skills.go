package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	defaultSkillCatalogBudgetBytes = 8 << 10
	skillMarkdownFileName          = "SKILL.md"
)

type skillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
	Scope       string `json:"scope"`
}

type skillCatalog struct {
	Skills   []skillInfo
	Warnings []string
}

type skillRoot struct {
	Path  string `json:"path"`
	Scope string `json:"scope"`
}

// One source of truth for discovery and the model's installation instructions.
// Directory names belong to the publisher; only the two roots and SKILL.md
// layout are pulse7's contract.
func skillRoots(workspace, home string) []skillRoot {
	return []skillRoot{
		{Path: filepath.Join(workspace, ".pulse7", "skills"), Scope: "workspace"},
		{Path: filepath.Join(home, ".pulse7", "skills"), Scope: "global"},
	}
}

func discoverSkills(workspace string) skillCatalog {
	home, err := os.UserHomeDir()
	if err != nil {
		return skillCatalog{Warnings: []string{fmt.Sprintf("无法确定个人目录，跳过个人 skills：%v", err)}}
	}
	return scanSkills(workspace, home)
}

func scanSkills(workspace, home string) skillCatalog {
	var catalog skillCatalog
	dirs := skillRoots(workspace, home)
	seen := map[string]bool{}
	for _, source := range dirs {
		entries, err := os.ReadDir(source.Path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			catalog.Warnings = append(catalog.Warnings, fmt.Sprintf("无法扫描 skills 目录 %s：%v", source.Path, err))
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			path := filepath.Join(source.Path, entry.Name(), skillMarkdownFileName)
			b, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				catalog.Warnings = append(catalog.Warnings, fmt.Sprintf("无法读取 skill %s：%v", path, err))
				continue
			}
			name, description, err := parseSkillFrontmatter(string(b))
			if err != nil {
				catalog.Warnings = append(catalog.Warnings, fmt.Sprintf("跳过 skill %s：%v", path, err))
				continue
			}
			if seen[name] {
				catalog.Warnings = append(catalog.Warnings, fmt.Sprintf("skill %s 被更高优先级或先发现的同名技能覆盖：%s", name, path))
				continue
			}
			seen[name] = true
			displayPath := path
			if source.Scope == "workspace" {
				displayPath, err = filepath.Rel(workspace, path)
				if err != nil {
					catalog.Warnings = append(catalog.Warnings, fmt.Sprintf("无法生成 skill 相对路径 %s：%v", path, err))
					continue
				}
			}
			catalog.Skills = append(catalog.Skills, skillInfo{Name: name, Description: description, Path: displayPath, Scope: source.Scope})
		}
	}
	return catalog
}

// The frontmatter block between the leading and closing "---" lines is YAML;
// only name and description are pulse7's contract, other keys belong to the
// publisher and are ignored.
func parseSkillFrontmatter(content string) (string, string, error) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", errors.New("frontmatter 缺失：首行必须是 ---")
	}
	end := -1
	for i, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			end = i + 1
			break
		}
	}
	if end < 0 {
		return "", "", errors.New("frontmatter 未闭合：缺少结束的 ---")
	}
	var meta struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &meta); err != nil {
		return "", "", fmt.Errorf("frontmatter YAML 无法解析：%v", err)
	}
	name, description := strings.TrimSpace(meta.Name), strings.TrimSpace(meta.Description)
	if name == "" || description == "" {
		return "", "", errors.New("frontmatter 缺少非空的 name 或 description")
	}
	return name, description, nil
}

func skillsSystemBlock(skills []skillInfo) string {
	if len(skills) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("可用技能（相关时用 read 工具读取完整内容）：\n")
	for _, skill := range skills {
		fmt.Fprintf(&b, "- %s (%s)：%s\n", skill.Name, skill.Path, skill.Description)
	}
	return strings.TrimRight(b.String(), "\n")
}

func printSkillWarnings(catalog skillCatalog) {
	for _, warning := range catalog.Warnings {
		out("[警告] %s\n", warning)
	}
}

func skillsList(catalog skillCatalog) string {
	if len(catalog.Skills) == 0 {
		return "当前未检测到 skills。"
	}
	return skillsSystemBlock(catalog.Skills)
}

func loadedSkillForRead(workspace, tool, argsJSON string) (skillInfo, bool) {
	if tool != "read" {
		return skillInfo{}, false
	}
	var args struct {
		Path string `json:"path"`
	}
	if json.Unmarshal([]byte(argsJSON), &args) != nil || args.Path == "" {
		return skillInfo{}, false
	}
	requested := args.Path
	if !filepath.IsAbs(requested) {
		requested = filepath.Join(workspace, requested)
	}
	requested, err := filepath.Abs(requested)
	if err != nil {
		return skillInfo{}, false
	}
	for _, skill := range discoverSkills(workspace).Skills {
		path := skill.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(workspace, path)
		}
		path, err = filepath.Abs(path)
		if err == nil && strings.EqualFold(filepath.Clean(requested), filepath.Clean(path)) {
			return skill, true
		}
	}
	return skillInfo{}, false
}

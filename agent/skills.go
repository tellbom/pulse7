package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func discoverSkills(workspace string) skillCatalog {
	home, err := os.UserHomeDir()
	if err != nil {
		return skillCatalog{Warnings: []string{fmt.Sprintf("无法确定个人目录，跳过个人 skills：%v", err)}}
	}
	return scanSkills(workspace, home)
}

func scanSkills(workspace, home string) skillCatalog {
	var catalog skillCatalog
	dirs := []struct {
		root    string
		project bool
	}{
		{root: filepath.Join(workspace, ".pulse7", "skills"), project: true},
		{root: filepath.Join(home, ".pulse7", "skills")},
	}
	seen := map[string]bool{}
	for _, source := range dirs {
		entries, err := os.ReadDir(source.root)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			catalog.Warnings = append(catalog.Warnings, fmt.Sprintf("无法扫描 skills 目录 %s：%v", source.root, err))
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			path := filepath.Join(source.root, entry.Name(), skillMarkdownFileName)
			b, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				catalog.Warnings = append(catalog.Warnings, fmt.Sprintf("无法读取 skill %s：%v", path, err))
				continue
			}
			name, description, ok := parseSkillFrontmatter(string(b))
			if !ok {
				catalog.Warnings = append(catalog.Warnings, fmt.Sprintf("跳过 frontmatter 缺失或格式错误的 skill：%s", path))
				continue
			}
			if seen[name] {
				catalog.Warnings = append(catalog.Warnings, fmt.Sprintf("skill %s 被更高优先级或先发现的同名技能覆盖：%s", name, path))
				continue
			}
			seen[name] = true
			scope := "global"
			if source.project {
				scope = "workspace"
			}
			displayPath := path
			if source.project {
				displayPath, err = filepath.Rel(workspace, path)
				if err != nil {
					catalog.Warnings = append(catalog.Warnings, fmt.Sprintf("无法生成 skill 相对路径 %s：%v", path, err))
					continue
				}
			}
			catalog.Skills = append(catalog.Skills, skillInfo{Name: name, Description: description, Path: displayPath, Scope: scope})
		}
	}
	return catalog
}

func parseSkillFrontmatter(content string) (string, string, bool) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if len(lines) < 4 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", false
	}
	name, description := "", ""
	closed := false
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "---" {
			closed = true
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return "", "", false
		}
		switch strings.TrimSpace(key) {
		case "name":
			name = strings.TrimSpace(value)
		case "description":
			description = strings.TrimSpace(value)
		}
	}
	return name, description, closed && name != "" && description != ""
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

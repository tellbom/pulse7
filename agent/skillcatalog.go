package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	openai "github.com/sashabaranov/go-openai"
)

const skillCatalogMarker = "[pulse7:skill-catalog:v1]\n"
const legacySkillHeading = "可用技能（相关时用 read 工具读取完整内容）：\n"

type skillCatalogState struct {
	Version      string   `json:"version"`
	Mode         string   `json:"mode"`
	Count        int      `json:"count"`
	BudgetBytes  int      `json:"budgetBytes"`
	ListingBytes int      `json:"listingBytes"`
	IndexPath    string   `json:"indexPath,omitempty"`
	Warnings     []string `json:"warnings"`
}

func effectiveSkillBudget(cfg *config) int {
	if cfg.skillCatalogBudgetBytes == 0 {
		return defaultSkillCatalogBudgetBytes
	}
	return cfg.skillCatalogBudgetBytes
}

// Measure the actual serialized system message, including JSON escaping and framing.
func skillListingBytes(content string) int {
	b, _ := json.Marshal(openai.ChatCompletionMessage{Role: "system", Content: content})
	return len(b)
}

func shortenSkillDescription(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	if limit < len("…") {
		return ""
	}
	n := limit - len("…")
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + "…"
}

func renderSkillListing(skills []skillInfo, descriptionBytes int, indexPath string) string {
	var b strings.Builder
	b.WriteString(skillCatalogMarker)
	b.WriteString("可用技能目录（仅元数据，不含正文）。匹配任务时先用 read 读取列出的 SKILL.md，再判断如何应用；相对资源路径以技能目录为基准。\n")
	if indexPath != "" {
		fmt.Fprintf(&b, "描述因目录预算缩短或省略；完整元数据索引：%s。可用 read 分页读取或 grep 检索，然后按 path 读取技能正文。\n", indexPath)
	}
	for _, skill := range skills {
		// JSON lines keep names and descriptions from breaking the catalog structure.
		row := skill
		if descriptionBytes == 0 {
			row.Description = ""
		} else if descriptionBytes > 0 {
			row.Description = shortenSkillDescription(row.Description, descriptionBytes)
		}
		encoded, _ := json.Marshal(row)
		b.Write(encoded)
		b.WriteByte('\n')
	}
	return b.String()
}

func buildSkillListing(cfg *config, catalog skillCatalog) (string, skillCatalogState, error) {
	state := skillCatalogState{Count: len(catalog.Skills), BudgetBytes: effectiveSkillBudget(cfg), Warnings: catalog.Warnings, Mode: "full"}
	if state.BudgetBytes < 1024 || state.BudgetBytes > 1048576 {
		return "", state, fmt.Errorf("skill_catalog_budget_bytes must be between 1024 and 1048576")
	}
	var index strings.Builder
	for _, skill := range catalog.Skills {
		b, _ := json.Marshal(skill)
		index.Write(b)
		index.WriteByte('\n')
	}
	state.Version = fmt.Sprintf("%x", sha256.Sum256([]byte(index.String())))
	if len(catalog.Skills) == 0 {
		state.Mode = "empty"
		return "", state, nil
	}
	listing := renderSkillListing(catalog.Skills, -1, "")
	if skillListingBytes(listing) > state.BudgetBytes {
		// Store metadata only, outside the user's project. Content addressing keeps
		// an older in-flight turn's index stable if another process installs a skill.
		path, err := filepath.Abs(filepath.Join(cfg.exeDirStore(), "data", "skill-catalogs", state.Version+".jsonl"))
		if err != nil {
			return "", state, err
		}
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return "", state, fmt.Errorf("skill catalog index: %w", err)
		}
		if old, readErr := os.ReadFile(path); readErr != nil || string(old) != index.String() {
			if err = writeSkillIndex(path, index.String()); err != nil {
				return "", state, fmt.Errorf("skill catalog index: %w", err)
			}
		}
		state.IndexPath = path
		// Use a common per-description byte allowance, without guessing relevance.
		lo, hi := 48, state.BudgetBytes
		best := ""
		for lo <= hi {
			mid := lo + (hi-lo)/2
			candidate := renderSkillListing(catalog.Skills, mid, path)
			if skillListingBytes(candidate) <= state.BudgetBytes {
				best = candidate
				lo = mid + 1
			} else {
				hi = mid - 1
			}
		}
		if best != "" {
			listing = best
			state.Mode = "shortened"
		} else {
			listing = renderSkillListing(catalog.Skills, 0, path)
			state.Mode = "names"
			if skillListingBytes(listing) > state.BudgetBytes {
				listing = skillCatalogMarker + fmt.Sprintf("已发现 %d 个技能，名称目录超过预算，未在此逐项展示。完整名称、描述和路径索引：%s。先用 read 分页读取或 grep 检索元数据，再按 path 读取匹配技能的 SKILL.md。索引不含技能正文。\n", len(catalog.Skills), path)
				state.Mode = "index"
			}
		}
	}
	state.ListingBytes = skillListingBytes(listing)
	if state.ListingBytes > state.BudgetBytes {
		return "", state, fmt.Errorf("skill catalog index reference exceeds configured budget")
	}
	return listing, state, nil
}

func writeSkillIndex(path, content string) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".skill-index-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.WriteString(content); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		// A concurrent process may already have published the same revision.
		if b, readErr := os.ReadFile(path); readErr == nil && string(b) == content {
			return nil
		}
		return err
	}
	return nil
}

// Refresh once per user turn, shared by CLI and HTTP. This is a request
// projection: persisted historical messages/tool results are not rewritten.
// Resume reconstructs the current directory again at this same boundary.
func refreshSkillCatalog(cfg *config, msgs *[]openai.ChatCompletionMessage) error {
	catalog := discoverSkills(cfg.workspace)
	listing, state, err := buildSkillListing(cfg, catalog)
	if err != nil {
		return err
	}
	updated := make([]openai.ChatCompletionMessage, 0, len(*msgs)+1)
	for _, m := range *msgs {
		if m.Role == "system" {
			if strings.HasPrefix(m.Content, skillCatalogMarker) {
				continue
			}
			// Older releases appended this generated suffix to the base system.
			// Only remove its exact list shape, not a heading in arbitrary prose.
			if at := strings.LastIndex(m.Content, "\n\n"+legacySkillHeading); at >= 0 {
				tail := m.Content[at+2+len(legacySkillHeading):]
				isList := tail != ""
				for _, line := range strings.Split(tail, "\n") {
					if !strings.HasPrefix(line, "- ") {
						isList = false
						break
					}
				}
				if isList {
					m.Content = m.Content[:at]
				}
			}
		}
		updated = append(updated, m)
	}
	if listing != "" {
		// Keep the catalog next to the base system, not after the active request.
		at := 0
		for at < len(updated) && updated[at].Role == "system" {
			at++
		}
		updated = append(updated, openai.ChatCompletionMessage{})
		copy(updated[at+1:], updated[at:])
		updated[at] = openai.ChatCompletionMessage{Role: "system", Content: listing}
	}
	*msgs = updated
	printSkillWarnings(catalog)
	emitRuntimeEvent("skill_catalog", state)
	return nil
}

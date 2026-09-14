package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

const defaultMicroKeepRecent = 8
const microMarker = "[pulse7 历史工具结果正文已从当前上下文省略]"
const compactIndexMarker = "[pulse7 历史工具结果恢复索引]"

// Stable content-addressed IDs reuse attachments across resume and repeated compaction.
func compactAttachment(value string) (string, error) {
	if sess == nil {
		return "", fmt.Errorf("no active session for compaction attachment")
	}
	sum := sha256.Sum256([]byte("microcompact\x00" + value))
	h := fmt.Sprintf("%x", sum[:16])
	uuid := h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
	key, err := largeSessionKey(sess.path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(value))
	ref := fmt.Sprintf("lc1.%s.%s.content.%x", key, uuid, hash)
	path := largeContentPath(sess.path, uuid, "content")
	if _, err := os.Stat(path); err == nil {
		_, _, err = verifyLargeContent(sess.path, ref)
		return ref, err
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return writeLargeContent(sess.path, uuid, "content", value)
}

func microCompact(original []openai.ChatCompletionMessage, keep int) ([]openai.ChatCompletionMessage, int) {
	projected := append([]openai.ChatCompletionMessage(nil), original...)
	if keep <= 0 {
		keep = defaultMicroKeepRecent
	}
	allowed := map[string]bool{"read": true, "ls": true, "tree": true, "grep": true, "glob": true, "shell": true, "task_output": true}
	calls := map[string]openai.ToolCall{}
	groups := map[string]int{}
	eligible := []int{}
	latest := -1
	for i, m := range original {
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			latest = i
			for _, c := range m.ToolCalls {
				calls[c.ID] = c
				groups[c.ID] = i
			}
		}
		if m.Role == "tool" && allowed[calls[m.ToolCallID].Function.Name] {
			eligible = append(eligible, i)
		}
	}
	protected := map[int]bool{}
	for i := len(eligible) - 1; i >= 0 && i >= len(eligible)-keep; i-- {
		protected[eligible[i]] = true
	}
	changed := 0
	wanted := map[string]bool{}
	for _, i := range eligible {
		m := original[i]
		if !protected[i] && groups[m.ToolCallID] != latest && !strings.HasPrefix(m.Content, microMarker) && len(m.Content) >= 1024 {
			wanted[m.ToolCallID] = true
		}
	}
	records, readErr := microRecords(wanted)
	if readErr != nil {
		out("[micro 跳过：历史结果索引不可读]\n")
		return projected, 0
	}
	for _, i := range eligible {
		m := original[i]
		c := calls[m.ToolCallID]
		if protected[i] || groups[m.ToolCallID] == latest || strings.HasPrefix(m.Content, microMarker) || len(m.Content) < 1024 {
			continue
		}
		// The persisted record is authoritative for outcome and existing raw-content refs.
		if sess == nil {
			continue
		}
		record, found := records[m.ToolCallID]
		if !found {
			out("[micro 跳过：无法读取历史结果 id=%s]\n", m.ToolCallID)
			continue
		}
		ref := record.LargeContent["content"]
		var err error
		if ref == "" {
			ref, err = compactAttachment(record.Content)
		} else {
			_, _, err = verifyLargeContent(sess.path, ref)
		}
		if err != nil {
			out("[micro 跳过：原文保存或校验失败 id=%s]\n", m.ToolCallID)
			continue
		}
		var args map[string]interface{}
		json.Unmarshal([]byte(c.Function.Arguments), &args)
		location := map[string]interface{}{}
		for _, key := range []string{"path", "content_ref", "offset", "limit", "byte_offset", "byte_limit", "pattern"} {
			if value, ok := args[key]; ok {
				encoded, _ := json.Marshal(value)
				if len(encoded) <= 512 {
					location[key] = value
				}
			}
		}
		facts, _ := json.Marshal(map[string]interface{}{"tool": c.Function.Name, "tool_call_id": m.ToolCallID, "arguments": location, "toolOutcome": record.ToolOutcome, "content_ref": ref})
		placeholder := microMarker + "\n" + string(facts) + "\n需要核实历史输出时可用 read(content_ref) 分页读取；检查文件当前内容时可按路径分页读取。"
		if len(placeholder) >= len(m.Content) {
			continue
		}
		projected[i].Content = placeholder
		changed++
	}
	return projected, changed
}

// Reuse the existing history index; one pass per compaction, not one scan per tool.
func microRecords(wanted map[string]bool) (map[string]sessionMessageRecord, error) {
	rows := map[string]sessionMessageRecord{}
	if len(wanted) == 0 || sess == nil {
		return rows, nil
	}
	idx, err := indexAPIHistory(sess.path)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(sess.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	for _, span := range idx.spans {
		raw, err := apiReadSpan(f, span)
		if err != nil {
			return nil, err
		}
		var row sessionMessageRecord
		if err = json.Unmarshal(raw, &row); err != nil {
			return nil, err
		}
		if row.Role == "tool" && wanted[row.ToolCallID] {
			rows[row.ToolCallID] = row
		}
	}
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !unchangedFile(idx.info, info) {
		return nil, fmt.Errorf("session changed during micro compaction")
	}
	return rows, nil
}

// Keep a bounded, locally generated recovery pointer even when summary/truncation drops pairs.
func compactRecoveryIndex(messages []openai.ChatCompletionMessage) (string, error) {
	lines := []string{}
	wanted := map[string]bool{}
	calls := map[string]openai.ToolCall{}
	for _, m := range messages {
		for _, c := range m.ToolCalls {
			calls[c.ID] = c
		}
		if m.Role == "tool" && !strings.HasPrefix(m.Content, microMarker) {
			wanted[m.ToolCallID] = true
		}
	}
	records, err := microRecords(wanted)
	if err != nil {
		return "", err
	}
	for _, m := range messages {
		if strings.HasPrefix(m.Content, microMarker) || strings.HasPrefix(m.Content, compactIndexMarker) {
			lines = append(lines, m.Content)
		} else if m.Role == "tool" {
			record, ok := records[m.ToolCallID]
			if !ok {
				continue
			}
			ref := record.LargeContent["content"]
			if ref == "" {
				ref, err = compactAttachment(record.Content)
			} else {
				_, _, err = verifyLargeContent(sess.path, ref)
			}
			if err != nil {
				return "", err
			}
			c := calls[m.ToolCallID]
			var args map[string]interface{}
			json.Unmarshal([]byte(c.Function.Arguments), &args)
			facts := map[string]interface{}{"tool": c.Function.Name, "tool_call_id": m.ToolCallID, "content_ref": ref, "toolOutcome": record.ToolOutcome}
			for _, key := range []string{"path", "content_ref", "offset", "limit", "byte_offset", "byte_limit"} {
				if v, ok := args[key]; ok {
					encoded, _ := json.Marshal(v)
					if len(encoded) <= 512 {
						facts["argument_"+key] = v
					}
				}
			}
			encoded, _ := json.Marshal(facts)
			lines = append(lines, string(encoded))
		}
	}
	if len(lines) == 0 {
		return "", nil
	}
	ref, err := compactAttachment(strings.Join(lines, "\n\n"))
	if err != nil {
		return "", err
	}
	return compactIndexMarker + "\n历史路径、调用状态和原文引用按需读取：read(content_ref=\"" + ref + "\")。这是历史调用记录，不表示文件现状或任务完成。", nil
}

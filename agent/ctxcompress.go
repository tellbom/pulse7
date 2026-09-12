package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	openai "github.com/sashabaranov/go-openai"
)

// T3 (slow-network): the summarize call gets its own timeout budget derived
// from the interrupt context - it no longer shares a deadline with the main
// conversation loop. Any failure still falls back to plain truncation;
// compression must never kill a task.
func compressCtx(parent context.Context, cfg *config) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, cfg.llmCompressTimeout)
}

// M4-T4 context compression: instead of losing messages to a hard truncate
// at the limit, summarize the OLDER conversation via one extra LLM call once
// context exceeds 65% of the budget. System prompt (incl. AGENT.md) and the
// last keepRecentRounds tool rounds stay verbatim.
const (
	compressThreshold    = 0.65
	keepRecentRounds     = 3
	codingTemperature    = 0.2
	compressionSummary   = "summary"
	compressionTruncate  = "truncate"
	toolResultHead       = 2048
	toolResultMinimum    = 512
	toolArgumentsHead    = 200
	toolArgumentsMinimum = 64
)

var errLocalContextBudget = errors.New("上下文保留内容超过预算")

func contextChars(msgs []openai.ChatCompletionMessage) int {
	b, _ := json.Marshal(msgs)
	return len(b)
}

func requestContextChars(msgs []openai.ChatCompletionMessage, tools []openai.Tool) int {
	b, _ := json.Marshal(tools)
	return contextChars(msgs) + len(b)
}

func maybeCompressContext(ctx context.Context, client *openai.Client, cfg *config,
	tools []openai.Tool, msgs *[]openai.ChatCompletionMessage) error {
	return compressContext(ctx, client, cfg, tools, msgs, false)
}

func emergencyCompressContext(ctx context.Context, client *openai.Client, cfg *config,
	tools []openai.Tool, msgs *[]openai.ChatCompletionMessage) error {
	before := requestContextChars(*msgs, tools)
	out("[紧急上下文压缩] 当前上下文超限，开始压缩后单次重试\n")
	if err := compressContext(ctx, client, cfg, tools, msgs, true); err != nil {
		if errors.Is(err, errLocalContextBudget) && requestContextChars(*msgs, tools) >= before {
			return fmt.Errorf("紧急上下文压缩未能缩小请求：%w", err)
		}
		return err
	}
	after := requestContextChars(*msgs, tools)
	if after >= before {
		return fmt.Errorf("紧急上下文压缩未能缩小请求（压缩前 %d 字符，压缩后 %d 字符）", before, after)
	}
	out("[紧急上下文压缩] 完成：%d → %d 字符\n", before, after)
	return nil
}

func compressContext(ctx context.Context, client *openai.Client, cfg *config,
	tools []openai.Tool, msgs *[]openai.ChatCompletionMessage, emergency bool) error {
	requestChars := requestContextChars(*msgs, tools)
	threshold := int(float64(cfg.maxCtx) * compressThreshold)
	if cfg.maxCtx <= 0 || (!emergency && requestChars <= threshold) {
		return nil
	}
	if emergency && requestChars <= threshold {
		threshold = int(float64(requestChars) * compressThreshold)
	}
	toolBytes, _ := json.Marshal(tools)
	truncateTo := threshold - len(toolBytes)
	n := len(*msgs)
	rounds, cut := 0, n
	for i := n - 1; i >= 0; i-- {
		if (*msgs)[i].Role == openai.ChatMessageRoleAssistant && len((*msgs)[i].ToolCalls) > 0 {
			rounds++
			if rounds >= keepRecentRounds {
				cut = i
				break
			}
		}
	}
	if rounds < keepRecentRounds {
		return truncateAndAudit(msgs, truncateTo, tools, cfg, emergency)
	}
	head := 0 // never compress leading system messages (AGENT.md lives here)
	for head < cut && (*msgs)[head].Role == openai.ChatMessageRoleSystem {
		head++
	}
	if head >= cut-1 {
		return truncateAndAudit(msgs, truncateTo, tools, cfg, emergency)
	}
	var b strings.Builder
	b.WriteString("请把以下较早的对话历史压缩为一段摘要。保留：已确认的事实、已完成的修改、当前目标；丢弃：完整文件内容与中间探索过程。直接输出摘要正文：\n")
	for _, m := range (*msgs)[head:cut] {
		encoded, _ := json.Marshal(m)
		fmt.Fprintf(&b, "%s\n", encoded)
	}
	cctx, cancel := compressCtx(ctx, cfg)
	defer cancel()
	resp, err := client.CreateChatCompletion(cctx, openai.ChatCompletionRequest{
		Model:       cfg.model,
		Temperature: codingTemperature,
		Messages: []openai.ChatCompletionMessage{{
			Role: openai.ChatMessageRoleUser, Content: b.String(),
		}},
	})
	if err != nil || len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		out("[上下文压缩失败，回退到截断：%v]\n", err)
		return truncateAndAudit(msgs, truncateTo, tools, cfg, emergency)
	}
	rebuilt := append([]openai.ChatCompletionMessage{}, (*msgs)[:head]...)
	rebuilt = append(rebuilt, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "[较早对话的摘要]\n" + resp.Choices[0].Message.Content,
	})
	// Keep the active user request and every system message verbatim.
	latestUser := -1
	for i := range *msgs {
		if (*msgs)[i].Role == openai.ChatMessageRoleUser {
			latestUser = i
		}
	}
	for i := head; i < cut; i++ {
		if i == latestUser || (*msgs)[i].Role == openai.ChatMessageRoleSystem {
			rebuilt = append(rebuilt, (*msgs)[i])
		}
	}
	rebuilt = append(rebuilt, (*msgs)[cut:]...)
	*msgs = rebuilt
	after := requestContextChars(*msgs, tools)
	emitCompaction(requestChars, after, emergency, compressionSummary, cut-head, resp.Choices[0].Message.Content)
	// T4 (PreRC02): compression must be auditable — write an audit record so
	// post-hoc analysis can see it happened and how much it saved.
	if err := auditCompression(cut-head, after, cfg, emergency, compressionSummary); err != nil {
		return err
	}
	if (!emergency && after > threshold) || after > cfg.maxCtx {
		return truncateAndAudit(msgs, truncateTo, tools, cfg, emergency)
	}
	return nil
}

type messageRange struct {
	start int
	end   int
}

// truncateContext removes complete assistant-tool/result groups. Leading
// system messages and the latest user request are retained, so cropping can
// never leave an orphan tool result or discard the active task statement.
func truncateContext(msgs *[]openai.ChatCompletionMessage, maxChars int) int {
	removed := 0
	for contextChars(*msgs) > maxChars {
		candidate, ok := oldestRemovableMessageRange(*msgs)
		if !ok {
			shrinkLatestToolGroup(msgs, maxChars)
			return removed
		}
		removed += candidate.end - candidate.start
		*msgs = append((*msgs)[:candidate.start], (*msgs)[candidate.end:]...)
	}
	return removed
}

func oldestRemovableMessageRange(msgs []openai.ChatCompletionMessage) (messageRange, bool) {
	latestUser := -1
	latestTool := -1
	for i := range msgs {
		if msgs[i].Role == openai.ChatMessageRoleAssistant && len(msgs[i].ToolCalls) > 0 {
			latestTool = i
		}
		if msgs[i].Role == openai.ChatMessageRoleUser {
			latestUser = i
		}
	}
	for i := 0; i < len(msgs); {
		end := i + 1
		if msgs[i].Role == openai.ChatMessageRoleAssistant && len(msgs[i].ToolCalls) > 0 {
			for end < len(msgs) && msgs[end].Role == openai.ChatMessageRoleTool {
				end++
			}
		}
		if i != latestTool && msgs[i].Role != openai.ChatMessageRoleSystem && !(i <= latestUser && latestUser < end) {
			return messageRange{start: i, end: end}, true
		}
		i = end
	}
	return messageRange{}, false
}

func auditCompression(compressed, afterChars int, cfg *config, emergency bool, method string) error {
	if curCfg == nil {
		return nil
	}
	dir := filepath.Join(cfg.exeDirStore(), "data", "sessions")
	return appendJSONLine(filepath.Join(dir, "audit.jsonl"), map[string]interface{}{
		"ts": time.Now().Format(time.RFC3339), "tool": "_compress",
		"messages_compressed": compressed,
		"context_chars_after": afterChars,
		"est_tokens_after":    afterChars / 4,
		"emergency":           emergency,
		"method":              method,
	})
}

// F1 retains at least the latest tool group; there is no zero-retention slice.
func shrinkLatestToolGroup(msgs *[]openai.ChatCompletionMessage, budget int) {
	latest := -1
	for i, m := range *msgs {
		if m.Role == openai.ChatMessageRoleAssistant && len(m.ToolCalls) > 0 {
			latest = i
		}
	}
	if latest < 0 {
		return
	}
	end := latest + 1
	for end < len(*msgs) && (*msgs)[end].Role == openai.ChatMessageRoleTool {
		end++
	}
	originals := append([]openai.ChatCompletionMessage(nil), (*msgs)[latest:end]...)
	calls := append([]openai.ToolCall(nil), originals[0].ToolCalls...)
	(*msgs)[latest].ToolCalls = append([]openai.ToolCall(nil), calls...)
	for _, limit := range []int{toolResultHead, toolResultMinimum} {
		for i := 1; i < len(originals); i++ {
			(*msgs)[latest+i].Content = truncatedHeader(originals[i].Content, limit)
		}
		if contextChars(*msgs) <= budget {
			return
		}
	}
	for _, limit := range []int{toolArgumentsHead, toolArgumentsMinimum} {
		for i, c := range calls {
			args := c.Function.Arguments
			var prior map[string]string
			if json.Unmarshal([]byte(args), &prior) == nil && len(prior) == 1 && prior["truncated_arguments"] != "" {
				args = prior["truncated_arguments"]
			}
			if len(args) > limit {
				encoded, _ := json.Marshal(map[string]string{"truncated_arguments": truncatedHeader(args, limit)})
				(*msgs)[latest].ToolCalls[i].Function.Arguments = string(encoded)
			}
		}
		if contextChars(*msgs) <= budget {
			return
		}
	}
}

func truncatedHeader(s string, limit int) string {
	original := len(s)
	marker := "\n[pulse7 truncated original_bytes="
	if i := strings.LastIndex(s, marker); i >= 0 {
		var total, dropped int
		if n, err := fmt.Sscanf(s[i:], "\n[pulse7 truncated original_bytes=%d discarded_bytes=%d]", &total, &dropped); err == nil && n == 2 && total-dropped == i {
			original = total
			s = s[:i]
		}
	}
	n := len(s)
	if n > limit {
		n = limit
		for n > 0 && !utf8.RuneStart(s[n]) {
			n--
		}
	}
	if n == original {
		return s
	}
	return fmt.Sprintf("%s\n[pulse7 truncated original_bytes=%d discarded_bytes=%d]", s[:n], original, original-n)
}

func truncationGroups(msgs []openai.ChatCompletionMessage) []string {
	groups := []string{}
	for _, m := range msgs {
		for _, c := range m.ToolCalls {
			groups = append(groups, c.ID)
		}
	}
	return groups
}

func truncateAndAudit(msgs *[]openai.ChatCompletionMessage, budget int, tools []openai.Tool, cfg *config, emergency bool) error {
	before := requestContextChars(*msgs, tools)
	groupsBefore := truncationGroups(*msgs)
	removed := truncateContext(msgs, budget)
	after := requestContextChars(*msgs, tools)
	kept := truncationGroups(*msgs)
	discarded := []string{}
	for _, id := range groupsBefore {
		found := false
		for _, k := range kept {
			if k == id {
				found = true
			}
		}
		if !found {
			discarded = append(discarded, id)
		}
	}
	if before != after {
		detail := fmt.Sprintf("original_bytes=%d discarded_bytes=%d kept_tool_call_ids=%v discarded_tool_call_ids=%v", before, before-after, kept, discarded)
		out("[上下文截断明细] %s\n", detail)
		reason := "threshold"
		if emergency {
			reason = "context_length_exceeded"
		}
		dropped := before - after
		emitRuntimeEvent("compaction", compactionEvent{Reason: reason, BeforeTokens: before / 4, AfterTokens: after / 4, Emergency: emergency, Summary: detail, Method: compressionTruncate, Removed: removed, OriginalBytes: &before, DiscardedBytes: &dropped, KeptToolCallIDs: &kept, DiscardedToolCallIDs: &discarded})
		if err := auditCompression(removed, after, cfg, emergency, compressionTruncate); err != nil {
			return err
		}
		if curCfg != nil {
			if err := appendJSONLine(filepath.Join(cfg.exeDirStore(), "data", "sessions", "audit.jsonl"), map[string]interface{}{"tool": "_truncate_detail", "original_bytes": before, "discarded_bytes": before - after, "kept_tool_call_ids": kept, "discarded_tool_call_ids": discarded}); err != nil {
				return err
			}
		}
	}
	if contextChars(*msgs) > budget {
		return fmt.Errorf("%w: retained_bytes=%d budget_bytes=%d", errLocalContextBudget, contextChars(*msgs), budget)
	}
	return nil
}

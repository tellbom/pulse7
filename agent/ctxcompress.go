package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// M4-T4 context compression: instead of losing messages to a hard truncate
// at the limit, summarize the OLDER conversation via one extra LLM call once
// context exceeds 75% of the budget. System prompt (incl. AGENT.md) and the
// last keepRecentRounds tool rounds stay verbatim. Any compression failure
// falls back to the original truncation - compression must never kill a task.
const (
	compressThreshold  = 0.75
	keepRecentRounds   = 3
	summarizeChunkCap  = 500 // per-message chars fed into the summarize prompt
)

func contextChars(msgs []openai.ChatCompletionMessage) int {
	total := 0
	for _, m := range msgs {
		total += len(m.Content)
	}
	return total
}

func maybeCompressContext(ctx context.Context, client *openai.Client, cfg *config,
	msgs *[]openai.ChatCompletionMessage) {
	if cfg.maxCtx <= 0 || contextChars(*msgs) <= int(float64(cfg.maxCtx)*compressThreshold) {
		return
	}
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
		return // not enough history worth compressing
	}
	head := 0 // never compress leading system messages (AGENT.md lives here)
	for head < cut && (*msgs)[head].Role == openai.ChatMessageRoleSystem {
		head++
	}
	if head >= cut-1 {
		return
	}
	var b strings.Builder
	b.WriteString("请把以下较早的对话历史压缩为一段摘要。保留：已确认的事实、已完成的修改、当前目标；丢弃：完整文件内容与中间探索过程。直接输出摘要正文：\n")
	for _, m := range (*msgs)[head:cut] {
		c := m.Content
		if len(c) > summarizeChunkCap {
			c = c[:summarizeChunkCap] + "…"
		}
		fmt.Fprintf(&b, "[%s] %s\n", m.Role, c)
	}
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: cfg.model,
		Messages: []openai.ChatCompletionMessage{{
			Role: openai.ChatMessageRoleUser, Content: b.String(),
		}},
	})
	if err != nil || len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		fmt.Printf("[上下文压缩失败，回退到截断：%v]\n", err)
		truncateContext(msgs, cfg.maxCtx)
		return
	}
	rebuilt := append([]openai.ChatCompletionMessage{}, (*msgs)[:head]...)
	rebuilt = append(rebuilt, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "[较早对话的摘要]\n" + resp.Choices[0].Message.Content,
	})
	rebuilt = append(rebuilt, (*msgs)[cut:]...)
	fmt.Printf("[上下文已压缩：%d 条消息 → 摘要]\n", cut-head)
	*msgs = rebuilt
	// T4 (PreRC02): compression must be auditable — write an audit record so
	// post-hoc analysis can see it happened and how much it saved.
	auditCompression(cut-head, contextChars(*msgs), cfg)
}

func auditCompression(compressed, afterChars int, cfg *config) {
	if curCfg == nil {
		return
	}
	dir := filepath.Join(cfg.exeDirStore(), "data", "sessions")
	f, err := os.OpenFile(filepath.Join(dir, "audit.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	b, _ := json.Marshal(map[string]interface{}{
		"ts": time.Now().Format(time.RFC3339), "tool": "_compress",
		"messages_compressed": compressed,
		"context_chars_after": afterChars,
		"est_tokens_after":    afterChars / 4,
	})
	f.Write(append(b, '\n'))
}

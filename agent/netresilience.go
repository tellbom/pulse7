package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// Slow-network resilience (T1): watchdog sentinels. The first-chunk timeout
// guards the queueing phase (request sent -> first data block) and IS
// retryable; the idle timeout guards an established stream that stopped
// producing and is NOT retryable (a retry would hit the same stall).
var (
	errFirstChunkTimeout = errors.New("首字节超时")
	errIdleTimeout       = errors.New("流空闲超时")
)

// streamSink accumulates ONE streaming attempt: displayed text plus tool
// calls reconstructed from deltas. A retried attempt builds a fresh sink so
// half-received fragments are discarded, never concatenated.
type streamSink struct {
	content strings.Builder
	toolAcc map[int]*openai.ToolCall
	order   []int
}

func newStreamSink() *streamSink {
	return &streamSink{toolAcc: map[int]*openai.ToolCall{}}
}

func (s *streamSink) onChunk(chunk openai.ChatCompletionStreamResponse) {
	if len(chunk.Choices) == 0 {
		return
	}
	d := chunk.Choices[0].Delta
	if d.Content != "" {
		outPrint(d.Content)
		s.content.WriteString(d.Content)
	}
	for _, tc := range d.ToolCalls {
		idx := 0
		if tc.Index != nil {
			idx = *tc.Index
		}
		e, ok := s.toolAcc[idx]
		if !ok {
			e = &openai.ToolCall{ID: tc.ID, Type: openai.ToolTypeFunction, Function: openai.FunctionCall{}}
			s.toolAcc[idx] = e
			s.order = append(s.order, idx)
		}
		if tc.ID != "" {
			e.ID = tc.ID
		}
		if tc.Function.Name != "" {
			e.Function.Name = tc.Function.Name
		}
		e.Function.Arguments += tc.Function.Arguments
	}
}

func (s *streamSink) calls() []openai.ToolCall {
	out := make([]openai.ToolCall, 0, len(s.order))
	for _, i := range s.order {
		c := *s.toolAcc[i]
		c.Index = nil
		out = append(out, c)
	}
	return out
}

// resetDeadline safely re-arms t after a received chunk: drain a value that
// fired between Stop and Reset so it cannot leak into the next select.
func resetDeadline(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
}

// llmStreamOnce performs one streaming chat request under the two watchdogs:
//   - first-chunk timeout: from request start until the first block arrives
//     (queueing / long prompt processing);
//   - idle timeout: maximum gap between consecutive blocks, reset by EVERY
//     received chunk (SSE keepalives with empty deltas count as data).
//
// A stream that keeps producing is never killed for being slow. ctx carries
// only the interrupt cancellation — there is deliberately no total budget.
func llmStreamOnce(ctx context.Context, client *openai.Client, cfg *config,
	req openai.ChatCompletionRequest, sink *streamSink) error {
	stream, err := client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return err
	}
	defer stream.Close()

	type recvRes struct {
		resp openai.ChatCompletionStreamResponse
		err  error
	}
	ch := make(chan recvRes, 1)
	go func() {
		for {
			r, err := stream.Recv()
			ch <- recvRes{r, err}
			if err != nil {
				return
			}
		}
	}()

	gotFirst := false
	deadline := time.NewTimer(cfg.llmFirstChunkTimeout)
	defer deadline.Stop()
	for {
		select {
		case r := <-ch:
			if errors.Is(r.err, io.EOF) {
				return nil
			}
			if r.err != nil {
				return r.err
			}
			gotFirst = true
			resetDeadline(deadline, cfg.llmIdleTimeout)
			sink.onChunk(r.resp)
		case <-deadline.C:
			if !gotFirst {
				return fmt.Errorf("模型 %v 内没有返回任何数据（首字节超时，可配置 llm_first_chunk_timeout_sec）: %w",
					cfg.llmFirstChunkTimeout, errFirstChunkTimeout)
			}
			return fmt.Errorf("模型流 %v 没有新数据，判定卡死（空闲超时，可配置 llm_idle_timeout_sec）: %w",
				cfg.llmIdleTimeout, errIdleTimeout)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

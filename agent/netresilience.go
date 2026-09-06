package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
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

// llmRetryDelays: backoff between retry attempts (5s then 15s; later
// attempts reuse the last value).
var llmRetryDelays = []time.Duration{5 * time.Second, 15 * time.Second}

func llmRetryDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return llmRetryDelays[0]
	}
	if attempt >= len(llmRetryDelays) {
		return llmRetryDelays[len(llmRetryDelays)-1]
	}
	return llmRetryDelays[attempt]
}

// retryableLLMError decides whether a failed request is worth re-sending.
// Retryable: connection-level failures (refused/reset/TLS handshake
// timeout), HTTP 5xx / 429, first-chunk timeout (queueing).
// NOT retryable: 4xx parameter/auth errors, balance-exhausted (bigmodel
// returns HTTP 429 + code 1113), idle timeout (an established stream
// stalled - a retry hits the same stall), cancellation.
func retryableLLMError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errIdleTimeout) || errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, errFirstChunkTimeout) {
		return true
	}
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		s := err.Error()
		if strings.Contains(s, "1113") || strings.Contains(s, "余额不足") {
			return false
		}
		return apiErr.HTTPStatusCode == 429 || apiErr.HTTPStatusCode >= 500
	}
	var reqErr *openai.RequestError // non-JSON error bodies land here
	if errors.As(err, &reqErr) {
		return reqErr.HTTPStatusCode == 429 || reqErr.HTTPStatusCode >= 500
	}
	var netErr net.Error // covers *url.Error, dial/temp errors, TLS handshake timeout
	if errors.As(err, &netErr) {
		return true
	}
	var urlErr *url.Error
	return errors.As(err, &urlErr)
}

// roundStream: one LLM round with retry. A retried attempt starts a fresh
// sink, so half-received fragments from the failed attempt are discarded -
// the whole request is re-sent, never stitched together.
func roundStream(ctx context.Context, client *openai.Client, cfg *config,
	req openai.ChatCompletionRequest) (string, []openai.ToolCall, error) {
	var lastErr error
	for attempt := 0; ; attempt++ {
		if attempt > 0 {
			delay := llmRetryDelay(attempt - 1)
			fmt.Printf("[重试] LLM 请求失败（%v），%v 后第 %d 次重试...\n", lastErr, delay, attempt)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return "", nil, ctx.Err()
			}
		}
		sink := newStreamSink()
		err := llmStreamOnce(ctx, client, cfg, req, sink)
		if err == nil {
			return sink.content.String(), sink.calls(), nil
		}
		lastErr = err
		if ctx.Err() != nil || interrupted() {
			return "", nil, err
		}
		if !retryableLLMError(err) || attempt >= cfg.llmMaxRetries {
			if attempt >= cfg.llmMaxRetries && retryableLLMError(err) {
				return "", nil, fmt.Errorf("重试 %d 次后仍失败: %w", cfg.llmMaxRetries, lastErr)
			}
			return "", nil, err
		}
	}
}

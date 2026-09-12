package main

import (
	"context"
	"encoding/json"
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
	errFirstChunkTimeout   = errors.New("首字节超时")
	errIdleTimeout         = errors.New("流空闲超时")
	errUnexpectedStreamEOF = errors.New("unexpected EOF before stream termination")
	errStreamLength        = errors.New("stream terminated with finish_reason=length")
)

// streamSink accumulates ONE streaming attempt: displayed text plus tool
// calls reconstructed from deltas. A retried attempt builds a fresh sink so
// half-received fragments are discarded, never concatenated.
// Narrative text streams through a linePrefixer (T2 output-layering): the
// model's running commentary is visually distinct from tool lines and from
// the framed final answer.
type streamSink struct {
	reasoning    strings.Builder
	content      strings.Builder
	attempt      uint64
	toolAcc      map[int]*openai.ToolCall
	order        []int
	finishReason openai.FinishReason
	terminalErr  error
}

func newStreamSink() *streamSink {
	return &streamSink{toolAcc: map[int]*openai.ToolCall{}}
}

func (s *streamSink) onChunk(chunk openai.ChatCompletionStreamResponse) {
	if len(chunk.Choices) == 0 {
		return
	}
	choice := chunk.Choices[0]
	d := choice.Delta
	if s.finishReason != "" && (d.Content != "" || d.ReasoningContent != "" || len(d.ToolCalls) > 0) {
		s.terminalErr = errors.New("stream delivered data after terminal finish_reason")
	}
	if d.Content != "" {
		emitRuntimeEvent("assistant_delta", assistantDeltaEvent{Delta: d.Content, Attempt: s.attempt})
		s.content.WriteString(d.Content)
	}
	if d.ReasoningContent != "" {
		emitRuntimeEvent("assistant_reasoning_delta", assistantReasoningDeltaEvent{Delta: d.ReasoningContent, Attempt: s.attempt})
		s.reasoning.WriteString(d.ReasoningContent)
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
	if choice.FinishReason != "" && choice.FinishReason != openai.FinishReasonNull {
		if s.finishReason != "" && s.finishReason != choice.FinishReason {
			s.terminalErr = fmt.Errorf("stream changed finish_reason from %s to %s", s.finishReason, choice.FinishReason)
		}
		s.finishReason = choice.FinishReason
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

func (s *streamSink) terminalResult() (bool, error) {
	if s.finishReason == "" {
		return false, nil
	}
	if s.terminalErr != nil {
		return true, s.terminalErr
	}
	switch s.finishReason {
	case openai.FinishReasonStop:
		if len(s.toolAcc) != 0 {
			return true, errors.New("finish_reason=stop received with an unfinished tool call")
		}
		return true, nil
	case openai.FinishReasonToolCalls:
		calls := s.calls()
		if len(calls) == 0 {
			return true, errors.New("finish_reason=tool_calls received without a tool call")
		}
		for _, call := range calls {
			if call.ID == "" || call.Function.Name == "" || !json.Valid([]byte(call.Function.Arguments)) {
				return true, fmt.Errorf("incomplete tool call at stream termination: id=%q name=%q arguments=%q",
					call.ID, call.Function.Name, call.Function.Arguments)
			}
		}
		return true, nil
	case openai.FinishReasonLength:
		return true, errStreamLength
	default:
		return true, fmt.Errorf("stream terminated with unsupported finish_reason=%s", s.finishReason)
	}
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
//   - first-chunk timeout: from request start until the first DATA chunk
//     arrives - this covers header-wait (queueing at the server or gateway)
//     and prompt processing, not just the body stream;
//   - idle timeout: maximum gap between consecutive chunks, reset by EVERY
//     received chunk (SSE keepalives with empty deltas count as data).
//
// A stream that keeps producing is never killed for being slow. ctx carries
// only the interrupt cancellation — there is deliberately no total budget.
// While waiting for the first chunk a heartbeat line is appended every 15s
// (T4): Win7 conhost parses no ANSI/VT, so output is line-append only.
func llmStreamOnce(ctx context.Context, client *openai.Client, cfg *config,
	req openai.ChatCompletionRequest, sink *streamSink) error {
	type openRes struct {
		stream *openai.ChatCompletionStream
		err    error
	}
	type recvRes struct {
		resp openai.ChatCompletionStreamResponse
		err  error
	}
	openCh := make(chan openRes, 1)
	go func() {
		s, err := client.CreateChatCompletionStream(ctx, req)
		openCh <- openRes{s, err}
	}()

	var stream *openai.ChatCompletionStream
	defer func() {
		if stream != nil {
			stream.Close()
		}
	}()

	gotFirst := false
	started := time.Now()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	deadline := time.NewTimer(cfg.llmFirstChunkTimeout)
	defer deadline.Stop()
	var recvCh <-chan recvRes
	for {
		select {
		case o := <-openCh:
			openCh = nil // phase done; never select on a closed phase again
			if o.err != nil {
				return o.err
			}
			stream = o.stream
			c := make(chan recvRes, 1)
			go func() {
				for {
					r, err := stream.Recv()
					c <- recvRes{r, err}
					if err != nil {
						return
					}
				}
			}()
			recvCh = c
		case r := <-recvCh:
			if errors.Is(r.err, io.EOF) {
				flushAssistantEvents()
				return errUnexpectedStreamEOF
			}
			if r.err != nil {
				return r.err
			}
			if !gotFirst {
				// T4: once content flows the screen itself shows life.
				gotFirst = true
				heartbeat.Stop()
			}
			resetDeadline(deadline, cfg.llmIdleTimeout)
			sink.onChunk(r.resp)
			if terminal, terminalErr := sink.terminalResult(); terminal {
				flushAssistantEvents()
				return terminalErr
			}
		case <-heartbeat.C:
			emitRuntimeEvent("heartbeat", heartbeatEvent{WaitedSeconds: int(time.Since(started).Round(time.Second).Seconds())})
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
	if errors.Is(err, errUnexpectedStreamEOF) {
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

func contextLengthExceededError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		fields := strings.ToLower(fmt.Sprint(apiErr.Code) + " " + apiErr.Type + " " + apiErr.Message)
		return strings.Contains(fields, "context_length_exceeded") ||
			strings.Contains(fields, "maximum context length") ||
			strings.Contains(fields, "context window")
	}
	var reqErr *openai.RequestError
	if errors.As(err, &reqErr) {
		body := strings.ToLower(string(reqErr.Body))
		return strings.Contains(body, "context_length_exceeded") ||
			strings.Contains(body, "maximum context length") ||
			strings.Contains(body, "context window")
	}
	return false
}

// roundStream: one LLM round with retry. A retried attempt starts a fresh
// sink, so half-received fragments from the failed attempt are discarded -
// the whole request is re-sent, never stitched together.
func roundStream(ctx context.Context, client *openai.Client, cfg *config, req openai.ChatCompletionRequest) (string, []openai.ToolCall, error) {
	message, err := roundStreamMessage(ctx, client, cfg, req)
	return message.Content, message.ToolCalls, err
}
func roundStreamMessage(ctx context.Context, client *openai.Client, cfg *config,
	req openai.ChatCompletionRequest) (openai.ChatCompletionMessage, error) {
	var lastErr error
	for attempt := 0; ; attempt++ {
		if attempt > 0 {
			delay := llmRetryDelay(attempt - 1)
			out("[重试] LLM 请求失败（%v），%v 后第 %d 次重试...\n", lastErr, delay, attempt)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return openai.ChatCompletionMessage{}, ctx.Err()
			}
		}
		resetAssistantEvents()
		sink := newStreamSink()
		sink.attempt = runtimeEvents.nextAttempt()
		emitRuntimeEvent("assistant_attempt", assistantAttemptEvent{Attempt: sink.attempt, Status: "start"})
		err := llmStreamOnce(ctx, client, cfg, req, sink)
		if err == nil {
			emitRuntimeEvent("assistant_attempt", assistantAttemptEvent{Attempt: sink.attempt, Status: "complete"})
			return openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: sink.content.String(), ReasoningContent: sink.reasoning.String(), ToolCalls: sink.calls()}, nil
		}
		emitRuntimeEvent("assistant_attempt", assistantAttemptEvent{Attempt: sink.attempt, Status: "discard", Error: err.Error()})
		resetAssistantEvents()
		lastErr = err
		if ctx.Err() != nil || interrupted() {
			return openai.ChatCompletionMessage{}, err
		}
		if !retryableLLMError(err) || attempt >= cfg.llmMaxRetries {
			if attempt >= cfg.llmMaxRetries && retryableLLMError(err) {
				return openai.ChatCompletionMessage{}, fmt.Errorf("重试 %d 次后仍失败: %w", cfg.llmMaxRetries, lastErr)
			}
			return openai.ChatCompletionMessage{}, err
		}
	}
}

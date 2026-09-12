package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestEmptyToolContentPresentOnWire(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(fmt.Sprint(streaming), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					Messages []map[string]json.RawMessage `json:"messages"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					http.Error(w, "decode", 400)
					return
				}
				last := request.Messages[len(request.Messages)-1]
				if string(last["content"]) != `""` || string(last["tool_call_id"]) != `"empty-ls"` {
					t.Errorf("empty tool response lost content or identity: %s", last)
					http.Error(w, "missing field content", 400)
					return
				}
				if streaming {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"continued\"}}]}\n\ndata: [DONE]\n\n")
				} else {
					fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"continued"}}]}`)
				}
			}))
			defer server.Close()
			cfg := openai.DefaultConfig("fixture")
			cfg.BaseURL = server.URL
			client := openai.NewClientWithConfig(cfg)
			// Resume of an old record without content must also serialize correctly.
			var oldTool openai.ChatCompletionMessage
			if err := json.Unmarshal([]byte(`{"role":"tool","tool_call_id":"empty-ls"}`), &oldTool); err != nil {
				t.Fatal(err)
			}
			request := openai.ChatCompletionRequest{Model: "fixture", Messages: []openai.ChatCompletionMessage{
				{Role: "assistant", ToolCalls: []openai.ToolCall{{ID: "empty-ls", Type: "function", Function: openai.FunctionCall{Name: "ls", Arguments: `{"path":"."}`}}}}, oldTool,
			}}
			if streaming {
				stream, err := client.CreateChatCompletionStream(context.Background(), request)
				if err != nil {
					t.Fatal(err)
				}
				defer stream.Close()
				chunk, err := stream.Recv()
				if err != nil {
					t.Fatal(err)
				}
				if chunk.Choices[0].Delta.Content != "continued" {
					t.Fatal(chunk)
				}
			} else {
				response, err := client.CreateChatCompletion(context.Background(), request)
				if err != nil {
					t.Fatal(err)
				}
				if response.Choices[0].Message.Content != "continued" {
					t.Fatal(response)
				}
			}
		})
	}
}

func TestToolContentMarshalPreservesOtherMessages(t *testing.T) {
	for _, message := range []openai.ChatCompletionMessage{
		{Role: "tool", Content: "unchanged", ToolCallID: "id"},
		{Role: "assistant", ToolCalls: []openai.ToolCall{{ID: "id", Type: "function", Function: openai.FunctionCall{Name: "ls", Arguments: `{}`}}}},
		{Role: "user", MultiContent: []openai.ChatMessagePart{{Type: openai.ChatMessagePartTypeText, Text: "text"}}},
	} {
		b, err := json.Marshal(message)
		if err != nil {
			t.Fatal(err)
		}
		var decoded map[string]json.RawMessage
		if err := json.Unmarshal(b, &decoded); err != nil {
			t.Fatal(err)
		}
		switch message.Role {
		case "tool":
			if string(decoded["content"]) != `"unchanged"` {
				t.Fatal(string(b))
			}
		case "assistant":
			if _, ok := decoded["content"]; ok {
				t.Fatal("assistant omission changed", string(b))
			}
			if len(decoded["tool_calls"]) == 0 {
				t.Fatal(string(b))
			}
		case "user":
			if len(decoded["content"]) == 0 || decoded["content"][0] != '[' {
				t.Fatal(string(b))
			}
		}
	}
}

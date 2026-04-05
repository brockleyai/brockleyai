package provider

import (
	"encoding/json"
	"testing"

	"github.com/brockleyai/brockleyai/internal/model"
)

var testTools = []model.LLMToolDefinition{
	{
		Name:        "get_weather",
		Description: "Get the current weather for a location",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"location":{"type":"string"}},"required":["location"]}`),
	},
	{
		Name:        "search",
		Description: "Search the web",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`),
	},
}

var testToolCalls = []model.ToolCall{
	{
		ID:        "call_123",
		Name:      "get_weather",
		Arguments: json.RawMessage(`{"location":"London"}`),
	},
}

var testMessagesWithTools = []model.Message{
	{Role: "user", Content: "What's the weather?"},
	{Role: "assistant", ToolCalls: testToolCalls},
	{Role: "tool", ToolCallID: "call_123", Content: `{"temp":20}`, ToolResultError: false},
	{Role: "assistant", Content: "It's 20 degrees in London."},
}

// TestOpenAICodec_EncodeTools verifies OpenAI tool definition encoding.
func TestOpenAICodec_EncodeTools(t *testing.T) {
	codec := &OpenAICodec{}
	raw, err := codec.EncodeTools(testTools, "auto")
	if err != nil {
		t.Fatalf("EncodeTools: %v", err)
	}

	var result struct {
		Tools      []openAIToolDef `json:"tools"`
		ToolChoice any             `json:"tool_choice"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(result.Tools))
	}
	if result.Tools[0].Function.Name != "get_weather" {
		t.Errorf("expected get_weather, got %s", result.Tools[0].Function.Name)
	}
	if result.Tools[0].Type != "function" {
		t.Errorf("expected type=function, got %s", result.Tools[0].Type)
	}
}

// TestOpenAICodec_EncodeMessages verifies OpenAI message encoding with tool calls.
func TestOpenAICodec_EncodeMessages(t *testing.T) {
	codec := &OpenAICodec{}
	raw, err := codec.EncodeMessages(testMessagesWithTools)
	if err != nil {
		t.Fatalf("EncodeMessages: %v", err)
	}

	var msgs []json.RawMessage
	if err := json.Unmarshal(raw, &msgs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}
}

// TestOpenAICodec_DecodeToolCalls verifies tool call extraction from OpenAI response.
func TestOpenAICodec_DecodeToolCalls(t *testing.T) {
	codec := &OpenAICodec{}
	respBody := json.RawMessage(`{
		"choices": [{
			"message": {
				"role": "assistant",
				"tool_calls": [{
					"id": "call_abc",
					"type": "function",
					"function": {"name": "get_weather", "arguments": "{\"location\":\"London\"}"}
				}]
			},
			"finish_reason": "tool_calls"
		}]
	}`)
	calls, finishReason, err := codec.DecodeToolCalls(respBody)
	if err != nil {
		t.Fatalf("DecodeToolCalls: %v", err)
	}
	if finishReason != "tool_calls" {
		t.Errorf("expected finish_reason=tool_calls, got %s", finishReason)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Name != "get_weather" {
		t.Errorf("expected get_weather, got %s", calls[0].Name)
	}
	if calls[0].ID != "call_abc" {
		t.Errorf("expected call_abc, got %s", calls[0].ID)
	}
}

// TestAnthropicCodec_EncodeTools verifies Anthropic tool definition encoding.
func TestAnthropicCodec_EncodeTools(t *testing.T) {
	codec := &AnthropicCodec{}
	raw, err := codec.EncodeTools(testTools, "auto")
	if err != nil {
		t.Fatalf("EncodeTools: %v", err)
	}

	var result struct {
		Tools      []anthropicToolDef          `json:"tools"`
		ToolChoice *anthropicToolChoiceRequest `json:"tool_choice"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(result.Tools))
	}
	if result.Tools[0].Name != "get_weather" {
		t.Errorf("expected get_weather, got %s", result.Tools[0].Name)
	}
	if result.ToolChoice == nil || result.ToolChoice.Type != "auto" {
		t.Errorf("expected tool_choice.type=auto")
	}
}

// TestAnthropicCodec_EncodeToolChoice_Required verifies Anthropic "required" → "any".
func TestAnthropicCodec_EncodeToolChoice_Required(t *testing.T) {
	codec := &AnthropicCodec{}
	raw, err := codec.EncodeTools(testTools, "required")
	if err != nil {
		t.Fatalf("EncodeTools: %v", err)
	}

	var result struct {
		ToolChoice *anthropicToolChoiceRequest `json:"tool_choice"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.ToolChoice == nil || result.ToolChoice.Type != "any" {
		t.Errorf("expected tool_choice.type=any for 'required'")
	}
}

// TestAnthropicCodec_DecodeToolCalls verifies tool call extraction from Anthropic response.
func TestAnthropicCodec_DecodeToolCalls(t *testing.T) {
	codec := &AnthropicCodec{}
	respBody := json.RawMessage(`{
		"content": [
			{"type": "text", "text": "Let me check the weather."},
			{"type": "tool_use", "id": "toolu_123", "name": "get_weather", "input": {"location": "London"}}
		],
		"stop_reason": "tool_use"
	}`)
	calls, finishReason, err := codec.DecodeToolCalls(respBody)
	if err != nil {
		t.Fatalf("DecodeToolCalls: %v", err)
	}
	if finishReason != "tool_calls" {
		t.Errorf("expected finish_reason=tool_calls, got %s", finishReason)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Name != "get_weather" {
		t.Errorf("expected get_weather, got %s", calls[0].Name)
	}
	if calls[0].ID != "toolu_123" {
		t.Errorf("expected toolu_123, got %s", calls[0].ID)
	}
}

// TestAnthropicCodec_FinishReasonMapping verifies stop reason translation.
func TestAnthropicCodec_FinishReasonMapping(t *testing.T) {
	codec := &AnthropicCodec{}
	tests := []struct {
		stopReason string
		wantFinish string
	}{
		{"end_turn", "stop"},
		{"tool_use", "tool_calls"},
		{"max_tokens", "length"},
	}
	for _, tc := range tests {
		body := json.RawMessage(`{"content":[],"stop_reason":"` + tc.stopReason + `"}`)
		_, finishReason, err := codec.DecodeToolCalls(body)
		if err != nil {
			t.Fatalf("DecodeToolCalls(%s): %v", tc.stopReason, err)
		}
		if finishReason != tc.wantFinish {
			t.Errorf("stop_reason=%s: expected %s, got %s", tc.stopReason, tc.wantFinish, finishReason)
		}
	}
}

// TestGoogleCodec_EncodeTools verifies Google tool definition encoding.
func TestGoogleCodec_EncodeTools(t *testing.T) {
	codec := &GoogleCodec{}
	raw, err := codec.EncodeTools(testTools, "auto")
	if err != nil {
		t.Fatalf("EncodeTools: %v", err)
	}

	var result struct {
		Tools []googleToolDef `json:"tools"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Tools) != 1 {
		t.Fatalf("expected 1 tool group, got %d", len(result.Tools))
	}
	if len(result.Tools[0].FunctionDeclarations) != 2 {
		t.Fatalf("expected 2 function declarations, got %d", len(result.Tools[0].FunctionDeclarations))
	}
	if result.Tools[0].FunctionDeclarations[0].Name != "get_weather" {
		t.Errorf("expected get_weather, got %s", result.Tools[0].FunctionDeclarations[0].Name)
	}
}

// TestGoogleCodec_DecodeToolCalls verifies tool call extraction from Google response.
func TestGoogleCodec_DecodeToolCalls(t *testing.T) {
	codec := &GoogleCodec{}
	respBody := json.RawMessage(`{
		"candidates": [{
			"content": {
				"parts": [
					{"functionCall": {"name": "get_weather", "args": {"location": "London"}}}
				]
			},
			"finishReason": "STOP"
		}]
	}`)
	calls, finishReason, err := codec.DecodeToolCalls(respBody)
	if err != nil {
		t.Fatalf("DecodeToolCalls: %v", err)
	}
	if finishReason != "tool_calls" {
		t.Errorf("expected finish_reason=tool_calls, got %s", finishReason)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Name != "get_weather" {
		t.Errorf("expected get_weather, got %s", calls[0].Name)
	}
}

// TestOpenAICodec_DecodeToolCalls_NoToolCalls verifies no tool calls returns nil.
func TestOpenAICodec_DecodeToolCalls_NoToolCalls(t *testing.T) {
	codec := &OpenAICodec{}
	respBody := json.RawMessage(`{
		"choices": [{
			"message": {"role": "assistant", "content": "Hello!"},
			"finish_reason": "stop"
		}]
	}`)
	calls, finishReason, err := codec.DecodeToolCalls(respBody)
	if err != nil {
		t.Fatalf("DecodeToolCalls: %v", err)
	}
	if finishReason != "stop" {
		t.Errorf("expected finish_reason=stop, got %s", finishReason)
	}
	if len(calls) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(calls))
	}
}

package worker

import (
	"encoding/json"
	"testing"

	"github.com/brockleyai/brockleyai/internal/model"
)

func TestResultKeyForExecution(t *testing.T) {
	key := ResultKeyForExecution("exec-123")
	expected := "exec:exec-123:results"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestResultKeyForLLMCall(t *testing.T) {
	key := ResultKeyForLLMCall("exec-123", "req-456")
	expected := "exec:exec-123:llm:req-456:mcp-results"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestNodeTaskResult_Serialization(t *testing.T) {
	result := NodeTaskResult{
		RequestID: "req-1",
		NodeID:    "node-1",
		Status:    "completed",
		Outputs: map[string]any{
			"response_text": "hello world",
		},
		Attempt: 0,
		LLMDebug: &model.LLMDebugTrace{
			Calls: []model.LLMCallTrace{
				{
					RequestID: "req-1",
					Provider:  "openrouter",
					Model:     "openai/gpt-oss-20b",
					Request:   json.RawMessage(`{"model":"openai/gpt-oss-20b"}`),
					Response:  json.RawMessage(`{"id":"resp-1"}`),
				},
			},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded NodeTaskResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.RequestID != result.RequestID {
		t.Errorf("request_id: expected %q, got %q", result.RequestID, decoded.RequestID)
	}
	if decoded.Status != "completed" {
		t.Errorf("status: expected completed, got %q", decoded.Status)
	}
	if decoded.Outputs["response_text"] != "hello world" {
		t.Errorf("outputs: expected 'hello world', got %v", decoded.Outputs["response_text"])
	}
	if decoded.LLMDebug == nil || len(decoded.LLMDebug.Calls) != 1 {
		t.Fatalf("expected llm_debug round-trip, got %#v", decoded.LLMDebug)
	}
	if decoded.LLMDebug.Calls[0].Provider != "openrouter" {
		t.Errorf("llm_debug.provider: expected openrouter, got %q", decoded.LLMDebug.Calls[0].Provider)
	}
}

func TestToolLoopState_Serialization(t *testing.T) {
	tls := ToolLoopState{
		MaxCalls:       25,
		MaxIterations:  10,
		Iteration:      2,
		TotalToolCalls: 5,
		History: []ToolCallHistoryEntry{
			{Name: "echo", Result: "ok", DurationMs: 100},
		},
		Routing: map[string]model.ToolRoute{
			"echo": {MCPURL: "http://mcp:9001"},
		},
	}

	data, err := json.Marshal(tls)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ToolLoopState
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.MaxCalls != 25 {
		t.Errorf("max_calls: expected 25, got %d", decoded.MaxCalls)
	}
	if decoded.Iteration != 2 {
		t.Errorf("iteration: expected 2, got %d", decoded.Iteration)
	}
	if len(decoded.History) != 1 {
		t.Errorf("history: expected 1, got %d", len(decoded.History))
	}
	if decoded.Routing["echo"].MCPURL != "http://mcp:9001" {
		t.Errorf("routing: unexpected MCP URL")
	}
}

func TestMCPCallResult_Serialization(t *testing.T) {
	result := MCPCallResult{
		RequestID:  "call-1",
		ToolCallID: "tc-1",
		ToolName:   "echo",
		Content:    "hello",
		DurationMs: 50,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded MCPCallResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ToolName != "echo" {
		t.Errorf("tool_name: expected echo, got %q", decoded.ToolName)
	}
	if decoded.Content != "hello" {
		t.Errorf("content: expected hello, got %v", decoded.Content)
	}
}

func TestLLMCallTask_Serialization(t *testing.T) {
	task := LLMCallTask{
		ExecutionID: "exec-1",
		RequestID:   "req-1",
		NodeID:      "node-1",
		Provider:    "openrouter",
		Request: &model.CompletionRequest{
			Model:      "gpt-4",
			UserPrompt: "Hello",
		},
		ToolLoop: &ToolLoopState{
			MaxCalls:      25,
			MaxIterations: 10,
		},
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded LLMCallTask
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Provider != "openrouter" {
		t.Errorf("provider: expected openrouter, got %q", decoded.Provider)
	}
	if decoded.Request.Model != "gpt-4" {
		t.Errorf("model: expected gpt-4, got %q", decoded.Request.Model)
	}
	if decoded.ToolLoop == nil {
		t.Fatal("tool_loop: expected non-nil")
	}
	if decoded.ToolLoop.MaxCalls != 25 {
		t.Errorf("tool_loop.max_calls: expected 25, got %d", decoded.ToolLoop.MaxCalls)
	}
}

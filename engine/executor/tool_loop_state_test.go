package executor

import (
	"encoding/json"
	"testing"

	"github.com/brockleyai/brockleyai/internal/model"
)

func TestBuildInitialToolLoopState(t *testing.T) {
	maxCalls := 15
	maxIter := 5
	cfg := &model.LLMNodeConfig{
		Provider:          "openrouter",
		Model:             "gpt-4",
		ToolLoop:          true,
		MaxToolCalls:      &maxCalls,
		MaxLoopIterations: &maxIter,
		ToolRouting: map[string]model.ToolRoute{
			"echo": {MCPURL: "http://mcp:9001"},
		},
	}

	inputs := map[string]any{"prompt": "test"}
	nctx := &NodeContext{
		State: map[string]any{"counter": 0},
		Meta:  map[string]any{"execution_id": "exec-1"},
	}

	state := BuildInitialToolLoopState(cfg, cfg.ToolRouting, inputs, nctx)

	if state.MaxCalls != 15 {
		t.Errorf("expected max_calls=15, got %d", state.MaxCalls)
	}
	if state.MaxIterations != 5 {
		t.Errorf("expected max_iterations=5, got %d", state.MaxIterations)
	}
	if state.Routing["echo"].MCPURL != "http://mcp:9001" {
		t.Error("unexpected routing")
	}
	if state.NodeInputs["prompt"] != "test" {
		t.Error("unexpected node inputs")
	}
	if state.NodeState["counter"] != 0 {
		t.Error("unexpected node state")
	}
	if state.NodeMeta["execution_id"] != "exec-1" {
		t.Error("unexpected node meta")
	}
}

func TestBuildInitialToolLoopState_Defaults(t *testing.T) {
	cfg := &model.LLMNodeConfig{
		Provider: "openrouter",
		Model:    "gpt-4",
		ToolLoop: true,
	}

	state := BuildInitialToolLoopState(cfg, nil, nil, nil)

	if state.MaxCalls != defaultMaxToolCalls {
		t.Errorf("expected default max_calls=%d, got %d", defaultMaxToolCalls, state.MaxCalls)
	}
	if state.MaxIterations != defaultMaxLoopIterations {
		t.Errorf("expected default max_iterations=%d, got %d", defaultMaxLoopIterations, state.MaxIterations)
	}
}

func TestSerializableToolLoopState_RoundTrip(t *testing.T) {
	state := &SerializableToolLoopState{
		MaxCalls:       25,
		MaxIterations:  10,
		Iteration:      3,
		TotalToolCalls: 7,
		History: []ToolCallHistoryEntry{
			{Name: "echo", Result: "ok", DurationMs: 100},
			{Name: "search", Result: `{"found": true}`, DurationMs: 200},
		},
		Routing: map[string]model.ToolRoute{
			"echo":   {MCPURL: "http://mcp:9001"},
			"search": {MCPURL: "http://mcp:9002"},
		},
		FinishReason: "",
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded SerializableToolLoopState
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Iteration != 3 {
		t.Errorf("iteration: expected 3, got %d", decoded.Iteration)
	}
	if decoded.TotalToolCalls != 7 {
		t.Errorf("total_tool_calls: expected 7, got %d", decoded.TotalToolCalls)
	}
	if len(decoded.History) != 2 {
		t.Errorf("history: expected 2, got %d", len(decoded.History))
	}
	if len(decoded.Routing) != 2 {
		t.Errorf("routing: expected 2, got %d", len(decoded.Routing))
	}
}

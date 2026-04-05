package worker

import (
	"encoding/json"
	"testing"

	"github.com/brockleyai/brockleyai/internal/model"
)

func TestResultKeyForSuperagent(t *testing.T) {
	key := ResultKeyForSuperagent("exec-123", "agent-1", 42)
	expected := "sa:exec-123:agent-1:42"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestResultKeyForSuperagent_UniquePerSeq(t *testing.T) {
	k1 := ResultKeyForSuperagent("exec-1", "node-1", 1)
	k2 := ResultKeyForSuperagent("exec-1", "node-1", 2)
	if k1 == k2 {
		t.Errorf("expected unique keys, both got %q", k1)
	}
}

func TestLLMCallTask_ResultKey_Serialization(t *testing.T) {
	task := LLMCallTask{
		ExecutionID: "exec-1",
		RequestID:   "req-1",
		NodeID:      "node-1",
		Provider:    "openai",
		APIKey:      "test-key",
		Request: &model.CompletionRequest{
			Model: "gpt-4",
			Messages: []model.Message{
				{Role: "user", Content: "hello"},
			},
		},
		ResultKey: "sa:exec-1:node-1:1",
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded LLMCallTask
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ResultKey != "sa:exec-1:node-1:1" {
		t.Errorf("expected result_key %q, got %q", "sa:exec-1:node-1:1", decoded.ResultKey)
	}
}

func TestLLMCallTask_ResultKey_OmittedWhenEmpty(t *testing.T) {
	task := LLMCallTask{
		ExecutionID: "exec-1",
		RequestID:   "req-1",
		NodeID:      "node-1",
		Provider:    "openai",
		APIKey:      "test-key",
		Request:     &model.CompletionRequest{Model: "gpt-4"},
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// result_key should not appear in JSON when empty.
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if _, exists := raw["result_key"]; exists {
		t.Error("result_key should be omitted when empty")
	}
}

func TestNodeRunTask_OutputPorts_Serialization(t *testing.T) {
	task := NodeRunTask{
		ExecutionID: "exec-1",
		RequestID:   "req-1",
		NodeID:      "agent-1",
		NodeType:    "superagent",
		NodeConfig:  json.RawMessage(`{}`),
		Inputs:      map[string]any{"topic": "test"},
		OutputPorts: []model.Port{
			{Name: "result", Schema: json.RawMessage(`{"type":"string"}`)},
			{Name: "sources", Schema: json.RawMessage(`{"type":"array"}`)},
		},
		ResultKey: "exec:exec-1:results",
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded NodeRunTask
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(decoded.OutputPorts) != 2 {
		t.Fatalf("expected 2 output ports, got %d", len(decoded.OutputPorts))
	}
	if decoded.OutputPorts[0].Name != "result" {
		t.Errorf("expected port name %q, got %q", "result", decoded.OutputPorts[0].Name)
	}
}

func TestNodeRunTask_OutputPorts_OmittedWhenEmpty(t *testing.T) {
	task := NodeRunTask{
		ExecutionID: "exec-1",
		RequestID:   "req-1",
		NodeID:      "node-1",
		NodeType:    "forEach",
		NodeConfig:  json.RawMessage(`{}`),
		ResultKey:   "exec:exec-1:results",
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if _, exists := raw["output_ports"]; exists {
		t.Error("output_ports should be omitted when empty")
	}
}

func TestResolveSkillHeaders(t *testing.T) {
	skill := model.SuperagentSkill{
		Name:   "test",
		MCPURL: "http://localhost:9090",
		Headers: []model.HeaderConfig{
			{Name: "Authorization", Value: "Bearer token123"},
			{Name: "X-Dynamic", FromInput: "some_input"}, // non-static, should be skipped
		},
	}

	headers := resolveSkillHeaders(skill)

	if headers["Authorization"] != "Bearer token123" {
		t.Errorf("expected Authorization header, got %q", headers["Authorization"])
	}
	if _, exists := headers["X-Dynamic"]; exists {
		t.Error("dynamic headers should not be included in static resolution")
	}
}

func TestTaskTypeSuperagent_Constant(t *testing.T) {
	if TaskTypeSuperagent != "node:superagent" {
		t.Errorf("expected %q, got %q", "node:superagent", TaskTypeSuperagent)
	}
}

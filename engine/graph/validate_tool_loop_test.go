package graph

import (
	"encoding/json"
	"testing"

	"github.com/brockleyai/brockleyai/internal/model"
)

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func makeToolLoopGraph(cfg model.LLMNodeConfig) *model.Graph {
	return &model.Graph{
		Nodes: []model.Node{
			{
				ID: "input", Name: "input", Type: model.NodeTypeInput,
				OutputPorts: []model.Port{{Name: "query", Schema: json.RawMessage(`{"type":"string"}`)}},
			},
			{
				ID: "llm", Name: "llm", Type: model.NodeTypeLLM,
				Config:     mustJSON(cfg),
				InputPorts: []model.Port{{Name: "user_prompt", Schema: json.RawMessage(`{"type":"string"}`)}},
				OutputPorts: []model.Port{
					{Name: "response_text", Schema: json.RawMessage(`{"type":"string"}`)},
				},
			},
			{
				ID: "output", Name: "output", Type: model.NodeTypeOutput,
				InputPorts: []model.Port{{Name: "result", Schema: json.RawMessage(`{"type":"string"}`)}},
			},
		},
		Edges: []model.Edge{
			{ID: "e1", SourceNodeID: "input", SourcePort: "query", TargetNodeID: "llm", TargetPort: "user_prompt"},
			{ID: "e2", SourceNodeID: "llm", SourcePort: "response_text", TargetNodeID: "output", TargetPort: "result"},
		},
	}
}

func TestValidate_ToolLoop_NoRouting(t *testing.T) {
	cfg := model.LLMNodeConfig{
		Provider:   "mock",
		Model:      "test",
		UserPrompt: "hello",
		ToolLoop:   true,
		// No routing configured
	}
	g := makeToolLoopGraph(cfg)
	result := Validate(g)
	if result.Valid {
		t.Fatal("expected validation to fail for tool_loop without routing")
	}
	found := false
	for _, e := range result.Errors {
		if e.Code == "TOOL_LOOP_NO_ROUTING" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected TOOL_LOOP_NO_ROUTING error, got: %+v", result.Errors)
	}
}

func TestValidate_ToolLoop_ValidConfig(t *testing.T) {
	cfg := model.LLMNodeConfig{
		Provider:   "mock",
		Model:      "test",
		UserPrompt: "hello",
		ToolLoop:   true,
		ToolRouting: map[string]model.ToolRoute{
			"echo": {MCPURL: "http://mcp:9001"},
		},
	}
	g := makeToolLoopGraph(cfg)
	result := Validate(g)
	if !result.Valid {
		t.Fatalf("expected valid, got errors: %+v", result.Errors)
	}
}

func TestValidate_ToolLoop_EmptyMCPURL(t *testing.T) {
	cfg := model.LLMNodeConfig{
		Provider:   "mock",
		Model:      "test",
		UserPrompt: "hello",
		ToolLoop:   true,
		ToolRouting: map[string]model.ToolRoute{
			"echo": {MCPURL: ""},
		},
	}
	g := makeToolLoopGraph(cfg)
	result := Validate(g)
	if result.Valid {
		t.Fatal("expected validation to fail for empty mcp_url")
	}
	found := false
	for _, e := range result.Errors {
		if e.Code == "TOOL_ROUTE_NO_TARGET" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected TOOL_ROUTE_NO_TARGET error")
	}
}

func TestValidate_ToolLoop_NegativeMaxToolCalls(t *testing.T) {
	neg := -1
	cfg := model.LLMNodeConfig{
		Provider:     "mock",
		Model:        "test",
		UserPrompt:   "hello",
		ToolLoop:     true,
		MaxToolCalls: &neg,
		ToolRouting: map[string]model.ToolRoute{
			"echo": {MCPURL: "http://mcp:9001"},
		},
	}
	g := makeToolLoopGraph(cfg)
	result := Validate(g)
	if result.Valid {
		t.Fatal("expected validation to fail for negative max_tool_calls")
	}
}

func TestValidate_ToolLoop_InvalidToolChoice(t *testing.T) {
	cfg := model.LLMNodeConfig{
		Provider:   "mock",
		Model:      "test",
		UserPrompt: "hello",
		ToolLoop:   true,
		ToolChoice: "nonexistent_tool",
		ToolRouting: map[string]model.ToolRoute{
			"echo": {MCPURL: "http://mcp:9001"},
		},
	}
	g := makeToolLoopGraph(cfg)
	result := Validate(g)
	if result.Valid {
		t.Fatal("expected validation to fail for invalid tool_choice")
	}
	found := false
	for _, e := range result.Errors {
		if e.Code == "TOOL_CHOICE_INVALID" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected TOOL_CHOICE_INVALID error")
	}
}

func TestValidate_ToolLoop_ToolChoiceAsToolName(t *testing.T) {
	cfg := model.LLMNodeConfig{
		Provider:   "mock",
		Model:      "test",
		UserPrompt: "hello",
		ToolLoop:   true,
		ToolChoice: "echo",
		Tools: []model.LLMToolDefinition{
			{Name: "echo", Description: "echo", Parameters: json.RawMessage(`{"type":"object"}`)},
		},
		ToolRouting: map[string]model.ToolRoute{
			"echo": {MCPURL: "http://mcp:9001"},
		},
	}
	g := makeToolLoopGraph(cfg)
	result := Validate(g)
	if !result.Valid {
		t.Fatalf("expected valid when tool_choice is a valid tool name, got errors: %+v", result.Errors)
	}
}

func TestValidate_ToolLoop_ToolNoRoutingWarning(t *testing.T) {
	cfg := model.LLMNodeConfig{
		Provider:   "mock",
		Model:      "test",
		UserPrompt: "hello",
		ToolLoop:   true,
		Tools: []model.LLMToolDefinition{
			{Name: "echo", Description: "echo", Parameters: json.RawMessage(`{"type":"object"}`)},
			{Name: "search", Description: "search", Parameters: json.RawMessage(`{"type":"object"}`)},
		},
		ToolRouting: map[string]model.ToolRoute{
			"echo": {MCPURL: "http://mcp:9001"},
			// "search" has no routing
		},
	}
	g := makeToolLoopGraph(cfg)
	result := Validate(g)
	if !result.Valid {
		t.Fatalf("expected valid (warning only), got errors: %+v", result.Errors)
	}
	found := false
	for _, w := range result.Warnings {
		if w.Code == "TOOL_NO_ROUTING" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected TOOL_NO_ROUTING warning")
	}
}

func TestValidate_ToolLoop_NegativeTimeout(t *testing.T) {
	neg := -5
	cfg := model.LLMNodeConfig{
		Provider:   "mock",
		Model:      "test",
		UserPrompt: "hello",
		ToolLoop:   true,
		ToolRouting: map[string]model.ToolRoute{
			"echo": {MCPURL: "http://mcp:9001", TimeoutSeconds: &neg},
		},
	}
	g := makeToolLoopGraph(cfg)
	result := Validate(g)
	if result.Valid {
		t.Fatal("expected validation to fail for negative timeout")
	}
}

func TestValidate_ToolLoop_FromState(t *testing.T) {
	cfg := model.LLMNodeConfig{
		Provider:             "mock",
		Model:                "test",
		UserPrompt:           "hello",
		ToolLoop:             true,
		ToolRoutingFromState: "tool_routing",
	}
	g := makeToolLoopGraph(cfg)
	result := Validate(g)
	if !result.Valid {
		t.Fatalf("expected valid for tool_routing_from_state, got errors: %+v", result.Errors)
	}
}

func TestValidate_ToolLoop_CompactedRouteRequiresMCP(t *testing.T) {
	cfg := model.LLMNodeConfig{
		Provider:   "mock",
		Model:      "test",
		UserPrompt: "hello",
		ToolLoop:   true,
		ToolRouting: map[string]model.ToolRoute{
			"tool": {APIToolID: "api-1", APIEndpoint: "ep", Compacted: true},
		},
	}
	g := makeToolLoopGraph(cfg)
	result := Validate(g)
	found := false
	for _, e := range result.Errors {
		if e.Code == "TOOL_ROUTE_COMPACTED_NO_MCP" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected TOOL_ROUTE_COMPACTED_NO_MCP error, got: %+v", result.Errors)
	}
}

func TestValidate_ToolLoop_CompactedNoContextWarning(t *testing.T) {
	cfg := model.LLMNodeConfig{
		Provider:   "mock",
		Model:      "test",
		UserPrompt: "hello",
		ToolLoop:   true,
		// No system_prompt — should warn
		ToolRouting: map[string]model.ToolRoute{
			"tool": {MCPURL: "http://mcp:9001", Compacted: true},
		},
	}
	g := makeToolLoopGraph(cfg)
	result := Validate(g)
	found := false
	for _, w := range result.Warnings {
		if w.Code == "TOOL_COMPACTED_NO_CONTEXT" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected TOOL_COMPACTED_NO_CONTEXT warning, got warnings: %+v", result.Warnings)
	}
}

func TestValidate_ToolLoop_CompactedWithSystemPrompt_NoWarning(t *testing.T) {
	cfg := model.LLMNodeConfig{
		Provider:     "mock",
		Model:        "test",
		UserPrompt:   "hello",
		SystemPrompt: "You have access to a knowledge base MCP server.",
		ToolLoop:     true,
		ToolRouting: map[string]model.ToolRoute{
			"tool": {MCPURL: "http://mcp:9001", Compacted: true},
		},
	}
	g := makeToolLoopGraph(cfg)
	result := Validate(g)
	for _, w := range result.Warnings {
		if w.Code == "TOOL_COMPACTED_NO_CONTEXT" {
			t.Errorf("unexpected TOOL_COMPACTED_NO_CONTEXT warning when system_prompt is set")
		}
	}
}

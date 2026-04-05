package worker

import (
	"encoding/json"
	"testing"

	"github.com/brockleyai/brockleyai/internal/model"
)

func TestBuildEdgeMaps(t *testing.T) {
	graph := &model.Graph{
		Nodes: []model.Node{
			{ID: "input", Type: "input"},
			{ID: "llm1", Type: "llm"},
			{ID: "output", Type: "output"},
		},
		Edges: []model.Edge{
			{ID: "e1", SourceNodeID: "input", SourcePort: "data", TargetNodeID: "llm1", TargetPort: "prompt"},
			{ID: "e2", SourceNodeID: "llm1", SourcePort: "response_text", TargetNodeID: "output", TargetPort: "result"},
		},
	}

	nodeMap, outEdges, inEdges := buildEdgeMaps(graph)

	if len(nodeMap) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(nodeMap))
	}
	if nodeMap["llm1"] == nil {
		t.Error("llm1 not in nodeMap")
	}
	if len(outEdges["input"]) != 1 {
		t.Errorf("expected 1 outgoing edge from input, got %d", len(outEdges["input"]))
	}
	if len(inEdges["llm1"]) != 1 {
		t.Errorf("expected 1 incoming edge to llm1, got %d", len(inEdges["llm1"]))
	}
}

func TestInitState(t *testing.T) {
	graph := &model.Graph{
		State: &model.GraphState{
			Fields: []model.StateField{
				{Name: "counter", Schema: json.RawMessage(`{"type":"integer"}`), Initial: json.RawMessage(`5`)},
				{Name: "items", Schema: json.RawMessage(`{"type":"array"}`)},
				{Name: "name", Schema: json.RawMessage(`{"type":"string"}`)},
			},
		},
	}

	state := initState(graph)

	if state["counter"] != float64(5) {
		t.Errorf("counter: expected 5, got %v (type %T)", state["counter"], state["counter"])
	}
	items, ok := state["items"].([]any)
	if !ok {
		t.Errorf("items: expected []any, got %T", state["items"])
	}
	if len(items) != 0 {
		t.Errorf("items: expected empty, got %v", items)
	}
	if state["name"] != "" {
		t.Errorf("name: expected empty string, got %v", state["name"])
	}
}

func TestInitState_NilState(t *testing.T) {
	graph := &model.Graph{}
	state := initState(graph)
	if len(state) != 0 {
		t.Errorf("expected empty state, got %d fields", len(state))
	}
}

func TestResolveInputs_EdgePriority(t *testing.T) {
	node := &model.Node{
		ID:   "n1",
		Type: "transform",
		InputPorts: []model.Port{
			{Name: "value", Default: json.RawMessage(`"default_val"`)},
		},
		StateReads: []model.StateBinding{
			{StateField: "stored_value", Port: "value"},
		},
	}

	nodeOutputs := map[string]map[string]any{
		"src": {"out": "edge_val"},
	}
	incoming := []model.Edge{
		{SourceNodeID: "src", SourcePort: "out", TargetPort: "value"},
	}
	state := map[string]any{
		"stored_value": "state_val",
	}

	inputs := resolveInputs(node, nodeOutputs, incoming, state)

	// Edge should take priority over state and default.
	if inputs["value"] != "edge_val" {
		t.Errorf("expected edge_val, got %v", inputs["value"])
	}
}

func TestResolveInputs_StatePriority(t *testing.T) {
	node := &model.Node{
		ID:   "n1",
		Type: "transform",
		InputPorts: []model.Port{
			{Name: "value", Default: json.RawMessage(`"default_val"`)},
		},
		StateReads: []model.StateBinding{
			{StateField: "stored_value", Port: "value"},
		},
	}

	nodeOutputs := map[string]map[string]any{}
	incoming := []model.Edge{}
	state := map[string]any{
		"stored_value": "state_val",
	}

	inputs := resolveInputs(node, nodeOutputs, incoming, state)

	// State should take priority over default.
	if inputs["value"] != "state_val" {
		t.Errorf("expected state_val, got %v", inputs["value"])
	}
}

func TestResolveInputs_DefaultOnly(t *testing.T) {
	node := &model.Node{
		ID:   "n1",
		Type: "transform",
		InputPorts: []model.Port{
			{Name: "value", Default: json.RawMessage(`"default_val"`)},
		},
	}

	inputs := resolveInputs(node, nil, nil, nil)

	if inputs["value"] != "default_val" {
		t.Errorf("expected default_val, got %v", inputs["value"])
	}
}

func TestResolveInputs_SkipsBackEdges(t *testing.T) {
	node := &model.Node{
		ID:   "n1",
		Type: "transform",
	}

	nodeOutputs := map[string]map[string]any{
		"src": {"out": "back_edge_val"},
	}
	incoming := []model.Edge{
		{SourceNodeID: "src", SourcePort: "out", TargetPort: "value", BackEdge: true},
	}

	inputs := resolveInputs(node, nodeOutputs, incoming, nil)

	if _, ok := inputs["value"]; ok {
		t.Errorf("expected no value for back-edge, got %v", inputs["value"])
	}
}

func TestApplyStateWrites_Replace(t *testing.T) {
	node := &model.Node{
		StateWrites: []model.StateBinding{
			{StateField: "result", Port: "output"},
		},
	}

	state := map[string]any{"result": "old"}
	outputs := map[string]any{"output": "new"}

	applyStateWrites(node, outputs, state)

	if state["result"] != "new" {
		t.Errorf("expected 'new', got %v", state["result"])
	}
}

func TestApplyStateWrites_Append(t *testing.T) {
	node := &model.Node{
		StateWrites: []model.StateBinding{
			{StateField: "items", Port: "item"},
		},
	}

	state := map[string]any{"items": []any{"a", "b"}}
	outputs := map[string]any{"item": "c"}

	applyStateWrites(node, outputs, state)

	items := state["items"].([]any)
	if len(items) != 3 || items[2] != "c" {
		t.Errorf("expected [a b c], got %v", items)
	}
}

func TestApplyStateWrites_NilOutputs(t *testing.T) {
	node := &model.Node{
		StateWrites: []model.StateBinding{
			{StateField: "result", Port: "output"},
		},
	}

	state := map[string]any{"result": "original"}
	applyStateWrites(node, nil, state)

	if state["result"] != "original" {
		t.Errorf("expected 'original' (unchanged), got %v", state["result"])
	}
}

func TestPropagateSkips(t *testing.T) {
	condNode := &model.Node{ID: "cond", Type: "conditional"}
	nodeMap := map[string]*model.Node{
		"cond":    condNode,
		"branch1": {ID: "branch1", Type: "llm"},
		"branch2": {ID: "branch2", Type: "llm"},
	}

	outEdges := map[string][]model.Edge{
		"cond": {
			{ID: "e1", SourceNodeID: "cond", SourcePort: "true", TargetNodeID: "branch1"},
			{ID: "e2", SourceNodeID: "cond", SourcePort: "false", TargetNodeID: "branch2"},
		},
	}
	inEdges := map[string][]model.Edge{
		"branch1": {{ID: "e1", SourceNodeID: "cond"}},
		"branch2": {{ID: "e2", SourceNodeID: "cond"}},
	}

	skipped := make(map[string]bool)
	deadEdges := make(map[string]bool)

	// Conditional outputs: "true" branch gets value, "false" gets nil.
	outputs := map[string]any{
		"true":  "value",
		"false": nil,
	}

	propagateSkips(condNode, outputs, outEdges["cond"], nodeMap, outEdges, inEdges, deadEdges, skipped)

	if skipped["branch1"] {
		t.Error("branch1 should not be skipped")
	}
	if !skipped["branch2"] {
		t.Error("branch2 should be skipped")
	}
}

func TestCopyMap(t *testing.T) {
	original := map[string]any{"a": 1, "b": "two"}
	copy := copyMap(original)

	if copy["a"] != 1 || copy["b"] != "two" {
		t.Error("copy doesn't match original")
	}

	// Modify copy — original should be unaffected.
	copy["a"] = 99
	if original["a"] != 1 {
		t.Error("modifying copy changed original")
	}
}

func TestCopyMap_Nil(t *testing.T) {
	result := copyMap(nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}
}

package graph

import (
	"encoding/json"
	"testing"

	"github.com/brockleyai/brockleyai/internal/model"
)

func boolPtr(b bool) *bool { return &b }

func makeSchema(typ string) json.RawMessage {
	return json.RawMessage(`{"type":"` + typ + `"}`)
}

func makeObjectSchema(props map[string]string) json.RawMessage {
	p := make(map[string]any)
	required := []string{}
	for k, v := range props {
		p[k] = map[string]string{"type": v}
		required = append(required, k)
	}
	b, _ := json.Marshal(map[string]any{"type": "object", "properties": p, "required": required})
	return b
}

func makeArraySchema(itemType string) json.RawMessage {
	b, _ := json.Marshal(map[string]any{"type": "array", "items": map[string]string{"type": itemType}})
	return b
}

func minimalValidGraph() *model.Graph {
	return &model.Graph{
		ID:        "g1",
		TenantID:  "default",
		Name:      "test",
		Namespace: "default",
		Nodes: []model.Node{
			{
				ID:          "input1",
				Name:        "Input",
				Type:        model.NodeTypeInput,
				InputPorts:  []model.Port{},
				OutputPorts: []model.Port{{Name: "query", Schema: makeSchema("string")}},
				Config:      json.RawMessage(`{}`),
			},
			{
				ID:          "output1",
				Name:        "Output",
				Type:        model.NodeTypeOutput,
				InputPorts:  []model.Port{{Name: "result", Schema: makeSchema("string")}},
				OutputPorts: []model.Port{},
				Config:      json.RawMessage(`{}`),
			},
		},
		Edges: []model.Edge{
			{ID: "e1", SourceNodeID: "input1", SourcePort: "query", TargetNodeID: "output1", TargetPort: "result"},
		},
	}
}

func TestValidate_MinimalValidGraph(t *testing.T) {
	g := minimalValidGraph()
	result := Validate(g)
	if !result.Valid {
		t.Errorf("expected valid graph, got errors: %+v", result.Errors)
	}
}

func TestValidate_EmptyGraph(t *testing.T) {
	g := &model.Graph{Nodes: []model.Node{}}
	result := Validate(g)
	if result.Valid {
		t.Error("expected invalid graph for empty nodes")
	}
	assertHasError(t, result, "EMPTY_GRAPH")
}

func TestValidate_NoInputNode(t *testing.T) {
	g := &model.Graph{
		Nodes: []model.Node{
			{ID: "n1", Name: "N1", Type: model.NodeTypeOutput, InputPorts: []model.Port{}, OutputPorts: []model.Port{}, Config: json.RawMessage(`{}`)},
		},
	}
	result := Validate(g)
	assertHasError(t, result, "NO_INPUT_NODE")
}

func TestValidate_DuplicateNodeID(t *testing.T) {
	g := &model.Graph{
		Nodes: []model.Node{
			{ID: "n1", Name: "A", Type: model.NodeTypeInput, InputPorts: []model.Port{}, OutputPorts: []model.Port{{Name: "out", Schema: makeSchema("string")}}, Config: json.RawMessage(`{}`)},
			{ID: "n1", Name: "B", Type: model.NodeTypeOutput, InputPorts: []model.Port{}, OutputPorts: []model.Port{}, Config: json.RawMessage(`{}`)},
		},
	}
	result := Validate(g)
	assertHasError(t, result, "DUPLICATE_NODE_ID")
}

func TestValidate_BareObjectSchema(t *testing.T) {
	g := minimalValidGraph()
	g.Nodes[0].OutputPorts[0].Schema = json.RawMessage(`{"type":"object"}`)
	result := Validate(g)
	assertHasError(t, result, "SCHEMA_VIOLATION")
}

func TestValidate_BareArraySchema(t *testing.T) {
	g := minimalValidGraph()
	g.Nodes[0].OutputPorts[0].Schema = json.RawMessage(`{"type":"array"}`)
	result := Validate(g)
	assertHasError(t, result, "SCHEMA_VIOLATION")
}

func TestValidate_ValidObjectSchema(t *testing.T) {
	g := minimalValidGraph()
	g.Nodes[0].OutputPorts[0].Schema = makeObjectSchema(map[string]string{"name": "string"})
	g.Nodes[1].InputPorts[0].Schema = makeObjectSchema(map[string]string{"name": "string"})
	result := Validate(g)
	if !result.Valid {
		t.Errorf("expected valid graph with typed object schema, got: %+v", result.Errors)
	}
}

func TestValidate_InvalidEdgeSourceNode(t *testing.T) {
	g := minimalValidGraph()
	g.Edges[0].SourceNodeID = "nonexistent"
	result := Validate(g)
	assertHasError(t, result, "INVALID_SOURCE_NODE")
}

func TestValidate_InvalidEdgeSourcePort(t *testing.T) {
	g := minimalValidGraph()
	g.Edges[0].SourcePort = "nonexistent"
	result := Validate(g)
	assertHasError(t, result, "INVALID_SOURCE_PORT")
}

func TestValidate_SelfReferencingEdge(t *testing.T) {
	g := minimalValidGraph()
	g.Edges[0].TargetNodeID = g.Edges[0].SourceNodeID
	g.Edges[0].TargetPort = "query"
	result := Validate(g)
	assertHasError(t, result, "SELF_REFERENCE")
}

func TestValidate_BackEdgeWithoutCondition(t *testing.T) {
	g := minimalValidGraph()
	g.Edges = append(g.Edges, model.Edge{
		ID: "e2", SourceNodeID: "output1", SourcePort: "result",
		TargetNodeID: "input1", TargetPort: "query",
		BackEdge: true,
	})
	// Add the port to output1 so the edge is valid
	g.Nodes[1].OutputPorts = append(g.Nodes[1].OutputPorts, model.Port{Name: "result", Schema: makeSchema("string")})
	result := Validate(g)
	assertHasError(t, result, "BACKEDGE_NO_CONDITION")
	assertHasError(t, result, "BACKEDGE_NO_MAX_ITERATIONS")
}

func TestValidate_BackEdgeValid(t *testing.T) {
	g := minimalValidGraph()
	maxIter := 5
	g.Nodes[1].OutputPorts = append(g.Nodes[1].OutputPorts, model.Port{Name: "result", Schema: makeSchema("string")})
	g.Edges = append(g.Edges, model.Edge{
		ID: "e2", SourceNodeID: "output1", SourcePort: "result",
		TargetNodeID: "input1", TargetPort: "query",
		BackEdge: true, Condition: "input.result == 'retry'", MaxIterations: &maxIter,
	})
	result := Validate(g)
	// Should not have back-edge errors (may have other warnings)
	for _, e := range result.Errors {
		if e.Code == "BACKEDGE_NO_CONDITION" || e.Code == "BACKEDGE_NO_MAX_ITERATIONS" {
			t.Errorf("unexpected back-edge error: %s", e.Message)
		}
	}
}

func TestValidate_UnguardedCycle(t *testing.T) {
	g := &model.Graph{
		Nodes: []model.Node{
			{ID: "a", Name: "A", Type: model.NodeTypeInput, InputPorts: []model.Port{{Name: "in", Schema: makeSchema("string")}}, OutputPorts: []model.Port{{Name: "out", Schema: makeSchema("string")}}, Config: json.RawMessage(`{}`)},
			{ID: "b", Name: "B", Type: model.NodeTypeTransform, InputPorts: []model.Port{{Name: "in", Schema: makeSchema("string")}}, OutputPorts: []model.Port{{Name: "out", Schema: makeSchema("string")}}, Config: json.RawMessage(`{}`)},
		},
		Edges: []model.Edge{
			{ID: "e1", SourceNodeID: "a", SourcePort: "out", TargetNodeID: "b", TargetPort: "in"},
			{ID: "e2", SourceNodeID: "b", SourcePort: "out", TargetNodeID: "a", TargetPort: "in"}, // cycle, not marked as back-edge
		},
	}
	result := Validate(g)
	assertHasError(t, result, "UNGUARDED_CYCLE")
}

func TestValidate_UnwiredRequiredPort(t *testing.T) {
	g := &model.Graph{
		Nodes: []model.Node{
			{ID: "in", Name: "In", Type: model.NodeTypeInput, InputPorts: []model.Port{}, OutputPorts: []model.Port{{Name: "out", Schema: makeSchema("string")}}, Config: json.RawMessage(`{}`)},
			{ID: "n1", Name: "N1", Type: model.NodeTypeTransform, InputPorts: []model.Port{
				{Name: "a", Schema: makeSchema("string")},
				{Name: "b", Schema: makeSchema("string")}, // not wired!
			}, OutputPorts: []model.Port{{Name: "out", Schema: makeSchema("string")}}, Config: json.RawMessage(`{}`)},
		},
		Edges: []model.Edge{
			{ID: "e1", SourceNodeID: "in", SourcePort: "out", TargetNodeID: "n1", TargetPort: "a"},
		},
	}
	result := Validate(g)
	assertHasError(t, result, "UNWIRED_REQUIRED_PORT")
}

func TestValidate_OptionalPortNotWired(t *testing.T) {
	g := &model.Graph{
		Nodes: []model.Node{
			{ID: "in", Name: "In", Type: model.NodeTypeInput, InputPorts: []model.Port{}, OutputPorts: []model.Port{{Name: "out", Schema: makeSchema("string")}}, Config: json.RawMessage(`{}`)},
			{ID: "n1", Name: "N1", Type: model.NodeTypeTransform, InputPorts: []model.Port{
				{Name: "a", Schema: makeSchema("string")},
				{Name: "b", Schema: makeSchema("string"), Required: boolPtr(false)}, // optional
			}, OutputPorts: []model.Port{{Name: "out", Schema: makeSchema("string")}}, Config: json.RawMessage(`{}`)},
		},
		Edges: []model.Edge{
			{ID: "e1", SourceNodeID: "in", SourcePort: "out", TargetNodeID: "n1", TargetPort: "a"},
		},
	}
	result := Validate(g)
	// Should not have unwired port error for optional port
	for _, e := range result.Errors {
		if e.Code == "UNWIRED_REQUIRED_PORT" && e.NodeID == "n1" {
			t.Errorf("unexpected error for optional port: %s", e.Message)
		}
	}
}

func TestValidate_StateReducerIncompatible(t *testing.T) {
	g := minimalValidGraph()
	g.State = &model.GraphState{
		Fields: []model.StateField{
			{Name: "counter", Schema: makeSchema("integer"), Reducer: model.ReducerAppend}, // append on non-array
		},
	}
	result := Validate(g)
	assertHasError(t, result, "REDUCER_INCOMPATIBLE")
}

func TestValidate_StateReducerCompatible(t *testing.T) {
	g := minimalValidGraph()
	g.State = &model.GraphState{
		Fields: []model.StateField{
			{Name: "items", Schema: makeArraySchema("string"), Reducer: model.ReducerAppend},
			{Name: "data", Schema: makeObjectSchema(map[string]string{"key": "string"}), Reducer: model.ReducerMerge},
			{Name: "count", Schema: makeSchema("integer"), Reducer: model.ReducerReplace},
		},
	}
	result := Validate(g)
	for _, e := range result.Errors {
		if e.Code == "REDUCER_INCOMPATIBLE" {
			t.Errorf("unexpected reducer error: %s", e.Message)
		}
	}
}

func TestValidate_InvalidStateBinding(t *testing.T) {
	g := minimalValidGraph()
	g.Nodes[1].StateReads = []model.StateBinding{
		{StateField: "nonexistent", Port: "result"},
	}
	result := Validate(g)
	assertHasError(t, result, "INVALID_STATE_REF")
}

func TestTopologicalSort_Simple(t *testing.T) {
	g := minimalValidGraph()
	order, err := TopologicalSort(g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(order))
	}
	if order[0] != "input1" || order[1] != "output1" {
		t.Errorf("expected [input1, output1], got %v", order)
	}
}

func TestParallelGroups_ForkJoin(t *testing.T) {
	g := &model.Graph{
		Nodes: []model.Node{
			{ID: "in", Name: "In", Type: model.NodeTypeInput},
			{ID: "a", Name: "A", Type: model.NodeTypeTransform},
			{ID: "b", Name: "B", Type: model.NodeTypeTransform},
			{ID: "out", Name: "Out", Type: model.NodeTypeOutput},
		},
		Edges: []model.Edge{
			{ID: "e1", SourceNodeID: "in", SourcePort: "o", TargetNodeID: "a", TargetPort: "i"},
			{ID: "e2", SourceNodeID: "in", SourcePort: "o", TargetNodeID: "b", TargetPort: "i"},
			{ID: "e3", SourceNodeID: "a", SourcePort: "o", TargetNodeID: "out", TargetPort: "i1"},
			{ID: "e4", SourceNodeID: "b", SourcePort: "o", TargetNodeID: "out", TargetPort: "i2"},
		},
	}
	groups, err := ParallelGroups(g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should be 3 groups: [in], [a, b], [out]
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d: %v", len(groups), groups)
	}
	if len(groups[1]) != 2 {
		t.Errorf("expected 2 parallel nodes in group 1, got %d", len(groups[1]))
	}
}

func assertHasError(t *testing.T, result *ValidationResult, code string) {
	t.Helper()
	for _, e := range result.Errors {
		if e.Code == code {
			return
		}
	}
	t.Errorf("expected error with code %q, got: %+v", code, result.Errors)
}

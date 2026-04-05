package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"

	"github.com/brockleyai/brockleyai/internal/model"
)

// makeInnerTransformGraph builds a simple inner graph that takes an "item" input,
// applies a transform expression, and outputs the result.
// The expression is evaluated against input.item.
func makeInnerTransformGraph(expression string) json.RawMessage {
	g := model.Graph{
		ID:   "inner-graph",
		Name: "inner",
		Nodes: []model.Node{
			{
				ID:   "inner-input",
				Name: "input",
				Type: model.NodeTypeInput,
				OutputPorts: []model.Port{
					{Name: "item", Schema: json.RawMessage(`{"type":"string"}`)},
					{Name: "index", Schema: json.RawMessage(`{"type":"integer"}`)},
					{Name: "context", Schema: json.RawMessage(`{"type":"string"}`)},
				},
			},
			{
				ID:   "inner-transform",
				Name: "transform",
				Type: model.NodeTypeTransform,
				InputPorts: []model.Port{
					{Name: "item", Schema: json.RawMessage(`{"type":"string"}`)},
				},
				OutputPorts: []model.Port{
					{Name: "result", Schema: json.RawMessage(`{"type":"string"}`)},
				},
				Config: mustJSON(model.TransformNodeConfig{
					Expressions: map[string]string{
						"result": expression,
					},
				}),
			},
			{
				ID:   "inner-output",
				Name: "output",
				Type: model.NodeTypeOutput,
				InputPorts: []model.Port{
					{Name: "result", Schema: json.RawMessage(`{"type":"string"}`)},
				},
			},
		},
		Edges: []model.Edge{
			{
				ID:           "e1",
				SourceNodeID: "inner-input",
				SourcePort:   "item",
				TargetNodeID: "inner-transform",
				TargetPort:   "item",
			},
			{
				ID:           "e2",
				SourceNodeID: "inner-transform",
				SourcePort:   "result",
				TargetNodeID: "inner-output",
				TargetPort:   "result",
			},
		},
	}
	return mustJSON(g)
}

func setupInnerGraphRunner(t *testing.T) {
	t.Helper()
	// We need to set the inner graph runner for foreach/subgraph tests.
	// Since the executor package can't import orchestrator (circular dep),
	// we set up a simple runner here that handles basic transform-only graphs.
	SetInnerGraphRunner(func(ctx context.Context, g *model.Graph, inputs map[string]any, deps *ExecutorDeps, executors *Registry, logger *slog.Logger) (map[string]any, error) {
		// Build node map.
		nodeMap := make(map[string]*model.Node)
		for i := range g.Nodes {
			nodeMap[g.Nodes[i].ID] = &g.Nodes[i]
		}

		// Build edge maps.
		outEdges := make(map[string][]model.Edge)
		inEdges := make(map[string][]model.Edge)
		for _, edge := range g.Edges {
			outEdges[edge.SourceNodeID] = append(outEdges[edge.SourceNodeID], edge)
			inEdges[edge.TargetNodeID] = append(inEdges[edge.TargetNodeID], edge)
		}

		// Set input node outputs.
		nodeOutputs := make(map[string]map[string]any)
		for i := range g.Nodes {
			if g.Nodes[i].Type == model.NodeTypeInput {
				nodeOutputs[g.Nodes[i].ID] = inputs
			}
		}

		// Simple topological execution (no parallelism needed for tests).
		// Process nodes in order: input -> transform -> output.
		processed := make(map[string]bool)
		for id := range nodeOutputs {
			processed[id] = true
		}

		maxIter := len(g.Nodes) * 2
		for iter := 0; iter < maxIter; iter++ {
			progress := false
			for _, node := range g.Nodes {
				if processed[node.ID] {
					continue
				}
				// Check all deps are processed.
				allReady := true
				for _, edge := range inEdges[node.ID] {
					if !processed[edge.SourceNodeID] {
						allReady = false
						break
					}
				}
				if !allReady {
					continue
				}

				// Resolve inputs from edges.
				resolvedInputs := make(map[string]any)
				for _, edge := range inEdges[node.ID] {
					srcOutputs := nodeOutputs[edge.SourceNodeID]
					if srcOutputs != nil {
						if val, ok := srcOutputs[edge.SourcePort]; ok {
							resolvedInputs[edge.TargetPort] = val
						}
					}
				}

				// Execute.
				exec, err := executors.Get(node.Type)
				if err != nil {
					return nil, fmt.Errorf("inner graph runner: %w", err)
				}
				result, err := exec.Execute(ctx, &node, resolvedInputs, nil, deps)
				if err != nil {
					return nil, fmt.Errorf("inner graph runner: node %s failed: %w", node.ID, err)
				}
				if result != nil {
					nodeOutputs[node.ID] = result.Outputs
				}
				processed[node.ID] = true
				progress = true
			}
			if !progress {
				break
			}
		}

		// Collect output node results.
		outputs := make(map[string]any)
		for _, node := range g.Nodes {
			if node.Type == model.NodeTypeOutput {
				if out, ok := nodeOutputs[node.ID]; ok {
					for k, v := range out {
						outputs[k] = v
					}
				}
			}
		}

		return outputs, nil
	})

	t.Cleanup(func() {
		SetInnerGraphRunner(nil)
	})
}

func TestForEachExecutor_BasicIteration(t *testing.T) {
	setupInnerGraphRunner(t)

	cfg := model.ForEachNodeConfig{
		Graph:       makeInnerTransformGraph("input.item | upper"),
		Concurrency: 0,
	}

	node := &model.Node{
		ID:     "foreach-1",
		Name:   "test-foreach",
		Type:   model.NodeTypeForEach,
		Config: mustJSON(cfg),
	}

	inputs := map[string]any{
		"items": []any{"hello", "world", "test"},
	}

	deps := &ExecutorDeps{
		Logger: slog.Default(),
	}

	exec := &ForEachExecutor{}
	result, err := exec.Execute(context.Background(), node, inputs, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	results, ok := result.Outputs["results"].([]any)
	if !ok {
		t.Fatalf("expected results to be []any, got %T", result.Outputs["results"])
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Each result is a map with "result" key from the inner graph's output.
	expected := []string{"HELLO", "WORLD", "TEST"}
	for i, r := range results {
		m, ok := r.(map[string]any)
		if !ok {
			t.Fatalf("result[%d]: expected map[string]any, got %T", i, r)
		}
		if m["result"] != expected[i] {
			t.Errorf("result[%d]: expected %q, got %v", i, expected[i], m["result"])
		}
	}

	errors, ok := result.Outputs["errors"].([]any)
	if !ok {
		t.Fatalf("expected errors to be []any, got %T", result.Outputs["errors"])
	}
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errors))
	}
}

func TestForEachExecutor_SequentialConcurrency(t *testing.T) {
	setupInnerGraphRunner(t)

	cfg := model.ForEachNodeConfig{
		Graph:       makeInnerTransformGraph("input.item | upper"),
		Concurrency: 1,
	}

	node := &model.Node{
		ID:     "foreach-seq",
		Name:   "test-foreach-seq",
		Type:   model.NodeTypeForEach,
		Config: mustJSON(cfg),
	}

	inputs := map[string]any{
		"items": []any{"alpha", "beta", "gamma"},
	}

	deps := &ExecutorDeps{
		Logger: slog.Default(),
	}

	exec := &ForEachExecutor{}
	result, err := exec.Execute(context.Background(), node, inputs, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	results, ok := result.Outputs["results"].([]any)
	if !ok {
		t.Fatalf("expected results to be []any, got %T", result.Outputs["results"])
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	expected := []string{"ALPHA", "BETA", "GAMMA"}
	for i, r := range results {
		m, ok := r.(map[string]any)
		if !ok {
			t.Fatalf("result[%d]: expected map[string]any, got %T", i, r)
		}
		if m["result"] != expected[i] {
			t.Errorf("result[%d]: expected %q, got %v", i, expected[i], m["result"])
		}
	}
}

func TestForEachExecutor_OnItemErrorContinue(t *testing.T) {
	// Set up a runner that fails for a specific item.
	SetInnerGraphRunner(func(ctx context.Context, g *model.Graph, inputs map[string]any, deps *ExecutorDeps, executors *Registry, logger *slog.Logger) (map[string]any, error) {
		item := inputs["item"]
		if item == "fail_me" {
			return nil, fmt.Errorf("intentional failure for item")
		}
		// For other items, return the item uppercased (simulate transform).
		s, _ := item.(string)
		return map[string]any{"result": s + "_OK"}, nil
	})
	t.Cleanup(func() {
		SetInnerGraphRunner(nil)
	})

	cfg := model.ForEachNodeConfig{
		Graph:       makeInnerTransformGraph("input.item"), // doesn't matter, runner is mocked
		OnItemError: "continue",
	}

	node := &model.Node{
		ID:     "foreach-err",
		Name:   "test-foreach-err",
		Type:   model.NodeTypeForEach,
		Config: mustJSON(cfg),
	}

	inputs := map[string]any{
		"items": []any{"good", "fail_me", "also_good"},
	}

	deps := &ExecutorDeps{
		Logger: slog.Default(),
	}

	exec := &ForEachExecutor{}
	result, err := exec.Execute(context.Background(), node, inputs, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error (should continue on item error): %v", err)
	}

	results, ok := result.Outputs["results"].([]any)
	if !ok {
		t.Fatalf("expected results to be []any, got %T", result.Outputs["results"])
	}
	// Two items succeeded.
	if len(results) != 2 {
		t.Fatalf("expected 2 successful results, got %d: %v", len(results), results)
	}

	errors, ok := result.Outputs["errors"].([]any)
	if !ok {
		t.Fatalf("expected errors to be []any, got %T", result.Outputs["errors"])
	}
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}

	errObj, ok := errors[0].(map[string]any)
	if !ok {
		t.Fatalf("expected error object to be map[string]any, got %T", errors[0])
	}
	if errObj["item"] != "fail_me" {
		t.Errorf("expected error item to be 'fail_me', got %v", errObj["item"])
	}
}

func TestSubgraphExecutor(t *testing.T) {
	setupInnerGraphRunner(t)

	innerGraph := makeInnerTransformGraph("input.item | upper")

	cfg := model.SubgraphNodeConfig{
		Graph: innerGraph,
		PortMapping: model.PortMapping{
			Inputs: map[string]string{
				"text": "item",
			},
			Outputs: map[string]string{
				"result": "transformed",
			},
		},
	}

	node := &model.Node{
		ID:     "subgraph-1",
		Name:   "test-subgraph",
		Type:   model.NodeTypeSubgraph,
		Config: mustJSON(cfg),
	}

	inputs := map[string]any{
		"text": "hello",
	}

	deps := &ExecutorDeps{
		Logger: slog.Default(),
	}

	exec := &SubgraphExecutor{}
	result, err := exec.Execute(context.Background(), node, inputs, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	transformed, ok := result.Outputs["transformed"]
	if !ok {
		t.Fatalf("expected output port 'transformed', got outputs: %v", result.Outputs)
	}
	if transformed != "HELLO" {
		t.Errorf("expected 'HELLO', got %v", transformed)
	}
}

package orchestrator

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/brockleyai/brockleyai/engine/executor"
	"github.com/brockleyai/brockleyai/engine/mock"
	"github.com/brockleyai/brockleyai/internal/model"
)

// counterExecutor increments the "count" input and outputs the new count.
type counterExecutor struct {
	callCount int
}

func (e *counterExecutor) Execute(_ context.Context, node *model.Node, inputs map[string]any, _ *executor.NodeContext, _ *executor.ExecutorDeps) (*executor.NodeResult, error) {
	e.callCount++
	count := 0
	if c, ok := inputs["count"]; ok {
		switch v := c.(type) {
		case int:
			count = v
		case int64:
			count = int(v)
		case float64:
			count = int(v)
		}
	}
	count++
	return &executor.NodeResult{
		Outputs: map[string]any{
			"count": count,
		},
	}, nil
}

// evaluatorExecutor passes through all inputs unchanged. Used as a checkpoint
// in the loop to separate counter from the back-edge source.
type evaluatorExecutor struct {
	callCount int
}

func (e *evaluatorExecutor) Execute(_ context.Context, node *model.Node, inputs map[string]any, _ *executor.NodeContext, _ *executor.ExecutorDeps) (*executor.NodeResult, error) {
	e.callCount++
	outputs := make(map[string]any, len(inputs))
	for k, v := range inputs {
		outputs[k] = v
	}
	return &executor.NodeResult{Outputs: outputs}, nil
}

// passthroughExec passes all inputs to outputs.
type passthroughExec struct{}

func (e *passthroughExec) Execute(_ context.Context, _ *model.Node, inputs map[string]any, _ *executor.NodeContext, _ *executor.ExecutorDeps) (*executor.NodeResult, error) {
	outputs := make(map[string]any, len(inputs))
	for k, v := range inputs {
		outputs[k] = v
	}
	return &executor.NodeResult{Outputs: outputs}, nil
}

func makeLoopGraph(maxIter int) *model.Graph {
	// Graph: input -> counter -> evaluator -> output
	// Back-edge: evaluator -> counter (condition: input.count < 3, max_iterations)
	return &model.Graph{
		ID:   "loop-test",
		Name: "loop-test",
		Nodes: []model.Node{
			{
				ID:   "input-1",
				Name: "input",
				Type: model.NodeTypeInput,
				OutputPorts: []model.Port{
					{Name: "count", Schema: json.RawMessage(`{"type":"integer"}`)},
				},
			},
			{
				ID:   "counter-1",
				Name: "counter",
				Type: "counter",
				InputPorts: []model.Port{
					{Name: "count", Schema: json.RawMessage(`{"type":"integer"}`)},
				},
				OutputPorts: []model.Port{
					{Name: "count", Schema: json.RawMessage(`{"type":"integer"}`)},
				},
				Config: json.RawMessage(`{}`),
			},
			{
				ID:   "eval-1",
				Name: "evaluator",
				Type: "evaluator",
				InputPorts: []model.Port{
					{Name: "count", Schema: json.RawMessage(`{"type":"integer"}`)},
				},
				OutputPorts: []model.Port{
					{Name: "count", Schema: json.RawMessage(`{"type":"integer"}`)},
				},
				Config: json.RawMessage(`{}`),
			},
			{
				ID:   "output-1",
				Name: "output",
				Type: model.NodeTypeOutput,
				InputPorts: []model.Port{
					{Name: "count", Schema: json.RawMessage(`{"type":"integer"}`)},
				},
			},
		},
		Edges: []model.Edge{
			{
				ID:           "e1",
				SourceNodeID: "input-1",
				SourcePort:   "count",
				TargetNodeID: "counter-1",
				TargetPort:   "count",
			},
			{
				ID:           "e2",
				SourceNodeID: "counter-1",
				SourcePort:   "count",
				TargetNodeID: "eval-1",
				TargetPort:   "count",
			},
			{
				ID:           "e3",
				SourceNodeID: "eval-1",
				SourcePort:   "count",
				TargetNodeID: "output-1",
				TargetPort:   "count",
			},
			{
				// Back-edge: evaluator loops back to counter.
				ID:            "back-1",
				SourceNodeID:  "eval-1",
				SourcePort:    "count",
				TargetNodeID:  "counter-1",
				TargetPort:    "count",
				BackEdge:      true,
				Condition:     "input.count < 3",
				MaxIterations: &maxIter,
			},
		},
	}
}

func makeTestRegistry(counter *counterExecutor, eval *evaluatorExecutor) *executor.Registry {
	reg := executor.NewRegistry()
	reg.Register(model.NodeTypeInput, &passthroughExec{})
	reg.Register(model.NodeTypeOutput, &passthroughExec{})
	reg.Register("counter", counter)
	reg.Register("evaluator", eval)
	return reg
}

func TestBackEdgeLoop(t *testing.T) {
	maxIter := 10
	g := makeLoopGraph(maxIter)

	counter := &counterExecutor{}
	eval := &evaluatorExecutor{}
	reg := makeTestRegistry(counter, eval)

	emitter := &mock.MockEventEmitter{}
	metrics := &noopMetrics{}
	logger := slog.Default()

	orch := New(reg, emitter, metrics, logger)

	inputs := map[string]any{"count": 0}
	deps := &executor.ExecutorDeps{
		EventEmitter: emitter,
		Logger:       logger,
	}

	result, err := orch.Execute(context.Background(), g, inputs, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "completed" {
		t.Fatalf("expected status 'completed', got %q", result.Status)
	}

	// Initial execution: counter(0)->1, eval passes 1.
	// Loop iter 1: condition 1<3=true -> re-exec counter(1)->2, eval passes 2.
	// Loop iter 2: condition 2<3=true -> re-exec counter(2)->3, eval passes 3.
	// Loop iter 3: condition 3<3=false -> stop.
	// Total counter calls: 3, eval calls: 3.
	if counter.callCount != 3 {
		t.Errorf("expected counter to be called 3 times, got %d", counter.callCount)
	}

	// Output should be the final count.
	outCount, ok := result.Outputs["count"]
	if !ok {
		t.Fatal("expected output 'count'")
	}
	if outCount != 3 {
		t.Errorf("expected final count 3, got %v", outCount)
	}
}

func TestBackEdgeLoop_MaxIterations(t *testing.T) {
	maxIter := 2
	g := makeLoopGraph(maxIter)
	// Override condition to always be true.
	for i := range g.Edges {
		if g.Edges[i].BackEdge {
			g.Edges[i].Condition = "true"
		}
	}

	counter := &counterExecutor{}
	eval := &evaluatorExecutor{}
	reg := makeTestRegistry(counter, eval)

	emitter := &mock.MockEventEmitter{}
	metrics := &noopMetrics{}
	logger := slog.Default()

	orch := New(reg, emitter, metrics, logger)

	inputs := map[string]any{"count": 0}
	deps := &executor.ExecutorDeps{
		EventEmitter: emitter,
		Logger:       logger,
	}

	result, err := orch.Execute(context.Background(), g, inputs, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Initial execution + 2 loop iterations = 3 total counter calls.
	if counter.callCount != 3 {
		t.Errorf("expected counter to be called 3 times (1 initial + 2 max_iterations), got %d", counter.callCount)
	}

	// Verify iteration count was tracked.
	if ic, ok := result.IterationCounts["back-1"]; ok {
		if ic != 2 {
			t.Errorf("expected iteration count 2 for back-edge, got %d", ic)
		}
	} else {
		t.Error("expected iteration count for back-edge 'back-1'")
	}
}

func TestBackEdgeLoop_StateAccumulation(t *testing.T) {
	maxIter := 5
	g := makeLoopGraph(maxIter)
	g.ID = "state-loop-test"
	g.Name = "state-loop-test"

	// Add state schema.
	g.State = &model.GraphState{
		Fields: []model.StateField{
			{
				Name:    "history",
				Schema:  json.RawMessage(`{"type":"array","items":{"type":"integer"}}`),
				Reducer: model.ReducerAppend,
				Initial: json.RawMessage(`[]`),
			},
		},
	}

	// Add state write on the counter node.
	for i := range g.Nodes {
		if g.Nodes[i].ID == "counter-1" {
			g.Nodes[i].StateWrites = []model.StateBinding{
				{StateField: "history", Port: "count"},
			}
		}
	}

	counter := &counterExecutor{}
	eval := &evaluatorExecutor{}
	reg := makeTestRegistry(counter, eval)

	emitter := &mock.MockEventEmitter{}
	metrics := &noopMetrics{}
	logger := slog.Default()

	orch := New(reg, emitter, metrics, logger)

	inputs := map[string]any{"count": 0}
	deps := &executor.ExecutorDeps{
		EventEmitter: emitter,
		Logger:       logger,
	}

	result, err := orch.Execute(context.Background(), g, inputs, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check state accumulated history.
	history, ok := result.State["history"].([]any)
	if !ok {
		t.Fatalf("expected state 'history' to be []any, got %T: %v", result.State["history"], result.State["history"])
	}
	// Counter runs 3 times: produces 1, 2, 3.
	if len(history) != 3 {
		t.Errorf("expected 3 history entries, got %d: %v", len(history), history)
	}
}

func TestEvalCondition_WithState(t *testing.T) {
	tests := []struct {
		name      string
		expr      string
		inputs    map[string]any
		state     map[string]any
		wantMatch bool
	}{
		{
			name:      "state.x > 5 with x=10",
			expr:      "state.x > 5",
			inputs:    map[string]any{},
			state:     map[string]any{"x": int64(10)},
			wantMatch: true,
		},
		{
			name:      "state.x > 5 with x=2",
			expr:      "state.x > 5",
			inputs:    map[string]any{},
			state:     map[string]any{"x": int64(2)},
			wantMatch: false,
		},
		{
			name:      "mixed input and state",
			expr:      "input.val + state.offset > 10",
			inputs:    map[string]any{"val": int64(5)},
			state:     map[string]any{"offset": int64(7)},
			wantMatch: true,
		},
		{
			name:      "nil state falls back gracefully",
			expr:      "input.val > 0",
			inputs:    map[string]any{"val": int64(1)},
			state:     nil,
			wantMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := EvalCondition(tt.expr, tt.inputs, tt.state)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.wantMatch {
				t.Errorf("expected %v, got %v", tt.wantMatch, result)
			}
		})
	}
}

package orchestrator

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/brockleyai/brockleyai/engine/executor"
	"github.com/brockleyai/brockleyai/engine/mock"
	"github.com/brockleyai/brockleyai/internal/model"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func testDeps(provider *mock.MockLLMProvider) *executor.ExecutorDeps {
	reg := &mockProviderRegistry{provider: provider}
	secrets := mock.NewMockSecretStore()
	secrets.Secrets["test-key"] = "sk-test"
	return &executor.ExecutorDeps{
		ProviderRegistry: reg,
		SecretStore:      secrets,
		EventEmitter:     &mock.MockEventEmitter{},
		Logger:           testLogger(),
	}
}

type mockProviderRegistry struct {
	provider model.LLMProvider
}

func (r *mockProviderRegistry) Get(name string) (model.LLMProvider, error) {
	return r.provider, nil
}

func setupExecutors() *executor.Registry {
	reg := executor.NewRegistry()
	reg.Register(model.NodeTypeInput, &executor.InputExecutor{})
	reg.Register(model.NodeTypeOutput, &executor.OutputExecutor{})
	reg.Register(model.NodeTypeLLM, &executor.LLMExecutor{})
	reg.Register(model.NodeTypeConditional, &executor.ConditionalExecutor{})
	reg.Register(model.NodeTypeTransform, &executor.TransformExecutor{})
	return reg
}

func js(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func TestExecute_SimpleLinear(t *testing.T) {
	// input -> llm -> output
	mockLLM := &mock.MockLLMProvider{
		Responses: []string{"Hello from LLM!"},
	}
	deps := testDeps(mockLLM)
	executors := setupExecutors()

	g := &model.Graph{
		ID: "g1", Name: "test", Namespace: "default", TenantID: "default",
		Nodes: []model.Node{
			{
				ID: "in", Name: "Input", Type: model.NodeTypeInput,
				InputPorts:  []model.Port{},
				OutputPorts: []model.Port{{Name: "query", Schema: js(map[string]string{"type": "string"})}},
				Config:      js(nil),
			},
			{
				ID: "llm1", Name: "LLM", Type: model.NodeTypeLLM,
				InputPorts:  []model.Port{{Name: "query", Schema: js(map[string]string{"type": "string"})}},
				OutputPorts: []model.Port{{Name: "response_text", Schema: js(map[string]string{"type": "string"})}},
				Config: js(model.LLMNodeConfig{
					Provider: "anthropic", Model: "test", APIKeyRef: "test-key",
					UserPrompt: "{{input.query}}", Variables: []model.TemplateVar{{Name: "query", Schema: js(map[string]string{"type": "string"})}},
					ResponseFormat: model.ResponseFormatText,
				}),
			},
			{
				ID: "out", Name: "Output", Type: model.NodeTypeOutput,
				InputPorts:  []model.Port{{Name: "result", Schema: js(map[string]string{"type": "string"})}},
				OutputPorts: []model.Port{},
				Config:      js(nil),
			},
		},
		Edges: []model.Edge{
			{ID: "e1", SourceNodeID: "in", SourcePort: "query", TargetNodeID: "llm1", TargetPort: "query"},
			{ID: "e2", SourceNodeID: "llm1", SourcePort: "response_text", TargetNodeID: "out", TargetPort: "result"},
		},
	}

	result, err := Execute(context.Background(), g, map[string]any{"query": "Hi"}, deps, executors, testLogger())
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Status != "completed" {
		t.Errorf("expected completed, got %s", result.Status)
	}
	if result.Outputs["result"] != "Hello from LLM!" {
		t.Errorf("expected 'Hello from LLM!', got %v", result.Outputs["result"])
	}
	if len(mockLLM.Calls) != 1 {
		t.Errorf("expected 1 LLM call, got %d", len(mockLLM.Calls))
	}
}

func TestExecute_ConditionalBranch(t *testing.T) {
	// input -> conditional -> [branch_a -> output, branch_b -> output]
	executors := setupExecutors()
	deps := testDeps(&mock.MockLLMProvider{})

	g := &model.Graph{
		ID: "g2", Name: "cond-test", Namespace: "default", TenantID: "default",
		Nodes: []model.Node{
			{
				ID: "in", Name: "Input", Type: model.NodeTypeInput,
				InputPorts:  []model.Port{},
				OutputPorts: []model.Port{{Name: "value", Schema: js(map[string]any{"type": "object", "properties": map[string]any{"category": map[string]string{"type": "string"}}, "required": []string{"category"}})}},
				Config:      js(nil),
			},
			{
				ID: "cond", Name: "Route", Type: model.NodeTypeConditional,
				InputPorts: []model.Port{{Name: "value", Schema: js(map[string]any{"type": "object", "properties": map[string]any{"category": map[string]string{"type": "string"}}, "required": []string{"category"}})}},
				OutputPorts: []model.Port{
					{Name: "branch_a", Schema: js(map[string]any{"type": "object", "properties": map[string]any{"category": map[string]string{"type": "string"}}, "required": []string{"category"}})},
					{Name: "branch_b", Schema: js(map[string]any{"type": "object", "properties": map[string]any{"category": map[string]string{"type": "string"}}, "required": []string{"category"}})},
				},
				Config: js(model.ConditionalNodeConfig{
					Branches: []model.Branch{
						{Label: "branch_a", Condition: `input.value.category == "billing"`},
					},
					DefaultLabel: "branch_b",
				}),
			},
			{
				ID: "handle_a", Name: "Handle A", Type: model.NodeTypeTransform,
				InputPorts:  []model.Port{{Name: "data", Schema: js(map[string]any{"type": "object", "properties": map[string]any{"category": map[string]string{"type": "string"}}, "required": []string{"category"}})}},
				OutputPorts: []model.Port{{Name: "result", Schema: js(map[string]string{"type": "string"})}},
				Config:      js(model.TransformNodeConfig{Expressions: map[string]string{"result": `"handled_a"`}}),
			},
			{
				ID: "handle_b", Name: "Handle B", Type: model.NodeTypeTransform,
				InputPorts:  []model.Port{{Name: "data", Schema: js(map[string]any{"type": "object", "properties": map[string]any{"category": map[string]string{"type": "string"}}, "required": []string{"category"}})}},
				OutputPorts: []model.Port{{Name: "result", Schema: js(map[string]string{"type": "string"})}},
				Config:      js(model.TransformNodeConfig{Expressions: map[string]string{"result": `"handled_b"`}}),
			},
			{
				ID: "out", Name: "Output", Type: model.NodeTypeOutput,
				InputPorts:  []model.Port{{Name: "result", Schema: js(map[string]string{"type": "string"}), Required: boolPtr(false)}},
				OutputPorts: []model.Port{},
				Config:      js(nil),
			},
		},
		Edges: []model.Edge{
			{ID: "e1", SourceNodeID: "in", SourcePort: "value", TargetNodeID: "cond", TargetPort: "value"},
			{ID: "e2", SourceNodeID: "cond", SourcePort: "branch_a", TargetNodeID: "handle_a", TargetPort: "data"},
			{ID: "e3", SourceNodeID: "cond", SourcePort: "branch_b", TargetNodeID: "handle_b", TargetPort: "data"},
			{ID: "e4", SourceNodeID: "handle_a", SourcePort: "result", TargetNodeID: "out", TargetPort: "result"},
		},
		// Note: handle_b -> out edge omitted for simplicity; handle_b result would go to a different output in a real graph
	}

	result, err := Execute(context.Background(), g, map[string]any{"value": map[string]any{"category": "billing"}}, deps, executors, testLogger())
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Status != "completed" {
		t.Errorf("expected completed, got %s", result.Status)
	}

	// branch_a should have been taken (billing matches)
	if result.Outputs["result"] != "handled_a" {
		t.Errorf("expected 'handled_a', got %v", result.Outputs["result"])
	}

	// Verify handle_b was skipped
	foundSkipped := false
	for _, step := range result.Steps {
		if step.NodeID == "handle_b" && step.Status == "skipped" {
			foundSkipped = true
		}
	}
	if !foundSkipped {
		t.Error("expected handle_b to be skipped")
	}
}

func TestExecute_ForkJoin(t *testing.T) {
	// input -> [transform_a, transform_b] -> output (both must complete)
	executors := setupExecutors()
	deps := testDeps(&mock.MockLLMProvider{})

	g := &model.Graph{
		ID: "g3", Name: "fork-join", Namespace: "default", TenantID: "default",
		Nodes: []model.Node{
			{
				ID: "in", Name: "Input", Type: model.NodeTypeInput,
				InputPorts:  []model.Port{},
				OutputPorts: []model.Port{{Name: "text", Schema: js(map[string]string{"type": "string"})}},
				Config:      js(nil),
			},
			{
				ID: "upper", Name: "Upper", Type: model.NodeTypeTransform,
				InputPorts:  []model.Port{{Name: "text", Schema: js(map[string]string{"type": "string"})}},
				OutputPorts: []model.Port{{Name: "result", Schema: js(map[string]string{"type": "string"})}},
				Config:      js(model.TransformNodeConfig{Expressions: map[string]string{"result": `input.text | upper`}}),
			},
			{
				ID: "length", Name: "Length", Type: model.NodeTypeTransform,
				InputPorts:  []model.Port{{Name: "text", Schema: js(map[string]string{"type": "string"})}},
				OutputPorts: []model.Port{{Name: "result", Schema: js(map[string]string{"type": "integer"})}},
				Config:      js(model.TransformNodeConfig{Expressions: map[string]string{"result": `input.text | length`}}),
			},
			{
				ID: "out", Name: "Output", Type: model.NodeTypeOutput,
				InputPorts: []model.Port{
					{Name: "upper", Schema: js(map[string]string{"type": "string"})},
					{Name: "len", Schema: js(map[string]string{"type": "integer"})},
				},
				OutputPorts: []model.Port{},
				Config:      js(nil),
			},
		},
		Edges: []model.Edge{
			{ID: "e1", SourceNodeID: "in", SourcePort: "text", TargetNodeID: "upper", TargetPort: "text"},
			{ID: "e2", SourceNodeID: "in", SourcePort: "text", TargetNodeID: "length", TargetPort: "text"},
			{ID: "e3", SourceNodeID: "upper", SourcePort: "result", TargetNodeID: "out", TargetPort: "upper"},
			{ID: "e4", SourceNodeID: "length", SourcePort: "result", TargetNodeID: "out", TargetPort: "len"},
		},
	}

	result, err := Execute(context.Background(), g, map[string]any{"text": "hello"}, deps, executors, testLogger())
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Outputs["upper"] != "HELLO" {
		t.Errorf("expected 'HELLO', got %v", result.Outputs["upper"])
	}
	// Length could be int or float depending on expression engine
	lenVal := result.Outputs["len"]
	switch v := lenVal.(type) {
	case int:
		if v != 5 {
			t.Errorf("expected length 5, got %d", v)
		}
	case int64:
		if v != 5 {
			t.Errorf("expected length 5, got %d", v)
		}
	case float64:
		if v != 5 {
			t.Errorf("expected length 5, got %f", v)
		}
	default:
		t.Errorf("unexpected length type %T: %v", lenVal, lenVal)
	}
}

func TestExecute_TransformExpressions(t *testing.T) {
	executors := setupExecutors()
	deps := testDeps(&mock.MockLLMProvider{})

	g := &model.Graph{
		ID: "g4", Name: "transform-test", Namespace: "default", TenantID: "default",
		Nodes: []model.Node{
			{
				ID: "in", Name: "Input", Type: model.NodeTypeInput,
				InputPorts: []model.Port{},
				OutputPorts: []model.Port{
					{Name: "items", Schema: js(map[string]any{"type": "array", "items": map[string]string{"type": "integer"}})},
				},
				Config: js(nil),
			},
			{
				ID: "t1", Name: "Aggregate", Type: model.NodeTypeTransform,
				InputPorts: []model.Port{
					{Name: "items", Schema: js(map[string]any{"type": "array", "items": map[string]string{"type": "integer"}})},
				},
				OutputPorts: []model.Port{
					{Name: "total", Schema: js(map[string]string{"type": "number"})},
					{Name: "count", Schema: js(map[string]string{"type": "integer"})},
				},
				Config: js(model.TransformNodeConfig{Expressions: map[string]string{
					"total": "input.items | sum",
					"count": "input.items | length",
				}}),
			},
			{
				ID: "out", Name: "Output", Type: model.NodeTypeOutput,
				InputPorts: []model.Port{
					{Name: "total", Schema: js(map[string]string{"type": "number"})},
					{Name: "count", Schema: js(map[string]string{"type": "integer"})},
				},
				OutputPorts: []model.Port{},
				Config:      js(nil),
			},
		},
		Edges: []model.Edge{
			{ID: "e1", SourceNodeID: "in", SourcePort: "items", TargetNodeID: "t1", TargetPort: "items"},
			{ID: "e2", SourceNodeID: "t1", SourcePort: "total", TargetNodeID: "out", TargetPort: "total"},
			{ID: "e3", SourceNodeID: "t1", SourcePort: "count", TargetNodeID: "out", TargetPort: "count"},
		},
	}

	result, err := Execute(context.Background(), g, map[string]any{"items": []any{1, 2, 3, 4, 5}}, deps, executors, testLogger())
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Sum of 1+2+3+4+5 = 15
	total := result.Outputs["total"]
	switch v := total.(type) {
	case float64:
		if v != 15 {
			t.Errorf("expected total 15, got %f", v)
		}
	case int64:
		if v != 15 {
			t.Errorf("expected total 15, got %d", v)
		}
	}
}

func TestExecute_LLMStructuredOutput(t *testing.T) {
	mockLLM := &mock.MockLLMProvider{
		Responses: []string{`{"category": "billing", "confidence": 0.95}`},
	}
	executors := setupExecutors()
	deps := testDeps(mockLLM)

	outputSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"category":   map[string]string{"type": "string"},
			"confidence": map[string]string{"type": "number"},
		},
		"required": []string{"category", "confidence"},
	}

	g := &model.Graph{
		ID: "g5", Name: "json-test", Namespace: "default", TenantID: "default",
		Nodes: []model.Node{
			{
				ID: "in", Name: "Input", Type: model.NodeTypeInput,
				InputPorts:  []model.Port{},
				OutputPorts: []model.Port{{Name: "text", Schema: js(map[string]string{"type": "string"})}},
				Config:      js(nil),
			},
			{
				ID: "llm1", Name: "Classify", Type: model.NodeTypeLLM,
				InputPorts:  []model.Port{{Name: "text", Schema: js(map[string]string{"type": "string"})}},
				OutputPorts: []model.Port{{Name: "response", Schema: js(outputSchema)}},
				Config: js(model.LLMNodeConfig{
					Provider: "anthropic", Model: "test", APIKeyRef: "test-key",
					UserPrompt:     "Classify: {{input.text}}",
					Variables:      []model.TemplateVar{{Name: "text", Schema: js(map[string]string{"type": "string"})}},
					ResponseFormat: model.ResponseFormatJSON,
					OutputSchema:   js(outputSchema),
				}),
			},
			{
				ID: "out", Name: "Output", Type: model.NodeTypeOutput,
				InputPorts:  []model.Port{{Name: "classification", Schema: js(outputSchema)}},
				OutputPorts: []model.Port{},
				Config:      js(nil),
			},
		},
		Edges: []model.Edge{
			{ID: "e1", SourceNodeID: "in", SourcePort: "text", TargetNodeID: "llm1", TargetPort: "text"},
			{ID: "e2", SourceNodeID: "llm1", SourcePort: "response", TargetNodeID: "out", TargetPort: "classification"},
		},
	}

	result, err := Execute(context.Background(), g, map[string]any{"text": "I was charged twice"}, deps, executors, testLogger())
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	classification, ok := result.Outputs["classification"].(map[string]any)
	if !ok {
		t.Fatalf("expected map output, got %T: %v", result.Outputs["classification"], result.Outputs["classification"])
	}
	if classification["category"] != "billing" {
		t.Errorf("expected category 'billing', got %v", classification["category"])
	}
}

func TestExecute_StateDirectlyAccessible(t *testing.T) {
	// input -> transform (uses state.x directly) -> output
	// Graph has state field "x" with initial value 10.
	executors := setupExecutors()
	deps := testDeps(&mock.MockLLMProvider{})

	g := &model.Graph{
		ID: "g-state", Name: "state-test", Namespace: "default", TenantID: "default",
		State: &model.GraphState{
			Fields: []model.StateField{
				{
					Name:    "x",
					Schema:  js(map[string]string{"type": "number"}),
					Reducer: model.ReducerReplace,
					Initial: js(10),
				},
			},
		},
		Nodes: []model.Node{
			{
				ID: "in", Name: "Input", Type: model.NodeTypeInput,
				OutputPorts: []model.Port{{Name: "val", Schema: js(map[string]string{"type": "number"})}},
				Config:      js(nil),
			},
			{
				ID: "t1", Name: "Transform", Type: model.NodeTypeTransform,
				InputPorts:  []model.Port{{Name: "val", Schema: js(map[string]string{"type": "number"})}},
				OutputPorts: []model.Port{{Name: "result", Schema: js(map[string]string{"type": "number"})}},
				Config:      js(model.TransformNodeConfig{Expressions: map[string]string{"result": "state.x + input.val"}}),
			},
			{
				ID: "out", Name: "Output", Type: model.NodeTypeOutput,
				InputPorts: []model.Port{{Name: "result", Schema: js(map[string]string{"type": "number"})}},
				Config:     js(nil),
			},
		},
		Edges: []model.Edge{
			{ID: "e1", SourceNodeID: "in", SourcePort: "val", TargetNodeID: "t1", TargetPort: "val"},
			{ID: "e2", SourceNodeID: "t1", SourcePort: "result", TargetNodeID: "out", TargetPort: "result"},
		},
	}

	result, err := Execute(context.Background(), g, map[string]any{"val": int64(5)}, deps, executors, testLogger())
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// state.x=10, input.val=5 → result=15
	got := result.Outputs["result"]
	switch v := got.(type) {
	case int64:
		if v != 15 {
			t.Errorf("expected 15, got %d", v)
		}
	case float64:
		if v != 15 {
			t.Errorf("expected 15, got %f", v)
		}
	default:
		t.Errorf("unexpected type %T: %v", got, got)
	}
}

func TestExecute_MetaAccessible(t *testing.T) {
	// input -> transform (uses meta.node_id) -> output
	executors := setupExecutors()
	deps := testDeps(&mock.MockLLMProvider{})

	g := &model.Graph{
		ID: "g-meta", Name: "meta-test", Namespace: "default", TenantID: "default",
		Nodes: []model.Node{
			{
				ID: "in", Name: "Input", Type: model.NodeTypeInput,
				OutputPorts: []model.Port{{Name: "val", Schema: js(map[string]string{"type": "string"})}},
				Config:      js(nil),
			},
			{
				ID: "t1", Name: "Transform", Type: model.NodeTypeTransform,
				InputPorts:  []model.Port{{Name: "val", Schema: js(map[string]string{"type": "string"})}},
				OutputPorts: []model.Port{{Name: "node_id", Schema: js(map[string]string{"type": "string"})}},
				Config:      js(model.TransformNodeConfig{Expressions: map[string]string{"node_id": "meta.node_id"}}),
			},
			{
				ID: "out", Name: "Output", Type: model.NodeTypeOutput,
				InputPorts: []model.Port{{Name: "node_id", Schema: js(map[string]string{"type": "string"})}},
				Config:     js(nil),
			},
		},
		Edges: []model.Edge{
			{ID: "e1", SourceNodeID: "in", SourcePort: "val", TargetNodeID: "t1", TargetPort: "val"},
			{ID: "e2", SourceNodeID: "t1", SourcePort: "node_id", TargetNodeID: "out", TargetPort: "node_id"},
		},
	}

	result, err := Execute(context.Background(), g, map[string]any{"val": "test"}, deps, executors, testLogger())
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Outputs["node_id"] != "t1" {
		t.Errorf("expected node_id 't1', got %v", result.Outputs["node_id"])
	}
}

func boolPtr(b bool) *bool { return &b }

package executor

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/brockleyai/brockleyai/engine/mock"
	"github.com/brockleyai/brockleyai/internal/model"
)

// mockProviderRegistry is a simple in-memory registry for tests.
type mockProviderRegistry struct {
	providers map[string]model.LLMProvider
}

func (r *mockProviderRegistry) Get(name string) (model.LLMProvider, error) {
	p, ok := r.providers[name]
	if !ok {
		return nil, &errProviderNotFound{name: name}
	}
	return p, nil
}

type errProviderNotFound struct{ name string }

func (e *errProviderNotFound) Error() string {
	return "provider not found: " + e.name
}

func newTestDeps(provider *mock.MockLLMProvider, secrets map[string]string) *ExecutorDeps {
	ss := mock.NewMockSecretStore()
	for k, v := range secrets {
		ss.Secrets[k] = v
	}
	reg := &mockProviderRegistry{
		providers: map[string]model.LLMProvider{
			"mock": provider,
		},
	}
	return &ExecutorDeps{
		ProviderRegistry: reg,
		SecretStore:      ss,
		EventEmitter:     &mock.MockEventEmitter{},
		Logger:           slog.Default(),
	}
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// --- LLM executor tests ---

func TestLLMExecutor_TextResponse(t *testing.T) {
	provider := &mock.MockLLMProvider{
		Responses: []string{"Hello, world!"},
	}
	deps := newTestDeps(provider, map[string]string{"my-key": "sk-test"})

	cfg := model.LLMNodeConfig{
		Provider:       "mock",
		Model:          "test-model",
		APIKeyRef:      "my-key",
		UserPrompt:     "Say hello to {{input.name}}",
		ResponseFormat: model.ResponseFormatText,
	}

	node := &model.Node{
		ID:     "llm-1",
		Name:   "test-llm",
		Type:   model.NodeTypeLLM,
		Config: mustJSON(cfg),
	}

	inputs := map[string]any{
		"name": "Alice",
	}

	exec := &LLMExecutor{}
	result, err := exec.Execute(context.Background(), node, inputs, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text, ok := result.Outputs["response_text"]
	if !ok {
		t.Fatal("expected output port 'response_text'")
	}
	if text != "Hello, world!" {
		t.Errorf("expected 'Hello, world!', got %q", text)
	}

	// Verify the provider received the rendered prompt.
	if len(provider.Calls) != 1 {
		t.Fatalf("expected 1 provider call, got %d", len(provider.Calls))
	}
	if provider.Calls[0].UserPrompt != "Say hello to Alice" {
		t.Errorf("expected rendered prompt 'Say hello to Alice', got %q", provider.Calls[0].UserPrompt)
	}
}

func TestLLMExecutor_JSONResponse(t *testing.T) {
	provider := &mock.MockLLMProvider{
		Responses: []string{`{"result": 42, "status": "ok"}`},
	}
	deps := newTestDeps(provider, map[string]string{"my-key": "sk-test"})

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"result": map[string]any{"type": "number"},
			"status": map[string]any{"type": "string"},
		},
	}

	cfg := model.LLMNodeConfig{
		Provider:       "mock",
		Model:          "test-model",
		APIKeyRef:      "my-key",
		UserPrompt:     "Compute something",
		ResponseFormat: model.ResponseFormatJSON,
		OutputSchema:   mustJSON(schema),
	}

	node := &model.Node{
		ID:     "llm-2",
		Name:   "test-llm-json",
		Type:   model.NodeTypeLLM,
		Config: mustJSON(cfg),
	}

	exec := &LLMExecutor{}
	result, err := exec.Execute(context.Background(), node, nil, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parsed, ok := result.Outputs["response"]
	if !ok {
		t.Fatal("expected output port 'response'")
	}

	m, ok := parsed.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", parsed)
	}
	if m["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", m["status"])
	}
	// JSON numbers unmarshal as float64.
	if m["result"] != float64(42) {
		t.Errorf("expected result 42, got %v", m["result"])
	}

	// Verify system prompt includes schema instruction.
	if len(provider.Calls) != 1 {
		t.Fatalf("expected 1 provider call, got %d", len(provider.Calls))
	}
	call := provider.Calls[0]
	if call.SystemPrompt == "" {
		t.Error("expected system prompt to contain schema instruction")
	}
}

func TestLLMExecutor_InvalidJSON(t *testing.T) {
	provider := &mock.MockLLMProvider{
		Responses: []string{"not valid json"},
	}
	deps := newTestDeps(provider, map[string]string{"my-key": "sk-test"})

	cfg := model.LLMNodeConfig{
		Provider:       "mock",
		Model:          "test-model",
		APIKeyRef:      "my-key",
		UserPrompt:     "Do something",
		ResponseFormat: model.ResponseFormatJSON,
	}

	node := &model.Node{
		ID:     "llm-3",
		Name:   "test-llm-bad-json",
		Type:   model.NodeTypeLLM,
		Config: mustJSON(cfg),
	}

	exec := &LLMExecutor{}
	_, err := exec.Execute(context.Background(), node, nil, nil, deps)
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestLLMExecutor_InlineAPIKey(t *testing.T) {
	provider := &mock.MockLLMProvider{
		Responses: []string{"Hi!"},
	}
	// No secrets in store — key is inline.
	deps := newTestDeps(provider, nil)

	cfg := model.LLMNodeConfig{
		Provider:       "mock",
		Model:          "test-model",
		APIKey:         "sk-inline-key-12345",
		UserPrompt:     "Hello",
		ResponseFormat: model.ResponseFormatText,
	}

	node := &model.Node{
		ID:     "llm-inline",
		Name:   "test-inline-key",
		Type:   model.NodeTypeLLM,
		Config: mustJSON(cfg),
	}

	exec := &LLMExecutor{}
	result, err := exec.Execute(context.Background(), node, nil, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := result.Outputs["response_text"]; !ok {
		t.Fatal("expected output port 'response_text'")
	}

	// Verify the API key was passed to the provider via the request.
	if len(provider.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(provider.Calls))
	}
	if provider.Calls[0].APIKey != "sk-inline-key-12345" {
		t.Errorf("expected inline API key, got %q", provider.Calls[0].APIKey)
	}
}

func TestLLMExecutor_InlineKeyOverridesRef(t *testing.T) {
	provider := &mock.MockLLMProvider{
		Responses: []string{"Hi!"},
	}
	// Secret store has a different key.
	deps := newTestDeps(provider, map[string]string{"my-key": "sk-from-store"})

	cfg := model.LLMNodeConfig{
		Provider:       "mock",
		Model:          "test-model",
		APIKey:         "sk-inline-wins",
		APIKeyRef:      "my-key",
		UserPrompt:     "Hello",
		ResponseFormat: model.ResponseFormatText,
	}

	node := &model.Node{
		ID:     "llm-override",
		Name:   "test-override",
		Type:   model.NodeTypeLLM,
		Config: mustJSON(cfg),
	}

	exec := &LLMExecutor{}
	_, err := exec.Execute(context.Background(), node, nil, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Inline key should win over ref.
	if provider.Calls[0].APIKey != "sk-inline-wins" {
		t.Errorf("expected inline key to override ref, got %q", provider.Calls[0].APIKey)
	}
}

func TestLLMExecutor_FallbackToRef(t *testing.T) {
	provider := &mock.MockLLMProvider{
		Responses: []string{"Hi!"},
	}
	deps := newTestDeps(provider, map[string]string{"my-key": "sk-from-ref"})

	cfg := model.LLMNodeConfig{
		Provider:       "mock",
		Model:          "test-model",
		APIKeyRef:      "my-key",
		UserPrompt:     "Hello",
		ResponseFormat: model.ResponseFormatText,
	}

	node := &model.Node{
		ID:     "llm-ref",
		Name:   "test-ref",
		Type:   model.NodeTypeLLM,
		Config: mustJSON(cfg),
	}

	exec := &LLMExecutor{}
	_, err := exec.Execute(context.Background(), node, nil, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if provider.Calls[0].APIKey != "sk-from-ref" {
		t.Errorf("expected ref key, got %q", provider.Calls[0].APIKey)
	}
}

func TestLLMExecutor_NoKey(t *testing.T) {
	provider := &mock.MockLLMProvider{
		Responses: []string{"Hi!"},
	}
	deps := newTestDeps(provider, nil)

	cfg := model.LLMNodeConfig{
		Provider:       "mock",
		Model:          "test-model",
		UserPrompt:     "Hello",
		ResponseFormat: model.ResponseFormatText,
	}

	node := &model.Node{
		ID:     "llm-nokey",
		Name:   "test-nokey",
		Type:   model.NodeTypeLLM,
		Config: mustJSON(cfg),
	}

	exec := &LLMExecutor{}
	_, err := exec.Execute(context.Background(), node, nil, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No key — provider should receive empty API key.
	if provider.Calls[0].APIKey != "" {
		t.Errorf("expected empty API key, got %q", provider.Calls[0].APIKey)
	}
}

// --- Conditional executor tests ---

func TestConditionalExecutor_BranchMatch(t *testing.T) {
	tests := []struct {
		name         string
		inputValue   any
		branches     []model.Branch
		defaultLabel string
		wantPort     string
	}{
		{
			name:       "first branch matches",
			inputValue: float64(100),
			branches: []model.Branch{
				{Label: "high", Condition: "input.value > 50"},
				{Label: "low", Condition: "input.value <= 50"},
			},
			defaultLabel: "other",
			wantPort:     "high",
		},
		{
			name:       "second branch matches",
			inputValue: float64(10),
			branches: []model.Branch{
				{Label: "high", Condition: "input.value > 50"},
				{Label: "low", Condition: "input.value <= 50"},
			},
			defaultLabel: "other",
			wantPort:     "low",
		},
		{
			name:       "no branch matches — default fires",
			inputValue: "unknown",
			branches: []model.Branch{
				{Label: "yes", Condition: "input.value == \"approved\""},
				{Label: "no", Condition: "input.value == \"rejected\""},
			},
			defaultLabel: "other",
			wantPort:     "other",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := model.ConditionalNodeConfig{
				Branches:     tt.branches,
				DefaultLabel: tt.defaultLabel,
			}

			node := &model.Node{
				ID:     "cond-1",
				Name:   "test-conditional",
				Type:   model.NodeTypeConditional,
				Config: mustJSON(cfg),
			}

			inputs := map[string]any{
				"value": tt.inputValue,
			}

			exec := &ConditionalExecutor{}
			result, err := exec.Execute(context.Background(), node, inputs, nil, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			val, ok := result.Outputs[tt.wantPort]
			if !ok {
				t.Fatalf("expected output port %q to be populated, got outputs: %v", tt.wantPort, result.Outputs)
			}

			// The matched port should carry the input value.
			if val != tt.inputValue {
				t.Errorf("expected port %q to have value %v, got %v", tt.wantPort, tt.inputValue, val)
			}

			// Other ports should not be populated.
			for port := range result.Outputs {
				if port != tt.wantPort {
					t.Errorf("unexpected port %q populated", port)
				}
			}
		})
	}
}

// --- Transform executor tests ---

func TestTransformExecutor(t *testing.T) {
	tests := []struct {
		name        string
		expressions map[string]string
		inputs      map[string]any
		wantOutputs map[string]any
	}{
		{
			name: "simple arithmetic",
			expressions: map[string]string{
				"doubled": "input.x * 2",
				"sum":     "input.x + input.y",
			},
			inputs: map[string]any{
				"x": int64(5),
				"y": int64(3),
			},
			wantOutputs: map[string]any{
				"doubled": int64(10),
				"sum":     int64(8),
			},
		},
		{
			name: "string concatenation",
			expressions: map[string]string{
				"greeting": "input.first + \" \" + input.last",
			},
			inputs: map[string]any{
				"first": "Jane",
				"last":  "Doe",
			},
			wantOutputs: map[string]any{
				"greeting": "Jane Doe",
			},
		},
		{
			name: "nested access",
			expressions: map[string]string{
				"name": "input.user.name",
			},
			inputs: map[string]any{
				"user": map[string]any{
					"name": "Bob",
				},
			},
			wantOutputs: map[string]any{
				"name": "Bob",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := model.TransformNodeConfig{
				Expressions: tt.expressions,
			}

			node := &model.Node{
				ID:     "transform-1",
				Name:   "test-transform",
				Type:   model.NodeTypeTransform,
				Config: mustJSON(cfg),
			}

			exec := &TransformExecutor{}
			result, err := exec.Execute(context.Background(), node, tt.inputs, nil, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for k, want := range tt.wantOutputs {
				got, ok := result.Outputs[k]
				if !ok {
					t.Errorf("missing output port %q", k)
					continue
				}
				if got != want {
					t.Errorf("output %q: got %v (%T), want %v (%T)", k, got, got, want, want)
				}
			}
		})
	}
}

// --- Passthrough executor tests ---

func TestInputExecutor(t *testing.T) {
	exec := &InputExecutor{}
	inputs := map[string]any{
		"a": "hello",
		"b": int64(42),
	}

	result, err := exec.Execute(context.Background(), &model.Node{}, inputs, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for k, v := range inputs {
		if result.Outputs[k] != v {
			t.Errorf("output %q: got %v, want %v", k, result.Outputs[k], v)
		}
	}
}

func TestOutputExecutor(t *testing.T) {
	exec := &OutputExecutor{}
	inputs := map[string]any{
		"result": "done",
		"count":  int64(7),
	}

	result, err := exec.Execute(context.Background(), &model.Node{}, inputs, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for k, v := range inputs {
		if result.Outputs[k] != v {
			t.Errorf("output %q: got %v, want %v", k, result.Outputs[k], v)
		}
	}
}

// --- HITL executor test ---

func TestHITLExecutor_ReturnsError(t *testing.T) {
	exec := &HITLExecutor{}
	_, err := exec.Execute(context.Background(), &model.Node{}, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error from HITL executor")
	}
}

// --- Registry tests ---

func TestRegistry_GetUnregistered(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for unregistered node type")
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	r.Register("input", &InputExecutor{})

	exec, err := r.Get("input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec == nil {
		t.Fatal("expected non-nil executor")
	}
}

// --- State/Meta tests ---

func TestTransformExecutor_WithState(t *testing.T) {
	cfg := model.TransformNodeConfig{
		Expressions: map[string]string{
			"incremented": "state.count + 1",
		},
	}
	node := &model.Node{
		ID: "t-state", Name: "test-state", Type: model.NodeTypeTransform,
		Config: mustJSON(cfg),
	}

	nctx := &NodeContext{
		State: map[string]any{"count": int64(5)},
	}

	exec := &TransformExecutor{}
	result, err := exec.Execute(context.Background(), node, map[string]any{}, nctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Outputs["incremented"] != int64(6) {
		t.Errorf("expected 6, got %v (%T)", result.Outputs["incremented"], result.Outputs["incremented"])
	}
}

func TestTransformExecutor_WithMeta(t *testing.T) {
	cfg := model.TransformNodeConfig{
		Expressions: map[string]string{
			"id": "meta.node_id",
		},
	}
	node := &model.Node{
		ID: "t-meta", Name: "test-meta", Type: model.NodeTypeTransform,
		Config: mustJSON(cfg),
	}

	nctx := &NodeContext{
		Meta: map[string]any{"node_id": "t-meta"},
	}

	exec := &TransformExecutor{}
	result, err := exec.Execute(context.Background(), node, map[string]any{}, nctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Outputs["id"] != "t-meta" {
		t.Errorf("expected 't-meta', got %v", result.Outputs["id"])
	}
}

func TestConditionalExecutor_WithState(t *testing.T) {
	cfg := model.ConditionalNodeConfig{
		Branches: []model.Branch{
			{Label: "loop", Condition: "state.count < 3"},
		},
		DefaultLabel: "done",
	}
	node := &model.Node{
		ID: "c-state", Name: "test-cond-state", Type: model.NodeTypeConditional,
		Config: mustJSON(cfg),
	}

	// state.count = 1, should match "loop"
	nctx := &NodeContext{
		State: map[string]any{"count": int64(1)},
	}
	exec := &ConditionalExecutor{}
	result, err := exec.Execute(context.Background(), node, map[string]any{"value": "test"}, nctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Outputs["loop"]; !ok {
		t.Errorf("expected 'loop' branch to fire, got %v", result.Outputs)
	}

	// state.count = 5, should match "done"
	nctx2 := &NodeContext{
		State: map[string]any{"count": int64(5)},
	}
	result2, err := exec.Execute(context.Background(), node, map[string]any{"value": "test"}, nctx2, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result2.Outputs["done"]; !ok {
		t.Errorf("expected 'done' branch to fire, got %v", result2.Outputs)
	}
}

func TestLLMExecutor_WithStateInTemplate(t *testing.T) {
	provider := &mock.MockLLMProvider{
		Responses: []string{"response"},
	}
	deps := newTestDeps(provider, map[string]string{"my-key": "sk-test"})

	cfg := model.LLMNodeConfig{
		Provider:       "mock",
		Model:          "test-model",
		APIKeyRef:      "my-key",
		UserPrompt:     "Count is {{state.count}}",
		ResponseFormat: model.ResponseFormatText,
	}
	node := &model.Node{
		ID: "llm-state", Name: "test-llm-state", Type: model.NodeTypeLLM,
		Config: mustJSON(cfg),
	}

	nctx := &NodeContext{
		State: map[string]any{"count": int64(42)},
	}

	exec := &LLMExecutor{}
	_, err := exec.Execute(context.Background(), node, map[string]any{}, nctx, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if provider.Calls[0].UserPrompt != "Count is 42" {
		t.Errorf("expected rendered prompt 'Count is 42', got %q", provider.Calls[0].UserPrompt)
	}
}

func TestLLMExecutor_WithMetaInTemplate(t *testing.T) {
	provider := &mock.MockLLMProvider{
		Responses: []string{"response"},
	}
	deps := newTestDeps(provider, map[string]string{"my-key": "sk-test"})

	cfg := model.LLMNodeConfig{
		Provider:       "mock",
		Model:          "test-model",
		APIKeyRef:      "my-key",
		UserPrompt:     "Node: {{meta.node_id}}",
		ResponseFormat: model.ResponseFormatText,
	}
	node := &model.Node{
		ID: "llm-meta", Name: "test-llm-meta", Type: model.NodeTypeLLM,
		Config: mustJSON(cfg),
	}

	nctx := &NodeContext{
		Meta: map[string]any{"node_id": "llm-meta"},
	}

	exec := &LLMExecutor{}
	_, err := exec.Execute(context.Background(), node, map[string]any{}, nctx, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if provider.Calls[0].UserPrompt != "Node: llm-meta" {
		t.Errorf("expected rendered prompt 'Node: llm-meta', got %q", provider.Calls[0].UserPrompt)
	}
}

func TestLLMExecutor_ToolCallsOutput(t *testing.T) {
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			{
				Content: "",
				ToolCalls: []model.ToolCall{
					{ID: "call_1", Name: "get_weather", Arguments: json.RawMessage(`{"location":"London"}`)},
				},
				FinishReason: "tool_calls",
			},
		},
	}
	deps := newTestDeps(provider, nil)

	cfg := model.LLMNodeConfig{
		Provider:       "mock",
		Model:          "test-model",
		UserPrompt:     "What's the weather?",
		ResponseFormat: model.ResponseFormatText,
		Tools: []model.LLMToolDefinition{
			{
				Name:        "get_weather",
				Description: "Get the weather",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"location":{"type":"string"}}}`),
			},
		},
		ToolChoice: "auto",
	}

	node := &model.Node{
		ID:     "llm-tools",
		Name:   "test-llm-tools",
		Type:   model.NodeTypeLLM,
		Config: mustJSON(cfg),
	}

	exec := &LLMExecutor{}
	result, err := exec.Execute(context.Background(), node, nil, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have tool_calls and finish_reason outputs.
	fr, ok := result.Outputs["finish_reason"]
	if !ok {
		t.Fatal("expected output port 'finish_reason'")
	}
	if fr != "tool_calls" {
		t.Errorf("expected finish_reason='tool_calls', got %v", fr)
	}

	tc, ok := result.Outputs["tool_calls"]
	if !ok {
		t.Fatal("expected output port 'tool_calls'")
	}
	calls, ok := tc.([]model.ToolCall)
	if !ok {
		t.Fatalf("expected []model.ToolCall, got %T", tc)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Name != "get_weather" {
		t.Errorf("expected get_weather, got %s", calls[0].Name)
	}

	// Verify tools were passed to the provider.
	if len(provider.Calls) != 1 {
		t.Fatalf("expected 1 provider call, got %d", len(provider.Calls))
	}
	if len(provider.Calls[0].Tools) != 1 {
		t.Fatalf("expected 1 tool in request, got %d", len(provider.Calls[0].Tools))
	}
	if provider.Calls[0].ToolChoice != "auto" {
		t.Errorf("expected tool_choice=auto, got %s", provider.Calls[0].ToolChoice)
	}
}

func TestLLMExecutor_NoToolCallsOutput_WhenNoTools(t *testing.T) {
	provider := &mock.MockLLMProvider{
		Responses: []string{"Hello!"},
	}
	deps := newTestDeps(provider, nil)

	cfg := model.LLMNodeConfig{
		Provider:       "mock",
		Model:          "test-model",
		UserPrompt:     "Say hi",
		ResponseFormat: model.ResponseFormatText,
	}

	node := &model.Node{
		ID:     "llm-no-tools",
		Name:   "test-no-tools",
		Type:   model.NodeTypeLLM,
		Config: mustJSON(cfg),
	}

	exec := &LLMExecutor{}
	result, err := exec.Execute(context.Background(), node, nil, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should NOT have tool_calls or finish_reason outputs when no tools configured.
	if _, ok := result.Outputs["tool_calls"]; ok {
		t.Error("unexpected tool_calls output when no tools configured")
	}
	if _, ok := result.Outputs["finish_reason"]; ok {
		t.Error("unexpected finish_reason output when no tools configured")
	}
}

func TestDefaultRegistry_HasBuiltins(t *testing.T) {
	r := NewDefaultRegistry()

	builtins := []string{
		model.NodeTypeInput,
		model.NodeTypeOutput,
		model.NodeTypeLLM,
		model.NodeTypeConditional,
		model.NodeTypeTransform,
		model.NodeTypeHumanInTheLoop,
		model.NodeTypeForEach,
		model.NodeTypeSubgraph,
	}

	for _, nt := range builtins {
		_, err := r.Get(nt)
		if err != nil {
			t.Errorf("expected executor for %q, got error: %v", nt, err)
		}
	}
}

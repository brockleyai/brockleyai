package executor

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brockleyai/brockleyai/engine/mock"
	"github.com/brockleyai/brockleyai/internal/model"
)

func newSuperagentDeps(provider *mock.MockLLMProvider, mcpClient *mock.MockMCPClient) *ExecutorDeps {
	reg := &mockProviderRegistry{
		providers: map[string]model.LLMProvider{"mock": provider},
	}
	cache := NewMCPClientCache()
	if mcpClient != nil {
		cache.clients["http://test-mcp:9001"] = mcpClient
	}
	return &ExecutorDeps{
		ProviderRegistry: reg,
		SecretStore:      mock.NewMockSecretStore(),
		MCPClientCache:   cache,
		EventEmitter:     &mock.MockEventEmitter{},
	}
}

func validSuperagentNode(cfg model.SuperagentNodeConfig) *model.Node {
	return &model.Node{
		ID:     "sa-1",
		Name:   "test-superagent",
		Type:   model.NodeTypeSuperagent,
		Config: mustJSON(cfg),
		InputPorts: []model.Port{
			{Name: "task", Schema: json.RawMessage(`{"type":"string"}`)},
		},
		OutputPorts: []model.Port{
			{Name: "result", Schema: json.RawMessage(`{"type":"string"}`)},
		},
	}
}

func baseSuperagentConfig() model.SuperagentNodeConfig {
	return model.SuperagentNodeConfig{
		Prompt:   "Do the task: {{input.task}}",
		Provider: "mock",
		Model:    "test-model",
		APIKey:   "sk-test",
		Skills: []model.SuperagentSkill{
			{
				Name:        "code",
				Description: "Code tools",
				MCPURL:      "http://test-mcp:9001",
			},
		},
	}
}

func baseMCPClient() *mock.MockMCPClient {
	return &mock.MockMCPClient{
		Tools: map[string]mock.MockTool{
			"echo": {
				Definition: model.ToolDefinition{
					Name:        "echo",
					Description: "Echo text",
					InputSchema: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"text": map[string]any{"type": "string"},
						},
					},
				},
				Handler: func(args map[string]any) (*model.ToolResult, error) {
					return &model.ToolResult{Content: args["text"], IsError: false}, nil
				},
			},
		},
	}
}

func TestSuperagent_ConfigParsing(t *testing.T) {
	cfg := baseSuperagentConfig()
	cfg.Overrides = &model.SuperagentOverrides{
		Evaluator: &model.EvaluatorOverride{Disabled: true},
	}
	maxIter := 1
	cfg.MaxIterations = &maxIter

	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			{Content: "Task completed", FinishReason: "stop"},
		},
	}

	mcpClient := baseMCPClient()
	deps := newSuperagentDeps(provider, mcpClient)
	node := validSuperagentNode(cfg)
	inputs := map[string]any{"task": "write tests"}

	exec := &SuperagentExecutor{}
	result, err := exec.Execute(context.Background(), node, inputs, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestSuperagent_ConfigParsing_MissingRequired(t *testing.T) {
	// Empty config JSON.
	node := &model.Node{
		ID:     "sa-bad",
		Name:   "bad-superagent",
		Type:   model.NodeTypeSuperagent,
		Config: json.RawMessage(`{"prompt":"","provider":"mock","model":"test","overrides":{"evaluator":{"disabled":true}},"max_iterations":1}`),
		OutputPorts: []model.Port{
			{Name: "result", Schema: json.RawMessage(`{"type":"string"}`)},
		},
	}

	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			{Content: "ok", FinishReason: "stop"},
		},
	}
	mcpClient := baseMCPClient()
	deps := newSuperagentDeps(provider, mcpClient)

	exec := &SuperagentExecutor{}
	result, err := exec.Execute(context.Background(), node, nil, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestSuperagent_SingleIteration(t *testing.T) {
	// LLM makes 2 tool calls, then returns text. Evaluator says done.
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			// Iteration 0: tool calls
			{
				ToolCalls: []model.ToolCall{
					{ID: "call_1", Name: "echo", Arguments: json.RawMessage(`{"text":"hello"}`)},
					{ID: "call_2", Name: "echo", Arguments: json.RawMessage(`{"text":"world"}`)},
				},
				FinishReason: "tool_calls",
			},
			// Iteration 0: final text
			{
				Content:      "Result: hello world",
				FinishReason: "stop",
			},
			// Evaluator: done
			{
				Content:      `{"needs_more_work":false,"stuck_detected":false,"should_compact":false,"reasoning":"done"}`,
				FinishReason: "stop",
			},
		},
	}

	mcpClient := baseMCPClient()
	deps := newSuperagentDeps(provider, mcpClient)
	cfg := baseSuperagentConfig()
	node := validSuperagentNode(cfg)
	inputs := map[string]any{"task": "echo test"}

	exec := &SuperagentExecutor{}
	result, err := exec.Execute(context.Background(), node, inputs, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Single string output port should get response text.
	resultVal, ok := result.Outputs["result"]
	if !ok {
		t.Fatal("expected 'result' output port")
	}
	if resultVal != "Result: hello world" {
		t.Errorf("expected 'Result: hello world', got %v", resultVal)
	}

	// Check meta outputs.
	if result.Outputs["_total_tool_calls"] != 2 {
		t.Errorf("expected _total_tool_calls=2, got %v", result.Outputs["_total_tool_calls"])
	}
	if result.Outputs["_finish_reason"] != "done" {
		t.Errorf("expected _finish_reason=done, got %v", result.Outputs["_finish_reason"])
	}
}

func TestSuperagent_OutputResolution_Buffer(t *testing.T) {
	// LLM calls _buffer_create, _buffer_append, _buffer_finalize, then stops.
	// Evaluator says done.
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			{
				ToolCalls: []model.ToolCall{
					{ID: "c1", Name: "_buffer_create", Arguments: json.RawMessage(`{"name":"output"}`)},
				},
				FinishReason: "tool_calls",
			},
			{
				ToolCalls: []model.ToolCall{
					{ID: "c2", Name: "_buffer_append", Arguments: json.RawMessage(`{"name":"output","content":"Hello from buffer"}`)},
				},
				FinishReason: "tool_calls",
			},
			{
				ToolCalls: []model.ToolCall{
					{ID: "c3", Name: "_buffer_finalize", Arguments: json.RawMessage(`{"name":"output","output_port":"result"}`)},
				},
				FinishReason: "tool_calls",
			},
			{
				Content:      "Done with buffer work",
				FinishReason: "stop",
			},
			// Evaluator: done
			{
				Content:      `{"needs_more_work":false,"stuck_detected":false,"should_compact":false,"reasoning":"done"}`,
				FinishReason: "stop",
			},
		},
	}

	mcpClient := baseMCPClient()
	deps := newSuperagentDeps(provider, mcpClient)
	cfg := baseSuperagentConfig()
	node := validSuperagentNode(cfg)
	inputs := map[string]any{"task": "use buffers"}

	exec := &SuperagentExecutor{}
	result, err := exec.Execute(context.Background(), node, inputs, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Buffer content should be used for the output port.
	resultVal, ok := result.Outputs["result"]
	if !ok {
		t.Fatal("expected 'result' output port")
	}
	if resultVal != "Hello from buffer" {
		t.Errorf("expected 'Hello from buffer', got %v", resultVal)
	}
}

func TestSuperagent_OutputResolution_SingleString(t *testing.T) {
	// Single string output port, no buffer operations. Response text used directly.
	// Evaluator says done.
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			{Content: "Direct response text", FinishReason: "stop"},
			// Evaluator: done
			{
				Content:      `{"needs_more_work":false,"stuck_detected":false,"should_compact":false,"reasoning":"done"}`,
				FinishReason: "stop",
			},
		},
	}

	mcpClient := baseMCPClient()
	deps := newSuperagentDeps(provider, mcpClient)
	cfg := baseSuperagentConfig()
	node := validSuperagentNode(cfg)
	inputs := map[string]any{"task": "simple task"}

	exec := &SuperagentExecutor{}
	result, err := exec.Execute(context.Background(), node, inputs, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultVal, ok := result.Outputs["result"]
	if !ok {
		t.Fatal("expected 'result' output port")
	}
	if resultVal != "Direct response text" {
		t.Errorf("expected 'Direct response text', got %v", resultVal)
	}
}

func TestSuperagent_OutputResolution_SingleStringFallsBackToExtractionWhenResponseEmpty(t *testing.T) {
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			{Content: "", FinishReason: "stop"},
			{
				Content:      `{"needs_more_work":false,"stuck_detected":false,"should_compact":false,"reasoning":"done"}`,
				FinishReason: "stop",
			},
			{Content: `"Extracted fallback text"`, FinishReason: "stop"},
		},
	}

	mcpClient := baseMCPClient()
	deps := newSuperagentDeps(provider, mcpClient)
	cfg := baseSuperagentConfig()
	node := validSuperagentNode(cfg)
	inputs := map[string]any{"task": "simple task"}

	exec := &SuperagentExecutor{}
	result, err := exec.Execute(context.Background(), node, inputs, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultVal, ok := result.Outputs["result"]
	if !ok {
		t.Fatal("expected 'result' output port")
	}
	if resultVal != "Extracted fallback text" {
		t.Errorf("expected extracted fallback text, got %v", resultVal)
	}
}

func TestSuperagent_PromptAssembly(t *testing.T) {
	cfg := baseSuperagentConfig()
	cfg.SystemPreamble = "Custom preamble"
	cfg.SharedMemory = &model.SharedMemoryConfig{Enabled: true}
	cfg.Overrides = &model.SuperagentOverrides{
		PromptAssembly: &model.PromptAssemblyOverride{
			ToolConventions: "Always use JSON",
			Style:           "Be concise",
		},
	}

	sharedMem := []MemoryEntry{
		{Key: "test/fact", Content: "The sky is blue", Tags: []string{"science"}},
	}

	outputPorts := []model.Port{
		{Name: "result", Schema: json.RawMessage(`{"type":"string"}`)},
		{Name: "_meta", Schema: json.RawMessage(`{"type":"object","properties":{"x":{"type":"string"}}}`)},
	}

	prompt, err := AssembleSystemPrompt(&cfg, map[string]any{"task": "test"}, sharedMem, nil, 0, outputPorts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check all sections are present.
	checks := []struct {
		name     string
		contains string
	}{
		{"preamble", "Custom preamble"},
		{"task", "## Task"},
		{"rendered task", "Do the task: test"},
		{"shared memory", "## Shared Memory"},
		{"memory entry", "The sky is blue"},
		{"memory tag", "[science]"},
		{"skills", "## Available Skills"},
		{"skill name", "**code**"},
		{"built-in tools", "## Built-In Tools"},
		{"memory tools", "### Shared Memory"},
		{"tool conventions", "## Tool Conventions"},
		{"style", "## Style Guidelines"},
		{"output requirements", "## Output Requirements"},
		{"output port", "**result**"},
		{"no meta port", ""},
	}

	for _, c := range checks {
		if c.contains == "" {
			continue
		}
		if !strings.Contains(prompt, c.contains) {
			t.Errorf("%s: expected prompt to contain %q", c.name, c.contains)
		}
	}

	// Meta output port should NOT appear.
	if strings.Contains(prompt, "**_meta**") {
		t.Error("meta output port should be excluded from output requirements")
	}
}

func TestSuperagent_PromptAssembly_WorkingMemory(t *testing.T) {
	cfg := baseSuperagentConfig()
	workingMem := map[string]any{"plan": "Step 1: do X"}

	prompt, err := AssembleSystemPrompt(&cfg, nil, nil, workingMem, 3, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(prompt, "## Current State") {
		t.Error("expected working memory section")
	}
	if !strings.Contains(prompt, "Iteration: 3") {
		t.Error("expected iteration in working memory")
	}
	if !strings.Contains(prompt, "Step 1: do X") {
		t.Error("expected plan in working memory")
	}
}

func TestSuperagent_MetaOutputs(t *testing.T) {
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			{
				ToolCalls: []model.ToolCall{
					{ID: "c1", Name: "_task_create", Arguments: json.RawMessage(`{"description":"test task"}`)},
				},
				FinishReason: "tool_calls",
			},
			{
				Content:      "Done",
				FinishReason: "stop",
			},
			// Evaluator: done
			{
				Content:      `{"needs_more_work":false,"stuck_detected":false,"should_compact":false,"reasoning":"done"}`,
				FinishReason: "stop",
			},
		},
	}

	mcpClient := baseMCPClient()
	deps := newSuperagentDeps(provider, mcpClient)
	cfg := baseSuperagentConfig()
	node := validSuperagentNode(cfg)
	inputs := map[string]any{"task": "meta test"}

	exec := &SuperagentExecutor{}
	result, err := exec.Execute(context.Background(), node, inputs, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify meta outputs exist.
	metaKeys := []string{
		"_conversation_history",
		"_iterations",
		"_total_tool_calls",
		"_finish_reason",
		"_tool_call_history",
		"_working_memory",
		"_tasks",
	}
	for _, key := range metaKeys {
		if _, ok := result.Outputs[key]; !ok {
			t.Errorf("expected meta output %q", key)
		}
	}

	if result.Outputs["_finish_reason"] != "done" {
		t.Errorf("expected _finish_reason=done, got %v", result.Outputs["_finish_reason"])
	}

	// Verify tasks were created via built-in tool.
	tasks, ok := result.Outputs["_tasks"].([]Task)
	if !ok {
		t.Fatalf("expected []Task, got %T", result.Outputs["_tasks"])
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Description != "test task" {
		t.Errorf("expected task description 'test task', got %q", tasks[0].Description)
	}
}

func TestSuperagent_APIKeyResolution(t *testing.T) {
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			{Content: "ok", FinishReason: "stop"},
			// Evaluator: done
			{
				Content:      `{"needs_more_work":false,"stuck_detected":false,"should_compact":false,"reasoning":"done"}`,
				FinishReason: "stop",
			},
		},
	}
	mcpClient := baseMCPClient()
	deps := newSuperagentDeps(provider, mcpClient)
	deps.SecretStore.(*mock.MockSecretStore).Secrets["my-key"] = "sk-from-store"

	cfg := baseSuperagentConfig()
	cfg.APIKey = ""
	cfg.APIKeyRef = "my-key"
	node := validSuperagentNode(cfg)

	exec := &SuperagentExecutor{}
	_, err := exec.Execute(context.Background(), node, map[string]any{"task": "test"}, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(provider.Calls) == 0 {
		t.Fatal("expected at least one provider call")
	}
	if provider.Calls[0].APIKey != "sk-from-store" {
		t.Errorf("expected API key from store, got %q", provider.Calls[0].APIKey)
	}
}

func TestSuperagent_EventEmission(t *testing.T) {
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			{Content: "Done", FinishReason: "stop"},
			// Evaluator: done
			{
				Content:      `{"needs_more_work":false,"stuck_detected":false,"should_compact":false,"reasoning":"done"}`,
				FinishReason: "stop",
			},
		},
	}
	mcpClient := baseMCPClient()
	emitter := &mock.MockEventEmitter{}
	deps := newSuperagentDeps(provider, mcpClient)
	deps.EventEmitter = emitter

	cfg := baseSuperagentConfig()
	node := validSuperagentNode(cfg)

	exec := &SuperagentExecutor{}
	_, err := exec.Execute(context.Background(), node, map[string]any{"task": "test"}, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	eventTypes := make(map[model.EventType]int)
	for _, ev := range emitter.Events {
		eventTypes[ev.Type]++
	}

	if eventTypes[model.EventSuperagentStarted] != 1 {
		t.Errorf("expected 1 superagent_started event, got %d", eventTypes[model.EventSuperagentStarted])
	}
	if eventTypes[model.EventSuperagentCompleted] != 1 {
		t.Errorf("expected 1 superagent_completed event, got %d", eventTypes[model.EventSuperagentCompleted])
	}
}

func TestDefaultRegistry_HasSuperagent(t *testing.T) {
	r := NewDefaultRegistry()
	_, err := r.Get(model.NodeTypeSuperagent)
	if err != nil {
		t.Errorf("expected executor for superagent, got error: %v", err)
	}
}

// --- Stuck Detector Unit Tests ---

func TestStuckDetector_WindowFilling(t *testing.T) {
	sd := NewStuckDetector(5, 3)

	// Record different tool calls - none should trigger stuck.
	calls := []struct {
		name string
		args string
	}{
		{"tool_a", `{"x":1}`},
		{"tool_b", `{"y":2}`},
		{"tool_c", `{"z":3}`},
		{"tool_d", `{"w":4}`},
		{"tool_e", `{"v":5}`},
	}

	for _, c := range calls {
		level := sd.Record(c.name, json.RawMessage(c.args))
		if level != StuckNone {
			t.Errorf("expected StuckNone for unique call %s, got %d", c.name, level)
		}
	}
}

func TestStuckDetector_ThresholdTrigger(t *testing.T) {
	sd := NewStuckDetector(10, 3)

	args := json.RawMessage(`{"text":"hello"}`)

	// First two calls: no stuck.
	for i := 0; i < 2; i++ {
		level := sd.Record("echo", args)
		if level != StuckNone {
			t.Errorf("call %d: expected StuckNone, got %d", i, level)
		}
	}

	// Third call should trigger StuckWarn.
	level := sd.Record("echo", args)
	if level != StuckWarn {
		t.Errorf("expected StuckWarn on 3rd identical call, got %d", level)
	}
}

func TestStuckDetector_Escalation(t *testing.T) {
	sd := NewStuckDetector(20, 3)

	args := json.RawMessage(`{"text":"hello"}`)

	// Trigger first breach -> StuckWarn
	for i := 0; i < 2; i++ {
		sd.Record("echo", args)
	}
	level := sd.Record("echo", args)
	if level != StuckWarn {
		t.Errorf("expected StuckWarn, got %d", level)
	}

	// Next identical call still meets threshold (4 out of 4 in window) -> second breach -> StuckReflect
	level = sd.Record("echo", args)
	if level != StuckReflect {
		t.Errorf("expected StuckReflect, got %d", level)
	}

	// Next identical call -> third breach -> StuckForceExit
	level = sd.Record("echo", args)
	if level != StuckForceExit {
		t.Errorf("expected StuckForceExit, got %d", level)
	}
}

func TestStuckDetector_ResetAfterReflection(t *testing.T) {
	sd := NewStuckDetector(20, 3)

	args := json.RawMessage(`{"text":"hello"}`)

	// Trigger first breach.
	for i := 0; i < 3; i++ {
		sd.Record("echo", args)
	}

	// Reset.
	sd.Reset()

	// breachCount should be 0 now. Next breach should be StuckWarn again (breach 1).
	level := sd.Record("echo", args)
	// 4 identical calls in window still >= threshold, so a breach occurs.
	if level != StuckWarn {
		t.Errorf("expected StuckWarn after reset, got %d", level)
	}
}

// --- P5 Agent Loop Tests ---

func TestSuperagent_MultiIteration(t *testing.T) {
	// Mock LLM: 2 iterations with evaluator calls between them.
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			// Iteration 0: tool call
			{
				ToolCalls: []model.ToolCall{
					{ID: "c1", Name: "echo", Arguments: json.RawMessage(`{"text":"first"}`)},
				},
				FinishReason: "tool_calls",
			},
			// Iteration 0: text response (tool loop done)
			{
				Content:      "First iteration done",
				FinishReason: "stop",
			},
			// Evaluator iteration 0: needs more work
			{
				Content:      `{"needs_more_work":true,"stuck_detected":false,"should_compact":false,"reasoning":"still working"}`,
				FinishReason: "stop",
			},
			// Iteration 1: tool call
			{
				ToolCalls: []model.ToolCall{
					{ID: "c2", Name: "echo", Arguments: json.RawMessage(`{"text":"second"}`)},
				},
				FinishReason: "tool_calls",
			},
			// Iteration 1: text response
			{
				Content:      "Second iteration done",
				FinishReason: "stop",
			},
			// Evaluator iteration 1: done
			{
				Content:      `{"needs_more_work":false,"stuck_detected":false,"should_compact":false,"reasoning":"done"}`,
				FinishReason: "stop",
			},
		},
	}

	mcpClient := baseMCPClient()
	deps := newSuperagentDeps(provider, mcpClient)
	cfg := baseSuperagentConfig()
	maxIter := 5
	cfg.MaxIterations = &maxIter
	node := validSuperagentNode(cfg)
	inputs := map[string]any{"task": "multi-iteration test"}

	exec := &SuperagentExecutor{}
	result, err := exec.Execute(context.Background(), node, inputs, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have run 2 iterations (index 0 and 1), but _iterations is the iteration counter at exit.
	// After iteration 1, evaluator says done, so we exit with iteration=1.
	iterations := result.Outputs["_iterations"]
	if iterations != 1 {
		t.Errorf("expected _iterations=1, got %v", iterations)
	}

	if result.Outputs["_finish_reason"] != "done" {
		t.Errorf("expected _finish_reason=done, got %v", result.Outputs["_finish_reason"])
	}

	if result.Outputs["_total_tool_calls"] != 2 {
		t.Errorf("expected _total_tool_calls=2, got %v", result.Outputs["_total_tool_calls"])
	}
}

func TestSuperagent_Timeout(t *testing.T) {
	// Create a context that will timeout quickly.
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			{Content: "ok", FinishReason: "stop"},
		},
	}

	mcpClient := baseMCPClient()
	deps := newSuperagentDeps(provider, mcpClient)
	cfg := baseSuperagentConfig()
	// Set a very short timeout - 1 millisecond.
	// But the timeout is applied inside runAgentLoop, so we use context cancellation.
	maxIter := 100
	cfg.MaxIterations = &maxIter
	cfg.Overrides = &model.SuperagentOverrides{
		Evaluator: &model.EvaluatorOverride{Disabled: true},
	}
	node := validSuperagentNode(cfg)

	// Use an already-cancelled context.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond) // Ensure timeout fires.

	exec := &SuperagentExecutor{}
	result, err := exec.Execute(ctx, node, map[string]any{"task": "test"}, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fr := result.Outputs["_finish_reason"]
	if fr != "timeout" && fr != "cancelled" {
		t.Errorf("expected _finish_reason timeout or cancelled, got %v", fr)
	}
}

func TestSuperagent_MaxIterations(t *testing.T) {
	// Evaluator disabled, MaxIterations=2.
	maxIter := 2
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			// Iteration 0: text response
			{Content: "iteration 0", FinishReason: "stop"},
			// Iteration 1: text response
			{Content: "iteration 1", FinishReason: "stop"},
			// Would be iteration 2 but should not reach here.
			{Content: "iteration 2", FinishReason: "stop"},
		},
	}

	mcpClient := baseMCPClient()
	deps := newSuperagentDeps(provider, mcpClient)
	cfg := baseSuperagentConfig()
	cfg.MaxIterations = &maxIter
	cfg.Overrides = &model.SuperagentOverrides{
		Evaluator: &model.EvaluatorOverride{Disabled: true},
	}
	node := validSuperagentNode(cfg)
	inputs := map[string]any{"task": "max iter test"}

	exec := &SuperagentExecutor{}
	result, err := exec.Execute(context.Background(), node, inputs, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outputs["_finish_reason"] != "max_iterations" {
		t.Errorf("expected _finish_reason=max_iterations, got %v", result.Outputs["_finish_reason"])
	}

	// Should have consumed exactly 2 provider calls (one per iteration).
	if len(provider.Calls) != 2 {
		t.Errorf("expected 2 provider calls, got %d", len(provider.Calls))
	}
}

func TestSuperagent_MaxTotalToolCalls(t *testing.T) {
	// MaxTotalToolCalls=3. Each iteration makes 2 tool calls.
	maxTotal := 3
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			// Iteration 0: 2 tool calls
			{
				ToolCalls: []model.ToolCall{
					{ID: "c1", Name: "echo", Arguments: json.RawMessage(`{"text":"a"}`)},
					{ID: "c2", Name: "echo", Arguments: json.RawMessage(`{"text":"b"}`)},
				},
				FinishReason: "tool_calls",
			},
			// Iteration 0: text done
			{Content: "iter 0 done", FinishReason: "stop"},
			// Evaluator: needs more work
			{
				Content:      `{"needs_more_work":true,"stuck_detected":false,"should_compact":false,"reasoning":"continue"}`,
				FinishReason: "stop",
			},
			// Iteration 1: 2 tool calls
			{
				ToolCalls: []model.ToolCall{
					{ID: "c3", Name: "echo", Arguments: json.RawMessage(`{"text":"c"}`)},
					{ID: "c4", Name: "echo", Arguments: json.RawMessage(`{"text":"d"}`)},
				},
				FinishReason: "tool_calls",
			},
			// Iteration 1: text done
			{Content: "iter 1 done", FinishReason: "stop"},
		},
	}

	mcpClient := baseMCPClient()
	deps := newSuperagentDeps(provider, mcpClient)
	cfg := baseSuperagentConfig()
	cfg.MaxTotalToolCalls = &maxTotal
	cfg.Overrides = &model.SuperagentOverrides{
		// Keep evaluator enabled but allow enough iterations.
	}
	node := validSuperagentNode(cfg)
	inputs := map[string]any{"task": "max calls test"}

	exec := &SuperagentExecutor{}
	result, err := exec.Execute(context.Background(), node, inputs, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outputs["_finish_reason"] != "max_tool_calls" {
		t.Errorf("expected _finish_reason=max_tool_calls, got %v", result.Outputs["_finish_reason"])
	}
}

func TestSuperagent_EvaluatorDisabled(t *testing.T) {
	maxIter := 3
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			{Content: "iter 0", FinishReason: "stop"},
			{Content: "iter 1", FinishReason: "stop"},
			{Content: "iter 2", FinishReason: "stop"},
		},
	}

	mcpClient := baseMCPClient()
	deps := newSuperagentDeps(provider, mcpClient)
	cfg := baseSuperagentConfig()
	cfg.MaxIterations = &maxIter
	cfg.Overrides = &model.SuperagentOverrides{
		Evaluator: &model.EvaluatorOverride{Disabled: true},
	}
	node := validSuperagentNode(cfg)
	inputs := map[string]any{"task": "eval disabled test"}

	exec := &SuperagentExecutor{}
	result, err := exec.Execute(context.Background(), node, inputs, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outputs["_finish_reason"] != "max_iterations" {
		t.Errorf("expected _finish_reason=max_iterations, got %v", result.Outputs["_finish_reason"])
	}

	// Should have consumed exactly 3 provider calls.
	if len(provider.Calls) != 3 {
		t.Errorf("expected 3 provider calls, got %d", len(provider.Calls))
	}
}

func TestSuperagent_ReflectionTriggered(t *testing.T) {
	// Evaluator returns stuck_detected=true, then done on second eval.
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			// Iteration 0: text response
			{Content: "working on it", FinishReason: "stop"},
			// Evaluator 0: stuck detected
			{
				Content:      `{"needs_more_work":true,"stuck_detected":true,"should_compact":false,"reasoning":"stuck"}`,
				FinishReason: "stop",
			},
			// Reflection call
			{
				Content:      `{"reflection_text":"need different approach","new_plan":"try plan B"}`,
				FinishReason: "stop",
			},
			// Iteration 1: text response
			{Content: "plan B worked", FinishReason: "stop"},
			// Evaluator 1: done
			{
				Content:      `{"needs_more_work":false,"stuck_detected":false,"should_compact":false,"reasoning":"done"}`,
				FinishReason: "stop",
			},
		},
	}

	mcpClient := baseMCPClient()
	deps := newSuperagentDeps(provider, mcpClient)
	cfg := baseSuperagentConfig()
	node := validSuperagentNode(cfg)
	inputs := map[string]any{"task": "reflection test"}

	exec := &SuperagentExecutor{}
	result, err := exec.Execute(context.Background(), node, inputs, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outputs["_finish_reason"] != "done" {
		t.Errorf("expected _finish_reason=done, got %v", result.Outputs["_finish_reason"])
	}

	// Verify reflection was injected into conversation history.
	msgs, ok := result.Outputs["_conversation_history"].([]model.Message)
	if !ok {
		t.Fatal("expected conversation history to be []model.Message")
	}

	foundReflection := false
	for _, msg := range msgs {
		if strings.Contains(msg.Content, "[Reflection]") && strings.Contains(msg.Content, "need different approach") {
			foundReflection = true
			break
		}
	}
	if !foundReflection {
		t.Error("expected reflection message in conversation history")
	}

	// Working memory should have the new plan.
	wm, ok := result.Outputs["_working_memory"].(map[string]any)
	if !ok {
		t.Fatal("expected working memory to be map")
	}
	if wm["plan"] != "try plan B" {
		t.Errorf("expected plan 'try plan B', got %v", wm["plan"])
	}
}

func TestSuperagent_MaxReflections(t *testing.T) {
	// Set max_reflections=1 via override. Evaluator keeps saying stuck.
	maxReflections := 1
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			// Iteration 0: text response
			{Content: "working", FinishReason: "stop"},
			// Evaluator 0: stuck
			{
				Content:      `{"needs_more_work":true,"stuck_detected":true,"should_compact":false,"reasoning":"stuck"}`,
				FinishReason: "stop",
			},
			// Reflection 1
			{
				Content:      `{"reflection_text":"trying again","new_plan":"plan C"}`,
				FinishReason: "stop",
			},
			// Iteration 1: text response
			{Content: "still stuck", FinishReason: "stop"},
			// Evaluator 1: still stuck
			{
				Content:      `{"needs_more_work":true,"stuck_detected":true,"should_compact":false,"reasoning":"still stuck"}`,
				FinishReason: "stop",
			},
		},
	}

	mcpClient := baseMCPClient()
	deps := newSuperagentDeps(provider, mcpClient)
	cfg := baseSuperagentConfig()
	cfg.Overrides = &model.SuperagentOverrides{
		Reflection: &model.ReflectionOverride{
			MaxReflections: &maxReflections,
		},
	}
	node := validSuperagentNode(cfg)
	inputs := map[string]any{"task": "max reflections test"}

	exec := &SuperagentExecutor{}
	result, err := exec.Execute(context.Background(), node, inputs, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outputs["_finish_reason"] != "stuck" {
		t.Errorf("expected _finish_reason=stuck, got %v", result.Outputs["_finish_reason"])
	}
}

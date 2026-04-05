package model

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestGraphRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	g := Graph{
		ID:        "g-1",
		TenantID:  "t-1",
		Name:      "test-graph",
		Namespace: "default",
		Version:   1,
		Status:    GraphStatusActive,
		Nodes: []Node{
			{
				ID:   "n-1",
				Name: "input",
				Type: NodeTypeInput,
				InputPorts: []Port{
					{Name: "query", Schema: json.RawMessage(`{"type":"string"}`)},
				},
				OutputPorts: []Port{
					{Name: "result", Schema: json.RawMessage(`{"type":"string"}`)},
				},
				Config: json.RawMessage(`{}`),
			},
		},
		Edges: []Edge{
			{
				ID:           "e-1",
				SourceNodeID: "n-1",
				SourcePort:   "result",
				TargetNodeID: "n-2",
				TargetPort:   "input",
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	data, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Graph
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != g.ID {
		t.Errorf("ID: got %q, want %q", got.ID, g.ID)
	}
	if got.Name != g.Name {
		t.Errorf("Name: got %q, want %q", got.Name, g.Name)
	}
	if got.Status != g.Status {
		t.Errorf("Status: got %q, want %q", got.Status, g.Status)
	}
	if len(got.Nodes) != 1 {
		t.Fatalf("Nodes: got %d, want 1", len(got.Nodes))
	}
	if got.Nodes[0].Type != NodeTypeInput {
		t.Errorf("Node.Type: got %q, want %q", got.Nodes[0].Type, NodeTypeInput)
	}
	if len(got.Edges) != 1 {
		t.Fatalf("Edges: got %d, want 1", len(got.Edges))
	}
	if got.Edges[0].SourceNodeID != "n-1" {
		t.Errorf("Edge.SourceNodeID: got %q, want %q", got.Edges[0].SourceNodeID, "n-1")
	}
	if !got.CreatedAt.Equal(g.CreatedAt) {
		t.Errorf("CreatedAt: got %v, want %v", got.CreatedAt, g.CreatedAt)
	}
}

func TestEnumSerialization(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		// GraphStatus
		{"GraphStatusDraft", GraphStatusDraft, `"draft"`},
		{"GraphStatusActive", GraphStatusActive, `"active"`},
		{"GraphStatusArchived", GraphStatusArchived, `"archived"`},
		// ExecutionStatus
		{"ExecutionStatusPending", ExecutionStatusPending, `"pending"`},
		{"ExecutionStatusRunning", ExecutionStatusRunning, `"running"`},
		{"ExecutionStatusCompleted", ExecutionStatusCompleted, `"completed"`},
		{"ExecutionStatusFailed", ExecutionStatusFailed, `"failed"`},
		{"ExecutionStatusCancelled", ExecutionStatusCancelled, `"cancelled"`},
		{"ExecutionStatusTimedOut", ExecutionStatusTimedOut, `"timed_out"`},
		// StepStatus
		{"StepStatusPending", StepStatusPending, `"pending"`},
		{"StepStatusRunning", StepStatusRunning, `"running"`},
		{"StepStatusCompleted", StepStatusCompleted, `"completed"`},
		{"StepStatusFailed", StepStatusFailed, `"failed"`},
		{"StepStatusSkipped", StepStatusSkipped, `"skipped"`},
		{"StepStatusRetrying", StepStatusRetrying, `"retrying"`},
		// Reducer
		{"ReducerReplace", ReducerReplace, `"replace"`},
		{"ReducerAppend", ReducerAppend, `"append"`},
		{"ReducerMerge", ReducerMerge, `"merge"`},
		// ExecutionTrigger
		{"TriggerAPI", TriggerAPI, `"api"`},
		{"TriggerUI", TriggerUI, `"ui"`},
		{"TriggerCLI", TriggerCLI, `"cli"`},
		{"TriggerTerraform", TriggerTerraform, `"terraform"`},
		{"TriggerMCP", TriggerMCP, `"mcp"`},
		{"TriggerScheduled", TriggerScheduled, `"scheduled"`},
		// ExecutionMode
		{"ExecutionModeSync", ExecutionModeSync, `"sync"`},
		{"ExecutionModeAsync", ExecutionModeAsync, `"async"`},
		// ResponseFormat
		{"ResponseFormatText", ResponseFormatText, `"text"`},
		{"ResponseFormatJSON", ResponseFormatJSON, `"json"`},
		// ProviderType
		{"ProviderOpenAI", ProviderOpenAI, `"openai"`},
		{"ProviderAnthropic", ProviderAnthropic, `"anthropic"`},
		{"ProviderGoogle", ProviderGoogle, `"google"`},
		{"ProviderOpenRouter", ProviderOpenRouter, `"openrouter"`},
		{"ProviderBedrock", ProviderBedrock, `"bedrock"`},
		{"ProviderCustom", ProviderCustom, `"custom"`},
		// EventType
		{"EventExecutionStarted", EventExecutionStarted, `"execution_started"`},
		{"EventExecutionCompleted", EventExecutionCompleted, `"execution_completed"`},
		{"EventNodeStarted", EventNodeStarted, `"node_started"`},
		{"EventNodeCompleted", EventNodeCompleted, `"node_completed"`},
		{"EventLLMToken", EventLLMToken, `"llm_token"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(data) != tc.want {
				t.Errorf("got %s, want %s", data, tc.want)
			}
		})
	}
}

func TestPortIsRequired(t *testing.T) {
	// Default (nil Required) should be true
	p := Port{Name: "test", Schema: json.RawMessage(`{}`)}
	if !p.IsRequired() {
		t.Error("expected default Port to be required")
	}

	// Explicit true
	tr := true
	p.Required = &tr
	if !p.IsRequired() {
		t.Error("expected Port with Required=true to be required")
	}

	// Explicit false
	fa := false
	p.Required = &fa
	if p.IsRequired() {
		t.Error("expected Port with Required=false to not be required")
	}
}

func TestExecutionEventSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	idx := 2
	total := 5

	ev := ExecutionEvent{
		Type:        EventNodeCompleted,
		ExecutionID: "exec-1",
		Timestamp:   now,
		NodeID:      "n-1",
		NodeType:    NodeTypeLLM,
		DurationMs:  150,
		LLMUsage: &LLMUsage{
			Provider:         "openai",
			Model:            "gpt-4",
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
		ItemIndex: &idx,
		ItemTotal: &total,
		Output:    json.RawMessage(`{"result":"ok"}`),
	}

	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ExecutionEvent
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Type != EventNodeCompleted {
		t.Errorf("Type: got %q, want %q", got.Type, EventNodeCompleted)
	}
	if got.ExecutionID != "exec-1" {
		t.Errorf("ExecutionID: got %q, want %q", got.ExecutionID, "exec-1")
	}
	if got.NodeID != "n-1" {
		t.Errorf("NodeID: got %q, want %q", got.NodeID, "n-1")
	}
	if got.LLMUsage == nil {
		t.Fatal("LLMUsage: got nil, want non-nil")
	}
	if got.LLMUsage.TotalTokens != 150 {
		t.Errorf("LLMUsage.TotalTokens: got %d, want 150", got.LLMUsage.TotalTokens)
	}
	if got.ItemIndex == nil || *got.ItemIndex != 2 {
		t.Errorf("ItemIndex: got %v, want 2", got.ItemIndex)
	}
	if got.ItemTotal == nil || *got.ItemTotal != 5 {
		t.Errorf("ItemTotal: got %v, want 5", got.ItemTotal)
	}
	if got.DurationMs != 150 {
		t.Errorf("DurationMs: got %d, want 150", got.DurationMs)
	}
	if !got.Timestamp.Equal(now) {
		t.Errorf("Timestamp: got %v, want %v", got.Timestamp, now)
	}

	// Verify omitted optional fields are absent from JSON
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if _, ok := raw["error"]; ok {
		t.Error("expected error field to be omitted when nil")
	}
	if _, ok := raw["state"]; ok {
		t.Error("expected state field to be omitted when nil")
	}
}

func TestExecutionEventMinimal(t *testing.T) {
	// Minimal event with only required fields
	ev := ExecutionEvent{
		Type:        EventExecutionStarted,
		ExecutionID: "exec-2",
		Timestamp:   time.Now().UTC(),
	}

	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ExecutionEvent
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Type != EventExecutionStarted {
		t.Errorf("Type: got %q, want %q", got.Type, EventExecutionStarted)
	}
	if got.NodeID != "" {
		t.Errorf("NodeID: got %q, want empty", got.NodeID)
	}
	if got.LLMUsage != nil {
		t.Error("LLMUsage: got non-nil, want nil")
	}
	if got.Error != nil {
		t.Error("Error: got non-nil, want nil")
	}
	if got.ItemIndex != nil {
		t.Error("ItemIndex: got non-nil, want nil")
	}
}

func intPtr(v int) *int             { return &v }
func float64Ptr(v float64) *float64 { return &v }
func boolPtr(v bool) *bool          { return &v }

func TestSuperagentNodeConfig_RoundTrip(t *testing.T) {
	cfg := SuperagentNodeConfig{
		Prompt: "Solve the task",
		Skills: []SuperagentSkill{
			{
				Name:           "web-search",
				Description:    "Search the web",
				MCPURL:         "http://localhost:8080/mcp",
				MCPTransport:   "http",
				Headers:        []HeaderConfig{{Name: "Authorization", Value: "Bearer tok"}},
				PromptFragment: "Use this to search",
				Tools:          []string{"search", "fetch"},
				TimeoutSeconds: intPtr(30),
			},
		},
		Provider:                     ProviderAnthropic,
		Model:                        "claude-sonnet-4-20250514",
		APIKey:                       "sk-test",
		APIKeyRef:                    "vault:anthropic-key",
		SystemPreamble:               "You are a helpful agent.",
		MaxIterations:                intPtr(20),
		MaxTotalToolCalls:            intPtr(100),
		MaxToolCallsPerIteration:     intPtr(10),
		MaxToolLoopRounds:            intPtr(5),
		TimeoutSeconds:               intPtr(300),
		Temperature:                  float64Ptr(0.7),
		MaxTokens:                    intPtr(4096),
		ConversationHistoryFromInput: "messages",
		SharedMemory: &SharedMemoryConfig{
			Enabled:       true,
			Namespace:     "project-x",
			InjectOnStart: boolPtr(true),
			AutoFlush:     boolPtr(false),
		},
		ToolPolicies: &ToolPolicies{
			Allowed:         []string{"search", "fetch"},
			Denied:          []string{"delete"},
			RequireApproval: []string{"deploy"},
		},
		Overrides: &SuperagentOverrides{
			Evaluator: &EvaluatorOverride{
				Provider:  ProviderOpenAI,
				Model:     "gpt-4o",
				APIKey:    "sk-eval",
				APIKeyRef: "vault:eval-key",
				Prompt:    "Evaluate progress",
				Disabled:  false,
			},
			Reflection: &ReflectionOverride{
				Provider:       ProviderAnthropic,
				Model:          "claude-sonnet-4-20250514",
				APIKey:         "sk-reflect",
				APIKeyRef:      "vault:reflect-key",
				Prompt:         "Reflect on actions",
				MaxReflections: intPtr(3),
				Disabled:       false,
			},
			ContextCompaction: &ContextCompactionOverride{
				Enabled:                boolPtr(true),
				Provider:               ProviderOpenAI,
				Model:                  "gpt-4o-mini",
				APIKey:                 "sk-compact",
				APIKeyRef:              "vault:compact-key",
				Prompt:                 "Summarize context",
				ContextWindowLimit:     intPtr(128000),
				CompactionThreshold:    float64Ptr(0.8),
				PreserveRecentMessages: intPtr(5),
			},
			StuckDetection: &StuckDetectionOverride{
				Enabled:         boolPtr(true),
				WindowSize:      intPtr(5),
				RepeatThreshold: intPtr(3),
			},
			PromptAssembly: &PromptAssemblyOverride{
				Template:        "custom-template",
				ToolConventions: "Always confirm before destructive actions",
				Style:           "concise",
			},
			OutputExtraction: &OutputExtractionOverride{
				Prompt:    "Extract final answer",
				Provider:  ProviderAnthropic,
				Model:     "claude-sonnet-4-20250514",
				APIKey:    "sk-extract",
				APIKeyRef: "vault:extract-key",
			},
			TaskTracking: &TaskTrackingOverride{
				Enabled:           boolPtr(true),
				ReminderFrequency: intPtr(3),
			},
		},
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got SuperagentNodeConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(cfg, got) {
		t.Errorf("round-trip mismatch:\n  got:  %+v\n  want: %+v", got, cfg)
	}
}

func TestSuperagentNodeConfig_Defaults(t *testing.T) {
	cfg := SuperagentNodeConfig{
		Prompt: "Do the thing",
		Skills: []SuperagentSkill{
			{Name: "tool", Description: "A tool", MCPURL: "http://localhost/mcp"},
		},
		Provider: ProviderOpenAI,
		Model:    "gpt-4o",
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got SuperagentNodeConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Required fields preserved
	if got.Prompt != "Do the thing" {
		t.Errorf("Prompt: got %q, want %q", got.Prompt, "Do the thing")
	}
	if got.Provider != ProviderOpenAI {
		t.Errorf("Provider: got %q, want %q", got.Provider, ProviderOpenAI)
	}
	if got.Model != "gpt-4o" {
		t.Errorf("Model: got %q, want %q", got.Model, "gpt-4o")
	}
	if len(got.Skills) != 1 {
		t.Fatalf("Skills: got %d, want 1", len(got.Skills))
	}

	// All optional pointer fields nil
	if got.MaxIterations != nil {
		t.Error("MaxIterations: got non-nil, want nil")
	}
	if got.MaxTotalToolCalls != nil {
		t.Error("MaxTotalToolCalls: got non-nil, want nil")
	}
	if got.MaxToolCallsPerIteration != nil {
		t.Error("MaxToolCallsPerIteration: got non-nil, want nil")
	}
	if got.MaxToolLoopRounds != nil {
		t.Error("MaxToolLoopRounds: got non-nil, want nil")
	}
	if got.TimeoutSeconds != nil {
		t.Error("TimeoutSeconds: got non-nil, want nil")
	}
	if got.Temperature != nil {
		t.Error("Temperature: got non-nil, want nil")
	}
	if got.MaxTokens != nil {
		t.Error("MaxTokens: got non-nil, want nil")
	}
	if got.SharedMemory != nil {
		t.Error("SharedMemory: got non-nil, want nil")
	}
	if got.ToolPolicies != nil {
		t.Error("ToolPolicies: got non-nil, want nil")
	}
	if got.Overrides != nil {
		t.Error("Overrides: got non-nil, want nil")
	}
}

func TestSuperagentEventTypes(t *testing.T) {
	tests := []struct {
		name  string
		value EventType
		want  string
	}{
		{"EventSuperagentStarted", EventSuperagentStarted, "superagent_started"},
		{"EventSuperagentIteration", EventSuperagentIteration, "superagent_iteration"},
		{"EventSuperagentEvaluation", EventSuperagentEvaluation, "superagent_evaluation"},
		{"EventSuperagentReflection", EventSuperagentReflection, "superagent_reflection"},
		{"EventSuperagentStuckWarning", EventSuperagentStuckWarning, "superagent_stuck_warning"},
		{"EventSuperagentCompaction", EventSuperagentCompaction, "superagent_compaction"},
		{"EventSuperagentMemoryStore", EventSuperagentMemoryStore, "superagent_memory_store"},
		{"EventSuperagentBufferFinalize", EventSuperagentBufferFinalize, "superagent_buffer_finalize"},
		{"EventSuperagentToolCall", EventSuperagentToolCall, "superagent_tool_call"},
		{"EventSuperagentCompleted", EventSuperagentCompleted, "superagent_completed"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.value) != tc.want {
				t.Errorf("got %q, want %q", tc.value, tc.want)
			}
		})
	}
}

func TestNodeTypeSuperagent(t *testing.T) {
	if NodeTypeSuperagent != "superagent" {
		t.Errorf("NodeTypeSuperagent: got %q, want %q", NodeTypeSuperagent, "superagent")
	}
}

func TestCodeExecutionConfig_RoundTrip(t *testing.T) {
	cfg := SuperagentNodeConfig{
		Prompt: "Compute things",
		Skills: []SuperagentSkill{
			{Name: "tool", Description: "A tool", MCPURL: "http://localhost/mcp"},
		},
		Provider: ProviderOpenAI,
		Model:    "gpt-4",
		APIKey:   "sk-test",
		CodeExecution: &CodeExecutionConfig{
			Enabled:                  true,
			MaxExecutionTimeSec:      intPtr(60),
			MaxMemoryMB:              intPtr(512),
			MaxOutputBytes:           intPtr(2097152),
			MaxCodeBytes:             intPtr(131072),
			MaxToolCallsPerExecution: intPtr(100),
			MaxExecutionsPerRun:      intPtr(30),
			AllowedModules:           []string{"json", "math", "csv"},
		},
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got SuperagentNodeConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.CodeExecution == nil {
		t.Fatal("expected code_execution to be present")
	}
	if !got.CodeExecution.Enabled {
		t.Error("expected code_execution.enabled = true")
	}
	if *got.CodeExecution.MaxExecutionTimeSec != 60 {
		t.Errorf("expected max_execution_time_sec 60, got %d", *got.CodeExecution.MaxExecutionTimeSec)
	}
	if *got.CodeExecution.MaxMemoryMB != 512 {
		t.Errorf("expected max_memory_mb 512, got %d", *got.CodeExecution.MaxMemoryMB)
	}
	if len(got.CodeExecution.AllowedModules) != 3 {
		t.Errorf("expected 3 allowed modules, got %d", len(got.CodeExecution.AllowedModules))
	}
}

func TestCodeExecutionConfig_OmittedWhenNil(t *testing.T) {
	cfg := SuperagentNodeConfig{
		Prompt: "Do things",
		Skills: []SuperagentSkill{
			{Name: "tool", Description: "A tool", MCPURL: "http://localhost/mcp"},
		},
		Provider: ProviderOpenAI,
		Model:    "gpt-4",
		APIKey:   "sk-test",
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// code_execution should not appear in JSON when nil.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if _, ok := raw["code_execution"]; ok {
		t.Error("expected code_execution to be omitted when nil")
	}
}

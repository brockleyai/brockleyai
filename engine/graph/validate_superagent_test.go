package graph

import (
	"encoding/json"
	"testing"

	"github.com/brockleyai/brockleyai/internal/model"
)

// makeSuperagentGraph creates a valid base graph with a superagent node.
func makeSuperagentGraph(cfg model.SuperagentNodeConfig) *model.Graph {
	return &model.Graph{
		Nodes: []model.Node{
			{
				ID: "input", Name: "input", Type: model.NodeTypeInput,
				OutputPorts: []model.Port{{Name: "task", Schema: json.RawMessage(`{"type":"string"}`)}},
			},
			{
				ID: "agent", Name: "agent", Type: model.NodeTypeSuperagent,
				Config: mustJSON(cfg),
				InputPorts: []model.Port{
					{Name: "task", Schema: json.RawMessage(`{"type":"string"}`)},
				},
				OutputPorts: []model.Port{
					{Name: "result", Schema: json.RawMessage(`{"type":"string"}`)},
				},
			},
			{
				ID: "output", Name: "output", Type: model.NodeTypeOutput,
				InputPorts: []model.Port{{Name: "result", Schema: json.RawMessage(`{"type":"string"}`)}},
			},
		},
		Edges: []model.Edge{
			{ID: "e1", SourceNodeID: "input", SourcePort: "task", TargetNodeID: "agent", TargetPort: "task"},
			{ID: "e2", SourceNodeID: "agent", SourcePort: "result", TargetNodeID: "output", TargetPort: "result"},
		},
	}
}

func validSuperagentConfig() model.SuperagentNodeConfig {
	return model.SuperagentNodeConfig{
		Prompt:   "Do the task: {{input.task}}",
		Provider: "openai",
		Model:    "gpt-4",
		APIKey:   "sk-test-key",
		Skills: []model.SuperagentSkill{
			{
				Name:        "code",
				Description: "Code editing tools",
				MCPURL:      "http://mcp:9001",
			},
		},
	}
}

func TestValidateSuperagent(t *testing.T) {
	tests := []struct {
		name       string
		modify     func(cfg *model.SuperagentNodeConfig, g *model.Graph)
		wantValid  bool
		wantCode   string // expected error code (if invalid)
		wantWarn   string // expected warning code (if valid but has warning)
		skipResync bool   // if true, don't re-serialize cfg into graph (for raw JSON tests)
	}{
		{
			name:      "valid config",
			modify:    func(cfg *model.SuperagentNodeConfig, g *model.Graph) {},
			wantValid: true,
		},
		{
			name: "missing prompt",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				cfg.Prompt = ""
			},
			wantValid: false,
			wantCode:  "SUPERAGENT_MISSING_CONFIG",
		},
		{
			name: "missing skills",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				cfg.Skills = nil
			},
			wantValid: false,
			wantCode:  "SUPERAGENT_MISSING_CONFIG",
		},
		{
			name: "missing provider",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				cfg.Provider = ""
			},
			wantValid: false,
			wantCode:  "SUPERAGENT_MISSING_CONFIG",
		},
		{
			name: "missing model",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				cfg.Model = ""
			},
			wantValid: false,
			wantCode:  "SUPERAGENT_MISSING_CONFIG",
		},
		{
			name: "missing api_key and api_key_ref",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				cfg.APIKey = ""
				cfg.APIKeyRef = ""
			},
			wantValid: false,
			wantCode:  "SUPERAGENT_MISSING_CONFIG",
		},
		{
			name: "api_key_ref is sufficient",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				cfg.APIKey = ""
				cfg.APIKeyRef = "my-secret"
			},
			wantValid: true,
		},
		{
			name: "skill missing name",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				cfg.Skills[0].Name = ""
			},
			wantValid: false,
			wantCode:  "SUPERAGENT_INVALID_SKILL",
		},
		{
			name: "skill missing description",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				cfg.Skills[0].Description = ""
			},
			wantValid: false,
			wantCode:  "SUPERAGENT_INVALID_SKILL",
		},
		{
			name: "skill missing mcp_url",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				cfg.Skills[0].MCPURL = ""
			},
			wantValid: false,
			wantCode:  "SUPERAGENT_INVALID_SKILL",
		},
		{
			name: "no output ports",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				for i := range g.Nodes {
					if g.Nodes[i].ID == "agent" {
						g.Nodes[i].OutputPorts = nil
					}
				}
				// Remove edge that references the output port.
				g.Edges = g.Edges[:1]
				// Also remove the output node's required input port reference.
				for i := range g.Nodes {
					if g.Nodes[i].ID == "output" {
						g.Nodes[i].InputPorts = nil
					}
				}
			},
			wantValid: false,
			wantCode:  "SUPERAGENT_NO_OUTPUT",
		},
		{
			name: "shared memory without state field",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				cfg.SharedMemory = &model.SharedMemoryConfig{Enabled: true}
				// Add state reads/writes on the node but no state schema.
				for i := range g.Nodes {
					if g.Nodes[i].ID == "agent" {
						g.Nodes[i].StateReads = []model.StateBinding{
							{StateField: "_superagent_memory", Port: "_memory_in"},
						}
						g.Nodes[i].StateWrites = []model.StateBinding{
							{StateField: "_superagent_memory", Port: "_memory_out"},
						}
						g.Nodes[i].InputPorts = append(g.Nodes[i].InputPorts,
							model.Port{Name: "_memory_in", Schema: json.RawMessage(`{"type":"object","properties":{"dummy":{"type":"string"}}}`)},
						)
						g.Nodes[i].OutputPorts = append(g.Nodes[i].OutputPorts,
							model.Port{Name: "_memory_out", Schema: json.RawMessage(`{"type":"object","properties":{"dummy":{"type":"string"}}}`)},
						)
					}
				}
			},
			wantValid: false,
			wantCode:  "SUPERAGENT_MISSING_SHARED_MEMORY_STATE",
		},
		{
			name: "shared memory with proper state",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				cfg.SharedMemory = &model.SharedMemoryConfig{Enabled: true}
				g.State = &model.GraphState{
					Fields: []model.StateField{
						{
							Name:    "_superagent_memory",
							Schema:  json.RawMessage(`{"type":"object","properties":{"dummy":{"type":"string"}}}`),
							Reducer: model.ReducerMerge,
						},
					},
				}
				for i := range g.Nodes {
					if g.Nodes[i].ID == "agent" {
						g.Nodes[i].StateReads = []model.StateBinding{
							{StateField: "_superagent_memory", Port: "_memory_in"},
						}
						g.Nodes[i].StateWrites = []model.StateBinding{
							{StateField: "_superagent_memory", Port: "_memory_out"},
						}
						g.Nodes[i].InputPorts = append(g.Nodes[i].InputPorts,
							model.Port{Name: "_memory_in", Schema: json.RawMessage(`{"type":"object","properties":{"dummy":{"type":"string"}}}`)},
						)
						g.Nodes[i].OutputPorts = append(g.Nodes[i].OutputPorts,
							model.Port{Name: "_memory_out", Schema: json.RawMessage(`{"type":"object","properties":{"dummy":{"type":"string"}}}`)},
						)
					}
				}
			},
			wantValid: true,
		},
		{
			name: "shared memory without state reads/writes",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				cfg.SharedMemory = &model.SharedMemoryConfig{Enabled: true}
				g.State = &model.GraphState{
					Fields: []model.StateField{
						{
							Name:    "_superagent_memory",
							Schema:  json.RawMessage(`{"type":"object","properties":{"dummy":{"type":"string"}}}`),
							Reducer: model.ReducerMerge,
						},
					},
				}
				// No state reads/writes on the node.
			},
			wantValid: false,
			wantCode:  "SUPERAGENT_MISSING_SHARED_MEMORY_STATE",
		},
		{
			name: "override provider without model",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				cfg.Overrides = &model.SuperagentOverrides{
					Evaluator: &model.EvaluatorOverride{
						Provider: "anthropic",
						// Model is empty.
					},
				}
			},
			wantValid: false,
			wantCode:  "SUPERAGENT_INVALID_OVERRIDE",
		},
		{
			name: "override provider with model is valid",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				cfg.Overrides = &model.SuperagentOverrides{
					Evaluator: &model.EvaluatorOverride{
						Provider: "anthropic",
						Model:    "claude-3-opus",
					},
				}
			},
			wantValid: true,
		},
		{
			name: "conversation_history_from_input references missing port",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				cfg.ConversationHistoryFromInput = "history"
				// No "history" input port on the node.
			},
			wantValid: false,
			wantCode:  "SUPERAGENT_MISSING_CONFIG",
		},
		{
			name: "template var warning for missing input port",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				cfg.Prompt = "Do {{input.missing_var}}"
			},
			wantValid: true,
			wantWarn:  "SUPERAGENT_TEMPLATE_VAR_MISSING",
		},
		{
			name: "template var matches declared input port",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				cfg.Prompt = "Do {{input.task}}"
			},
			wantValid: true,
		},
		// --- New validation rules ---
		{
			name: "malformed config JSON",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				for i := range g.Nodes {
					if g.Nodes[i].ID == "agent" {
						g.Nodes[i].Config = json.RawMessage(`{invalid json}`)
					}
				}
			},
			wantValid:  false,
			wantCode:   "SUPERAGENT_MISSING_CONFIG",
			skipResync: true,
		},
		{
			name: "max_iterations zero",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				zero := 0
				cfg.MaxIterations = &zero
			},
			wantValid: false,
			wantCode:  "SUPERAGENT_MISSING_CONFIG",
		},
		{
			name: "max_iterations negative",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				neg := -1
				cfg.MaxIterations = &neg
			},
			wantValid: false,
			wantCode:  "SUPERAGENT_MISSING_CONFIG",
		},
		{
			name: "max_total_tool_calls zero",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				zero := 0
				cfg.MaxTotalToolCalls = &zero
			},
			wantValid: false,
			wantCode:  "SUPERAGENT_MISSING_CONFIG",
		},
		{
			name: "max_tool_calls_per_iteration zero",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				zero := 0
				cfg.MaxToolCallsPerIteration = &zero
			},
			wantValid: false,
			wantCode:  "SUPERAGENT_MISSING_CONFIG",
		},
		{
			name: "timeout_seconds zero",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				zero := 0
				cfg.TimeoutSeconds = &zero
			},
			wantValid: false,
			wantCode:  "SUPERAGENT_MISSING_CONFIG",
		},
		{
			name: "positive limits are valid",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				five := 5
				cfg.MaxIterations = &five
				cfg.MaxTotalToolCalls = &five
				cfg.MaxToolCallsPerIteration = &five
				cfg.TimeoutSeconds = &five
			},
			wantValid: true,
		},
		{
			name: "stuck detection window_size zero causes panic",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				zero := 0
				cfg.Overrides = &model.SuperagentOverrides{
					StuckDetection: &model.StuckDetectionOverride{
						WindowSize: &zero,
					},
				}
			},
			wantValid: false,
			wantCode:  "SUPERAGENT_INVALID_OVERRIDE",
		},
		{
			name: "stuck detection window_size positive is valid",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				ten := 10
				cfg.Overrides = &model.SuperagentOverrides{
					StuckDetection: &model.StuckDetectionOverride{
						WindowSize: &ten,
					},
				}
			},
			wantValid: true,
		},
		{
			name: "compaction threshold zero",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				zero := 0.0
				cfg.Overrides = &model.SuperagentOverrides{
					ContextCompaction: &model.ContextCompactionOverride{
						CompactionThreshold: &zero,
					},
				}
			},
			wantValid: false,
			wantCode:  "SUPERAGENT_INVALID_OVERRIDE",
		},
		{
			name: "compaction threshold above 1.0",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				high := 1.5
				cfg.Overrides = &model.SuperagentOverrides{
					ContextCompaction: &model.ContextCompactionOverride{
						CompactionThreshold: &high,
					},
				}
			},
			wantValid: false,
			wantCode:  "SUPERAGENT_INVALID_OVERRIDE",
		},
		{
			name: "compaction threshold 0.8 is valid",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				valid := 0.8
				cfg.Overrides = &model.SuperagentOverrides{
					ContextCompaction: &model.ContextCompactionOverride{
						CompactionThreshold: &valid,
					},
				}
			},
			wantValid: true,
		},
		{
			name: "override different provider without credentials warns",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				cfg.Overrides = &model.SuperagentOverrides{
					Evaluator: &model.EvaluatorOverride{
						Provider: "anthropic",
						Model:    "claude-3-opus",
						// No api_key or api_key_ref — different provider than main (openai).
					},
				}
			},
			wantValid: true,
			wantWarn:  "SUPERAGENT_INVALID_OVERRIDE",
		},
		{
			name: "override different provider with api_key is valid",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				cfg.Overrides = &model.SuperagentOverrides{
					Evaluator: &model.EvaluatorOverride{
						Provider: "anthropic",
						Model:    "claude-3-opus",
						APIKey:   "sk-anthropic-key",
					},
				}
			},
			wantValid: true,
		},
		{
			name: "override same provider without credentials is valid",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				cfg.Overrides = &model.SuperagentOverrides{
					Evaluator: &model.EvaluatorOverride{
						Provider: "openai", // Same as main.
						Model:    "gpt-3.5-turbo",
					},
				}
			},
			wantValid: true,
		},
		{
			name: "compacted skill without mcp_url",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				cfg.Skills = []model.SuperagentSkill{
					{
						Name:        "api-skill",
						Description: "API tool skill",
						APIToolID:   "api-1",
						Endpoints:   []string{"ep1"},
						Compacted:   true, // compacted only makes sense with MCP
					},
				}
			},
			wantValid: false,
			wantCode:  "SUPERAGENT_COMPACTED_NO_MCP",
		},
		{
			name: "compacted skill with no description or tools warns",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				cfg.Skills = []model.SuperagentSkill{
					{
						Name:        "compacted-skill",
						Description: "", // empty description
						MCPURL:      "http://mcp:9001",
						Compacted:   true,
						// No tools listed
					},
				}
			},
			wantValid: false, // description is required regardless of compacted
			wantCode:  "SUPERAGENT_INVALID_SKILL",
		},
		{
			name: "compacted skill with description is valid",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				cfg.Skills = []model.SuperagentSkill{
					{
						Name:        "compacted-skill",
						Description: "Knowledge base MCP server for searching documents",
						MCPURL:      "http://mcp:9001",
						Compacted:   true,
					},
				}
			},
			wantValid: true,
		},
		{
			name: "compacted skill with tools is valid",
			modify: func(cfg *model.SuperagentNodeConfig, g *model.Graph) {
				cfg.Skills = []model.SuperagentSkill{
					{
						Name:        "compacted-skill",
						Description: "KB tools",
						MCPURL:      "http://mcp:9001",
						Compacted:   true,
						Tools:       []string{"search_kb", "get_document"},
					},
				}
			},
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validSuperagentConfig()
			g := makeSuperagentGraph(cfg)

			// Apply modifications.
			// Re-parse cfg from graph node since makeSuperagentGraph used a copy.
			tt.modify(&cfg, g)
			// Re-serialize config into the graph node (unless test sets raw JSON directly).
			if !tt.skipResync {
				for i := range g.Nodes {
					if g.Nodes[i].ID == "agent" {
						g.Nodes[i].Config = mustJSON(cfg)
					}
				}
			}

			result := Validate(g)

			if tt.wantValid && !result.Valid {
				t.Fatalf("expected valid, got errors: %+v", result.Errors)
			}
			if !tt.wantValid && result.Valid {
				t.Fatal("expected validation to fail but got valid")
			}

			if tt.wantCode != "" {
				found := false
				for _, e := range result.Errors {
					if e.Code == tt.wantCode {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error code %q, got errors: %+v", tt.wantCode, result.Errors)
				}
			}

			if tt.wantWarn != "" {
				found := false
				for _, w := range result.Warnings {
					if w.Code == tt.wantWarn {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected warning code %q, got warnings: %+v", tt.wantWarn, result.Warnings)
				}
			}
		})
	}
}

func TestValidateSuperagent_CodeExecution(t *testing.T) {
	intPtr := func(v int) *int { return &v }

	tests := []struct {
		name      string
		ce        *model.CodeExecutionConfig
		wantValid bool
		wantCode  string
	}{
		{
			name:      "code execution disabled",
			ce:        &model.CodeExecutionConfig{Enabled: false, MaxExecutionTimeSec: intPtr(-1)},
			wantValid: true, // validation skipped when disabled
		},
		{
			name:      "code execution enabled with defaults",
			ce:        &model.CodeExecutionConfig{Enabled: true},
			wantValid: true,
		},
		{
			name:      "code execution nil",
			ce:        nil,
			wantValid: true,
		},
		{
			name:      "max_execution_time_sec negative",
			ce:        &model.CodeExecutionConfig{Enabled: true, MaxExecutionTimeSec: intPtr(-1)},
			wantValid: false,
			wantCode:  "SUPERAGENT_INVALID_CODE_EXEC",
		},
		{
			name:      "max_execution_time_sec too large",
			ce:        &model.CodeExecutionConfig{Enabled: true, MaxExecutionTimeSec: intPtr(500)},
			wantValid: false,
			wantCode:  "SUPERAGENT_INVALID_CODE_EXEC",
		},
		{
			name:      "max_execution_time_sec valid",
			ce:        &model.CodeExecutionConfig{Enabled: true, MaxExecutionTimeSec: intPtr(60)},
			wantValid: true,
		},
		{
			name:      "max_memory_mb negative",
			ce:        &model.CodeExecutionConfig{Enabled: true, MaxMemoryMB: intPtr(-1)},
			wantValid: false,
			wantCode:  "SUPERAGENT_INVALID_CODE_EXEC",
		},
		{
			name:      "max_memory_mb too large",
			ce:        &model.CodeExecutionConfig{Enabled: true, MaxMemoryMB: intPtr(4096)},
			wantValid: false,
			wantCode:  "SUPERAGENT_INVALID_CODE_EXEC",
		},
		{
			name:      "max_output_bytes negative",
			ce:        &model.CodeExecutionConfig{Enabled: true, MaxOutputBytes: intPtr(0)},
			wantValid: false,
			wantCode:  "SUPERAGENT_INVALID_CODE_EXEC",
		},
		{
			name:      "max_output_bytes too large",
			ce:        &model.CodeExecutionConfig{Enabled: true, MaxOutputBytes: intPtr(20 * 1048576)},
			wantValid: false,
			wantCode:  "SUPERAGENT_INVALID_CODE_EXEC",
		},
		{
			name:      "max_code_bytes negative",
			ce:        &model.CodeExecutionConfig{Enabled: true, MaxCodeBytes: intPtr(-1)},
			wantValid: false,
			wantCode:  "SUPERAGENT_INVALID_CODE_EXEC",
		},
		{
			name:      "max_code_bytes too large",
			ce:        &model.CodeExecutionConfig{Enabled: true, MaxCodeBytes: intPtr(2 * 1048576)},
			wantValid: false,
			wantCode:  "SUPERAGENT_INVALID_CODE_EXEC",
		},
		{
			name:      "max_tool_calls_per_execution negative",
			ce:        &model.CodeExecutionConfig{Enabled: true, MaxToolCallsPerExecution: intPtr(0)},
			wantValid: false,
			wantCode:  "SUPERAGENT_INVALID_CODE_EXEC",
		},
		{
			name:      "max_tool_calls_per_execution too large",
			ce:        &model.CodeExecutionConfig{Enabled: true, MaxToolCallsPerExecution: intPtr(1000)},
			wantValid: false,
			wantCode:  "SUPERAGENT_INVALID_CODE_EXEC",
		},
		{
			name:      "max_executions_per_run negative",
			ce:        &model.CodeExecutionConfig{Enabled: true, MaxExecutionsPerRun: intPtr(-1)},
			wantValid: false,
			wantCode:  "SUPERAGENT_INVALID_CODE_EXEC",
		},
		{
			name:      "max_executions_per_run too large",
			ce:        &model.CodeExecutionConfig{Enabled: true, MaxExecutionsPerRun: intPtr(200)},
			wantValid: false,
			wantCode:  "SUPERAGENT_INVALID_CODE_EXEC",
		},
		{
			name:      "all limits valid",
			ce:        &model.CodeExecutionConfig{Enabled: true, MaxExecutionTimeSec: intPtr(30), MaxMemoryMB: intPtr(256), MaxOutputBytes: intPtr(1048576), MaxCodeBytes: intPtr(65536), MaxToolCallsPerExecution: intPtr(50), MaxExecutionsPerRun: intPtr(20)},
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validSuperagentConfig()
			cfg.CodeExecution = tt.ce

			g := makeSuperagentGraph(cfg)
			result := Validate(g)

			if tt.wantValid && !result.Valid {
				t.Errorf("expected valid, got errors: %+v", result.Errors)
			}
			if !tt.wantValid && result.Valid {
				t.Errorf("expected invalid, got valid")
			}
			if tt.wantCode != "" {
				found := false
				for _, e := range result.Errors {
					if e.Code == tt.wantCode {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error code %q, got: %+v", tt.wantCode, result.Errors)
				}
			}
		})
	}
}

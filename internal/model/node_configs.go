package model

import "encoding/json"

// LLMNodeConfig is the type-specific config for LLM nodes.
type LLMNodeConfig struct {
	Provider             ProviderType      `json:"provider"`
	Model                string            `json:"model"`
	APIKey               string            `json:"api_key,omitempty"`
	APIKeyRef            string            `json:"api_key_ref,omitempty"`
	BaseURL              string            `json:"base_url,omitempty"`
	BedrockRegion        string            `json:"bedrock_region,omitempty"`
	SystemPrompt         string            `json:"system_prompt,omitempty"`
	UserPrompt           string            `json:"user_prompt"`
	Messages             []PromptMessage   `json:"messages,omitempty"` // if set, takes priority over system_prompt/user_prompt
	Variables            []TemplateVar     `json:"variables"`
	Temperature          *float64          `json:"temperature,omitempty"`
	MaxTokens            *int              `json:"max_tokens,omitempty"`
	ResponseFormat       ResponseFormat    `json:"response_format"`
	OutputSchema         json.RawMessage   `json:"output_schema,omitempty"`
	ValidateOutput       *bool             `json:"validate_output,omitempty"`
	MaxValidationRetries *int              `json:"max_validation_retries,omitempty"` // default: 2
	ExtraHeaders         map[string]string `json:"extra_headers,omitempty"`

	// Tool calling: define tools available to this LLM.
	Tools      []LLMToolDefinition `json:"tools,omitempty"`
	ToolChoice string              `json:"tool_choice,omitempty"` // "auto" (default), "none", "required"

	// Tool loop: when true, the node runs an internal loop.
	ToolLoop             bool                 `json:"tool_loop,omitempty"`
	MaxToolCalls         *int                 `json:"max_tool_calls,omitempty"`      // total tool invocations (default: 25)
	MaxLoopIterations    *int                 `json:"max_loop_iterations,omitempty"` // LLM round-trips (default: 10)
	ToolRouting          map[string]ToolRoute `json:"tool_routing,omitempty"`
	ToolRoutingFromState string               `json:"tool_routing_from_state,omitempty"`
	ToolRoutingFromInput bool                 `json:"tool_routing_from_input,omitempty"`
	MessagesFromState    string               `json:"messages_from_state,omitempty"`

	// API tool references. Each ref selects a specific endpoint from an
	// API tool definition and auto-derives the tool schema + routing.
	// Merged with explicit Tools/ToolRouting at resolution time.
	APITools []APIToolRef `json:"api_tools,omitempty"`
}

// APIToolRef references a specific endpoint from an API tool definition.
// At resolution time, it auto-derives an LLMToolDefinition (from the endpoint's
// name, description, and input_schema) and a ToolRoute entry.
type APIToolRef struct {
	APIToolID string         `json:"api_tool_id"`         // library resource ID
	Endpoint  string         `json:"endpoint"`            // endpoint name within the definition
	ToolName  string         `json:"tool_name,omitempty"` // override tool name (default: endpoint name)
	Headers   []HeaderConfig `json:"headers,omitempty"`   // per-ref header overrides
}

// APIToolNodeConfig is the type-specific config for standalone api_tool nodes.
// Calls ONE API endpoint (like ToolNodeConfig calls one MCP tool).
type APIToolNodeConfig struct {
	// Reference to library resource + specific endpoint
	APIToolID string `json:"api_tool_id,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`

	// OR inline endpoint definition (self-contained graphs)
	InlineEndpoint *InlineAPIEndpoint `json:"inline_endpoint,omitempty"`

	// Header overrides (merged on top of definition defaults)
	Headers []HeaderConfig `json:"headers,omitempty"`
}

// InlineAPIEndpoint is a self-contained API endpoint for standalone nodes.
type InlineAPIEndpoint struct {
	BaseURL         string           `json:"base_url"`
	Method          string           `json:"method"`
	Path            string           `json:"path"`
	DefaultHeaders  []HeaderConfig   `json:"default_headers,omitempty"`
	InputSchema     json.RawMessage  `json:"input_schema,omitempty"`
	OutputSchema    json.RawMessage  `json:"output_schema,omitempty"`
	RequestMapping  *RequestMapping  `json:"request_mapping,omitempty"`
	ResponseMapping *ResponseMapping `json:"response_mapping,omitempty"`
	Retry           *RetryConfig     `json:"retry,omitempty"`
	TimeoutMs       *int             `json:"timeout_ms,omitempty"`
}

// ToolRoute maps a tool to its execution target.
// Exactly one of MCPURL or APIToolID+APIEndpoint must be set.
type ToolRoute struct {
	// MCP routing (existing)
	MCPURL       string `json:"mcp_url,omitempty"`
	MCPTransport string `json:"mcp_transport,omitempty"` // default: "http"

	// API endpoint routing (new)
	APIToolID   string `json:"api_tool_id,omitempty"`  // ref to APIToolDefinition library resource
	APIEndpoint string `json:"api_endpoint,omitempty"` // endpoint name within the definition

	// Shared
	Headers        []HeaderConfig `json:"headers,omitempty"`
	TimeoutSeconds *int           `json:"timeout_seconds,omitempty"` // per-call timeout (default: 30)
	Compacted      bool           `json:"compacted,omitempty"`       // opt-in compacted discovery mode
}

// PromptMessage is a single message in a prompt chain.
type PromptMessage struct {
	Role    string `json:"role"`    // "system", "user", "assistant"
	Content string `json:"content"` // template string, supports {{input.x}} variables
}

// TemplateVar declares a template variable with a typed schema.
type TemplateVar struct {
	Name        string          `json:"name"`
	Schema      json.RawMessage `json:"schema"`
	Description string          `json:"description,omitempty"`
}

// ToolNodeConfig is the type-specific config for tool (MCP) nodes.
type ToolNodeConfig struct {
	ToolName     string         `json:"tool_name"`
	MCPURL       string         `json:"mcp_url"`
	MCPTransport string         `json:"mcp_transport,omitempty"` // "sse" (default) or "stdio"
	Headers      []HeaderConfig `json:"headers,omitempty"`
}

// HeaderConfig defines a custom HTTP header for MCP connections.
type HeaderConfig struct {
	Name      string `json:"name"`
	Value     string `json:"value,omitempty"`      // static value
	FromInput string `json:"from_input,omitempty"` // dynamic: input port name
	SecretRef string `json:"secret_ref,omitempty"` // secret store reference
}

// ConditionalNodeConfig is the type-specific config for conditional nodes.
type ConditionalNodeConfig struct {
	Branches     []Branch `json:"branches"`
	DefaultLabel string   `json:"default_label"`
}

// Branch defines a single conditional branch with a label and condition expression.
type Branch struct {
	Label     string `json:"label"`
	Condition string `json:"condition"`
}

// TransformNodeConfig is the type-specific config for transform nodes.
type TransformNodeConfig struct {
	Expressions map[string]string `json:"expressions"`
}

// ForEachNodeConfig is the type-specific config for foreach nodes.
type ForEachNodeConfig struct {
	Graph       json.RawMessage `json:"graph"`                   // inline subgraph
	Concurrency int             `json:"concurrency,omitempty"`   // 0 = unlimited
	OnItemError string          `json:"on_item_error,omitempty"` // "continue" (default) or "abort"
}

// SubgraphNodeConfig is the type-specific config for subgraph nodes.
type SubgraphNodeConfig struct {
	Graph       json.RawMessage `json:"graph"` // inline subgraph
	PortMapping PortMapping     `json:"port_mapping"`
}

// PortMapping maps outer node ports to inner graph ports.
type PortMapping struct {
	Inputs  map[string]string `json:"inputs"`  // outer port -> inner node.port
	Outputs map[string]string `json:"outputs"` // inner node.port -> outer port
}

// HumanInTheLoopConfig is the type-specific config for HITL nodes.
type HumanInTheLoopConfig struct {
	PromptText     string   `json:"prompt_text"`
	TimeoutSeconds *int     `json:"timeout_seconds,omitempty"`
	AllowedActions []string `json:"allowed_actions,omitempty"`
}

// SuperagentNodeConfig is the type-specific config for superagent nodes.
type SuperagentNodeConfig struct {
	// Required
	Prompt    string            `json:"prompt"`
	Skills    []SuperagentSkill `json:"skills"`
	Provider  ProviderType      `json:"provider"`
	Model     string            `json:"model"`
	APIKey    string            `json:"api_key,omitempty"`
	APIKeyRef string            `json:"api_key_ref,omitempty"`
	BaseURL   string            `json:"base_url,omitempty"`

	// Optional
	SystemPreamble               string               `json:"system_preamble,omitempty"`
	MaxIterations                *int                 `json:"max_iterations,omitempty"`
	MaxTotalToolCalls            *int                 `json:"max_total_tool_calls,omitempty"`
	MaxToolCallsPerIteration     *int                 `json:"max_tool_calls_per_iteration,omitempty"`
	MaxToolLoopRounds            *int                 `json:"max_tool_loop_rounds,omitempty"`
	TimeoutSeconds               *int                 `json:"timeout_seconds,omitempty"`
	Temperature                  *float64             `json:"temperature,omitempty"`
	MaxTokens                    *int                 `json:"max_tokens,omitempty"`
	SharedMemory                 *SharedMemoryConfig  `json:"shared_memory,omitempty"`
	ConversationHistoryFromInput string               `json:"conversation_history_from_input,omitempty"`
	ToolPolicies                 *ToolPolicies        `json:"tool_policies,omitempty"`
	Overrides                    *SuperagentOverrides `json:"overrides,omitempty"`
	CodeExecution                *CodeExecutionConfig `json:"code_execution,omitempty"`
}

// CodeExecutionConfig configures optional Python code execution for superagent nodes.
type CodeExecutionConfig struct {
	Enabled                  bool     `json:"enabled"`
	MaxExecutionTimeSec      *int     `json:"max_execution_time_sec,omitempty"`       // default: 30
	MaxMemoryMB              *int     `json:"max_memory_mb,omitempty"`                // default: 256
	MaxOutputBytes           *int     `json:"max_output_bytes,omitempty"`             // default: 1048576 (1MB)
	MaxCodeBytes             *int     `json:"max_code_bytes,omitempty"`               // default: 65536 (64KB)
	MaxToolCallsPerExecution *int     `json:"max_tool_calls_per_execution,omitempty"` // default: 50
	MaxExecutionsPerRun      *int     `json:"max_executions_per_run,omitempty"`       // default: 20
	AllowedModules           []string `json:"allowed_modules,omitempty"`              // optional stdlib override
}

// SuperagentSkill defines a tool source (MCP server or API tool definition)
// providing tools to the superagent.
// Exactly one of MCPURL or APIToolID must be set.
type SuperagentSkill struct {
	Name        string `json:"name"`
	Description string `json:"description"`

	// MCP routing (existing)
	MCPURL       string `json:"mcp_url,omitempty"`
	MCPTransport string `json:"mcp_transport,omitempty"`

	// API tool routing (new).
	// When APIToolID is set, Endpoints is required and non-empty.
	APIToolID string   `json:"api_tool_id,omitempty"`
	Endpoints []string `json:"endpoints,omitempty"` // which endpoints to expose as tools

	// Shared
	Headers        []HeaderConfig `json:"headers,omitempty"`
	PromptFragment string         `json:"prompt_fragment,omitempty"`
	Tools          []string       `json:"tools,omitempty"` // MCP tool allowlist (existing)
	TimeoutSeconds *int           `json:"timeout_seconds,omitempty"`
	Compacted      bool           `json:"compacted,omitempty"` // opt-in compacted discovery mode
}

// SharedMemoryConfig configures cross-node shared memory for superagent nodes.
type SharedMemoryConfig struct {
	Enabled       bool   `json:"enabled"`
	Namespace     string `json:"namespace,omitempty"`
	InjectOnStart *bool  `json:"inject_on_start,omitempty"`
	AutoFlush     *bool  `json:"auto_flush,omitempty"`
}

// ToolPolicies configures tool access policies for superagent nodes.
type ToolPolicies struct {
	Allowed         []string `json:"allowed,omitempty"`
	Denied          []string `json:"denied,omitempty"`
	RequireApproval []string `json:"require_approval,omitempty"`
}

// SuperagentOverrides allows overriding internal superagent components.
type SuperagentOverrides struct {
	Evaluator         *EvaluatorOverride         `json:"evaluator,omitempty"`
	Reflection        *ReflectionOverride        `json:"reflection,omitempty"`
	ContextCompaction *ContextCompactionOverride `json:"context_compaction,omitempty"`
	StuckDetection    *StuckDetectionOverride    `json:"stuck_detection,omitempty"`
	PromptAssembly    *PromptAssemblyOverride    `json:"prompt_assembly,omitempty"`
	OutputExtraction  *OutputExtractionOverride  `json:"output_extraction,omitempty"`
	TaskTracking      *TaskTrackingOverride      `json:"task_tracking,omitempty"`
}

// EvaluatorOverride configures the evaluation LLM.
type EvaluatorOverride struct {
	Provider  ProviderType `json:"provider,omitempty"`
	Model     string       `json:"model,omitempty"`
	APIKey    string       `json:"api_key,omitempty"`
	APIKeyRef string       `json:"api_key_ref,omitempty"`
	Prompt    string       `json:"prompt,omitempty"`
	Disabled  bool         `json:"disabled,omitempty"`
}

// ReflectionOverride configures the reflection LLM.
type ReflectionOverride struct {
	Provider       ProviderType `json:"provider,omitempty"`
	Model          string       `json:"model,omitempty"`
	APIKey         string       `json:"api_key,omitempty"`
	APIKeyRef      string       `json:"api_key_ref,omitempty"`
	Prompt         string       `json:"prompt,omitempty"`
	MaxReflections *int         `json:"max_reflections,omitempty"`
	Disabled       bool         `json:"disabled,omitempty"`
}

// ContextCompactionOverride configures context compaction behavior.
type ContextCompactionOverride struct {
	Enabled                *bool        `json:"enabled,omitempty"`
	Provider               ProviderType `json:"provider,omitempty"`
	Model                  string       `json:"model,omitempty"`
	APIKey                 string       `json:"api_key,omitempty"`
	APIKeyRef              string       `json:"api_key_ref,omitempty"`
	Prompt                 string       `json:"prompt,omitempty"`
	ContextWindowLimit     *int         `json:"context_window_limit,omitempty"`
	CompactionThreshold    *float64     `json:"compaction_threshold,omitempty"`
	PreserveRecentMessages *int         `json:"preserve_recent_messages,omitempty"`
}

// StuckDetectionOverride configures stuck detection parameters.
type StuckDetectionOverride struct {
	Enabled         *bool `json:"enabled,omitempty"`
	WindowSize      *int  `json:"window_size,omitempty"`
	RepeatThreshold *int  `json:"repeat_threshold,omitempty"`
}

// PromptAssemblyOverride configures custom prompt assembly.
type PromptAssemblyOverride struct {
	Template        string `json:"template,omitempty"`
	ToolConventions string `json:"tool_conventions,omitempty"`
	Style           string `json:"style,omitempty"`
}

// OutputExtractionOverride configures the output extraction LLM.
type OutputExtractionOverride struct {
	Prompt    string       `json:"prompt,omitempty"`
	Provider  ProviderType `json:"provider,omitempty"`
	Model     string       `json:"model,omitempty"`
	APIKey    string       `json:"api_key,omitempty"`
	APIKeyRef string       `json:"api_key_ref,omitempty"`
}

// TaskTrackingOverride configures task tracking behavior.
type TaskTrackingOverride struct {
	Enabled           *bool `json:"enabled,omitempty"`
	ReminderFrequency *int  `json:"reminder_frequency,omitempty"`
}

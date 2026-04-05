package model

import (
	"context"
	"encoding/json"
)

// Store is the persistence interface for all domain objects.
// Implemented by PostgreSQL store (production) and MockStore (tests).
type Store interface {
	// Graphs
	CreateGraph(ctx context.Context, graph *Graph) error
	GetGraph(ctx context.Context, tenantID, id string) (*Graph, error)
	ListGraphs(ctx context.Context, tenantID, namespace string, cursor string, limit int) ([]*Graph, string, error)
	UpdateGraph(ctx context.Context, graph *Graph) error
	DeleteGraph(ctx context.Context, tenantID, id string) error

	// Library: Schemas
	CreateSchema(ctx context.Context, schema *SchemaLibrary) error
	GetSchema(ctx context.Context, tenantID, id string) (*SchemaLibrary, error)
	ListSchemas(ctx context.Context, tenantID, namespace string, cursor string, limit int) ([]*SchemaLibrary, string, error)
	UpdateSchema(ctx context.Context, schema *SchemaLibrary) error
	DeleteSchema(ctx context.Context, tenantID, id string) error

	// Library: Prompt Templates
	CreatePromptTemplate(ctx context.Context, pt *PromptLibrary) error
	GetPromptTemplate(ctx context.Context, tenantID, id string) (*PromptLibrary, error)
	ListPromptTemplates(ctx context.Context, tenantID, namespace string, cursor string, limit int) ([]*PromptLibrary, string, error)
	UpdatePromptTemplate(ctx context.Context, pt *PromptLibrary) error
	DeletePromptTemplate(ctx context.Context, tenantID, id string) error

	// Library: Provider Configs
	CreateProviderConfig(ctx context.Context, pc *ProviderConfigLibrary) error
	GetProviderConfig(ctx context.Context, tenantID, id string) (*ProviderConfigLibrary, error)
	ListProviderConfigs(ctx context.Context, tenantID, namespace string, cursor string, limit int) ([]*ProviderConfigLibrary, string, error)
	UpdateProviderConfig(ctx context.Context, pc *ProviderConfigLibrary) error
	DeleteProviderConfig(ctx context.Context, tenantID, id string) error

	// Library: API Tool Definitions
	CreateAPITool(ctx context.Context, at *APIToolDefinition) error
	GetAPITool(ctx context.Context, tenantID, id string) (*APIToolDefinition, error)
	ListAPITools(ctx context.Context, tenantID, namespace string, cursor string, limit int) ([]*APIToolDefinition, string, error)
	UpdateAPITool(ctx context.Context, at *APIToolDefinition) error
	DeleteAPITool(ctx context.Context, tenantID, id string) error

	// Executions
	CreateExecution(ctx context.Context, exec *Execution) error
	GetExecution(ctx context.Context, tenantID, id string) (*Execution, error)
	ListExecutions(ctx context.Context, tenantID string, graphID string, status string, cursor string, limit int) ([]*Execution, string, error)
	UpdateExecution(ctx context.Context, exec *Execution) error

	// Execution Steps
	InsertExecutionStep(ctx context.Context, step *ExecutionStep) error
	ListExecutionSteps(ctx context.Context, executionID string) ([]*ExecutionStep, error)
}

// LLMProvider abstracts LLM API calls behind a common interface.
type LLMProvider interface {
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
	Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error)
	Name() string
}

// Message represents a single message in a conversation for LLM providers.
type Message struct {
	Role            string     `json:"role"`                        // "system", "user", "assistant", "tool"
	Content         string     `json:"content,omitempty"`           // text content
	ToolCalls       []ToolCall `json:"tool_calls,omitempty"`        // assistant requesting tool use
	ToolCallID      string     `json:"tool_call_id,omitempty"`      // for role="tool" — which call this answers
	ToolResultError bool       `json:"tool_result_error,omitempty"` // for role="tool" — was the result an error
}

// LLMToolDefinition describes a tool available to an LLM via the provider's
// native function-calling API. Distinct from ToolDefinition (MCP-facing)
// because providers expect a JSON Schema blob, not an arbitrary input_schema.
type LLMToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema for tool input
}

// ToolCall represents an LLM's request to invoke a tool.
type ToolCall struct {
	ID        string          `json:"id"`        // provider-assigned call ID
	Name      string          `json:"name"`      // tool name
	Arguments json.RawMessage `json:"arguments"` // tool input as JSON
}

// CompletionRequest is the input to an LLM provider call.
type CompletionRequest struct {
	APIKey         string              `json:"-"` // runtime only, never serialized
	Model          string              `json:"model"`
	BaseURL        string              `json:"base_url,omitempty"` // per-request base URL override (used for custom endpoints / mock LLM in E2E)
	Messages       []Message           `json:"messages,omitempty"` // if set, takes priority over SystemPrompt/UserPrompt
	SystemPrompt   string              `json:"system_prompt,omitempty"`
	UserPrompt     string              `json:"user_prompt"`
	Temperature    *float64            `json:"temperature,omitempty"`
	MaxTokens      *int                `json:"max_tokens,omitempty"`
	ResponseFormat ResponseFormat      `json:"response_format"`
	OutputSchema   any                 `json:"output_schema,omitempty"`
	ExtraHeaders   map[string]string   `json:"extra_headers,omitempty"`
	Tools          []LLMToolDefinition `json:"tools,omitempty"`
	ToolChoice     string              `json:"tool_choice,omitempty"` // "auto", "none", "required"
}

// CompletionResponse is the output from an LLM provider call.
type CompletionResponse struct {
	Content      string          `json:"content"`
	Model        string          `json:"model"`
	Usage        LLMUsage        `json:"usage"`
	FinishReason string          `json:"finish_reason"` // "stop", "tool_calls", "length"
	ToolCalls    []ToolCall      `json:"tool_calls,omitempty"`
	RawRequest   json.RawMessage `json:"-"`
	RawResponse  json.RawMessage `json:"-"`
}

// StreamChunk is an incremental piece of a streaming LLM response.
type StreamChunk struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"` // accumulated tool call fragments
	Done      bool       `json:"done"`
	Usage     *LLMUsage  `json:"usage,omitempty"` // only on final chunk
}

// MCPClient abstracts MCP tool server interactions.
type MCPClient interface {
	ListTools(ctx context.Context) ([]ToolDefinition, error)
	CallTool(ctx context.Context, name string, arguments map[string]any) (*ToolResult, error)
}

// ToolDefinition describes an MCP tool.
type ToolDefinition struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	InputSchema  any    `json:"input_schema"`
	OutputSchema any    `json:"output_schema,omitempty"`
}

// ToolResult is the output from an MCP tool call.
type ToolResult struct {
	Content any    `json:"content"`
	IsError bool   `json:"is_error"`
	Error   string `json:"error,omitempty"`
}

// SecretStore resolves api_key_ref names to actual secret values.
type SecretStore interface {
	GetSecret(ctx context.Context, ref string) (string, error)
}

// TaskQueue enqueues execution tasks for workers.
type TaskQueue interface {
	Enqueue(ctx context.Context, task *ExecutionTask) error
}

// EventEmitter publishes execution events during graph execution.
// Implementations handle dual-write to Redis pub/sub and PostgreSQL.
type EventEmitter interface {
	Emit(event ExecutionEvent)
}

// MetricsCollector records operational metrics.
// NoopMetricsCollector is used when Prometheus is not configured.
type MetricsCollector interface {
	ExecutionStarted(graphID, graphName string)
	ExecutionCompleted(graphID, graphName string, durationMs int64, status string)
	NodeStarted(graphID, nodeID, nodeType string)
	NodeCompleted(graphID, nodeID, nodeType string, durationMs int64, status string)
	ProviderCallCompleted(provider, model string, durationMs int64, promptTokens, completionTokens int, status string)
	MCPCallCompleted(toolName, mcpURL string, durationMs int64, status string)
	HTTPRequestCompleted(method, path string, statusCode int, durationMs int64)
}

// TraceExporter sends LLM execution traces to external observability platforms.
type TraceExporter interface {
	ExportSpan(ctx context.Context, span TraceSpan)
	Flush(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

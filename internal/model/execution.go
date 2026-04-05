package model

import (
	"encoding/json"
	"time"
)

// Execution represents a single invocation of a graph.
type Execution struct {
	ID              string           `json:"id"`
	TenantID        string           `json:"tenant_id"`
	GraphID         string           `json:"graph_id"`
	GraphVersion    int              `json:"graph_version"`
	Status          ExecutionStatus  `json:"status"`
	Input           json.RawMessage  `json:"input"`
	Output          json.RawMessage  `json:"output,omitempty"`
	State           json.RawMessage  `json:"state,omitempty"`
	Error           *ExecutionError  `json:"error,omitempty"`
	IterationCounts map[string]int   `json:"iteration_counts,omitempty"`
	StartedAt       *time.Time       `json:"started_at,omitempty"`
	CompletedAt     *time.Time       `json:"completed_at,omitempty"`
	TimeoutSeconds  *int             `json:"timeout_seconds,omitempty"`
	Trigger         ExecutionTrigger `json:"trigger"`
	CorrelationID   string           `json:"correlation_id,omitempty"`
	Metadata        json.RawMessage  `json:"metadata,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

// ExecutionError captures details about a failed execution.
type ExecutionError struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	NodeID  string          `json:"node_id,omitempty"`
	StepID  string          `json:"step_id,omitempty"`
	Details json.RawMessage `json:"details,omitempty"`
}

// ExecutionStep records the result of executing a single node.
type ExecutionStep struct {
	ID          string          `json:"id"`
	ExecutionID string          `json:"execution_id"`
	NodeID      string          `json:"node_id"`
	NodeType    string          `json:"node_type"`
	Iteration   int             `json:"iteration"`
	Status      StepStatus      `json:"status"`
	Input       json.RawMessage `json:"input,omitempty"`
	Output      json.RawMessage `json:"output,omitempty"`
	StateBefore json.RawMessage `json:"state_before,omitempty"`
	StateAfter  json.RawMessage `json:"state_after,omitempty"`
	Error       json.RawMessage `json:"error,omitempty"`
	Attempt     int             `json:"attempt"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	DurationMs  *int64          `json:"duration_ms,omitempty"`
	LLMUsage    *LLMUsage       `json:"llm_usage,omitempty"`
	LLMDebug    *LLMDebugTrace  `json:"llm_debug,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// LLMUsage tracks token consumption for an LLM call.
type LLMUsage struct {
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	CostEstimateUSD  float64 `json:"cost_estimate_usd,omitempty"`
}

// LLMDebugTrace captures debug-only provider request/response payloads for a step.
type LLMDebugTrace struct {
	Calls []LLMCallTrace `json:"calls"`
}

// LLMCallTrace records one provider round-trip for a node execution.
type LLMCallTrace struct {
	RequestID string          `json:"request_id,omitempty"`
	Provider  string          `json:"provider"`
	Model     string          `json:"model"`
	Request   json.RawMessage `json:"request,omitempty"`
	Response  json.RawMessage `json:"response,omitempty"`
}

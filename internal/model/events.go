package model

import (
	"encoding/json"
	"time"
)

// ExecutionEvent is published to Redis pub/sub during graph execution
// and persisted to PostgreSQL as execution steps.
type ExecutionEvent struct {
	Type        EventType       `json:"type"`
	ExecutionID string          `json:"execution_id"`
	Timestamp   time.Time       `json:"timestamp"`
	NodeID      string          `json:"node_id,omitempty"`
	NodeType    string          `json:"node_type,omitempty"`
	Iteration   int             `json:"iteration,omitempty"`
	Input       json.RawMessage `json:"input,omitempty"`
	Output      json.RawMessage `json:"output,omitempty"`
	DurationMs  int64           `json:"duration_ms,omitempty"`
	LLMUsage    *LLMUsage       `json:"llm_usage,omitempty"`
	LLMDebug    *LLMDebugTrace  `json:"llm_debug,omitempty"`
	Error       *ExecutionError `json:"error,omitempty"`
	State       json.RawMessage `json:"state,omitempty"`
	ItemIndex   *int            `json:"item_index,omitempty"`
	ItemTotal   *int            `json:"item_total,omitempty"`
	Status      string          `json:"status,omitempty"`
	Attempt     int             `json:"attempt,omitempty"`
}

// TraceSpan is an OpenInference-compatible span for LLM observability export.
type TraceSpan struct {
	TraceID      string         `json:"trace_id"`
	SpanID       string         `json:"span_id"`
	ParentSpanID string         `json:"parent_span_id,omitempty"`
	Name         string         `json:"name"`
	Kind         string         `json:"kind"` // "LLM", "TOOL", "CHAIN"
	StartTime    time.Time      `json:"start_time"`
	EndTime      time.Time      `json:"end_time"`
	Status       string         `json:"status"` // "OK", "ERROR"
	Attributes   map[string]any `json:"attributes"`
}

// ExecutionTask is the asynq task payload for graph execution.
// Contains everything the worker needs — no database read required.
type ExecutionTask struct {
	ExecutionID string          `json:"execution_id"`
	GraphID     string          `json:"graph_id"`
	GraphName   string          `json:"graph_name"`
	TenantID    string          `json:"tenant_id"`
	Graph       json.RawMessage `json:"graph"` // full self-contained graph
	Input       json.RawMessage `json:"input"`
	Timeout     int             `json:"timeout,omitempty"`
	Debug       bool            `json:"debug,omitempty"`
}

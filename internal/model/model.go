// Package model defines the core domain types for Brockley.
// All other packages import these types. This package has zero external dependencies.
package model

import (
	"encoding/json"
	"time"
)

// Graph represents a complete, self-contained agent workflow.
// Nodes, edges, and state are stored as structured data (JSONB in PostgreSQL).
type Graph struct {
	ID          string          `json:"id"`
	TenantID    string          `json:"tenant_id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Namespace   string          `json:"namespace"`
	Version     int             `json:"version"`
	Status      GraphStatus     `json:"status"`
	Nodes       []Node          `json:"nodes"`
	Edges       []Edge          `json:"edges"`
	State       *GraphState     `json:"state,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DeletedAt   *time.Time      `json:"deleted_at,omitempty"`
}

// GraphState defines typed fields that persist across graph execution.
type GraphState struct {
	Fields []StateField `json:"fields"`
}

// StateField defines a single state field with a type, reducer, and initial value.
type StateField struct {
	Name    string          `json:"name"`
	Schema  json.RawMessage `json:"schema"`
	Reducer Reducer         `json:"reducer"`
	Initial json.RawMessage `json:"initial,omitempty"`
}

// Node represents a single step in a graph.
type Node struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Type           string          `json:"type"` // built-in or custom from registry
	InputPorts     []Port          `json:"input_ports"`
	OutputPorts    []Port          `json:"output_ports"`
	StateReads     []StateBinding  `json:"state_reads,omitempty"`
	StateWrites    []StateBinding  `json:"state_writes,omitempty"`
	Config         json.RawMessage `json:"config"`
	RetryPolicy    *RetryPolicy    `json:"retry_policy,omitempty"`
	TimeoutSeconds *int            `json:"timeout_seconds,omitempty"`
	Position       *Position       `json:"position,omitempty"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
}

// Port defines a typed input or output on a node.
type Port struct {
	Name     string          `json:"name"`
	Schema   json.RawMessage `json:"schema"`
	Required *bool           `json:"required,omitempty"` // default true for inputs
	Default  json.RawMessage `json:"default,omitempty"`
}

// IsRequired returns whether this port is required (defaults to true for input ports).
func (p Port) IsRequired() bool {
	if p.Required != nil {
		return *p.Required
	}
	return true // default
}

// StateBinding maps a state field to/from a port.
type StateBinding struct {
	StateField string `json:"state_field"`
	Port       string `json:"port"`
}

// Edge connects an output port on one node to an input port on another.
type Edge struct {
	ID            string `json:"id"`
	SourceNodeID  string `json:"source_node_id"`
	SourcePort    string `json:"source_port"`
	TargetNodeID  string `json:"target_node_id"`
	TargetPort    string `json:"target_port"`
	BackEdge      bool   `json:"back_edge,omitempty"`
	Condition     string `json:"condition,omitempty"`
	MaxIterations *int   `json:"max_iterations,omitempty"`
}

// Position stores UI layout coordinates.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// RetryPolicy configures retry behavior for a node.
type RetryPolicy struct {
	MaxRetries          int     `json:"max_retries"`
	BackoffStrategy     string  `json:"backoff_strategy,omitempty"` // "fixed" or "exponential"
	InitialDelaySeconds float64 `json:"initial_delay_seconds,omitempty"`
	MaxDelaySeconds     float64 `json:"max_delay_seconds,omitempty"`
}

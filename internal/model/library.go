package model

import (
	"encoding/json"
	"time"
)

// SchemaLibrary is a reusable JSON Schema definition in the library.
type SchemaLibrary struct {
	ID          string          `json:"id"`
	TenantID    string          `json:"tenant_id"`
	Name        string          `json:"name"`
	Namespace   string          `json:"namespace"`
	Description string          `json:"description,omitempty"`
	JSONSchema  json.RawMessage `json:"json_schema"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DeletedAt   *time.Time      `json:"deleted_at,omitempty"`
}

// PromptLibrary is a reusable prompt template in the library.
type PromptLibrary struct {
	ID             string          `json:"id"`
	TenantID       string          `json:"tenant_id"`
	Name           string          `json:"name"`
	Namespace      string          `json:"namespace"`
	Description    string          `json:"description,omitempty"`
	SystemPrompt   string          `json:"system_prompt,omitempty"`
	UserPrompt     string          `json:"user_prompt"`
	Variables      []TemplateVar   `json:"variables"`
	ResponseFormat ResponseFormat  `json:"response_format,omitempty"`
	OutputSchema   json.RawMessage `json:"output_schema,omitempty"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	DeletedAt      *time.Time      `json:"deleted_at,omitempty"`
}

// APIToolDefinition is a reusable library resource that catalogs REST endpoints
// with shared configuration. It is the storage/management layer — individual
// endpoints are selected per-node via APIToolRef or SuperagentSkill.
type APIToolDefinition struct {
	ID             string          `json:"id"`
	TenantID       string          `json:"tenant_id"`
	Name           string          `json:"name"`
	Namespace      string          `json:"namespace"`
	Description    string          `json:"description,omitempty"`
	BaseURL        string          `json:"base_url"`
	DefaultHeaders []HeaderConfig  `json:"default_headers,omitempty"`
	DefaultTimeout int             `json:"default_timeout_ms,omitempty"` // ms, 0 = 30s default
	Retry          *RetryConfig    `json:"retry,omitempty"`
	Endpoints      []APIEndpoint   `json:"endpoints"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	DeletedAt      *time.Time      `json:"deleted_at,omitempty"`
}

// RetryConfig configures retry behavior for API tool HTTP calls.
type RetryConfig struct {
	MaxRetries    int   `json:"max_retries"`
	BackoffMs     int   `json:"backoff_ms"`
	RetryOnStatus []int `json:"retry_on_status,omitempty"` // HTTP status codes
}

// APIEndpoint defines a single REST endpoint within an API tool definition.
type APIEndpoint struct {
	Name            string           `json:"name"`
	Description     string           `json:"description"`
	Method          string           `json:"method"` // GET, POST, PUT, PATCH, DELETE
	Path            string           `json:"path"`   // supports {{input.x}} templates
	InputSchema     json.RawMessage  `json:"input_schema"`
	OutputSchema    json.RawMessage  `json:"output_schema,omitempty"`
	Headers         []HeaderConfig   `json:"headers,omitempty"` // endpoint-specific (merged with defaults)
	RequestMapping  *RequestMapping  `json:"request_mapping,omitempty"`
	ResponseMapping *ResponseMapping `json:"response_mapping,omitempty"`
	TimeoutMs       *int             `json:"timeout_ms,omitempty"` // overrides default
}

// RequestMapping configures how input schema fields map to the HTTP request.
type RequestMapping struct {
	Mode string `json:"mode"` // "json_body" (default), "form", "query_params", "path_and_body"
}

// ResponseMapping configures how the HTTP response maps to tool output.
type ResponseMapping struct {
	Mode       string `json:"mode"`                 // "json_body" (default), "text", "jq", "headers_and_body"
	Expression string `json:"expression,omitempty"` // for "jq" mode
}

// ProviderConfigLibrary is a reusable LLM provider config in the library.
type ProviderConfigLibrary struct {
	ID           string            `json:"id"`
	TenantID     string            `json:"tenant_id"`
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	Provider     ProviderType      `json:"provider"`
	BaseURL      string            `json:"base_url,omitempty"`
	APIKeyRef    string            `json:"api_key_ref"`
	DefaultModel string            `json:"default_model,omitempty"`
	ExtraHeaders map[string]string `json:"extra_headers,omitempty"`
	Metadata     json.RawMessage   `json:"metadata,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	DeletedAt    *time.Time        `json:"deleted_at,omitempty"`
}

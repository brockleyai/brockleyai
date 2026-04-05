package api

// ListResponse is the standard paginated list response envelope.
type ListResponse[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

// CreateGraphRequest is the body for POST /api/v1/graphs.
type CreateGraphRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	GraphJSON   any    `json:"graph_json"` // full self-contained graph body (nodes, edges, state)
}

// InvokeRequest is the body for POST /api/v1/executions.
type InvokeRequest struct {
	GraphID       string `json:"graph_id"`
	Input         any    `json:"input"`
	Mode          string `json:"mode,omitempty"` // "sync" or "async" (default: "async")
	Timeout       int    `json:"timeout_seconds,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	Debug         bool   `json:"debug,omitempty"`
}

// HealthResponse is returned by GET /health.
type HealthResponse struct {
	Status     string            `json:"status"`
	Version    string            `json:"version"`
	Components map[string]string `json:"components"`
}

// VersionResponse is returned by GET /version.
type VersionResponse struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	APIVersion string `json:"api_version"`
}

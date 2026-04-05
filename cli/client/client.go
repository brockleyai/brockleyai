// Package client provides a Go HTTP client for the Brockley REST API.
// Used by the CLI, Terraform provider, and CI/CD integrations.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client is an HTTP client for the Brockley API server.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New creates a new Brockley API client.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ListResponse is the standard paginated list response envelope.
type ListResponse[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

// APIError is the standard error response from the server.
type APIError struct {
	StatusCode int
	Code       string `json:"code"`
	Message    string `json:"message"`
	RequestID  string `json:"request_id,omitempty"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("brockley API error (HTTP %d, %s): %s", e.StatusCode, e.Code, e.Message)
}

func (c *Client) do(ctx context.Context, method, path string, body any, result any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return parseAPIError(resp)
	}

	if result != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}
	return nil
}

func parseAPIError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	var errResp struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
		return &APIError{
			StatusCode: resp.StatusCode,
			Code:       errResp.Error.Code,
			Message:    errResp.Error.Message,
			RequestID:  errResp.Error.RequestID,
		}
	}
	return &APIError{
		StatusCode: resp.StatusCode,
		Code:       "UNKNOWN",
		Message:    string(body),
	}
}

// Graph operations

// CreateGraph creates a new graph.
func (c *Client) CreateGraph(ctx context.Context, graph any) (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.do(ctx, http.MethodPost, "/api/v1/graphs", graph, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetGraph fetches a graph by ID.
func (c *Client) GetGraph(ctx context.Context, id string) (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.do(ctx, http.MethodGet, "/api/v1/graphs/"+id, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListGraphs lists graphs with optional filters.
func (c *Client) ListGraphs(ctx context.Context, namespace, status, cursor string, limit int) (*ListResponse[json.RawMessage], error) {
	params := url.Values{}
	if namespace != "" {
		params.Set("namespace", namespace)
	}
	if status != "" {
		params.Set("status", status)
	}
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	path := "/api/v1/graphs"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var result ListResponse[json.RawMessage]
	if err := c.do(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateGraph updates a graph by ID.
func (c *Client) UpdateGraph(ctx context.Context, id string, graph any) (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.do(ctx, http.MethodPut, "/api/v1/graphs/"+id, graph, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// DeleteGraph deletes a graph by ID.
func (c *Client) DeleteGraph(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/graphs/"+id, nil, nil)
}

// ValidateGraph validates a graph by ID.
func (c *Client) ValidateGraph(ctx context.Context, id string) (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.do(ctx, http.MethodPost, "/api/v1/graphs/"+id+"/validate", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Execution operations

// InvokeExecution invokes a graph execution.
func (c *Client) InvokeExecution(ctx context.Context, req *InvokeRequest) (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.do(ctx, http.MethodPost, "/api/v1/executions", req, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// InvokeRequest is the request body for invoking a graph execution.
type InvokeRequest struct {
	GraphID       string `json:"graph_id"`
	Input         any    `json:"input"`
	Mode          string `json:"mode,omitempty"`
	Timeout       int    `json:"timeout_seconds,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	Debug         bool   `json:"debug,omitempty"`
}

// GetExecution fetches an execution by ID.
func (c *Client) GetExecution(ctx context.Context, id string) (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.do(ctx, http.MethodGet, "/api/v1/executions/"+id, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListExecutions lists executions with optional filters.
func (c *Client) ListExecutions(ctx context.Context, graphID, status, cursor string, limit int) (*ListResponse[json.RawMessage], error) {
	params := url.Values{}
	if graphID != "" {
		params.Set("graph_id", graphID)
	}
	if status != "" {
		params.Set("status", status)
	}
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	path := "/api/v1/executions"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var result ListResponse[json.RawMessage]
	if err := c.do(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetExecutionSteps fetches steps for an execution.
func (c *Client) GetExecutionSteps(ctx context.Context, executionID string) (*ListResponse[json.RawMessage], error) {
	var result ListResponse[json.RawMessage]
	if err := c.do(ctx, http.MethodGet, "/api/v1/executions/"+executionID+"/steps", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CancelExecution cancels a running execution.
func (c *Client) CancelExecution(ctx context.Context, id string) (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.do(ctx, http.MethodPost, "/api/v1/executions/"+id+"/cancel", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Library operations

// CreateSchema creates a new schema library entry.
func (c *Client) CreateSchema(ctx context.Context, schema any) (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.do(ctx, http.MethodPost, "/api/v1/schemas", schema, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetSchema fetches a schema by ID.
func (c *Client) GetSchema(ctx context.Context, id string) (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.do(ctx, http.MethodGet, "/api/v1/schemas/"+id, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListSchemas lists schemas.
func (c *Client) ListSchemas(ctx context.Context, namespace, cursor string, limit int) (*ListResponse[json.RawMessage], error) {
	params := url.Values{}
	if namespace != "" {
		params.Set("namespace", namespace)
	}
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	path := "/api/v1/schemas"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var result ListResponse[json.RawMessage]
	if err := c.do(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateSchema updates a schema by ID.
func (c *Client) UpdateSchema(ctx context.Context, id string, schema any) (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.do(ctx, http.MethodPut, "/api/v1/schemas/"+id, schema, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// DeleteSchema deletes a schema by ID.
func (c *Client) DeleteSchema(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/schemas/"+id, nil, nil)
}

// CreatePromptTemplate creates a new prompt template.
func (c *Client) CreatePromptTemplate(ctx context.Context, pt any) (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.do(ctx, http.MethodPost, "/api/v1/prompt-templates", pt, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetPromptTemplate fetches a prompt template by ID.
func (c *Client) GetPromptTemplate(ctx context.Context, id string) (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.do(ctx, http.MethodGet, "/api/v1/prompt-templates/"+id, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListPromptTemplates lists prompt templates.
func (c *Client) ListPromptTemplates(ctx context.Context, namespace, cursor string, limit int) (*ListResponse[json.RawMessage], error) {
	params := url.Values{}
	if namespace != "" {
		params.Set("namespace", namespace)
	}
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	path := "/api/v1/prompt-templates"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var result ListResponse[json.RawMessage]
	if err := c.do(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdatePromptTemplate updates a prompt template by ID.
func (c *Client) UpdatePromptTemplate(ctx context.Context, id string, pt any) (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.do(ctx, http.MethodPut, "/api/v1/prompt-templates/"+id, pt, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// DeletePromptTemplate deletes a prompt template by ID.
func (c *Client) DeletePromptTemplate(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/prompt-templates/"+id, nil, nil)
}

// CreateProviderConfig creates a new provider config.
func (c *Client) CreateProviderConfig(ctx context.Context, pc any) (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.do(ctx, http.MethodPost, "/api/v1/provider-configs", pc, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetProviderConfig fetches a provider config by ID.
func (c *Client) GetProviderConfig(ctx context.Context, id string) (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.do(ctx, http.MethodGet, "/api/v1/provider-configs/"+id, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListProviderConfigs lists provider configs.
func (c *Client) ListProviderConfigs(ctx context.Context, namespace, cursor string, limit int) (*ListResponse[json.RawMessage], error) {
	params := url.Values{}
	if namespace != "" {
		params.Set("namespace", namespace)
	}
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	path := "/api/v1/provider-configs"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var result ListResponse[json.RawMessage]
	if err := c.do(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateProviderConfig updates a provider config by ID.
func (c *Client) UpdateProviderConfig(ctx context.Context, id string, pc any) (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.do(ctx, http.MethodPut, "/api/v1/provider-configs/"+id, pc, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// DeleteProviderConfig deletes a provider config by ID.
func (c *Client) DeleteProviderConfig(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/provider-configs/"+id, nil, nil)
}

// API Tool operations

// CreateAPITool creates a new API tool definition.
func (c *Client) CreateAPITool(ctx context.Context, body any) (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.do(ctx, http.MethodPost, "/api/v1/api-tools", body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetAPITool fetches an API tool definition by ID.
func (c *Client) GetAPITool(ctx context.Context, id string) (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.do(ctx, http.MethodGet, "/api/v1/api-tools/"+id, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListAPITools lists API tool definitions with optional filters.
func (c *Client) ListAPITools(ctx context.Context, namespace, cursor string, limit int) (*ListResponse[json.RawMessage], error) {
	params := url.Values{}
	if namespace != "" {
		params.Set("namespace", namespace)
	}
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	path := "/api/v1/api-tools"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var result ListResponse[json.RawMessage]
	if err := c.do(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateAPITool updates an API tool definition by ID.
func (c *Client) UpdateAPITool(ctx context.Context, id string, body any) (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.do(ctx, http.MethodPut, "/api/v1/api-tools/"+id, body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// DeleteAPITool deletes an API tool definition by ID.
func (c *Client) DeleteAPITool(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/api-tools/"+id, nil, nil)
}

// Health checks

// Health performs a health check against the server.
func (c *Client) Health(ctx context.Context) (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.do(ctx, http.MethodGet, "/health", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

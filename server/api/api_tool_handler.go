package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/brockleyai/brockleyai/engine/executor"
	"github.com/brockleyai/brockleyai/internal/model"
	"github.com/brockleyai/brockleyai/server/middleware"
)

// --- API Tool Definition Handler ---

// APIToolHandler handles /api/v1/api-tools endpoints.
type APIToolHandler struct {
	store  model.Store
	logger *slog.Logger
}

// NewAPIToolHandler creates a new APIToolHandler.
func NewAPIToolHandler(store model.Store, logger *slog.Logger) *APIToolHandler {
	return &APIToolHandler{store: store, logger: logger}
}

// validHTTPMethods is the set of allowed HTTP methods for API tool endpoints.
var validHTTPMethods = map[string]bool{
	"GET":    true,
	"POST":   true,
	"PUT":    true,
	"PATCH":  true,
	"DELETE": true,
}

// Create handles POST /api/v1/api-tools.
func (h *APIToolHandler) Create(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())

	var body struct {
		ID             string               `json:"id"`
		Name           string               `json:"name"`
		Description    string               `json:"description"`
		Namespace      string               `json:"namespace"`
		BaseURL        string               `json:"base_url"`
		DefaultHeaders []model.HeaderConfig `json:"default_headers"`
		DefaultTimeout int                  `json:"default_timeout_ms"`
		Retry          *model.RetryConfig   `json:"retry"`
		Endpoints      []model.APIEndpoint  `json:"endpoints"`
		Metadata       json.RawMessage      `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "invalid request body: "+err.Error(), requestID)
		return
	}

	// Validate required fields.
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "name is required", requestID)
		return
	}
	if body.BaseURL == "" {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "base_url is required", requestID)
		return
	}
	if !strings.HasPrefix(body.BaseURL, "http://") && !strings.HasPrefix(body.BaseURL, "https://") {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "base_url must start with http:// or https://", requestID)
		return
	}
	if len(body.Endpoints) == 0 {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "endpoints must not be empty", requestID)
		return
	}

	// Validate each endpoint.
	endpointNames := make(map[string]bool, len(body.Endpoints))
	for i, ep := range body.Endpoints {
		if ep.Name == "" {
			writeError(w, http.StatusBadRequest, ErrCodeValidation, "endpoint name is required", requestID)
			return
		}
		if ep.Description == "" {
			writeError(w, http.StatusBadRequest, ErrCodeValidation, "endpoint description is required", requestID)
			return
		}
		if ep.Method == "" {
			writeError(w, http.StatusBadRequest, ErrCodeValidation, "endpoint method is required", requestID)
			return
		}
		if ep.Path == "" {
			writeError(w, http.StatusBadRequest, ErrCodeValidation, "endpoint path is required", requestID)
			return
		}

		method := strings.ToUpper(ep.Method)
		if !validHTTPMethods[method] {
			writeError(w, http.StatusBadRequest, ErrCodeValidation, "invalid HTTP method: "+ep.Method, requestID)
			return
		}
		// Normalize method to uppercase.
		body.Endpoints[i].Method = method

		if endpointNames[ep.Name] {
			writeError(w, http.StatusBadRequest, ErrCodeValidation, "duplicate endpoint name: "+ep.Name, requestID)
			return
		}
		endpointNames[ep.Name] = true
	}

	if body.Namespace == "" {
		body.Namespace = "default"
	}

	now := time.Now().UTC()
	id := body.ID
	if id == "" {
		id = generateAPIToolID()
	}
	at := &model.APIToolDefinition{
		ID:             id,
		TenantID:       tenantID,
		Name:           body.Name,
		Description:    body.Description,
		Namespace:      body.Namespace,
		BaseURL:        body.BaseURL,
		DefaultHeaders: body.DefaultHeaders,
		DefaultTimeout: body.DefaultTimeout,
		Retry:          body.Retry,
		Endpoints:      body.Endpoints,
		Metadata:       body.Metadata,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := h.store.CreateAPITool(r.Context(), at); err != nil {
		h.logger.Error("failed to create api tool", "error", err, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to create api tool", requestID)
		return
	}

	h.logger.Info("api tool created", "api_tool_id", at.ID, "name", at.Name, "request_id", requestID)
	writeJSON(w, http.StatusCreated, at)
}

// Get handles GET /api/v1/api-tools/{id}.
func (h *APIToolHandler) Get(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())
	id := r.PathValue("id")

	at, err := h.store.GetAPITool(r.Context(), tenantID, id)
	if err != nil {
		h.logger.Error("failed to get api tool", "error", err, "api_tool_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to get api tool", requestID)
		return
	}
	if at == nil {
		writeError(w, http.StatusNotFound, ErrCodeNotFound, "api tool not found", requestID)
		return
	}

	writeJSON(w, http.StatusOK, at)
}

// List handles GET /api/v1/api-tools.
func (h *APIToolHandler) List(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())

	namespace := r.URL.Query().Get("namespace")
	cursor := r.URL.Query().Get("cursor")
	limit := parseLimit(r)

	tools, nextCursor, err := h.store.ListAPITools(r.Context(), tenantID, namespace, cursor, limit)
	if err != nil {
		h.logger.Error("failed to list api tools", "error", err, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to list api tools", requestID)
		return
	}

	writeJSON(w, http.StatusOK, ListResponse[*model.APIToolDefinition]{
		Items:      tools,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	})
}

// Update handles PUT /api/v1/api-tools/{id}.
func (h *APIToolHandler) Update(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())
	id := r.PathValue("id")

	existing, err := h.store.GetAPITool(r.Context(), tenantID, id)
	if err != nil {
		h.logger.Error("failed to get api tool for update", "error", err, "api_tool_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to get api tool", requestID)
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, ErrCodeNotFound, "api tool not found", requestID)
		return
	}

	var body struct {
		Name           *string               `json:"name"`
		Description    *string               `json:"description"`
		BaseURL        *string               `json:"base_url"`
		DefaultHeaders *[]model.HeaderConfig `json:"default_headers"`
		DefaultTimeout *int                  `json:"default_timeout_ms"`
		Retry          *model.RetryConfig    `json:"retry"`
		Endpoints      *[]model.APIEndpoint  `json:"endpoints"`
		Metadata       json.RawMessage       `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "invalid request body: "+err.Error(), requestID)
		return
	}

	if body.Name != nil {
		existing.Name = *body.Name
	}
	if body.Description != nil {
		existing.Description = *body.Description
	}
	if body.BaseURL != nil {
		baseURL := *body.BaseURL
		if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
			writeError(w, http.StatusBadRequest, ErrCodeValidation, "base_url must start with http:// or https://", requestID)
			return
		}
		existing.BaseURL = baseURL
	}
	if body.DefaultHeaders != nil {
		existing.DefaultHeaders = *body.DefaultHeaders
	}
	if body.DefaultTimeout != nil {
		existing.DefaultTimeout = *body.DefaultTimeout
	}
	if body.Retry != nil {
		existing.Retry = body.Retry
	}
	if body.Endpoints != nil {
		endpoints := *body.Endpoints
		if len(endpoints) == 0 {
			writeError(w, http.StatusBadRequest, ErrCodeValidation, "endpoints must not be empty", requestID)
			return
		}

		// Validate each endpoint.
		endpointNames := make(map[string]bool, len(endpoints))
		for i, ep := range endpoints {
			if ep.Name == "" {
				writeError(w, http.StatusBadRequest, ErrCodeValidation, "endpoint name is required", requestID)
				return
			}
			if ep.Description == "" {
				writeError(w, http.StatusBadRequest, ErrCodeValidation, "endpoint description is required", requestID)
				return
			}
			if ep.Method == "" {
				writeError(w, http.StatusBadRequest, ErrCodeValidation, "endpoint method is required", requestID)
				return
			}
			if ep.Path == "" {
				writeError(w, http.StatusBadRequest, ErrCodeValidation, "endpoint path is required", requestID)
				return
			}

			method := strings.ToUpper(ep.Method)
			if !validHTTPMethods[method] {
				writeError(w, http.StatusBadRequest, ErrCodeValidation, "invalid HTTP method: "+ep.Method, requestID)
				return
			}
			endpoints[i].Method = method

			if endpointNames[ep.Name] {
				writeError(w, http.StatusBadRequest, ErrCodeValidation, "duplicate endpoint name: "+ep.Name, requestID)
				return
			}
			endpointNames[ep.Name] = true
		}
		existing.Endpoints = endpoints
	}
	if body.Metadata != nil {
		existing.Metadata = body.Metadata
	}

	existing.UpdatedAt = time.Now().UTC()

	if err := h.store.UpdateAPITool(r.Context(), existing); err != nil {
		h.logger.Error("failed to update api tool", "error", err, "api_tool_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to update api tool", requestID)
		return
	}

	h.logger.Info("api tool updated", "api_tool_id", id, "request_id", requestID)
	writeJSON(w, http.StatusOK, existing)
}

// Delete handles DELETE /api/v1/api-tools/{id}.
func (h *APIToolHandler) Delete(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())
	id := r.PathValue("id")

	if err := h.store.DeleteAPITool(r.Context(), tenantID, id); err != nil {
		h.logger.Error("failed to delete api tool", "error", err, "api_tool_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to delete api tool", requestID)
		return
	}

	h.logger.Info("api tool deleted", "api_tool_id", id, "request_id", requestID)
	w.WriteHeader(http.StatusNoContent)
}

// Test handles POST /api/v1/api-tools/{id}/test.
// It executes a single endpoint call with sample input and returns raw HTTP response details + mapped result.
func (h *APIToolHandler) Test(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())
	id := r.PathValue("id")

	var body struct {
		Endpoint        string         `json:"endpoint"`
		Input           map[string]any `json:"input"`
		BaseURLOverride string         `json:"base_url_override,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "invalid request body: "+err.Error(), requestID)
		return
	}

	if body.Endpoint == "" {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "endpoint is required", requestID)
		return
	}

	// Load the API tool definition.
	at, err := h.store.GetAPITool(r.Context(), tenantID, id)
	if err != nil {
		h.logger.Error("failed to get api tool for test", "error", err, "api_tool_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to get api tool", requestID)
		return
	}
	if at == nil {
		writeError(w, http.StatusNotFound, ErrCodeNotFound, "api tool not found", requestID)
		return
	}

	// Find the endpoint.
	ep := executor.FindEndpoint(at, body.Endpoint)
	if ep == nil {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "endpoint not found: "+body.Endpoint, requestID)
		return
	}

	// Apply base URL override if provided.
	if body.BaseURLOverride != "" {
		if !strings.HasPrefix(body.BaseURLOverride, "http://") && !strings.HasPrefix(body.BaseURLOverride, "https://") {
			writeError(w, http.StatusBadRequest, ErrCodeValidation, "base_url_override must start with http:// or https://", requestID)
			return
		}
		at.BaseURL = body.BaseURLOverride
	}

	// Create a dispatcher and execute the call.
	dispatcher := executor.NewAPIToolDispatcher(h.store, h.logger)
	route := model.ToolRoute{
		APIToolID:   id,
		APIEndpoint: body.Endpoint,
	}
	input := body.Input
	if input == nil {
		input = map[string]any{}
	}

	start := time.Now()
	result, err := dispatcher.CallEndpoint(r.Context(), tenantID, route, body.Endpoint, input)
	durationMs := time.Since(start).Milliseconds()

	if err != nil {
		h.logger.Error("api tool test failed", "error", err, "api_tool_id", id, "endpoint", body.Endpoint, "request_id", requestID)
		writeJSON(w, http.StatusOK, map[string]any{
			"success":     false,
			"error":       err.Error(),
			"duration_ms": durationMs,
		})
		return
	}

	h.logger.Info("api tool test completed", "api_tool_id", id, "endpoint", body.Endpoint, "is_error", result.IsError, "duration_ms", durationMs, "request_id", requestID)
	writeJSON(w, http.StatusOK, map[string]any{
		"success":     !result.IsError,
		"result":      result.Content,
		"error":       result.Error,
		"is_error":    result.IsError,
		"duration_ms": durationMs,
	})
}

func generateAPIToolID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return "atool_" + hex.EncodeToString(b)
}

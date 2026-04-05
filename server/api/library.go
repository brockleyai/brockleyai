package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/brockleyai/brockleyai/internal/model"
	"github.com/brockleyai/brockleyai/server/middleware"
)

// --- Schema Handler ---

// SchemaHandler handles /api/v1/schemas endpoints.
type SchemaHandler struct {
	store  model.Store
	logger *slog.Logger
}

// NewSchemaHandler creates a new SchemaHandler.
func NewSchemaHandler(store model.Store, logger *slog.Logger) *SchemaHandler {
	return &SchemaHandler{store: store, logger: logger}
}

// Create handles POST /api/v1/schemas.
func (h *SchemaHandler) Create(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())

	var body struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Namespace   string          `json:"namespace"`
		JSONSchema  json.RawMessage `json:"json_schema"`
		Metadata    json.RawMessage `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "invalid request body: "+err.Error(), requestID)
		return
	}

	if body.Name == "" {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "name is required", requestID)
		return
	}
	if body.JSONSchema == nil {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "json_schema is required", requestID)
		return
	}
	if body.Namespace == "" {
		body.Namespace = "default"
	}

	now := time.Now().UTC()
	schema := &model.SchemaLibrary{
		ID:          generateSchemaID(),
		TenantID:    tenantID,
		Name:        body.Name,
		Description: body.Description,
		Namespace:   body.Namespace,
		JSONSchema:  body.JSONSchema,
		Metadata:    body.Metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.store.CreateSchema(r.Context(), schema); err != nil {
		h.logger.Error("failed to create schema", "error", err, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to create schema", requestID)
		return
	}

	h.logger.Info("schema created", "schema_id", schema.ID, "name", schema.Name, "request_id", requestID)
	writeJSON(w, http.StatusCreated, schema)
}

// Get handles GET /api/v1/schemas/{id}.
func (h *SchemaHandler) Get(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())
	id := r.PathValue("id")

	schema, err := h.store.GetSchema(r.Context(), tenantID, id)
	if err != nil {
		h.logger.Error("failed to get schema", "error", err, "schema_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to get schema", requestID)
		return
	}
	if schema == nil {
		writeError(w, http.StatusNotFound, ErrCodeNotFound, "schema not found", requestID)
		return
	}

	writeJSON(w, http.StatusOK, schema)
}

// List handles GET /api/v1/schemas.
func (h *SchemaHandler) List(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())

	namespace := r.URL.Query().Get("namespace")
	cursor := r.URL.Query().Get("cursor")
	limit := parseLimit(r)

	schemas, nextCursor, err := h.store.ListSchemas(r.Context(), tenantID, namespace, cursor, limit)
	if err != nil {
		h.logger.Error("failed to list schemas", "error", err, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to list schemas", requestID)
		return
	}

	writeJSON(w, http.StatusOK, ListResponse[*model.SchemaLibrary]{
		Items:      schemas,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	})
}

// Update handles PUT /api/v1/schemas/{id}.
func (h *SchemaHandler) Update(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())
	id := r.PathValue("id")

	existing, err := h.store.GetSchema(r.Context(), tenantID, id)
	if err != nil {
		h.logger.Error("failed to get schema for update", "error", err, "schema_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to get schema", requestID)
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, ErrCodeNotFound, "schema not found", requestID)
		return
	}

	var body struct {
		Name        *string         `json:"name"`
		Description *string         `json:"description"`
		JSONSchema  json.RawMessage `json:"json_schema"`
		Metadata    json.RawMessage `json:"metadata"`
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
	if body.JSONSchema != nil {
		existing.JSONSchema = body.JSONSchema
	}
	if body.Metadata != nil {
		existing.Metadata = body.Metadata
	}

	existing.UpdatedAt = time.Now().UTC()

	if err := h.store.UpdateSchema(r.Context(), existing); err != nil {
		h.logger.Error("failed to update schema", "error", err, "schema_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to update schema", requestID)
		return
	}

	h.logger.Info("schema updated", "schema_id", id, "request_id", requestID)
	writeJSON(w, http.StatusOK, existing)
}

// Delete handles DELETE /api/v1/schemas/{id}.
func (h *SchemaHandler) Delete(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())
	id := r.PathValue("id")

	if err := h.store.DeleteSchema(r.Context(), tenantID, id); err != nil {
		h.logger.Error("failed to delete schema", "error", err, "schema_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to delete schema", requestID)
		return
	}

	h.logger.Info("schema deleted", "schema_id", id, "request_id", requestID)
	w.WriteHeader(http.StatusNoContent)
}

// --- Prompt Template Handler ---

// PromptTemplateHandler handles /api/v1/prompt-templates endpoints.
type PromptTemplateHandler struct {
	store  model.Store
	logger *slog.Logger
}

// NewPromptTemplateHandler creates a new PromptTemplateHandler.
func NewPromptTemplateHandler(store model.Store, logger *slog.Logger) *PromptTemplateHandler {
	return &PromptTemplateHandler{store: store, logger: logger}
}

// Create handles POST /api/v1/prompt-templates.
func (h *PromptTemplateHandler) Create(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())

	var body struct {
		Name           string               `json:"name"`
		Description    string               `json:"description"`
		Namespace      string               `json:"namespace"`
		SystemPrompt   string               `json:"system_prompt"`
		UserPrompt     string               `json:"user_prompt"`
		Variables      []model.TemplateVar  `json:"variables"`
		ResponseFormat model.ResponseFormat `json:"response_format"`
		OutputSchema   json.RawMessage      `json:"output_schema"`
		Metadata       json.RawMessage      `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "invalid request body: "+err.Error(), requestID)
		return
	}

	if body.Name == "" {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "name is required", requestID)
		return
	}
	if body.UserPrompt == "" {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "user_prompt is required", requestID)
		return
	}
	if body.Namespace == "" {
		body.Namespace = "default"
	}

	now := time.Now().UTC()
	pt := &model.PromptLibrary{
		ID:             generatePromptTemplateID(),
		TenantID:       tenantID,
		Name:           body.Name,
		Description:    body.Description,
		Namespace:      body.Namespace,
		SystemPrompt:   body.SystemPrompt,
		UserPrompt:     body.UserPrompt,
		Variables:      body.Variables,
		ResponseFormat: body.ResponseFormat,
		OutputSchema:   body.OutputSchema,
		Metadata:       body.Metadata,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := h.store.CreatePromptTemplate(r.Context(), pt); err != nil {
		h.logger.Error("failed to create prompt template", "error", err, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to create prompt template", requestID)
		return
	}

	h.logger.Info("prompt template created", "template_id", pt.ID, "name", pt.Name, "request_id", requestID)
	writeJSON(w, http.StatusCreated, pt)
}

// Get handles GET /api/v1/prompt-templates/{id}.
func (h *PromptTemplateHandler) Get(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())
	id := r.PathValue("id")

	pt, err := h.store.GetPromptTemplate(r.Context(), tenantID, id)
	if err != nil {
		h.logger.Error("failed to get prompt template", "error", err, "template_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to get prompt template", requestID)
		return
	}
	if pt == nil {
		writeError(w, http.StatusNotFound, ErrCodeNotFound, "prompt template not found", requestID)
		return
	}

	writeJSON(w, http.StatusOK, pt)
}

// List handles GET /api/v1/prompt-templates.
func (h *PromptTemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())

	namespace := r.URL.Query().Get("namespace")
	cursor := r.URL.Query().Get("cursor")
	limit := parseLimit(r)

	templates, nextCursor, err := h.store.ListPromptTemplates(r.Context(), tenantID, namespace, cursor, limit)
	if err != nil {
		h.logger.Error("failed to list prompt templates", "error", err, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to list prompt templates", requestID)
		return
	}

	writeJSON(w, http.StatusOK, ListResponse[*model.PromptLibrary]{
		Items:      templates,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	})
}

// Update handles PUT /api/v1/prompt-templates/{id}.
func (h *PromptTemplateHandler) Update(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())
	id := r.PathValue("id")

	existing, err := h.store.GetPromptTemplate(r.Context(), tenantID, id)
	if err != nil {
		h.logger.Error("failed to get prompt template for update", "error", err, "template_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to get prompt template", requestID)
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, ErrCodeNotFound, "prompt template not found", requestID)
		return
	}

	var body struct {
		Name           *string               `json:"name"`
		Description    *string               `json:"description"`
		SystemPrompt   *string               `json:"system_prompt"`
		UserPrompt     *string               `json:"user_prompt"`
		Variables      *[]model.TemplateVar  `json:"variables"`
		ResponseFormat *model.ResponseFormat `json:"response_format"`
		OutputSchema   json.RawMessage       `json:"output_schema"`
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
	if body.SystemPrompt != nil {
		existing.SystemPrompt = *body.SystemPrompt
	}
	if body.UserPrompt != nil {
		existing.UserPrompt = *body.UserPrompt
	}
	if body.Variables != nil {
		existing.Variables = *body.Variables
	}
	if body.ResponseFormat != nil {
		existing.ResponseFormat = *body.ResponseFormat
	}
	if body.OutputSchema != nil {
		existing.OutputSchema = body.OutputSchema
	}
	if body.Metadata != nil {
		existing.Metadata = body.Metadata
	}

	existing.UpdatedAt = time.Now().UTC()

	if err := h.store.UpdatePromptTemplate(r.Context(), existing); err != nil {
		h.logger.Error("failed to update prompt template", "error", err, "template_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to update prompt template", requestID)
		return
	}

	h.logger.Info("prompt template updated", "template_id", id, "request_id", requestID)
	writeJSON(w, http.StatusOK, existing)
}

// Delete handles DELETE /api/v1/prompt-templates/{id}.
func (h *PromptTemplateHandler) Delete(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())
	id := r.PathValue("id")

	if err := h.store.DeletePromptTemplate(r.Context(), tenantID, id); err != nil {
		h.logger.Error("failed to delete prompt template", "error", err, "template_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to delete prompt template", requestID)
		return
	}

	h.logger.Info("prompt template deleted", "template_id", id, "request_id", requestID)
	w.WriteHeader(http.StatusNoContent)
}

// --- Provider Config Handler ---

// ProviderConfigHandler handles /api/v1/provider-configs endpoints.
type ProviderConfigHandler struct {
	store  model.Store
	logger *slog.Logger
}

// NewProviderConfigHandler creates a new ProviderConfigHandler.
func NewProviderConfigHandler(store model.Store, logger *slog.Logger) *ProviderConfigHandler {
	return &ProviderConfigHandler{store: store, logger: logger}
}

// Create handles POST /api/v1/provider-configs.
func (h *ProviderConfigHandler) Create(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())

	var body struct {
		Name         string             `json:"name"`
		Namespace    string             `json:"namespace"`
		Provider     model.ProviderType `json:"provider"`
		BaseURL      string             `json:"base_url"`
		APIKeyRef    string             `json:"api_key_ref"`
		DefaultModel string             `json:"default_model"`
		ExtraHeaders map[string]string  `json:"extra_headers"`
		Metadata     json.RawMessage    `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "invalid request body: "+err.Error(), requestID)
		return
	}

	if body.Name == "" {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "name is required", requestID)
		return
	}
	if body.Provider == "" {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "provider is required", requestID)
		return
	}
	if body.Namespace == "" {
		body.Namespace = "default"
	}

	now := time.Now().UTC()
	pc := &model.ProviderConfigLibrary{
		ID:           generateProviderConfigID(),
		TenantID:     tenantID,
		Name:         body.Name,
		Namespace:    body.Namespace,
		Provider:     body.Provider,
		BaseURL:      body.BaseURL,
		APIKeyRef:    body.APIKeyRef,
		DefaultModel: body.DefaultModel,
		ExtraHeaders: body.ExtraHeaders,
		Metadata:     body.Metadata,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := h.store.CreateProviderConfig(r.Context(), pc); err != nil {
		h.logger.Error("failed to create provider config", "error", err, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to create provider config", requestID)
		return
	}

	h.logger.Info("provider config created", "provider_config_id", pc.ID, "name", pc.Name, "request_id", requestID)

	// Redact api_key_ref before returning.
	resp := redactProviderConfig(pc)
	writeJSON(w, http.StatusCreated, resp)
}

// Get handles GET /api/v1/provider-configs/{id}.
func (h *ProviderConfigHandler) Get(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())
	id := r.PathValue("id")

	pc, err := h.store.GetProviderConfig(r.Context(), tenantID, id)
	if err != nil {
		h.logger.Error("failed to get provider config", "error", err, "provider_config_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to get provider config", requestID)
		return
	}
	if pc == nil {
		writeError(w, http.StatusNotFound, ErrCodeNotFound, "provider config not found", requestID)
		return
	}

	resp := redactProviderConfig(pc)
	writeJSON(w, http.StatusOK, resp)
}

// List handles GET /api/v1/provider-configs.
func (h *ProviderConfigHandler) List(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())

	namespace := r.URL.Query().Get("namespace")
	cursor := r.URL.Query().Get("cursor")
	limit := parseLimit(r)

	configs, nextCursor, err := h.store.ListProviderConfigs(r.Context(), tenantID, namespace, cursor, limit)
	if err != nil {
		h.logger.Error("failed to list provider configs", "error", err, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to list provider configs", requestID)
		return
	}

	// Redact api_key_ref in each item.
	redacted := make([]*providerConfigResponse, 0, len(configs))
	for _, pc := range configs {
		redacted = append(redacted, redactProviderConfig(pc))
	}

	writeJSON(w, http.StatusOK, ListResponse[*providerConfigResponse]{
		Items:      redacted,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	})
}

// Update handles PUT /api/v1/provider-configs/{id}.
func (h *ProviderConfigHandler) Update(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())
	id := r.PathValue("id")

	existing, err := h.store.GetProviderConfig(r.Context(), tenantID, id)
	if err != nil {
		h.logger.Error("failed to get provider config for update", "error", err, "provider_config_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to get provider config", requestID)
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, ErrCodeNotFound, "provider config not found", requestID)
		return
	}

	var body struct {
		Name         *string             `json:"name"`
		Provider     *model.ProviderType `json:"provider"`
		BaseURL      *string             `json:"base_url"`
		APIKeyRef    *string             `json:"api_key_ref"`
		DefaultModel *string             `json:"default_model"`
		ExtraHeaders map[string]string   `json:"extra_headers"`
		Metadata     json.RawMessage     `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "invalid request body: "+err.Error(), requestID)
		return
	}

	if body.Name != nil {
		existing.Name = *body.Name
	}
	if body.Provider != nil {
		existing.Provider = *body.Provider
	}
	if body.BaseURL != nil {
		existing.BaseURL = *body.BaseURL
	}
	if body.APIKeyRef != nil {
		existing.APIKeyRef = *body.APIKeyRef
	}
	if body.DefaultModel != nil {
		existing.DefaultModel = *body.DefaultModel
	}
	if body.ExtraHeaders != nil {
		existing.ExtraHeaders = body.ExtraHeaders
	}
	if body.Metadata != nil {
		existing.Metadata = body.Metadata
	}

	existing.UpdatedAt = time.Now().UTC()

	if err := h.store.UpdateProviderConfig(r.Context(), existing); err != nil {
		h.logger.Error("failed to update provider config", "error", err, "provider_config_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to update provider config", requestID)
		return
	}

	h.logger.Info("provider config updated", "provider_config_id", id, "request_id", requestID)

	resp := redactProviderConfig(existing)
	writeJSON(w, http.StatusOK, resp)
}

// Delete handles DELETE /api/v1/provider-configs/{id}.
func (h *ProviderConfigHandler) Delete(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())
	id := r.PathValue("id")

	if err := h.store.DeleteProviderConfig(r.Context(), tenantID, id); err != nil {
		h.logger.Error("failed to delete provider config", "error", err, "provider_config_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to delete provider config", requestID)
		return
	}

	h.logger.Info("provider config deleted", "provider_config_id", id, "request_id", requestID)
	w.WriteHeader(http.StatusNoContent)
}

// --- Helpers ---

// providerConfigResponse is the API response shape for provider configs.
// It mirrors ProviderConfigLibrary but redacts the api_key_ref field.
type providerConfigResponse struct {
	ID           string             `json:"id"`
	TenantID     string             `json:"tenant_id"`
	Name         string             `json:"name"`
	Namespace    string             `json:"namespace"`
	Provider     model.ProviderType `json:"provider"`
	BaseURL      string             `json:"base_url,omitempty"`
	APIKeyRef    string             `json:"api_key_ref"`
	DefaultModel string             `json:"default_model,omitempty"`
	ExtraHeaders map[string]string  `json:"extra_headers,omitempty"`
	Metadata     json.RawMessage    `json:"metadata,omitempty"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

// redactProviderConfig returns a response copy with api_key_ref masked.
func redactProviderConfig(pc *model.ProviderConfigLibrary) *providerConfigResponse {
	apiKeyDisplay := "***"
	if pc.APIKeyRef == "" {
		apiKeyDisplay = ""
	}
	return &providerConfigResponse{
		ID:           pc.ID,
		TenantID:     pc.TenantID,
		Name:         pc.Name,
		Namespace:    pc.Namespace,
		Provider:     pc.Provider,
		BaseURL:      pc.BaseURL,
		APIKeyRef:    apiKeyDisplay,
		DefaultModel: pc.DefaultModel,
		ExtraHeaders: pc.ExtraHeaders,
		Metadata:     pc.Metadata,
		CreatedAt:    pc.CreatedAt,
		UpdatedAt:    pc.UpdatedAt,
	}
}

// parseLimit extracts and validates the limit query parameter.
func parseLimit(r *http.Request) int {
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}
	return limit
}

func generateSchemaID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return "schema_" + hex.EncodeToString(b)
}

func generatePromptTemplateID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return "tmpl_" + hex.EncodeToString(b)
}

func generateProviderConfigID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return "prov_" + hex.EncodeToString(b)
}

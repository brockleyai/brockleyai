package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/brockleyai/brockleyai/engine/graph"
	"github.com/brockleyai/brockleyai/internal/model"
	"github.com/brockleyai/brockleyai/server/middleware"
)

// GraphHandler handles /api/v1/graphs endpoints.
type GraphHandler struct {
	store  model.Store
	logger *slog.Logger
}

// NewGraphHandler creates a new GraphHandler.
func NewGraphHandler(store model.Store, logger *slog.Logger) *GraphHandler {
	return &GraphHandler{store: store, logger: logger}
}

// Create handles POST /api/v1/graphs.
func (h *GraphHandler) Create(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())

	var body struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Namespace   string          `json:"namespace"`
		Status      string          `json:"status"`
		Nodes       json.RawMessage `json:"nodes"`
		Edges       json.RawMessage `json:"edges"`
		State       json.RawMessage `json:"state"`
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
	if body.Namespace == "" {
		body.Namespace = "default"
	}

	var nodes []model.Node
	if body.Nodes != nil {
		if err := json.Unmarshal(body.Nodes, &nodes); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeValidation, "invalid nodes: "+err.Error(), requestID)
			return
		}
	}

	var edges []model.Edge
	if body.Edges != nil {
		if err := json.Unmarshal(body.Edges, &edges); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeValidation, "invalid edges: "+err.Error(), requestID)
			return
		}
	}

	var state *model.GraphState
	if body.State != nil {
		state = &model.GraphState{}
		if err := json.Unmarshal(body.State, state); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeValidation, "invalid state: "+err.Error(), requestID)
			return
		}
	}

	status := model.GraphStatusDraft
	if body.Status != "" {
		status = model.GraphStatus(body.Status)
	}

	now := time.Now().UTC()
	graph := &model.Graph{
		ID:          generateGraphID(),
		TenantID:    tenantID,
		Name:        body.Name,
		Description: body.Description,
		Namespace:   body.Namespace,
		Version:     1,
		Status:      status,
		Nodes:       nodes,
		Edges:       edges,
		State:       state,
		Metadata:    body.Metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.store.CreateGraph(r.Context(), graph); err != nil {
		h.logger.Error("failed to create graph", "error", err, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to create graph", requestID)
		return
	}

	h.logger.Info("graph created", "graph_id", graph.ID, "name", graph.Name, "request_id", requestID)
	resp := copyGraphForMasking(graph)
	maskGraphSecrets(resp)
	writeJSON(w, http.StatusCreated, resp)
}

// Get handles GET /api/v1/graphs/{id}.
func (h *GraphHandler) Get(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())
	id := r.PathValue("id")

	graph, err := h.store.GetGraph(r.Context(), tenantID, id)
	if err != nil {
		h.logger.Error("failed to get graph", "error", err, "graph_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to get graph", requestID)
		return
	}
	if graph == nil {
		writeError(w, http.StatusNotFound, ErrCodeNotFound, "graph not found", requestID)
		return
	}

	resp := copyGraphForMasking(graph)
	maskGraphSecrets(resp)
	writeJSON(w, http.StatusOK, resp)
}

// List handles GET /api/v1/graphs.
func (h *GraphHandler) List(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())

	namespace := r.URL.Query().Get("namespace")
	cursor := r.URL.Query().Get("cursor")
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	graphs, nextCursor, err := h.store.ListGraphs(r.Context(), tenantID, namespace, cursor, limit)
	if err != nil {
		h.logger.Error("failed to list graphs", "error", err, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to list graphs", requestID)
		return
	}

	maskedGraphs := make([]*model.Graph, len(graphs))
	for i, g := range graphs {
		maskedGraphs[i] = copyGraphForMasking(g)
		maskGraphSecrets(maskedGraphs[i])
	}
	writeJSON(w, http.StatusOK, ListResponse[*model.Graph]{
		Items:      maskedGraphs,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	})
}

// Update handles PUT /api/v1/graphs/{id}.
func (h *GraphHandler) Update(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())
	id := r.PathValue("id")

	existing, err := h.store.GetGraph(r.Context(), tenantID, id)
	if err != nil {
		h.logger.Error("failed to get graph for update", "error", err, "graph_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to get graph", requestID)
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, ErrCodeNotFound, "graph not found", requestID)
		return
	}

	var body struct {
		Name        *string         `json:"name"`
		Description *string         `json:"description"`
		Status      *string         `json:"status"`
		Nodes       json.RawMessage `json:"nodes"`
		Edges       json.RawMessage `json:"edges"`
		State       json.RawMessage `json:"state"`
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
	if body.Status != nil {
		existing.Status = model.GraphStatus(*body.Status)
	}
	if body.Nodes != nil {
		var nodes []model.Node
		if err := json.Unmarshal(body.Nodes, &nodes); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeValidation, "invalid nodes: "+err.Error(), requestID)
			return
		}
		preserveSecretsOnUpdate(existing.Nodes, nodes)
		existing.Nodes = nodes
	}
	if body.Edges != nil {
		var edges []model.Edge
		if err := json.Unmarshal(body.Edges, &edges); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeValidation, "invalid edges: "+err.Error(), requestID)
			return
		}
		existing.Edges = edges
	}
	if body.State != nil {
		var state model.GraphState
		if err := json.Unmarshal(body.State, &state); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeValidation, "invalid state: "+err.Error(), requestID)
			return
		}
		existing.State = &state
	}
	if body.Metadata != nil {
		existing.Metadata = body.Metadata
	}

	existing.Version++
	existing.UpdatedAt = time.Now().UTC()

	if err := h.store.UpdateGraph(r.Context(), existing); err != nil {
		h.logger.Error("failed to update graph", "error", err, "graph_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to update graph", requestID)
		return
	}

	h.logger.Info("graph updated", "graph_id", id, "version", existing.Version, "request_id", requestID)
	resp := copyGraphForMasking(existing)
	maskGraphSecrets(resp)
	writeJSON(w, http.StatusOK, resp)
}

// Delete handles DELETE /api/v1/graphs/{id}.
func (h *GraphHandler) Delete(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())
	id := r.PathValue("id")

	if err := h.store.DeleteGraph(r.Context(), tenantID, id); err != nil {
		h.logger.Error("failed to delete graph", "error", err, "graph_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to delete graph", requestID)
		return
	}

	h.logger.Info("graph deleted", "graph_id", id, "request_id", requestID)
	w.WriteHeader(http.StatusNoContent)
}

// Validate handles POST /api/v1/graphs/{id}/validate.
func (h *GraphHandler) Validate(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())
	id := r.PathValue("id")

	g, err := h.store.GetGraph(r.Context(), tenantID, id)
	if err != nil {
		h.logger.Error("failed to get graph for validation", "error", err, "graph_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to get graph", requestID)
		return
	}
	if g == nil {
		writeError(w, http.StatusNotFound, ErrCodeNotFound, "graph not found", requestID)
		return
	}

	result := graph.Validate(g)

	h.logger.Info("graph validated", "graph_id", id, "valid", result.Valid, "errors", len(result.Errors), "request_id", requestID)

	if result.Valid {
		writeJSON(w, http.StatusOK, result)
	} else {
		writeJSON(w, http.StatusUnprocessableEntity, result)
	}
}

func generateGraphID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return "graph_" + hex.EncodeToString(b)
}

// copyGraphForMasking creates a shallow copy of the graph with a deep-copied Nodes slice,
// so that masking doesn't mutate the stored original.
func copyGraphForMasking(g *model.Graph) *model.Graph {
	cp := *g
	if len(g.Nodes) > 0 {
		cp.Nodes = make([]model.Node, len(g.Nodes))
		for i, n := range g.Nodes {
			cp.Nodes[i] = n
			if len(n.Config) > 0 {
				cfgCopy := make(json.RawMessage, len(n.Config))
				copy(cfgCopy, n.Config)
				cp.Nodes[i].Config = cfgCopy
			}
		}
	}
	return &cp
}

// maskSecret masks an API key for safe display: first 4 + "..." + last 4.
// Short keys (<=8 chars) are fully masked as "****".
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "..." + s[len(s)-4:]
}

// isMaskedKey returns true if the string looks like a masked key rather than a real one.
// Masked keys are short and contain "..." or equal "****".
func isMaskedKey(s string) bool {
	if s == "" {
		return false
	}
	if s == "****" {
		return true
	}
	return len(s) <= 11 && strings.Contains(s, "...")
}

// maskGraphSecrets walks all nodes in the graph and masks api_key values on LLM nodes.
func maskGraphSecrets(g *model.Graph) {
	for i := range g.Nodes {
		maskNodeSecret(&g.Nodes[i])
	}
}

// maskNodeSecret masks the api_key in a single node's config if it's an LLM node.
// Also recurses into foreach and subgraph inner graphs.
func maskNodeSecret(n *model.Node) {
	switch n.Type {
	case model.NodeTypeLLM:
		var cfg map[string]any
		if json.Unmarshal(n.Config, &cfg) != nil {
			return
		}
		if apiKey, ok := cfg["api_key"].(string); ok && apiKey != "" {
			cfg["api_key"] = maskSecret(apiKey)
			if b, err := json.Marshal(cfg); err == nil {
				n.Config = b
			}
		}
	case model.NodeTypeForEach:
		var cfg map[string]any
		if json.Unmarshal(n.Config, &cfg) != nil {
			return
		}
		if graphRaw, ok := cfg["graph"]; ok {
			maskInnerGraph(graphRaw, cfg, "graph", n)
		}
	case model.NodeTypeSubgraph:
		var cfg map[string]any
		if json.Unmarshal(n.Config, &cfg) != nil {
			return
		}
		if graphRaw, ok := cfg["graph"]; ok {
			maskInnerGraph(graphRaw, cfg, "graph", n)
		}
	}
}

// maskInnerGraph masks secrets in an inner graph embedded in a config field.
func maskInnerGraph(graphRaw any, cfg map[string]any, key string, n *model.Node) {
	graphBytes, err := json.Marshal(graphRaw)
	if err != nil {
		return
	}
	var inner model.Graph
	if json.Unmarshal(graphBytes, &inner) != nil {
		return
	}
	maskGraphSecrets(&inner)
	innerBytes, err := json.Marshal(inner)
	if err != nil {
		return
	}
	var innerRaw any
	if json.Unmarshal(innerBytes, &innerRaw) != nil {
		return
	}
	cfg[key] = innerRaw
	if b, err := json.Marshal(cfg); err == nil {
		n.Config = b
	}
}

// preserveSecretsOnUpdate preserves real API keys when the incoming update contains masked values.
// For each incoming LLM node with a masked api_key, copy the stored key from the matching existing node.
func preserveSecretsOnUpdate(existing, incoming []model.Node) {
	existingByID := make(map[string]*model.Node, len(existing))
	for i := range existing {
		existingByID[existing[i].ID] = &existing[i]
	}

	for i := range incoming {
		if incoming[i].Type != model.NodeTypeLLM {
			continue
		}

		var inCfg map[string]any
		if json.Unmarshal(incoming[i].Config, &inCfg) != nil {
			continue
		}

		apiKey, ok := inCfg["api_key"].(string)
		if !ok || apiKey == "" {
			continue
		}

		if !isMaskedKey(apiKey) {
			// Real new key — keep it.
			continue
		}

		// Masked key — restore from existing node.
		existNode, found := existingByID[incoming[i].ID]
		if !found {
			continue
		}

		var exCfg map[string]any
		if json.Unmarshal(existNode.Config, &exCfg) != nil {
			continue
		}

		if realKey, ok := exCfg["api_key"].(string); ok && realKey != "" {
			inCfg["api_key"] = realKey
			if b, err := json.Marshal(inCfg); err == nil {
				incoming[i].Config = b
			}
		}
	}
}

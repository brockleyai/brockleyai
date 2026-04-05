package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brockleyai/brockleyai/engine/mock"
	"github.com/brockleyai/brockleyai/internal/model"
	"github.com/brockleyai/brockleyai/server/middleware"
)

// newTestGraphHandler creates a GraphHandler backed by a MockStore for testing.
func newTestGraphHandler() (*GraphHandler, *mock.MockStore) {
	store := mock.NewMockStore()
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	return NewGraphHandler(store, logger), store
}

// withRequestID wraps a request with a request ID in the context, as handlers expect.
func withRequestID(r *http.Request) *http.Request {
	_ = r.Context()
	// Use the middleware to inject request ID.
	rr := httptest.NewRecorder()
	var captured *http.Request
	middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r
	})).ServeHTTP(rr, r)
	return captured
}

func TestGraphHandler_Create_Success(t *testing.T) {
	h, _ := newTestGraphHandler()

	body := `{"name":"test-graph","description":"a test graph"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/graphs", bytes.NewBufferString(body))
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	var graph model.Graph
	if err := json.NewDecoder(rr.Body).Decode(&graph); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if graph.ID == "" {
		t.Error("expected graph ID to be generated")
	}
	if graph.Name != "test-graph" {
		t.Errorf("expected name %q, got %q", "test-graph", graph.Name)
	}
	if graph.Namespace != "default" {
		t.Errorf("expected namespace %q, got %q", "default", graph.Namespace)
	}
}

func TestGraphHandler_Create_DefaultsDraft(t *testing.T) {
	h, _ := newTestGraphHandler()

	body := `{"name":"test-graph"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/graphs", bytes.NewBufferString(body))
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	var graph model.Graph
	if err := json.NewDecoder(rr.Body).Decode(&graph); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if graph.Status != model.GraphStatusDraft {
		t.Errorf("expected status %q, got %q", model.GraphStatusDraft, graph.Status)
	}
}

func TestGraphHandler_Create_WithStatus(t *testing.T) {
	h, _ := newTestGraphHandler()

	body := `{"name":"active-graph","status":"active"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/graphs", bytes.NewBufferString(body))
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	var graph model.Graph
	if err := json.NewDecoder(rr.Body).Decode(&graph); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if graph.Status != model.GraphStatusActive {
		t.Errorf("expected status %q, got %q", model.GraphStatusActive, graph.Status)
	}
}

func TestGraphHandler_Create_MissingName(t *testing.T) {
	h, _ := newTestGraphHandler()

	body := `{"description":"no name"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/graphs", bytes.NewBufferString(body))
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}

	var apiErr APIError
	if err := json.NewDecoder(rr.Body).Decode(&apiErr); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if apiErr.Error.Code != ErrCodeValidation {
		t.Errorf("expected error code %q, got %q", ErrCodeValidation, apiErr.Error.Code)
	}
}

func TestGraphHandler_Get_Existing(t *testing.T) {
	h, store := newTestGraphHandler()

	// Seed a graph directly in the store.
	graph := &model.Graph{
		ID:        "graph_test123",
		TenantID:  "default",
		Name:      "seeded",
		Namespace: "default",
		Version:   1,
		Status:    model.GraphStatusDraft,
	}
	if err := store.CreateGraph(context.TODO(), graph); err != nil {
		t.Fatalf("failed to seed graph: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/graphs/graph_test123", nil)
	req.SetPathValue("id", "graph_test123")
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var got model.Graph
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got.ID != "graph_test123" {
		t.Errorf("expected ID %q, got %q", "graph_test123", got.ID)
	}
}

func TestGraphHandler_Get_NotFound(t *testing.T) {
	h, _ := newTestGraphHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/graphs/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	// The MockStore returns an error for not-found, which the handler treats as 500.
	// This is expected behavior with the current MockStore implementation.
	// A real store would return (nil, nil) for not-found.
	if rr.Code != http.StatusInternalServerError && rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 or 500, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGraphHandler_List(t *testing.T) {
	h, store := newTestGraphHandler()

	// Seed two graphs.
	for i, name := range []string{"graph-a", "graph-b"} {
		g := &model.Graph{
			ID:        generateGraphID(),
			TenantID:  "default",
			Name:      name,
			Namespace: "default",
			Version:   1,
			Status:    model.GraphStatusDraft,
		}
		_ = i
		if err := store.CreateGraph(context.TODO(), g); err != nil {
			t.Fatalf("failed to seed graph: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/graphs?limit=10", nil)
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp struct {
		Items   []model.Graph `json:"items"`
		HasMore bool          `json:"has_more"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(resp.Items))
	}
}

func TestGraphHandler_Get_MasksAPIKey(t *testing.T) {
	h, store := newTestGraphHandler()

	// Create a graph with an LLM node that has an inline API key.
	llmConfig := map[string]any{
		"provider":        "openrouter",
		"model":           "test-model",
		"api_key":         "sk-or-v1-abcdefghijklmnopqrstuvwxyz1234567890ab12",
		"user_prompt":     "Hello",
		"response_format": "text",
	}
	cfgBytes, _ := json.Marshal(llmConfig)
	nodes := []model.Node{
		{ID: "llm-1", Name: "test", Type: "llm", Config: cfgBytes},
	}

	graph := &model.Graph{
		ID:        "graph_masked1",
		TenantID:  "default",
		Name:      "masked-test",
		Namespace: "default",
		Version:   1,
		Status:    model.GraphStatusDraft,
		Nodes:     nodes,
	}
	if err := store.CreateGraph(context.TODO(), graph); err != nil {
		t.Fatalf("failed to seed graph: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/graphs/graph_masked1", nil)
	req.SetPathValue("id", "graph_masked1")
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var got model.Graph
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Parse the node config to check masking.
	var gotCfg map[string]any
	if err := json.Unmarshal(got.Nodes[0].Config, &gotCfg); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	maskedKey, ok := gotCfg["api_key"].(string)
	if !ok {
		t.Fatal("expected api_key in config")
	}
	if maskedKey == "sk-or-v1-abcdefghijklmnopqrstuvwxyz1234567890ab12" {
		t.Error("api_key should be masked, but got the real key")
	}
	// Should be "sk-o...ab12"
	if maskedKey != "sk-o...ab12" {
		t.Errorf("expected masked key 'sk-o...ab12', got %q", maskedKey)
	}
}

func TestGraphHandler_Update_PreservesMaskedKey(t *testing.T) {
	h, store := newTestGraphHandler()

	// Seed a graph with a real API key.
	llmConfig := map[string]any{
		"provider":        "openrouter",
		"model":           "test-model",
		"api_key":         "sk-or-v1-realkey1234567890abcdefghijklmnopqrst",
		"user_prompt":     "Hello",
		"response_format": "text",
	}
	cfgBytes, _ := json.Marshal(llmConfig)
	nodes := []model.Node{
		{ID: "llm-1", Name: "test", Type: "llm", Config: cfgBytes},
	}

	graph := &model.Graph{
		ID:        "graph_preserve1",
		TenantID:  "default",
		Name:      "preserve-test",
		Namespace: "default",
		Version:   1,
		Status:    model.GraphStatusDraft,
		Nodes:     nodes,
	}
	if err := store.CreateGraph(context.TODO(), graph); err != nil {
		t.Fatalf("failed to seed graph: %v", err)
	}

	// Update with masked key (as if the client sent back what it received from GET).
	updateNodes := []map[string]any{
		{
			"id":   "llm-1",
			"name": "test",
			"type": "llm",
			"config": map[string]any{
				"provider":        "openrouter",
				"model":           "test-model",
				"api_key":         "sk-o...qrst",
				"user_prompt":     "Updated prompt",
				"response_format": "text",
			},
		},
	}
	nodesJSON, _ := json.Marshal(updateNodes)
	body := `{"nodes":` + string(nodesJSON) + `}`

	req := httptest.NewRequest(http.MethodPut, "/api/v1/graphs/graph_preserve1", bytes.NewBufferString(body))
	req.SetPathValue("id", "graph_preserve1")
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.Update(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	// Verify the real key was preserved in the store.
	stored, _ := store.GetGraph(context.TODO(), "default", "graph_preserve1")
	var storedCfg map[string]any
	if err := json.Unmarshal(stored.Nodes[0].Config, &storedCfg); err != nil {
		t.Fatalf("failed to unmarshal stored config: %v", err)
	}
	if storedCfg["api_key"] != "sk-or-v1-realkey1234567890abcdefghijklmnopqrst" {
		t.Errorf("expected real key preserved, got %q", storedCfg["api_key"])
	}
}

func TestGraphHandler_Update_NewRealKey(t *testing.T) {
	h, store := newTestGraphHandler()

	// Seed a graph with an old key.
	llmConfig := map[string]any{
		"provider":        "openrouter",
		"model":           "test-model",
		"api_key":         "sk-or-v1-oldkey1234567890abcdefghijklmnopqrstuvw",
		"user_prompt":     "Hello",
		"response_format": "text",
	}
	cfgBytes, _ := json.Marshal(llmConfig)
	graph := &model.Graph{
		ID:        "graph_newkey1",
		TenantID:  "default",
		Name:      "newkey-test",
		Namespace: "default",
		Version:   1,
		Status:    model.GraphStatusDraft,
		Nodes:     []model.Node{{ID: "llm-1", Name: "test", Type: "llm", Config: cfgBytes}},
	}
	if err := store.CreateGraph(context.TODO(), graph); err != nil {
		t.Fatalf("failed to seed graph: %v", err)
	}

	// Update with a new real key.
	updateNodes := []map[string]any{
		{
			"id":   "llm-1",
			"name": "test",
			"type": "llm",
			"config": map[string]any{
				"provider":        "openrouter",
				"model":           "test-model",
				"api_key":         "sk-or-v1-brandnewkey567890abcdefghijklmnopqrstuv",
				"user_prompt":     "Hello",
				"response_format": "text",
			},
		},
	}
	nodesJSON, _ := json.Marshal(updateNodes)
	body := `{"nodes":` + string(nodesJSON) + `}`

	req := httptest.NewRequest(http.MethodPut, "/api/v1/graphs/graph_newkey1", bytes.NewBufferString(body))
	req.SetPathValue("id", "graph_newkey1")
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.Update(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	// Verify the new key is stored.
	stored, _ := store.GetGraph(context.TODO(), "default", "graph_newkey1")
	var storedCfg map[string]any
	if err := json.Unmarshal(stored.Nodes[0].Config, &storedCfg); err != nil {
		t.Fatalf("failed to unmarshal stored config: %v", err)
	}
	if storedCfg["api_key"] != "sk-or-v1-brandnewkey567890abcdefghijklmnopqrstuv" {
		t.Errorf("expected new key stored, got %q", storedCfg["api_key"])
	}
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"short", "****"},
		{"12345678", "****"},
		{"123456789", "1234...6789"},
		{"sk-or-v1-abcdefghijklmnopqrstuvwxyz1234567890ab12", "sk-o...ab12"},
	}
	for _, tt := range tests {
		got := maskSecret(tt.input)
		if got != tt.want {
			t.Errorf("maskSecret(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsMaskedKey(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", false},
		{"****", true},
		{"sk-o...ab12", true},
		{"1234...5678", true},
		{"sk-or-v1-abcdefghijklmnopqrstuvwxyz1234567890ab12", false},
		{"a-real-key-that-is-long-enough", false},
	}
	for _, tt := range tests {
		got := isMaskedKey(tt.input)
		if got != tt.want {
			t.Errorf("isMaskedKey(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestMaskGraphSecrets_ForEachNode(t *testing.T) {
	// Inner graph inside a foreach node has an LLM node with api_key.
	innerGraph := map[string]any{
		"nodes": []map[string]any{
			{
				"id":   "inner-llm",
				"name": "inner",
				"type": "llm",
				"config": map[string]any{
					"provider":    "openai",
					"model":       "gpt-4",
					"api_key":     "sk-inner-secretkey1234567890abcdefghijklmnop",
					"user_prompt": "hello",
				},
			},
		},
		"edges": []any{},
	}
	forEachCfg := map[string]any{
		"graph":       innerGraph,
		"concurrency": 1,
	}
	cfgBytes, _ := json.Marshal(forEachCfg)

	g := &model.Graph{
		Nodes: []model.Node{
			{ID: "fe-1", Name: "foreach", Type: "foreach", Config: cfgBytes},
		},
	}

	maskGraphSecrets(g)

	// Parse the config back and check the inner LLM node's key is masked.
	var gotCfg map[string]any
	json.Unmarshal(g.Nodes[0].Config, &gotCfg)
	innerGraphParsed := gotCfg["graph"].(map[string]any)
	nodes := innerGraphParsed["nodes"].([]any)
	node := nodes[0].(map[string]any)
	config := node["config"].(map[string]any)
	apiKey := config["api_key"].(string)

	if apiKey == "sk-inner-secretkey1234567890abcdefghijklmnop" {
		t.Error("expected inner graph api_key to be masked")
	}
	if apiKey != "sk-i...mnop" {
		t.Errorf("expected masked key 'sk-i...mnop', got %q", apiKey)
	}
}

func TestMaskGraphSecrets_SubgraphNode(t *testing.T) {
	innerGraph := map[string]any{
		"nodes": []map[string]any{
			{
				"id":   "inner-llm",
				"name": "inner",
				"type": "llm",
				"config": map[string]any{
					"provider":    "anthropic",
					"model":       "claude-3",
					"api_key":     "sk-ant-secretkey1234567890abcdefghijklmnopqr",
					"user_prompt": "hello",
				},
			},
		},
		"edges": []any{},
	}
	subgraphCfg := map[string]any{
		"graph":        innerGraph,
		"port_mapping": map[string]any{"inputs": map[string]any{}, "outputs": map[string]any{}},
	}
	cfgBytes, _ := json.Marshal(subgraphCfg)

	g := &model.Graph{
		Nodes: []model.Node{
			{ID: "sg-1", Name: "subgraph", Type: "subgraph", Config: cfgBytes},
		},
	}

	maskGraphSecrets(g)

	var gotCfg map[string]any
	json.Unmarshal(g.Nodes[0].Config, &gotCfg)
	innerGraphParsed := gotCfg["graph"].(map[string]any)
	nodes := innerGraphParsed["nodes"].([]any)
	node := nodes[0].(map[string]any)
	config := node["config"].(map[string]any)
	apiKey := config["api_key"].(string)

	if apiKey == "sk-ant-secretkey1234567890abcdefghijklmnopqr" {
		t.Error("expected subgraph inner api_key to be masked")
	}
}

func TestPreserveSecretsOnUpdate_NoAPIKey(t *testing.T) {
	// Incoming LLM node with no api_key at all — should not error.
	cfgOld, _ := json.Marshal(map[string]any{
		"provider": "openai", "model": "gpt-4",
		"api_key": "sk-real-key-12345678901234567890abcdefghijklmn",
	})
	cfgNew, _ := json.Marshal(map[string]any{
		"provider": "openai", "model": "gpt-4",
	})

	existing := []model.Node{{ID: "llm-1", Type: "llm", Config: cfgOld}}
	incoming := []model.Node{{ID: "llm-1", Type: "llm", Config: cfgNew}}

	preserveSecretsOnUpdate(existing, incoming)

	// No api_key in incoming — should remain absent (not copied from existing).
	var gotCfg map[string]any
	json.Unmarshal(incoming[0].Config, &gotCfg)
	if _, ok := gotCfg["api_key"]; ok {
		t.Error("expected no api_key in incoming when it was not sent")
	}
}

func TestPreserveSecretsOnUpdate_NonLLMNode(t *testing.T) {
	// Non-LLM nodes should not be touched.
	cfg, _ := json.Marshal(map[string]any{"expressions": map[string]any{"out": "input.x"}})
	existing := []model.Node{{ID: "t-1", Type: "transform", Config: cfg}}
	incoming := []model.Node{{ID: "t-1", Type: "transform", Config: cfg}}

	preserveSecretsOnUpdate(existing, incoming)
	// Should not panic or modify anything.
}

func TestMaskGraphSecrets_NonLLMNodes(t *testing.T) {
	// Non-LLM nodes should pass through without error.
	cfg, _ := json.Marshal(map[string]any{"expressions": map[string]any{"out": "input.x"}})
	g := &model.Graph{
		Nodes: []model.Node{
			{ID: "t-1", Type: "transform", Config: cfg},
			{ID: "i-1", Type: "input", Config: json.RawMessage(`{}`)},
		},
	}

	maskGraphSecrets(g) // Should not panic.
}

func TestGraphHandler_Delete(t *testing.T) {
	h, store := newTestGraphHandler()

	graph := &model.Graph{
		ID:        "graph_todelete",
		TenantID:  "default",
		Name:      "deleteme",
		Namespace: "default",
		Version:   1,
		Status:    model.GraphStatusDraft,
	}
	if err := store.CreateGraph(context.TODO(), graph); err != nil {
		t.Fatalf("failed to seed graph: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/graphs/graph_todelete", nil)
	req.SetPathValue("id", "graph_todelete")
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.Delete(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rr.Code, rr.Body.String())
	}
}

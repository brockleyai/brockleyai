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
)

func newTestAPIToolHandler() (*APIToolHandler, *mock.MockStore) {
	store := mock.NewMockStore()
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	return NewAPIToolHandler(store, logger), store
}

func seedAPITool(t *testing.T, store *mock.MockStore, backendURL string) *model.APIToolDefinition {
	t.Helper()
	at := &model.APIToolDefinition{
		ID:        "atool_test1",
		TenantID:  "default",
		Name:      "Test API",
		Namespace: "default",
		BaseURL:   backendURL,
		Endpoints: []model.APIEndpoint{
			{
				Name:        "get_user",
				Description: "Get a user by ID",
				Method:      "GET",
				Path:        "/users/{{input.user_id}}",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"user_id":{"type":"string"}}}`),
			},
			{
				Name:        "create_user",
				Description: "Create a new user",
				Method:      "POST",
				Path:        "/users",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`),
			},
		},
	}
	if err := store.CreateAPITool(context.TODO(), at); err != nil {
		t.Fatalf("failed to seed api tool: %v", err)
	}
	return at
}

func TestAPIToolHandler_Test_Success(t *testing.T) {
	// Start a test HTTP server that the dispatcher will call.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/usr_42" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"id": "usr_42", "name": "Alice"})
			return
		}
		http.NotFound(w, r)
	}))
	defer backend.Close()

	h, store := newTestAPIToolHandler()
	seedAPITool(t, store, backend.URL)

	body := `{"endpoint":"get_user","input":{"user_id":"usr_42"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/api-tools/atool_test1/test", bytes.NewBufferString(body))
	req.SetPathValue("id", "atool_test1")
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.Test(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["success"] != true {
		t.Errorf("expected success=true, got %v; error=%v", resp["success"], resp["error"])
	}
	if resp["is_error"] != false {
		t.Error("expected is_error=false")
	}

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result to be a map, got %T", resp["result"])
	}
	if result["name"] != "Alice" {
		t.Errorf("expected name=Alice, got %v", result["name"])
	}
}

func TestAPIToolHandler_Test_EndpointNotFound(t *testing.T) {
	h, store := newTestAPIToolHandler()
	seedAPITool(t, store, "http://localhost:9999")

	body := `{"endpoint":"nonexistent","input":{}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/api-tools/atool_test1/test", bytes.NewBufferString(body))
	req.SetPathValue("id", "atool_test1")
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.Test(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestAPIToolHandler_Test_ToolNotFound(t *testing.T) {
	h, _ := newTestAPIToolHandler()

	body := `{"endpoint":"get_user","input":{}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/api-tools/atool_nonexistent/test", bytes.NewBufferString(body))
	req.SetPathValue("id", "atool_nonexistent")
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.Test(rr, req)

	// MockStore returns error for missing items, handler treats as 500.
	if rr.Code != http.StatusInternalServerError && rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 or 500, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAPIToolHandler_Test_MissingEndpoint(t *testing.T) {
	h, _ := newTestAPIToolHandler()

	body := `{"input":{}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/api-tools/atool_test1/test", bytes.NewBufferString(body))
	req.SetPathValue("id", "atool_test1")
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.Test(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestAPIToolHandler_Test_HTTPError(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	}))
	defer backend.Close()

	h, store := newTestAPIToolHandler()
	seedAPITool(t, store, backend.URL)

	body := `{"endpoint":"get_user","input":{"user_id":"missing"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/api-tools/atool_test1/test", bytes.NewBufferString(body))
	req.SetPathValue("id", "atool_test1")
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.Test(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["success"] != false {
		t.Error("expected success=false for HTTP error")
	}
	if resp["is_error"] != true {
		t.Error("expected is_error=true for HTTP error")
	}
}

func TestAPIToolHandler_Test_BaseURLOverride(t *testing.T) {
	// Original backend (should NOT be called).
	originalBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"source": "original"})
	}))
	defer originalBackend.Close()

	// Override backend (should be called).
	overrideBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"source": "override"})
	}))
	defer overrideBackend.Close()

	h, store := newTestAPIToolHandler()
	seedAPITool(t, store, originalBackend.URL)

	body, _ := json.Marshal(map[string]any{
		"endpoint":          "get_user",
		"input":             map[string]any{"user_id": "1"},
		"base_url_override": overrideBackend.URL,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/api-tools/atool_test1/test", bytes.NewReader(body))
	req.SetPathValue("id", "atool_test1")
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.Test(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["success"] != true {
		t.Errorf("expected success=true, got %v; error=%v", resp["success"], resp["error"])
	}

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result to be a map, got %T", resp["result"])
	}
	if result["source"] != "override" {
		t.Errorf("expected source=override, got %v — base_url_override was not applied", result["source"])
	}
}

func TestAPIToolHandler_Test_PostEndpoint(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users" && r.Method == "POST" {
			var input map[string]any
			json.NewDecoder(r.Body).Decode(&input)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"id": "new_user", "name": input["name"]})
			return
		}
		http.NotFound(w, r)
	}))
	defer backend.Close()

	h, store := newTestAPIToolHandler()
	seedAPITool(t, store, backend.URL)

	body := `{"endpoint":"create_user","input":{"name":"Bob"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/api-tools/atool_test1/test", bytes.NewBufferString(body))
	req.SetPathValue("id", "atool_test1")
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.Test(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["success"] != true {
		t.Errorf("expected success=true, got %v; error=%v", resp["success"], resp["error"])
	}

	result := resp["result"].(map[string]any)
	if result["name"] != "Bob" {
		t.Errorf("expected name=Bob, got %v", result["name"])
	}
}

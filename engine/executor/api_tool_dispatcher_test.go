package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brockleyai/brockleyai/engine/mock"
	"github.com/brockleyai/brockleyai/internal/model"
)

func newTestAPIToolDef(baseURL string) *model.APIToolDefinition {
	return &model.APIToolDefinition{
		ID:       "test-api",
		TenantID: "default",
		Name:     "Test API",
		BaseURL:  baseURL,
		Endpoints: []model.APIEndpoint{
			{
				Name:        "get_customer",
				Description: "Get a customer by ID",
				Method:      "GET",
				Path:        "/customers/{{input.customer_id}}",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"customer_id":{"type":"string"}},"required":["customer_id"]}`),
			},
			{
				Name:        "create_charge",
				Description: "Create a charge",
				Method:      "POST",
				Path:        "/charges",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"amount":{"type":"integer"},"currency":{"type":"string"}},"required":["amount","currency"]}`),
			},
			{
				Name:           "search",
				Description:    "Search items",
				Method:         "GET",
				Path:           "/search",
				RequestMapping: &model.RequestMapping{Mode: "query_params"},
			},
			{
				Name:           "submit_form",
				Description:    "Submit a form",
				Method:         "POST",
				Path:           "/forms/submit",
				RequestMapping: &model.RequestMapping{Mode: "form"},
			},
			{
				Name:            "text_response",
				Description:     "Returns text",
				Method:          "GET",
				Path:            "/text",
				ResponseMapping: &model.ResponseMapping{Mode: "text"},
			},
			{
				Name:            "jq_response",
				Description:     "JQ extraction",
				Method:          "GET",
				Path:            "/data",
				ResponseMapping: &model.ResponseMapping{Mode: "jq", Expression: ".result.id"},
			},
			{
				Name:            "headers_response",
				Description:     "Returns headers and body",
				Method:          "GET",
				Path:            "/info",
				ResponseMapping: &model.ResponseMapping{Mode: "headers_and_body"},
			},
		},
	}
}

func TestAPIToolDispatcher_PathTemplateResolution(t *testing.T) {
	path := "/customers/{{input.customer_id}}/orders/{{input.order_id}}"
	args := map[string]any{
		"customer_id": "cus_123",
		"order_id":    "ord_456",
		"extra":       "value",
	}

	resolved, remaining := resolvePathTemplate(path, args)

	if resolved != "/customers/cus_123/orders/ord_456" {
		t.Errorf("unexpected resolved path: %s", resolved)
	}
	if _, ok := remaining["customer_id"]; ok {
		t.Error("customer_id should have been consumed")
	}
	if _, ok := remaining["order_id"]; ok {
		t.Error("order_id should have been consumed")
	}
	if _, ok := remaining["extra"]; !ok {
		t.Error("extra should remain")
	}
}

func TestAPIToolDispatcher_JSONBody(t *testing.T) {
	var receivedBody map[string]any
	var receivedMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": "ch_123", "status": "succeeded"})
	}))
	defer ts.Close()

	store := mock.NewMockStore()
	def := newTestAPIToolDef(ts.URL)
	store.CreateAPITool(context.Background(), def)

	d := NewAPIToolDispatcher(store, slog.Default())
	result, err := d.CallEndpoint(context.Background(), "default",
		model.ToolRoute{APIToolID: "test-api", APIEndpoint: "create_charge"},
		"create_charge",
		map[string]any{"amount": 1000, "currency": "usd"},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Error)
	}
	if receivedMethod != "POST" {
		t.Errorf("expected POST, got %s", receivedMethod)
	}
	if receivedBody["currency"] != "usd" {
		t.Errorf("expected currency=usd, got %v", receivedBody["currency"])
	}
}

func TestAPIToolDispatcher_PathParams(t *testing.T) {
	var receivedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"email": "test@example.com"})
	}))
	defer ts.Close()

	store := mock.NewMockStore()
	def := newTestAPIToolDef(ts.URL)
	store.CreateAPITool(context.Background(), def)

	d := NewAPIToolDispatcher(store, slog.Default())
	result, err := d.CallEndpoint(context.Background(), "default",
		model.ToolRoute{APIToolID: "test-api", APIEndpoint: "get_customer"},
		"get_customer",
		map[string]any{"customer_id": "cus_123"},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Error)
	}
	if receivedPath != "/customers/cus_123" {
		t.Errorf("expected /customers/cus_123, got %s", receivedPath)
	}
}

func TestAPIToolDispatcher_QueryParams(t *testing.T) {
	var receivedQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		json.NewEncoder(w).Encode(map[string]any{"results": []string{"a", "b"}})
	}))
	defer ts.Close()

	store := mock.NewMockStore()
	def := newTestAPIToolDef(ts.URL)
	store.CreateAPITool(context.Background(), def)

	d := NewAPIToolDispatcher(store, slog.Default())
	result, err := d.CallEndpoint(context.Background(), "default",
		model.ToolRoute{APIToolID: "test-api", APIEndpoint: "search"},
		"search",
		map[string]any{"q": "hello world"},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Error)
	}
	if !strings.Contains(receivedQuery, "q=hello+world") {
		t.Errorf("expected query param q=hello+world, got %s", receivedQuery)
	}
}

func TestAPIToolDispatcher_FormBody(t *testing.T) {
	var receivedContentType string
	var receivedBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer ts.Close()

	store := mock.NewMockStore()
	def := newTestAPIToolDef(ts.URL)
	store.CreateAPITool(context.Background(), def)

	d := NewAPIToolDispatcher(store, slog.Default())
	result, err := d.CallEndpoint(context.Background(), "default",
		model.ToolRoute{APIToolID: "test-api", APIEndpoint: "submit_form"},
		"submit_form",
		map[string]any{"name": "Alice", "age": "30"},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Error)
	}
	if receivedContentType != "application/x-www-form-urlencoded" {
		t.Errorf("expected form content type, got %s", receivedContentType)
	}
	if !strings.Contains(receivedBody, "name=Alice") {
		t.Errorf("expected name=Alice in body, got %s", receivedBody)
	}
}

func TestAPIToolDispatcher_TextResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "Hello, World!")
	}))
	defer ts.Close()

	store := mock.NewMockStore()
	def := newTestAPIToolDef(ts.URL)
	store.CreateAPITool(context.Background(), def)

	d := NewAPIToolDispatcher(store, slog.Default())
	result, err := d.CallEndpoint(context.Background(), "default",
		model.ToolRoute{APIToolID: "test-api", APIEndpoint: "text_response"},
		"text_response",
		map[string]any{},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %v", result.Content)
	}
}

func TestAPIToolDispatcher_JQResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{"id": "item_42", "name": "test"},
		})
	}))
	defer ts.Close()

	store := mock.NewMockStore()
	def := newTestAPIToolDef(ts.URL)
	store.CreateAPITool(context.Background(), def)

	d := NewAPIToolDispatcher(store, slog.Default())
	result, err := d.CallEndpoint(context.Background(), "default",
		model.ToolRoute{APIToolID: "test-api", APIEndpoint: "jq_response"},
		"jq_response",
		map[string]any{},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "item_42" {
		t.Errorf("expected 'item_42', got %v", result.Content)
	}
}

func TestAPIToolDispatcher_HeadersAndBodyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "my-value")
		json.NewEncoder(w).Encode(map[string]any{"data": "test"})
	}))
	defer ts.Close()

	store := mock.NewMockStore()
	def := newTestAPIToolDef(ts.URL)
	store.CreateAPITool(context.Background(), def)

	d := NewAPIToolDispatcher(store, slog.Default())
	result, err := d.CallEndpoint(context.Background(), "default",
		model.ToolRoute{APIToolID: "test-api", APIEndpoint: "headers_response"},
		"headers_response",
		map[string]any{},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.Content.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", result.Content)
	}
	headers, _ := m["headers"].(map[string]string)
	if headers["X-Custom"] != "my-value" {
		t.Errorf("expected X-Custom=my-value, got %v", headers["X-Custom"])
	}
}

func TestAPIToolDispatcher_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		fmt.Fprint(w, "not found")
	}))
	defer ts.Close()

	store := mock.NewMockStore()
	def := newTestAPIToolDef(ts.URL)
	store.CreateAPITool(context.Background(), def)

	d := NewAPIToolDispatcher(store, slog.Default())
	result, err := d.CallEndpoint(context.Background(), "default",
		model.ToolRoute{APIToolID: "test-api", APIEndpoint: "get_customer"},
		"get_customer",
		map[string]any{"customer_id": "bad"},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for HTTP 404")
	}
	if !strings.Contains(result.Error, "HTTP 404") {
		t.Errorf("expected HTTP 404 in error, got %s", result.Error)
	}
}

func TestAPIToolDispatcher_RetrySuccess(t *testing.T) {
	var attempts int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			w.WriteHeader(503)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer ts.Close()

	store := mock.NewMockStore()
	def := newTestAPIToolDef(ts.URL)
	def.Retry = &model.RetryConfig{
		MaxRetries:    3,
		BackoffMs:     10,
		RetryOnStatus: []int{503},
	}
	store.CreateAPITool(context.Background(), def)

	d := NewAPIToolDispatcher(store, slog.Default())
	result, err := d.CallEndpoint(context.Background(), "default",
		model.ToolRoute{APIToolID: "test-api", APIEndpoint: "get_customer"},
		"get_customer",
		map[string]any{"customer_id": "cus_1"},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Error)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestAPIToolDispatcher_HeaderMerging(t *testing.T) {
	var receivedHeaders http.Header
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer ts.Close()

	store := mock.NewMockStore()
	def := newTestAPIToolDef(ts.URL)
	def.DefaultHeaders = []model.HeaderConfig{
		{Name: "X-Default", Value: "default-val"},
		{Name: "X-Override", Value: "from-default"},
	}
	def.Endpoints[1].Headers = []model.HeaderConfig{
		{Name: "X-Endpoint", Value: "endpoint-val"},
		{Name: "X-Override", Value: "from-endpoint"},
	}
	store.CreateAPITool(context.Background(), def)

	d := NewAPIToolDispatcher(store, slog.Default())
	result, err := d.CallEndpoint(context.Background(), "default",
		model.ToolRoute{
			APIToolID:   "test-api",
			APIEndpoint: "create_charge",
			Headers: []model.HeaderConfig{
				{Name: "X-Route", Value: "route-val"},
				{Name: "X-Override", Value: "from-route"},
			},
		},
		"create_charge",
		map[string]any{"amount": 100, "currency": "usd"},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Error)
	}

	if receivedHeaders.Get("X-Default") != "default-val" {
		t.Errorf("expected X-Default=default-val, got %s", receivedHeaders.Get("X-Default"))
	}
	if receivedHeaders.Get("X-Endpoint") != "endpoint-val" {
		t.Errorf("expected X-Endpoint=endpoint-val, got %s", receivedHeaders.Get("X-Endpoint"))
	}
	if receivedHeaders.Get("X-Route") != "route-val" {
		t.Errorf("expected X-Route=route-val, got %s", receivedHeaders.Get("X-Route"))
	}
	// Route override wins (applied last).
	if receivedHeaders.Get("X-Override") != "from-route" {
		t.Errorf("expected X-Override=from-route, got %s", receivedHeaders.Get("X-Override"))
	}
}

func TestAPIToolDispatcher_CacheBehavior(t *testing.T) {
	store := mock.NewMockStore()
	def := &model.APIToolDefinition{
		ID:       "cached-api",
		TenantID: "default",
		Name:     "Cached API",
		BaseURL:  "http://localhost:9999",
		Endpoints: []model.APIEndpoint{
			{Name: "ep1", Description: "test", Method: "GET", Path: "/test"},
		},
	}
	store.CreateAPITool(context.Background(), def)

	d := NewAPIToolDispatcher(store, slog.Default())

	// First call should fetch from store.
	def1, err := d.ResolveDefinition(context.Background(), "default", "cached-api")
	if err != nil {
		t.Fatalf("first resolve failed: %v", err)
	}

	// Delete from store.
	store.DeleteAPITool(context.Background(), "default", "cached-api")

	// Second call should hit cache (not store).
	def2, err := d.ResolveDefinition(context.Background(), "default", "cached-api")
	if err != nil {
		t.Fatalf("cached resolve failed: %v", err)
	}

	if def1 != def2 {
		t.Error("expected same pointer from cache")
	}
}

func TestAPIToolDispatcher_NotFound(t *testing.T) {
	store := mock.NewMockStore()
	d := NewAPIToolDispatcher(store, slog.Default())

	_, err := d.ResolveDefinition(context.Background(), "default", "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent API tool")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestAPIToolDispatcher_Timeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer ts.Close()

	store := mock.NewMockStore()
	def := newTestAPIToolDef(ts.URL)
	store.CreateAPITool(context.Background(), def)

	d := NewAPIToolDispatcher(store, slog.Default())
	timeout := 1
	result, err := d.CallEndpoint(context.Background(), "default",
		model.ToolRoute{
			APIToolID:      "test-api",
			APIEndpoint:    "get_customer",
			TimeoutSeconds: &timeout,
		},
		"get_customer",
		map[string]any{"customer_id": "slow"},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for timeout")
	}
}

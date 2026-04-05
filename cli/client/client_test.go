package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientGetGraph(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/graphs/graph_123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": "graph_123", "name": "test"})
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key")
	result, err := c.GetGraph(context.Background(), "graph_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var data map[string]string
	if err := json.Unmarshal(result, &data); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if data["id"] != "graph_123" {
		t.Errorf("expected id graph_123, got %s", data["id"])
	}
}

func TestClientListGraphs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("namespace") != "default" {
			t.Errorf("expected namespace=default, got %s", r.URL.Query().Get("namespace"))
		}
		if r.URL.Query().Get("limit") != "10" {
			t.Errorf("expected limit=10, got %s", r.URL.Query().Get("limit"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"items":    []map[string]string{{"id": "g1"}},
			"has_more": false,
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key")
	result, err := c.ListGraphs(context.Background(), "default", "", "", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(result.Items))
	}
}

func TestClientAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{
				"code":    "NOT_FOUND",
				"message": "graph not found",
			},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key")
	_, err := c.GetGraph(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", apiErr.StatusCode)
	}
	if apiErr.Code != "NOT_FOUND" {
		t.Errorf("expected code NOT_FOUND, got %s", apiErr.Code)
	}
}

func TestClientCreateGraph(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json content type")
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "test-graph" {
			t.Errorf("expected name test-graph, got %v", body["name"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": "graph_new", "name": "test-graph"})
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key")
	result, err := c.CreateGraph(context.Background(), map[string]string{"name": "test-graph"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var data map[string]string
	json.Unmarshal(result, &data)
	if data["id"] != "graph_new" {
		t.Errorf("expected id graph_new, got %s", data["id"])
	}
}

func TestClientDeleteGraph(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key")
	if err := c.DeleteGraph(context.Background(), "graph_123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientInvokeExecution(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var body InvokeRequest
		json.NewDecoder(r.Body).Decode(&body)
		if body.GraphID != "graph_123" {
			t.Errorf("expected graph_id graph_123, got %s", body.GraphID)
		}
		if !body.Debug {
			t.Error("expected debug=true in invoke request")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"execution_id": "exec_001", "status": "pending"})
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key")
	result, err := c.InvokeExecution(context.Background(), &InvokeRequest{
		GraphID: "graph_123",
		Input:   map[string]string{"text": "hello"},
		Mode:    "async",
		Debug:   true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var data map[string]string
	json.Unmarshal(result, &data)
	if data["status"] != "pending" {
		t.Errorf("expected status pending, got %s", data["status"])
	}
}

func TestClientNoAuthHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("expected no auth header")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	_, err := c.Health(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

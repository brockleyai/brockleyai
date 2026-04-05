package executor

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brockleyai/brockleyai/engine/mock"
	"github.com/brockleyai/brockleyai/internal/model"
)

func newToolTestDeps(secrets map[string]string) *ExecutorDeps {
	ss := mock.NewMockSecretStore()
	for k, v := range secrets {
		ss.Secrets[k] = v
	}
	return &ExecutorDeps{
		SecretStore:  ss,
		EventEmitter: &mock.MockEventEmitter{},
		Logger:       slog.Default(),
	}
}

func TestToolExecutor_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(body, &req)

		if req["method"] != "tools/call" {
			t.Errorf("expected method tools/call, got %v", req["method"])
		}

		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": "result data"},
				},
				"isError": false,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := model.ToolNodeConfig{
		ToolName: "my_tool",
		MCPURL:   server.URL,
	}

	node := &model.Node{
		ID:     "tool-1",
		Name:   "test-tool",
		Type:   model.NodeTypeTool,
		Config: mustJSON(cfg),
	}

	inputs := map[string]any{
		"query": "hello",
	}

	exec := &ToolExecutor{}
	deps := newToolTestDeps(nil)
	result, err := exec.Execute(context.Background(), node, inputs, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, ok := result.Outputs["result"]
	if !ok {
		t.Fatal("expected output port 'result'")
	}
	if content != "result data" {
		t.Errorf("expected 'result data', got %v", content)
	}
}

func TestToolExecutor_CustomHeaders(t *testing.T) {
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": "ok"},
				},
				"isError": false,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := model.ToolNodeConfig{
		ToolName: "my_tool",
		MCPURL:   server.URL,
		Headers: []model.HeaderConfig{
			{Name: "X-Static", Value: "static-value"},
			{Name: "X-Dynamic", FromInput: "api_key"},
			{Name: "Authorization", SecretRef: "my-secret"},
		},
	}

	node := &model.Node{
		ID:     "tool-2",
		Name:   "test-tool-headers",
		Type:   model.NodeTypeTool,
		Config: mustJSON(cfg),
	}

	inputs := map[string]any{
		"api_key": "dynamic-key-123",
	}

	exec := &ToolExecutor{}
	deps := newToolTestDeps(map[string]string{"my-secret": "Bearer secret-token"})
	_, err := exec.Execute(context.Background(), node, inputs, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := receivedHeaders.Get("X-Static"); got != "static-value" {
		t.Errorf("expected X-Static 'static-value', got %q", got)
	}
	if got := receivedHeaders.Get("X-Dynamic"); got != "dynamic-key-123" {
		t.Errorf("expected X-Dynamic 'dynamic-key-123', got %q", got)
	}
	if got := receivedHeaders.Get("Authorization"); got != "Bearer secret-token" {
		t.Errorf("expected Authorization 'Bearer secret-token', got %q", got)
	}
}

func TestToolExecutor_ToolError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": "something went wrong"},
				},
				"isError": true,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := model.ToolNodeConfig{
		ToolName: "failing_tool",
		MCPURL:   server.URL,
	}

	node := &model.Node{
		ID:     "tool-3",
		Name:   "test-tool-error",
		Type:   model.NodeTypeTool,
		Config: mustJSON(cfg),
	}

	exec := &ToolExecutor{}
	deps := newToolTestDeps(nil)
	_, err := exec.Execute(context.Background(), node, nil, nil, deps)
	if err == nil {
		t.Fatal("expected error from tool executor when tool returns error")
	}
}

func TestToolExecutor_TemplateHeaderValue(t *testing.T) {
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": "ok"},
				},
				"isError": false,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := model.ToolNodeConfig{
		ToolName: "my_tool",
		MCPURL:   server.URL,
		Headers: []model.HeaderConfig{
			{Name: "Authorization", Value: "Bearer {{input.token}}"},
		},
	}

	node := &model.Node{
		ID:     "tool-tmpl",
		Name:   "test-tool-template",
		Type:   model.NodeTypeTool,
		Config: mustJSON(cfg),
	}

	inputs := map[string]any{
		"token": "my-secret-token",
	}

	exec := &ToolExecutor{}
	deps := newToolTestDeps(nil)
	_, err := exec.Execute(context.Background(), node, inputs, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := receivedHeaders.Get("Authorization"); got != "Bearer my-secret-token" {
		t.Errorf("expected Authorization 'Bearer my-secret-token', got %q", got)
	}
}

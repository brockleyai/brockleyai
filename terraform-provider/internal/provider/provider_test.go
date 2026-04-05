package provider

import (
	"encoding/json"
	"testing"
)

// TestAccProtoV6ProviderFactories creates provider factories for acceptance testing.
// Usage: pass to resource.Test() in acceptance tests.
func TestAccProtoV6ProviderFactories(t *testing.T) {
	t.Helper()
	// Acceptance tests require a running server.
	// This is a placeholder verifying the provider can be instantiated.
	p := New("test")()
	if p == nil {
		t.Fatal("provider factory returned nil")
	}
}

func TestBuildGraphBody(t *testing.T) {
	plan := GraphResourceModel{}
	plan.Name = stringValue("test-graph")
	plan.Namespace = stringValue("default")
	plan.Description = stringValue("A test graph")
	plan.Nodes = stringValue(`[{"id":"input-1","name":"Input","type":"input"}]`)
	plan.Edges = stringValue(`[]`)

	body := buildGraphBody(plan)

	if body["name"] != "test-graph" {
		t.Errorf("expected name test-graph, got %v", body["name"])
	}
	if body["namespace"] != "default" {
		t.Errorf("expected namespace default, got %v", body["namespace"])
	}
	if body["description"] != "A test graph" {
		t.Errorf("expected description, got %v", body["description"])
	}
	if body["nodes"] == nil {
		t.Error("expected nodes to be set")
	}
}

func TestPopulateGraphModel(t *testing.T) {
	data := json.RawMessage(`{
		"id": "graph_123",
		"name": "test",
		"namespace": "default",
		"description": "desc",
		"status": "active",
		"version": 3,
		"nodes": [{"id": "n1"}],
		"edges": []
	}`)

	var model GraphResourceModel
	populateGraphModel(&model, data)

	if model.ID.ValueString() != "graph_123" {
		t.Errorf("expected id graph_123, got %s", model.ID.ValueString())
	}
	if model.Name.ValueString() != "test" {
		t.Errorf("expected name test, got %s", model.Name.ValueString())
	}
	if model.Version.ValueInt64() != 3 {
		t.Errorf("expected version 3, got %d", model.Version.ValueInt64())
	}
}

func TestBuildSchemaBody(t *testing.T) {
	plan := SchemaResourceModel{}
	plan.Name = stringValue("ticket-input")
	plan.JSONSchema = stringValue(`{"type":"object","properties":{"id":{"type":"string"}}}`)

	body := buildSchemaBody(plan)
	if body["name"] != "ticket-input" {
		t.Errorf("expected name ticket-input, got %v", body["name"])
	}
	if body["json_schema"] == nil {
		t.Error("expected json_schema to be set")
	}
}

func TestBuildProviderConfigBody(t *testing.T) {
	plan := ProviderConfigResourceModel{}
	plan.Name = stringValue("anthropic-primary")
	plan.Provider = stringValue("anthropic")
	plan.APIKeyRef = stringValue("anthropic-key")
	plan.DefaultModel = stringValue("claude-sonnet-4-20250514")

	body := buildProviderConfigBody(plan)
	if body["name"] != "anthropic-primary" {
		t.Errorf("expected name anthropic-primary, got %v", body["name"])
	}
	if body["provider"] != "anthropic" {
		t.Errorf("expected provider anthropic, got %v", body["provider"])
	}
	if body["default_model"] != "claude-sonnet-4-20250514" {
		t.Errorf("expected default_model, got %v", body["default_model"])
	}
}

func TestPopulateProviderConfigModel(t *testing.T) {
	data := json.RawMessage(`{
		"id": "pc_123",
		"name": "openai-primary",
		"namespace": "default",
		"provider": "openai",
		"api_key_ref": "openai-key",
		"default_model": "gpt-4o"
	}`)

	var model ProviderConfigResourceModel
	populateProviderConfigModel(&model, data)

	if model.ID.ValueString() != "pc_123" {
		t.Errorf("expected id pc_123, got %s", model.ID.ValueString())
	}
	if model.Provider.ValueString() != "openai" {
		t.Errorf("expected provider openai, got %s", model.Provider.ValueString())
	}
}

func TestSanitizeTerraformNameTF(t *testing.T) {
	// test the jsonStr helper
	data := json.RawMessage(`{"id":"test_id","name":"test_name"}`)
	if jsonStr(data, "id") != "test_id" {
		t.Errorf("expected test_id")
	}
}

package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/brockleyai/brockleyai/internal/model"
)

func getTestDB(t *testing.T) *PostgresStore {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping PostgreSQL integration tests")
	}
	store, err := New(context.Background(), url)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// cleanupTenant removes all data for a given tenant to ensure test isolation.
func cleanupTenant(t *testing.T, store *PostgresStore, tenantID string) {
	t.Helper()
	ctx := context.Background()
	// execution_steps doesn't have tenant_id — delete via execution_id join
	_, _ = store.pool.Exec(ctx, "DELETE FROM execution_steps WHERE execution_id IN (SELECT id FROM executions WHERE tenant_id = $1)", tenantID)
	tables := []string{"executions", "graphs", "schemas", "prompt_templates", "provider_configs"}
	for _, table := range tables {
		_, err := store.pool.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1", table), tenantID)
		if err != nil {
			t.Logf("warning: failed to clean up %s: %v", table, err)
		}
	}
}

func TestGraphCRUD(t *testing.T) {
	store := getTestDB(t)
	ctx := context.Background()
	tenantID := "test-graph-crud"
	t.Cleanup(func() { cleanupTenant(t, store, tenantID) })

	// Create
	graph := &model.Graph{
		ID:          "g-001",
		TenantID:    tenantID,
		Name:        "test-graph",
		Description: "A test graph",
		Namespace:   "default",
		Version:     1,
		Status:      model.GraphStatusDraft,
		Nodes: []model.Node{
			{ID: "n1", Name: "input", Type: "input"},
		},
		Edges: []model.Edge{
			{ID: "e1", SourceNodeID: "n1", SourcePort: "out", TargetNodeID: "n2", TargetPort: "in"},
		},
		State: &model.GraphState{
			Fields: []model.StateField{
				{Name: "counter", Schema: json.RawMessage(`{"type":"integer"}`), Reducer: model.ReducerReplace},
			},
		},
	}
	if err := store.CreateGraph(ctx, graph); err != nil {
		t.Fatalf("CreateGraph: %v", err)
	}

	// Get
	got, err := store.GetGraph(ctx, tenantID, "g-001")
	if err != nil {
		t.Fatalf("GetGraph: %v", err)
	}
	if got.Name != "test-graph" {
		t.Errorf("expected name 'test-graph', got %q", got.Name)
	}
	if len(got.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(got.Nodes))
	}
	if len(got.Edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(got.Edges))
	}
	if got.State == nil || len(got.State.Fields) != 1 {
		t.Errorf("expected 1 state field, got %v", got.State)
	}

	// List
	graphs, nextCursor, err := store.ListGraphs(ctx, tenantID, "default", "", 10)
	if err != nil {
		t.Fatalf("ListGraphs: %v", err)
	}
	if len(graphs) != 1 {
		t.Errorf("expected 1 graph, got %d", len(graphs))
	}
	if nextCursor != "" {
		t.Errorf("expected empty cursor, got %q", nextCursor)
	}

	// Update
	got.Name = "updated-graph"
	got.Status = model.GraphStatusActive
	if err := store.UpdateGraph(ctx, got); err != nil {
		t.Fatalf("UpdateGraph: %v", err)
	}
	updated, err := store.GetGraph(ctx, tenantID, "g-001")
	if err != nil {
		t.Fatalf("GetGraph after update: %v", err)
	}
	if updated.Name != "updated-graph" {
		t.Errorf("expected name 'updated-graph', got %q", updated.Name)
	}
	if updated.Status != model.GraphStatusActive {
		t.Errorf("expected status active, got %q", updated.Status)
	}

	// Delete (soft)
	if err := store.DeleteGraph(ctx, tenantID, "g-001"); err != nil {
		t.Fatalf("DeleteGraph: %v", err)
	}
	_, err = store.GetGraph(ctx, tenantID, "g-001")
	if err == nil {
		t.Error("expected error after soft delete, got nil")
	}

	// Tenant isolation
	graph2 := &model.Graph{
		ID:        "g-002",
		TenantID:  "other-tenant",
		Name:      "other-graph",
		Namespace: "default",
		Version:   1,
		Status:    model.GraphStatusDraft,
		Nodes:     []model.Node{},
		Edges:     []model.Edge{},
	}
	if err := store.CreateGraph(ctx, graph2); err != nil {
		t.Fatalf("CreateGraph other tenant: %v", err)
	}
	t.Cleanup(func() { cleanupTenant(t, store, "other-tenant") })

	_, err = store.GetGraph(ctx, tenantID, "g-002")
	if err == nil {
		t.Error("expected error getting graph from different tenant, got nil")
	}
}

func TestSchemaCRUD(t *testing.T) {
	store := getTestDB(t)
	ctx := context.Background()
	tenantID := "test-schema-crud"
	t.Cleanup(func() { cleanupTenant(t, store, tenantID) })

	schema := &model.SchemaLibrary{
		ID:          "s-001",
		TenantID:    tenantID,
		Name:        "test-schema",
		Namespace:   "default",
		Description: "A test schema",
		JSONSchema:  json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`),
	}
	if err := store.CreateSchema(ctx, schema); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	got, err := store.GetSchema(ctx, tenantID, "s-001")
	if err != nil {
		t.Fatalf("GetSchema: %v", err)
	}
	if got.Name != "test-schema" {
		t.Errorf("expected name 'test-schema', got %q", got.Name)
	}

	schemas, _, err := store.ListSchemas(ctx, tenantID, "default", "", 10)
	if err != nil {
		t.Fatalf("ListSchemas: %v", err)
	}
	if len(schemas) != 1 {
		t.Errorf("expected 1 schema, got %d", len(schemas))
	}

	got.Description = "updated description"
	if err := store.UpdateSchema(ctx, got); err != nil {
		t.Fatalf("UpdateSchema: %v", err)
	}

	if err := store.DeleteSchema(ctx, tenantID, "s-001"); err != nil {
		t.Fatalf("DeleteSchema: %v", err)
	}
	_, err = store.GetSchema(ctx, tenantID, "s-001")
	if err == nil {
		t.Error("expected error after soft delete")
	}
}

func TestPromptTemplateCRUD(t *testing.T) {
	store := getTestDB(t)
	ctx := context.Background()
	tenantID := "test-pt-crud"
	t.Cleanup(func() { cleanupTenant(t, store, tenantID) })

	pt := &model.PromptLibrary{
		ID:           "pt-001",
		TenantID:     tenantID,
		Name:         "test-prompt",
		Namespace:    "default",
		Description:  "A test prompt",
		SystemPrompt: "You are a helpful assistant.",
		UserPrompt:   "Hello {{name}}",
		Variables: []model.TemplateVar{
			{Name: "name", Schema: json.RawMessage(`{"type":"string"}`)},
		},
		ResponseFormat: model.ResponseFormatText,
	}
	if err := store.CreatePromptTemplate(ctx, pt); err != nil {
		t.Fatalf("CreatePromptTemplate: %v", err)
	}

	got, err := store.GetPromptTemplate(ctx, tenantID, "pt-001")
	if err != nil {
		t.Fatalf("GetPromptTemplate: %v", err)
	}
	if got.SystemPrompt != "You are a helpful assistant." {
		t.Errorf("expected system prompt, got %q", got.SystemPrompt)
	}
	if len(got.Variables) != 1 {
		t.Errorf("expected 1 variable, got %d", len(got.Variables))
	}

	templates, _, err := store.ListPromptTemplates(ctx, tenantID, "default", "", 10)
	if err != nil {
		t.Fatalf("ListPromptTemplates: %v", err)
	}
	if len(templates) != 1 {
		t.Errorf("expected 1 template, got %d", len(templates))
	}

	got.Description = "updated"
	if err := store.UpdatePromptTemplate(ctx, got); err != nil {
		t.Fatalf("UpdatePromptTemplate: %v", err)
	}

	if err := store.DeletePromptTemplate(ctx, tenantID, "pt-001"); err != nil {
		t.Fatalf("DeletePromptTemplate: %v", err)
	}
	_, err = store.GetPromptTemplate(ctx, tenantID, "pt-001")
	if err == nil {
		t.Error("expected error after soft delete")
	}
}

func TestProviderConfigCRUD(t *testing.T) {
	store := getTestDB(t)
	ctx := context.Background()
	tenantID := "test-pc-crud"
	t.Cleanup(func() { cleanupTenant(t, store, tenantID) })

	pc := &model.ProviderConfigLibrary{
		ID:           "pc-001",
		TenantID:     tenantID,
		Name:         "test-provider",
		Namespace:    "default",
		Provider:     model.ProviderOpenAI,
		BaseURL:      "https://api.openai.com",
		APIKeyRef:    "openai-key",
		DefaultModel: "gpt-4",
		ExtraHeaders: map[string]string{"X-Custom": "value"},
	}
	if err := store.CreateProviderConfig(ctx, pc); err != nil {
		t.Fatalf("CreateProviderConfig: %v", err)
	}

	got, err := store.GetProviderConfig(ctx, tenantID, "pc-001")
	if err != nil {
		t.Fatalf("GetProviderConfig: %v", err)
	}
	if got.Provider != model.ProviderOpenAI {
		t.Errorf("expected provider openai, got %q", got.Provider)
	}
	if got.ExtraHeaders["X-Custom"] != "value" {
		t.Errorf("expected extra header X-Custom=value, got %v", got.ExtraHeaders)
	}

	configs, _, err := store.ListProviderConfigs(ctx, tenantID, "default", "", 10)
	if err != nil {
		t.Fatalf("ListProviderConfigs: %v", err)
	}
	if len(configs) != 1 {
		t.Errorf("expected 1 config, got %d", len(configs))
	}

	got.DefaultModel = "gpt-4-turbo"
	if err := store.UpdateProviderConfig(ctx, got); err != nil {
		t.Fatalf("UpdateProviderConfig: %v", err)
	}

	if err := store.DeleteProviderConfig(ctx, tenantID, "pc-001"); err != nil {
		t.Fatalf("DeleteProviderConfig: %v", err)
	}
	_, err = store.GetProviderConfig(ctx, tenantID, "pc-001")
	if err == nil {
		t.Error("expected error after soft delete")
	}
}

func TestExecutionCRUD(t *testing.T) {
	store := getTestDB(t)
	ctx := context.Background()
	tenantID := "test-exec-crud"
	t.Cleanup(func() { cleanupTenant(t, store, tenantID) })

	exec := &model.Execution{
		ID:           "ex-001",
		TenantID:     tenantID,
		GraphID:      "g-001",
		GraphVersion: 1,
		Status:       model.ExecutionStatusPending,
		Input:        json.RawMessage(`{"key":"value"}`),
		Trigger:      model.TriggerAPI,
	}
	if err := store.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("CreateExecution: %v", err)
	}

	got, err := store.GetExecution(ctx, tenantID, "ex-001")
	if err != nil {
		t.Fatalf("GetExecution: %v", err)
	}
	if got.Status != model.ExecutionStatusPending {
		t.Errorf("expected status pending, got %q", got.Status)
	}

	// Update status
	got.Status = model.ExecutionStatusRunning
	if err := store.UpdateExecution(ctx, got); err != nil {
		t.Fatalf("UpdateExecution: %v", err)
	}
	updated, err := store.GetExecution(ctx, tenantID, "ex-001")
	if err != nil {
		t.Fatalf("GetExecution after update: %v", err)
	}
	if updated.Status != model.ExecutionStatusRunning {
		t.Errorf("expected status running, got %q", updated.Status)
	}

	// List with filters
	exec2 := &model.Execution{
		ID:           "ex-002",
		TenantID:     tenantID,
		GraphID:      "g-001",
		GraphVersion: 1,
		Status:       model.ExecutionStatusCompleted,
		Input:        json.RawMessage(`{}`),
		Trigger:      model.TriggerAPI,
	}
	if err := store.CreateExecution(ctx, exec2); err != nil {
		t.Fatalf("CreateExecution 2: %v", err)
	}

	// Filter by graph_id
	execs, _, err := store.ListExecutions(ctx, tenantID, "g-001", "", "", 10)
	if err != nil {
		t.Fatalf("ListExecutions by graph: %v", err)
	}
	if len(execs) != 2 {
		t.Errorf("expected 2 executions, got %d", len(execs))
	}

	// Filter by status
	execs, _, err = store.ListExecutions(ctx, tenantID, "", "completed", "", 10)
	if err != nil {
		t.Fatalf("ListExecutions by status: %v", err)
	}
	if len(execs) != 1 {
		t.Errorf("expected 1 completed execution, got %d", len(execs))
	}
}

func TestExecutionStepInsert(t *testing.T) {
	store := getTestDB(t)
	ctx := context.Background()
	tenantID := "test-step-insert"
	t.Cleanup(func() { cleanupTenant(t, store, tenantID) })

	// Create parent execution first.
	exec := &model.Execution{
		ID:           "ex-step-001",
		TenantID:     tenantID,
		GraphID:      "g-001",
		GraphVersion: 1,
		Status:       model.ExecutionStatusRunning,
		Input:        json.RawMessage(`{}`),
		Trigger:      model.TriggerAPI,
	}
	if err := store.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("CreateExecution: %v", err)
	}

	step1 := &model.ExecutionStep{
		ID:          "step-001",
		ExecutionID: "ex-step-001",
		NodeID:      "n1",
		NodeType:    "input",
		Iteration:   0,
		Status:      model.StepStatusCompleted,
		Input:       json.RawMessage(`{"in":1}`),
		Output:      json.RawMessage(`{"out":2}`),
		Attempt:     1,
	}
	step2 := &model.ExecutionStep{
		ID:          "step-002",
		ExecutionID: "ex-step-001",
		NodeID:      "n2",
		NodeType:    "llm",
		Iteration:   0,
		Status:      model.StepStatusCompleted,
		Attempt:     1,
		LLMUsage: &model.LLMUsage{
			Provider:         "openai",
			Model:            "gpt-4",
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
		LLMDebug: &model.LLMDebugTrace{
			Calls: []model.LLMCallTrace{
				{
					RequestID: "n2_llm",
					Provider:  "openai",
					Model:     "gpt-4",
					Request:   json.RawMessage(`{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`),
					Response:  json.RawMessage(`{"id":"resp-1","choices":[{"message":{"role":"assistant","content":"hi"}}]}`),
				},
			},
		},
	}
	if err := store.InsertExecutionStep(ctx, step1); err != nil {
		t.Fatalf("InsertExecutionStep 1: %v", err)
	}
	if err := store.InsertExecutionStep(ctx, step2); err != nil {
		t.Fatalf("InsertExecutionStep 2: %v", err)
	}

	steps, err := store.ListExecutionSteps(ctx, "ex-step-001")
	if err != nil {
		t.Fatalf("ListExecutionSteps: %v", err)
	}
	if len(steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(steps))
	}
	// Verify LLM usage on second step.
	if steps[1].LLMUsage == nil {
		t.Error("expected LLM usage on step 2")
	} else if steps[1].LLMUsage.TotalTokens != 150 {
		t.Errorf("expected total_tokens=150, got %d", steps[1].LLMUsage.TotalTokens)
	}
	if steps[1].LLMDebug == nil {
		t.Fatal("expected llm_debug on step 2")
	}
	if len(steps[1].LLMDebug.Calls) != 1 {
		t.Fatalf("expected 1 llm_debug call, got %d", len(steps[1].LLMDebug.Calls))
	}
	if steps[1].LLMDebug.Calls[0].RequestID != "n2_llm" {
		t.Errorf("expected request_id n2_llm, got %q", steps[1].LLMDebug.Calls[0].RequestID)
	}
}

func TestPagination(t *testing.T) {
	store := getTestDB(t)
	ctx := context.Background()
	tenantID := "test-pagination"
	t.Cleanup(func() { cleanupTenant(t, store, tenantID) })

	// Create 5 graphs with IDs that sort lexicographically: g-page-1 through g-page-5.
	for i := 1; i <= 5; i++ {
		g := &model.Graph{
			ID:        fmt.Sprintf("g-page-%d", i),
			TenantID:  tenantID,
			Name:      fmt.Sprintf("graph-%d", i),
			Namespace: "default",
			Version:   1,
			Status:    model.GraphStatusDraft,
			Nodes:     []model.Node{},
			Edges:     []model.Edge{},
		}
		if err := store.CreateGraph(ctx, g); err != nil {
			t.Fatalf("CreateGraph %d: %v", i, err)
		}
	}

	// Page 1: limit=2
	page1, cursor1, err := store.ListGraphs(ctx, tenantID, "default", "", 2)
	if err != nil {
		t.Fatalf("ListGraphs page 1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("expected 2 graphs on page 1, got %d", len(page1))
	}
	if cursor1 == "" {
		t.Fatal("expected non-empty cursor after page 1")
	}

	// Page 2: limit=2
	page2, cursor2, err := store.ListGraphs(ctx, tenantID, "default", cursor1, 2)
	if err != nil {
		t.Fatalf("ListGraphs page 2: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("expected 2 graphs on page 2, got %d", len(page2))
	}
	if cursor2 == "" {
		t.Fatal("expected non-empty cursor after page 2")
	}

	// Page 3: should have 1 remaining
	page3, cursor3, err := store.ListGraphs(ctx, tenantID, "default", cursor2, 2)
	if err != nil {
		t.Fatalf("ListGraphs page 3: %v", err)
	}
	if len(page3) != 1 {
		t.Errorf("expected 1 graph on page 3, got %d", len(page3))
	}
	if cursor3 != "" {
		t.Errorf("expected empty cursor on last page, got %q", cursor3)
	}

	// Verify no duplicates across pages.
	seen := map[string]bool{}
	for _, g := range append(append(page1, page2...), page3...) {
		if seen[g.ID] {
			t.Errorf("duplicate graph ID across pages: %s", g.ID)
		}
		seen[g.ID] = true
	}
	if len(seen) != 5 {
		t.Errorf("expected 5 unique graphs, got %d", len(seen))
	}
}

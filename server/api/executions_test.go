package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/brockleyai/brockleyai/engine/mock"
	"github.com/brockleyai/brockleyai/internal/model"
	"github.com/brockleyai/brockleyai/server/middleware"
)

// newTestExecutionHandler creates an ExecutionHandler backed by a MockStore for testing.
func newTestExecutionHandler() (*ExecutionHandler, *mock.MockStore) {
	store := mock.NewMockStore()
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	return NewExecutionHandler(store, nil, "", logger), store
}

// withTenantID wraps a request with the given tenant ID in the context.
func withTenantID(r *http.Request, tenantID string) *http.Request {
	ctx := middleware.SetTenantID(r.Context(), tenantID)
	return r.WithContext(ctx)
}

func TestGetSteps_EnforcesTenantIsolation(t *testing.T) {
	h, store := newTestExecutionHandler()

	// Create an execution belonging to tenant-a.
	exec := &model.Execution{
		ID:        "exec_123",
		TenantID:  "tenant-a",
		GraphID:   "graph_1",
		Status:    model.ExecutionStatusCompleted,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := store.CreateExecution(context.TODO(), exec); err != nil {
		t.Fatalf("failed to seed execution: %v", err)
	}

	// Insert a step for this execution.
	step := &model.ExecutionStep{
		ID:          "step_1",
		ExecutionID: "exec_123",
		NodeID:      "node_1",
		NodeType:    "llm",
		Status:      model.StepStatusCompleted,
	}
	if err := store.InsertExecutionStep(context.TODO(), step); err != nil {
		t.Fatalf("failed to seed step: %v", err)
	}

	// Request steps as tenant-a (owner) -- should succeed.
	t.Run("OwnerCanAccessSteps", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/exec_123/steps", nil)
		req.SetPathValue("id", "exec_123")
		req = withRequestID(req)
		req = withTenantID(req, "tenant-a")
		rr := httptest.NewRecorder()

		h.GetSteps(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var resp ListResponse[*model.ExecutionStep]
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp.Items) != 1 {
			t.Errorf("expected 1 step, got %d", len(resp.Items))
		}
	})

	// Request steps as tenant-b (different tenant) -- should get not found / error.
	t.Run("OtherTenantCannotAccessSteps", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/exec_123/steps", nil)
		req.SetPathValue("id", "exec_123")
		req = withRequestID(req)
		req = withTenantID(req, "tenant-b")
		rr := httptest.NewRecorder()

		h.GetSteps(rr, req)

		// The MockStore returns an error for not-found (tenant mismatch),
		// which the handler treats as 500 or 404 depending on implementation.
		// Either is acceptable -- the key is that it does NOT return 200 with steps.
		if rr.Code == http.StatusOK {
			t.Fatalf("expected non-200 status for cross-tenant access, got %d: %s", rr.Code, rr.Body.String())
		}
	})
}

func TestGetSteps_NonexistentExecution(t *testing.T) {
	h, _ := newTestExecutionHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/exec_nonexistent/steps", nil)
	req.SetPathValue("id", "exec_nonexistent")
	req = withRequestID(req)
	rr := httptest.NewRecorder()

	h.GetSteps(rr, req)

	// Should not return 200.
	if rr.Code == http.StatusOK {
		t.Fatalf("expected non-200 for nonexistent execution, got %d", rr.Code)
	}
}

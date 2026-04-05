package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupTestOrchestratorHandler(t *testing.T, timeout time.Duration) (*OrchestratorHandler, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	h := &OrchestratorHandler{
		rdb:          rdb,
		logger:       slog.Default(),
		brpopTimeout: timeout,
	}
	return h, mr
}

func TestWaitForResult_Success(t *testing.T) {
	h, _ := setupTestOrchestratorHandler(t, 2*time.Second)
	resultKey := "exec:test-1:results"

	// Push a result before waiting.
	expected := NodeTaskResult{
		RequestID: "req-1",
		NodeID:    "node-1",
		Status:    "completed",
		Outputs:   map[string]any{"response_text": "hello"},
		Attempt:   0,
	}
	resultJSON, _ := json.Marshal(expected)
	h.rdb.LPush(context.Background(), resultKey, string(resultJSON))

	result, err := h.waitForResult(context.Background(), resultKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NodeID != "node-1" {
		t.Errorf("node_id: expected %q, got %q", "node-1", result.NodeID)
	}
	if result.Status != "completed" {
		t.Errorf("status: expected %q, got %q", "completed", result.Status)
	}
}

func TestWaitForResult_TimeoutThenSuccess(t *testing.T) {
	h, _ := setupTestOrchestratorHandler(t, 200*time.Millisecond)
	resultKey := "exec:test-2:results"

	// Push a result after a delay (after the first BRPOP timeout).
	go func() {
		time.Sleep(300 * time.Millisecond)
		result := NodeTaskResult{
			RequestID: "req-2",
			NodeID:    "node-2",
			Status:    "completed",
			Attempt:   1,
		}
		resultJSON, _ := json.Marshal(result)
		h.rdb.LPush(context.Background(), resultKey, string(resultJSON))
	}()

	result, err := h.waitForResult(context.Background(), resultKey)
	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}
	if result.NodeID != "node-2" {
		t.Errorf("node_id: expected %q, got %q", "node-2", result.NodeID)
	}
}

func TestWaitForResult_AllRetriesExhausted(t *testing.T) {
	h, _ := setupTestOrchestratorHandler(t, 100*time.Millisecond)
	resultKey := "exec:test-3:results"

	// Never push a result — all BRPOP attempts should time out.
	_, err := h.waitForResult(context.Background(), resultKey)
	if err == nil {
		t.Fatal("expected error after all retries exhausted")
	}
}

func TestWaitForResult_ContextCancelled(t *testing.T) {
	// Use a short BRPOP timeout so the test doesn't block waiting for miniredis
	// (which doesn't propagate context cancellation for blocking commands).
	h, _ := setupTestOrchestratorHandler(t, 1*time.Second)
	resultKey := "exec:test-4:results"

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after a short delay.
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	_, err := h.waitForResult(ctx, resultKey)
	if err == nil {
		t.Fatal("expected error on context cancellation")
	}
	if ctx.Err() == nil {
		t.Error("expected context to be cancelled")
	}
}

func TestWaitForResult_FailedResult(t *testing.T) {
	h, _ := setupTestOrchestratorHandler(t, 2*time.Second)
	resultKey := "exec:test-5:results"

	// Push a failed result.
	expected := NodeTaskResult{
		RequestID: "req-5",
		NodeID:    "node-5",
		Status:    "failed",
		Error:     "provider error",
		Attempt:   2,
	}
	resultJSON, _ := json.Marshal(expected)
	h.rdb.LPush(context.Background(), resultKey, string(resultJSON))

	result, err := h.waitForResult(context.Background(), resultKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "failed" {
		t.Errorf("status: expected %q, got %q", "failed", result.Status)
	}
	if result.Error != "provider error" {
		t.Errorf("error: expected %q, got %q", "provider error", result.Error)
	}
	if result.Attempt != 2 {
		t.Errorf("attempt: expected 2, got %d", result.Attempt)
	}
}

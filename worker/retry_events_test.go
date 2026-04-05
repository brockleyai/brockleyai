package worker

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/redis/go-redis/v9"

	"github.com/brockleyai/brockleyai/internal/model"
)

// mockPublisher captures Publish calls for testing.
type mockPublisher struct {
	mu    sync.Mutex
	calls []publishCall
}

type publishCall struct {
	Channel string
	Message string
}

func (m *mockPublisher) Publish(_ context.Context, channel string, message interface{}) *redis.IntCmd {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, publishCall{Channel: channel, Message: message.(string)})
	cmd := redis.NewIntCmd(context.Background())
	cmd.SetVal(1)
	return cmd
}

func TestEmitRetryingEvent_PublishesCorrectEvent(t *testing.T) {
	pub := &mockPublisher{}

	emitRetryingEvent(pub, "exec-123", "node-abc", model.NodeTypeLLM, 2, "provider timeout")

	pub.mu.Lock()
	defer pub.mu.Unlock()

	if len(pub.calls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(pub.calls))
	}

	call := pub.calls[0]

	// Verify channel.
	expectedChannel := "execution:exec-123:events"
	if call.Channel != expectedChannel {
		t.Errorf("channel: expected %q, got %q", expectedChannel, call.Channel)
	}

	// Unmarshal and verify event fields.
	var event model.ExecutionEvent
	if err := json.Unmarshal([]byte(call.Message), &event); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}

	if event.Type != model.EventNodeRetrying {
		t.Errorf("event type: expected %q, got %q", model.EventNodeRetrying, event.Type)
	}
	if event.ExecutionID != "exec-123" {
		t.Errorf("execution_id: expected %q, got %q", "exec-123", event.ExecutionID)
	}
	if event.NodeID != "node-abc" {
		t.Errorf("node_id: expected %q, got %q", "node-abc", event.NodeID)
	}
	if event.NodeType != model.NodeTypeLLM {
		t.Errorf("node_type: expected %q, got %q", model.NodeTypeLLM, event.NodeType)
	}
	if event.Attempt != 2 {
		t.Errorf("attempt: expected 2, got %d", event.Attempt)
	}
	if event.Error == nil {
		t.Fatal("expected error to be set")
	}
	if event.Error.Code != "RETRY" {
		t.Errorf("error code: expected %q, got %q", "RETRY", event.Error.Code)
	}
	if event.Error.Message != "provider timeout" {
		t.Errorf("error message: expected %q, got %q", "provider timeout", event.Error.Message)
	}
	if event.Error.NodeID != "node-abc" {
		t.Errorf("error node_id: expected %q, got %q", "node-abc", event.Error.NodeID)
	}
	if event.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestEmitRetryingEvent_MCPToolNode(t *testing.T) {
	pub := &mockPublisher{}

	emitRetryingEvent(pub, "exec-456", "node-xyz", model.NodeTypeTool, 1, "connection refused")

	pub.mu.Lock()
	defer pub.mu.Unlock()

	if len(pub.calls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(pub.calls))
	}

	var event model.ExecutionEvent
	if err := json.Unmarshal([]byte(pub.calls[0].Message), &event); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}

	if event.NodeType != model.NodeTypeTool {
		t.Errorf("node_type: expected %q, got %q", model.NodeTypeTool, event.NodeType)
	}
	if event.Attempt != 1 {
		t.Errorf("attempt: expected 1, got %d", event.Attempt)
	}
}

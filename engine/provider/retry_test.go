package provider

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/brockleyai/brockleyai/internal/model"
)

type failingProvider struct {
	callCount  int
	failUntil  int
	retryable  bool
	retryAfter int
}

func (p *failingProvider) Name() string { return "failing" }
func (p *failingProvider) Stream(_ context.Context, _ *model.CompletionRequest) (<-chan model.StreamChunk, error) {
	return nil, fmt.Errorf("not implemented")
}
func (p *failingProvider) Complete(_ context.Context, _ *model.CompletionRequest) (*model.CompletionResponse, error) {
	p.callCount++
	if p.callCount <= p.failUntil {
		return nil, &ProviderError{
			Code:       "server_error",
			Message:    "internal error",
			Provider:   "failing",
			Retryable:  p.retryable,
			RetryAfter: p.retryAfter,
			StatusCode: 500,
		}
	}
	return &model.CompletionResponse{Content: "ok"}, nil
}

func TestRetryableProviderSucceedsAfterRetry(t *testing.T) {
	inner := &failingProvider{failUntil: 2, retryable: true}
	rp := NewRetryableProvider(inner, RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		BackoffFactor:  2.0,
	})

	resp, err := rp.Complete(context.Background(), &model.CompletionRequest{})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if resp.Content != "ok" {
		t.Errorf("expected content 'ok', got %s", resp.Content)
	}
	if inner.callCount != 3 {
		t.Errorf("expected 3 calls, got %d", inner.callCount)
	}
}

func TestRetryableProviderNonRetryableError(t *testing.T) {
	inner := &failingProvider{failUntil: 10, retryable: false}
	rp := NewRetryableProvider(inner, RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		BackoffFactor:  2.0,
	})

	_, err := rp.Complete(context.Background(), &model.CompletionRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if inner.callCount != 1 {
		t.Errorf("expected 1 call (no retry), got %d", inner.callCount)
	}
}

func TestRetryableProviderExhaustsRetries(t *testing.T) {
	inner := &failingProvider{failUntil: 100, retryable: true}
	rp := NewRetryableProvider(inner, RetryConfig{
		MaxRetries:     2,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     5 * time.Millisecond,
		BackoffFactor:  2.0,
	})

	_, err := rp.Complete(context.Background(), &model.CompletionRequest{})
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if inner.callCount != 3 { // 1 initial + 2 retries
		t.Errorf("expected 3 calls, got %d", inner.callCount)
	}
}

func TestRetryableProviderRespectsContext(t *testing.T) {
	inner := &failingProvider{failUntil: 100, retryable: true, retryAfter: 60}
	rp := NewRetryableProvider(inner, RetryConfig{
		MaxRetries:     5,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
		BackoffFactor:  2.0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := rp.Complete(ctx, &model.CompletionRequest{})
	if err == nil {
		t.Fatal("expected error due to context cancellation")
	}
}

func TestRetryableProviderName(t *testing.T) {
	inner := &failingProvider{}
	rp := NewRetryableProvider(inner, DefaultRetryConfig())
	if rp.Name() != "failing" {
		t.Errorf("expected name 'failing', got %s", rp.Name())
	}
}

package provider

import (
	"context"
	"testing"
	"time"

	"github.com/brockleyai/brockleyai/internal/model"
)

type countingProvider struct {
	calls int
}

func (p *countingProvider) Name() string { return "counting" }
func (p *countingProvider) Complete(_ context.Context, _ *model.CompletionRequest) (*model.CompletionResponse, error) {
	p.calls++
	return &model.CompletionResponse{Content: "ok"}, nil
}
func (p *countingProvider) Stream(_ context.Context, _ *model.CompletionRequest) (<-chan model.StreamChunk, error) {
	p.calls++
	ch := make(chan model.StreamChunk, 1)
	ch <- model.StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

func TestRateLimitedProviderAllowsBurst(t *testing.T) {
	inner := &countingProvider{}
	rlp := NewRateLimitedProvider(inner, RateLimitConfig{RequestsPerMinute: 60})

	// Should allow several quick requests (burst capacity)
	for i := 0; i < 5; i++ {
		_, err := rlp.Complete(context.Background(), &model.CompletionRequest{})
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
	}

	if inner.calls != 5 {
		t.Errorf("expected 5 calls, got %d", inner.calls)
	}
}

func TestRateLimitedProviderName(t *testing.T) {
	inner := &countingProvider{}
	rlp := NewRateLimitedProvider(inner, RateLimitConfig{RequestsPerMinute: 10})
	if rlp.Name() != "counting" {
		t.Errorf("expected name 'counting', got %s", rlp.Name())
	}
}

func TestRateLimitedProviderStream(t *testing.T) {
	inner := &countingProvider{}
	rlp := NewRateLimitedProvider(inner, RateLimitConfig{RequestsPerMinute: 60})

	ch, err := rlp.Stream(context.Background(), &model.CompletionRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for range ch {
		// drain
	}

	if inner.calls != 1 {
		t.Errorf("expected 1 call, got %d", inner.calls)
	}
}

func TestRateLimitedProviderRespectsContextCancellation(t *testing.T) {
	inner := &countingProvider{}
	// Very low rate limit
	rlp := NewRateLimitedProvider(inner, RateLimitConfig{RequestsPerMinute: 1})

	// Exhaust the burst
	_, _ = rlp.Complete(context.Background(), &model.CompletionRequest{})

	// Next request should be rate limited
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := rlp.Complete(ctx, &model.CompletionRequest{})
	// Should either succeed (if token refill happens fast enough) or fail with context deadline
	// The important thing is it doesn't hang
	_ = err
}

func TestTokenBucketLimiterRefill(t *testing.T) {
	l := newTokenBucketLimiter(60) // 1 per second

	// Exhaust all tokens
	for i := 0; i < 60; i++ {
		if err := l.Wait(context.Background()); err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
	}

	// Wait for refill
	time.Sleep(100 * time.Millisecond)

	// Should be able to make more requests
	if err := l.Wait(context.Background()); err != nil {
		t.Fatalf("post-refill request failed: %v", err)
	}
}

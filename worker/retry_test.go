package worker

import (
	"testing"
	"time"

	"github.com/brockleyai/brockleyai/internal/model"
)

func TestShouldRetry_NilPolicy(t *testing.T) {
	if shouldRetry(nil, 0) {
		t.Error("expected false for nil policy")
	}
}

func TestShouldRetry_ZeroMaxRetries(t *testing.T) {
	policy := &model.RetryPolicy{MaxRetries: 0}
	if shouldRetry(policy, 0) {
		t.Error("expected false for zero max retries")
	}
}

func TestShouldRetry_WithinLimit(t *testing.T) {
	policy := &model.RetryPolicy{MaxRetries: 3}
	if !shouldRetry(policy, 0) {
		t.Error("expected true for attempt 0 with max 3")
	}
	if !shouldRetry(policy, 2) {
		t.Error("expected true for attempt 2 with max 3")
	}
}

func TestShouldRetry_AtLimit(t *testing.T) {
	policy := &model.RetryPolicy{MaxRetries: 3}
	if shouldRetry(policy, 3) {
		t.Error("expected false for attempt 3 with max 3")
	}
}

func TestRetryDelay_FixedBackoff(t *testing.T) {
	policy := &model.RetryPolicy{
		MaxRetries:          3,
		BackoffStrategy:     "fixed",
		InitialDelaySeconds: 2.0,
	}

	delay0 := retryDelay(policy, 0)
	delay1 := retryDelay(policy, 1)
	delay2 := retryDelay(policy, 2)

	expected := 2 * time.Second
	if delay0 != expected {
		t.Errorf("attempt 0: expected %v, got %v", expected, delay0)
	}
	if delay1 != expected {
		t.Errorf("attempt 1: expected %v, got %v", expected, delay1)
	}
	if delay2 != expected {
		t.Errorf("attempt 2: expected %v, got %v", expected, delay2)
	}
}

func TestRetryDelay_ExponentialBackoff(t *testing.T) {
	policy := &model.RetryPolicy{
		MaxRetries:          5,
		BackoffStrategy:     "exponential",
		InitialDelaySeconds: 1.0,
		MaxDelaySeconds:     30.0,
	}

	delay0 := retryDelay(policy, 0) // 1 * 2^0 = 1s
	delay1 := retryDelay(policy, 1) // 1 * 2^1 = 2s
	delay2 := retryDelay(policy, 2) // 1 * 2^2 = 4s
	delay3 := retryDelay(policy, 3) // 1 * 2^3 = 8s

	if delay0 != 1*time.Second {
		t.Errorf("attempt 0: expected 1s, got %v", delay0)
	}
	if delay1 != 2*time.Second {
		t.Errorf("attempt 1: expected 2s, got %v", delay1)
	}
	if delay2 != 4*time.Second {
		t.Errorf("attempt 2: expected 4s, got %v", delay2)
	}
	if delay3 != 8*time.Second {
		t.Errorf("attempt 3: expected 8s, got %v", delay3)
	}
}

func TestRetryDelay_MaxDelayCap(t *testing.T) {
	policy := &model.RetryPolicy{
		MaxRetries:          10,
		BackoffStrategy:     "exponential",
		InitialDelaySeconds: 1.0,
		MaxDelaySeconds:     5.0,
	}

	delay5 := retryDelay(policy, 5) // 1 * 2^5 = 32, but capped at 5
	if delay5 != 5*time.Second {
		t.Errorf("attempt 5: expected 5s (capped), got %v", delay5)
	}
}

func TestRetryDelay_DefaultValues(t *testing.T) {
	policy := &model.RetryPolicy{MaxRetries: 3}
	delay := retryDelay(policy, 0)
	if delay != 1*time.Second {
		t.Errorf("default: expected 1s, got %v", delay)
	}
}

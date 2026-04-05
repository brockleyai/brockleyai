package provider

import (
	"context"
	"math"
	"math/rand/v2"
	"time"

	"github.com/brockleyai/brockleyai/internal/model"
)

// RetryConfig controls retry behavior for provider calls.
type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	BackoffFactor  float64
}

// DefaultRetryConfig returns sensible retry defaults for LLM provider calls.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
		BackoffFactor:  2.0,
	}
}

// RetryableProvider wraps an LLMProvider with automatic retry logic.
type RetryableProvider struct {
	inner  model.LLMProvider
	config RetryConfig
}

var _ model.LLMProvider = (*RetryableProvider)(nil)

// NewRetryableProvider wraps a provider with retry logic.
func NewRetryableProvider(inner model.LLMProvider, config RetryConfig) *RetryableProvider {
	return &RetryableProvider{
		inner:  inner,
		config: config,
	}
}

func (p *RetryableProvider) Name() string {
	return p.inner.Name()
}

func (p *RetryableProvider) Complete(ctx context.Context, req *model.CompletionRequest) (*model.CompletionResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		resp, err := p.inner.Complete(ctx, req)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Check if retryable
		pe, ok := err.(*ProviderError)
		if !ok || !pe.Retryable {
			return nil, err
		}

		// Don't retry if context is done
		if ctx.Err() != nil {
			return nil, lastErr
		}

		// Last attempt — don't wait
		if attempt == p.config.MaxRetries {
			break
		}

		// Calculate backoff
		backoff := p.calculateBackoff(attempt, pe.RetryAfter)

		select {
		case <-ctx.Done():
			return nil, lastErr
		case <-time.After(backoff):
		}
	}

	return nil, lastErr
}

func (p *RetryableProvider) Stream(ctx context.Context, req *model.CompletionRequest) (<-chan model.StreamChunk, error) {
	var lastErr error
	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		ch, err := p.inner.Stream(ctx, req)
		if err == nil {
			return ch, nil
		}

		lastErr = err

		pe, ok := err.(*ProviderError)
		if !ok || !pe.Retryable {
			return nil, err
		}

		if ctx.Err() != nil {
			return nil, lastErr
		}

		if attempt == p.config.MaxRetries {
			break
		}

		backoff := p.calculateBackoff(attempt, pe.RetryAfter)

		select {
		case <-ctx.Done():
			return nil, lastErr
		case <-time.After(backoff):
		}
	}

	return nil, lastErr
}

func (p *RetryableProvider) calculateBackoff(attempt, retryAfterSeconds int) time.Duration {
	if retryAfterSeconds > 0 {
		return time.Duration(retryAfterSeconds) * time.Second
	}

	backoff := float64(p.config.InitialBackoff) * math.Pow(p.config.BackoffFactor, float64(attempt))
	if backoff > float64(p.config.MaxBackoff) {
		backoff = float64(p.config.MaxBackoff)
	}

	// Add jitter (10-30%)
	jitter := 1.0 + (rand.Float64()*0.2 + 0.1)
	backoff *= jitter

	return time.Duration(backoff)
}

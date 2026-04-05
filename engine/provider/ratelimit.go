package provider

import (
	"context"
	"sync"
	"time"

	"github.com/brockleyai/brockleyai/internal/model"
)

// RateLimitConfig defines rate limits for a provider.
type RateLimitConfig struct {
	RequestsPerMinute int
	TokensPerMinute   int
}

// RateLimitedProvider wraps an LLMProvider with rate limiting.
type RateLimitedProvider struct {
	inner   model.LLMProvider
	limiter *tokenBucketLimiter
}

var _ model.LLMProvider = (*RateLimitedProvider)(nil)

// NewRateLimitedProvider wraps a provider with rate limiting.
func NewRateLimitedProvider(inner model.LLMProvider, config RateLimitConfig) *RateLimitedProvider {
	return &RateLimitedProvider{
		inner:   inner,
		limiter: newTokenBucketLimiter(config.RequestsPerMinute),
	}
}

func (p *RateLimitedProvider) Name() string {
	return p.inner.Name()
}

func (p *RateLimitedProvider) Complete(ctx context.Context, req *model.CompletionRequest) (*model.CompletionResponse, error) {
	if err := p.limiter.Wait(ctx); err != nil {
		return nil, &ProviderError{
			Code:       "rate_limited",
			Message:    "client-side rate limit exceeded",
			Provider:   p.inner.Name(),
			Retryable:  true,
			RetryAfter: 1,
		}
	}
	return p.inner.Complete(ctx, req)
}

func (p *RateLimitedProvider) Stream(ctx context.Context, req *model.CompletionRequest) (<-chan model.StreamChunk, error) {
	if err := p.limiter.Wait(ctx); err != nil {
		return nil, &ProviderError{
			Code:       "rate_limited",
			Message:    "client-side rate limit exceeded",
			Provider:   p.inner.Name(),
			Retryable:  true,
			RetryAfter: 1,
		}
	}
	return p.inner.Stream(ctx, req)
}

// tokenBucketLimiter implements a simple token bucket rate limiter.
type tokenBucketLimiter struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

func newTokenBucketLimiter(requestsPerMinute int) *tokenBucketLimiter {
	if requestsPerMinute <= 0 {
		requestsPerMinute = 1000 // effectively unlimited
	}
	maxTokens := float64(requestsPerMinute)
	return &tokenBucketLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: float64(requestsPerMinute) / 60.0,
		lastRefill: time.Now(),
	}
}

func (l *tokenBucketLimiter) Wait(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(l.lastRefill).Seconds()
	l.tokens += elapsed * l.refillRate
	if l.tokens > l.maxTokens {
		l.tokens = l.maxTokens
	}
	l.lastRefill = now

	if l.tokens >= 1 {
		l.tokens--
		return nil
	}

	// Wait for a token to become available
	waitTime := time.Duration((1.0 - l.tokens) / l.refillRate * float64(time.Second))
	l.tokens = 0

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(waitTime):
		return nil
	}
}

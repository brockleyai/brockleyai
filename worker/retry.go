package worker

import (
	"math"
	"time"

	"github.com/brockleyai/brockleyai/internal/model"
)

// shouldRetry checks if a failed task should be retried based on its retry policy.
func shouldRetry(policy *model.RetryPolicy, attempt int) bool {
	if policy == nil || policy.MaxRetries <= 0 {
		return false
	}
	return attempt < policy.MaxRetries
}

// retryDelay calculates the delay before the next retry attempt.
func retryDelay(policy *model.RetryPolicy, attempt int) time.Duration {
	if policy == nil {
		return time.Second
	}

	initialDelay := policy.InitialDelaySeconds
	if initialDelay <= 0 {
		initialDelay = 1.0
	}
	maxDelay := policy.MaxDelaySeconds
	if maxDelay <= 0 {
		maxDelay = 60.0
	}

	var delaySec float64
	switch policy.BackoffStrategy {
	case "exponential":
		delaySec = initialDelay * math.Pow(2, float64(attempt))
	default: // "fixed" or unset
		delaySec = initialDelay
	}

	if delaySec > maxDelay {
		delaySec = maxDelay
	}

	return time.Duration(delaySec * float64(time.Second))
}

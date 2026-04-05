package provider

import (
	"fmt"
	"net/http"
	"strconv"
)

// ProviderError is a structured error from an LLM provider.
type ProviderError struct {
	Code       string // "rate_limited", "auth_failed", "model_not_found", "context_length", "content_filtered", "server_error"
	Message    string
	Provider   string
	Retryable  bool
	RetryAfter int // seconds to wait before retry (if retryable)
	StatusCode int
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("%s: %s (code: %s, status: %d, retryable: %v)", e.Provider, e.Message, e.Code, e.StatusCode, e.Retryable)
}

// classifyHTTPError classifies an HTTP error response into a ProviderError.
func classifyHTTPError(providerName string, statusCode int, message, errType, errCode string, headers http.Header) *ProviderError {
	pe := &ProviderError{
		Provider:   providerName,
		Message:    message,
		StatusCode: statusCode,
	}

	switch statusCode {
	case http.StatusTooManyRequests:
		pe.Code = "rate_limited"
		pe.Retryable = true
		if ra := headers.Get("Retry-After"); ra != "" {
			if sec, err := strconv.Atoi(ra); err == nil {
				pe.RetryAfter = sec
			}
		}
		if pe.RetryAfter == 0 {
			pe.RetryAfter = 5 // default 5 seconds
		}

	case http.StatusUnauthorized, http.StatusForbidden:
		pe.Code = "auth_failed"
		pe.Retryable = false

	case http.StatusBadRequest:
		pe.Code = classifyBadRequest(errType, errCode, message)
		pe.Retryable = false

	case http.StatusNotFound:
		pe.Code = "model_not_found"
		pe.Retryable = false

	case http.StatusRequestEntityTooLarge:
		pe.Code = "context_length"
		pe.Retryable = false

	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		pe.Code = "server_error"
		pe.Retryable = true
		pe.RetryAfter = 2

	default:
		pe.Code = "unknown_error"
		pe.Retryable = statusCode >= 500
	}

	return pe
}

func classifyBadRequest(errType, errCode, message string) string {
	// Check for context length errors (different providers report differently)
	for _, keyword := range []string{"context_length", "maximum context", "token limit", "too many tokens", "max_tokens"} {
		if containsCI(message, keyword) || containsCI(errCode, keyword) {
			return "context_length"
		}
	}
	// Check for content filter
	for _, keyword := range []string{"content_filter", "content_policy", "safety", "moderation"} {
		if containsCI(message, keyword) || containsCI(errType, keyword) {
			return "content_filtered"
		}
	}
	return "invalid_request"
}

func containsCI(s, sub string) bool {
	if len(s) < len(sub) {
		return false
	}
	sl := toLower(s)
	subl := toLower(sub)
	for i := 0; i <= len(sl)-len(subl); i++ {
		if sl[i:i+len(subl)] == subl {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

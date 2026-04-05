package provider

import (
	"net/http"
	"testing"
)

func TestClassifyHTTPError(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		message   string
		errType   string
		errCode   string
		wantCode  string
		wantRetry bool
	}{
		{
			name:      "rate limited",
			status:    429,
			message:   "Rate limit exceeded",
			wantCode:  "rate_limited",
			wantRetry: true,
		},
		{
			name:      "unauthorized",
			status:    401,
			message:   "Invalid API key",
			wantCode:  "auth_failed",
			wantRetry: false,
		},
		{
			name:      "forbidden",
			status:    403,
			message:   "Forbidden",
			wantCode:  "auth_failed",
			wantRetry: false,
		},
		{
			name:      "not found",
			status:    404,
			message:   "Model not found",
			wantCode:  "model_not_found",
			wantRetry: false,
		},
		{
			name:      "server error",
			status:    500,
			message:   "Internal server error",
			wantCode:  "server_error",
			wantRetry: true,
		},
		{
			name:      "bad gateway",
			status:    502,
			message:   "Bad Gateway",
			wantCode:  "server_error",
			wantRetry: true,
		},
		{
			name:      "service unavailable",
			status:    503,
			message:   "Service Unavailable",
			wantCode:  "server_error",
			wantRetry: true,
		},
		{
			name:      "context length error",
			status:    400,
			message:   "maximum context length exceeded",
			wantCode:  "context_length",
			wantRetry: false,
		},
		{
			name:      "content filter",
			status:    400,
			message:   "content_policy_violation",
			errType:   "content_filter",
			wantCode:  "content_filtered",
			wantRetry: false,
		},
		{
			name:      "generic bad request",
			status:    400,
			message:   "Invalid parameter",
			wantCode:  "invalid_request",
			wantRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pe := classifyHTTPError("test", tt.status, tt.message, tt.errType, tt.errCode, http.Header{})
			if pe.Code != tt.wantCode {
				t.Errorf("expected code %q, got %q", tt.wantCode, pe.Code)
			}
			if pe.Retryable != tt.wantRetry {
				t.Errorf("expected retryable=%v, got %v", tt.wantRetry, pe.Retryable)
			}
		})
	}
}

func TestClassifyHTTPErrorRetryAfterHeader(t *testing.T) {
	headers := http.Header{}
	headers.Set("Retry-After", "10")

	pe := classifyHTTPError("test", 429, "rate limited", "", "", headers)
	if pe.RetryAfter != 10 {
		t.Errorf("expected retry_after=10, got %d", pe.RetryAfter)
	}
}

func TestProviderErrorMessage(t *testing.T) {
	pe := &ProviderError{
		Code:       "rate_limited",
		Message:    "Too many requests",
		Provider:   "openai",
		Retryable:  true,
		StatusCode: 429,
	}
	msg := pe.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestContainsCI(t *testing.T) {
	tests := []struct {
		s, sub string
		want   bool
	}{
		{"Hello World", "hello", true},
		{"Hello World", "WORLD", true},
		{"foo", "foobar", false},
		{"", "a", false},
		{"abc", "", true},
	}

	for _, tt := range tests {
		got := containsCI(tt.s, tt.sub)
		if got != tt.want {
			t.Errorf("containsCI(%q, %q) = %v, want %v", tt.s, tt.sub, got, tt.want)
		}
	}
}

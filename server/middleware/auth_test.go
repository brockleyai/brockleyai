package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConstantTimeKeyMatch(t *testing.T) {
	validKeys := []string{"key-alpha", "key-beta", "key-gamma"}

	tests := []struct {
		name      string
		candidate string
		want      bool
	}{
		{"ExactMatchFirst", "key-alpha", true},
		{"ExactMatchMiddle", "key-beta", true},
		{"ExactMatchLast", "key-gamma", true},
		{"NoMatch", "key-delta", false},
		{"EmptyCandidate", "", false},
		{"PartialMatch", "key-alph", false},
		{"SupersetMatch", "key-alpha-extra", false},
		{"CaseSensitive", "Key-Alpha", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := constantTimeKeyMatch(tt.candidate, validKeys)
			if got != tt.want {
				t.Errorf("constantTimeKeyMatch(%q, keys) = %v, want %v", tt.candidate, got, tt.want)
			}
		})
	}
}

func TestConstantTimeKeyMatch_EmptyKeyList(t *testing.T) {
	if constantTimeKeyMatch("any-key", nil) {
		t.Error("expected no match against empty key list")
	}
	if constantTimeKeyMatch("any-key", []string{}) {
		t.Error("expected no match against empty key list")
	}
}

func TestAuth_MissingAuthHeader(t *testing.T) {
	handler := Auth([]string{"valid-key"}, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/graphs", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestAuth_InvalidKey(t *testing.T) {
	handler := Auth([]string{"valid-key"}, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/graphs", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestAuth_ValidKey(t *testing.T) {
	called := false
	handler := Auth([]string{"valid-key"}, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/graphs", nil)
	req.Header.Set("Authorization", "Bearer valid-key")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if !called {
		t.Error("expected handler to be called")
	}
}

func TestAuth_DevModeSkipsAuth(t *testing.T) {
	called := false
	handler := Auth([]string{"valid-key"}, true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/graphs", nil)
	// No Authorization header.
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if !called {
		t.Error("expected handler to be called in dev mode")
	}
}

func TestAuth_HealthEndpointsSkipAuth(t *testing.T) {
	paths := []string{"/health", "/health/ready", "/version"}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			called := false
			handler := Auth([]string{"valid-key"}, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, path, nil)
			// No Authorization header.
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("expected status %d for %s, got %d", http.StatusOK, path, rr.Code)
			}
			if !called {
				t.Errorf("expected handler to be called for health endpoint %s", path)
			}
		})
	}
}

func TestAuth_NoKeysConfigured(t *testing.T) {
	called := false
	handler := Auth(nil, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/graphs", nil)
	// No Authorization header and no keys configured.
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if !called {
		t.Error("expected handler to be called when no keys are configured")
	}
}

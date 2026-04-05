package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// Auth validates API key authentication.
// In development mode (isDev=true), auth is skipped entirely.
// Keys are compared against the provided validKeys list using constant-time
// comparison to prevent timing attacks.
func Auth(validKeys []string, isDev bool) func(http.Handler) http.Handler {
	// Store trimmed keys in a slice for constant-time comparison.
	var keys []string
	for _, k := range validKeys {
		k = strings.TrimSpace(k)
		if k != "" {
			keys = append(keys, k)
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth in development mode
			if isDev {
				next.ServeHTTP(w, r)
				return
			}

			// Skip auth for health endpoints
			if r.URL.Path == "/health" || r.URL.Path == "/health/ready" || r.URL.Path == "/version" || r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			// No keys configured = no auth enforced
			if len(keys) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			auth := r.Header.Get("Authorization")
			if auth == "" {
				http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"missing Authorization header"}}`, http.StatusUnauthorized)
				return
			}

			key := strings.TrimPrefix(auth, "Bearer ")
			if key == auth {
				// No "Bearer " prefix
				http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"invalid Authorization format, expected: Bearer <key>"}}`, http.StatusUnauthorized)
				return
			}

			if !constantTimeKeyMatch(key, keys) {
				http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"invalid API key"}}`, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// constantTimeKeyMatch checks whether candidateKey matches any of the valid
// keys using constant-time comparison. All keys are always compared to prevent
// leaking which key (or how many keys) are configured via timing side-channels.
func constantTimeKeyMatch(candidateKey string, validKeys []string) bool {
	match := 0
	for _, k := range validKeys {
		match |= subtle.ConstantTimeCompare([]byte(candidateKey), []byte(k))
	}
	return match == 1
}

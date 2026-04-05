// Package api implements the HTTP handlers for the Brockley REST API.
package api

import (
	"encoding/json"
	"net/http"
)

// APIError is the standard error response format.
type APIError struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody contains the error details.
type ErrorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// Standard error codes.
const (
	ErrCodeValidation      = "VALIDATION_ERROR"
	ErrCodeSchemaViolation = "SCHEMA_VIOLATION"
	ErrCodeUnauthorized    = "UNAUTHORIZED"
	ErrCodeForbidden       = "FORBIDDEN"
	ErrCodeNotFound        = "NOT_FOUND"
	ErrCodeConflict        = "CONFLICT"
	ErrCodeGraphInvalid    = "GRAPH_INVALID"
	ErrCodeRateLimited     = "RATE_LIMITED"
	ErrCodeInternalError   = "INTERNAL_ERROR"
	ErrCodeServiceUnavail  = "SERVICE_UNAVAILABLE"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string, requestID string) {
	writeJSON(w, status, APIError{
		Error: ErrorBody{
			Code:      code,
			Message:   message,
			RequestID: requestID,
		},
	})
}

package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthReady_ProductionRedactsErrors(t *testing.T) {
	dbErr := errors.New("connection refused: dial tcp 10.0.1.5:5432: connect timeout")
	h := NewHealthHandler(func() error { return dbErr }, nil, false)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rr := httptest.NewRecorder()

	h.Ready(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	dbStatus := resp.Components["database"]
	if dbStatus != "unhealthy" {
		t.Errorf("expected production DB status to be %q, got %q", "unhealthy", dbStatus)
	}
	if strings.Contains(dbStatus, "10.0.1.5") {
		t.Error("production health response should not contain internal IP address")
	}
	if strings.Contains(dbStatus, "connection refused") {
		t.Error("production health response should not contain raw error details")
	}
}

func TestHealthReady_DevModeShowsErrors(t *testing.T) {
	dbErr := errors.New("connection refused: dial tcp 10.0.1.5:5432: connect timeout")
	h := NewHealthHandler(func() error { return dbErr }, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rr := httptest.NewRecorder()

	h.Ready(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	dbStatus := resp.Components["database"]
	if !strings.Contains(dbStatus, "connection refused") {
		t.Errorf("expected dev mode DB status to contain raw error, got %q", dbStatus)
	}
}

func TestHealthReady_RedisErrorRedactedInProduction(t *testing.T) {
	redisErr := errors.New("dial tcp 10.0.2.10:6379: i/o timeout")
	h := NewHealthHandler(nil, func() error { return redisErr }, false)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rr := httptest.NewRecorder()

	h.Ready(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	redisStatus := resp.Components["redis"]
	if redisStatus != "unhealthy" {
		t.Errorf("expected production Redis status to be %q, got %q", "unhealthy", redisStatus)
	}
	if strings.Contains(redisStatus, "10.0.2.10") {
		t.Error("production health response should not contain internal Redis IP")
	}
}

func TestHealthReady_HealthyResponse(t *testing.T) {
	h := NewHealthHandler(func() error { return nil }, func() error { return nil }, false)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rr := httptest.NewRecorder()

	h.Ready(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "ready" {
		t.Errorf("expected status %q, got %q", "ready", resp.Status)
	}
	if resp.Components["database"] != "healthy" {
		t.Errorf("expected database %q, got %q", "healthy", resp.Components["database"])
	}
	if resp.Components["redis"] != "healthy" {
		t.Errorf("expected redis %q, got %q", "healthy", resp.Components["redis"])
	}
}

func TestHealth_AlwaysReturns200(t *testing.T) {
	h := NewHealthHandler(nil, nil, false)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	h.Health(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

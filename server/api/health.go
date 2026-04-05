package api

import "net/http"

// HealthHandler handles health and version endpoints.
type HealthHandler struct {
	CheckDB    func() error
	CheckRedis func() error
	isDev      bool
}

// NewHealthHandler creates a new HealthHandler.
// When isDev is false (production), raw error details from health checks are
// redacted to avoid leaking internal infrastructure information.
func NewHealthHandler(checkDB, checkRedis func() error, isDev bool) *HealthHandler {
	return &HealthHandler{
		CheckDB:    checkDB,
		CheckRedis: checkRedis,
		isDev:      isDev,
	}
}

// Health handles GET /health. Always returns 200.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "healthy",
		"version": "0.1.0",
	})
}

// Ready handles GET /health/ready. Checks DB and Redis connectivity.
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	components := map[string]string{}
	healthy := true

	if h.CheckDB != nil {
		if err := h.CheckDB(); err != nil {
			if h.isDev {
				components["database"] = "unhealthy: " + err.Error()
			} else {
				components["database"] = "unhealthy"
			}
			healthy = false
		} else {
			components["database"] = "healthy"
		}
	} else {
		components["database"] = "not configured"
	}

	if h.CheckRedis != nil {
		if err := h.CheckRedis(); err != nil {
			if h.isDev {
				components["redis"] = "unhealthy: " + err.Error()
			} else {
				components["redis"] = "unhealthy"
			}
			healthy = false
		} else {
			components["redis"] = "healthy"
		}
	} else {
		components["redis"] = "not configured"
	}

	status := "ready"
	statusCode := http.StatusOK
	if !healthy {
		status = "not ready"
		statusCode = http.StatusServiceUnavailable
	}

	writeJSON(w, statusCode, HealthResponse{
		Status:     status,
		Version:    "0.1.0",
		Components: components,
	})
}

// Version handles GET /version.
func (h *HealthHandler) Version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, VersionResponse{
		Name:       "brockley",
		Version:    "0.1.0",
		APIVersion: "v1",
	})
}

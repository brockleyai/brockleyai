package api

import (
	"log/slog"
	"net/http"

	"github.com/brockleyai/brockleyai/internal/model"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewRouter creates the main HTTP router with all API routes registered.
// It uses Go 1.22's enhanced net/http.ServeMux with method-based routing.
func NewRouter(store model.Store, logger *slog.Logger, checkDB func() error, checkRedis func() error, metricsEnabled bool, queue model.TaskQueue, redisAddr string, isDev bool) http.Handler {
	mux := http.NewServeMux()

	// Health and version endpoints (no auth required).
	health := NewHealthHandler(checkDB, checkRedis, isDev)
	mux.HandleFunc("GET /health", health.Health)
	mux.HandleFunc("GET /health/ready", health.Ready)
	mux.HandleFunc("GET /version", health.Version)

	// Graph endpoints.
	graphs := NewGraphHandler(store, logger)
	mux.HandleFunc("POST /api/v1/graphs", graphs.Create)
	mux.HandleFunc("GET /api/v1/graphs", graphs.List)
	mux.HandleFunc("GET /api/v1/graphs/{id}", graphs.Get)
	mux.HandleFunc("PUT /api/v1/graphs/{id}", graphs.Update)
	mux.HandleFunc("DELETE /api/v1/graphs/{id}", graphs.Delete)
	mux.HandleFunc("POST /api/v1/graphs/{id}/validate", graphs.Validate)

	// Schema endpoints.
	schemas := NewSchemaHandler(store, logger)
	mux.HandleFunc("POST /api/v1/schemas", schemas.Create)
	mux.HandleFunc("GET /api/v1/schemas", schemas.List)
	mux.HandleFunc("GET /api/v1/schemas/{id}", schemas.Get)
	mux.HandleFunc("PUT /api/v1/schemas/{id}", schemas.Update)
	mux.HandleFunc("DELETE /api/v1/schemas/{id}", schemas.Delete)

	// Prompt template endpoints.
	prompts := NewPromptTemplateHandler(store, logger)
	mux.HandleFunc("POST /api/v1/prompt-templates", prompts.Create)
	mux.HandleFunc("GET /api/v1/prompt-templates", prompts.List)
	mux.HandleFunc("GET /api/v1/prompt-templates/{id}", prompts.Get)
	mux.HandleFunc("PUT /api/v1/prompt-templates/{id}", prompts.Update)
	mux.HandleFunc("DELETE /api/v1/prompt-templates/{id}", prompts.Delete)

	// Provider config endpoints.
	providers := NewProviderConfigHandler(store, logger)
	mux.HandleFunc("POST /api/v1/provider-configs", providers.Create)
	mux.HandleFunc("GET /api/v1/provider-configs", providers.List)
	mux.HandleFunc("GET /api/v1/provider-configs/{id}", providers.Get)
	mux.HandleFunc("PUT /api/v1/provider-configs/{id}", providers.Update)
	mux.HandleFunc("DELETE /api/v1/provider-configs/{id}", providers.Delete)

	// API tool definition endpoints.
	apiTools := NewAPIToolHandler(store, logger)
	mux.HandleFunc("POST /api/v1/api-tools", apiTools.Create)
	mux.HandleFunc("GET /api/v1/api-tools", apiTools.List)
	mux.HandleFunc("GET /api/v1/api-tools/{id}", apiTools.Get)
	mux.HandleFunc("PUT /api/v1/api-tools/{id}", apiTools.Update)
	mux.HandleFunc("DELETE /api/v1/api-tools/{id}", apiTools.Delete)
	mux.HandleFunc("POST /api/v1/api-tools/{id}/test", apiTools.Test)

	// Execution endpoints.
	exec := NewExecutionHandler(store, queue, redisAddr, logger)
	mux.HandleFunc("POST /api/v1/executions", exec.Invoke)
	mux.HandleFunc("GET /api/v1/executions", exec.List)
	mux.HandleFunc("GET /api/v1/executions/{id}", exec.Get)
	mux.HandleFunc("GET /api/v1/executions/{id}/steps", exec.GetSteps)
	mux.HandleFunc("POST /api/v1/executions/{id}/cancel", exec.Cancel)
	mux.HandleFunc("GET /api/v1/executions/{id}/stream", exec.Stream)

	// Metrics endpoint (opt-in via config).
	if metricsEnabled {
		mux.Handle("GET /metrics", promhttp.Handler())
	}

	return mux
}

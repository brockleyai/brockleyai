package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/brockleyai/brockleyai/engine/mock"
	"github.com/brockleyai/brockleyai/internal/model"
	"github.com/brockleyai/brockleyai/server/api"
	"github.com/brockleyai/brockleyai/server/middleware"
	"github.com/brockleyai/brockleyai/server/store/postgres"
	"github.com/brockleyai/brockleyai/worker"
)

func main() {
	// Load configuration from environment variables.
	env := getEnv("BROCKLEY_ENV", "production")
	host := getEnv("BROCKLEY_HOST", "0.0.0.0")
	port := getEnv("BROCKLEY_PORT", "8000")
	apiKeysRaw := getEnv("BROCKLEY_API_KEYS", "")
	corsOrigins := getEnv("BROCKLEY_CORS_ORIGINS", "*")
	logLevel := getEnv("BROCKLEY_LOG_LEVEL", "info")
	logFormat := getEnv("BROCKLEY_LOG_FORMAT", "json")
	databaseURL := os.Getenv("DATABASE_URL")
	redisURL := os.Getenv("REDIS_URL")
	metricsEnabled := getEnv("BROCKLEY_METRICS_ENABLED", "false") == "true"

	isDev := env == "development"

	// Set up structured logger.
	logger := setupLogger(logLevel, logFormat, isDev)

	// Parse API keys.
	var apiKeys []string
	if apiKeysRaw != "" {
		for _, k := range strings.Split(apiKeysRaw, ",") {
			k = strings.TrimSpace(k)
			if k != "" {
				apiKeys = append(apiKeys, k)
			}
		}
	}

	// Set up store: PostgreSQL if DATABASE_URL is set, otherwise in-memory MockStore.
	var store model.Store
	var checkDB func() error

	if databaseURL != "" {
		pgStore, err := postgres.New(context.Background(), databaseURL)
		if err != nil {
			logger.Error("failed to connect to PostgreSQL", "error", err)
			os.Exit(1)
		}
		defer pgStore.Close()
		store = pgStore
		checkDB = func() error { return pgStore.CheckHealth(context.Background()) }
		logger.Info("connected to PostgreSQL")
	} else {
		store = mock.NewMockStore()
		logger.Warn("DATABASE_URL not set, using in-memory store (data will not persist across restarts)")
	}

	// Set up task queue if Redis is configured.
	var queue model.TaskQueue
	var redisAddr string
	if redisURL != "" {
		redisAddr = parseRedisAddr(redisURL)
		q := worker.NewAsynqTaskQueue(redisAddr)
		queue = q
		defer q.Close()
		logger.Info("task queue configured", "redis", redisAddr)
	} else {
		logger.Warn("REDIS_URL not set, execution endpoints will be unavailable")
	}

	// Create router.
	router := api.NewRouter(store, logger, checkDB, nil, metricsEnabled, queue, redisAddr, isDev)

	// Build middleware chain: RequestID -> CORS -> Auth -> Logging -> Router.
	handler := middleware.RequestID(
		middleware.CORS(corsOrigins)(
			middleware.Auth(apiKeys, isDev)(
				middleware.Logging(logger)(
					router,
				),
			),
		),
	)

	addr := host + ":" + port
	srv := &http.Server{
		Addr:        addr,
		Handler:     handler,
		ReadTimeout: 30 * time.Second,
		// Sync execution requests can legitimately stay open for minutes while
		// the worker completes the graph, so do not apply a short write timeout.
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in a goroutine.
	errCh := make(chan error, 1)
	go func() {
		logger.Info("server started", "addr", addr, "env", env, "metrics", metricsEnabled)
		errCh <- srv.ListenAndServe()
	}()

	// Wait for interrupt signal or server error.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info("received signal, shutting down", "signal", sig.String())
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}

	// Graceful shutdown with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped")
}

// parseRedisAddr extracts host:port from a redis:// URL.
func parseRedisAddr(redisURL string) string {
	addr := redisURL
	for _, prefix := range []string{"redis://", "rediss://"} {
		if strings.HasPrefix(addr, prefix) {
			addr = strings.TrimPrefix(addr, prefix)
			break
		}
	}
	if idx := strings.Index(addr, "/"); idx != -1 {
		addr = addr[:idx]
	}
	if idx := strings.LastIndex(addr, "@"); idx != -1 {
		addr = addr[idx+1:]
	}
	if addr == "" {
		return "localhost:6379"
	}
	return addr
}

// getEnv returns the value of an environment variable or a default.
func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// setupLogger creates a structured slog.Logger based on configuration.
func setupLogger(level, format string, isDev bool) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}

	var handler slog.Handler
	if isDev || strings.ToLower(format) == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

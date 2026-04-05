package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"github.com/brockleyai/brockleyai/server/store/postgres"
	"github.com/brockleyai/brockleyai/worker"
)

func main() {
	// Load configuration from environment variables.
	databaseURL := os.Getenv("DATABASE_URL")
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379")
	logLevel := getEnv("BROCKLEY_LOG_LEVEL", "info")
	logFormat := getEnv("BROCKLEY_LOG_FORMAT", "json")
	concurrencyStr := getEnv("BROCKLEY_CONCURRENCY", "10")

	concurrency, err := strconv.Atoi(concurrencyStr)
	if err != nil || concurrency < 1 {
		concurrency = 10
	}

	// Set up structured logger.
	logger := setupLogger(logLevel, logFormat)

	if databaseURL == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	// Parse Redis address from REDIS_URL.
	redisAddr := parseRedisAddr(redisURL)

	// Connect to PostgreSQL.
	pgStore, err := postgres.New(context.Background(), databaseURL)
	if err != nil {
		logger.Error("failed to connect to PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer pgStore.Close()
	logger.Info("connected to PostgreSQL")

	// Create Redis client for the orchestrator and node handlers.
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer rdb.Close()

	// Create asynq client for node task dispatch.
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
	defer asynqClient.Close()

	// Create handlers.
	orchestratorHandler := worker.NewOrchestratorHandler(pgStore, rdb, asynqClient, logger)
	llmCallHandler := worker.NewLLMCallHandler(rdb, asynqClient, logger)
	mcpCallHandler := worker.NewMCPCallHandler(rdb, asynqClient, logger)
	apiCallHandler := worker.NewAPICallHandler(pgStore, rdb, asynqClient, logger)
	nodeRunHandler := worker.NewNodeRunHandler(rdb, logger)
	superagentHandler := worker.NewSuperagentHandler(pgStore, rdb, asynqClient, logger)

	// Create asynq server with queue priorities.
	// The orchestrator queue has lower priority since it's long-running (waiting on node results).
	// The nodes queue has higher priority since node tasks should be picked up quickly.
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Concurrency: concurrency,
			Queues: map[string]int{
				worker.QueueOrchestrator: 3, // lower priority
				worker.QueueNodes:        7, // higher priority
			},
			Logger: newAsynqLogger(logger),
		},
	)

	// Register handlers.
	mux := asynq.NewServeMux()
	mux.HandleFunc(worker.TaskTypeGraphStart, orchestratorHandler.ProcessTask)
	mux.HandleFunc(worker.TaskTypeLLMCall, llmCallHandler.ProcessTask)
	mux.HandleFunc(worker.TaskTypeMCPCall, mcpCallHandler.ProcessTask)
	mux.HandleFunc(worker.TaskTypeAPICall, apiCallHandler.ProcessTask)
	mux.HandleFunc(worker.TaskTypeNodeRun, nodeRunHandler.ProcessTask)
	mux.HandleFunc(worker.TaskTypeSuperagent, superagentHandler.ProcessTask)

	// Start server in a goroutine.
	errCh := make(chan error, 1)
	go func() {
		logger.Info("worker started",
			"concurrency", concurrency,
			"redis", redisAddr,
			"queues", map[string]int{worker.QueueOrchestrator: 3, worker.QueueNodes: 7},
		)
		errCh <- srv.Run(mux)
	}()

	// Wait for interrupt signal or server error.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info("received signal, shutting down", "signal", sig.String())
		srv.Shutdown()
	case err := <-errCh:
		if err != nil {
			logger.Error("worker error", "error", err)
			os.Exit(1)
		}
	}

	logger.Info("worker stopped")
}

// parseRedisAddr extracts host:port from a redis:// URL.
func parseRedisAddr(redisURL string) string {
	addr := redisURL
	// Strip scheme prefix.
	for _, prefix := range []string{"redis://", "rediss://"} {
		if strings.HasPrefix(addr, prefix) {
			addr = strings.TrimPrefix(addr, prefix)
			break
		}
	}
	// Strip any path suffix (e.g., /0).
	if idx := strings.Index(addr, "/"); idx != -1 {
		addr = addr[:idx]
	}
	// Strip userinfo (user:pass@).
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

// setupLogger creates a structured slog.Logger.
func setupLogger(level, format string) *slog.Logger {
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
	if strings.ToLower(format) == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

// asynqLogger adapts slog.Logger to asynq's Logger interface.
type asynqLogger struct {
	logger *slog.Logger
}

func newAsynqLogger(logger *slog.Logger) *asynqLogger {
	return &asynqLogger{logger: logger}
}

func (l *asynqLogger) Debug(args ...interface{}) {
	l.logger.Debug("asynq", "msg", args)
}

func (l *asynqLogger) Info(args ...interface{}) {
	l.logger.Info("asynq", "msg", args)
}

func (l *asynqLogger) Warn(args ...interface{}) {
	l.logger.Warn("asynq", "msg", args)
}

func (l *asynqLogger) Error(args ...interface{}) {
	l.logger.Error("asynq", "msg", args)
}

func (l *asynqLogger) Fatal(args ...interface{}) {
	l.logger.Error("asynq fatal", "msg", args)
	os.Exit(1)
}

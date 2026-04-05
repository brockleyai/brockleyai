package main

import (
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"github.com/brockleyai/brockleyai/worker"
)

func main() {
	// Load configuration from environment variables.
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379")
	logLevel := getEnv("BROCKLEY_LOG_LEVEL", "info")
	logFormat := getEnv("BROCKLEY_LOG_FORMAT", "json")
	concurrencyStr := getEnv("BROCKLEY_CODERUNNER_CONCURRENCY", "3")

	concurrency, err := strconv.Atoi(concurrencyStr)
	if err != nil || concurrency < 1 {
		concurrency = 3
	}

	// Set up structured logger.
	logger := setupLogger(logLevel, logFormat)

	// Parse Redis address from REDIS_URL.
	redisAddr := parseRedisAddr(redisURL)

	// Create Redis client.
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer rdb.Close()

	// Create handler.
	codeExecHandler := worker.NewCodeExecHandler(rdb, logger)

	// Create asynq server — only processes the code queue.
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Concurrency: concurrency,
			Queues: map[string]int{
				worker.QueueCode: 1,
			},
			Logger: newAsynqLogger(logger),
		},
	)

	// Register handler — only node:code-exec.
	mux := asynq.NewServeMux()
	mux.HandleFunc(worker.TaskTypeCodeExec, codeExecHandler.ProcessTask)

	// Start server in a goroutine.
	errCh := make(chan error, 1)
	go func() {
		logger.Info("coderunner started",
			"concurrency", concurrency,
			"redis", redisAddr,
			"queue", worker.QueueCode,
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
			logger.Error("coderunner error", "error", err)
			os.Exit(1)
		}
	}

	logger.Info("coderunner stopped")
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

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

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

type asynqLogger struct {
	logger *slog.Logger
}

func newAsynqLogger(logger *slog.Logger) *asynqLogger {
	return &asynqLogger{logger: logger}
}

func (l *asynqLogger) Debug(args ...interface{}) { l.logger.Debug("asynq", "msg", args) }
func (l *asynqLogger) Info(args ...interface{})  { l.logger.Info("asynq", "msg", args) }
func (l *asynqLogger) Warn(args ...interface{})  { l.logger.Warn("asynq", "msg", args) }
func (l *asynqLogger) Error(args ...interface{}) { l.logger.Error("asynq", "msg", args) }
func (l *asynqLogger) Fatal(args ...interface{}) {
	l.logger.Error("asynq fatal", "msg", args)
	os.Exit(1)
}

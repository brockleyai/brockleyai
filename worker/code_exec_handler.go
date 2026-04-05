package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

// CodeExecHandler processes node:code-exec tasks.
// It runs inside the coderunner component, executing Python code in an isolated subprocess
// and relaying tool calls through Redis back to the superagent handler.
type CodeExecHandler struct {
	rdb    *redis.Client
	logger *slog.Logger
}

// NewCodeExecHandler creates a new CodeExecHandler.
func NewCodeExecHandler(rdb *redis.Client, logger *slog.Logger) *CodeExecHandler {
	return &CodeExecHandler{
		rdb:    rdb,
		logger: logger,
	}
}

// ProcessTask handles an asynq task for code execution.
func (h *CodeExecHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var t CodeExecTask
	if err := json.Unmarshal(task.Payload(), &t); err != nil {
		return fmt.Errorf("code_exec: unmarshal task: %w", err)
	}

	logger := h.logger.With(
		"execution_id", t.ExecutionID,
		"node_id", t.NodeID,
		"seq", t.Seq,
	)
	logger.Info("code exec handler started")
	start := time.Now()

	// Validate payload.
	if t.Code == "" {
		return h.pushResult(ctx, t, CodeExecResult{
			Status:     "error",
			Error:      "empty code",
			DurationMs: time.Since(start).Milliseconds(),
		})
	}
	if t.MaxCodeBytes > 0 && len(t.Code) > t.MaxCodeBytes {
		return h.pushResult(ctx, t, CodeExecResult{
			Status:     "error",
			Error:      fmt.Sprintf("code size %d exceeds limit %d", len(t.Code), t.MaxCodeBytes),
			DurationMs: time.Since(start).Milliseconds(),
		})
	}

	// Apply deadline.
	timeout := time.Duration(t.MaxExecutionTimeSec) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Run code in subprocess.
	runner := &codeExecRunner{
		rdb:    h.rdb,
		logger: logger,
	}

	result, err := runner.Run(ctx, t)
	if err != nil {
		result = CodeExecResult{
			Status:     "error",
			Error:      err.Error(),
			DurationMs: time.Since(start).Milliseconds(),
		}
	} else {
		result.DurationMs = time.Since(start).Milliseconds()
	}

	logger.Info("code exec handler completed",
		"status", result.Status,
		"tool_calls", result.ToolCalls,
		"duration_ms", result.DurationMs,
	)

	return h.pushResult(ctx, t, result)
}

// pushResult pushes the final result to the result Redis key.
func (h *CodeExecHandler) pushResult(ctx context.Context, t CodeExecTask, result CodeExecResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshaling result: %w", err)
	}

	// Use a background context for the push — we need this to succeed even if ctx is cancelled.
	pushCtx := context.Background()
	if err := h.rdb.LPush(pushCtx, t.ResultKey, string(data)).Err(); err != nil {
		return fmt.Errorf("pushing result to %s: %w", t.ResultKey, err)
	}
	return nil
}

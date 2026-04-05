package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"github.com/brockleyai/brockleyai/engine/executor"
	"github.com/brockleyai/brockleyai/engine/provider"
	"github.com/brockleyai/brockleyai/internal/model"
	"github.com/brockleyai/brockleyai/internal/secret"
)

// NodeRunHandler processes node:run tasks.
// Used for complex nodes like forEach, subgraph, and standalone tool nodes.
type NodeRunHandler struct {
	rdb    *redis.Client
	logger *slog.Logger
}

// NewNodeRunHandler creates a new NodeRunHandler.
func NewNodeRunHandler(rdb *redis.Client, logger *slog.Logger) *NodeRunHandler {
	return &NodeRunHandler{rdb: rdb, logger: logger}
}

// ProcessTask handles an asynq task for node execution.
func (h *NodeRunHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var t NodeRunTask
	if err := json.Unmarshal(task.Payload(), &t); err != nil {
		return fmt.Errorf("node_run: unmarshal task: %w", err)
	}

	logger := h.logger.With(
		"execution_id", t.ExecutionID,
		"request_id", t.RequestID,
		"node_id", t.NodeID,
		"node_type", t.NodeType,
	)
	logger.Info("node run started")

	start := time.Now()

	// Build the node from the task payload.
	node := &model.Node{
		ID:     t.NodeID,
		Type:   t.NodeType,
		Config: t.NodeConfig,
	}

	// Build executor deps.
	providerRegistry := provider.NewDefaultRegistry()
	secretStore := secret.NewEnvSecretStore()
	deps := &executor.ExecutorDeps{
		ProviderRegistry: providerRegistry,
		SecretStore:      secretStore,
		MCPClientCache:   executor.NewMCPClientCache(),
		EventEmitter:     &noopEventEmitter{},
		Logger:           logger,
	}

	// Superagent nodes need event emission for SSE.
	if t.NodeType == model.NodeTypeSuperagent {
		deps.EventEmitter = &RedisEventEmitter{
			client:      h.rdb,
			executionID: t.ExecutionID,
			logger:      logger,
		}
	}

	// Build NodeContext.
	nctx := &executor.NodeContext{
		State: t.State,
		Meta:  t.Meta,
	}

	// Get the executor.
	registry := executor.NewDefaultRegistry()
	exec, err := registry.Get(t.NodeType)
	if err != nil {
		return h.pushFailure(ctx, t, fmt.Sprintf("no executor for node type %q", t.NodeType))
	}

	// Execute the node.
	result, err := exec.Execute(ctx, node, t.Inputs, nctx, deps)
	durationMs := time.Since(start).Milliseconds()

	if err != nil {
		logger.Error("node run failed", "error", err, "duration_ms", durationMs)
		return h.pushFailure(ctx, t, err.Error())
	}

	logger.Info("node run completed", "duration_ms", durationMs)

	var outputs map[string]any
	if result != nil {
		outputs = result.Outputs
	}

	return h.pushResult(ctx, t, NodeTaskResult{
		RequestID: t.RequestID,
		NodeID:    t.NodeID,
		Status:    "completed",
		Outputs:   outputs,
	})
}

// pushFailure pushes a failure result.
func (h *NodeRunHandler) pushFailure(ctx context.Context, t NodeRunTask, errMsg string) error {
	return h.pushResult(ctx, t, NodeTaskResult{
		RequestID: t.RequestID,
		NodeID:    t.NodeID,
		Status:    "failed",
		Error:     errMsg,
	})
}

// pushResult marshals and pushes a NodeTaskResult to the result Redis key.
func (h *NodeRunHandler) pushResult(ctx context.Context, t NodeRunTask, result NodeTaskResult) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("node_run: marshal result: %w", err)
	}
	return h.rdb.LPush(ctx, t.ResultKey, string(resultJSON)).Err()
}

// noopEventEmitter is used for node tasks that shouldn't emit events
// (the orchestrator handles event emission).
type noopEventEmitter struct{}

func (noopEventEmitter) Emit(model.ExecutionEvent) {}

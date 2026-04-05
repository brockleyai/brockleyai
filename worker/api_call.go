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
	"github.com/brockleyai/brockleyai/internal/model"
)

// APICallHandler processes node:api-call tasks.
// It executes one API tool HTTP endpoint call and pushes the result to Redis.
type APICallHandler struct {
	store       model.Store
	rdb         *redis.Client
	asynqClient *asynq.Client
	logger      *slog.Logger
}

// NewAPICallHandler creates a new APICallHandler.
func NewAPICallHandler(store model.Store, rdb *redis.Client, asynqClient *asynq.Client, logger *slog.Logger) *APICallHandler {
	return &APICallHandler{store: store, rdb: rdb, asynqClient: asynqClient, logger: logger}
}

// ProcessTask handles an asynq task for API tool calls.
func (h *APICallHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var t APICallTask
	if err := json.Unmarshal(task.Payload(), &t); err != nil {
		return fmt.Errorf("api_call: unmarshal task: %w", err)
	}

	logger := h.logger.With(
		"execution_id", t.ExecutionID,
		"request_id", t.RequestID,
		"node_id", t.NodeID,
		"tool_name", t.ToolName,
		"api_tool_id", t.APIToolID,
		"api_endpoint", t.APIEndpoint,
	)
	logger.Info("api call started")

	start := time.Now()

	// Create API tool dispatcher.
	dispatcher := executor.NewAPIToolDispatcher(h.store, h.logger)

	// Build a ToolRoute from the task fields.
	route := model.ToolRoute{
		APIToolID:   t.APIToolID,
		APIEndpoint: t.APIEndpoint,
		Headers:     t.Headers,
	}
	if t.TimeoutSeconds > 0 {
		route.TimeoutSeconds = &t.TimeoutSeconds
	}

	// Execute the API endpoint call.
	toolResult, err := dispatcher.CallEndpoint(ctx, t.TenantID, route, t.ToolName, t.Arguments)
	durationMs := time.Since(start).Milliseconds()

	var result APICallResult
	result.RequestID = t.RequestID
	result.ToolCallID = t.RequestID
	result.ToolName = t.ToolName
	result.DurationMs = durationMs

	if err != nil {
		result.Error = fmt.Sprintf("Error: %v", err)
		result.IsError = true
		logger.Error("api call failed", "error", err, "duration_ms", durationMs)
	} else if toolResult.IsError {
		result.Error = toolResult.Error
		result.IsError = true
		result.Content = toolResult.Content
		logger.Info("api call returned error", "error", toolResult.Error, "duration_ms", durationMs)
	} else {
		result.Content = toolResult.Content
		logger.Info("api call completed", "duration_ms", durationMs)
	}

	// Check retry on network/dispatch errors (not tool-returned errors).
	if result.IsError && result.Content == nil && shouldRetry(t.RetryPolicy, t.Attempt) {
		return h.retryAPICall(ctx, t, result.Error, logger)
	}

	// Push result to the specified Redis key.
	// Tool-loop API calls push APICallResult (consumed by LLMCallHandler).
	// Standalone tool nodes push NodeTaskResult (consumed by the orchestrator).
	var pushPayload any
	if t.ForToolLoop {
		pushPayload = result
	} else {
		pushPayload = h.toNodeTaskResult(t, result)
	}

	resultJSON, err := json.Marshal(pushPayload)
	if err != nil {
		return fmt.Errorf("api_call: marshal result: %w", err)
	}

	if err := h.rdb.LPush(ctx, t.ResultKey, string(resultJSON)).Err(); err != nil {
		return fmt.Errorf("api_call: lpush result: %w", err)
	}

	return nil
}

// toNodeTaskResult converts an APICallResult into a NodeTaskResult for the orchestrator.
func (h *APICallHandler) toNodeTaskResult(t APICallTask, r APICallResult) NodeTaskResult {
	if r.IsError {
		return NodeTaskResult{
			RequestID: t.RequestID,
			NodeID:    t.NodeID,
			Status:    "failed",
			Error:     r.Error,
		}
	}

	// Convert content to string for the node output.
	var resultStr string
	if r.Content != nil {
		switch v := r.Content.(type) {
		case string:
			resultStr = v
		default:
			b, err := json.Marshal(r.Content)
			if err != nil {
				resultStr = fmt.Sprintf("%v", r.Content)
			} else {
				resultStr = string(b)
			}
		}
	}

	return NodeTaskResult{
		RequestID: t.RequestID,
		NodeID:    t.NodeID,
		Status:    "completed",
		Outputs:   map[string]any{"result": resultStr},
	}
}

// retryAPICall re-enqueues the API call task with an incremented attempt count
// and emits an EventNodeRetrying event for SSE consumers.
func (h *APICallHandler) retryAPICall(ctx context.Context, t APICallTask, errMsg string, logger *slog.Logger) error {
	delay := retryDelay(t.RetryPolicy, t.Attempt)
	t.Attempt++

	logger.Info("retrying api call", "attempt", t.Attempt, "delay", delay, "tool_name", t.ToolName)

	nodeType := model.NodeTypeTool
	if t.ForToolLoop {
		nodeType = model.NodeTypeLLM
	}
	emitRetryingEvent(h.rdb, t.ExecutionID, t.NodeID, nodeType, t.Attempt, errMsg)

	payload, err := json.Marshal(t)
	if err != nil {
		// Can't retry — push failure.
		h.pushAPIFailure(ctx, t, fmt.Sprintf("marshal retry: %v", err))
		return nil
	}

	asynqTask := asynq.NewTask(TaskTypeAPICall, payload,
		asynq.Queue(QueueNodes),
		asynq.ProcessIn(delay),
	)
	if _, retryErr := h.asynqClient.Enqueue(asynqTask); retryErr != nil {
		// Can't retry — push failure.
		h.pushAPIFailure(ctx, t, fmt.Sprintf("enqueue retry: %v", retryErr))
		return nil
	}

	// Don't push any result — the retry will push when it completes/fails.
	return nil
}

// pushAPIFailure pushes a failure result in the correct format for the caller.
func (h *APICallHandler) pushAPIFailure(ctx context.Context, t APICallTask, errMsg string) {
	var payload any
	if t.ForToolLoop {
		payload = APICallResult{
			RequestID: t.RequestID,
			ToolName:  t.ToolName,
			Error:     errMsg,
			IsError:   true,
		}
	} else {
		payload = NodeTaskResult{
			RequestID: t.RequestID,
			NodeID:    t.NodeID,
			Status:    "failed",
			Error:     errMsg,
		}
	}
	resultJSON, _ := json.Marshal(payload)
	h.rdb.LPush(ctx, t.ResultKey, string(resultJSON))
}

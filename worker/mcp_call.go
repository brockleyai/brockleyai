package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"github.com/brockleyai/brockleyai/engine/mcp"
	"github.com/brockleyai/brockleyai/internal/model"
)

// MCPCallHandler processes node:mcp-call tasks.
// It executes one MCP tool call or list_tools call and pushes the result to Redis.
type MCPCallHandler struct {
	rdb         *redis.Client
	asynqClient *asynq.Client
	logger      *slog.Logger
}

// NewMCPCallHandler creates a new MCPCallHandler.
func NewMCPCallHandler(rdb *redis.Client, asynqClient *asynq.Client, logger *slog.Logger) *MCPCallHandler {
	return &MCPCallHandler{rdb: rdb, asynqClient: asynqClient, logger: logger}
}

// ProcessTask handles an asynq task for MCP calls.
func (h *MCPCallHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var t MCPCallTask
	if err := json.Unmarshal(task.Payload(), &t); err != nil {
		return fmt.Errorf("mcp_call: unmarshal task: %w", err)
	}

	logger := h.logger.With(
		"execution_id", t.ExecutionID,
		"request_id", t.RequestID,
		"node_id", t.NodeID,
		"tool_name", t.ToolName,
		"operation", t.Operation,
	)
	logger.Info("mcp call started")

	start := time.Now()

	// Create MCP client.
	client := mcp.NewClient(t.MCPURL, t.Headers)

	var result MCPCallResult
	result.RequestID = t.RequestID
	result.ToolCallID = t.RequestID
	result.ToolName = t.ToolName

	switch t.Operation {
	case "call_tool":
		// Apply per-call timeout.
		timeoutSec := t.TimeoutSeconds
		if timeoutSec <= 0 {
			timeoutSec = 30
		}
		callCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
		defer cancel()

		toolResult, err := client.CallTool(callCtx, t.ToolName, t.Arguments)
		result.DurationMs = time.Since(start).Milliseconds()

		if err != nil {
			result.Error = fmt.Sprintf("Error: %v", err)
			result.IsError = true
			logger.Error("mcp call_tool failed", "error", err, "duration_ms", result.DurationMs)
		} else if toolResult.IsError {
			result.Error = fmt.Sprintf("Error: %s", toolResult.Error)
			result.IsError = true
			result.Content = toolResult.Content
			logger.Info("mcp call_tool returned error", "error", toolResult.Error, "duration_ms", result.DurationMs)
		} else {
			result.Content = toolResult.Content
			logger.Info("mcp call_tool completed", "duration_ms", result.DurationMs)
		}

	case "list_tools":
		tools, err := client.ListTools(ctx)
		result.DurationMs = time.Since(start).Milliseconds()

		if err != nil {
			result.Error = fmt.Sprintf("Error: %v", err)
			result.IsError = true
			logger.Error("mcp list_tools failed", "error", err, "duration_ms", result.DurationMs)
		} else {
			result.Content = tools
			logger.Info("mcp list_tools completed", "tool_count", len(tools), "duration_ms", result.DurationMs)
		}

	default:
		result.Error = fmt.Sprintf("unknown operation: %s", t.Operation)
		result.IsError = true
		result.DurationMs = time.Since(start).Milliseconds()
	}

	// Check retry on network errors (not tool-returned errors).
	if result.IsError && result.Content == nil && shouldRetry(t.RetryPolicy, t.Attempt) {
		return h.retryMCPCall(ctx, t, result.Error, logger)
	}

	// Push result to the specified Redis key.
	// Tool-loop MCP calls push MCPCallResult (consumed by LLMCallHandler).
	// Standalone tool nodes push NodeTaskResult (consumed by the orchestrator).
	var pushPayload any
	if t.ForToolLoop {
		pushPayload = result
	} else {
		pushPayload = h.toNodeTaskResult(t, result)
	}

	resultJSON, err := json.Marshal(pushPayload)
	if err != nil {
		return fmt.Errorf("mcp_call: marshal result: %w", err)
	}

	if err := h.rdb.LPush(ctx, t.ResultKey, string(resultJSON)).Err(); err != nil {
		return fmt.Errorf("mcp_call: lpush result: %w", err)
	}

	return nil
}

// toNodeTaskResult converts an MCPCallResult into a NodeTaskResult for the orchestrator.
func (h *MCPCallHandler) toNodeTaskResult(t MCPCallTask, r MCPCallResult) NodeTaskResult {
	if r.IsError {
		return NodeTaskResult{
			RequestID: t.RequestID,
			NodeID:    t.NodeID,
			Status:    "failed",
			Error:     r.Error,
		}
	}

	// Extract text content from the MCP result for the node output.
	var resultStr string
	if r.Content != nil {
		// Content may be a plain string (single text block) or []contentBlock.
		switch v := r.Content.(type) {
		case string:
			resultStr = v
		default:
			b, err := json.Marshal(r.Content)
			if err != nil {
				resultStr = fmt.Sprintf("%v", r.Content)
			} else {
				resultStr = extractMCPTextContent(b)
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

// extractMCPTextContent extracts the text from MCP content responses.
// MCP tools return content as [{"type":"text","text":"value"}].
func extractMCPTextContent(raw []byte) string {
	// Try array of content blocks first.
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) == nil && len(blocks) > 0 {
		for _, b := range blocks {
			if b.Type == "text" {
				return b.Text
			}
		}
	}
	// Fallback: return the raw JSON string.
	return string(raw)
}

// retryMCPCall re-enqueues the MCP call task with an incremented attempt count
// and emits an EventNodeRetrying event for SSE consumers.
func (h *MCPCallHandler) retryMCPCall(ctx context.Context, t MCPCallTask, errMsg string, logger *slog.Logger) error {
	delay := retryDelay(t.RetryPolicy, t.Attempt)
	t.Attempt++

	logger.Info("retrying mcp call", "attempt", t.Attempt, "delay", delay, "tool_name", t.ToolName)

	nodeType := model.NodeTypeTool
	if t.ForToolLoop {
		nodeType = model.NodeTypeLLM
	}
	emitRetryingEvent(h.rdb, t.ExecutionID, t.NodeID, nodeType, t.Attempt, errMsg)

	payload, err := json.Marshal(t)
	if err != nil {
		// Can't retry — push failure.
		h.pushMCPFailure(ctx, t, fmt.Sprintf("marshal retry: %v", err))
		return nil
	}

	asynqTask := asynq.NewTask(TaskTypeMCPCall, payload,
		asynq.Queue(QueueNodes),
		asynq.ProcessIn(delay),
	)
	if _, retryErr := h.asynqClient.Enqueue(asynqTask); retryErr != nil {
		// Can't retry — push failure.
		h.pushMCPFailure(ctx, t, fmt.Sprintf("enqueue retry: %v", retryErr))
		return nil
	}

	// Don't push any result — the retry will push when it completes/fails.
	return nil
}

// pushMCPFailure pushes a failure result in the correct format for the caller.
func (h *MCPCallHandler) pushMCPFailure(ctx context.Context, t MCPCallTask, errMsg string) {
	var payload any
	if t.ForToolLoop {
		payload = MCPCallResult{
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

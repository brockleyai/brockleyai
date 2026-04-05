package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"github.com/brockleyai/brockleyai/engine/provider"
	"github.com/brockleyai/brockleyai/internal/model"
)

// LLMCallHandler processes node:llm-call tasks.
// It executes one provider.Complete() call. If a tool loop is active and the LLM
// returns tool_calls, it dispatches node:mcp-call tasks for each tool call,
// waits for results, and enqueues the next LLM call iteration.
type LLMCallHandler struct {
	rdb         *redis.Client
	asynqClient *asynq.Client
	logger      *slog.Logger
}

// NewLLMCallHandler creates a new LLMCallHandler.
func NewLLMCallHandler(rdb *redis.Client, asynqClient *asynq.Client, logger *slog.Logger) *LLMCallHandler {
	return &LLMCallHandler{
		rdb:         rdb,
		asynqClient: asynqClient,
		logger:      logger,
	}
}

// ProcessTask handles an asynq task for LLM calls.
func (h *LLMCallHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var t LLMCallTask
	if err := json.Unmarshal(task.Payload(), &t); err != nil {
		return fmt.Errorf("llm_call: unmarshal task: %w", err)
	}

	logger := h.logger.With(
		"execution_id", t.ExecutionID,
		"request_id", t.RequestID,
		"node_id", t.NodeID,
		"provider", t.Provider,
		"attempt", t.Attempt,
	)
	logger.Info("llm call started")

	// Look up provider.
	registry := provider.NewDefaultRegistry()
	llmProvider, err := registry.Get(t.Provider)
	if err != nil {
		return h.pushFailure(ctx, t, fmt.Sprintf("provider lookup failed: %v", err))
	}

	// Restore API key — CompletionRequest.APIKey is json:"-" so it's not serialized in the task payload.
	t.Request.APIKey = t.APIKey

	// Call the LLM.
	start := time.Now()
	resp, err := llmProvider.Complete(ctx, t.Request)
	durationMs := time.Since(start).Milliseconds()

	if err != nil {
		logger.Error("llm call failed", "error", err, "duration_ms", durationMs)
		// Check retry policy.
		if shouldRetry(t.RetryPolicy, t.Attempt) {
			return h.retryLLMCall(ctx, t, fmt.Sprintf("provider call failed: %v", err), logger)
		}
		return h.pushFailure(ctx, t, fmt.Sprintf("provider call failed: %v", err))
	}

	logger.Info("llm call completed",
		"finish_reason", resp.FinishReason,
		"tool_calls", len(resp.ToolCalls),
		"duration_ms", durationMs,
	)

	trace := buildLLMCallTrace(t, resp)

	// Non-tool-loop nodes complete immediately.
	if t.ToolLoop == nil {
		return h.pushLLMResult(ctx, t, resp, trace)
	}

	tls := t.ToolLoop
	appendToolLoopTrace(tls, trace)

	// Tool loop is complete once the model stops asking for tools.
	if resp.FinishReason != "tool_calls" || len(resp.ToolCalls) == 0 {
		return h.pushToolLoopResult(ctx, t, resp, tls)
	}

	// Tool loop active: check limits.
	if tls.Iteration >= tls.MaxIterations || tls.TotalToolCalls+len(resp.ToolCalls) > tls.MaxCalls {
		tls.FinishReason = "limit_reached"
		return h.pushToolLoopResult(ctx, t, resp, tls)
	}

	// Append assistant message with tool calls to the conversation.
	t.Request.Messages = append(t.Request.Messages, model.Message{
		Role:      "assistant",
		Content:   resp.Content,
		ToolCalls: resp.ToolCalls,
	})

	// Dispatch MCP calls for each tool call and wait for results.
	mcpResultKey := ResultKeyForLLMCall(t.ExecutionID, t.RequestID)
	err = h.dispatchAndCollectMCPCalls(ctx, t, resp.ToolCalls, mcpResultKey, logger)
	if err != nil {
		return h.pushFailure(ctx, t, fmt.Sprintf("mcp dispatch failed: %v", err))
	}

	// Increment iteration.
	tls.Iteration++
	tls.TotalToolCalls += len(resp.ToolCalls)

	// Enqueue next LLM call with updated messages.
	return h.enqueueNextLLMCall(ctx, t, logger)
}

// dispatchAndCollectMCPCalls sends MCP tasks for each tool call and waits for all results.
func (h *LLMCallHandler) dispatchAndCollectMCPCalls(
	ctx context.Context,
	t LLMCallTask,
	toolCalls []model.ToolCall,
	mcpResultKey string,
	logger *slog.Logger,
) error {
	tls := t.ToolLoop

	for _, tc := range toolCalls {
		route, ok := tls.Routing[tc.Name]
		if !ok {
			// Unknown tool — feed error back to LLM as tool result message.
			errMsg := fmt.Sprintf("Error: tool %q is not available", tc.Name)
			t.Request.Messages = append(t.Request.Messages, model.Message{
				Role:            "tool",
				ToolCallID:      tc.ID,
				Content:         errMsg,
				ToolResultError: true,
			})
			tls.History = append(tls.History, ToolCallHistoryEntry{
				Name:      tc.Name,
				Arguments: tc.Arguments,
				Result:    errMsg,
				IsError:   true,
			})
			continue
		}

		// Parse arguments.
		var args map[string]any
		if err := json.Unmarshal(tc.Arguments, &args); err != nil {
			args = map[string]any{"raw": string(tc.Arguments)}
		}

		timeoutSec := 30
		if route.TimeoutSeconds != nil {
			timeoutSec = *route.TimeoutSeconds
		}

		if route.APIToolID != "" {
			// API endpoint dispatch.
			apiTask := APICallTask{
				ExecutionID:    t.ExecutionID,
				RequestID:      tc.ID,
				NodeID:         t.NodeID,
				TenantID:       stringFromMeta(tls.NodeMeta, "tenant_id"),
				APIToolID:      route.APIToolID,
				APIEndpoint:    route.APIEndpoint,
				Headers:        route.Headers,
				ToolName:       tc.Name,
				Arguments:      args,
				TimeoutSeconds: timeoutSec,
				ResultKey:      mcpResultKey,
				ForToolLoop:    true,
			}

			payload, err := json.Marshal(apiTask)
			if err != nil {
				return fmt.Errorf("marshal api task: %w", err)
			}

			asynqTask := asynq.NewTask(TaskTypeAPICall, payload, asynq.Queue(QueueNodes))
			if _, err := h.asynqClient.Enqueue(asynqTask); err != nil {
				return fmt.Errorf("enqueue api task for tool %q: %w", tc.Name, err)
			}
			logger.Info("dispatched api call", "tool_name", tc.Name, "tool_call_id", tc.ID)
		} else {
			// MCP dispatch (existing path).
			headers := resolveRouteHeadersStatic(route)

			mcpTask := MCPCallTask{
				ExecutionID:    t.ExecutionID,
				RequestID:      tc.ID,
				NodeID:         t.NodeID,
				Operation:      "call_tool",
				MCPURL:         route.MCPURL,
				Headers:        headers,
				ToolName:       tc.Name,
				Arguments:      args,
				TimeoutSeconds: timeoutSec,
				ResultKey:      mcpResultKey,
				ForToolLoop:    true,
			}

			payload, err := json.Marshal(mcpTask)
			if err != nil {
				return fmt.Errorf("marshal mcp task: %w", err)
			}

			asynqTask := asynq.NewTask(TaskTypeMCPCall, payload, asynq.Queue(QueueNodes))
			if _, err := h.asynqClient.Enqueue(asynqTask); err != nil {
				return fmt.Errorf("enqueue mcp task for tool %q: %w", tc.Name, err)
			}
			logger.Info("dispatched mcp call", "tool_name", tc.Name, "tool_call_id", tc.ID)
		}
	}

	// Count how many MCP tasks we actually dispatched (excludes unknown tools).
	dispatchedCount := 0
	for _, tc := range toolCalls {
		if _, ok := tls.Routing[tc.Name]; ok {
			dispatchedCount++
		}
	}

	// Wait for all dispatched MCP results via BRPOP.
	for i := 0; i < dispatchedCount; i++ {
		result, err := h.rdb.BRPop(ctx, 5*time.Minute, mcpResultKey).Result()
		if err != nil {
			return fmt.Errorf("brpop mcp result: %w", err)
		}
		if len(result) < 2 {
			return fmt.Errorf("brpop mcp result: unexpected result length")
		}

		var mcpResult MCPCallResult
		if err := json.Unmarshal([]byte(result[1]), &mcpResult); err != nil {
			return fmt.Errorf("unmarshal mcp result: %w", err)
		}

		// Build tool result message.
		var resultStr string
		if mcpResult.IsError {
			resultStr = mcpResult.Error
		} else {
			resultBytes, err := json.Marshal(mcpResult.Content)
			if err != nil {
				resultStr = fmt.Sprintf("%v", mcpResult.Content)
			} else {
				resultStr = string(resultBytes)
			}
		}

		t.Request.Messages = append(t.Request.Messages, model.Message{
			Role:            "tool",
			ToolCallID:      mcpResult.ToolCallID,
			Content:         resultStr,
			ToolResultError: mcpResult.IsError,
		})

		tls.History = append(tls.History, ToolCallHistoryEntry{
			Name:       mcpResult.ToolName,
			Arguments:  nil, // already recorded in tool call
			Result:     resultStr,
			DurationMs: mcpResult.DurationMs,
			IsError:    mcpResult.IsError,
		})
	}

	// Clean up MCP result key.
	h.rdb.Del(ctx, mcpResultKey)

	return nil
}

// enqueueNextLLMCall creates the next LLM call task with updated messages for the next tool loop iteration.
func (h *LLMCallHandler) enqueueNextLLMCall(ctx context.Context, t LLMCallTask, logger *slog.Logger) error {
	nextRequestID := fmt.Sprintf("%s_iter%d", t.NodeID, t.ToolLoop.Iteration)

	nextTask := LLMCallTask{
		ExecutionID: t.ExecutionID,
		RequestID:   nextRequestID,
		NodeID:      t.NodeID,
		Provider:    t.Provider,
		APIKey:      t.APIKey,
		Request:     t.Request,
		ToolLoop:    t.ToolLoop,
		Attempt:     0,
		RetryPolicy: t.RetryPolicy,
		Debug:       t.Debug,
	}

	payload, err := json.Marshal(nextTask)
	if err != nil {
		return fmt.Errorf("marshal next llm task: %w", err)
	}

	asynqTask := asynq.NewTask(TaskTypeLLMCall, payload, asynq.Queue(QueueNodes))
	if _, err := h.asynqClient.Enqueue(asynqTask); err != nil {
		return fmt.Errorf("enqueue next llm call: %w", err)
	}

	logger.Info("enqueued next llm call iteration", "iteration", t.ToolLoop.Iteration, "request_id", nextRequestID)

	// Wait for the next LLM call's result and forward it to the orchestrator.
	resultKey := ResultKeyForExecution(t.ExecutionID)
	result, err := h.rdb.BRPop(ctx, 10*time.Minute, resultKey).Result()
	if err != nil {
		return fmt.Errorf("brpop next llm result: %w", err)
	}
	if len(result) < 2 {
		return fmt.Errorf("brpop next llm result: unexpected length")
	}

	// The result from the child LLM call is already on the exec results key.
	// We just pass it through — it was pushed by the child to the same key.
	// Actually, we need to forward the child's result back to the exec results key.
	// But the child already pushes to exec:{execution_id}:results, so the orchestrator
	// will receive it directly. We don't need to do anything here.
	// However, we popped it — need to re-push it.
	if err := h.rdb.LPush(ctx, resultKey, result[1]).Err(); err != nil {
		return fmt.Errorf("re-push next llm result: %w", err)
	}

	return nil
}

// retryLLMCall re-enqueues the LLM call task with an incremented attempt count
// and emits an EventNodeRetrying event for SSE consumers.
func (h *LLMCallHandler) retryLLMCall(ctx context.Context, t LLMCallTask, errMsg string, logger *slog.Logger) error {
	delay := retryDelay(t.RetryPolicy, t.Attempt)
	t.Attempt++

	logger.Info("retrying llm call", "attempt", t.Attempt, "delay", delay)

	emitRetryingEvent(h.rdb, t.ExecutionID, t.NodeID, model.NodeTypeLLM, t.Attempt, errMsg)

	payload, err := json.Marshal(t)
	if err != nil {
		return h.pushFailure(ctx, t, fmt.Sprintf("marshal retry task: %v", err))
	}

	asynqTask := asynq.NewTask(TaskTypeLLMCall, payload,
		asynq.Queue(QueueNodes),
		asynq.ProcessIn(delay),
	)
	if _, err := h.asynqClient.Enqueue(asynqTask); err != nil {
		return h.pushFailure(ctx, t, fmt.Sprintf("enqueue retry: %v", err))
	}

	// Don't push any result — the retry will push when it completes/fails.
	return nil
}

// pushLLMResult pushes a direct LLM result (no tool loop) to the execution results key.
func (h *LLMCallHandler) pushLLMResult(ctx context.Context, t LLMCallTask, resp *model.CompletionResponse, trace *model.LLMCallTrace) error {
	outputs := make(map[string]any)
	outputs["response_text"] = resp.Content

	// Try to parse as JSON.
	var parsed any
	if json.Unmarshal([]byte(resp.Content), &parsed) == nil {
		outputs["response"] = parsed
	}

	outputs["finish_reason"] = resp.FinishReason
	if len(resp.ToolCalls) > 0 {
		outputs["tool_calls"] = resp.ToolCalls
	}

	return h.pushResult(ctx, t, NodeTaskResult{
		RequestID: t.RequestID,
		NodeID:    t.NodeID,
		Status:    "completed",
		Outputs:   outputs,
		Attempt:   t.Attempt,
		LLMUsage:  completionUsage(resp),
		LLMDebug:  debugTraceForResult(t.Debug, trace),
	})
}

// pushToolLoopResult pushes the final result of a tool loop to the execution results key.
func (h *LLMCallHandler) pushToolLoopResult(ctx context.Context, t LLMCallTask, resp *model.CompletionResponse, tls *ToolLoopState) error {
	outputs := make(map[string]any)

	if resp != nil {
		outputs["response_text"] = resp.Content
		var parsed any
		if json.Unmarshal([]byte(resp.Content), &parsed) == nil {
			outputs["response"] = parsed
		}
	} else {
		outputs["response_text"] = ""
	}

	finishReason := tls.FinishReason
	if finishReason == "" && resp != nil {
		finishReason = resp.FinishReason
	}

	outputs["finish_reason"] = finishReason
	outputs["messages"] = t.Request.Messages
	outputs["tool_call_history"] = tls.History
	outputs["iterations"] = tls.Iteration
	outputs["total_tool_calls"] = tls.TotalToolCalls

	return h.pushResult(ctx, t, NodeTaskResult{
		RequestID: t.RequestID,
		NodeID:    t.NodeID,
		Status:    "completed",
		Outputs:   outputs,
		Attempt:   t.Attempt,
		LLMUsage:  completionUsage(resp),
		LLMDebug:  toolLoopDebugTrace(t.Debug, tls),
	})
}

// pushFailure pushes a failure result to the execution results key.
func (h *LLMCallHandler) pushFailure(ctx context.Context, t LLMCallTask, errMsg string) error {
	return h.pushResult(ctx, t, NodeTaskResult{
		RequestID: t.RequestID,
		NodeID:    t.NodeID,
		Status:    "failed",
		Error:     errMsg,
		Attempt:   t.Attempt,
	})
}

// pushResult marshals and pushes a NodeTaskResult to the appropriate Redis key.
// If the task has a ResultKey set (e.g., from a superagent coordinator), push there.
// Otherwise, push to the default execution results key.
func (h *LLMCallHandler) pushResult(ctx context.Context, t LLMCallTask, result NodeTaskResult) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("llm_call: marshal result: %w", err)
	}
	key := t.ResultKey
	if key == "" {
		key = ResultKeyForExecution(t.ExecutionID)
	}
	return h.rdb.LPush(ctx, key, string(resultJSON)).Err()
}

// resolveRouteHeadersStatic resolves static headers from a ToolRoute.
// For distributed execution, dynamic headers (from_input, secret_ref) are resolved
// at the orchestrator level and baked into the task payload.
func stringFromMeta(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	v, _ := meta[key].(string)
	return v
}

func resolveRouteHeadersStatic(route model.ToolRoute) map[string]string {
	headers := make(map[string]string)
	for _, hc := range route.Headers {
		if hc.Value != "" {
			headers[hc.Name] = hc.Value
		}
	}
	return headers
}

func buildLLMCallTrace(t LLMCallTask, resp *model.CompletionResponse) *model.LLMCallTrace {
	if !t.Debug || resp == nil || len(resp.RawRequest) == 0 || len(resp.RawResponse) == 0 {
		return nil
	}
	modelName := resp.Model
	if modelName == "" && t.Request != nil {
		modelName = t.Request.Model
	}
	return &model.LLMCallTrace{
		RequestID: t.RequestID,
		Provider:  t.Provider,
		Model:     modelName,
		Request:   resp.RawRequest,
		Response:  resp.RawResponse,
	}
}

func appendToolLoopTrace(tls *ToolLoopState, trace *model.LLMCallTrace) {
	if tls == nil || trace == nil {
		return
	}
	tls.DebugTraces = append(tls.DebugTraces, *trace)
}

func debugTraceForResult(enabled bool, trace *model.LLMCallTrace) *model.LLMDebugTrace {
	if !enabled || trace == nil {
		return nil
	}
	return &model.LLMDebugTrace{Calls: []model.LLMCallTrace{*trace}}
}

func toolLoopDebugTrace(enabled bool, tls *ToolLoopState) *model.LLMDebugTrace {
	if !enabled || tls == nil || len(tls.DebugTraces) == 0 {
		return nil
	}
	calls := make([]model.LLMCallTrace, len(tls.DebugTraces))
	copy(calls, tls.DebugTraces)
	return &model.LLMDebugTrace{Calls: calls}
}

func completionUsage(resp *model.CompletionResponse) *model.LLMUsage {
	if resp == nil {
		return nil
	}
	usage := resp.Usage
	if usage.Provider == "" && usage.Model == "" && usage.TotalTokens == 0 && usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		return nil
	}
	return &usage
}

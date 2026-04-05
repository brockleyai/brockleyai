package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/brockleyai/brockleyai/internal/model"
)

const (
	defaultMaxToolCalls      = 25
	defaultMaxLoopIterations = 10
	defaultToolTimeoutSec    = 30
)

// ToolInterceptor intercepts a tool call before MCP dispatch.
// If it returns (result, true), the result is used directly and MCP dispatch is skipped.
// If it returns ("", false), normal MCP dispatch proceeds.
type ToolInterceptor func(toolName string, args json.RawMessage) (string, bool)

// ToolLoopConfig configures a reusable tool loop execution.
type ToolLoopConfig struct {
	Provider      model.LLMProvider
	Request       *model.CompletionRequest
	Routing       map[string]model.ToolRoute
	MaxCalls      int
	MaxIterations int
	Interceptor   ToolInterceptor // optional; nil means no interception
}

// ToolLoopResult holds the output of RunToolLoop.
type ToolLoopResult struct {
	Response       *model.CompletionResponse
	Messages       []model.Message
	History        []ToolCallHistoryEntry
	Iterations     int
	TotalToolCalls int
	FinishReason   string
}

// ToolCallHistoryEntry records a single tool invocation during a tool loop.
type ToolCallHistoryEntry struct {
	Name       string          `json:"name"`
	Arguments  json.RawMessage `json:"arguments"`
	Result     string          `json:"result"`
	DurationMs int64           `json:"duration_ms"`
	IsError    bool            `json:"is_error"`
}

// executeToolLoop runs the tool loop for an LLM node.
// It calls the LLM, dispatches tool calls via MCP, feeds results back,
// and repeats until the LLM produces a final text response or limits are hit.
func executeToolLoop(
	ctx context.Context,
	cfg *model.LLMNodeConfig,
	req *model.CompletionRequest,
	provider model.LLMProvider,
	deps *ExecutorDeps,
	nctx *NodeContext,
) (*NodeResult, error) {
	maxCalls := defaultMaxToolCalls
	if cfg.MaxToolCalls != nil {
		maxCalls = *cfg.MaxToolCalls
	}
	maxIterations := defaultMaxLoopIterations
	if cfg.MaxLoopIterations != nil {
		maxIterations = *cfg.MaxLoopIterations
	}

	// Resolve tool routing.
	routing, err := resolveToolRouting(ctx, cfg, nctx, deps)
	if err != nil {
		return nil, fmt.Errorf("tool loop: %w", err)
	}

	// Resolve API tool refs into tool definitions and routing entries.
	if len(cfg.APITools) > 0 && deps.APIToolDispatcher != nil {
		tenantID, _ := nctx.Meta["tenant_id"].(string)
		apiTools, apiRouting, resolveErr := resolveAPIToolRefs(ctx, cfg.APITools, deps.APIToolDispatcher, tenantID)
		if resolveErr != nil {
			return nil, fmt.Errorf("tool loop: %w", resolveErr)
		}
		req.Tools = append(req.Tools, apiTools...)
		for k, v := range apiRouting {
			routing[k] = v
		}
	}

	// Auto-discover tool definitions if none provided.
	if len(req.Tools) == 0 && deps.MCPClientCache != nil {
		discovered, err := autoDiscoverTools(ctx, routing, deps.MCPClientCache)
		if err != nil {
			return nil, fmt.Errorf("tool loop: auto-discover tools: %w", err)
		}
		req.Tools = discovered
	}

	// Load messages from state if configured.
	if cfg.MessagesFromState != "" && nctx != nil {
		stateMessages, err := loadMessagesFromState(nctx.State, cfg.MessagesFromState)
		if err != nil {
			return nil, fmt.Errorf("tool loop: loading messages from state: %w", err)
		}
		// Prepend state messages before existing messages.
		req.Messages = append(stateMessages, req.Messages...)
	}

	// Build interceptor for API tool introspection if API tools are present.
	var interceptor ToolInterceptor
	if len(cfg.APITools) > 0 && deps.APIToolDispatcher != nil {
		tenantID, _ := nctx.Meta["tenant_id"].(string)
		scopedIDs := uniqueAPIToolIDs(cfg.APITools)
		req.Tools = append(req.Tools, apiToolIntrospectionTools...)
		interceptor = NewAPIToolInterceptor(deps.APIToolDispatcher, tenantID, scopedIDs)
	}

	// Build interceptor for compacted MCP introspection.
	compactedURLs := collectCompactedMCPURLs(routing)
	if len(compactedURLs) > 0 && deps.MCPClientCache != nil {
		req.Tools = append(req.Tools, mcpToolIntrospectionTools...)
		mcpInterceptor := NewMCPToolInterceptor(deps.MCPClientCache, compactedURLs)
		interceptor = ChainInterceptors(interceptor, mcpInterceptor)
	}

	result, err := RunToolLoop(ctx, ToolLoopConfig{
		Provider:      provider,
		Request:       req,
		Routing:       routing,
		MaxCalls:      maxCalls,
		MaxIterations: maxIterations,
		Interceptor:   interceptor,
	}, deps, nctx)
	if err != nil {
		return nil, err
	}

	return buildToolLoopResult(result.Response, result.Messages, result.History, result.Iterations, result.TotalToolCalls, result.FinishReason), nil
}

// RunToolLoop executes a reusable tool loop. It calls the LLM provider,
// dispatches tool calls (checking the optional interceptor first, then MCP),
// feeds results back, and repeats until the LLM produces a final text
// response or limits are hit.
func RunToolLoop(
	ctx context.Context,
	cfg ToolLoopConfig,
	deps *ExecutorDeps,
	nctx *NodeContext,
) (*ToolLoopResult, error) {
	req := cfg.Request
	var history []ToolCallHistoryEntry
	totalToolCalls := 0
	iteration := 0
	var lastResp *model.CompletionResponse

	for {
		// Check context cancellation.
		select {
		case <-ctx.Done():
			return &ToolLoopResult{
				Response: lastResp, Messages: req.Messages, History: history,
				Iterations: iteration, TotalToolCalls: totalToolCalls, FinishReason: "cancelled",
			}, nil
		default:
		}

		// Call the LLM.
		resp, err := cfg.Provider.Complete(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("tool loop: provider call (iteration %d): %w", iteration, err)
		}
		lastResp = resp

		// If no tool calls, we're done.
		if resp.FinishReason != "tool_calls" || len(resp.ToolCalls) == 0 {
			return &ToolLoopResult{
				Response: resp, Messages: req.Messages, History: history,
				Iterations: iteration, TotalToolCalls: totalToolCalls, FinishReason: resp.FinishReason,
			}, nil
		}

		// Check limits before executing tool calls.
		if iteration >= cfg.MaxIterations || totalToolCalls+len(resp.ToolCalls) > cfg.MaxCalls {
			return &ToolLoopResult{
				Response: resp, Messages: req.Messages, History: history,
				Iterations: iteration, TotalToolCalls: totalToolCalls, FinishReason: "limit_reached",
			}, nil
		}

		// Append assistant message with tool calls to the conversation.
		assistantMsg := model.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		}
		req.Messages = append(req.Messages, assistantMsg)

		// Execute each tool call.
		for _, tc := range resp.ToolCalls {
			entry := ToolCallHistoryEntry{
				Name:      tc.Name,
				Arguments: tc.Arguments,
			}
			start := time.Now()

			// Emit tool call started event.
			emitToolEvent(deps, ctx, model.EventToolCallStarted, map[string]any{
				"tool_name": tc.Name,
				"arguments": string(tc.Arguments),
				"iteration": iteration,
			})

			// Check interceptor first (for built-in tools like _task_*, _buffer_*, _memory_*).
			if cfg.Interceptor != nil {
				if result, handled := cfg.Interceptor(tc.Name, tc.Arguments); handled {
					entry.Result = result
					entry.DurationMs = time.Since(start).Milliseconds()
					history = append(history, entry)
					totalToolCalls++

					req.Messages = append(req.Messages, model.Message{
						Role:       "tool",
						ToolCallID: tc.ID,
						Content:    result,
					})

					emitToolEvent(deps, ctx, model.EventToolCallCompleted, map[string]any{
						"tool_name":      tc.Name,
						"result_preview": truncate(result, 500),
						"is_error":       false,
						"duration_ms":    entry.DurationMs,
					})
					continue
				}
			}

			// Normal MCP dispatch.
			route, ok := cfg.Routing[tc.Name]
			if !ok {
				// Unknown tool — feed error back to LLM.
				errMsg := fmt.Sprintf("Error: tool %q is not available. Available tools: %s", tc.Name, availableToolNames(cfg.Routing))
				entry.Result = errMsg
				entry.IsError = true
				entry.DurationMs = time.Since(start).Milliseconds()
				history = append(history, entry)
				totalToolCalls++

				req.Messages = append(req.Messages, model.Message{
					Role:            "tool",
					ToolCallID:      tc.ID,
					Content:         errMsg,
					ToolResultError: true,
				})

				emitToolEvent(deps, ctx, model.EventToolCallCompleted, map[string]any{
					"tool_name":   tc.Name,
					"is_error":    true,
					"duration_ms": entry.DurationMs,
				})
				continue
			}

			// Parse arguments.
			var args map[string]any
			if err := json.Unmarshal(tc.Arguments, &args); err != nil {
				args = map[string]any{"raw": string(tc.Arguments)}
			}

			// Create per-call timeout context.
			timeoutSec := defaultToolTimeoutSec
			if route.TimeoutSeconds != nil {
				timeoutSec = *route.TimeoutSeconds
			}
			callCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)

			var result *model.ToolResult
			if route.APIToolID != "" {
				// API endpoint dispatch.
				tenantID, _ := ctx.Value("tenant_id").(string)
				result, err = deps.APIToolDispatcher.CallEndpoint(callCtx, tenantID, route, tc.Name, args)
			} else {
				// MCP dispatch (existing path).
				headers, headerErr := resolveRouteHeaders(ctx, route, nil, nctx, deps)
				if headerErr != nil {
					cancel()
					return nil, fmt.Errorf("tool loop: resolving headers for tool %q: %w", tc.Name, headerErr)
				}

				if deps.MCPClientCache == nil {
					cancel()
					return nil, fmt.Errorf("tool loop: MCPClientCache is required for tool loop execution")
				}
				client := deps.MCPClientCache.GetOrCreate(route.MCPURL, headers)

				result, err = client.CallTool(callCtx, tc.Name, args)
			}
			cancel()

			entry.DurationMs = time.Since(start).Milliseconds()

			if err != nil {
				entry.Result = fmt.Sprintf("Error: %v", err)
				entry.IsError = true
				history = append(history, entry)
				totalToolCalls++

				req.Messages = append(req.Messages, model.Message{
					Role:            "tool",
					ToolCallID:      tc.ID,
					Content:         entry.Result,
					ToolResultError: true,
				})

				emitToolEvent(deps, ctx, model.EventToolCallCompleted, map[string]any{
					"tool_name":   tc.Name,
					"is_error":    true,
					"duration_ms": entry.DurationMs,
				})
				continue
			}

			// Process result.
			var resultStr string
			if result.IsError {
				resultStr = fmt.Sprintf("Error: %s", result.Error)
				entry.IsError = true
			} else {
				resultBytes, err := json.Marshal(result.Content)
				if err != nil {
					resultStr = fmt.Sprintf("%v", result.Content)
				} else {
					resultStr = string(resultBytes)
				}
			}
			entry.Result = resultStr
			history = append(history, entry)
			totalToolCalls++

			req.Messages = append(req.Messages, model.Message{
				Role:            "tool",
				ToolCallID:      tc.ID,
				Content:         resultStr,
				ToolResultError: result.IsError,
			})

			emitToolEvent(deps, ctx, model.EventToolCallCompleted, map[string]any{
				"tool_name":      tc.Name,
				"result_preview": truncate(resultStr, 500),
				"is_error":       result.IsError,
				"duration_ms":    entry.DurationMs,
			})
		}

		iteration++

		// Emit iteration event.
		emitToolEvent(deps, ctx, model.EventToolLoopIteration, map[string]any{
			"iteration":             iteration,
			"tool_calls_this_round": len(resp.ToolCalls),
			"total_tool_calls":      totalToolCalls,
		})
	}
}

// buildToolLoopResult constructs the output map for a completed tool loop.
func buildToolLoopResult(
	resp *model.CompletionResponse,
	messages []model.Message,
	history []ToolCallHistoryEntry,
	iterations, totalToolCalls int,
	finishReason string,
) *NodeResult {
	outputs := make(map[string]any)

	if resp != nil {
		outputs["response_text"] = resp.Content
		// Try to parse as JSON for response_format: json
		var parsed any
		if json.Unmarshal([]byte(resp.Content), &parsed) == nil {
			outputs["response"] = parsed
		}
	} else {
		outputs["response_text"] = ""
	}

	outputs["finish_reason"] = finishReason
	outputs["messages"] = messages
	outputs["tool_call_history"] = history
	outputs["iterations"] = iterations
	outputs["total_tool_calls"] = totalToolCalls

	return &NodeResult{Outputs: outputs}
}

// resolveToolRouting gets the tool routing map from config, state, or input.
func resolveToolRouting(ctx context.Context, cfg *model.LLMNodeConfig, nctx *NodeContext, deps *ExecutorDeps) (map[string]model.ToolRoute, error) {
	if len(cfg.ToolRouting) > 0 {
		return cfg.ToolRouting, nil
	}

	if cfg.ToolRoutingFromState != "" && nctx != nil {
		val, ok := nctx.State[cfg.ToolRoutingFromState]
		if !ok {
			return nil, fmt.Errorf("state field %q not found for tool_routing_from_state", cfg.ToolRoutingFromState)
		}
		// Marshal and unmarshal to convert to ToolRoute map.
		b, err := json.Marshal(val)
		if err != nil {
			return nil, fmt.Errorf("marshaling tool routing from state: %w", err)
		}
		var routing map[string]model.ToolRoute
		if err := json.Unmarshal(b, &routing); err != nil {
			return nil, fmt.Errorf("unmarshaling tool routing from state: %w", err)
		}
		return routing, nil
	}

	// API tools provide their own routing — an empty MCP routing map is valid.
	if len(cfg.APITools) > 0 {
		return make(map[string]model.ToolRoute), nil
	}

	return nil, fmt.Errorf("no tool routing configured (need tool_routing, tool_routing_from_state, or tool_routing_from_input)")
}

// resolveAPIToolRefs resolves API tool references into LLM tool definitions
// and tool routing entries. Each ref selects a specific endpoint from an API
// tool definition in the library.
func resolveAPIToolRefs(
	ctx context.Context,
	refs []model.APIToolRef,
	dispatcher *APIToolDispatcher,
	tenantID string,
) ([]model.LLMToolDefinition, map[string]model.ToolRoute, error) {
	var tools []model.LLMToolDefinition
	routing := make(map[string]model.ToolRoute)

	for _, ref := range refs {
		def, err := dispatcher.ResolveDefinition(ctx, tenantID, ref.APIToolID)
		if err != nil {
			return nil, nil, fmt.Errorf("resolving API tool ref %q: %w", ref.APIToolID, err)
		}

		ep := FindEndpoint(def, ref.Endpoint)
		if ep == nil {
			return nil, nil, fmt.Errorf("endpoint %q not found in API tool %q", ref.Endpoint, ref.APIToolID)
		}

		// Determine tool name: use override if set, otherwise endpoint name.
		toolName := ep.Name
		if ref.ToolName != "" {
			toolName = ref.ToolName
		}

		// Build LLM tool definition from endpoint schema.
		params := ep.InputSchema
		if len(params) == 0 {
			params = json.RawMessage(`{"type":"object"}`)
		}
		tools = append(tools, model.LLMToolDefinition{
			Name:        toolName,
			Description: ep.Description,
			Parameters:  params,
		})

		// Build routing entry pointing to the API endpoint.
		route := model.ToolRoute{
			APIToolID:   ref.APIToolID,
			APIEndpoint: ref.Endpoint,
			Headers:     ref.Headers,
		}
		routing[toolName] = route
	}

	return tools, routing, nil
}

// autoDiscoverTools queries MCP servers for tool definitions.
func autoDiscoverTools(ctx context.Context, routing map[string]model.ToolRoute, cache *MCPClientCache) ([]model.LLMToolDefinition, error) {
	// Collect unique MCP URLs.
	seen := make(map[string]bool)
	var tools []model.LLMToolDefinition

	for _, route := range routing {
		if route.Compacted {
			continue // compacted URLs use on-demand discovery
		}
		if seen[route.MCPURL] {
			continue
		}
		seen[route.MCPURL] = true

		defs, err := cache.ListToolsCached(ctx, route.MCPURL, nil)
		if err != nil {
			return nil, fmt.Errorf("listing tools from %s: %w", route.MCPURL, err)
		}

		for _, def := range defs {
			// Convert MCP ToolDefinition.InputSchema → LLMToolDefinition.Parameters
			params, err := json.Marshal(def.InputSchema)
			if err != nil {
				params = json.RawMessage(`{"type":"object"}`)
			}
			tools = append(tools, model.LLMToolDefinition{
				Name:        def.Name,
				Description: def.Description,
				Parameters:  params,
			})
		}
	}

	return tools, nil
}

// loadMessagesFromState loads conversation messages from a graph state field.
func loadMessagesFromState(state map[string]any, fieldName string) ([]model.Message, error) {
	val, ok := state[fieldName]
	if !ok {
		return nil, nil // Field not set yet — not an error
	}

	b, err := json.Marshal(val)
	if err != nil {
		return nil, fmt.Errorf("marshaling state field %q: %w", fieldName, err)
	}

	var messages []model.Message
	if err := json.Unmarshal(b, &messages); err != nil {
		return nil, fmt.Errorf("unmarshaling messages from state field %q: %w", fieldName, err)
	}

	return messages, nil
}

// resolveRouteHeaders resolves headers for a ToolRoute.
func resolveRouteHeaders(ctx context.Context, route model.ToolRoute, inputs map[string]any, nctx *NodeContext, deps *ExecutorDeps) (map[string]string, error) {
	headers := make(map[string]string)
	for _, hc := range route.Headers {
		val, err := resolveHeaderValue(ctx, hc, inputs, nctx, deps)
		if err != nil {
			return nil, err
		}
		headers[hc.Name] = val
	}
	return headers, nil
}

// emitToolEvent emits a tool-loop-related event through the event emitter.
func emitToolEvent(deps *ExecutorDeps, ctx context.Context, eventType model.EventType, data map[string]any) {
	if deps == nil || deps.EventEmitter == nil {
		return
	}
	dataJSON, _ := json.Marshal(data)
	execID, _ := ctx.Value("execution_id").(string)
	deps.EventEmitter.Emit(model.ExecutionEvent{
		Type:        eventType,
		ExecutionID: execID,
		Timestamp:   time.Now(),
		Output:      dataJSON,
	})
}

// availableToolNames returns a comma-separated list of available tool names.
func availableToolNames(routing map[string]model.ToolRoute) string {
	names := ""
	for name := range routing {
		if names != "" {
			names += ", "
		}
		names += name
	}
	return names
}

// truncate limits a string to n characters.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

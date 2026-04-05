package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"github.com/brockleyai/brockleyai/engine/codeexec"
	"github.com/brockleyai/brockleyai/engine/executor"
	"github.com/brockleyai/brockleyai/internal/model"
	"github.com/brockleyai/brockleyai/internal/secret"
)

// SuperagentHandler processes node:superagent tasks.
// It stays alive as a coordinator, dispatching every LLM call and MCP call
// as a separate asynq task. Built-in tools (_task_*, _buffer_*, _memory_*)
// are handled locally with zero I/O.
type SuperagentHandler struct {
	store       model.Store
	rdb         *redis.Client
	asynqClient *asynq.Client
	logger      *slog.Logger
	seqCounter  int64 // atomic counter for unique result keys
}

// NewSuperagentHandler creates a new SuperagentHandler.
func NewSuperagentHandler(store model.Store, rdb *redis.Client, asynqClient *asynq.Client, logger *slog.Logger) *SuperagentHandler {
	return &SuperagentHandler{
		store:       store,
		rdb:         rdb,
		asynqClient: asynqClient,
		logger:      logger,
	}
}

// superagentContext holds mutable state during a superagent handler execution.
type superagentContext struct {
	cfg           *model.SuperagentNodeConfig
	task          NodeRunTask
	tasks         *executor.TaskTracker
	buffers       *executor.BufferManager
	memory        *executor.MemoryStore
	dispatcher    *executor.BuiltInToolDispatcher
	messages      []model.Message
	workingMem    map[string]any
	iteration     int
	totalTools    int
	reflections   int
	toolHistory   []executor.ToolCallHistoryEntry
	lastResp      string
	emitter       *RedisEventEmitter
	llmTraces     []model.LLMCallTrace
	mcpToolCache  map[string][]model.ToolDefinition // cached MCP tool listings for compacted introspection
	compactedURLs []string                          // compacted MCP URLs for introspection scoping
	codeExecCount int                               // number of _code_execute calls in this run
}

// ProcessTask handles an asynq task for superagent execution.
func (h *SuperagentHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var t NodeRunTask
	if err := json.Unmarshal(task.Payload(), &t); err != nil {
		return fmt.Errorf("superagent: unmarshal task: %w", err)
	}

	logger := h.logger.With(
		"execution_id", t.ExecutionID,
		"request_id", t.RequestID,
		"node_id", t.NodeID,
	)
	logger.Info("superagent handler started")
	start := time.Now()

	// 1. Parse config.
	var cfg model.SuperagentNodeConfig
	if err := json.Unmarshal(t.NodeConfig, &cfg); err != nil {
		return h.pushFailure(ctx, t, fmt.Sprintf("invalid config: %v", err))
	}

	// 2. Resolve API key.
	apiKey := cfg.APIKey
	if apiKey == "" && cfg.APIKeyRef != "" {
		secretStore := secret.NewEnvSecretStore()
		if key, err := secretStore.GetSecret(ctx, cfg.APIKeyRef); err == nil {
			apiKey = key
		} else {
			return h.pushFailure(ctx, t, fmt.Sprintf("resolving api_key_ref %q: %v", cfg.APIKeyRef, err))
		}
	}

	// 3. Resolve tools from skills via distributed MCP list_tools calls.
	routing := make(map[string]model.ToolRoute)
	var mcpToolDefs []model.LLMToolDefinition
	mcpToolCache := make(map[string][]model.ToolDefinition) // for compacted introspection

	for _, skill := range cfg.Skills {
		// API tool skill handling: resolve endpoints from the API tool definition.
		if skill.APIToolID != "" {
			tenantID, _ := t.Meta["tenant_id"].(string)
			apiDispatcher := executor.NewAPIToolDispatcher(h.store, logger)
			def, err := apiDispatcher.ResolveDefinition(ctx, tenantID, skill.APIToolID)
			if err != nil {
				return h.pushFailure(ctx, t, fmt.Sprintf("resolving API tool for skill %q: %v", skill.Name, err))
			}
			for _, epName := range skill.Endpoints {
				ep := executor.FindEndpoint(def, epName)
				if ep == nil {
					return h.pushFailure(ctx, t, fmt.Sprintf("endpoint %q not found in API tool %q", epName, skill.APIToolID))
				}
				routing[ep.Name] = model.ToolRoute{
					APIToolID:      skill.APIToolID,
					APIEndpoint:    ep.Name,
					Headers:        skill.Headers,
					TimeoutSeconds: skill.TimeoutSeconds,
				}
				mcpToolDefs = append(mcpToolDefs, model.LLMToolDefinition{
					Name:        ep.Name,
					Description: ep.Description,
					Parameters:  ep.InputSchema,
				})
			}
			continue
		}

		headers := resolveSkillHeaders(skill)
		timeout := skill.TimeoutSeconds

		// Dispatch list_tools via MCP call task.
		tools, err := h.listMCPTools(ctx, t.ExecutionID, t.NodeID, skill.MCPURL, headers)
		if err != nil {
			return h.pushFailure(ctx, t, fmt.Sprintf("listing tools for skill %q: %v", skill.Name, err))
		}
		mcpToolCache[skill.MCPURL] = tools // cache for compacted introspection

		// Build policy sets.
		allowSet := make(map[string]bool)
		for _, tn := range skill.Tools {
			allowSet[tn] = true
		}
		deniedSet := make(map[string]bool)
		approvalSet := make(map[string]bool)
		if cfg.ToolPolicies != nil {
			for _, d := range cfg.ToolPolicies.Denied {
				deniedSet[d] = true
			}
			for _, a := range cfg.ToolPolicies.RequireApproval {
				approvalSet[a] = true
			}
		}

		for _, tool := range tools {
			// Compacted mode: skip tools not in explicit allowlist.
			if skill.Compacted && !allowSet[tool.Name] {
				continue
			}
			// Apply allowlist filter (non-compacted mode).
			if !skill.Compacted && len(allowSet) > 0 && !allowSet[tool.Name] {
				continue
			}
			if deniedSet[tool.Name] || approvalSet[tool.Name] {
				continue
			}

			routing[tool.Name] = model.ToolRoute{
				MCPURL:         skill.MCPURL,
				Headers:        skill.Headers,
				TimeoutSeconds: timeout,
			}

			params, err := json.Marshal(tool.InputSchema)
			if err != nil {
				params = json.RawMessage(`{"type":"object"}`)
			}
			mcpToolDefs = append(mcpToolDefs, model.LLMToolDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  params,
			})
		}
	}

	// 4. Get built-in tool definitions and merge.
	builtInDefs := executor.GetBuiltInToolDefinitions(&cfg)
	allTools := append(builtInDefs, mcpToolDefs...)

	// Append MCP introspection tools if any compacted skills are present.
	compactedURLs := executor.CollectCompactedSkillURLs(cfg.Skills)
	if len(compactedURLs) > 0 {
		allTools = append(allTools, executor.MCPToolIntrospectionTools()...)
	}

	// 5. Initialize superagent state.
	namespace := t.NodeID
	if cfg.SharedMemory != nil && cfg.SharedMemory.Namespace != "" {
		namespace = cfg.SharedMemory.Namespace
	}
	taskTracker := executor.NewTaskTracker()
	bufferMgr := executor.NewBufferManager()
	memoryStore := executor.NewMemoryStore(namespace)

	// Load shared memory from state.
	if cfg.SharedMemory != nil && cfg.SharedMemory.Enabled && t.State != nil {
		if memIn, ok := t.State["_memory_in"]; ok {
			if memMap, ok := memIn.(map[string]any); ok {
				memoryStore.LoadFromState(memMap)
			}
		}
	}

	dispatcher := executor.NewBuiltInToolDispatcher(taskTracker, bufferMgr, memoryStore)

	// 6. Assemble system prompt.
	var sharedMemory []executor.MemoryEntry
	injectOnStart := true
	if cfg.SharedMemory != nil && cfg.SharedMemory.InjectOnStart != nil {
		injectOnStart = *cfg.SharedMemory.InjectOnStart
	}
	if cfg.SharedMemory != nil && cfg.SharedMemory.Enabled && injectOnStart {
		sharedMemory = memoryStore.List(nil)
	}

	systemPrompt, err := executor.AssembleSystemPrompt(&cfg, t.Inputs, sharedMemory, make(map[string]any), 0, t.OutputPorts)
	if err != nil {
		return h.pushFailure(ctx, t, fmt.Sprintf("assembling system prompt: %v", err))
	}

	// 7. Build initial messages.
	messages := []model.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: "Start the task now. Use tools if they help, and provide the final answer when you are done."},
	}

	// Load conversation history from input if configured.
	if cfg.ConversationHistoryFromInput != "" {
		if histInput, ok := t.Inputs[cfg.ConversationHistoryFromInput]; ok {
			histBytes, err := json.Marshal(histInput)
			if err == nil {
				var histMsgs []model.Message
				if json.Unmarshal(histBytes, &histMsgs) == nil {
					messages = append(messages, histMsgs...)
				}
			}
		}
	}

	// 8. Create event emitter.
	emitter := &RedisEventEmitter{
		client:      h.rdb,
		executionID: t.ExecutionID,
		logger:      logger,
	}

	sctx := &superagentContext{
		cfg:           &cfg,
		task:          t,
		tasks:         taskTracker,
		buffers:       bufferMgr,
		memory:        memoryStore,
		dispatcher:    dispatcher,
		messages:      messages,
		workingMem:    make(map[string]any),
		emitter:       emitter,
		mcpToolCache:  mcpToolCache,
		compactedURLs: compactedURLs,
	}

	// 9. Run agent loop.
	finishReason, err := h.runAgentLoop(ctx, sctx, apiKey, routing, allTools, logger)
	if err != nil {
		return h.pushFailure(ctx, t, fmt.Sprintf("agent loop: %v", err))
	}

	// 10. Emit completed event.
	h.emitEvent(sctx, model.EventSuperagentCompleted, map[string]any{
		"node_id":          t.NodeID,
		"iterations":       sctx.iteration,
		"total_tool_calls": sctx.totalTools,
		"finish_reason":    finishReason,
	})

	// 11. Resolve outputs.
	outputs, err := h.resolveOutputs(ctx, sctx, apiKey, logger)
	if err != nil {
		return h.pushFailure(ctx, t, fmt.Sprintf("resolving outputs: %v", err))
	}

	// 12. Build meta outputs.
	meta := map[string]any{
		"_conversation_history": sctx.messages,
		"_iterations":           sctx.iteration,
		"_total_tool_calls":     sctx.totalTools,
		"_finish_reason":        finishReason,
		"_tool_call_history":    sctx.toolHistory,
		"_working_memory":       sctx.workingMem,
		"_tasks":                sctx.tasks.List(),
	}
	if cfg.SharedMemory != nil && cfg.SharedMemory.Enabled {
		meta["_memory_out"] = sctx.memory.ToStateValue()
	}
	for k, v := range meta {
		outputs[k] = v
	}

	durationMs := time.Since(start).Milliseconds()
	logger.Info("superagent handler completed",
		"finish_reason", finishReason,
		"iterations", sctx.iteration,
		"total_tool_calls", sctx.totalTools,
		"duration_ms", durationMs,
	)

	return h.pushResult(ctx, t, NodeTaskResult{
		RequestID: t.RequestID,
		NodeID:    t.NodeID,
		Status:    "completed",
		Outputs:   outputs,
		LLMDebug:  superagentDebugTrace(sctx),
	})
}

// --- Agent Loop ---

func (h *SuperagentHandler) runAgentLoop(
	ctx context.Context,
	sctx *superagentContext,
	apiKey string,
	routing map[string]model.ToolRoute,
	allTools []model.LLMToolDefinition,
	logger *slog.Logger,
) (string, error) {
	cfg := sctx.cfg

	// Apply timeout.
	timeoutSec := executor.IntOrDefault(cfg.TimeoutSeconds, 600)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	// Initialize stuck detector.
	windowSize := 20
	repeatThreshold := 3
	if cfg.Overrides != nil && cfg.Overrides.StuckDetection != nil {
		sd := cfg.Overrides.StuckDetection
		if sd.WindowSize != nil {
			windowSize = *sd.WindowSize
		}
		if sd.RepeatThreshold != nil {
			repeatThreshold = *sd.RepeatThreshold
		}
	}
	stuckDetector := executor.NewStuckDetector(windowSize, repeatThreshold)

	// Emit started event.
	h.emitEvent(sctx, model.EventSuperagentStarted, map[string]any{
		"node_id":    sctx.task.NodeID,
		"skills":     len(cfg.Skills),
		"tool_count": len(allTools),
	})

	// Determine limits.
	maxIterations := executor.IntOrDefault(cfg.MaxIterations, 25)
	maxTotalCalls := executor.IntOrDefault(cfg.MaxTotalToolCalls, 200)
	maxReflections := 3
	if cfg.Overrides != nil && cfg.Overrides.Reflection != nil && cfg.Overrides.Reflection.MaxReflections != nil {
		maxReflections = *cfg.Overrides.Reflection.MaxReflections
	}
	evaluatorDisabled := cfg.Overrides != nil && cfg.Overrides.Evaluator != nil && cfg.Overrides.Evaluator.Disabled

	maxCallsPerIter := executor.IntOrDefault(cfg.MaxToolCallsPerIteration, 25)

	for sctx.iteration = 0; sctx.iteration < maxIterations; sctx.iteration++ {
		// Check context.
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return "timeout", nil
			}
			return "cancelled", nil
		default:
		}

		// Inject task reminder.
		taskReminderEnabled := true
		if cfg.Overrides != nil && cfg.Overrides.TaskTracking != nil &&
			cfg.Overrides.TaskTracking.Enabled != nil && !*cfg.Overrides.TaskTracking.Enabled {
			taskReminderEnabled = false
		}
		if taskReminderEnabled && len(sctx.tasks.List()) > 0 && sctx.iteration > 0 {
			reminder := sctx.tasks.BuildTaskReminder()
			if reminder != "" {
				sctx.messages = append(sctx.messages, model.Message{
					Role:    "system",
					Content: "Task Status Update:\n" + reminder,
				})
			}
		}

		// Inner tool loop.
		iterToolCalls := 0
		for {
			// Dispatch LLM call.
			resp, err := h.callLLM(ctx, sctx, apiKey, &model.CompletionRequest{
				APIKey:      apiKey,
				Model:       cfg.Model,
				BaseURL:     cfg.BaseURL,
				Messages:    sctx.messages,
				Temperature: cfg.Temperature,
				MaxTokens:   cfg.MaxTokens,
				Tools:       allTools,
				ToolChoice:  "auto",
			})
			if err != nil {
				if ctx.Err() != nil {
					if ctx.Err() == context.DeadlineExceeded {
						return "timeout", nil
					}
					return "cancelled", nil
				}
				return "", fmt.Errorf("llm call at iteration %d: %w", sctx.iteration, err)
			}

			// No tool calls — text response, end inner loop.
			if resp.FinishReason != "tool_calls" || len(resp.ToolCalls) == 0 {
				sctx.messages = append(sctx.messages, model.Message{
					Role:    "assistant",
					Content: resp.Content,
				})
				sctx.lastResp = resp.Content
				break
			}

			// Append assistant message with tool calls.
			sctx.messages = append(sctx.messages, model.Message{
				Role:      "assistant",
				Content:   resp.Content,
				ToolCalls: resp.ToolCalls,
			})

			// Process each tool call.
			for _, tc := range resp.ToolCalls {
				iterToolCalls++

				// Try built-in tool / MCP introspection first.
				interceptor := sctx.dispatcher.AsInterceptor()
				if len(sctx.compactedURLs) > 0 && sctx.mcpToolCache != nil {
					mcpInterceptor := executor.NewMCPToolInterceptorFromCache(sctx.mcpToolCache, sctx.compactedURLs)
					interceptor = executor.ChainInterceptors(interceptor, mcpInterceptor)
				}
				if result, handled := interceptor(tc.Name, tc.Arguments); handled {
					sctx.messages = append(sctx.messages, model.Message{
						Role:       "tool",
						ToolCallID: tc.ID,
						Content:    result,
					})
					sctx.toolHistory = append(sctx.toolHistory, executor.ToolCallHistoryEntry{
						Name:      tc.Name,
						Arguments: tc.Arguments,
						Result:    result,
					})
					continue
				}

				// Code execution tools — handled locally.
				if executor.IsCodeExecTool(tc.Name) {
					result, codeErr := h.handleCodeExecTool(ctx, sctx, routing, tc.Name, tc.Arguments, logger)
					isError := codeErr != nil
					if isError {
						result = fmt.Sprintf(`{"error": %q}`, codeErr.Error())
					}
					sctx.messages = append(sctx.messages, model.Message{
						Role:            "tool",
						ToolCallID:      tc.ID,
						Content:         result,
						ToolResultError: isError,
					})
					sctx.toolHistory = append(sctx.toolHistory, executor.ToolCallHistoryEntry{
						Name:      tc.Name,
						Arguments: tc.Arguments,
						Result:    result,
						IsError:   isError,
					})
					continue
				}

				// MCP tool call.
				route, ok := routing[tc.Name]
				if !ok {
					errMsg := fmt.Sprintf("Error: tool %q is not available", tc.Name)
					sctx.messages = append(sctx.messages, model.Message{
						Role:            "tool",
						ToolCallID:      tc.ID,
						Content:         errMsg,
						ToolResultError: true,
					})
					sctx.toolHistory = append(sctx.toolHistory, executor.ToolCallHistoryEntry{
						Name:      tc.Name,
						Arguments: tc.Arguments,
						Result:    errMsg,
						IsError:   true,
					})
					continue
				}

				if route.APIToolID != "" {
					// API endpoint dispatch.
					result, isError, apiDuration, err := h.callAPITool(ctx, sctx, route, tc.ID, tc.Name, tc.Arguments)
					if err != nil {
						if ctx.Err() != nil {
							if ctx.Err() == context.DeadlineExceeded {
								return "timeout", nil
							}
							return "cancelled", nil
						}
						return "", fmt.Errorf("api call for tool %q: %w", tc.Name, err)
					}

					sctx.messages = append(sctx.messages, model.Message{
						Role:            "tool",
						ToolCallID:      tc.ID,
						Content:         result,
						ToolResultError: isError,
					})
					sctx.toolHistory = append(sctx.toolHistory, executor.ToolCallHistoryEntry{
						Name:       tc.Name,
						Arguments:  tc.Arguments,
						Result:     result,
						DurationMs: apiDuration,
						IsError:    isError,
					})
					sctx.totalTools++
				} else {
					// MCP tool dispatch.
					result, isError, mcpDuration, err := h.callMCPTool(ctx, sctx, route, tc.ID, tc.Name, tc.Arguments)
					if err != nil {
						if ctx.Err() != nil {
							if ctx.Err() == context.DeadlineExceeded {
								return "timeout", nil
							}
							return "cancelled", nil
						}
						return "", fmt.Errorf("mcp call for tool %q: %w", tc.Name, err)
					}

					sctx.messages = append(sctx.messages, model.Message{
						Role:            "tool",
						ToolCallID:      tc.ID,
						Content:         result,
						ToolResultError: isError,
					})
					sctx.toolHistory = append(sctx.toolHistory, executor.ToolCallHistoryEntry{
						Name:       tc.Name,
						Arguments:  tc.Arguments,
						Result:     result,
						DurationMs: mcpDuration,
						IsError:    isError,
					})
					sctx.totalTools++
				}
			}

			// Check per-iteration tool call limit.
			if iterToolCalls >= maxCallsPerIter {
				break
			}

			// Check total tool call limit.
			if sctx.totalTools >= maxTotalCalls {
				return "max_tool_calls", nil
			}
		}

		// Check max total tool calls after inner loop.
		if sctx.totalTools >= maxTotalCalls {
			return "max_tool_calls", nil
		}

		// Stuck detection on new tool calls.
		for _, entry := range sctx.toolHistory[len(sctx.toolHistory)-iterToolCalls:] {
			level := stuckDetector.Record(entry.Name, entry.Arguments)
			switch level {
			case executor.StuckWarn:
				sctx.messages = append(sctx.messages, model.Message{
					Role:    "system",
					Content: "WARNING: You appear to be repeating the same tool calls. Try a different approach or tool. If you believe you're done, report your findings.",
				})
				h.emitEvent(sctx, model.EventSuperagentStuckWarning, map[string]any{
					"node_id":   sctx.task.NodeID,
					"iteration": sctx.iteration,
					"tool_name": entry.Name,
				})
			case executor.StuckReflect:
				if err := h.reflect(ctx, sctx, apiKey); err != nil {
					return "", fmt.Errorf("reflection at iteration %d: %w", sctx.iteration, err)
				}
				stuckDetector.Reset()
			case executor.StuckForceExit:
				return "stuck", nil
			}
		}

		// When evaluator is disabled and the LLM returned a text response
		// (no tool calls this iteration), the agent is done.
		if evaluatorDisabled && iterToolCalls == 0 {
			return "done", nil
		}

		// Evaluator.
		if !evaluatorDisabled {
			evalResult, err := h.evaluate(ctx, sctx, apiKey)
			if err != nil {
				if ctx.Err() != nil {
					if ctx.Err() == context.DeadlineExceeded {
						return "timeout", nil
					}
					return "cancelled", nil
				}
				// Non-fatal: continue on evaluator error.
			} else {
				if !evalResult.NeedsMoreWork {
					return "done", nil
				}
				if evalResult.StuckDetected {
					if sctx.reflections >= maxReflections {
						return "stuck", nil
					}
					if err := h.reflect(ctx, sctx, apiKey); err != nil {
						return "", fmt.Errorf("reflection at iteration %d: %w", sctx.iteration, err)
					}
					stuckDetector.Reset()
				}
				if evalResult.ShouldCompact {
					if err := h.compactContext(ctx, sctx, apiKey); err != nil {
						_ = err // Non-fatal
					}
				}
			}

			// Proactive compaction check.
			contextWindowLimit := 128000
			compactionThreshold := 0.75
			if cfg.Overrides != nil && cfg.Overrides.ContextCompaction != nil {
				if cfg.Overrides.ContextCompaction.ContextWindowLimit != nil {
					contextWindowLimit = *cfg.Overrides.ContextCompaction.ContextWindowLimit
				}
				if cfg.Overrides.ContextCompaction.CompactionThreshold != nil {
					compactionThreshold = *cfg.Overrides.ContextCompaction.CompactionThreshold
				}
			}
			if estimateTokens(sctx.messages) > int(float64(contextWindowLimit)*compactionThreshold) {
				if err := h.compactContext(ctx, sctx, apiKey); err != nil {
					_ = err // Non-fatal
				}
			}
		}

		// Check reflection count.
		if sctx.reflections > maxReflections {
			return "stuck", nil
		}

		// Emit iteration event.
		h.emitEvent(sctx, model.EventSuperagentIteration, map[string]any{
			"node_id":          sctx.task.NodeID,
			"iteration":        sctx.iteration,
			"total_tool_calls": sctx.totalTools,
		})
	}

	return "max_iterations", nil
}

// estimateTokens provides a rough token count based on character count.
func estimateTokens(messages []model.Message) int {
	total := 0
	for _, m := range messages {
		total += len(m.Content) / 4
	}
	return total
}

// --- Evaluator ---

func (h *SuperagentHandler) evaluate(
	ctx context.Context,
	sctx *superagentContext,
	apiKey string,
) (*executor.EvaluatorResult, error) {
	cfg := sctx.cfg

	// Resolve override provider/model.
	provider := string(cfg.Provider)
	evalModel := cfg.Model
	evalKey := apiKey
	if cfg.Overrides != nil && cfg.Overrides.Evaluator != nil {
		ov := cfg.Overrides.Evaluator
		if ov.Provider != "" && ov.Model != "" {
			provider = string(ov.Provider)
			evalModel = ov.Model
		}
		if ov.APIKey != "" {
			evalKey = ov.APIKey
		} else if ov.APIKeyRef != "" {
			if key, err := secret.NewEnvSecretStore().GetSecret(ctx, ov.APIKeyRef); err == nil {
				evalKey = key
			}
		}
	}

	// Build evaluator prompt.
	prompt := executor.DefaultEvaluatorPrompt
	if cfg.Overrides != nil && cfg.Overrides.Evaluator != nil && cfg.Overrides.Evaluator.Prompt != "" {
		prompt = cfg.Overrides.Evaluator.Prompt
	}

	var outReqs []string
	for _, port := range sctx.task.OutputPorts {
		if !strings.HasPrefix(port.Name, "_") {
			outReqs = append(outReqs, fmt.Sprintf("- %s (schema: %s)", port.Name, string(port.Schema)))
		}
	}
	outputRequirements := strings.Join(outReqs, "\n")

	planStr, _ := sctx.workingMem["plan"].(string)
	prompt = strings.ReplaceAll(prompt, "{{task}}", cfg.Prompt)
	prompt = strings.ReplaceAll(prompt, "{{output_requirements}}", outputRequirements)
	prompt = strings.ReplaceAll(prompt, "{{task_list}}", sctx.tasks.BuildTaskReminder())
	prompt = strings.ReplaceAll(prompt, "{{iteration}}", fmt.Sprintf("%d", sctx.iteration))
	prompt = strings.ReplaceAll(prompt, "{{plan}}", planStr)

	msgs := make([]model.Message, len(sctx.messages))
	copy(msgs, sctx.messages)
	msgs = append(msgs, model.Message{
		Role:    "user",
		Content: prompt,
	})

	resp, err := h.callLLMDirect(ctx, sctx, evalKey, provider, evalModel, &model.CompletionRequest{
		APIKey:         evalKey,
		Model:          evalModel,
		BaseURL:        cfg.BaseURL,
		Messages:       msgs,
		ResponseFormat: model.ResponseFormatJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("evaluator LLM call: %w", err)
	}

	var result executor.EvaluatorResult
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		return &executor.EvaluatorResult{NeedsMoreWork: true, Reasoning: "evaluator response unparseable"}, nil
	}

	h.emitEvent(sctx, model.EventSuperagentEvaluation, map[string]any{
		"node_id":         sctx.task.NodeID,
		"iteration":       sctx.iteration,
		"needs_more_work": result.NeedsMoreWork,
		"stuck_detected":  result.StuckDetected,
		"should_compact":  result.ShouldCompact,
		"reasoning":       result.Reasoning,
	})

	return &result, nil
}

// --- Reflection ---

func (h *SuperagentHandler) reflect(
	ctx context.Context,
	sctx *superagentContext,
	apiKey string,
) error {
	cfg := sctx.cfg

	provider := string(cfg.Provider)
	refModel := cfg.Model
	refKey := apiKey
	if cfg.Overrides != nil && cfg.Overrides.Reflection != nil {
		ov := cfg.Overrides.Reflection
		if ov.Provider != "" && ov.Model != "" {
			provider = string(ov.Provider)
			refModel = ov.Model
		}
		if ov.APIKey != "" {
			refKey = ov.APIKey
		} else if ov.APIKeyRef != "" {
			if key, err := secret.NewEnvSecretStore().GetSecret(ctx, ov.APIKeyRef); err == nil {
				refKey = key
			}
		}
	}

	prompt := executor.DefaultReflectionPrompt
	if cfg.Overrides != nil && cfg.Overrides.Reflection != nil && cfg.Overrides.Reflection.Prompt != "" {
		prompt = cfg.Overrides.Reflection.Prompt
	}

	planStr, _ := sctx.workingMem["plan"].(string)
	prompt = strings.ReplaceAll(prompt, "{{task}}", cfg.Prompt)
	prompt = strings.ReplaceAll(prompt, "{{task_list}}", sctx.tasks.BuildTaskReminder())
	prompt = strings.ReplaceAll(prompt, "{{plan}}", planStr)
	prompt = strings.ReplaceAll(prompt, "{{iteration}}", fmt.Sprintf("%d", sctx.iteration))

	msgs := make([]model.Message, len(sctx.messages))
	copy(msgs, sctx.messages)
	msgs = append(msgs, model.Message{
		Role:    "user",
		Content: prompt,
	})

	resp, err := h.callLLMDirect(ctx, sctx, refKey, provider, refModel, &model.CompletionRequest{
		APIKey:         refKey,
		Model:          refModel,
		BaseURL:        cfg.BaseURL,
		Messages:       msgs,
		ResponseFormat: model.ResponseFormatJSON,
	})
	if err != nil {
		return fmt.Errorf("reflection LLM call: %w", err)
	}

	var result executor.ReflectionResult
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		result.ReflectionText = resp.Content
	}

	sctx.messages = append(sctx.messages, model.Message{
		Role:    "system",
		Content: fmt.Sprintf("[Reflection] %s\n\nRevised Plan: %s", result.ReflectionText, result.NewPlan),
	})

	if result.NewPlan != "" {
		sctx.workingMem["plan"] = result.NewPlan
	}

	sctx.reflections++

	h.emitEvent(sctx, model.EventSuperagentReflection, map[string]any{
		"node_id":          sctx.task.NodeID,
		"iteration":        sctx.iteration,
		"reflection_count": sctx.reflections,
		"reflection_text":  result.ReflectionText,
		"new_plan":         result.NewPlan,
	})

	return nil
}

// --- Context Compaction ---

func (h *SuperagentHandler) compactContext(
	ctx context.Context,
	sctx *superagentContext,
	apiKey string,
) error {
	cfg := sctx.cfg
	messagesBefore := len(sctx.messages)

	provider := string(cfg.Provider)
	compModel := cfg.Model
	compKey := apiKey
	if cfg.Overrides != nil && cfg.Overrides.ContextCompaction != nil {
		ov := cfg.Overrides.ContextCompaction
		if ov.Provider != "" && ov.Model != "" {
			provider = string(ov.Provider)
			compModel = ov.Model
		}
		if ov.APIKey != "" {
			compKey = ov.APIKey
		} else if ov.APIKeyRef != "" {
			if key, err := secret.NewEnvSecretStore().GetSecret(ctx, ov.APIKeyRef); err == nil {
				compKey = key
			}
		}
	}

	memoriesFlushed := 0

	// Step 1: Memory flush (if auto_flush enabled).
	if cfg.SharedMemory != nil && cfg.SharedMemory.AutoFlush != nil && *cfg.SharedMemory.AutoFlush {
		flushMsgs := make([]model.Message, len(sctx.messages))
		copy(flushMsgs, sctx.messages)
		flushMsgs = append(flushMsgs, model.Message{
			Role:    "user",
			Content: executor.DefaultMemoryFlushPrompt,
		})

		flushResp, err := h.callLLMDirect(ctx, sctx, compKey, provider, compModel, &model.CompletionRequest{
			APIKey:         compKey,
			Model:          compModel,
			BaseURL:        cfg.BaseURL,
			Messages:       flushMsgs,
			ResponseFormat: model.ResponseFormatJSON,
		})
		if err == nil {
			var facts []struct {
				Key     string   `json:"key"`
				Content string   `json:"content"`
				Tags    []string `json:"tags"`
			}
			if json.Unmarshal([]byte(flushResp.Content), &facts) == nil {
				for _, fact := range facts {
					sctx.memory.Store(fact.Key, fact.Content, fact.Tags)
					memoriesFlushed++
				}
			}
		}
	}

	// Step 2: Summarize conversation.
	compactionPrompt := executor.DefaultCompactionPrompt
	if cfg.Overrides != nil && cfg.Overrides.ContextCompaction != nil && cfg.Overrides.ContextCompaction.Prompt != "" {
		compactionPrompt = cfg.Overrides.ContextCompaction.Prompt
	}

	compMsgs := make([]model.Message, len(sctx.messages))
	copy(compMsgs, sctx.messages)
	compMsgs = append(compMsgs, model.Message{
		Role:    "user",
		Content: compactionPrompt,
	})

	summaryResp, err := h.callLLMDirect(ctx, sctx, compKey, provider, compModel, &model.CompletionRequest{
		APIKey:   compKey,
		Model:    compModel,
		BaseURL:  cfg.BaseURL,
		Messages: compMsgs,
	})
	if err != nil {
		return fmt.Errorf("compaction summary LLM call: %w", err)
	}
	summary := summaryResp.Content

	// Step 3: Reconstruct context.
	var sharedMem []executor.MemoryEntry
	if cfg.SharedMemory != nil && cfg.SharedMemory.Enabled {
		sharedMem = sctx.memory.List(nil)
	}

	systemPrompt, err := executor.AssembleSystemPrompt(cfg, sctx.task.Inputs, sharedMem, sctx.workingMem, sctx.iteration, sctx.task.OutputPorts)
	if err != nil {
		return fmt.Errorf("compaction reassemble prompt: %w", err)
	}

	preserveRecent := 5
	if cfg.Overrides != nil && cfg.Overrides.ContextCompaction != nil && cfg.Overrides.ContextCompaction.PreserveRecentMessages != nil {
		preserveRecent = *cfg.Overrides.ContextCompaction.PreserveRecentMessages
	}

	newMessages := []model.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "system", Content: "[Context Summary from previous conversation]\n" + summary},
	}
	if preserveRecent > 0 && len(sctx.messages) > preserveRecent {
		newMessages = append(newMessages, sctx.messages[len(sctx.messages)-preserveRecent:]...)
	} else if preserveRecent > 0 && len(sctx.messages) > 1 {
		newMessages = append(newMessages, sctx.messages[1:]...)
	}
	sctx.messages = newMessages

	h.emitEvent(sctx, model.EventSuperagentCompaction, map[string]any{
		"node_id":          sctx.task.NodeID,
		"iteration":        sctx.iteration,
		"messages_before":  messagesBefore,
		"messages_after":   len(sctx.messages),
		"memories_flushed": memoriesFlushed,
	})

	return nil
}

// --- Output Resolution ---

func (h *SuperagentHandler) resolveOutputs(
	ctx context.Context,
	sctx *superagentContext,
	apiKey string,
	logger *slog.Logger,
) (map[string]any, error) {
	outputs := make(map[string]any)
	finalizedBuffers := sctx.buffers.GetFinalizedBuffers()
	responseText := sctx.lastResp
	if strings.TrimSpace(responseText) == "" {
		responseText = lastNonEmptyAssistantMessage(sctx.messages)
	}

	var nonMetaPorts []model.Port
	for _, port := range sctx.task.OutputPorts {
		if !strings.HasPrefix(port.Name, "_") {
			nonMetaPorts = append(nonMetaPorts, port)
		}
	}

	for _, port := range nonMetaPorts {
		// Priority 1: Finalized buffer.
		if content, ok := finalizedBuffers[port.Name]; ok {
			outputs[port.Name] = content
			continue
		}

		// Priority 2: Single string port with a non-empty final response.
		if len(nonMetaPorts) == 1 && executor.IsStringSchema(port.Schema) && strings.TrimSpace(responseText) != "" {
			outputs[port.Name] = responseText
			continue
		}

		// Priority 3: Extraction LLM.
		extracted, err := h.extractOutput(ctx, sctx, port, apiKey)
		if err != nil {
			return nil, fmt.Errorf("extracting output for port %q: %w", port.Name, err)
		}
		outputs[port.Name] = extracted
	}

	return outputs, nil
}

func lastNonEmptyAssistantMessage(messages []model.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role == "assistant" && strings.TrimSpace(msg.Content) != "" {
			return msg.Content
		}
	}
	return ""
}

func (h *SuperagentHandler) extractOutput(
	ctx context.Context,
	sctx *superagentContext,
	port model.Port,
	apiKey string,
) (any, error) {
	cfg := sctx.cfg
	provider := string(cfg.Provider)
	extractModel := cfg.Model
	extractKey := apiKey

	if cfg.Overrides != nil && cfg.Overrides.OutputExtraction != nil {
		oe := cfg.Overrides.OutputExtraction
		if oe.Provider != "" && oe.Model != "" {
			provider = string(oe.Provider)
			extractModel = oe.Model
		}
		if oe.APIKey != "" {
			extractKey = oe.APIKey
		} else if oe.APIKeyRef != "" {
			if key, err := secret.NewEnvSecretStore().GetSecret(ctx, oe.APIKeyRef); err == nil {
				extractKey = key
			}
		}
	}

	schemaStr := string(port.Schema)
	extractionPrompt := strings.ReplaceAll(executor.DefaultExtractionPrompt, "{{schema}}", schemaStr)

	msgs := make([]model.Message, len(sctx.messages))
	copy(msgs, sctx.messages)
	msgs = append(msgs, model.Message{
		Role:    "user",
		Content: extractionPrompt,
	})

	resp, err := h.callLLMDirect(ctx, sctx, extractKey, provider, extractModel, &model.CompletionRequest{
		APIKey:         extractKey,
		Model:          extractModel,
		BaseURL:        cfg.BaseURL,
		Messages:       msgs,
		ResponseFormat: model.ResponseFormatJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("extraction LLM call: %w", err)
	}

	var parsed any
	if err := json.Unmarshal([]byte(resp.Content), &parsed); err != nil {
		return resp.Content, nil
	}
	return parsed, nil
}

// --- Distributed LLM and MCP Calls ---

// callLLM dispatches a node:llm-call task and waits for the result via BRPOP.
// Uses the primary provider from the superagent config.
func (h *SuperagentHandler) callLLM(
	ctx context.Context,
	sctx *superagentContext,
	apiKey string,
	req *model.CompletionRequest,
) (*model.CompletionResponse, error) {
	return h.callLLMDirect(ctx, sctx, apiKey, string(sctx.cfg.Provider), sctx.cfg.Model, req)
}

// callLLMDirect dispatches a node:llm-call task with an explicit provider/model.
func (h *SuperagentHandler) callLLMDirect(
	ctx context.Context,
	sctx *superagentContext,
	apiKey string,
	providerName string,
	modelName string,
	req *model.CompletionRequest,
) (*model.CompletionResponse, error) {
	seq := atomic.AddInt64(&h.seqCounter, 1)
	resultKey := ResultKeyForSuperagent(sctx.task.ExecutionID, sctx.task.NodeID, seq)
	requestID := fmt.Sprintf("%s_sa_llm_%d", sctx.task.NodeID, seq)

	llmTask := LLMCallTask{
		ExecutionID: sctx.task.ExecutionID,
		RequestID:   requestID,
		NodeID:      sctx.task.NodeID,
		Provider:    providerName,
		APIKey:      apiKey,
		Request:     req,
		ResultKey:   resultKey,
		Debug:       sctx.task.Debug,
	}

	payload, err := json.Marshal(llmTask)
	if err != nil {
		return nil, fmt.Errorf("marshal llm task: %w", err)
	}

	asynqTask := asynq.NewTask(TaskTypeLLMCall, payload, asynq.Queue(QueueNodes))
	if _, err := h.asynqClient.Enqueue(asynqTask); err != nil {
		return nil, fmt.Errorf("enqueue llm call: %w", err)
	}

	// BRPOP with timeout derived from context deadline.
	brpopTimeout := 5 * time.Minute
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 {
			brpopTimeout = remaining + 10*time.Second
		}
	}

	result, err := h.rdb.BRPop(ctx, brpopTimeout, resultKey).Result()
	if err != nil {
		return nil, fmt.Errorf("brpop llm result: %w", err)
	}
	if len(result) < 2 {
		return nil, fmt.Errorf("brpop llm result: unexpected length")
	}

	// Clean up result key.
	h.rdb.Del(ctx, resultKey)

	// Parse NodeTaskResult.
	var nodeResult NodeTaskResult
	if err := json.Unmarshal([]byte(result[1]), &nodeResult); err != nil {
		return nil, fmt.Errorf("unmarshal llm result: %w", err)
	}
	if nodeResult.Status == "failed" {
		return nil, fmt.Errorf("llm call failed: %s", nodeResult.Error)
	}
	if nodeResult.LLMDebug != nil {
		sctx.llmTraces = append(sctx.llmTraces, nodeResult.LLMDebug.Calls...)
	}

	// Reconstruct CompletionResponse from outputs.
	resp := &model.CompletionResponse{
		Content: fmt.Sprintf("%v", nodeResult.Outputs["response_text"]),
	}
	if fr, ok := nodeResult.Outputs["finish_reason"].(string); ok {
		resp.FinishReason = fr
	}
	if tcs, ok := nodeResult.Outputs["tool_calls"]; ok {
		tcBytes, err := json.Marshal(tcs)
		if err == nil {
			var toolCalls []model.ToolCall
			if json.Unmarshal(tcBytes, &toolCalls) == nil {
				resp.ToolCalls = toolCalls
			}
		}
	}

	return resp, nil
}

// callMCPTool dispatches a node:mcp-call task and waits for the result via BRPOP.
func (h *SuperagentHandler) callMCPTool(
	ctx context.Context,
	sctx *superagentContext,
	route model.ToolRoute,
	toolCallID string,
	toolName string,
	args json.RawMessage,
) (string, bool, int64, error) {
	seq := atomic.AddInt64(&h.seqCounter, 1)
	resultKey := ResultKeyForSuperagent(sctx.task.ExecutionID, sctx.task.NodeID, seq)

	// Parse arguments.
	var parsedArgs map[string]any
	if err := json.Unmarshal(args, &parsedArgs); err != nil {
		parsedArgs = map[string]any{"raw": string(args)}
	}

	// Resolve headers.
	headers := resolveRouteHeadersStatic(route)

	timeoutSec := 30
	if route.TimeoutSeconds != nil {
		timeoutSec = *route.TimeoutSeconds
	}

	mcpTask := MCPCallTask{
		ExecutionID:    sctx.task.ExecutionID,
		RequestID:      toolCallID,
		NodeID:         sctx.task.NodeID,
		Operation:      "call_tool",
		MCPURL:         route.MCPURL,
		Headers:        headers,
		ToolName:       toolName,
		Arguments:      parsedArgs,
		TimeoutSeconds: timeoutSec,
		ResultKey:      resultKey,
		ForToolLoop:    true,
	}

	payload, err := json.Marshal(mcpTask)
	if err != nil {
		return "", false, 0, fmt.Errorf("marshal mcp task: %w", err)
	}

	asynqTask := asynq.NewTask(TaskTypeMCPCall, payload, asynq.Queue(QueueNodes))
	if _, err := h.asynqClient.Enqueue(asynqTask); err != nil {
		return "", false, 0, fmt.Errorf("enqueue mcp call: %w", err)
	}

	// BRPOP with timeout.
	brpopTimeout := 5 * time.Minute
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 {
			brpopTimeout = remaining + 10*time.Second
		}
	}

	result, err := h.rdb.BRPop(ctx, brpopTimeout, resultKey).Result()
	if err != nil {
		return "", false, 0, fmt.Errorf("brpop mcp result: %w", err)
	}
	if len(result) < 2 {
		return "", false, 0, fmt.Errorf("brpop mcp result: unexpected length")
	}

	// Clean up result key.
	h.rdb.Del(ctx, resultKey)

	// Parse MCPCallResult.
	var mcpResult MCPCallResult
	if err := json.Unmarshal([]byte(result[1]), &mcpResult); err != nil {
		return "", false, 0, fmt.Errorf("unmarshal mcp result: %w", err)
	}

	if mcpResult.IsError {
		errStr := mcpResult.Error
		if errStr == "" {
			errStr = "unknown MCP error"
		}
		return errStr, true, mcpResult.DurationMs, nil
	}

	// Serialize content.
	var resultStr string
	if mcpResult.Content != nil {
		resultBytes, err := json.Marshal(mcpResult.Content)
		if err != nil {
			resultStr = fmt.Sprintf("%v", mcpResult.Content)
		} else {
			resultStr = string(resultBytes)
		}
	}

	return resultStr, false, mcpResult.DurationMs, nil
}

// callAPITool dispatches a node:api-call task and waits for the result via BRPOP.
func (h *SuperagentHandler) callAPITool(
	ctx context.Context,
	sctx *superagentContext,
	route model.ToolRoute,
	toolCallID string,
	toolName string,
	args json.RawMessage,
) (string, bool, int64, error) {
	seq := atomic.AddInt64(&h.seqCounter, 1)
	resultKey := ResultKeyForSuperagent(sctx.task.ExecutionID, sctx.task.NodeID, seq)

	// Parse arguments.
	var parsedArgs map[string]any
	if err := json.Unmarshal(args, &parsedArgs); err != nil {
		parsedArgs = map[string]any{"raw": string(args)}
	}

	timeoutSec := 30
	if route.TimeoutSeconds != nil {
		timeoutSec = *route.TimeoutSeconds
	}

	tenantID, _ := sctx.task.Meta["tenant_id"].(string)

	apiTask := APICallTask{
		ExecutionID:    sctx.task.ExecutionID,
		RequestID:      toolCallID,
		NodeID:         sctx.task.NodeID,
		TenantID:       tenantID,
		APIToolID:      route.APIToolID,
		APIEndpoint:    route.APIEndpoint,
		Headers:        route.Headers,
		ToolName:       toolName,
		Arguments:      parsedArgs,
		TimeoutSeconds: timeoutSec,
		ResultKey:      resultKey,
		ForToolLoop:    true,
	}

	payload, err := json.Marshal(apiTask)
	if err != nil {
		return "", false, 0, fmt.Errorf("marshal api task: %w", err)
	}

	asynqTask := asynq.NewTask(TaskTypeAPICall, payload, asynq.Queue(QueueNodes))
	if _, err := h.asynqClient.Enqueue(asynqTask); err != nil {
		return "", false, 0, fmt.Errorf("enqueue api call: %w", err)
	}

	// BRPOP with timeout.
	brpopTimeout := 5 * time.Minute
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 {
			brpopTimeout = remaining + 10*time.Second
		}
	}

	result, err := h.rdb.BRPop(ctx, brpopTimeout, resultKey).Result()
	if err != nil {
		return "", false, 0, fmt.Errorf("brpop api result: %w", err)
	}
	if len(result) < 2 {
		return "", false, 0, fmt.Errorf("brpop api result: unexpected length")
	}

	// Clean up result key.
	h.rdb.Del(ctx, resultKey)

	// Parse APICallResult.
	var apiResult APICallResult
	if err := json.Unmarshal([]byte(result[1]), &apiResult); err != nil {
		return "", false, 0, fmt.Errorf("unmarshal api result: %w", err)
	}

	if apiResult.IsError {
		errStr := apiResult.Error
		if errStr == "" {
			errStr = "unknown API tool error"
		}
		return errStr, true, apiResult.DurationMs, nil
	}

	// Serialize content.
	var resultStr string
	if apiResult.Content != nil {
		resultBytes, err := json.Marshal(apiResult.Content)
		if err != nil {
			resultStr = fmt.Sprintf("%v", apiResult.Content)
		} else {
			resultStr = string(resultBytes)
		}
	}

	return resultStr, false, apiResult.DurationMs, nil
}

// listMCPTools dispatches a list_tools MCP call and returns the tool definitions.
func (h *SuperagentHandler) listMCPTools(
	ctx context.Context,
	executionID, nodeID string,
	mcpURL string,
	headers map[string]string,
) ([]model.ToolDefinition, error) {
	seq := atomic.AddInt64(&h.seqCounter, 1)
	resultKey := ResultKeyForSuperagent(executionID, nodeID, seq)

	mcpTask := MCPCallTask{
		ExecutionID: executionID,
		RequestID:   fmt.Sprintf("%s_list_%d", nodeID, seq),
		NodeID:      nodeID,
		Operation:   "list_tools",
		MCPURL:      mcpURL,
		Headers:     headers,
		ResultKey:   resultKey,
		ForToolLoop: true,
	}

	payload, err := json.Marshal(mcpTask)
	if err != nil {
		return nil, fmt.Errorf("marshal list_tools task: %w", err)
	}

	asynqTask := asynq.NewTask(TaskTypeMCPCall, payload, asynq.Queue(QueueNodes))
	if _, err := h.asynqClient.Enqueue(asynqTask); err != nil {
		return nil, fmt.Errorf("enqueue list_tools: %w", err)
	}

	result, err := h.rdb.BRPop(ctx, 2*time.Minute, resultKey).Result()
	if err != nil {
		return nil, fmt.Errorf("brpop list_tools: %w", err)
	}
	if len(result) < 2 {
		return nil, fmt.Errorf("brpop list_tools: unexpected length")
	}

	h.rdb.Del(ctx, resultKey)

	var mcpResult MCPCallResult
	if err := json.Unmarshal([]byte(result[1]), &mcpResult); err != nil {
		return nil, fmt.Errorf("unmarshal list_tools result: %w", err)
	}
	if mcpResult.IsError {
		return nil, fmt.Errorf("list_tools error: %s", mcpResult.Error)
	}

	// Content is []MCPToolDefinition.
	contentBytes, err := json.Marshal(mcpResult.Content)
	if err != nil {
		return nil, fmt.Errorf("marshal list_tools content: %w", err)
	}

	var tools []model.ToolDefinition
	if err := json.Unmarshal(contentBytes, &tools); err != nil {
		return nil, fmt.Errorf("unmarshal tool definitions: %w", err)
	}

	return tools, nil
}

// --- Helpers ---

// resolveSkillHeaders resolves static headers from a SuperagentSkill.
func resolveSkillHeaders(skill model.SuperagentSkill) map[string]string {
	headers := make(map[string]string)
	for _, hc := range skill.Headers {
		if hc.Value != "" {
			headers[hc.Name] = hc.Value
		}
	}
	return headers
}

func (h *SuperagentHandler) emitEvent(sctx *superagentContext, eventType model.EventType, data map[string]any) {
	if sctx.emitter == nil {
		return
	}
	dataJSON, _ := json.Marshal(data)
	sctx.emitter.Emit(model.ExecutionEvent{
		Type:        eventType,
		ExecutionID: sctx.task.ExecutionID,
		Timestamp:   time.Now(),
		Output:      dataJSON,
	})
}

func (h *SuperagentHandler) pushResult(ctx context.Context, t NodeRunTask, result NodeTaskResult) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("superagent: marshal result: %w", err)
	}
	return h.rdb.LPush(ctx, t.ResultKey, string(resultJSON)).Err()
}

func (h *SuperagentHandler) pushFailure(ctx context.Context, t NodeRunTask, errMsg string) error {
	return h.pushResult(ctx, t, NodeTaskResult{
		RequestID: t.RequestID,
		NodeID:    t.NodeID,
		Status:    "failed",
		Error:     errMsg,
	})
}

func superagentDebugTrace(sctx *superagentContext) *model.LLMDebugTrace {
	if sctx == nil || !sctx.task.Debug || len(sctx.llmTraces) == 0 {
		return nil
	}
	calls := make([]model.LLMCallTrace, len(sctx.llmTraces))
	copy(calls, sctx.llmTraces)
	return &model.LLMDebugTrace{Calls: calls}
}

// --- Code Execution ---

// handleCodeExecTool dispatches _code_guidelines and _code_execute tool calls.
func (h *SuperagentHandler) handleCodeExecTool(
	ctx context.Context,
	sctx *superagentContext,
	routing map[string]model.ToolRoute,
	toolName string,
	argsRaw json.RawMessage,
	logger *slog.Logger,
) (string, error) {
	switch toolName {
	case "_code_guidelines":
		return h.handleCodeGuidelines(sctx, routing), nil
	case "_code_execute":
		return h.handleCodeExecute(ctx, sctx, routing, argsRaw, logger)
	default:
		return "", fmt.Errorf("unknown code exec tool: %s", toolName)
	}
}

// handleCodeGuidelines returns the static guidelines plus the dynamic tool list.
func (h *SuperagentHandler) handleCodeGuidelines(sctx *superagentContext, routing map[string]model.ToolRoute) string {
	guidelines := codeexec.Guidelines()

	// Build available tools list — exclude require_approval tools.
	approvalSet := make(map[string]bool)
	if sctx.cfg.ToolPolicies != nil {
		for _, a := range sctx.cfg.ToolPolicies.RequireApproval {
			approvalSet[a] = true
		}
	}

	var availableTools []string
	for name := range routing {
		if !approvalSet[name] {
			availableTools = append(availableTools, name)
		}
	}

	return codeexec.GuidelinesWithTools(guidelines, availableTools)
}

// handleCodeExecute enqueues a code execution task and runs the Redis relay loop.
func (h *SuperagentHandler) handleCodeExecute(
	ctx context.Context,
	sctx *superagentContext,
	routing map[string]model.ToolRoute,
	argsRaw json.RawMessage,
	logger *slog.Logger,
) (string, error) {
	ce := sctx.cfg.CodeExecution
	if ce == nil || !ce.Enabled {
		return "", fmt.Errorf("code execution is not enabled for this node")
	}

	// Check per-run execution limit.
	maxExecs := codeexec.IntOrDefault(ce.MaxExecutionsPerRun, 20)
	sctx.codeExecCount++
	if sctx.codeExecCount > maxExecs {
		return "", fmt.Errorf("code execution limit reached (%d per run)", maxExecs)
	}

	// Parse arguments.
	var args struct {
		Code    string `json:"code"`
		Timeout *int   `json:"timeout,omitempty"`
	}
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		return "", fmt.Errorf("invalid _code_execute arguments: %w", err)
	}
	if args.Code == "" {
		return "", fmt.Errorf("code is required")
	}

	// Resolve limits.
	maxExecTime := codeexec.IntOrDefault(ce.MaxExecutionTimeSec, 30)
	if args.Timeout != nil && *args.Timeout > 0 && *args.Timeout < maxExecTime {
		maxExecTime = *args.Timeout
	}
	maxMemory := codeexec.IntOrDefault(ce.MaxMemoryMB, 256)
	maxOutput := codeexec.IntOrDefault(ce.MaxOutputBytes, 1048576)
	maxCode := codeexec.IntOrDefault(ce.MaxCodeBytes, 65536)
	maxToolCalls := codeexec.IntOrDefault(ce.MaxToolCallsPerExecution, 50)

	// Allocate Redis keys with TTL.
	seq := atomic.AddInt64(&h.seqCounter, 1)
	callbackKey := CodeExecCallbackKey(sctx.task.ExecutionID, sctx.task.NodeID, seq)
	responseKey := CodeExecResponseKey(sctx.task.ExecutionID, sctx.task.NodeID, seq)
	resultKey := CodeExecResultKey(sctx.task.ExecutionID, sctx.task.NodeID, seq)

	ttl := time.Duration(maxExecTime+60) * time.Second
	pipe := h.rdb.Pipeline()
	pipe.Expire(ctx, callbackKey, ttl)
	pipe.Expire(ctx, responseKey, ttl)
	pipe.Expire(ctx, resultKey, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		logger.Warn("setting TTL on code exec keys", "error", err)
	}

	// Clean up keys on exit.
	defer func() {
		cleanCtx := context.Background()
		h.rdb.Del(cleanCtx, callbackKey, responseKey, resultKey)
	}()

	// Enqueue code execution task.
	task := CodeExecTask{
		ExecutionID:         sctx.task.ExecutionID,
		NodeID:              sctx.task.NodeID,
		Seq:                 seq,
		Code:                args.Code,
		MaxExecutionTimeSec: maxExecTime,
		MaxMemoryMB:         maxMemory,
		MaxOutputBytes:      maxOutput,
		MaxCodeBytes:        maxCode,
		MaxToolCalls:        maxToolCalls,
		AllowedModules:      ce.AllowedModules,
		CallbackKey:         callbackKey,
		ResponseKey:         responseKey,
		ResultKey:           resultKey,
	}

	payload, err := json.Marshal(task)
	if err != nil {
		return "", fmt.Errorf("marshaling code exec task: %w", err)
	}

	if _, err := h.asynqClient.Enqueue(
		asynq.NewTask(TaskTypeCodeExec, payload),
		asynq.Queue(QueueCode),
		asynq.MaxRetry(0),
	); err != nil {
		return "", fmt.Errorf("enqueuing code exec task: %w", err)
	}

	logger.Info("code exec task enqueued",
		"callback_key", callbackKey,
		"result_key", resultKey,
		"max_exec_time", maxExecTime,
	)

	// Run relay loop — wait for tool callbacks or the final result.
	return h.codeExecRelayLoop(ctx, sctx, routing, callbackKey, responseKey, resultKey, logger)
}

// codeExecRelayLoop runs a multi-key BRPOP loop, dispatching tool calls from the
// coderunner and returning the final result.
func (h *SuperagentHandler) codeExecRelayLoop(
	ctx context.Context,
	sctx *superagentContext,
	routing map[string]model.ToolRoute,
	callbackKey, responseKey, resultKey string,
	logger *slog.Logger,
) (string, error) {
	// Build approval set for filtering.
	approvalSet := make(map[string]bool)
	if sctx.cfg.ToolPolicies != nil {
		for _, a := range sctx.cfg.ToolPolicies.RequireApproval {
			approvalSet[a] = true
		}
	}

	for {
		// Multi-key BRPOP: wait for tool callback or final result.
		result, err := h.rdb.BRPop(ctx, 5*time.Second, callbackKey, resultKey).Result()
		if err != nil {
			if err == redis.Nil {
				// 5s timeout — just a wake-up to check ctx.Done().
				if ctx.Err() != nil {
					// Push cancel to coderunner.
					cancelResp := CodeToolResponse{Type: "cancel"}
					cancelJSON, _ := json.Marshal(cancelResp)
					h.rdb.LPush(context.Background(), responseKey, string(cancelJSON))
					if ctx.Err() == context.DeadlineExceeded {
						return codeexec.FormatCodeExecResult("timeout", "", "", "", "", "execution timed out", 0, 0), nil
					}
					return codeexec.FormatCodeExecResult("cancelled", "", "", "", "", "execution cancelled", 0, 0), nil
				}
				continue
			}
			if ctx.Err() != nil {
				cancelResp := CodeToolResponse{Type: "cancel"}
				cancelJSON, _ := json.Marshal(cancelResp)
				h.rdb.LPush(context.Background(), responseKey, string(cancelJSON))
				return codeexec.FormatCodeExecResult("error", "", "", "", "", "relay error: "+err.Error(), 0, 0), nil
			}
			return "", fmt.Errorf("BRPOP relay error: %w", err)
		}

		key := result[0]
		data := result[1]

		switch key {
		case callbackKey:
			// Tool callback from coderunner.
			var req CodeToolRequest
			if err := json.Unmarshal([]byte(data), &req); err != nil {
				logger.Error("invalid tool request from coderunner", "error", err)
				errResp := CodeToolResponse{Type: "error", Content: "invalid request", IsError: true, Seq: 0}
				errJSON, _ := json.Marshal(errResp)
				h.rdb.LPush(ctx, responseKey, string(errJSON))
				continue
			}

			// Dispatch tool through normal routing.
			resp := h.dispatchCodeToolCall(ctx, sctx, routing, approvalSet, req, logger)
			respJSON, _ := json.Marshal(resp)
			h.rdb.LPush(ctx, responseKey, string(respJSON))

		case resultKey:
			// Final result from coderunner.
			var execResult CodeExecResult
			if err := json.Unmarshal([]byte(data), &execResult); err != nil {
				return codeexec.FormatCodeExecResult("error", "", "", "", "", "invalid result from coderunner", 0, 0), nil
			}
			return codeexec.FormatCodeExecResult(
				execResult.Status,
				execResult.Output,
				execResult.Stdout,
				execResult.Stderr,
				execResult.Traceback,
				execResult.Error,
				execResult.ToolCalls,
				execResult.DurationMs,
			), nil
		}
	}
}

// dispatchCodeToolCall dispatches a single tool call from code through the normal routing.
func (h *SuperagentHandler) dispatchCodeToolCall(
	ctx context.Context,
	sctx *superagentContext,
	routing map[string]model.ToolRoute,
	approvalSet map[string]bool,
	req CodeToolRequest,
	logger *slog.Logger,
) CodeToolResponse {
	resp := CodeToolResponse{Seq: req.Seq, Type: "result"}

	// Check if tool requires approval — denied from code.
	if approvalSet[req.Name] {
		resp.Type = "error"
		resp.Content = fmt.Sprintf("tool %q requires approval and cannot be called from code", req.Name)
		resp.IsError = true
		return resp
	}

	// Try built-in tools first.
	if result, handled := sctx.dispatcher.AsInterceptor()(req.Name, mustMarshal(req.Arguments)); handled {
		resp.Content = result
		return resp
	}

	// Check routing.
	route, ok := routing[req.Name]
	if !ok {
		resp.Type = "error"
		resp.Content = fmt.Sprintf("tool %q is not available", req.Name)
		resp.IsError = true
		return resp
	}

	// Marshal arguments for MCP/API dispatch.
	argsJSON := mustMarshal(req.Arguments)

	if route.APIToolID != "" {
		result, isError, _, err := h.callAPITool(ctx, sctx, route, "", req.Name, argsJSON)
		if err != nil {
			resp.Type = "error"
			resp.Content = fmt.Sprintf("API tool error: %v", err)
			resp.IsError = true
		} else {
			resp.Content = result
			resp.IsError = isError
		}
	} else {
		result, isError, _, err := h.callMCPTool(ctx, sctx, route, "", req.Name, argsJSON)
		if err != nil {
			resp.Type = "error"
			resp.Content = fmt.Sprintf("MCP tool error: %v", err)
			resp.IsError = true
		} else {
			resp.Content = result
			resp.IsError = isError
		}
	}

	sctx.totalTools++
	return resp
}

// mustMarshal marshals v to JSON, returning empty object on error.
func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}

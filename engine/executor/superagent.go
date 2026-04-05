package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"time"

	"github.com/brockleyai/brockleyai/internal/model"
)

// SuperagentExecutor handles nodes of type "superagent".
type SuperagentExecutor struct{}

var _ NodeExecutor = (*SuperagentExecutor)(nil)

// superagentState holds mutable state during a superagent execution.
type superagentState struct {
	config           *model.SuperagentNodeConfig
	node             *model.Node
	tasks            *TaskTracker
	buffers          *BufferManager
	memory           *MemoryStore
	dispatcher       *BuiltInToolDispatcher
	messages         []model.Message
	workingMemory    map[string]any
	iteration        int
	totalToolCalls   int
	reflectionCount  int
	toolCallHistory  []ToolCallHistoryEntry
	lastResponseText string
}

// --- Stuck Detector ---

// StuckLevel represents the escalation level of stuck detection.
type StuckLevel int

const (
	StuckNone      StuckLevel = iota
	StuckWarn                 // First breach: inject warning
	StuckReflect              // Second breach: trigger reflection
	StuckForceExit            // Third+ breach: force exit
)

// StuckDetector detects when the agent is repeating the same tool calls.
type StuckDetector struct {
	window          []uint64
	windowSize      int
	repeatThreshold int
	head            int
	count           int
	breachCount     int
}

// NewStuckDetector creates a StuckDetector with the given window size and repeat threshold.
func NewStuckDetector(windowSize, repeatThreshold int) *StuckDetector {
	return &StuckDetector{
		window:          make([]uint64, windowSize),
		windowSize:      windowSize,
		repeatThreshold: repeatThreshold,
	}
}

// Record adds a tool call hash to the circular buffer and returns the stuck level.
func (sd *StuckDetector) Record(toolName string, args json.RawMessage) StuckLevel {
	h := fnv.New64a()
	h.Write([]byte(toolName + ":" + canonicalArgs(args)))
	hash := h.Sum64()

	// Add to circular buffer.
	sd.window[sd.head] = hash
	sd.head = (sd.head + 1) % sd.windowSize
	if sd.count < sd.windowSize {
		sd.count++
	}

	// Count occurrences of each hash in the buffer.
	counts := make(map[uint64]int)
	for i := 0; i < sd.count; i++ {
		counts[sd.window[i]]++
	}

	// Check if any hash exceeds the threshold.
	for _, c := range counts {
		if c >= sd.repeatThreshold {
			sd.breachCount++
			switch sd.breachCount {
			case 1:
				return StuckWarn
			case 2:
				return StuckReflect
			default:
				return StuckForceExit
			}
		}
	}

	return StuckNone
}

// Reset resets the breach count (called after successful reflection).
func (sd *StuckDetector) Reset() {
	sd.breachCount = 0
}

// canonicalArgs sorts JSON keys for deterministic hashing.
func canonicalArgs(args json.RawMessage) string {
	var m map[string]any
	if err := json.Unmarshal(args, &m); err != nil {
		return string(args)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b, err := json.Marshal(reorderMap(m, keys))
	if err != nil {
		return string(args)
	}
	return string(b)
}

// reorderMap creates an ordered representation for canonical JSON.
func reorderMap(m map[string]any, keys []string) map[string]any {
	ordered := make(map[string]any, len(keys))
	for _, k := range keys {
		ordered[k] = m[k]
	}
	return ordered
}

// --- Evaluator and Reflection Result Types ---

// EvaluatorResult holds the output of the evaluator LLM call.
type EvaluatorResult struct {
	NeedsMoreWork bool   `json:"needs_more_work"`
	StuckDetected bool   `json:"stuck_detected"`
	ShouldCompact bool   `json:"should_compact"`
	Reasoning     string `json:"reasoning"`
}

// ReflectionResult holds the output of the reflection LLM call.
type ReflectionResult struct {
	ReflectionText string `json:"reflection_text"`
	NewPlan        string `json:"new_plan"`
}

// --- Token Estimation ---

// estimateTokens provides a rough token count based on character count.
func estimateTokens(messages []model.Message) int {
	total := 0
	for _, m := range messages {
		total += len(m.Content) / 4
	}
	return total
}

const (
	defaultContextWindowLimit  = 128000
	defaultCompactionThreshold = 0.75
)

// --- Execute ---

func (e *SuperagentExecutor) Execute(ctx context.Context, node *model.Node, inputs map[string]any, nctx *NodeContext, deps *ExecutorDeps) (*NodeResult, error) {
	// 1. Parse config.
	var cfg model.SuperagentNodeConfig
	if err := json.Unmarshal(node.Config, &cfg); err != nil {
		return nil, fmt.Errorf("superagent executor: invalid config: %w", err)
	}

	// 2. Resolve API key: inline api_key takes priority over api_key_ref.
	apiKey := cfg.APIKey
	if apiKey == "" && cfg.APIKeyRef != "" && deps.SecretStore != nil {
		var err error
		apiKey, err = deps.SecretStore.GetSecret(ctx, cfg.APIKeyRef)
		if err != nil {
			return nil, fmt.Errorf("superagent executor: resolving api_key_ref %q: %w", cfg.APIKeyRef, err)
		}
	}

	// 3. Look up provider.
	if deps.ProviderRegistry == nil {
		return nil, fmt.Errorf("superagent executor: no provider registry configured")
	}
	provider, err := deps.ProviderRegistry.Get(string(cfg.Provider))
	if err != nil {
		return nil, fmt.Errorf("superagent executor: looking up provider %q: %w", cfg.Provider, err)
	}

	// 4. Resolve tools from skills.
	routing := make(map[string]model.ToolRoute)
	var mcpToolDefs []model.LLMToolDefinition

	for _, skill := range cfg.Skills {
		// API tool skill handling: resolve endpoints from the API tool definition.
		if skill.APIToolID != "" {
			if deps.APIToolDispatcher == nil {
				return nil, fmt.Errorf("superagent executor: APIToolDispatcher required for API tool skill %q", skill.Name)
			}
			tenantID, _ := nctx.Meta["tenant_id"].(string)
			def, err := deps.APIToolDispatcher.ResolveDefinition(ctx, tenantID, skill.APIToolID)
			if err != nil {
				return nil, fmt.Errorf("superagent executor: resolving API tool for skill %q: %w", skill.Name, err)
			}
			for _, epName := range skill.Endpoints {
				ep := FindEndpoint(def, epName)
				if ep == nil {
					return nil, fmt.Errorf("superagent executor: endpoint %q not found in API tool %q", epName, skill.APIToolID)
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

		// Resolve headers for skill.
		headers := make(map[string]string)
		for _, hc := range skill.Headers {
			val, err := resolveHeaderValue(ctx, model.HeaderConfig(hc), inputs, nctx, deps)
			if err != nil {
				return nil, fmt.Errorf("superagent executor: resolving header for skill %q: %w", skill.Name, err)
			}
			headers[hc.Name] = val
		}

		// Get MCP client via cache.
		if deps.MCPClientCache == nil {
			return nil, fmt.Errorf("superagent executor: MCPClientCache is required")
		}
		client := deps.MCPClientCache.GetOrCreate(skill.MCPURL, headers)

		// List tools from MCP server.
		tools, err := client.ListTools(ctx)
		if err != nil {
			return nil, fmt.Errorf("superagent executor: listing tools for skill %q: %w", skill.Name, err)
		}

		// Build allowlist set if provided.
		allowSet := make(map[string]bool)
		for _, t := range skill.Tools {
			allowSet[t] = true
		}

		// Build denied set from tool policies.
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

		timeout := skill.TimeoutSeconds
		for _, tool := range tools {
			// Compacted mode: skip tools not in explicit allowlist.
			// When compacted + no allowlist, no tools get full schemas.
			if skill.Compacted && !allowSet[tool.Name] {
				continue
			}
			// Apply allowlist filter (non-compacted mode).
			if !skill.Compacted && len(allowSet) > 0 && !allowSet[tool.Name] {
				continue
			}
			// Apply denied policy.
			if deniedSet[tool.Name] {
				continue
			}
			// Exclude require_approval tools (P4 skeleton doesn't support approval).
			if approvalSet[tool.Name] {
				continue
			}

			// Add routing entry.
			routing[tool.Name] = model.ToolRoute{
				MCPURL:         skill.MCPURL,
				Headers:        skill.Headers,
				TimeoutSeconds: timeout,
			}

			// Convert to LLM tool definition.
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

	// 5. Collect API tool IDs from skills for introspection.
	var apiToolIDs []string
	for _, skill := range cfg.Skills {
		if skill.APIToolID != "" {
			apiToolIDs = append(apiToolIDs, skill.APIToolID)
		}
	}

	// 6. Get built-in tool definitions.
	builtInDefs := GetBuiltInToolDefinitions(&cfg)

	// 7. Merge built-in + MCP + API tool definitions.
	allTools := append(builtInDefs, mcpToolDefs...)

	// Append API tool introspection tools if any API tool skills are present.
	if len(apiToolIDs) > 0 {
		allTools = append(allTools, apiToolIntrospectionTools...)
	}

	// Append MCP introspection tools if any compacted skills are present.
	compactedURLs := CollectCompactedSkillURLs(cfg.Skills)
	if len(compactedURLs) > 0 {
		allTools = append(allTools, mcpToolIntrospectionTools...)
	}

	// 8. Initialize superagent state.
	namespace := node.ID
	if cfg.SharedMemory != nil && cfg.SharedMemory.Namespace != "" {
		namespace = cfg.SharedMemory.Namespace
	}
	tasks := NewTaskTracker()
	buffers := NewBufferManager()
	memory := NewMemoryStore(namespace)

	// Load shared memory from state if enabled.
	if cfg.SharedMemory != nil && cfg.SharedMemory.Enabled && nctx != nil {
		if memIn, ok := nctx.State["_memory_in"]; ok {
			if memMap, ok := memIn.(map[string]any); ok {
				memory.LoadFromState(memMap)
			}
		}
	}

	dispatcher := NewBuiltInToolDispatcher(tasks, buffers, memory)

	state := &superagentState{
		config:        &cfg,
		node:          node,
		tasks:         tasks,
		buffers:       buffers,
		memory:        memory,
		dispatcher:    dispatcher,
		workingMemory: make(map[string]any),
	}

	// 9. Build initial shared memory entries for injection.
	var sharedMemory []MemoryEntry
	injectOnStart := true
	if cfg.SharedMemory != nil && cfg.SharedMemory.InjectOnStart != nil {
		injectOnStart = *cfg.SharedMemory.InjectOnStart
	}
	if cfg.SharedMemory != nil && cfg.SharedMemory.Enabled && injectOnStart {
		sharedMemory = memory.List(nil)
	}

	// 10. Assemble system prompt.
	systemPrompt, err := AssembleSystemPrompt(&cfg, inputs, sharedMemory, state.workingMemory, 0, node.OutputPorts)
	if err != nil {
		return nil, fmt.Errorf("superagent executor: assembling system prompt: %w", err)
	}

	// 11. Build initial messages.
	state.messages = []model.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: "Start the task now. Use tools if they help, and provide the final answer when you are done."},
	}

	// Load conversation history from input if configured.
	if cfg.ConversationHistoryFromInput != "" {
		if histInput, ok := inputs[cfg.ConversationHistoryFromInput]; ok {
			histBytes, err := json.Marshal(histInput)
			if err == nil {
				var histMsgs []model.Message
				if json.Unmarshal(histBytes, &histMsgs) == nil {
					state.messages = append(state.messages, histMsgs...)
				}
			}
		}
	}

	// 12. Run the agent loop.
	finishReason, err := e.runAgentLoop(ctx, state, deps, nctx, apiKey, provider, routing, allTools, inputs, apiToolIDs, compactedURLs)
	if err != nil {
		return nil, fmt.Errorf("superagent executor: agent loop: %w", err)
	}

	// 13. Emit completed event.
	emitSuperagentEvent(deps, ctx, model.EventSuperagentCompleted, map[string]any{
		"node_id":          state.node.ID,
		"iterations":       state.iteration,
		"total_tool_calls": state.totalToolCalls,
		"finish_reason":    finishReason,
	})

	// 14. Resolve outputs.
	outputs, err := e.resolveOutputs(ctx, &cfg, state, node, deps, apiKey, provider)
	if err != nil {
		return nil, fmt.Errorf("superagent executor: resolving outputs: %w", err)
	}

	// 15. Build meta outputs.
	metaOutputs := e.buildMetaOutputs(state, finishReason)
	for k, v := range metaOutputs {
		outputs[k] = v
	}

	return &NodeResult{Outputs: outputs}, nil
}

// --- Agent Loop ---

func (e *SuperagentExecutor) runAgentLoop(
	ctx context.Context,
	state *superagentState,
	deps *ExecutorDeps,
	nctx *NodeContext,
	apiKey string,
	provider model.LLMProvider,
	routing map[string]model.ToolRoute,
	allTools []model.LLMToolDefinition,
	inputs map[string]any,
	apiToolIDs []string,
	compactedURLs []string,
) (string, error) {
	cfg := state.config

	// 1. Apply timeout.
	timeoutSec := IntOrDefault(cfg.TimeoutSeconds, 600)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	// 2. Initialize stuck detector.
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
	stuckDetector := NewStuckDetector(windowSize, repeatThreshold)

	// 3. Emit started event.
	emitSuperagentEvent(deps, ctx, model.EventSuperagentStarted, map[string]any{
		"node_id":    state.node.ID,
		"skills":     len(cfg.Skills),
		"tool_count": len(allTools),
	})

	// 4. Determine limits.
	maxIterations := IntOrDefault(cfg.MaxIterations, 25)
	maxTotalCalls := IntOrDefault(cfg.MaxTotalToolCalls, 200)
	maxReflections := 3
	if cfg.Overrides != nil && cfg.Overrides.Reflection != nil && cfg.Overrides.Reflection.MaxReflections != nil {
		maxReflections = *cfg.Overrides.Reflection.MaxReflections
	}

	evaluatorDisabled := cfg.Overrides != nil && cfg.Overrides.Evaluator != nil && cfg.Overrides.Evaluator.Disabled

	// 5. Agent loop.
	for state.iteration = 0; state.iteration < maxIterations; state.iteration++ {
		// a. Check context cancellation.
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return "timeout", nil
			}
			return "cancelled", nil
		default:
		}

		// b. Inject task reminder if tasks exist.
		taskReminderEnabled := true
		if cfg.Overrides != nil && cfg.Overrides.TaskTracking != nil &&
			cfg.Overrides.TaskTracking.Enabled != nil && !*cfg.Overrides.TaskTracking.Enabled {
			taskReminderEnabled = false
		}
		if taskReminderEnabled && len(state.tasks.List()) > 0 && state.iteration > 0 {
			reminder := state.tasks.BuildTaskReminder()
			if reminder != "" {
				state.messages = append(state.messages, model.Message{
					Role:    "system",
					Content: "Task Status Update:\n" + reminder,
				})
			}
		}

		// c. Build CompletionRequest.
		maxCallsPerIter := IntOrDefault(cfg.MaxToolCallsPerIteration, 25)
		maxToolLoopRounds := IntOrDefault(cfg.MaxToolLoopRounds, 10)

		interceptor := state.dispatcher.AsInterceptor()
		if len(apiToolIDs) > 0 && deps.APIToolDispatcher != nil {
			tenantID, _ := nctx.Meta["tenant_id"].(string)
			apiInterceptor := NewAPIToolInterceptor(deps.APIToolDispatcher, tenantID, apiToolIDs)
			interceptor = ChainInterceptors(interceptor, apiInterceptor)
		}
		if len(compactedURLs) > 0 && deps.MCPClientCache != nil {
			mcpInterceptor := NewMCPToolInterceptor(deps.MCPClientCache, compactedURLs)
			interceptor = ChainInterceptors(interceptor, mcpInterceptor)
		}

		req := &model.CompletionRequest{
			APIKey:      apiKey,
			Model:       cfg.Model,
			BaseURL:     cfg.BaseURL,
			Messages:    state.messages,
			Temperature: cfg.Temperature,
			MaxTokens:   cfg.MaxTokens,
			Tools:       allTools,
			ToolChoice:  "auto",
		}

		// d. Call RunToolLoop.
		toolLoopResult, err := RunToolLoop(ctx, ToolLoopConfig{
			Provider:      provider,
			Request:       req,
			Routing:       routing,
			MaxCalls:      maxCallsPerIter,
			MaxIterations: maxToolLoopRounds,
			Interceptor:   interceptor,
		}, deps, nctx)
		if err != nil {
			// Check if this was a context cancellation/timeout.
			if ctx.Err() != nil {
				if ctx.Err() == context.DeadlineExceeded {
					return "timeout", nil
				}
				return "cancelled", nil
			}
			return "", fmt.Errorf("tool loop iteration %d: %w", state.iteration, err)
		}

		// e. Update state.
		state.messages = toolLoopResult.Messages
		state.toolCallHistory = append(state.toolCallHistory, toolLoopResult.History...)
		state.totalToolCalls += toolLoopResult.TotalToolCalls
		if toolLoopResult.Response != nil {
			state.lastResponseText = toolLoopResult.Response.Content
		}

		// f. Check max total tool calls.
		if state.totalToolCalls >= maxTotalCalls {
			return "max_tool_calls", nil
		}

		// g. Run stuck detection on new tool calls.
		for _, entry := range toolLoopResult.History {
			level := stuckDetector.Record(entry.Name, entry.Arguments)
			switch level {
			case StuckWarn:
				// Inject warning message.
				state.messages = append(state.messages, model.Message{
					Role:    "system",
					Content: "WARNING: You appear to be repeating the same tool calls. Try a different approach or tool. If you believe you're done, report your findings.",
				})
				emitSuperagentEvent(deps, ctx, model.EventSuperagentStuckWarning, map[string]any{
					"node_id":   state.node.ID,
					"iteration": state.iteration,
					"tool_name": entry.Name,
				})
			case StuckReflect:
				if err := e.reflect(ctx, state, deps, apiKey, provider); err != nil {
					return "", fmt.Errorf("reflection at iteration %d: %w", state.iteration, err)
				}
				stuckDetector.Reset()
			case StuckForceExit:
				return "stuck", nil
			}
		}

		// h. Run evaluator if not disabled.
		if !evaluatorDisabled {
			evalResult, err := e.evaluate(ctx, state, deps, apiKey, provider)
			if err != nil {
				// Non-fatal: if evaluator fails, continue the loop.
				// But if context is cancelled, exit.
				if ctx.Err() != nil {
					if ctx.Err() == context.DeadlineExceeded {
						return "timeout", nil
					}
					return "cancelled", nil
				}
				// Log and continue on evaluator error.
			} else {
				if !evalResult.NeedsMoreWork {
					return "done", nil
				}
				if evalResult.StuckDetected {
					if state.reflectionCount >= maxReflections {
						return "stuck", nil
					}
					if err := e.reflect(ctx, state, deps, apiKey, provider); err != nil {
						return "", fmt.Errorf("reflection at iteration %d: %w", state.iteration, err)
					}
					stuckDetector.Reset()
				}
				if evalResult.ShouldCompact {
					if err := e.compactContext(ctx, state, deps, nctx, apiKey, provider, inputs); err != nil {
						// Non-fatal: continue without compaction.
						_ = err
					}
				}
			}

			// Proactive compaction check based on token estimate.
			contextWindowLimit := defaultContextWindowLimit
			compactionThreshold := defaultCompactionThreshold
			if cfg.Overrides != nil && cfg.Overrides.ContextCompaction != nil {
				if cfg.Overrides.ContextCompaction.ContextWindowLimit != nil {
					contextWindowLimit = *cfg.Overrides.ContextCompaction.ContextWindowLimit
				}
				if cfg.Overrides.ContextCompaction.CompactionThreshold != nil {
					compactionThreshold = *cfg.Overrides.ContextCompaction.CompactionThreshold
				}
			}
			if estimateTokens(state.messages) > int(float64(contextWindowLimit)*compactionThreshold) {
				if err := e.compactContext(ctx, state, deps, nctx, apiKey, provider, inputs); err != nil {
					_ = err // Non-fatal
				}
			}
		}

		// j. Check reflection count.
		if state.reflectionCount > maxReflections {
			return "stuck", nil
		}

		// k. Emit iteration event.
		emitSuperagentEvent(deps, ctx, model.EventSuperagentIteration, map[string]any{
			"node_id":          state.node.ID,
			"iteration":        state.iteration,
			"total_tool_calls": state.totalToolCalls,
		})
	}

	// 6. Max iterations reached.
	return "max_iterations", nil
}

// --- Evaluator ---

func (e *SuperagentExecutor) evaluate(
	ctx context.Context,
	state *superagentState,
	deps *ExecutorDeps,
	apiKey string,
	provider model.LLMProvider,
) (*EvaluatorResult, error) {
	cfg := state.config

	// Resolve override provider.
	evalProvider := provider
	evalAPIKey := apiKey
	evalModel := cfg.Model
	if cfg.Overrides != nil && cfg.Overrides.Evaluator != nil {
		ov := cfg.Overrides.Evaluator
		p, k, m, err := e.resolveOverrideProvider(ctx, cfg, ov.Provider, ov.Model, ov.APIKey, ov.APIKeyRef, deps, provider, apiKey, cfg.Model)
		if err == nil {
			evalProvider = p
			evalAPIKey = k
			evalModel = m
		}
	}

	// Build evaluator prompt.
	prompt := DefaultEvaluatorPrompt
	if cfg.Overrides != nil && cfg.Overrides.Evaluator != nil && cfg.Overrides.Evaluator.Prompt != "" {
		prompt = cfg.Overrides.Evaluator.Prompt
	}

	// Build output requirements string.
	var outReqs []string
	for _, port := range state.node.OutputPorts {
		if !strings.HasPrefix(port.Name, "_") {
			outReqs = append(outReqs, fmt.Sprintf("- %s (schema: %s)", port.Name, string(port.Schema)))
		}
	}
	outputRequirements := strings.Join(outReqs, "\n")

	// Replace template variables.
	planStr, _ := state.workingMemory["plan"].(string)
	prompt = strings.ReplaceAll(prompt, "{{task}}", cfg.Prompt)
	prompt = strings.ReplaceAll(prompt, "{{output_requirements}}", outputRequirements)
	prompt = strings.ReplaceAll(prompt, "{{task_list}}", state.tasks.BuildTaskReminder())
	prompt = strings.ReplaceAll(prompt, "{{iteration}}", fmt.Sprintf("%d", state.iteration))
	prompt = strings.ReplaceAll(prompt, "{{plan}}", planStr)

	// Build messages for evaluator call (conversation history + evaluator prompt).
	msgs := make([]model.Message, len(state.messages))
	copy(msgs, state.messages)
	msgs = append(msgs, model.Message{
		Role:    "user",
		Content: prompt,
	})

	resp, err := evalProvider.Complete(ctx, &model.CompletionRequest{
		APIKey:         evalAPIKey,
		Model:          evalModel,
		BaseURL:        state.config.BaseURL,
		Messages:       msgs,
		ResponseFormat: model.ResponseFormatJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("evaluator LLM call: %w", err)
	}

	var result EvaluatorResult
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		// If we can't parse the response, assume needs more work.
		return &EvaluatorResult{NeedsMoreWork: true, Reasoning: "evaluator response unparseable"}, nil
	}

	emitSuperagentEvent(deps, ctx, model.EventSuperagentEvaluation, map[string]any{
		"node_id":         state.node.ID,
		"iteration":       state.iteration,
		"needs_more_work": result.NeedsMoreWork,
		"stuck_detected":  result.StuckDetected,
		"should_compact":  result.ShouldCompact,
		"reasoning":       result.Reasoning,
	})

	return &result, nil
}

// --- Reflection ---

func (e *SuperagentExecutor) reflect(
	ctx context.Context,
	state *superagentState,
	deps *ExecutorDeps,
	apiKey string,
	provider model.LLMProvider,
) error {
	cfg := state.config

	// Resolve override provider.
	refProvider := provider
	refAPIKey := apiKey
	refModel := cfg.Model
	if cfg.Overrides != nil && cfg.Overrides.Reflection != nil {
		ov := cfg.Overrides.Reflection
		p, k, m, err := e.resolveOverrideProvider(ctx, cfg, ov.Provider, ov.Model, ov.APIKey, ov.APIKeyRef, deps, provider, apiKey, cfg.Model)
		if err == nil {
			refProvider = p
			refAPIKey = k
			refModel = m
		}
	}

	// Build reflection prompt.
	prompt := DefaultReflectionPrompt
	if cfg.Overrides != nil && cfg.Overrides.Reflection != nil && cfg.Overrides.Reflection.Prompt != "" {
		prompt = cfg.Overrides.Reflection.Prompt
	}

	planStr, _ := state.workingMemory["plan"].(string)
	prompt = strings.ReplaceAll(prompt, "{{task}}", cfg.Prompt)
	prompt = strings.ReplaceAll(prompt, "{{task_list}}", state.tasks.BuildTaskReminder())
	prompt = strings.ReplaceAll(prompt, "{{plan}}", planStr)
	prompt = strings.ReplaceAll(prompt, "{{iteration}}", fmt.Sprintf("%d", state.iteration))

	// Build messages.
	msgs := make([]model.Message, len(state.messages))
	copy(msgs, state.messages)
	msgs = append(msgs, model.Message{
		Role:    "user",
		Content: prompt,
	})

	resp, err := refProvider.Complete(ctx, &model.CompletionRequest{
		APIKey:         refAPIKey,
		Model:          refModel,
		BaseURL:        state.config.BaseURL,
		Messages:       msgs,
		ResponseFormat: model.ResponseFormatJSON,
	})
	if err != nil {
		return fmt.Errorf("reflection LLM call: %w", err)
	}

	var result ReflectionResult
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		// Use raw content as reflection text if unparseable.
		result.ReflectionText = resp.Content
	}

	// Inject reflection into main conversation.
	state.messages = append(state.messages, model.Message{
		Role:    "system",
		Content: fmt.Sprintf("[Reflection] %s\n\nRevised Plan: %s", result.ReflectionText, result.NewPlan),
	})

	// Update working memory.
	if result.NewPlan != "" {
		state.workingMemory["plan"] = result.NewPlan
	}

	state.reflectionCount++

	emitSuperagentEvent(deps, ctx, model.EventSuperagentReflection, map[string]any{
		"node_id":          state.node.ID,
		"iteration":        state.iteration,
		"reflection_count": state.reflectionCount,
		"reflection_text":  result.ReflectionText,
		"new_plan":         result.NewPlan,
	})

	return nil
}

// --- Context Compaction ---

func (e *SuperagentExecutor) compactContext(
	ctx context.Context,
	state *superagentState,
	deps *ExecutorDeps,
	nctx *NodeContext,
	apiKey string,
	provider model.LLMProvider,
	inputs map[string]any,
) error {
	cfg := state.config
	messagesBefore := len(state.messages)

	// Resolve override provider for compaction.
	compProvider := provider
	compAPIKey := apiKey
	compModel := cfg.Model
	if cfg.Overrides != nil && cfg.Overrides.ContextCompaction != nil {
		ov := cfg.Overrides.ContextCompaction
		p, k, m, err := e.resolveOverrideProvider(ctx, cfg, ov.Provider, ov.Model, ov.APIKey, ov.APIKeyRef, deps, provider, apiKey, cfg.Model)
		if err == nil {
			compProvider = p
			compAPIKey = k
			compModel = m
		}
	}

	memoriesFlushed := 0

	// Step 1: Memory flush (if auto_flush enabled).
	if cfg.SharedMemory != nil && cfg.SharedMemory.AutoFlush != nil && *cfg.SharedMemory.AutoFlush {
		flushMsgs := make([]model.Message, len(state.messages))
		copy(flushMsgs, state.messages)
		flushMsgs = append(flushMsgs, model.Message{
			Role:    "user",
			Content: DefaultMemoryFlushPrompt,
		})

		flushResp, err := compProvider.Complete(ctx, &model.CompletionRequest{
			APIKey:         compAPIKey,
			Model:          compModel,
			BaseURL:        state.config.BaseURL,
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
					state.memory.Store(fact.Key, fact.Content, fact.Tags)
					memoriesFlushed++
				}
			}
		}
	}

	// Step 2: Summarize conversation.
	compactionPrompt := DefaultCompactionPrompt
	if cfg.Overrides != nil && cfg.Overrides.ContextCompaction != nil && cfg.Overrides.ContextCompaction.Prompt != "" {
		compactionPrompt = cfg.Overrides.ContextCompaction.Prompt
	}

	compMsgs := make([]model.Message, len(state.messages))
	copy(compMsgs, state.messages)
	compMsgs = append(compMsgs, model.Message{
		Role:    "user",
		Content: compactionPrompt,
	})

	summaryResp, err := compProvider.Complete(ctx, &model.CompletionRequest{
		APIKey:   compAPIKey,
		Model:    compModel,
		BaseURL:  state.config.BaseURL,
		Messages: compMsgs,
	})
	if err != nil {
		return fmt.Errorf("compaction summary LLM call: %w", err)
	}
	summary := summaryResp.Content

	// Step 3: Reconstruct context.
	var sharedMemory []MemoryEntry
	if cfg.SharedMemory != nil && cfg.SharedMemory.Enabled {
		sharedMemory = state.memory.List(nil)
	}

	systemPrompt, err := AssembleSystemPrompt(cfg, inputs, sharedMemory, state.workingMemory, state.iteration, state.node.OutputPorts)
	if err != nil {
		return fmt.Errorf("compaction reassemble prompt: %w", err)
	}

	preserveRecent := 5
	if cfg.Overrides != nil && cfg.Overrides.ContextCompaction != nil && cfg.Overrides.ContextCompaction.PreserveRecentMessages != nil {
		preserveRecent = *cfg.Overrides.ContextCompaction.PreserveRecentMessages
	}

	// Build new messages: system prompt + summary + last N messages.
	newMessages := []model.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "system", Content: "[Context Summary from previous conversation]\n" + summary},
	}

	// Preserve recent messages.
	if preserveRecent > 0 && len(state.messages) > preserveRecent {
		newMessages = append(newMessages, state.messages[len(state.messages)-preserveRecent:]...)
	} else if preserveRecent > 0 && len(state.messages) > 1 {
		// Keep all non-system messages if fewer than preserveRecent.
		newMessages = append(newMessages, state.messages[1:]...)
	}

	state.messages = newMessages

	emitSuperagentEvent(deps, ctx, model.EventSuperagentCompaction, map[string]any{
		"node_id":          state.node.ID,
		"iteration":        state.iteration,
		"messages_before":  messagesBefore,
		"messages_after":   len(state.messages),
		"memories_flushed": memoriesFlushed,
	})

	return nil
}

// --- Override Provider Resolution ---

func (e *SuperagentExecutor) resolveOverrideProvider(
	ctx context.Context,
	cfg *model.SuperagentNodeConfig,
	overrideProvider model.ProviderType,
	overrideModel string,
	overrideAPIKey string,
	overrideAPIKeyRef string,
	deps *ExecutorDeps,
	fallbackProvider model.LLMProvider,
	fallbackAPIKey string,
	fallbackModel string,
) (model.LLMProvider, string, string, error) {
	resultProvider := fallbackProvider
	resultAPIKey := fallbackAPIKey
	resultModel := fallbackModel

	if overrideProvider != "" && overrideModel != "" {
		p, err := deps.ProviderRegistry.Get(string(overrideProvider))
		if err != nil {
			return fallbackProvider, fallbackAPIKey, fallbackModel, err
		}
		resultProvider = p
		resultModel = overrideModel
	}

	if overrideAPIKey != "" {
		resultAPIKey = overrideAPIKey
	} else if overrideAPIKeyRef != "" && deps.SecretStore != nil {
		key, err := deps.SecretStore.GetSecret(ctx, overrideAPIKeyRef)
		if err == nil {
			resultAPIKey = key
		}
	}

	return resultProvider, resultAPIKey, resultModel, nil
}

// --- Output Resolution ---

// resolveOutputs determines the output for each output port.
func (e *SuperagentExecutor) resolveOutputs(
	ctx context.Context,
	cfg *model.SuperagentNodeConfig,
	state *superagentState,
	node *model.Node,
	deps *ExecutorDeps,
	apiKey string,
	provider model.LLMProvider,
) (map[string]any, error) {
	outputs := make(map[string]any)
	finalizedBuffers := state.buffers.GetFinalizedBuffers()

	// Get response text from the final LLM response.
	responseText := state.lastResponseText
	if strings.TrimSpace(responseText) == "" {
		responseText = lastNonEmptyAssistantMessage(state.messages)
	}

	// Count non-meta output ports.
	var nonMetaPorts []model.Port
	for _, port := range node.OutputPorts {
		if !strings.HasPrefix(port.Name, "_") {
			nonMetaPorts = append(nonMetaPorts, port)
		}
	}

	for _, port := range nonMetaPorts {
		// Priority 1: Check finalized buffers.
		if content, ok := finalizedBuffers[port.Name]; ok {
			outputs[port.Name] = content
			continue
		}

		// Priority 2: Single string output port with a non-empty final response.
		if len(nonMetaPorts) == 1 && IsStringSchema(port.Schema) && strings.TrimSpace(responseText) != "" {
			outputs[port.Name] = responseText
			continue
		}

		// Priority 3: Call extraction LLM.
		extracted, err := e.extractOutput(ctx, cfg, state, port, deps, apiKey, provider)
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

// extractOutput calls an LLM to extract structured output from the conversation.
func (e *SuperagentExecutor) extractOutput(
	ctx context.Context,
	cfg *model.SuperagentNodeConfig,
	state *superagentState,
	port model.Port,
	deps *ExecutorDeps,
	apiKey string,
	provider model.LLMProvider,
) (any, error) {
	// Use override provider/model if set.
	extractProvider := provider
	extractAPIKey := apiKey
	extractModel := cfg.Model
	if cfg.Overrides != nil && cfg.Overrides.OutputExtraction != nil {
		oe := cfg.Overrides.OutputExtraction
		if oe.Provider != "" && oe.Model != "" {
			p, err := deps.ProviderRegistry.Get(string(oe.Provider))
			if err == nil {
				extractProvider = p
			}
			extractModel = oe.Model
		}
		if oe.APIKey != "" {
			extractAPIKey = oe.APIKey
		} else if oe.APIKeyRef != "" && deps.SecretStore != nil {
			if key, err := deps.SecretStore.GetSecret(ctx, oe.APIKeyRef); err == nil {
				extractAPIKey = key
			}
		}
	}

	// Build extraction prompt.
	schemaStr := string(port.Schema)
	extractionPrompt := strings.ReplaceAll(DefaultExtractionPrompt, "{{schema}}", schemaStr)

	// Build messages: conversation history + extraction instruction.
	msgs := make([]model.Message, len(state.messages))
	copy(msgs, state.messages)
	msgs = append(msgs, model.Message{
		Role:    "user",
		Content: extractionPrompt,
	})

	resp, err := extractProvider.Complete(ctx, &model.CompletionRequest{
		APIKey:         extractAPIKey,
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
		// Return raw text if JSON parsing fails.
		return resp.Content, nil
	}
	return parsed, nil
}

// buildMetaOutputs constructs meta output fields.
func (e *SuperagentExecutor) buildMetaOutputs(state *superagentState, finishReason string) map[string]any {
	meta := map[string]any{
		"_conversation_history": state.messages,
		"_iterations":           state.iteration,
		"_total_tool_calls":     state.totalToolCalls,
		"_finish_reason":        finishReason,
		"_tool_call_history":    state.toolCallHistory,
		"_working_memory":       state.workingMemory,
		"_tasks":                state.tasks.List(),
	}

	if state.config.SharedMemory != nil && state.config.SharedMemory.Enabled {
		meta["_memory_out"] = state.memory.ToStateValue()
	}

	return meta
}

// emitSuperagentEvent emits a superagent-related event through the event emitter.
func emitSuperagentEvent(deps *ExecutorDeps, ctx context.Context, eventType model.EventType, data map[string]any) {
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

// IsStringSchema checks if a JSON Schema represents a simple string type.
func IsStringSchema(schema json.RawMessage) bool {
	var s map[string]any
	if err := json.Unmarshal(schema, &s); err != nil {
		return false
	}
	typ, _ := s["type"].(string)
	return typ == "string"
}

// IntOrDefault returns the value pointed to by p, or d if p is nil.
func IntOrDefault(p *int, d int) int {
	if p != nil {
		return *p
	}
	return d
}

// Float64OrDefault returns the value pointed to by p, or d if p is nil.
func Float64OrDefault(p *float64, d float64) float64 {
	if p != nil {
		return *p
	}
	return d
}

// BoolOrDefault returns the value pointed to by p, or d if p is nil.
func BoolOrDefault(p *bool, d bool) bool {
	if p != nil {
		return *p
	}
	return d
}

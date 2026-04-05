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
	"github.com/brockleyai/brockleyai/engine/expression"
	graphpkg "github.com/brockleyai/brockleyai/engine/graph"
	"github.com/brockleyai/brockleyai/engine/orchestrator"
	"github.com/brockleyai/brockleyai/engine/provider"
	"github.com/brockleyai/brockleyai/internal/model"
	"github.com/brockleyai/brockleyai/internal/secret"
)

// OrchestratorHandler processes graph:start tasks.
// It is the main orchestrator that walks the graph in topological order,
// dispatches node tasks to workers, and collects results.
type OrchestratorHandler struct {
	store        model.Store
	rdb          *redis.Client
	asynqClient  *asynq.Client
	logger       *slog.Logger
	brpopTimeout time.Duration // default 10*time.Minute, overridable for tests
}

// NewOrchestratorHandler creates a new OrchestratorHandler.
func NewOrchestratorHandler(store model.Store, rdb *redis.Client, asynqClient *asynq.Client, logger *slog.Logger) *OrchestratorHandler {
	return &OrchestratorHandler{
		store:        store,
		rdb:          rdb,
		asynqClient:  asynqClient,
		logger:       logger,
		brpopTimeout: 10 * time.Minute,
	}
}

// ProcessTask handles an asynq task for graph orchestration.
func (h *OrchestratorHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	// 1. Deserialize and validate.
	execTask, graph, inputs, err := h.deserializeAndValidate(task)
	if err != nil {
		return err
	}

	logger := h.logger.With(
		"execution_id", execTask.ExecutionID,
		"graph_id", execTask.GraphID,
		"graph_name", execTask.GraphName,
	)
	logger.Info("orchestrator started")

	// 2. Update execution status to running.
	h.updateExecutionStatus(ctx, execTask, model.ExecutionStatusRunning, logger)

	// 3. Initialize execution state.
	nodeMap, outEdges, inEdges := buildEdgeMaps(graph)
	state := initState(graph)
	nodeOutputs := make(map[string]map[string]any)
	skipped := make(map[string]bool)
	deadEdges := make(map[string]bool)
	iterationCounts := make(map[string]int)

	// Set input node outputs.
	for i := range graph.Nodes {
		if graph.Nodes[i].Type == model.NodeTypeInput {
			nodeOutputs[graph.Nodes[i].ID] = inputs
		}
	}

	// 4. Create Redis event emitter and step writer.
	stepWriter := NewStepWriter(h.store, 100)
	emitter := &RedisEventEmitter{
		client:      h.rdb,
		executionID: execTask.ExecutionID,
		stepWriter:  stepWriter,
		logger:      logger,
	}

	// Create executor deps for in-process execution.
	providerRegistry := provider.NewDefaultRegistry()
	secretStore := secret.NewEnvSecretStore()
	deps := &executor.ExecutorDeps{
		ProviderRegistry:  providerRegistry,
		SecretStore:       secretStore,
		MCPClientCache:    executor.NewMCPClientCache(),
		APIToolDispatcher: executor.NewAPIToolDispatcher(h.store, logger),
		EventEmitter:      emitter,
		Logger:            logger,
	}

	start := time.Now()
	emitter.Emit(model.ExecutionEvent{
		Type: model.EventExecutionStarted, ExecutionID: execTask.ExecutionID,
		Timestamp: time.Now(),
	})

	// Apply timeout if configured.
	execCtx := ctx
	if execTask.Timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, time.Duration(execTask.Timeout)*time.Second)
		defer cancel()
	}

	// 5. Compute parallel groups.
	groups, err := graphpkg.ParallelGroups(graph)
	if err != nil {
		h.failExecution(ctx, execTask, fmt.Errorf("failed to compute execution order: %w", err), emitter, stepWriter, logger)
		return nil
	}

	// 6. Execute forward pass.
	resultKey := ResultKeyForExecution(execTask.ExecutionID)
	// Set TTL on result key as safety net for cleanup if orchestrator dies.
	ttl := 30 * time.Minute
	if execTask.Timeout > 0 {
		ttl = time.Duration(execTask.Timeout+120) * time.Second
	}
	h.rdb.Expire(ctx, resultKey, ttl)
	err = h.executeForwardPass(execCtx, execTask, graph, groups, nodeMap, outEdges, inEdges, nodeOutputs, state, skipped, deadEdges, deps, emitter, resultKey, logger)
	if err != nil {
		h.failExecution(ctx, execTask, err, emitter, stepWriter, logger)
		return nil
	}

	// 7. Execute loop pass (back-edges).
	err = h.executeLoopPass(execCtx, execTask, graph, nodeMap, outEdges, inEdges, nodeOutputs, state, skipped, iterationCounts, deps, emitter, resultKey, logger)
	if err != nil {
		h.failExecution(ctx, execTask, err, emitter, stepWriter, logger)
		return nil
	}

	// 8. Collect outputs and finalize.
	outputs := h.collectOutputs(graph, nodeOutputs)
	duration := time.Since(start).Milliseconds()

	// Clean up Redis result key.
	h.rdb.Del(ctx, resultKey)

	// Flush step writer.
	stepWriter.Close()

	// Update execution to completed.
	h.completeExecution(ctx, execTask, outputs, state, iterationCounts, duration, emitter, logger)

	return nil
}

// deserializeAndValidate parses the task payload and validates the graph.
func (h *OrchestratorHandler) deserializeAndValidate(task *asynq.Task) (*model.ExecutionTask, *model.Graph, map[string]any, error) {
	var execTask model.ExecutionTask
	if err := json.Unmarshal(task.Payload(), &execTask); err != nil {
		return nil, nil, nil, fmt.Errorf("orchestrator: unmarshal task: %w", err)
	}

	var graph model.Graph
	if err := json.Unmarshal(execTask.Graph, &graph); err != nil {
		return nil, nil, nil, fmt.Errorf("orchestrator: unmarshal graph: %w", err)
	}

	// Validate graph.
	valResult := graphpkg.Validate(&graph)
	if !valResult.Valid {
		return nil, nil, nil, fmt.Errorf("graph validation failed: %s", valResult.Errors[0].Message)
	}

	var inputs map[string]any
	if execTask.Input != nil {
		if err := json.Unmarshal(execTask.Input, &inputs); err != nil {
			return nil, nil, nil, fmt.Errorf("orchestrator: unmarshal inputs: %w", err)
		}
	}
	if inputs == nil {
		inputs = make(map[string]any)
	}

	return &execTask, &graph, inputs, nil
}

// updateExecutionStatus updates the execution status in the store.
func (h *OrchestratorHandler) updateExecutionStatus(ctx context.Context, execTask *model.ExecutionTask, status model.ExecutionStatus, logger *slog.Logger) {
	now := time.Now().UTC()
	exec, err := h.store.GetExecution(ctx, execTask.TenantID, execTask.ExecutionID)
	if err != nil || exec == nil {
		logger.Error("failed to get execution for status update", "error", err)
		return
	}
	exec.Status = status
	if status == model.ExecutionStatusRunning {
		exec.StartedAt = &now
	}
	exec.UpdatedAt = now
	if err := h.store.UpdateExecution(ctx, exec); err != nil {
		logger.Error("failed to update execution status", "error", err, "status", status)
	}
}

// executeForwardPass walks the graph in topological order, executing each group.
func (h *OrchestratorHandler) executeForwardPass(
	ctx context.Context,
	execTask *model.ExecutionTask,
	graph *model.Graph,
	groups [][]string,
	nodeMap map[string]*model.Node,
	outEdges, inEdges map[string][]model.Edge,
	nodeOutputs map[string]map[string]any,
	state map[string]any,
	skipped, deadEdges map[string]bool,
	deps *executor.ExecutorDeps,
	emitter *RedisEventEmitter,
	resultKey string,
	logger *slog.Logger,
) error {
	for _, group := range groups {
		if err := h.executeGroup(ctx, execTask, graph, group, nodeMap, outEdges, inEdges, nodeOutputs, state, skipped, deadEdges, deps, emitter, resultKey, logger); err != nil {
			return err
		}
	}
	return nil
}

// executeGroup fans out tasks for a group of nodes and collects results.
func (h *OrchestratorHandler) executeGroup(
	ctx context.Context,
	execTask *model.ExecutionTask,
	graph *model.Graph,
	group []string,
	nodeMap map[string]*model.Node,
	outEdges, inEdges map[string][]model.Edge,
	nodeOutputs map[string]map[string]any,
	state map[string]any,
	skipped, deadEdges map[string]bool,
	deps *executor.ExecutorDeps,
	emitter *RedisEventEmitter,
	resultKey string,
	logger *slog.Logger,
) error {
	// Determine runnable nodes.
	runnable := h.filterRunnable(group, nodeMap, skipped, nodeOutputs, emitter, execTask.ExecutionID)

	if len(runnable) == 0 {
		return nil
	}

	// Fan-out: dispatch each node.
	pending := 0
	for _, nodeID := range runnable {
		node := nodeMap[nodeID]
		resolvedInputs := resolveInputs(node, nodeOutputs, inEdges[nodeID], state)

		nctx := &executor.NodeContext{
			State: copyMap(state),
			Meta: map[string]any{
				"node_id":      nodeID,
				"node_name":    node.Name,
				"node_type":    node.Type,
				"execution_id": execTask.ExecutionID,
				"graph_id":     graph.ID,
				"graph_name":   graph.Name,
				"tenant_id":    execTask.TenantID,
			},
		}

		emitter.Emit(model.ExecutionEvent{
			Type: model.EventNodeStarted, ExecutionID: execTask.ExecutionID,
			NodeID: nodeID, NodeType: node.Type, Timestamp: time.Now(),
		})

		if err := h.dispatchNode(ctx, execTask, nodeID, node, resolvedInputs, nctx, state, deps, resultKey, logger); err != nil {
			return fmt.Errorf("dispatch node %s: %w", nodeID, err)
		}
		pending++
	}

	// Fan-in: collect all results (both distributed and in-process push to Redis).
	for i := 0; i < pending; i++ {
		result, err := h.waitForResult(ctx, resultKey)
		if err != nil {
			return fmt.Errorf("waiting for result: %w", err)
		}

		h.processNodeResult(result, nodeMap, outEdges, inEdges, nodeOutputs, state, skipped, deadEdges, emitter, execTask.ExecutionID, logger)
	}

	return nil
}

// filterRunnable returns node IDs from the group that should be executed.
func (h *OrchestratorHandler) filterRunnable(
	group []string,
	nodeMap map[string]*model.Node,
	skipped map[string]bool,
	nodeOutputs map[string]map[string]any,
	emitter *RedisEventEmitter,
	executionID string,
) []string {
	var runnable []string
	for _, nodeID := range group {
		node := nodeMap[nodeID]
		if node == nil {
			continue
		}
		if node.Type == model.NodeTypeInput {
			continue
		}
		if skipped[nodeID] {
			emitter.Emit(model.ExecutionEvent{
				Type: model.EventNodeSkipped, ExecutionID: executionID,
				NodeID: nodeID, NodeType: node.Type, Timestamp: time.Now(),
			})
			continue
		}
		if _, done := nodeOutputs[nodeID]; done {
			continue
		}
		runnable = append(runnable, nodeID)
	}
	return runnable
}

// dispatchNode routes a node to the appropriate dispatch method.
// All nodes push results to the Redis result key (whether distributed or in-process).
func (h *OrchestratorHandler) dispatchNode(
	ctx context.Context,
	execTask *model.ExecutionTask,
	nodeID string,
	node *model.Node,
	inputs map[string]any,
	nctx *executor.NodeContext,
	state map[string]any,
	deps *executor.ExecutorDeps,
	resultKey string,
	logger *slog.Logger,
) error {
	switch node.Type {
	case model.NodeTypeLLM:
		return h.dispatchLLMNode(ctx, execTask, nodeID, node, inputs, nctx, deps, resultKey, logger)
	case model.NodeTypeTool:
		return h.dispatchToolNode(ctx, execTask, nodeID, node, inputs, nctx, deps, resultKey, logger)
	case model.NodeTypeForEach, model.NodeTypeSubgraph:
		return h.dispatchComplexNode(ctx, execTask, nodeID, node, inputs, nctx, state, resultKey, logger)
	case model.NodeTypeSuperagent:
		return h.dispatchSuperagentNode(ctx, execTask, nodeID, node, inputs, nctx, state, resultKey, logger)
	case model.NodeTypeAPITool:
		return h.dispatchAPIToolNode(ctx, execTask, nodeID, node, inputs, nctx, deps, resultKey, logger)
	default:
		// Transform, conditional, output — execute in-process, push result to Redis.
		return h.executeInProcess(ctx, execTask, nodeID, node, inputs, nctx, deps, state, resultKey, logger)
	}
}

// dispatchLLMNode creates an LLM call task.
func (h *OrchestratorHandler) dispatchLLMNode(
	ctx context.Context,
	execTask *model.ExecutionTask,
	nodeID string,
	node *model.Node,
	inputs map[string]any,
	nctx *executor.NodeContext,
	deps *executor.ExecutorDeps,
	resultKey string,
	logger *slog.Logger,
) error {
	var cfg model.LLMNodeConfig
	if err := json.Unmarshal(node.Config, &cfg); err != nil {
		return fmt.Errorf("unmarshal llm config: %w", err)
	}

	// Build messages and request the same way the LLM executor does.
	exprCtx := &expression.Context{Input: inputs}
	if nctx != nil {
		exprCtx.State = nctx.State
		exprCtx.Meta = nctx.Meta
	}

	var messages []model.Message
	var systemPrompt, userPrompt string

	if len(cfg.Messages) > 0 {
		for i, msg := range cfg.Messages {
			rendered, err := expression.RenderTemplate(msg.Content, exprCtx)
			if err != nil {
				return fmt.Errorf("rendering message[%d]: %w", i, err)
			}
			messages = append(messages, model.Message{Role: msg.Role, Content: rendered})
		}
		for _, m := range messages {
			if m.Role == "system" && systemPrompt == "" {
				systemPrompt = m.Content
			}
			if m.Role == "user" {
				userPrompt = m.Content
			}
		}
	} else {
		var err error
		userPrompt, err = expression.RenderTemplate(cfg.UserPrompt, exprCtx)
		if err != nil {
			return fmt.Errorf("rendering user_prompt: %w", err)
		}
		if cfg.SystemPrompt != "" {
			systemPrompt, err = expression.RenderTemplate(cfg.SystemPrompt, exprCtx)
			if err != nil {
				return fmt.Errorf("rendering system_prompt: %w", err)
			}
		}
	}

	if cfg.ResponseFormat == model.ResponseFormatJSON && len(cfg.OutputSchema) > 0 {
		systemPrompt += "\n\nYou MUST respond with valid JSON matching this schema:\n" + string(cfg.OutputSchema)
	}

	apiKey := cfg.APIKey
	if apiKey == "" && cfg.APIKeyRef != "" && deps.SecretStore != nil {
		var err error
		apiKey, err = deps.SecretStore.GetSecret(ctx, cfg.APIKeyRef)
		if err != nil {
			return fmt.Errorf("resolving api_key_ref %q: %w", cfg.APIKeyRef, err)
		}
	}

	req := &model.CompletionRequest{
		APIKey:         apiKey,
		Model:          cfg.Model,
		BaseURL:        cfg.BaseURL,
		Messages:       messages,
		SystemPrompt:   systemPrompt,
		UserPrompt:     userPrompt,
		Temperature:    cfg.Temperature,
		MaxTokens:      cfg.MaxTokens,
		ResponseFormat: cfg.ResponseFormat,
		ExtraHeaders:   cfg.ExtraHeaders,
		Tools:          cfg.Tools,
		ToolChoice:     cfg.ToolChoice,
	}

	if len(cfg.OutputSchema) > 0 {
		var schema any
		if err := json.Unmarshal(cfg.OutputSchema, &schema); err == nil {
			req.OutputSchema = schema
		}
	}

	// For tool loop, ensure messages include the initial conversation.
	if cfg.ToolLoop && len(req.Messages) == 0 {
		if systemPrompt != "" {
			req.Messages = append(req.Messages, model.Message{Role: "system", Content: systemPrompt})
		}
		req.Messages = append(req.Messages, model.Message{Role: "user", Content: userPrompt})
	}

	requestID := fmt.Sprintf("%s_llm", nodeID)

	var toolLoop *ToolLoopState
	if cfg.ToolLoop {
		routing, err := resolveToolRoutingForDispatch(ctx, &cfg, nctx, deps)
		if err != nil {
			return fmt.Errorf("resolve tool routing: %w", err)
		}

		// Auto-discover tools if none provided.
		if len(req.Tools) == 0 && deps.MCPClientCache != nil {
			discovered, err := autoDiscoverToolsForDispatch(ctx, routing, deps.MCPClientCache)
			if err != nil {
				return fmt.Errorf("auto-discover tools: %w", err)
			}
			req.Tools = discovered
		}

		// Load messages from state if configured.
		if cfg.MessagesFromState != "" && nctx != nil {
			stateMessages, err := loadMessagesFromStateForDispatch(nctx.State, cfg.MessagesFromState)
			if err != nil {
				return fmt.Errorf("loading messages from state: %w", err)
			}
			req.Messages = append(stateMessages, req.Messages...)
		}

		toolLoop = &ToolLoopState{
			MaxCalls:      defaultMaxToolCalls,
			MaxIterations: defaultMaxLoopIterations,
			Routing:       routing,
			NodeInputs:    inputs,
			NodeState:     nctx.State,
			NodeMeta:      nctx.Meta,
		}
		if cfg.MaxToolCalls != nil {
			toolLoop.MaxCalls = *cfg.MaxToolCalls
		}
		if cfg.MaxLoopIterations != nil {
			toolLoop.MaxIterations = *cfg.MaxLoopIterations
		}
	}

	llmTask := LLMCallTask{
		ExecutionID: execTask.ExecutionID,
		RequestID:   requestID,
		NodeID:      nodeID,
		Provider:    string(cfg.Provider),
		APIKey:      apiKey,
		Request:     req,
		ToolLoop:    toolLoop,
		RetryPolicy: node.RetryPolicy,
		Debug:       execTask.Debug,
	}

	payload, err := json.Marshal(llmTask)
	if err != nil {
		return fmt.Errorf("marshal llm task: %w", err)
	}

	asynqTask := asynq.NewTask(TaskTypeLLMCall, payload, asynq.Queue(QueueNodes))
	if _, err := h.asynqClient.Enqueue(asynqTask); err != nil {
		return fmt.Errorf("enqueue llm task: %w", err)
	}

	logger.Info("dispatched llm call", "node_id", nodeID, "request_id", requestID, "tool_loop", cfg.ToolLoop)
	return nil
}

// dispatchToolNode creates an MCP call task for standalone tool nodes.
func (h *OrchestratorHandler) dispatchToolNode(
	ctx context.Context,
	execTask *model.ExecutionTask,
	nodeID string,
	node *model.Node,
	inputs map[string]any,
	nctx *executor.NodeContext,
	deps *executor.ExecutorDeps,
	resultKey string,
	logger *slog.Logger,
) error {
	var cfg model.ToolNodeConfig
	if err := json.Unmarshal(node.Config, &cfg); err != nil {
		return fmt.Errorf("unmarshal tool config: %w", err)
	}

	// Resolve headers.
	headers := make(map[string]string)
	for _, hc := range cfg.Headers {
		if hc.Value != "" {
			headers[hc.Name] = hc.Value
		} else if hc.SecretRef != "" && deps.SecretStore != nil {
			val, err := deps.SecretStore.GetSecret(ctx, hc.SecretRef)
			if err == nil {
				headers[hc.Name] = val
			}
		} else if hc.FromInput != "" {
			if v, ok := inputs[hc.FromInput]; ok {
				headers[hc.Name] = fmt.Sprintf("%v", v)
			}
		}
	}

	requestID := fmt.Sprintf("%s_tool", nodeID)

	mcpTask := MCPCallTask{
		ExecutionID:    execTask.ExecutionID,
		RequestID:      requestID,
		NodeID:         nodeID,
		Operation:      "call_tool",
		MCPURL:         cfg.MCPURL,
		Headers:        headers,
		ToolName:       cfg.ToolName,
		Arguments:      inputs,
		TimeoutSeconds: 30,
		ResultKey:      resultKey,
		RetryPolicy:    node.RetryPolicy,
	}

	payload, err := json.Marshal(mcpTask)
	if err != nil {
		return fmt.Errorf("marshal mcp task: %w", err)
	}

	asynqTask := asynq.NewTask(TaskTypeMCPCall, payload, asynq.Queue(QueueNodes))
	if _, err := h.asynqClient.Enqueue(asynqTask); err != nil {
		return fmt.Errorf("enqueue mcp task: %w", err)
	}

	logger.Info("dispatched tool node", "node_id", nodeID, "tool_name", cfg.ToolName)
	return nil
}

// dispatchAPIToolNode creates a node:api-call task for standalone api_tool nodes.
func (h *OrchestratorHandler) dispatchAPIToolNode(
	ctx context.Context,
	execTask *model.ExecutionTask,
	nodeID string,
	node *model.Node,
	inputs map[string]any,
	nctx *executor.NodeContext,
	deps *executor.ExecutorDeps,
	resultKey string,
	logger *slog.Logger,
) error {
	var cfg model.APIToolNodeConfig
	if err := json.Unmarshal(node.Config, &cfg); err != nil {
		return fmt.Errorf("unmarshal api_tool config: %w", err)
	}

	requestID := fmt.Sprintf("%s_api_tool", nodeID)

	apiTask := APICallTask{
		ExecutionID:    execTask.ExecutionID,
		RequestID:      requestID,
		NodeID:         nodeID,
		TenantID:       execTask.TenantID,
		APIToolID:      cfg.APIToolID,
		APIEndpoint:    cfg.Endpoint,
		Headers:        cfg.Headers,
		ToolName:       cfg.Endpoint,
		Arguments:      inputs,
		TimeoutSeconds: 30,
		ResultKey:      resultKey,
		RetryPolicy:    node.RetryPolicy,
	}

	payload, err := json.Marshal(apiTask)
	if err != nil {
		return fmt.Errorf("marshal api_tool task: %w", err)
	}

	asynqTask := asynq.NewTask(TaskTypeAPICall, payload, asynq.Queue(QueueNodes))
	if _, err := h.asynqClient.Enqueue(asynqTask); err != nil {
		return fmt.Errorf("enqueue api_tool task: %w", err)
	}

	logger.Info("dispatched api_tool node", "node_id", nodeID, "endpoint", cfg.Endpoint)
	return nil
}

// dispatchComplexNode creates a node:run task for forEach and subgraph nodes.
func (h *OrchestratorHandler) dispatchComplexNode(
	ctx context.Context,
	execTask *model.ExecutionTask,
	nodeID string,
	node *model.Node,
	inputs map[string]any,
	nctx *executor.NodeContext,
	state map[string]any,
	resultKey string,
	logger *slog.Logger,
) error {
	requestID := fmt.Sprintf("%s_run", nodeID)

	nodeRunTask := NodeRunTask{
		ExecutionID: execTask.ExecutionID,
		RequestID:   requestID,
		NodeID:      nodeID,
		NodeType:    node.Type,
		NodeConfig:  node.Config,
		Inputs:      inputs,
		State:       state,
		Meta:        nctx.Meta,
		ResultKey:   resultKey,
		Debug:       execTask.Debug,
	}

	payload, err := json.Marshal(nodeRunTask)
	if err != nil {
		return fmt.Errorf("marshal node run task: %w", err)
	}

	asynqTask := asynq.NewTask(TaskTypeNodeRun, payload, asynq.Queue(QueueNodes))
	if _, err := h.asynqClient.Enqueue(asynqTask); err != nil {
		return fmt.Errorf("enqueue node run task: %w", err)
	}

	logger.Info("dispatched complex node", "node_id", nodeID, "node_type", node.Type)
	return nil
}

// dispatchSuperagentNode creates a node:superagent task.
// The superagent handler stays alive as a coordinator and dispatches
// every LLM call and MCP call as a separate asynq task.
func (h *OrchestratorHandler) dispatchSuperagentNode(
	ctx context.Context,
	execTask *model.ExecutionTask,
	nodeID string,
	node *model.Node,
	inputs map[string]any,
	nctx *executor.NodeContext,
	state map[string]any,
	resultKey string,
	logger *slog.Logger,
) error {
	requestID := fmt.Sprintf("%s_sa", nodeID)

	nodeRunTask := NodeRunTask{
		ExecutionID: execTask.ExecutionID,
		RequestID:   requestID,
		NodeID:      nodeID,
		NodeType:    node.Type,
		NodeConfig:  node.Config,
		Inputs:      inputs,
		State:       state,
		Meta:        nctx.Meta,
		OutputPorts: node.OutputPorts,
		ResultKey:   resultKey,
		Debug:       execTask.Debug,
	}

	payload, err := json.Marshal(nodeRunTask)
	if err != nil {
		return fmt.Errorf("marshal superagent task: %w", err)
	}

	// Superagent tasks get longer timeouts.
	var sa struct {
		TimeoutSeconds *int `json:"timeout_seconds"`
	}
	timeout := 11 * time.Minute
	if json.Unmarshal(node.Config, &sa) == nil && sa.TimeoutSeconds != nil {
		timeout = time.Duration(*sa.TimeoutSeconds+60) * time.Second
	}

	asynqTask := asynq.NewTask(TaskTypeSuperagent, payload,
		asynq.Queue(QueueNodes),
		asynq.Timeout(timeout),
	)
	if _, err := h.asynqClient.Enqueue(asynqTask); err != nil {
		return fmt.Errorf("enqueue superagent task: %w", err)
	}

	logger.Info("dispatched superagent node", "node_id", nodeID)
	return nil
}

// executeInProcess runs a node directly in the orchestrator process.
// Used for transform, conditional, output, and other pure-computation nodes.
func (h *OrchestratorHandler) executeInProcess(
	ctx context.Context,
	execTask *model.ExecutionTask,
	nodeID string,
	node *model.Node,
	inputs map[string]any,
	nctx *executor.NodeContext,
	deps *executor.ExecutorDeps,
	state map[string]any,
	resultKey string,
	logger *slog.Logger,
) error {
	nodeStart := time.Now()

	registry := executor.NewDefaultRegistry()
	exec, err := registry.Get(node.Type)
	if err != nil {
		logger.Error("no executor for node type", "node_id", nodeID, "error", err)
		return nil // skip unknown node types
	}

	result, err := exec.Execute(ctx, node, inputs, nctx, deps)
	durationMs := time.Since(nodeStart).Milliseconds()

	if err != nil {
		logger.Error("in-process node failed", "node_id", nodeID, "error", err, "duration_ms", durationMs)
		deps.EventEmitter.Emit(model.ExecutionEvent{
			Type: model.EventNodeFailed, ExecutionID: execTask.ExecutionID,
			NodeID: nodeID, NodeType: node.Type, Timestamp: time.Now(),
			DurationMs: durationMs,
			Error:      &model.ExecutionError{Code: "NODE_FAILED", Message: err.Error(), NodeID: nodeID},
		})
		return fmt.Errorf("node %s failed: %w", nodeID, err)
	}

	logger.Info("in-process node completed", "node_id", nodeID, "duration_ms", durationMs)

	// Push result to Redis so the fan-in loop handles all nodes uniformly.
	nodeResult := NodeTaskResult{
		RequestID: fmt.Sprintf("%s_inproc", nodeID),
		NodeID:    nodeID,
		Status:    "completed",
	}
	if result != nil {
		nodeResult.Outputs = result.Outputs
	}

	resultJSON, _ := json.Marshal(nodeResult)
	h.rdb.LPush(ctx, resultKey, string(resultJSON))

	return nil
}

// maxBRPOPRetries is the number of times to retry BRPOP on timeout before failing.
// asynq automatically retries crashed node tasks (default MaxRetry=25), so the
// orchestrator just needs to wait long enough for re-dispatched tasks to produce results.
const maxBRPOPRetries = 3

// waitForResult BRPOPs a NodeTaskResult from the execution results key.
// On BRPOP timeout, it retries up to maxBRPOPRetries times to handle cases where
// a worker crashed and asynq is re-dispatching the task. Context cancellation
// (graph timeout or shutdown) causes immediate failure without retrying.
func (h *OrchestratorHandler) waitForResult(ctx context.Context, resultKey string) (*NodeTaskResult, error) {
	timeout := h.brpopTimeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}

	for attempt := 0; attempt <= maxBRPOPRetries; attempt++ {
		result, err := h.rdb.BRPop(ctx, timeout, resultKey).Result()
		if err == nil {
			if len(result) < 2 {
				return nil, fmt.Errorf("brpop result: unexpected length")
			}
			var taskResult NodeTaskResult
			if err := json.Unmarshal([]byte(result[1]), &taskResult); err != nil {
				return nil, fmt.Errorf("unmarshal result: %w", err)
			}
			return &taskResult, nil
		}

		// Context cancelled (graph timeout or shutdown) — fail immediately.
		if ctx.Err() != nil {
			return nil, fmt.Errorf("brpop result: %w", ctx.Err())
		}

		// BRPOP timeout — log and retry if attempts remain.
		if attempt < maxBRPOPRetries {
			h.logger.Warn("brpop timeout, retrying",
				"result_key", resultKey, "attempt", attempt+1, "max", maxBRPOPRetries)
		}
	}

	return nil, fmt.Errorf("brpop result: timed out after %d attempts", maxBRPOPRetries+1)
}

// processNodeResult applies a completed node result to the execution state.
func (h *OrchestratorHandler) processNodeResult(
	result *NodeTaskResult,
	nodeMap map[string]*model.Node,
	outEdges, inEdges map[string][]model.Edge,
	nodeOutputs map[string]map[string]any,
	state map[string]any,
	skipped, deadEdges map[string]bool,
	emitter *RedisEventEmitter,
	executionID string,
	logger *slog.Logger,
) {
	node := nodeMap[result.NodeID]
	if node == nil {
		logger.Error("unknown node in result", "node_id", result.NodeID)
		return
	}

	if result.Status == "failed" {
		logger.Error("node failed", "node_id", result.NodeID, "error", result.Error)
		emitter.Emit(model.ExecutionEvent{
			Type: model.EventNodeFailed, ExecutionID: executionID,
			NodeID: result.NodeID, NodeType: node.Type, Timestamp: time.Now(),
			Error: &model.ExecutionError{Code: "NODE_FAILED", Message: result.Error, NodeID: result.NodeID},
		})
		return
	}

	// Store outputs.
	nodeOutputs[result.NodeID] = result.Outputs

	// Apply state writes.
	applyStateWrites(node, result.Outputs, state)

	// Emit completion event.
	outputJSON, _ := json.Marshal(result.Outputs)
	emitter.Emit(model.ExecutionEvent{
		Type: model.EventNodeCompleted, ExecutionID: executionID,
		NodeID: result.NodeID, NodeType: node.Type, Timestamp: time.Now(),
		Output: outputJSON, LLMUsage: result.LLMUsage, LLMDebug: result.LLMDebug,
	})

	// Handle conditional skip propagation.
	if node.Type == model.NodeTypeConditional && result.Outputs != nil {
		propagateSkips(node, result.Outputs, outEdges[result.NodeID], nodeMap, outEdges, inEdges, deadEdges, skipped)
	}
}

// executeLoopPass handles back-edge loop iterations.
func (h *OrchestratorHandler) executeLoopPass(
	ctx context.Context,
	execTask *model.ExecutionTask,
	graph *model.Graph,
	nodeMap map[string]*model.Node,
	outEdges, inEdges map[string][]model.Edge,
	nodeOutputs map[string]map[string]any,
	state map[string]any,
	skipped map[string]bool,
	iterationCounts map[string]int,
	deps *executor.ExecutorDeps,
	emitter *RedisEventEmitter,
	resultKey string,
	logger *slog.Logger,
) error {
	// Collect back-edges.
	var backEdges []model.Edge
	for _, edge := range graph.Edges {
		if edge.BackEdge {
			backEdges = append(backEdges, edge)
		}
	}
	if len(backEdges) == 0 {
		return nil
	}

	// Get topological order for loop body identification.
	topoOrder, err := graphpkg.TopologicalSort(graph)
	if err != nil {
		return fmt.Errorf("loop detection: %w", err)
	}
	topoIndex := make(map[string]int)
	for i, id := range topoOrder {
		topoIndex[id] = i
	}

	type loopInfo struct {
		edge    model.Edge
		bodyIDs []string
		maxIter int
	}

	var loops []loopInfo
	for _, be := range backEdges {
		targetIdx, targetOK := topoIndex[be.TargetNodeID]
		sourceIdx, sourceOK := topoIndex[be.SourceNodeID]
		if !targetOK || !sourceOK {
			continue
		}

		var body []string
		for _, id := range topoOrder {
			idx := topoIndex[id]
			if idx >= targetIdx && idx <= sourceIdx {
				body = append(body, id)
			}
		}

		maxIter := 10
		if be.MaxIterations != nil && *be.MaxIterations > 0 {
			maxIter = *be.MaxIterations
		}

		loops = append(loops, loopInfo{edge: be, bodyIDs: body, maxIter: maxIter})
	}

	// Find downstream nodes for each loop.
	findDownstream := func(bodyIDs []string) []string {
		bodySet := make(map[string]bool)
		for _, id := range bodyIDs {
			bodySet[id] = true
		}
		visited := make(map[string]bool)
		queue := make([]string, 0)
		for _, id := range bodyIDs {
			for _, edge := range outEdges[id] {
				if !edge.BackEdge && !bodySet[edge.TargetNodeID] && !visited[edge.TargetNodeID] {
					visited[edge.TargetNodeID] = true
					queue = append(queue, edge.TargetNodeID)
				}
			}
		}
		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			for _, edge := range outEdges[curr] {
				if !edge.BackEdge && !bodySet[edge.TargetNodeID] && !visited[edge.TargetNodeID] {
					visited[edge.TargetNodeID] = true
					queue = append(queue, edge.TargetNodeID)
				}
			}
		}
		var result []string
		for _, id := range topoOrder {
			if visited[id] {
				result = append(result, id)
			}
		}
		return result
	}

	// Execute loops.
	for _, loop := range loops {
		loopRan := false
		for iteration := 1; iteration <= loop.maxIter; iteration++ {
			// Evaluate back-edge condition.
			sourceOutputs := nodeOutputs[loop.edge.SourceNodeID]
			condResult, err := orchestrator.EvalCondition(loop.edge.Condition, sourceOutputs, copyMap(state))
			if err != nil {
				return fmt.Errorf("loop condition evaluation: %w", err)
			}
			if !condResult {
				break
			}

			iterationCounts[loop.edge.ID] = iteration

			// Save back-edge value before clearing.
			var backEdgeValue any
			var hasBackEdgeValue bool
			if srcOut := nodeOutputs[loop.edge.SourceNodeID]; srcOut != nil {
				if val, ok := srcOut[loop.edge.SourcePort]; ok {
					backEdgeValue = val
					hasBackEdgeValue = true
				}
			}

			// Clear loop body outputs.
			for _, nodeID := range loop.bodyIDs {
				delete(nodeOutputs, nodeID)
				delete(skipped, nodeID)
			}

			// Re-execute loop body nodes sequentially.
			for _, nodeID := range loop.bodyIDs {
				node := nodeMap[nodeID]
				if node == nil || node.Type == model.NodeTypeInput {
					continue
				}

				resolvedInputs := resolveInputs(node, nodeOutputs, inEdges[nodeID], state)
				if hasBackEdgeValue && loop.edge.TargetNodeID == nodeID {
					resolvedInputs[loop.edge.TargetPort] = backEdgeValue
				}

				nctx := &executor.NodeContext{
					State: copyMap(state),
					Meta: map[string]any{
						"node_id":      nodeID,
						"node_name":    node.Name,
						"node_type":    node.Type,
						"execution_id": execTask.ExecutionID,
						"graph_id":     graph.ID,
						"graph_name":   graph.Name,
						"iteration":    iteration,
					},
				}

				// Dispatch and wait for result.
				if err := h.dispatchNode(ctx, execTask, nodeID, node, resolvedInputs, nctx, state, deps, resultKey, logger); err != nil {
					return fmt.Errorf("loop iteration %d, node %s: %w", iteration, nodeID, err)
				}

				result, err := h.waitForResult(ctx, resultKey)
				if err != nil {
					return fmt.Errorf("loop iteration %d, waiting for node %s: %w", iteration, nodeID, err)
				}
				if result.Status == "failed" {
					return fmt.Errorf("loop node %s failed on iteration %d: %s", nodeID, iteration, result.Error)
				}
				nodeOutputs[result.NodeID] = result.Outputs
				applyStateWrites(node, result.Outputs, state)

				// Emit completion event.
				outputJSON, _ := json.Marshal(nodeOutputs[nodeID])
				emitter.Emit(model.ExecutionEvent{
					Type: model.EventNodeCompleted, ExecutionID: execTask.ExecutionID,
					NodeID: nodeID, NodeType: node.Type, Timestamp: time.Now(),
					Output: outputJSON, Iteration: iteration, LLMUsage: result.LLMUsage, LLMDebug: result.LLMDebug,
				})
			}
			loopRan = true
		}

		// After loop, re-execute downstream nodes.
		if loopRan {
			downstream := findDownstream(loop.bodyIDs)
			for _, nodeID := range downstream {
				node := nodeMap[nodeID]
				if node == nil {
					continue
				}

				resolvedInputs := resolveInputs(node, nodeOutputs, inEdges[nodeID], state)
				nctx := &executor.NodeContext{
					State: copyMap(state),
					Meta: map[string]any{
						"node_id":      nodeID,
						"node_name":    node.Name,
						"node_type":    node.Type,
						"execution_id": execTask.ExecutionID,
						"graph_id":     graph.ID,
						"graph_name":   graph.Name,
					},
				}

				if err := h.dispatchNode(ctx, execTask, nodeID, node, resolvedInputs, nctx, state, deps, resultKey, logger); err != nil {
					return fmt.Errorf("downstream node %s: %w", nodeID, err)
				}

				result, err := h.waitForResult(ctx, resultKey)
				if err != nil {
					return fmt.Errorf("waiting for downstream node %s: %w", nodeID, err)
				}
				nodeOutputs[result.NodeID] = result.Outputs
				applyStateWrites(node, result.Outputs, state)
			}
		}
	}

	return nil
}

// collectOutputs gathers values from output nodes.
func (h *OrchestratorHandler) collectOutputs(graph *model.Graph, nodeOutputs map[string]map[string]any) map[string]any {
	outputs := make(map[string]any)
	for _, node := range graph.Nodes {
		if node.Type == model.NodeTypeOutput {
			if out, ok := nodeOutputs[node.ID]; ok {
				for k, v := range out {
					outputs[k] = v
				}
			}
		}
	}
	return outputs
}

// completeExecution updates the execution to completed status.
func (h *OrchestratorHandler) completeExecution(
	ctx context.Context,
	execTask *model.ExecutionTask,
	outputs, state map[string]any,
	iterationCounts map[string]int,
	durationMs int64,
	emitter *RedisEventEmitter,
	logger *slog.Logger,
) {
	completedAt := time.Now().UTC()
	exec, err := h.store.GetExecution(ctx, execTask.TenantID, execTask.ExecutionID)
	if err != nil || exec == nil {
		logger.Error("failed to get execution for completion", "error", err)
		return
	}

	exec.Status = model.ExecutionStatusCompleted
	exec.CompletedAt = &completedAt
	exec.UpdatedAt = completedAt
	exec.IterationCounts = iterationCounts

	if outputs != nil {
		outputJSON, _ := json.Marshal(outputs)
		exec.Output = outputJSON
	}
	if state != nil {
		stateJSON, _ := json.Marshal(state)
		exec.State = stateJSON
	}

	if err := h.store.UpdateExecution(ctx, exec); err != nil {
		logger.Error("failed to update execution to completed", "error", err)
	}

	outputJSON, _ := json.Marshal(outputs)
	stateJSON, _ := json.Marshal(state)
	emitter.Emit(model.ExecutionEvent{
		Type: model.EventExecutionCompleted, ExecutionID: execTask.ExecutionID,
		Timestamp: completedAt, Output: outputJSON, State: stateJSON,
		DurationMs: durationMs, Status: string(model.ExecutionStatusCompleted),
	})

	logger.Info("execution completed", "duration_ms", durationMs)
}

// failExecution marks an execution as failed.
func (h *OrchestratorHandler) failExecution(
	ctx context.Context,
	execTask *model.ExecutionTask,
	execErr error,
	emitter *RedisEventEmitter,
	stepWriter *StepWriter,
	logger *slog.Logger,
) {
	logger.Error("execution failed", "error", execErr)

	stepWriter.Close()

	now := time.Now().UTC()
	exec, err := h.store.GetExecution(ctx, execTask.TenantID, execTask.ExecutionID)
	if err != nil || exec == nil {
		logger.Error("failed to get execution for failure update", "error", err)
		return
	}

	exec.Status = model.ExecutionStatusFailed
	exec.CompletedAt = &now
	exec.UpdatedAt = now
	exec.Error = &model.ExecutionError{
		Code:    "EXECUTION_FAILED",
		Message: execErr.Error(),
	}
	if err := h.store.UpdateExecution(ctx, exec); err != nil {
		logger.Error("failed to update execution to failed", "error", err)
	}

	emitter.Emit(model.ExecutionEvent{
		Type: model.EventExecutionFailed, ExecutionID: execTask.ExecutionID,
		Timestamp: now, Status: string(model.ExecutionStatusFailed),
		Error: exec.Error,
	})

	// Clean up Redis result key.
	h.rdb.Del(ctx, ResultKeyForExecution(execTask.ExecutionID))
}

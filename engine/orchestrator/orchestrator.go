// Package orchestrator implements the graph execution engine.
// It walks a graph in topological order, executes nodes, manages state,
// handles loops (back-edges), conditional skip propagation, and emits events.
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/brockleyai/brockleyai/engine/executor"
	"github.com/brockleyai/brockleyai/engine/expression"
	graphpkg "github.com/brockleyai/brockleyai/engine/graph"
	"github.com/brockleyai/brockleyai/internal/model"
)

// ExecutionResult is the output of a graph execution.
type ExecutionResult struct {
	Outputs         map[string]any `json:"outputs"`
	State           map[string]any `json:"state"`
	Steps           []StepRecord   `json:"steps"`
	IterationCounts map[string]int `json:"iteration_counts"`
	DurationMs      int64          `json:"duration_ms"`
	Status          string         `json:"status"`
}

// StepRecord is a recorded node execution for the result.
type StepRecord struct {
	NodeID     string         `json:"node_id"`
	NodeType   string         `json:"node_type"`
	Iteration  int            `json:"iteration"`
	Status     string         `json:"status"`
	Inputs     map[string]any `json:"inputs,omitempty"`
	Outputs    map[string]any `json:"outputs,omitempty"`
	DurationMs int64          `json:"duration_ms"`
	Error      string         `json:"error,omitempty"`
}

// Orchestrator executes a graph using registered node executors.
type Orchestrator struct {
	executors *executor.Registry
	emitter   model.EventEmitter
	metrics   model.MetricsCollector
	logger    *slog.Logger
}

// New creates a new Orchestrator.
func New(executors *executor.Registry, emitter model.EventEmitter, metrics model.MetricsCollector, logger *slog.Logger) *Orchestrator {
	return &Orchestrator{
		executors: executors,
		emitter:   emitter,
		metrics:   metrics,
		logger:    logger,
	}
}

// Execute runs a graph with the given inputs.
func (o *Orchestrator) Execute(ctx context.Context, g *model.Graph, inputs map[string]any, deps *executor.ExecutorDeps) (*ExecutionResult, error) {
	start := time.Now()
	execID := ctx.Value("execution_id")
	if execID == nil {
		execID = "local"
	}

	logger := o.logger.With("execution_id", execID, "graph_id", g.ID, "graph_name", g.Name)
	logger.Info("execution started")

	o.metrics.ExecutionStarted(g.ID, g.Name)
	o.emitter.Emit(model.ExecutionEvent{
		Type: model.EventExecutionStarted, ExecutionID: fmt.Sprint(execID),
		Timestamp: time.Now(),
	})

	// Validate first
	valResult := graphpkg.Validate(g)
	if !valResult.Valid {
		errMsg := fmt.Sprintf("graph validation failed: %d errors", len(valResult.Errors))
		return &ExecutionResult{Status: "failed"}, fmt.Errorf("%s: %s", errMsg, valResult.Errors[0].Message)
	}

	// Build node map
	nodeMap := make(map[string]*model.Node)
	for i := range g.Nodes {
		nodeMap[g.Nodes[i].ID] = &g.Nodes[i]
	}

	// Initialize state
	state := o.initState(g)

	// Build edge maps
	outEdges := make(map[string][]model.Edge) // nodeID -> outgoing edges
	inEdges := make(map[string][]model.Edge)  // nodeID -> incoming edges
	for _, edge := range g.Edges {
		outEdges[edge.SourceNodeID] = append(outEdges[edge.SourceNodeID], edge)
		inEdges[edge.TargetNodeID] = append(inEdges[edge.TargetNodeID], edge)
	}

	// Track outputs per node
	nodeOutputs := make(map[string]map[string]any) // nodeID -> portName -> value
	var steps []StepRecord
	var mu sync.Mutex
	skipped := make(map[string]bool)
	deadEdges := make(map[string]bool)
	iterationCounts := make(map[string]int)

	// Set input node outputs from execution inputs
	for i := range g.Nodes {
		if g.Nodes[i].Type == model.NodeTypeInput {
			nodeOutputs[g.Nodes[i].ID] = inputs
		}
	}

	// Get execution order (parallel groups)
	groups, err := graphpkg.ParallelGroups(g)
	if err != nil {
		return nil, fmt.Errorf("failed to compute execution order: %w", err)
	}

	// Execute in topological order by groups
	for _, group := range groups {
		var wg sync.WaitGroup
		for _, nodeID := range group {
			node := nodeMap[nodeID]
			if node == nil {
				continue
			}

			// Skip input nodes (already have outputs set)
			if node.Type == model.NodeTypeInput {
				continue
			}

			// Skip if marked as skipped (conditional branch not taken)
			if skipped[nodeID] {
				mu.Lock()
				steps = append(steps, StepRecord{
					NodeID: nodeID, NodeType: node.Type, Status: "skipped",
				})
				mu.Unlock()
				o.emitter.Emit(model.ExecutionEvent{
					Type: model.EventNodeSkipped, ExecutionID: fmt.Sprint(execID),
					NodeID: nodeID, NodeType: node.Type, Timestamp: time.Now(),
				})
				continue
			}

			wg.Add(1)
			go func(nodeID string, node *model.Node) {
				defer wg.Done()

				nodeStart := time.Now()
				logger := logger.With("node_id", nodeID, "node_type", node.Type)
				logger.Info("node started")

				o.metrics.NodeStarted(g.ID, nodeID, node.Type)
				o.emitter.Emit(model.ExecutionEvent{
					Type: model.EventNodeStarted, ExecutionID: fmt.Sprint(execID),
					NodeID: nodeID, NodeType: node.Type, Timestamp: time.Now(),
				})

				// Resolve input ports
				resolvedInputs := o.resolveInputs(node, nodeOutputs, inEdges[nodeID], state, &mu)

				// Build NodeContext with state snapshot and metadata.
				mu.Lock()
				nctx := &executor.NodeContext{
					State: copyMap(state),
					Meta: map[string]any{
						"node_id":      nodeID,
						"node_name":    node.Name,
						"node_type":    node.Type,
						"execution_id": fmt.Sprint(execID),
						"graph_id":     g.ID,
						"graph_name":   g.Name,
					},
				}
				mu.Unlock()

				// Execute the node
				exec, execErr := o.executors.Get(node.Type)
				if execErr != nil {
					logger.Error("no executor for node type", "error", execErr)
					mu.Lock()
					steps = append(steps, StepRecord{
						NodeID: nodeID, NodeType: node.Type, Status: "failed",
						Error: execErr.Error(), DurationMs: time.Since(nodeStart).Milliseconds(),
					})
					mu.Unlock()
					return
				}

				result, execErr := exec.Execute(ctx, node, resolvedInputs, nctx, deps)

				durationMs := time.Since(nodeStart).Milliseconds()

				if execErr != nil {
					logger.Error("node failed", "error", execErr, "duration_ms", durationMs)
					o.metrics.NodeCompleted(g.ID, nodeID, node.Type, durationMs, "failed")

					mu.Lock()
					steps = append(steps, StepRecord{
						NodeID: nodeID, NodeType: node.Type, Status: "failed",
						Inputs: resolvedInputs, Error: execErr.Error(), DurationMs: durationMs,
					})
					mu.Unlock()

					o.emitter.Emit(model.ExecutionEvent{
						Type: model.EventNodeFailed, ExecutionID: fmt.Sprint(execID),
						NodeID: nodeID, NodeType: node.Type, Timestamp: time.Now(),
						DurationMs: durationMs,
						Error:      &model.ExecutionError{Code: "NODE_FAILED", Message: execErr.Error(), NodeID: nodeID},
					})
					return
				}

				logger.Info("node completed", "duration_ms", durationMs)
				o.metrics.NodeCompleted(g.ID, nodeID, node.Type, durationMs, "completed")

				// Store outputs
				mu.Lock()
				if result != nil {
					nodeOutputs[nodeID] = result.Outputs
				}

				// Apply state writes
				o.applyStateWrites(node, result, state)

				steps = append(steps, StepRecord{
					NodeID: nodeID, NodeType: node.Type, Status: "completed",
					Inputs: resolvedInputs, Outputs: result.Outputs, DurationMs: durationMs,
				})
				mu.Unlock()

				// Emit completion
				outputJSON, _ := json.Marshal(result.Outputs)
				inputJSON, _ := json.Marshal(resolvedInputs)
				o.emitter.Emit(model.ExecutionEvent{
					Type: model.EventNodeCompleted, ExecutionID: fmt.Sprint(execID),
					NodeID: nodeID, NodeType: node.Type, Timestamp: time.Now(),
					Input: inputJSON, Output: outputJSON, DurationMs: durationMs,
				})

				// Handle conditional skip propagation
				if node.Type == model.NodeTypeConditional && result != nil {
					o.propagateSkips(node, result.Outputs, outEdges[nodeID], nodeMap, outEdges, inEdges, deadEdges, skipped, &mu)
				}

			}(nodeID, node)
		}
		wg.Wait()
	}

	// Execute loops (back-edges).
	if loopErr := o.executeLoops(ctx, g, deps, nodeMap, nodeOutputs, inEdges, outEdges, state, &steps, iterationCounts, skipped, &mu, execID, logger); loopErr != nil {
		return &ExecutionResult{Status: "failed", Steps: steps}, loopErr
	}

	// Collect output node results
	outputs := make(map[string]any)
	for _, node := range g.Nodes {
		if node.Type == model.NodeTypeOutput {
			if out, ok := nodeOutputs[node.ID]; ok {
				for k, v := range out {
					outputs[k] = v
				}
			}
		}
	}

	duration := time.Since(start).Milliseconds()
	logger.Info("execution completed", "duration_ms", duration)
	o.metrics.ExecutionCompleted(g.ID, g.Name, duration, "completed")

	outputJSON, _ := json.Marshal(outputs)
	stateJSON, _ := json.Marshal(state)
	o.emitter.Emit(model.ExecutionEvent{
		Type: model.EventExecutionCompleted, ExecutionID: fmt.Sprint(execID),
		Timestamp: time.Now(), Output: outputJSON, State: stateJSON,
		DurationMs: duration, Status: "completed",
	})

	return &ExecutionResult{
		Outputs:         outputs,
		State:           state,
		Steps:           steps,
		IterationCounts: iterationCounts,
		DurationMs:      duration,
		Status:          "completed",
	}, nil
}

// copyMap creates a shallow copy of a map.
func copyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	cp := make(map[string]any, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

func (o *Orchestrator) initState(g *model.Graph) map[string]any {
	state := make(map[string]any)
	if g.State == nil {
		return state
	}
	for _, field := range g.State.Fields {
		if field.Initial != nil {
			var v any
			json.Unmarshal(field.Initial, &v)
			state[field.Name] = v
		} else {
			// Zero values by type
			var schema map[string]any
			json.Unmarshal(field.Schema, &schema)
			typ, _ := schema["type"].(string)
			switch typ {
			case "array":
				state[field.Name] = []any{}
			case "object":
				state[field.Name] = map[string]any{}
			case "string":
				state[field.Name] = ""
			case "integer", "number":
				state[field.Name] = 0
			case "boolean":
				state[field.Name] = false
			default:
				state[field.Name] = nil
			}
		}
	}
	return state
}

func (o *Orchestrator) resolveInputs(node *model.Node, nodeOutputs map[string]map[string]any, incoming []model.Edge, state map[string]any, mu *sync.Mutex) map[string]any {
	inputs := make(map[string]any)

	mu.Lock()
	defer mu.Unlock()

	// Priority 3: defaults
	for _, port := range node.InputPorts {
		if port.Default != nil {
			var v any
			json.Unmarshal(port.Default, &v)
			inputs[port.Name] = v
		}
	}

	// Priority 2: state reads
	for _, sr := range node.StateReads {
		if v, ok := state[sr.StateField]; ok {
			inputs[sr.Port] = v
		}
	}

	// Priority 1: edges (highest priority)
	for _, edge := range incoming {
		if edge.BackEdge {
			continue // back-edges handled in loop logic
		}
		srcOutputs, ok := nodeOutputs[edge.SourceNodeID]
		if !ok {
			continue
		}
		if v, ok := srcOutputs[edge.SourcePort]; ok {
			inputs[edge.TargetPort] = v
		}
	}

	return inputs
}

func (o *Orchestrator) applyStateWrites(node *model.Node, result *executor.NodeResult, state map[string]any) {
	if result == nil {
		return
	}
	for _, sw := range node.StateWrites {
		val, ok := result.Outputs[sw.Port]
		if !ok {
			continue
		}
		// Find the reducer for this state field
		// For simplicity, use replace as default
		// A full implementation would look up the field's reducer from the graph state schema
		existing := state[sw.StateField]
		switch arr := existing.(type) {
		case []any:
			// Assume append reducer for arrays
			state[sw.StateField] = append(arr, val)
		default:
			// Replace
			state[sw.StateField] = val
		}
	}
}

func (o *Orchestrator) propagateSkips(condNode *model.Node, outputs map[string]any, outgoing []model.Edge, nodeMap map[string]*model.Node, allOutEdges map[string][]model.Edge, allInEdges map[string][]model.Edge, deadEdges map[string]bool, skipped map[string]bool, mu *sync.Mutex) {
	mu.Lock()
	defer mu.Unlock()

	// Find which output ports have nil values (branches not taken)
	for _, edge := range outgoing {
		if outputs[edge.SourcePort] == nil {
			// Mark this edge as dead
			deadEdges[edge.ID] = true
			// Try to skip the target node
			o.markSkipped(edge.TargetNodeID, allOutEdges, allInEdges, deadEdges, skipped)
		}
	}
}

func (o *Orchestrator) markSkipped(nodeID string, outEdges map[string][]model.Edge, inEdges map[string][]model.Edge, deadEdges map[string]bool, skipped map[string]bool) {
	if skipped[nodeID] {
		return
	}
	// Only skip this node if ALL its non-back-edge incoming edges are dead or from skipped nodes.
	for _, inEdge := range inEdges[nodeID] {
		if inEdge.BackEdge {
			continue
		}
		if !deadEdges[inEdge.ID] && !skipped[inEdge.SourceNodeID] {
			// There's a live incoming edge — don't skip this node.
			return
		}
	}
	skipped[nodeID] = true
	// Mark all outgoing edges as dead and propagate
	for _, edge := range outEdges[nodeID] {
		deadEdges[edge.ID] = true
		o.markSkipped(edge.TargetNodeID, outEdges, inEdges, deadEdges, skipped)
	}
}

// Convenience function for direct execution without an Orchestrator instance.
func Execute(ctx context.Context, g *model.Graph, inputs map[string]any, deps *executor.ExecutorDeps, executors *executor.Registry, logger *slog.Logger) (*ExecutionResult, error) {
	emitter := deps.EventEmitter
	if emitter == nil {
		emitter = &noopEmitter{}
	}
	var metrics model.MetricsCollector = &noopMetrics{}
	orch := New(executors, emitter, metrics, logger)
	return orch.Execute(ctx, g, inputs, deps)
}

type noopEmitter struct{}

func (noopEmitter) Emit(model.ExecutionEvent) {}

type noopMetrics struct{}

func (noopMetrics) ExecutionStarted(string, string)                               {}
func (noopMetrics) ExecutionCompleted(string, string, int64, string)              {}
func (noopMetrics) NodeStarted(string, string, string)                            {}
func (noopMetrics) NodeCompleted(string, string, string, int64, string)           {}
func (noopMetrics) ProviderCallCompleted(string, string, int64, int, int, string) {}
func (noopMetrics) MCPCallCompleted(string, string, int64, string)                {}
func (noopMetrics) HTTPRequestCompleted(string, string, int, int64)               {}

// EvalCondition evaluates a back-edge or branch condition.
// The optional state parameter populates the state.* namespace in the expression context.
func EvalCondition(conditionExpr string, inputs map[string]any, state ...map[string]any) (bool, error) {
	ctx := &expression.Context{
		Input: inputs,
	}
	if len(state) > 0 && state[0] != nil {
		ctx.State = state[0]
	}
	result, err := expression.Eval(conditionExpr, ctx)
	if err != nil {
		return false, fmt.Errorf("condition evaluation error: %w", err)
	}
	return isTruthy(result), nil
}

func isTruthy(v any) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val != ""
	case int, int64, float64:
		return val != 0
	case []any:
		return len(val) > 0
	case map[string]any:
		return len(val) > 0
	default:
		return true
	}
}

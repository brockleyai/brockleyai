package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/brockleyai/brockleyai/engine/executor"
	graphpkg "github.com/brockleyai/brockleyai/engine/graph"
	"github.com/brockleyai/brockleyai/internal/model"
)

// executeLoops checks for back-edges whose conditions are true and re-executes
// the loop body nodes until all conditions are false or max_iterations is reached.
// It modifies nodeOutputs, steps, and iterationCounts in place.
func (o *Orchestrator) executeLoops(
	ctx context.Context,
	g *model.Graph,
	deps *executor.ExecutorDeps,
	nodeMap map[string]*model.Node,
	nodeOutputs map[string]map[string]any,
	inEdges map[string][]model.Edge,
	outEdges map[string][]model.Edge,
	state map[string]any,
	steps *[]StepRecord,
	iterationCounts map[string]int,
	skipped map[string]bool,
	mu *sync.Mutex,
	execID any,
	logger *slog.Logger,
) error {
	// Collect back-edges.
	var backEdges []model.Edge
	for _, edge := range g.Edges {
		if edge.BackEdge {
			backEdges = append(backEdges, edge)
		}
	}
	if len(backEdges) == 0 {
		return nil
	}

	// For each back-edge, identify the loop body: nodes between target and source
	// in topological order.
	type loopInfo struct {
		edge    model.Edge
		bodyIDs []string
		maxIter int
	}

	// Get topological order for determining loop bodies.
	topoOrder, err := graphpkg.TopologicalSort(g)
	if err != nil {
		return fmt.Errorf("loop detection: %w", err)
	}
	topoIndex := make(map[string]int)
	for i, id := range topoOrder {
		topoIndex[id] = i
	}

	var loops []loopInfo
	for _, be := range backEdges {
		targetIdx, targetOK := topoIndex[be.TargetNodeID]
		sourceIdx, sourceOK := topoIndex[be.SourceNodeID]
		if !targetOK || !sourceOK {
			continue
		}

		// Loop body: all nodes from target through source in topological order.
		var body []string
		for _, id := range topoOrder {
			idx := topoIndex[id]
			if idx >= targetIdx && idx <= sourceIdx {
				body = append(body, id)
			}
		}

		maxIter := 10 // default safety
		if be.MaxIterations != nil && *be.MaxIterations > 0 {
			maxIter = *be.MaxIterations
		}

		loops = append(loops, loopInfo{
			edge:    be,
			bodyIDs: body,
			maxIter: maxIter,
		})
	}

	// Identify downstream nodes for each loop: nodes reachable from the loop
	// body via forward edges that are NOT part of the loop body itself.
	// These need re-execution after the loop completes to pick up updated values.
	findDownstream := func(bodyIDs []string) []string {
		bodySet := make(map[string]bool)
		for _, id := range bodyIDs {
			bodySet[id] = true
		}
		// BFS from loop body nodes through forward edges.
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
		// Return in topological order.
		var result []string
		for _, id := range topoOrder {
			if visited[id] {
				result = append(result, id)
			}
		}
		return result
	}

	// Iterate loops.
	for _, loop := range loops {
		loopRan := false
		for iteration := 1; iteration <= loop.maxIter; iteration++ {
			// Evaluate the back-edge condition using the source node's outputs.
			mu.Lock()
			sourceOutputs := nodeOutputs[loop.edge.SourceNodeID]
			mu.Unlock()

			mu.Lock()
			stateCopy := copyMap(state)
			mu.Unlock()
			condResult, err := EvalCondition(loop.edge.Condition, sourceOutputs, stateCopy)
			if err != nil {
				return fmt.Errorf("loop condition evaluation failed for edge %s: %w", loop.edge.ID, err)
			}
			if !condResult {
				break
			}

			mu.Lock()
			iterationCounts[loop.edge.ID] = iteration

			// Save the back-edge value BEFORE clearing loop body outputs.
			var backEdgeValue any
			var hasBackEdgeValue bool
			if srcOut := nodeOutputs[loop.edge.SourceNodeID]; srcOut != nil {
				if val, ok := srcOut[loop.edge.SourcePort]; ok {
					backEdgeValue = val
					hasBackEdgeValue = true
				}
			}

			// Clear node outputs for loop body nodes so they re-execute with fresh inputs.
			for _, nodeID := range loop.bodyIDs {
				delete(nodeOutputs, nodeID)
				delete(skipped, nodeID)
			}
			mu.Unlock()

			// Re-execute loop body nodes in topological order (sequentially).
			for _, nodeID := range loop.bodyIDs {
				node := nodeMap[nodeID]
				if node == nil {
					continue
				}

				// Skip input nodes -- they keep their original outputs.
				if node.Type == model.NodeTypeInput {
					continue
				}

				nodeStart := time.Now()
				nodeLogger := logger.With("node_id", nodeID, "node_type", node.Type, "iteration", iteration)
				nodeLogger.Info("loop iteration node started")

				// Resolve inputs from forward edges normally.
				resolvedInputs := o.resolveInputs(node, nodeOutputs, inEdges[nodeID], state, mu)

				// Apply back-edge value if this is the target node.
				if hasBackEdgeValue && loop.edge.TargetNodeID == nodeID {
					resolvedInputs[loop.edge.TargetPort] = backEdgeValue
				}

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
						"iteration":    iteration,
					},
				}
				mu.Unlock()

				exec, execErr := o.executors.Get(node.Type)
				if execErr != nil {
					return fmt.Errorf("loop execution: no executor for node %s: %w", nodeID, execErr)
				}

				result, execErr := exec.Execute(ctx, node, resolvedInputs, nctx, deps)
				durationMs := time.Since(nodeStart).Milliseconds()

				if execErr != nil {
					nodeLogger.Error("loop iteration node failed", "error", execErr, "duration_ms", durationMs)
					mu.Lock()
					*steps = append(*steps, StepRecord{
						NodeID: nodeID, NodeType: node.Type, Status: "failed",
						Iteration: iteration, Inputs: resolvedInputs,
						Error: execErr.Error(), DurationMs: durationMs,
					})
					mu.Unlock()
					return fmt.Errorf("loop execution: node %s failed on iteration %d: %w", nodeID, iteration, execErr)
				}

				mu.Lock()
				if result != nil {
					nodeOutputs[nodeID] = result.Outputs
				}
				o.applyStateWrites(node, result, state)
				*steps = append(*steps, StepRecord{
					NodeID: nodeID, NodeType: node.Type, Status: "completed",
					Iteration: iteration, Inputs: resolvedInputs,
					Outputs: result.Outputs, DurationMs: durationMs,
				})
				mu.Unlock()

				// Emit events.
				outputJSON, _ := json.Marshal(result.Outputs)
				inputJSON, _ := json.Marshal(resolvedInputs)
				o.emitter.Emit(model.ExecutionEvent{
					Type: model.EventNodeCompleted, ExecutionID: fmt.Sprint(execID),
					NodeID: nodeID, NodeType: node.Type, Timestamp: time.Now(),
					Input: inputJSON, Output: outputJSON, DurationMs: durationMs,
				})
			}
			loopRan = true
		}

		// After the loop completes, re-execute downstream nodes so they pick
		// up updated values from the loop body.
		if loopRan {
			downstream := findDownstream(loop.bodyIDs)
			for _, nodeID := range downstream {
				node := nodeMap[nodeID]
				if node == nil {
					continue
				}

				nodeStart := time.Now()

				resolvedInputs := o.resolveInputs(node, nodeOutputs, inEdges[nodeID], state, mu)

				// Build NodeContext for downstream node.
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

				exec, execErr := o.executors.Get(node.Type)
				if execErr != nil {
					return fmt.Errorf("loop downstream: no executor for node %s: %w", nodeID, execErr)
				}

				result, execErr := exec.Execute(ctx, node, resolvedInputs, nctx, deps)
				durationMs := time.Since(nodeStart).Milliseconds()

				if execErr != nil {
					mu.Lock()
					*steps = append(*steps, StepRecord{
						NodeID: nodeID, NodeType: node.Type, Status: "failed",
						Inputs: resolvedInputs, Error: execErr.Error(), DurationMs: durationMs,
					})
					mu.Unlock()
					return fmt.Errorf("loop downstream: node %s failed: %w", nodeID, execErr)
				}

				mu.Lock()
				if result != nil {
					nodeOutputs[nodeID] = result.Outputs
				}
				o.applyStateWrites(node, result, state)
				*steps = append(*steps, StepRecord{
					NodeID: nodeID, NodeType: node.Type, Status: "completed",
					Inputs: resolvedInputs, Outputs: result.Outputs, DurationMs: durationMs,
				})
				mu.Unlock()
			}
		}
	}

	return nil
}

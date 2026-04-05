package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/brockleyai/brockleyai/engine/executor"
	"github.com/brockleyai/brockleyai/internal/model"
)

const (
	defaultMaxToolCalls      = 25
	defaultMaxLoopIterations = 10
)

// buildEdgeMaps builds node map and edge lookup maps from a graph.
func buildEdgeMaps(graph *model.Graph) (map[string]*model.Node, map[string][]model.Edge, map[string][]model.Edge) {
	nodeMap := make(map[string]*model.Node)
	for i := range graph.Nodes {
		nodeMap[graph.Nodes[i].ID] = &graph.Nodes[i]
	}

	outEdges := make(map[string][]model.Edge)
	inEdges := make(map[string][]model.Edge)
	for _, edge := range graph.Edges {
		outEdges[edge.SourceNodeID] = append(outEdges[edge.SourceNodeID], edge)
		inEdges[edge.TargetNodeID] = append(inEdges[edge.TargetNodeID], edge)
	}

	return nodeMap, outEdges, inEdges
}

// initState initializes the execution state from the graph's state schema.
func initState(g *model.Graph) map[string]any {
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

// resolveInputs resolves input ports for a node. Priority: edges > state reads > defaults.
func resolveInputs(node *model.Node, nodeOutputs map[string]map[string]any, incoming []model.Edge, state map[string]any) map[string]any {
	inputs := make(map[string]any)

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
			continue
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

// applyStateWrites applies state writes from node outputs to the execution state.
func applyStateWrites(node *model.Node, outputs map[string]any, state map[string]any) {
	if outputs == nil {
		return
	}
	for _, sw := range node.StateWrites {
		val, ok := outputs[sw.Port]
		if !ok {
			continue
		}
		existing := state[sw.StateField]
		switch arr := existing.(type) {
		case []any:
			state[sw.StateField] = append(arr, val)
		default:
			state[sw.StateField] = val
		}
	}
}

// propagateSkips marks downstream nodes as skipped when conditional branches produce nil.
func propagateSkips(condNode *model.Node, outputs map[string]any, outgoing []model.Edge, nodeMap map[string]*model.Node, allOutEdges, allInEdges map[string][]model.Edge, deadEdges, skipped map[string]bool) {
	for _, edge := range outgoing {
		if outputs[edge.SourcePort] == nil {
			deadEdges[edge.ID] = true
			markSkipped(edge.TargetNodeID, allOutEdges, allInEdges, deadEdges, skipped)
		}
	}
}

// markSkipped recursively marks a node and its descendants as skipped.
func markSkipped(nodeID string, outEdges, inEdges map[string][]model.Edge, deadEdges, skipped map[string]bool) {
	if skipped[nodeID] {
		return
	}
	for _, inEdge := range inEdges[nodeID] {
		if inEdge.BackEdge {
			continue
		}
		if !deadEdges[inEdge.ID] && !skipped[inEdge.SourceNodeID] {
			return
		}
	}
	skipped[nodeID] = true
	for _, edge := range outEdges[nodeID] {
		deadEdges[edge.ID] = true
		markSkipped(edge.TargetNodeID, outEdges, inEdges, deadEdges, skipped)
	}
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

// resolveToolRoutingForDispatch resolves tool routing for LLM dispatch.
func resolveToolRoutingForDispatch(ctx context.Context, cfg *model.LLMNodeConfig, nctx *executor.NodeContext, deps *executor.ExecutorDeps) (map[string]model.ToolRoute, error) {
	if len(cfg.ToolRouting) > 0 {
		return cfg.ToolRouting, nil
	}

	if cfg.ToolRoutingFromState != "" && nctx != nil {
		val, ok := nctx.State[cfg.ToolRoutingFromState]
		if !ok {
			return nil, fmt.Errorf("state field %q not found for tool_routing_from_state", cfg.ToolRoutingFromState)
		}
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

	return nil, fmt.Errorf("no tool routing configured")
}

// autoDiscoverToolsForDispatch queries MCP servers for tool definitions.
func autoDiscoverToolsForDispatch(ctx context.Context, routing map[string]model.ToolRoute, cache *executor.MCPClientCache) ([]model.LLMToolDefinition, error) {
	seen := make(map[string]bool)
	var tools []model.LLMToolDefinition

	for _, route := range routing {
		if seen[route.MCPURL] {
			continue
		}
		seen[route.MCPURL] = true

		defs, err := cache.ListToolsCached(ctx, route.MCPURL, nil)
		if err != nil {
			return nil, fmt.Errorf("listing tools from %s: %w", route.MCPURL, err)
		}

		for _, def := range defs {
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

// loadMessagesFromStateForDispatch loads conversation messages from a graph state field.
func loadMessagesFromStateForDispatch(state map[string]any, fieldName string) ([]model.Message, error) {
	val, ok := state[fieldName]
	if !ok {
		return nil, nil
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

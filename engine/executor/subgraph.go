package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/brockleyai/brockleyai/internal/model"
)

// SubgraphExecutor handles nodes of type "subgraph".
// It executes an inner graph, mapping outer ports to inner ports via PortMapping.
type SubgraphExecutor struct{}

var _ NodeExecutor = (*SubgraphExecutor)(nil)

func (e *SubgraphExecutor) Execute(ctx context.Context, node *model.Node, inputs map[string]any, _ *NodeContext, deps *ExecutorDeps) (*NodeResult, error) {
	var cfg model.SubgraphNodeConfig
	if err := json.Unmarshal(node.Config, &cfg); err != nil {
		return nil, fmt.Errorf("subgraph executor: invalid config: %w", err)
	}

	// Map outer input ports to inner graph input ports.
	innerInputs := make(map[string]any)
	for outerPort, innerMapping := range cfg.PortMapping.Inputs {
		val, ok := inputs[outerPort]
		if !ok {
			continue
		}
		// innerMapping is the inner port name (the input node of the inner graph
		// receives these as its outputs).
		// Format can be "portName" or "nodeID.portName" — for the input node,
		// we just use the port name directly since the orchestrator sets all
		// inputs on the input node.
		portName := innerMapping
		if parts := strings.SplitN(innerMapping, ".", 2); len(parts) == 2 {
			portName = parts[1]
		}
		innerInputs[portName] = val
	}

	// Execute the inner graph.
	innerOutputs, err := executeInnerGraph(ctx, cfg.Graph, innerInputs, deps)
	if err != nil {
		return nil, fmt.Errorf("subgraph executor: inner graph execution failed: %w", err)
	}

	// Map inner graph output ports back to outer output ports.
	outputs := make(map[string]any)
	for innerMapping, outerPort := range cfg.PortMapping.Outputs {
		// innerMapping can be "portName" or "nodeID.portName"
		portName := innerMapping
		if parts := strings.SplitN(innerMapping, ".", 2); len(parts) == 2 {
			portName = parts[1]
		}
		if val, ok := innerOutputs[portName]; ok {
			outputs[outerPort] = val
		}
	}

	return &NodeResult{Outputs: outputs}, nil
}

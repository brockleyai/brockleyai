package executor

import (
	"context"

	"github.com/brockleyai/brockleyai/internal/model"
)

// InputExecutor handles nodes of type "input".
// It passes input port values through to output ports unchanged.
type InputExecutor struct{}

var _ NodeExecutor = (*InputExecutor)(nil)

func (e *InputExecutor) Execute(_ context.Context, _ *model.Node, inputs map[string]any, _ *NodeContext, _ *ExecutorDeps) (*NodeResult, error) {
	outputs := make(map[string]any, len(inputs))
	for k, v := range inputs {
		outputs[k] = v
	}
	return &NodeResult{Outputs: outputs}, nil
}

// OutputExecutor handles nodes of type "output".
// It passes input port values through to outputs (terminal node).
type OutputExecutor struct{}

var _ NodeExecutor = (*OutputExecutor)(nil)

func (e *OutputExecutor) Execute(_ context.Context, _ *model.Node, inputs map[string]any, _ *NodeContext, _ *ExecutorDeps) (*NodeResult, error) {
	outputs := make(map[string]any, len(inputs))
	for k, v := range inputs {
		outputs[k] = v
	}
	return &NodeResult{Outputs: outputs}, nil
}

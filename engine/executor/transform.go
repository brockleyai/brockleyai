package executor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/brockleyai/brockleyai/engine/expression"
	"github.com/brockleyai/brockleyai/internal/model"
)

// TransformExecutor handles nodes of type "transform".
// It evaluates each configured expression against the node's inputs,
// producing one output port per expression.
type TransformExecutor struct{}

var _ NodeExecutor = (*TransformExecutor)(nil)

func (e *TransformExecutor) Execute(ctx context.Context, node *model.Node, inputs map[string]any, nctx *NodeContext, deps *ExecutorDeps) (*NodeResult, error) {
	var cfg model.TransformNodeConfig
	if err := json.Unmarshal(node.Config, &cfg); err != nil {
		return nil, fmt.Errorf("transform executor: invalid config: %w", err)
	}

	exprCtx := &expression.Context{
		Input: inputs,
	}
	if nctx != nil {
		exprCtx.State = nctx.State
		exprCtx.Meta = nctx.Meta
	}

	outputs := make(map[string]any, len(cfg.Expressions))
	for name, expr := range cfg.Expressions {
		result, err := expression.Eval(expr, exprCtx)
		if err != nil {
			return nil, fmt.Errorf("transform executor: evaluating expression %q for output %q: %w", expr, name, err)
		}
		outputs[name] = result
	}

	return &NodeResult{Outputs: outputs}, nil
}

package executor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/brockleyai/brockleyai/engine/expression"
	"github.com/brockleyai/brockleyai/internal/model"
)

// ConditionalExecutor handles nodes of type "conditional".
// It evaluates branch conditions in order, firing the first matching branch's
// output port with the input value. If no branch matches, the default port fires.
type ConditionalExecutor struct{}

var _ NodeExecutor = (*ConditionalExecutor)(nil)

func (e *ConditionalExecutor) Execute(ctx context.Context, node *model.Node, inputs map[string]any, nctx *NodeContext, deps *ExecutorDeps) (*NodeResult, error) {
	var cfg model.ConditionalNodeConfig
	if err := json.Unmarshal(node.Config, &cfg); err != nil {
		return nil, fmt.Errorf("conditional executor: invalid config: %w", err)
	}

	inputValue := inputs["value"]

	exprCtx := &expression.Context{
		Input: inputs,
	}
	if nctx != nil {
		exprCtx.State = nctx.State
		exprCtx.Meta = nctx.Meta
	}

	outputs := make(map[string]any)

	// Evaluate each branch condition in order.
	matched := false
	for _, branch := range cfg.Branches {
		result, err := expression.Eval(branch.Condition, exprCtx)
		if err != nil {
			return nil, fmt.Errorf("conditional executor: evaluating condition %q: %w", branch.Condition, err)
		}
		if isTruthy(result) {
			outputs[branch.Label] = inputValue
			matched = true
			break
		}
	}

	// If no branch matched, fire the default port.
	if !matched {
		if cfg.DefaultLabel != "" {
			outputs[cfg.DefaultLabel] = inputValue
		}
	}

	return &NodeResult{Outputs: outputs}, nil
}

// isTruthy checks whether a value is truthy in the same sense as the expression engine.
func isTruthy(val any) bool {
	if val == nil {
		return false
	}
	switch v := val.(type) {
	case bool:
		return v
	case string:
		return v != ""
	case int64:
		return v != 0
	case float64:
		return v != 0
	case []any:
		return len(v) > 0
	case map[string]any:
		return true
	}
	return true
}

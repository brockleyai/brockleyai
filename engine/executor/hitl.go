package executor

import (
	"context"
	"fmt"

	"github.com/brockleyai/brockleyai/internal/model"
)

// HITLExecutor handles nodes of type "human_in_the_loop".
// This is a placeholder — full HITL support requires an external input mechanism
// and will be implemented in Phase 0.5+.
type HITLExecutor struct{}

var _ NodeExecutor = (*HITLExecutor)(nil)

func (e *HITLExecutor) Execute(_ context.Context, _ *model.Node, _ map[string]any, _ *NodeContext, _ *ExecutorDeps) (*NodeResult, error) {
	return nil, fmt.Errorf("human-in-the-loop execution not yet implemented — requires external input mechanism")
}

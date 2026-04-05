package orchestrator

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/brockleyai/brockleyai/engine/executor"
	"github.com/brockleyai/brockleyai/internal/model"
)

func init() {
	executor.SetInnerGraphRunner(runInnerGraph)
}

// runInnerGraph executes a graph and returns the output node results.
// This is used by ForEach and Subgraph executors via the SetInnerGraphRunner callback.
// Inner graphs use a noop event emitter to avoid leaking execution-level events
// (e.g., EventExecutionCompleted) to the outer execution's Redis channel.
func runInnerGraph(ctx context.Context, g *model.Graph, inputs map[string]any, deps *executor.ExecutorDeps, executors *executor.Registry, logger *slog.Logger) (map[string]any, error) {
	// Create a copy of deps with a noop emitter so inner graph events
	// don't interfere with the outer execution's event stream.
	innerDeps := *deps
	innerDeps.EventEmitter = &noopEmitter{}

	result, err := Execute(ctx, g, inputs, &innerDeps, executors, logger)
	if err != nil {
		return nil, err
	}
	if result.Status != "completed" {
		return nil, fmt.Errorf("inner graph execution failed with status: %s", result.Status)
	}
	return result.Outputs, nil
}

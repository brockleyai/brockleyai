package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/brockleyai/brockleyai/internal/model"
)

// ForEachExecutor handles nodes of type "foreach".
// It iterates over an input array, executing an inner subgraph for each item.
// Supports concurrency control and configurable error handling.
type ForEachExecutor struct{}

var _ NodeExecutor = (*ForEachExecutor)(nil)

func (e *ForEachExecutor) Execute(ctx context.Context, node *model.Node, inputs map[string]any, _ *NodeContext, deps *ExecutorDeps) (*NodeResult, error) {
	var cfg model.ForEachNodeConfig
	if err := json.Unmarshal(node.Config, &cfg); err != nil {
		return nil, fmt.Errorf("foreach executor: invalid config: %w", err)
	}

	// Get items input port (must be []any).
	rawItems, ok := inputs["items"]
	if !ok {
		return nil, fmt.Errorf("foreach executor: missing required input port 'items'")
	}
	items, ok := rawItems.([]any)
	if !ok {
		return nil, fmt.Errorf("foreach executor: input port 'items' must be an array, got %T", rawItems)
	}

	// Get optional context input port.
	contextValue := inputs["context"]

	// Prepare ordered result and error slices.
	results := make([]any, len(items))
	errors := make([]any, 0)
	succeeded := make([]bool, len(items))

	var mu sync.Mutex
	var firstAbortErr error

	// Determine concurrency.
	concurrency := cfg.Concurrency
	if concurrency < 0 {
		concurrency = 0
	}

	onItemError := cfg.OnItemError
	if onItemError == "" {
		onItemError = "continue"
	}

	// Execute items.
	var wg sync.WaitGroup
	var sem chan struct{}
	if concurrency > 0 {
		sem = make(chan struct{}, concurrency)
	}

	for i, item := range items {
		// Check for abort.
		if onItemError == "abort" {
			mu.Lock()
			aborted := firstAbortErr != nil
			mu.Unlock()
			if aborted {
				break
			}
		}

		wg.Add(1)
		go func(idx int, currentItem any) {
			defer wg.Done()

			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}

			// Check for abort again inside goroutine.
			if onItemError == "abort" {
				mu.Lock()
				aborted := firstAbortErr != nil
				mu.Unlock()
				if aborted {
					return
				}
			}

			// Build inner graph inputs.
			innerInputs := map[string]any{
				"item":    currentItem,
				"index":   idx,
				"context": contextValue,
			}

			output, err := executeInnerGraph(ctx, cfg.Graph, innerInputs, deps)
			if err != nil {
				mu.Lock()
				errObj := map[string]any{
					"index": idx,
					"error": err.Error(),
					"item":  currentItem,
				}
				errors = append(errors, errObj)
				if onItemError == "abort" && firstAbortErr == nil {
					firstAbortErr = err
				}
				mu.Unlock()
				return
			}

			mu.Lock()
			results[idx] = output
			succeeded[idx] = true
			mu.Unlock()
		}(i, item)
	}

	wg.Wait()

	if onItemError == "abort" && firstAbortErr != nil {
		return nil, fmt.Errorf("foreach executor: item failed: %w", firstAbortErr)
	}

	// Collect only successful results in order.
	filteredResults := make([]any, 0, len(items))
	for i := range items {
		if succeeded[i] {
			filteredResults = append(filteredResults, results[i])
		}
	}

	return &NodeResult{
		Outputs: map[string]any{
			"results": filteredResults,
			"errors":  errors,
		},
	}, nil
}

// executeInnerGraph unmarshals a graph from JSON, creates an Orchestrator, and runs it.
// It returns the inner graph's output node results.
func executeInnerGraph(ctx context.Context, graphJSON json.RawMessage, inputs map[string]any, deps *ExecutorDeps) (map[string]any, error) {
	var g model.Graph
	if err := json.Unmarshal(graphJSON, &g); err != nil {
		return nil, fmt.Errorf("executeInnerGraph: invalid graph JSON: %w", err)
	}

	// Create a fresh executor registry with default executors.
	reg := NewDefaultRegistry()

	// Use a lightweight logger for inner graphs.
	logger := slog.Default()
	if deps != nil && deps.Logger != nil {
		logger = deps.Logger
	}

	// We import the orchestrator package indirectly through a callback to avoid
	// a circular dependency. Instead, we use the package-level function via
	// the orchestrator convenience function. Since executor depends on the
	// orchestrator package would cause a cycle, we use the InnerGraphRunner.
	if innerRunner == nil {
		return nil, fmt.Errorf("executeInnerGraph: inner graph runner not configured (call SetInnerGraphRunner)")
	}

	return innerRunner(ctx, &g, inputs, deps, reg, logger)
}

// InnerGraphRunnerFunc is the type for the function that runs an inner graph.
// This indirection avoids a circular dependency between executor and orchestrator.
type InnerGraphRunnerFunc func(ctx context.Context, g *model.Graph, inputs map[string]any, deps *ExecutorDeps, executors *Registry, logger *slog.Logger) (map[string]any, error)

var innerRunner InnerGraphRunnerFunc

// SetInnerGraphRunner sets the function used to execute inner graphs (foreach, subgraph).
// This must be called during initialization to wire the orchestrator into the executor package.
func SetInnerGraphRunner(fn InnerGraphRunnerFunc) {
	innerRunner = fn
}

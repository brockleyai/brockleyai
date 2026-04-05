// Package executor provides node executors for the Brockley graph engine.
// Each built-in node type has a corresponding executor that processes inputs
// and produces outputs according to the node's configuration.
package executor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/brockleyai/brockleyai/internal/model"
)

// NodeResult is what an executor returns after processing a node.
type NodeResult struct {
	Outputs map[string]any // output port name -> value
}

// NodeContext provides state and metadata to node executors.
// State is a read-only snapshot of the graph's current state fields.
// Meta contains execution metadata (node_id, node_name, node_type, execution_id, etc.).
type NodeContext struct {
	State map[string]any // read-only snapshot of graph state
	Meta  map[string]any // execution metadata
}

// NodeExecutor executes a single node given its resolved inputs.
type NodeExecutor interface {
	Execute(ctx context.Context, node *model.Node, inputs map[string]any, nctx *NodeContext, deps *ExecutorDeps) (*NodeResult, error)
}

// ProviderRegistry looks up LLM providers by name.
type ProviderRegistry interface {
	Get(name string) (model.LLMProvider, error)
}

// ExecutorDeps holds dependencies injected into executors.
type ExecutorDeps struct {
	ProviderRegistry  ProviderRegistry
	SecretStore       model.SecretStore
	MCPClient         model.MCPClient    // can be nil if no MCP configured (for standalone tool nodes)
	MCPClientCache    *MCPClientCache    // for tool loop MCP dispatch (scoped to execution)
	APIToolDispatcher *APIToolDispatcher // for API endpoint dispatch in tool loops
	EventEmitter      model.EventEmitter
	Logger            *slog.Logger
}

// Registry maps node types to executors.
type Registry struct {
	mu        sync.RWMutex
	executors map[string]NodeExecutor
}

// NewRegistry creates an empty executor registry.
func NewRegistry() *Registry {
	return &Registry{
		executors: make(map[string]NodeExecutor),
	}
}

// Register adds an executor for the given node type.
func (r *Registry) Register(nodeType string, exec NodeExecutor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executors[nodeType] = exec
}

// Get returns the executor for the given node type.
func (r *Registry) Get(nodeType string) (NodeExecutor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	exec, ok := r.executors[nodeType]
	if !ok {
		return nil, fmt.Errorf("no executor registered for node type %q", nodeType)
	}
	return exec, nil
}

// NewDefaultRegistry creates a registry pre-loaded with all built-in node executors.
func NewDefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(model.NodeTypeInput, &InputExecutor{})
	r.Register(model.NodeTypeOutput, &OutputExecutor{})
	r.Register(model.NodeTypeLLM, &LLMExecutor{})
	r.Register(model.NodeTypeConditional, &ConditionalExecutor{})
	r.Register(model.NodeTypeTransform, &TransformExecutor{})
	r.Register(model.NodeTypeTool, &ToolExecutor{})
	r.Register(model.NodeTypeHumanInTheLoop, &HITLExecutor{})
	r.Register(model.NodeTypeForEach, &ForEachExecutor{})
	r.Register(model.NodeTypeSubgraph, &SubgraphExecutor{})
	r.Register(model.NodeTypeSuperagent, &SuperagentExecutor{})
	r.Register(model.NodeTypeAPITool, &APIToolNodeExecutor{})
	return r
}

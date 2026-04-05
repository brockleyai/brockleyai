# Custom Node Types

Brockley's node execution system is extensible. You can define new node types and register them with the executor registry alongside the built-in types.

## Architecture

Node execution is handled by two registries:

1. **Executor Registry** (`executor.Registry`): Maps node type strings to `NodeExecutor` implementations. When the orchestrator encounters a node, it looks up the executor by the node's `type` field.

2. **Provider Registry** (`provider.Registry`): Maps provider names to `LLMProvider` implementations. Used by the LLM executor to call different LLM APIs.

## NodeExecutor Interface

Every node type (built-in or custom) implements the `NodeExecutor` interface:

```go
type NodeExecutor interface {
    Execute(
        ctx context.Context,
        node *model.Node,
        inputs map[string]any,
        nctx *NodeContext,
        deps *ExecutorDeps,
    ) (*NodeResult, error)
}
```

### Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `ctx` | `context.Context` | Carries cancellation, deadlines, and correlation IDs. |
| `node` | `*model.Node` | The full node definition including `Config` (as `json.RawMessage`), ports, and metadata. |
| `inputs` | `map[string]any` | Resolved input values keyed by input port name. |
| `nctx` | `*NodeContext` | Node execution context: `State` (graph state snapshot) and `Meta` (execution metadata). |
| `deps` | `*ExecutorDeps` | Shared dependencies: provider registry, secret store, MCP client, event emitter, logger. |

### Return Value

```go
type NodeResult struct {
    Outputs map[string]any // output port name -> value
}
```

Return a `NodeResult` with one entry per output port, or return an error to fail the node.

### ExecutorDeps

Dependencies injected into every executor:

```go
type ExecutorDeps struct {
    ProviderRegistry ProviderRegistry      // LLM provider lookup
    SecretStore      model.SecretStore     // Secret resolution
    MCPClient        model.MCPClient       // MCP tool server client (may be nil)
    EventEmitter     model.EventEmitter    // Execution event publishing
    Logger           *slog.Logger          // Structured logger
}
```

## Registering a Custom Node Type

### Step 1: Define the Config Struct

Create a struct for your node's configuration. It will be deserialized from the node's `Config` field (`json.RawMessage`).

```go
type SentimentConfig struct {
    Endpoint  string `json:"endpoint"`
    Threshold float64 `json:"threshold,omitempty"`
}
```

### Step 2: Implement NodeExecutor

```go
package executor

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/brockleyai/brockleyai/internal/model"
)

type SentimentExecutor struct{}

func (e *SentimentExecutor) Execute(
    ctx context.Context,
    node *model.Node,
    inputs map[string]any,
    nctx *NodeContext,
    deps *ExecutorDeps,
) (*NodeResult, error) {
    // 1. Unmarshal config
    var cfg SentimentConfig
    if err := json.Unmarshal(node.Config, &cfg); err != nil {
        return nil, fmt.Errorf("sentiment executor: invalid config: %w", err)
    }

    // 2. Read inputs
    text, _ := inputs["text"].(string)

    // 3. Do work (respecting context cancellation)
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    score, err := callSentimentAPI(ctx, cfg.Endpoint, text)
    if err != nil {
        return nil, fmt.Errorf("sentiment executor: %w", err)
    }

    // 4. Log with correlation IDs
    deps.Logger.Info("sentiment analysis complete",
        "score", score,
        "threshold", cfg.Threshold,
    )

    // 5. Return outputs
    return &NodeResult{
        Outputs: map[string]any{
            "score":    score,
            "positive": score >= cfg.Threshold,
        },
    }, nil
}
```

### Step 3: Register the Executor

Register your executor with the registry at initialization time:

```go
registry := executor.NewDefaultRegistry()
registry.Register("sentiment", &SentimentExecutor{})
```

Or add it to `NewDefaultRegistry()` to include it in the default set:

```go
func NewDefaultRegistry() *Registry {
    r := NewRegistry()
    // Built-in types
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
    r.Register(model.NodeTypeAPITool, &APIToolExecutor{})
    // Custom
    r.Register("sentiment", &SentimentExecutor{})
    return r
}
```

### Step 4: Use in a Graph

```json
{
  "id": "analyze-sentiment",
  "name": "Analyze Sentiment",
  "type": "sentiment",
  "input_ports": [
    {"name": "text", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {"name": "score", "schema": {"type": "number"}},
    {"name": "positive", "schema": {"type": "boolean"}}
  ],
  "config": {
    "endpoint": "https://api.example.com/sentiment",
    "threshold": 0.6
  }
}
```

## Built-in Node Types

For reference, the built-in types registered by `NewDefaultRegistry()`:

| Type String | Executor | Description |
|-------------|----------|-------------|
| `input` | `InputExecutor` | [Graph entry point](input-output.md). Passes inputs through. |
| `output` | `OutputExecutor` | [Graph terminal](input-output.md). Passes inputs through. |
| `llm` | `LLMExecutor` | [Calls an LLM provider](llm.md). |
| `tool` | `ToolExecutor` | [Calls an MCP tool](tool.md). |
| `api_tool` | `APIToolExecutor` | [Calls a REST API endpoint](api-tool.md). |
| `conditional` | `ConditionalExecutor` | [Routes data based on conditions](conditional.md). |
| `transform` | `TransformExecutor` | [Evaluates expressions](transform.md). |
| `human_in_the_loop` | `HITLExecutor` | [Pauses for human input](human-in-the-loop.md) (not yet implemented). |
| `foreach` | `ForEachExecutor` | [Iterates over an array](foreach.md). |
| `subgraph` | `SubgraphExecutor` | [Executes an inner graph](subgraph.md). |
| `superagent` | `SuperagentExecutor` | [Autonomous agent loop](superagent.md). |

## Best Practices

- **Unmarshal config defensively.** Always handle invalid config JSON with a clear error message prefixed with your executor name.
- **Use the logger.** `deps.Logger` provides a structured logger with correlation IDs already set. Use it for all logging within your executor.
- **Respect context cancellation.** Check `ctx.Done()` in long-running operations to support timeouts and cancellation.
- **Keep outputs deterministic.** Given the same inputs and config, produce the same outputs. This makes graphs easier to test and debug.
- **Use the secret store for credentials.** Call `deps.SecretStore.GetSecret(ctx, ref)` to resolve secret references. Never hard-code secrets in config.
- **Return structured errors.** Wrap errors with context about which executor and what operation failed.
- **Write tests.** Follow the Interface -> Mock -> Test -> Implementation pattern. Test against `MockLLMProvider` and `MockMCPClient` where applicable.

## See Also

- [Data Model: NodeTypeDefinition](../specs/data-model.md) -- custom node type metadata
- [Architecture Overview](../specs/architecture.md) -- executor registry and node dispatch
- [LLM Node](llm.md) -- reference implementation for a complex executor
- [Transform Node](transform.md) -- reference implementation for a simple executor

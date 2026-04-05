# Brockley Engine

Standalone Go package for validating and executing AI agent graphs. The engine has zero infrastructure dependencies -- no database, no message queue, no network calls required. Bring your own LLM provider and run graphs in any Go program.

## Install

```bash
go get github.com/brockleyai/brockleyai/engine
```

## Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/brockleyai/brockleyai/engine/graph"
    "github.com/brockleyai/brockleyai/engine/executor"
    "github.com/brockleyai/brockleyai/engine/mock"
)

func main() {
    // Define a graph
    g := graph.New("example")

    start := g.AddNode("start", "input", nil)
    llm := g.AddNode("llm", "llm_call", map[string]interface{}{
        "provider": "openai",
        "model":    "gpt-4o",
        "prompt":   "Summarize: {{start.output}}",
    })
    end := g.AddNode("end", "output", nil)

    g.AddEdge(start, "output", llm, "input")
    g.AddEdge(llm, "output", end, "input")

    // Validate
    if err := g.Validate(); err != nil {
        log.Fatalf("invalid graph: %v", err)
    }

    // Execute with a mock provider (swap for a real provider in production)
    provider := mock.NewProvider()
    exec := executor.New(provider)

    result, err := exec.Run(context.Background(), g, map[string]interface{}{
        "input": "Hello, world!",
    })
    if err != nil {
        log.Fatalf("execution failed: %v", err)
    }

    fmt.Println(result)
}
```

## Key Packages

| Package | Description |
|---|---|
| `graph` | Graph definition, node types, edge connections, validation |
| `expression` | Expression language parser and evaluator for port references |
| `executor` | Single-graph execution engine with retry and timeout support |
| `orchestrator` | Multi-step execution coordination, foreach, conditionals |
| `provider` | LLM provider interface and registry |
| `mock` | Mock provider and utilities for testing |
| `mcp` | MCP tool discovery and invocation |

## Embedding the Engine

The engine is designed to be embedded. Common use cases:

- **CLI tools** -- validate and run graphs from the command line
- **CI pipelines** -- execute graphs as part of automated workflows
- **Custom servers** -- build your own API on top of the engine
- **Testing** -- validate graph definitions in unit tests

The engine makes no network calls on its own. All external communication (LLM calls, tool invocations) goes through the provider interface, which you control.

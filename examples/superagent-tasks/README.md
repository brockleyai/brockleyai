# E2E Superagent Tasks

## Representations

The graph is defined in two equivalent formats:

- **`graph.json`** -- JSON representation (used by the Brockley API and CLI)
- **`graph.tf`** -- Terraform representation (used by the Brockley Terraform provider)

Both files describe the same graph. Choose whichever fits your workflow.

## Graph Diagram

```
input-1 (input)
└── agent-1 (superagent)
    └── output-1 (output)
```

## Nodes

| Node | Type | Description |
|------|------|-------------|
| `input-1` | input | Accepts: topic |
| `agent-1` | superagent | Superagent (max 3 iterations) |
| `output-1` | output | Produces: result |

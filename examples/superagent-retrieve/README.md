# E2E Superagent Retrieve

## Representations

The graph is defined in two equivalent formats:

- **`graph.json`** -- JSON representation (used by the Brockley API and CLI)
- **`graph.tf`** -- Terraform representation (used by the Brockley Terraform provider)

Both files describe the same graph. Choose whichever fits your workflow.

## Graph Diagram

```
input-1 (input)
├── seed-secret (tool)
│   └── seed-ready (transform)
│       └── agent-1 (superagent)
│           ├── output-1 (output)
│           └── tool-check (transform)
│               └── (output-1) *
└── (agent-1) *
```

## Nodes

| Node | Type | Description |
|------|------|-------------|
| `input-1` | input | Accepts: secret_key, secret_value |
| `seed-secret` | tool | MCP tool: store_value |
| `agent-1` | superagent | Superagent (max 3 iterations) |
| `seed-ready` | transform | Computes: seed_status |
| `tool-check` | transform | Computes: used_tool, finish_reason, total_tool_calls |
| `output-1` | output | Produces: result, used_tool, finish_reason, total_tool_calls |

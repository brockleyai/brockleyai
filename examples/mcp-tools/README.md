# E2E MCP Tools

E2E test: tool nodes via MCP server, tool chaining, conditional routing on tool output, forEach with tool nodes, tool error handling.

## Representations

The graph is defined in two equivalent formats:

- **`graph.json`** -- JSON representation (used by the Brockley API and CLI)
- **`graph.tf`** -- Terraform representation (used by the Brockley Terraform provider)

Both files describe the same graph. Choose whichever fits your workflow.

## Graph Diagram

```
input-1 (input)
├── echo-tool (tool)
│   └── word-count-tool (tool)
│       └── router (conditional)
│           ├── [long]
│           │   └── handler-long (transform)
│           │       └── output-1 (output)
│           └── [short]
│               └── handler-short (transform)
│                   └── (output-1) *
└── foreach-1 (foreach)
    └── format-lookups (transform)
        └── (output-1) *
```

## Nodes

| Node | Type | Description |
|------|------|-------------|
| `input-1` | input | Accepts: text, items |
| `echo-tool` | tool | MCP tool: echo |
| `word-count-tool` | tool | MCP tool: word_count |
| `router` | conditional | Routes: long | short (default) |
| `handler-long` | transform | Computes: result |
| `handler-short` | transform | Computes: result |
| `foreach-1` | foreach | ForEach (concurrency=1) |
| `format-lookups` | transform | Computes: lookups |
| `output-1` | output | Produces: classification, lookups |

### Inner graph: foreach-1

```
inner-input (input)
├── inner-lookup (tool)
│   └── inner-combine (transform)
│       └── inner-output (output)
└── (inner-combine) *
```

| Node | Type | Description |
|------|------|-------------|
| `inner-input` | input | Accepts: item, index |
| `inner-lookup` | tool | MCP tool: lookup |
| `inner-combine` | transform | Computes: result |
| `inner-output` | output | Produces: result |

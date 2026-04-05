# E2E API Tool Standalone

E2E test: standalone api_tool node calling a REST endpoint via referenced API tool definition.

## Representations

The graph is defined in two equivalent formats:

- **`graph.json`** -- JSON representation (used by the Brockley API and CLI)
- **`graph.tf`** -- Terraform representation (used by the Brockley Terraform provider)

Both files describe the same graph. Choose whichever fits your workflow.

## Graph Diagram

```
input-1 (input)
└── get-customer (api_tool)
    └── output-1 (output)
```

## Nodes

| Node | Type | Description |
|------|------|-------------|
| `input-1` | input | Accepts: customer_id |
| `get-customer` | api_tool | API tool: get_customer |
| `output-1` | output | Produces: result |

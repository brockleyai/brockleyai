# E2E Comprehensive

E2E test: transforms, conditionals, parallel fork/join, skip propagation, exclusive fan-in, broad expression coverage.

## Representations

The graph is defined in two equivalent formats:

- **`graph.json`** -- JSON representation (used by the Brockley API and CLI)
- **`graph.tf`** -- Terraform representation (used by the Brockley Terraform provider)

Both files describe the same graph. Choose whichever fits your workflow.

## Graph Diagram

```
input-1 (input)
└── transform-a (transform)
    ├── transform-b (transform)
    │   └── joiner (transform)
    │       └── conditional-a (conditional)
    │           ├── [premium]
    │           │   └── conditional-b (conditional)
    │           │       ├── [high]
    │           │       │   └── handler-high (transform)
    │           │       │       └── output-1 (output)
    │           │       └── [low]
    │           │           └── handler-low (transform)
    │           │               └── (output-1) *
    │           └── [standard]
    │               └── handler-standard (transform)
    │                   └── (output-1) *
    ├── transform-c (transform)
    └── transform-d (transform)
```

## Nodes

| Node | Type | Description |
|------|------|-------------|
| `input-1` | input | Accepts: data |
| `transform-a` | transform | Computes 9 expressions |
| `transform-b` | transform | Computes 6 expressions |
| `transform-c` | transform | Computes 4 expressions |
| `transform-d` | transform | Computes 7 expressions |
| `joiner` | transform | Computes: value |
| `conditional-a` | conditional | Routes: premium | standard (default) |
| `conditional-b` | conditional | Routes: high | low (default) |
| `handler-high` | transform | Computes: result |
| `handler-low` | transform | Computes: result |
| `handler-standard` | transform | Computes: result |
| `output-1` | output | Produces: result |

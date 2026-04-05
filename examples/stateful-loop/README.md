# E2E Stateful Loop

E2E test: foreach, back-edge loops, state reducers (replace, append), post-loop processing.

## Representations

The graph is defined in two equivalent formats:

- **`graph.json`** -- JSON representation (used by the Brockley API and CLI)
- **`graph.tf`** -- Terraform representation (used by the Brockley Terraform provider)

Both files describe the same graph. Choose whichever fits your workflow.

## Graph Diagram

```
input-1 (input)
└── foreach-1 (foreach)
    └── prepare (transform)
        └── loop-body (transform)
            └── evaluator (conditional)
                └── [done]
                    └── post-process (transform)
                        └── output-1 (output)

Back-edge: evaluator --> loop-body (max 10 iterations)
```

## Nodes

| Node | Type | Description |
|------|------|-------------|
| `input-1` | input | Accepts: items |
| `foreach-1` | foreach | ForEach (concurrency=2) |
| `prepare` | transform | Computes: value |
| `loop-body` | transform | Computes: value, count, log_entry |
| `evaluator` | conditional | Routes: loop | done (default) |
| `post-process` | transform | Computes: final_value, char_count |
| `output-1` | output | Produces: final_value, char_count |

### Inner graph: foreach-1

```
inner-input (input)
└── inner-transform (transform)
    └── inner-output (output)
```

| Node | Type | Description |
|------|------|-------------|
| `inner-input` | input | Accepts: item, index |
| `inner-transform` | transform | Computes: result |
| `inner-output` | output | Produces: result |

## State Fields

| Field | Reducer | Initial |
|-------|---------|---------|
| `count` | replace | `0` |
| `log` | append | `[]` |

## Back-Edges (Loops)

- `evaluator` → `loop-body` (max 10 iterations)

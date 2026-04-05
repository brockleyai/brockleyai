# E2E Tool Loop Error Recovery

E2E test: behavior when a tool call fails and the LLM recovers.

## Representations

The graph is defined in two equivalent formats:

- **`graph.json`** -- JSON representation (used by the Brockley API and CLI)
- **`graph.tf`** -- Terraform representation (used by the Brockley Terraform provider)

Both files describe the same graph. Choose whichever fits your workflow.

## Graph Diagram

```
input-1 (input)
└── assistant (llm)
    └── output-1 (output)
```

## Nodes

| Node | Type | Description |
|------|------|-------------|
| `input-1` | input | Accepts: user_message |
| `assistant` | llm | LLM call (openai, tool-loop-error-recovery, tool-loop) |
| `output-1` | output | Produces: response, total_tool_calls, finish_reason |

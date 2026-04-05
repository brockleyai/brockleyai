# E2E Tool Loop Limits

E2E test: tool loop safety limit enforcement. LLM keeps requesting tool calls; should stop at limit.

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
| `assistant` | llm | LLM call (openai, tool-loop-limits, tool-loop) |
| `output-1` | output | Produces: response, finish_reason, total_tool_calls |

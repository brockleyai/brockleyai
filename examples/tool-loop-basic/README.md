# E2E Tool Loop Basic

E2E test: single LLM node with tool_loop calling echo and word_count tools.

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
| `assistant` | llm | LLM call (openai, tool-loop-basic, tool-loop) |
| `output-1` | output | Produces: response, finish_reason, total_tool_calls, iterations |

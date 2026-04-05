# E2E Tool Loop Two Nodes

E2E test: two separate LLM nodes with tool loops wired sequentially.

## Representations

The graph is defined in two equivalent formats:

- **`graph.json`** -- JSON representation (used by the Brockley API and CLI)
- **`graph.tf`** -- Terraform representation (used by the Brockley Terraform provider)

Both files describe the same graph. Choose whichever fits your workflow.

## Graph Diagram

```
input-1 (input)
└── llm-1 (llm)
    ├── llm-2 (llm)
    │   └── output-1 (output)
    └── (output-1) *
```

## Nodes

| Node | Type | Description |
|------|------|-------------|
| `input-1` | input | Accepts: user_message |
| `llm-1` | llm | LLM call (openai, tool-loop-two-nodes-1, tool-loop) |
| `llm-2` | llm | LLM call (openai, tool-loop-two-nodes-2, tool-loop) |
| `output-1` | output | Produces: first_response, second_response, first_tool_calls, second_tool_calls |

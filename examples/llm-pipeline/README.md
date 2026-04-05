# E2E LLM Pipeline

E2E test: LLM classification (JSON response), conditional routing on structured output, LLM text response, template rendering with #if/#each, env var expansion.

## Representations

The graph is defined in two equivalent formats:

- **`graph.json`** -- JSON representation (used by the Brockley API and CLI)
- **`graph.tf`** -- Terraform representation (used by the Brockley Terraform provider)

Both files describe the same graph. Choose whichever fits your workflow.

## Graph Diagram

```
input-1 (input)
├── classify (llm)
│   └── merge (transform)
│       └── router (conditional)
│           ├── [billing]
│           │   └── billing-responder (llm)
│           │       └── output-1 (output)
│           ├── [technical]
│           │   └── technical-responder (llm)
│           │       └── (output-1) *
│           └── [general]
│               └── general-responder (llm)
│                   └── (output-1) *
└── (merge) *
```

## Nodes

| Node | Type | Description |
|------|------|-------------|
| `input-1` | input | Accepts: ticket |
| `classify` | llm | LLM call (openrouter, openai/gpt-oss-20b) |
| `merge` | transform | Computes: merged |
| `router` | conditional | Routes: billing | technical | general (default) |
| `billing-responder` | llm | LLM call (openrouter, openai/gpt-oss-20b) |
| `technical-responder` | llm | LLM call (openrouter, openai/gpt-oss-20b) |
| `general-responder` | llm | LLM call (openrouter, openai/gpt-oss-20b) |
| `output-1` | output | Produces: reply |

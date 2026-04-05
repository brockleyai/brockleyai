# Frequently Asked Questions

## General

### What is Brockley?

Brockley is an open-source AI agent infrastructure platform. It lets you define agent workflows as typed, validated graphs and manage them through a visual editor, REST API, CLI, Terraform provider, or MCP server.

See [Introduction](../getting-started/introduction.md) for a full overview.

### Is Brockley really open source?

Yes. The execution engine, API server, web UI, CLI, Terraform provider, MCP server, and all built-in node types are fully open source under the Apache 2.0 license. This is everything you need to build, validate, deploy, and run agent workflows. The open-source core is fully functional and not artificially limited.

### What language is Brockley written in?

The server, engine, worker, and CLI are written in **Go**. The web UI is a **React** application. The Terraform provider is also Go (using the Terraform Plugin SDK).

### Does Brockley require a cloud account?

No. Brockley is fully self-hostable. You need:
- PostgreSQL (any version 14+)
- Redis (any version 7+)
- Docker or Kubernetes for deployment

No cloud-specific services are required.

## LLM Providers

### Which LLM providers are supported?

Built-in providers:

| Provider | Models | Auth |
|----------|--------|------|
| **OpenAI** | GPT-4o, GPT-4, GPT-3.5-turbo, etc. | API key |
| **Anthropic** | Claude Opus, Sonnet, Haiku | API key |
| **Google** | Gemini Pro, Gemini Flash | API key |
| **OpenRouter** | Any model on OpenRouter | API key |
| **AWS Bedrock** | Claude, Titan, etc. via Bedrock | AWS SigV4 |
| **Custom** | Any OpenAI-compatible API | Configurable |

### How do I add my LLM API key?

LLM API keys are referenced by name using `api_key_ref` in node config. The actual key is stored as an environment variable.

For example, in an LLM node config:

```json
{
  "provider": "openai",
  "model": "gpt-4o",
  "api_key_ref": "openai-key"
}
```

The engine resolves the reference to the environment variable `BROCKLEY_SECRET_OPENAI_KEY`. The rule is: uppercase the ref, replace hyphens with underscores, prepend `BROCKLEY_SECRET_`. Set it on the worker:

```bash
export BROCKLEY_SECRET_OPENAI_KEY="sk-your-key-here"
```

See [Providers Overview](../providers/overview.md) for full details on secret resolution.

### Can I use a custom or self-hosted LLM?

Yes. Use the `custom` provider type with a `base_url` pointing to any OpenAI-compatible API:

```json
{
  "provider": "custom",
  "model": "my-model",
  "api_key_ref": "MY_LLM_KEY",
  "base_url": "http://my-llm-server:8080/v1"
}
```

This works with vLLM, Ollama (with OpenAI compatibility), LiteLLM, and similar tools.

### Does Brockley support structured output (JSON mode)?

Yes. Set `response_format: "json"` and provide an `output_schema` on the [LLM node](../nodes/llm.md):

```json
{
  "response_format": "json",
  "output_schema": {
    "type": "object",
    "properties": {
      "intent": {"type": "string"},
      "confidence": {"type": "number"}
    }
  },
  "validate_output": true
}
```

When `validate_output` is `true`, the engine validates the LLM response against the schema and fails the node if it does not match.

### Does Brockley support streaming LLM responses?

Yes. Connect to the SSE streaming endpoint to receive `llm_token` events in real-time as the LLM generates output:

```bash
curl -N http://localhost:8000/api/v1/executions/EXEC_ID/stream
```

## Graphs and Nodes

### How do I add a custom node type?

Brockley has a `NodeTypeRegistry` that lets you register custom node types. You implement the `NodeExecutor` interface in Go and register it with the executor registry.

This requires modifying the Go codebase. Custom node types are not yet dynamically loadable -- they must be compiled into the server/worker binary.

### Can I reuse graphs as subgraphs?

Yes. Use the `subgraph` node type to embed one graph inside another. The subgraph is defined inline in the node's config with a port mapping that connects outer ports to inner graph ports. See [Subgraphs](../concepts/subgraphs.md) for patterns and state isolation details.

### What is the maximum graph size?

There is no hard limit on the number of nodes or edges. In practice, graphs with hundreds of nodes work fine. Execution time depends on the individual node operations (LLM calls, tool invocations) rather than graph size.

### Can graphs call other graphs?

Not directly via reference (e.g., "run graph X"). You can embed a graph inline using the `subgraph` node type. Dynamic graph references are not currently supported.

### How do loops work?

Loops use **back-edges** -- edges marked with `back_edge: true`. A back-edge must have:
- A `condition` expression that determines whether to continue
- A `max_iterations` limit to prevent infinite loops

See [Loops](../concepts/loops.md) for the full guide, or [Execution](../concepts/execution.md) for how loops are executed.

## MCP Tools

### How do I connect an MCP tool?

Use the [`tool` node type](../nodes/tool.md) with the MCP server URL:

```json
{
  "type": "tool",
  "config": {
    "tool_name": "search",
    "mcp_url": "http://my-mcp-server:3001/sse",
    "mcp_transport": "sse"
  }
}
```

The engine discovers the tool's input/output schema from the MCP server and creates ports accordingly.

### What MCP transports are supported?

- `sse` (default) -- Server-Sent Events over HTTP. The most common transport.
- `stdio` -- Standard I/O. For MCP servers running as local processes.

### Can I pass authentication headers to MCP servers?

Yes. Use the `headers` config on tool nodes:

```json
{
  "headers": [
    {"name": "Authorization", "value": "Bearer static-token"},
    {"name": "X-User-Token", "from_input": "user_token"},
    {"name": "X-API-Key", "secret_ref": "MCP_API_KEY"}
  ]
}
```

Headers can be:
- **Static**: hardcoded `value`
- **Dynamic**: from an input port via `from_input`
- **Secret**: from the secret store via `secret_ref`

## Deployment

### What are the minimum system requirements?

For local development: Docker with 4GB+ RAM allocated.

For production:
- Server: 256MB RAM, 0.25 CPU per replica (minimum)
- Worker: 256MB RAM, 0.25 CPU per replica (minimum)
- PostgreSQL: 1GB+ RAM, 10GB+ storage
- Redis: 256MB+ RAM

Actual requirements depend on workload. LLM-heavy workflows are bottlenecked by provider API latency, not local compute.

### Can I run Brockley without Redis?

Partially. Without Redis:
- Health and management API endpoints work
- Graph CRUD operations work
- **Execution does not work** (task queue and event streaming require Redis)

For any workflow execution, Redis is required.

### Can I run Brockley without the web UI?

Yes. The web UI is optional. You can use the API, CLI, or Terraform provider exclusively.

### How do I back up my data?

Back up PostgreSQL using standard PostgreSQL backup tools (`pg_dump`, continuous archiving, or your managed service's backup feature). Redis data is transient (task queue and events) and does not need to be backed up.

### Does Brockley support multiple tenants?

Yes. The data model includes `tenant_id` on all resources. You set the tenant via API headers. The architecture supports clean extension points for additional tenant isolation if needed.

## API

### How do I authenticate API requests?

If `BROCKLEY_API_KEYS` is configured, include the key in the `Authorization` header:

```bash
curl -H "Authorization: Bearer your-api-key" http://localhost:8000/api/v1/graphs
```

Health endpoints (`/health`, `/health/ready`, `/version`) do not require authentication.

### What is the API response format for errors?

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "graph_id is required",
    "request_id": "req-abc123"
  }
}
```

Every error response includes a `code`, `message`, and `request_id` for debugging.

### Is there rate limiting?

The open-source server does not enforce rate limiting. You can add rate limiting at the ingress/load balancer level (e.g., nginx, Kong, or cloud load balancers).

## CLI

### How do I install the CLI?

Build from source:

```bash
git clone https://github.com/brockleyai/brockleyai.git
cd brockleyai
go build -o brockley ./cmd/brockley
# Move to your PATH
mv brockley /usr/local/bin/
```

### Can I validate graphs locally without a server?

Yes. The CLI validates graphs using the engine library directly, with no network calls:

```bash
brockley validate my-graph.yaml
```

## See Also

- [Common Errors](common-errors.md) -- detailed error reference with fixes
- [Introduction](../getting-started/introduction.md) -- product overview
- [Quickstart](../getting-started/quickstart.md) -- get started in 5 minutes
- [Providers Overview](../providers/overview.md) -- how providers work, secret resolution
- [Configuration Reference](../deployment/configuration.md) -- all environment variables

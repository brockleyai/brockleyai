# Monitoring and Observability

Brockley provides three layers of observability: Prometheus metrics, structured logging, and LLM trace export. All are configurable and designed to work together to give full visibility into graph execution.

---

## Prometheus Metrics

### Enabling Metrics

Set the environment variable to enable Prometheus metrics collection:

```bash
BROCKLEY_METRICS_ENABLED=true
```

When enabled, metrics are exposed at the `/metrics` endpoint in Prometheus exposition format. When disabled (the default), a no-op collector is used with zero overhead.

### What Metrics Are Emitted

#### Execution Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `brockley_executions_total` | Counter | `graph_name`, `status` | Total graph executions |
| `brockley_execution_duration_seconds` | Histogram | `graph_name`, `status` | Execution duration |
| `brockley_executions_in_progress` | Gauge | `graph_name` | Currently running executions |

#### Node Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `brockley_node_executions_total` | Counter | `graph_name`, `node_type`, `status` | Total node executions |
| `brockley_node_duration_seconds` | Histogram | `graph_name`, `node_type` | Node execution duration |
| `brockley_loop_iterations_total` | Counter | `graph_name`, `edge_id` | Total loop iterations |

#### Provider Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `brockley_provider_requests_total` | Counter | `provider`, `model`, `status` | Total LLM provider API calls |
| `brockley_provider_duration_seconds` | Histogram | `provider`, `model` | Provider call latency |
| `brockley_provider_tokens_total` | Counter | `provider`, `model`, `type` | Token consumption (`type` = `prompt` or `completion`) |
| `brockley_provider_rate_limits_total` | Counter | `provider` | Rate limit events |

#### MCP Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `brockley_mcp_calls_total` | Counter | `tool_name`, `status` | Total MCP tool calls |
| `brockley_mcp_duration_seconds` | Histogram | `tool_name` | MCP call latency |

#### Queue Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `brockley_queue_tasks_enqueued_total` | Counter | | Total tasks enqueued |
| `brockley_queue_tasks_dequeued_total` | Counter | | Total tasks picked up by workers |
| `brockley_queue_wait_seconds` | Histogram | | Time tasks spend waiting in queue |
| `brockley_queue_depth` | Gauge | | Current queue depth |

#### HTTP Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `brockley_http_requests_total` | Counter | `method`, `path`, `status_code` | Total HTTP requests |
| `brockley_http_duration_seconds` | Histogram | `method`, `path` | HTTP request latency |

### Histogram Buckets

Default duration buckets (in seconds):

```
0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300
```

### Prometheus Scrape Configuration

```yaml
# prometheus.yml
scrape_configs:
  - job_name: brockley
    scrape_interval: 15s
    static_configs:
      - targets: ['brockley-server:8000']
    metrics_path: /metrics
```

### MetricsCollector Interface

The engine uses a `MetricsCollector` interface. When metrics are disabled, a `NoopMetricsCollector` is used. When enabled, a `PrometheusMetricsCollector` registers and updates Prometheus counters, histograms, and gauges.

```go
type MetricsCollector interface {
    ExecutionStarted(graphID, graphName string)
    ExecutionCompleted(graphID, graphName string, durationMs int64, status string)
    NodeStarted(graphID, nodeID, nodeType string)
    NodeCompleted(graphID, nodeID, nodeType string, durationMs int64, status string)
    ProviderCallCompleted(provider, model string, durationMs int64, promptTokens, completionTokens int, status string)
    MCPCallCompleted(toolName, mcpURL string, durationMs int64, status string)
    HTTPRequestCompleted(method, path string, statusCode int, durationMs int64)
}
```

---

## Structured Logging

Brockley uses Go's standard library `log/slog` for structured JSON logging. Logging is always on and cannot be disabled -- only the level can be adjusted.

### Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `BROCKLEY_LOG_LEVEL` | `info` | Minimum log level: `debug`, `info`, `warn`, `error` |
| `BROCKLEY_LOG_FORMAT` | `json` | Output format: `json` (production) or `text` (human-readable for local dev) |

### JSON Log Format

Every log entry is a JSON object with consistent fields:

```json
{
  "time": "2026-03-14T10:30:45.123Z",
  "level": "INFO",
  "msg": "node completed",
  "component": "engine",
  "execution_id": "exec-abc123",
  "graph_id": "graph-xyz",
  "graph_name": "customer-classifier",
  "node_id": "llm-1",
  "node_type": "llm",
  "duration_ms": 1523,
  "status": "completed"
}
```

### Log Levels

| Level | When Used |
|-------|-----------|
| `ERROR` | Node execution failed, provider error, validation failure, database connection lost |
| `WARN` | Retry triggered, rate limit hit, slow provider response (>5s) |
| `INFO` | Execution started/completed, node started/completed, graph saved, server started |
| `DEBUG` | Expression evaluation, port resolution, state read/write, template rendering, provider request bodies (truncated) |

### Correlation IDs

Every log entry within an execution includes `execution_id`. Every log entry within an HTTP request includes `request_id`. These are injected at the boundary and propagated through all child operations.

If the client provides an `X-Request-Id` header, the server uses it. Otherwise, a unique ID is generated.

### Key Log Messages

| Message | Level | Context |
|---------|-------|---------|
| `"execution started"` | INFO | `execution_id`, `graph_id`, `graph_name` |
| `"execution completed"` | INFO | `execution_id`, `graph_id`, `status`, `duration_ms` |
| `"execution failed"` | ERROR | `execution_id`, `graph_id`, `error`, `node_id` |
| `"node started"` | INFO | `execution_id`, `node_id`, `node_type`, `iteration` |
| `"node completed"` | INFO | `execution_id`, `node_id`, `node_type`, `duration_ms` |
| `"node failed"` | ERROR | `execution_id`, `node_id`, `error`, `attempt` |
| `"provider call completed"` | INFO | `execution_id`, `provider`, `model`, `duration_ms`, `prompt_tokens`, `completion_tokens` |
| `"mcp tool call completed"` | INFO | `execution_id`, `tool_name`, `mcp_url`, `duration_ms` |
| `"node retry scheduled"` | WARN | `execution_id`, `node_id`, `attempt`, `max_retries` |

---

## LLM Observability Platforms

Brockley supports optional integration with external LLM observability platforms for prompt/completion tracing, cost tracking, and quality monitoring. All integrations are opt-in.

### Supported Platforms

| Platform | Transport | Self-Hostable | Auth Method |
|----------|-----------|---------------|-------------|
| Langfuse | OTLP HTTP | Yes | Basic Auth (public key + secret key) |
| Opik (Comet) | OTLP HTTP | Yes | API key + workspace header |
| Arize Phoenix | OTLP HTTP/gRPC | Yes | Bearer token (optional for self-hosted) |
| LangSmith | Custom REST API | No (cloud only) | API key |

### Configuring Langfuse

```bash
BROCKLEY_TRACE_LANGFUSE_ENABLED=true
BROCKLEY_TRACE_LANGFUSE_HOST=https://cloud.langfuse.com    # or self-hosted URL
BROCKLEY_TRACE_LANGFUSE_PUBLIC_KEY=pk-lf-...
BROCKLEY_TRACE_LANGFUSE_SECRET_KEY=sk-lf-...
```

Langfuse natively supports OTLP. The exporter sends spans to Langfuse's `/api/public/otel/v1/traces` endpoint using Basic Auth.

### Configuring Opik

```bash
BROCKLEY_TRACE_OPIK_ENABLED=true
BROCKLEY_TRACE_OPIK_HOST=https://www.comet.com/opik        # or self-hosted URL
BROCKLEY_TRACE_OPIK_API_KEY=sk-opik-...
BROCKLEY_TRACE_OPIK_WORKSPACE=default
```

Opik supports OTLP. The exporter sends spans to Opik's `/api/v1/private/otel` endpoint with API key and workspace headers.

### Configuring Arize Phoenix

```bash
BROCKLEY_TRACE_PHOENIX_ENABLED=true
BROCKLEY_TRACE_PHOENIX_HOST=http://localhost:6006           # or cloud URL
BROCKLEY_TRACE_PHOENIX_API_KEY=                             # optional for self-hosted
```

Phoenix supports standard OTLP. The exporter sends spans to Phoenix's `/v1/traces` endpoint with an optional bearer token.

### Configuring LangSmith

```bash
BROCKLEY_TRACE_LANGSMITH_ENABLED=true
BROCKLEY_TRACE_LANGSMITH_API_KEY=ls-...
BROCKLEY_TRACE_LANGSMITH_PROJECT=brockley-traces
```

LangSmith uses a custom REST API (not OTLP). The exporter maps spans to LangSmith's Run model and sends them via `POST /runs`.

### Generic OTLP

For any OpenTelemetry-compatible receiver (Jaeger, Grafana Tempo, Datadog, etc.):

```bash
BROCKLEY_TRACE_OTLP_ENABLED=true
BROCKLEY_TRACE_OTLP_ENDPOINT=http://otel-collector:4318/v1/traces
BROCKLEY_TRACE_OTLP_HEADERS="Authorization=Bearer token123"
BROCKLEY_TRACE_OTLP_PROTOCOL=http/protobuf    # or http/json, grpc
```

### What Gets Traced

| Operation | Span Kind | Attributes |
|-----------|-----------|------------|
| Graph execution (overall) | CHAIN | `input.value`, `output.value` |
| LLM node execution | LLM | `llm.system`, `llm.model_name`, `llm.input_messages`, `llm.output_messages`, token counts |
| Tool node execution (MCP) | TOOL | `tool.name`, `tool.parameters`, `tool.result` |
| Subgraph execution | CHAIN | `input.value`, `output.value` |
| ForEach execution | CHAIN | Parent span with child spans per item |

Conditional and transform nodes are not traced (too fast/simple to produce useful trace data).

### Span Hierarchy

Spans are hierarchical. Each execution produces a root CHAIN span, with child spans for each node:

```
Execution (CHAIN)
  +-- Node: classify (LLM)
  |     +-- LLM call to Anthropic
  +-- Node: route (not traced)
  +-- Node: fetch_data (TOOL)
  |     +-- MCP tool call
  +-- Node: summarize (LLM)
        +-- LLM call to OpenAI
```

ForEach creates nested spans:

```
Execution (CHAIN)
  +-- Node: process_items (CHAIN)
        +-- Item 0 (CHAIN)
        |     +-- Node: analyze (LLM)
        +-- Item 1 (CHAIN)
              +-- Node: analyze (LLM)
```

### OpenInference Attributes

Trace spans follow the OpenInference standard for LLM observability. Key attributes on LLM spans:

| Attribute | Description |
|-----------|-------------|
| `openinference.span.kind` | `"LLM"`, `"TOOL"`, or `"CHAIN"` |
| `llm.system` | Provider name (e.g., `"openai"`, `"anthropic"`) |
| `llm.model_name` | Model identifier |
| `llm.input_messages` | Rendered prompt messages |
| `llm.output_messages` | LLM response |
| `llm.token_count.prompt` | Prompt token count |
| `llm.token_count.completion` | Completion token count |
| `brockley.execution_id` | Execution ID |
| `brockley.graph_id` | Graph ID |
| `brockley.node_id` | Node ID |

---

## Privacy Controls

By default, full prompt and completion text is included in trace spans. This may contain sensitive data. Use these environment variables to control what is exported:

```bash
BROCKLEY_TRACE_REDACT_PROMPTS=true       # Replace prompt content with "[REDACTED]"
BROCKLEY_TRACE_REDACT_COMPLETIONS=true   # Replace completion content with "[REDACTED]"
BROCKLEY_TRACE_INCLUDE_METADATA_ONLY=true  # Only send token counts, timing, model -- no content
```

These apply globally to all configured exporters.

---

## Health Endpoints

| Endpoint | Purpose | Details |
|----------|---------|---------|
| `GET /health` | Liveness | Returns 200 if the process is alive. No dependency checks. |
| `GET /health/ready` | Readiness | Returns 200 if server, PostgreSQL, and Redis are connected. 503 otherwise. |
| `GET /metrics` | Prometheus | Metrics in Prometheus exposition format (only if metrics enabled). |

## See Also

- [Configuration Reference](configuration.md) -- all environment variables including trace and metrics config
- [Local Development](local-dev.md) -- development setup with Docker Compose
- [Health API](../api/health.md) -- health endpoint reference
- [Kubernetes Deployment](kubernetes.md) -- probe configuration, HPA setup
- [Architecture Overview](../specs/architecture.md) -- system components and data flow

# Architecture Overview

Brockley consists of five main components that work together to provide a complete agent workflow platform.

## System Diagram

```
                    ┌─────────────────────────────────────┐
                    │          User Interfaces             │
                    ├──────────┬──────────┬───────────────┤
                    │  Web UI  │Terraform │  CLI / MCP    │
                    │ (React)  │ Provider │  Server       │
                    └────┬─────┴────┬─────┴──────┬────────┘
                         │          │            │
                         ▼          ▼            ▼
                    ┌─────────────────────────────────────┐
                    │           API Server                 │
                    │  ┌─────────────┬──────────────────┐  │
                    │  │ Management  │  Execution        │  │
                    │  │ API (CRUD)  │  API (invoke)     │  │
                    │  └─────────────┴──────────────────┘  │
                    └──────┬──────────────┬───────────────┘
                           │              │
                    ┌──────┼──────┐       │
                    │      │      │       │
                    ▼      ▼      ▼       ▼
               ┌────────┐ ┌───────┐ ┌──────────┐
               │Postgres│ │ Redis │ │ Workers  │
               │   SQL  │ │       │ │ (Asynq)  │
               └────────┘ └───────┘ └────┬─────┘
                                         │
                                         ▼
                               ┌──────────────────┐
                               │ Execution Engine  │
                               │ ├─ Orchestrator   │
                               │ ├─ Node Executors │
                               │ ├─ LLM Providers  │
                               │ └─ MCP Client     │
                               └──────────────────┘
```

## Components

### API Server

The API server is a Go HTTP server built on `net/http`. It exposes a REST API for managing graphs, schemas, prompt templates, provider configs, and executions.

**Key endpoints:**

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/api/v1/graphs` | Create a graph |
| `GET` | `/api/v1/graphs` | List graphs |
| `GET` | `/api/v1/graphs/{id}` | Get a graph |
| `PUT` | `/api/v1/graphs/{id}` | Update a graph |
| `DELETE` | `/api/v1/graphs/{id}` | Delete a graph |
| `POST` | `/api/v1/graphs/{id}/validate` | Validate a graph |
| `POST` | `/api/v1/executions` | Invoke a graph |
| `GET` | `/api/v1/executions/{id}` | Get execution status |
| `GET` | `/api/v1/executions/{id}/steps` | Get execution steps |
| `GET` | `/api/v1/executions/{id}/stream` | Stream execution events (SSE) |
| `POST` | `/api/v1/executions/{id}/cancel` | Cancel an execution |
| `GET` | `/health` | Health check |
| `GET` | `/health/ready` | Readiness check (DB + Redis) |
| `GET` | `/metrics` | Prometheus metrics (opt-in) |

The server handles authentication (API key), request IDs, CORS, and structured logging.

**Default port:** `8000`

### Workers

Workers are separate processes that execute graphs asynchronously. When you invoke a graph via the API, the server enqueues a task in Redis. A worker picks up the task, runs the graph through the execution engine, and writes results back to PostgreSQL.

Workers use [Asynq](https://github.com/hibiken/asynq), a Go task queue backed by Redis. You can run multiple workers for horizontal scaling. Each worker has a configurable concurrency level.

**Key configuration:**
- `BROCKLEY_CONCURRENCY` -- number of concurrent tasks per worker (default: 10)

### Execution Engine

The engine is the core of Brockley. It is a standalone Go library (the `engine/` package) with no HTTP or database dependencies. The engine:

1. **Validates** the graph structure, port types, edge connections, cycles, and state bindings
2. **Plans** execution order using topological sorting
3. **Executes** nodes in order, running independent nodes in parallel
4. **Manages state** across the execution, applying reducers for state writes
5. **Handles loops** via back-edges with conditions and iteration limits
6. **Propagates skips** through conditional branches
7. **Emits events** for real-time streaming (node started, completed, failed, LLM tokens, etc.)

The engine contains these sub-packages:

| Package | Purpose |
|---------|---------|
| `engine/graph` | Graph validation (structure, typing, cycles, reachability) |
| `engine/orchestrator` | Execution orchestration, topological walk, loop handling |
| `engine/executor` | Node type executors (LLM, transform, conditional, foreach, etc.) |
| `engine/expression` | Expression language parser and evaluator |
| `engine/provider` | LLM provider implementations (OpenAI, Anthropic, Google, Bedrock, OpenRouter) |

### PostgreSQL

PostgreSQL is the primary data store. It stores:

- Graphs (with nodes, edges, and state as JSONB columns)
- Schemas, prompt templates, and provider configs (the "library")
- Executions and execution steps
- Tenant data (for multi-tenancy)

The server runs migrations automatically on startup. No manual schema setup is required.

### Redis

Redis serves two purposes:

1. **Task queue** -- Asynq uses Redis to enqueue and distribute execution tasks to workers
2. **Event streaming** -- execution events (node started, node completed, LLM tokens, etc.) are published to Redis Pub/Sub channels, enabling SSE streaming to clients

## How Data Flows

Here is what happens when you execute a graph:

1. **Client sends POST /api/v1/executions** with `graph_id`, `input`, and `mode` (sync or async)
2. **Server loads the graph** from PostgreSQL
3. **Server creates an execution record** in PostgreSQL with status `pending`
4. **Server enqueues a task** in Redis via Asynq
5. **Worker picks up the task** from the Redis queue
6. **Worker runs the execution engine:**
   - Validates the graph
   - Builds topological order
   - Walks nodes in order, executing each one
   - For LLM nodes: calls the configured provider (OpenAI, Anthropic, etc.)
   - For tool nodes: calls the MCP server
   - Applies state updates after each node
   - Emits events to Redis Pub/Sub
7. **Worker writes results** back to PostgreSQL (execution output, steps, status)
8. **Client receives results:**
   - **Sync mode**: server polls for completion and returns the result
   - **Async mode**: server returns the execution ID immediately; client polls or streams
   - **Streaming**: client connects to SSE endpoint and receives events in real-time

## Deployment Topology

### Local Development

```
make dev
```

Runs everything in Docker Compose: server, worker, web UI, PostgreSQL, and Redis. All on a single machine. Hot-reload is enabled for the server and worker.

### Production (Kubernetes)

In production, you typically deploy:

- **2+ server replicas** behind a load balancer
- **2+ worker replicas** for execution throughput
- **1 web UI replica** (static frontend)
- **External PostgreSQL** (RDS, Cloud SQL, etc.)
- **External Redis** (ElastiCache, Memorystore, etc.)

The included Helm chart supports both embedded (single-pod) and external database/Redis configurations.

## Security

**API key authentication.** Configure one or more API keys via `BROCKLEY_API_KEYS` (comma-separated). Every API request must include a valid key in the `Authorization` header. In development mode (`BROCKLEY_AUTH_DISABLED=true`), authentication is bypassed so you can test without keys.

**Secrets handling.** LLM provider API keys are never stored in graph definitions. Nodes reference keys by name (e.g., `api_key_ref: "OPENAI_API_KEY"`), and the server resolves them from environment variables at execution time. This keeps secrets out of graph JSON, version control, and API responses.

**CORS.** Configure allowed origins via `BROCKLEY_CORS_ORIGINS`. Defaults to `*` in development. In production, restrict this to your actual frontend domains.

**Request tracing.** Every API request gets a unique `X-Request-ID` header. This ID propagates through the server, worker, and execution engine, appearing in all log entries and execution records.

**Network isolation.** Workers pull tasks from Redis -- they never expose HTTP endpoints. The only externally-facing service is the API server. In Kubernetes, use NetworkPolicy to restrict worker egress to only LLM provider endpoints and MCP servers.

For a full list of security-related configuration options, see [Configuration Reference](../deployment/configuration.md).

## Observability

**Structured logging.** All components emit structured logs via Go's `slog` package. Set `BROCKLEY_LOG_FORMAT` to `json` (default) or `text`. Control verbosity with `BROCKLEY_LOG_LEVEL` (`debug`, `info`, `warn`, `error`). Every log entry includes `execution_id` and `request_id` for correlation.

**Prometheus metrics.** Enable via `BROCKLEY_METRICS_ENABLED=true`. The server exposes a `/metrics` endpoint with standard Go runtime metrics plus Brockley-specific counters and histograms: execution count, execution duration, node execution duration by type, LLM token usage, and task queue depth.

**Execution tracing.** Brockley exports OpenTelemetry-compatible traces via OTLP. Configure with `BROCKLEY_TRACE_EXPORTER` and `BROCKLEY_TRACE_ENDPOINT` to send traces to Langfuse, Opik, Phoenix, LangSmith, or any OTLP-compatible collector. Traces include spans for each node execution with LLM token counts and tool call details.

**Execution step records.** Every node execution is recorded as a step with full input data, output data, state snapshots (before and after), execution duration, and LLM usage (provider, model, prompt tokens, completion tokens, cost estimate). Query steps via `GET /api/v1/executions/{id}/steps`.

**Real-time streaming.** Connect to `GET /api/v1/executions/{id}/stream` for Server-Sent Events. Events include node start/complete/fail, LLM token delivery, state updates, foreach item progress, superagent iterations, and more.

For monitoring setup and alerting recommendations, see [Monitoring](../deployment/monitoring.md). For all environment variables, see [Configuration Reference](../deployment/configuration.md).

## Next Steps

- [Graphs](../concepts/graphs.md) -- deep dive into graph structure, validation, and versioning
- [Nodes](../concepts/nodes.md) -- all built-in node types and their configuration
- [Execution Model](../concepts/execution.md) -- how the orchestrator runs graphs
- [Local Development Setup](../deployment/local-dev.md) -- details on the development environment
- [Configuration Reference](../deployment/configuration.md) -- all environment variables
- [Monitoring](../deployment/monitoring.md) -- metrics, alerting, and trace export setup
- [Kubernetes Deployment](../deployment/kubernetes.md) -- production deployment with Helm

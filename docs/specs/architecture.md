# Architecture Overview

Brockley is an open-source, self-hostable platform for defining, validating, executing, and operationalizing AI agent workflows. This document describes the high-level architecture for contributors and operators.

---

## High-Level Architecture

Brockley is structured as a layered system with clear separation between user interfaces, API surface, execution infrastructure, and storage.

```text
                            ┌──────────────────────────────────────┐
                            │          USER INTERFACES             │
                            ├──────────┬───────────┬───────────────┤
                            │ Web UI   │ Terraform │ CLI / MCP     │
                            │ (React)  │ Provider  │ Server        │
                            └────┬─────┴─────┬─────┴──────┬────────┘
                                 │           │            │
                                 ▼           ▼            ▼
                            ┌──────────────────────────────────────┐
                            │            API SERVER                │
                            │  ┌──────────────┬─────────────────┐  │
                            │  │ Management   │ Execution       │  │
                            │  │ API (CRUD)   │ API (invoke)    │  │
                            │  └──────────────┴─────────────────┘  │
                            └──────────┬───────────────────────────┘
                                       │
                            ┌──────────┼───────────┐
                            │          │           │
                            ▼          ▼           ▼
                       ┌────────┐ ┌───────┐ ┌──────────┐
                       │Postgres│ │ Redis │ │ Workers  │
                       │        │ │       │ │ (asynq)  │
                       └────────┘ └───────┘ └────┬─────┘
                                                 │
                                                 ▼
                                       ┌──────────────────┐
                                       │  Execution Engine │
                                       │  ─────────────── │
                                       │  Orchestrator     │
                                       │  Node Executors   │
                                       │  LLM Providers    │
                                       │  MCP Client       │
                                       └──────────────────┘
```

---

## User Interfaces

All user interfaces communicate exclusively through the API Server. None interact with the database, Redis, or workers directly.

**Web UI (React)** -- A browser-based visual interface for building graphs, configuring nodes, managing schemas, viewing execution history, and monitoring runs. Communicates over HTTP and WebSocket (for streaming execution updates).

**Terraform / OpenTofu Provider** -- A Go-based Terraform provider that maps Brockley resources (graphs, schemas, prompt templates, provider configs) to Terraform resource types. Enables version-controlled, reviewable, CI/CD-driven workflow management.

**CLI** -- A developer-oriented command-line tool for validating graphs, invoking executions, inspecting results, exporting definitions, and scripting automation.

**MCP Server** -- An MCP-compatible tool server that exposes Brockley operations (create graph, add node, validate, invoke, export) as tools that AI coding agents can call.

---

## API Server

The API Server is the single gateway for all external access. It exposes two logical API surfaces:

**Management API** -- CRUD operations for all persistent domain objects: graphs, nodes, edges, schemas, prompt templates, provider configs. Also handles validation endpoints, listing, search, and export.

**Execution API** -- Invocation of graph executions (sync, async, streaming), status polling, cancellation, and retrieval of execution results and step-level details.

The API Server is a Go application. It handles authentication, request validation, and routing. It does not execute graph logic directly; it delegates execution to workers.

---

## Storage

**PostgreSQL** -- Primary persistent store for all domain objects (graphs, schemas, templates, configs) and execution records (executions, execution steps). Chosen for strong relational integrity, JSONB support for flexible document storage, and mature operational tooling.

**Redis** -- Used for:
- asynq task broker (task queue for execution dispatch -- task payloads contain full graph + inputs)
- asynq result storage
- Pub/sub for real-time execution event streaming (worker -> server -> client)
- Caching (optional, for frequently accessed graphs and configs)

---

## Workers (asynq)

Background task workers that process distributed execution tasks from Redis. Workers are horizontally scalable. Each worker process embeds the engine package.

### Distributed Execution Model

The execution engine uses a distributed task model with four asynq task types:

| Task Type | Role | Queue |
|-----------|------|-------|
| `graph:start` | Orchestrator -- stays alive, walks graph, dispatches node tasks, collects results | `orchestrator` |
| `node:llm-call` | One `provider.Complete()` call; handles tool loop MCP dispatch | `nodes` |
| `node:mcp-call` | One `client.CallTool()` or `client.ListTools()` call | `nodes` |
| `node:run` | Complex node execution (forEach, subgraph) | `nodes` |
| `node:code-exec` | Python code execution requested by a superagent | `code` |

Pure-computation nodes (transform, conditional, input, output) execute in-process within the orchestrator.

### Coderunner

The coderunner is a separate binary (`cmd/coderunner`) that processes `node:code-exec` tasks from the `code` queue. It executes Python code in a subprocess sandbox and relays tool calls back to the superagent handler through a Redis relay protocol.

**Redis relay protocol:** When a superagent's LLM calls the `_code_execute` tool, the superagent handler enqueues a `node:code-exec` task. The coderunner picks it up, launches a Python subprocess, and executes the code. If the Python code calls tools (e.g., MCP tools), the coderunner publishes tool call requests to a Redis key (`exec:{execution_id}:code:{request_id}:tool-requests`) and BRPOPs for results on `exec:{execution_id}:code:{request_id}:tool-results`. The superagent handler monitors the request key, dispatches the tool calls, and pushes results back. This relay continues until the Python code completes, at which point the coderunner pushes the final result to the standard node result key.

### Result Delivery

All node tasks push results to a Redis list per execution (`exec:{execution_id}:results`). The orchestrator BRPOPs from this list. Tool loop MCP results use a scoped key (`exec:{execution_id}:llm:{request_id}:mcp-results`).

### Event Pipeline

Workers publish execution events to Redis pub/sub (real-time) and write execution steps to PostgreSQL (fire-and-forget, non-blocking). The final execution status is written synchronously.

### Queue Priorities

The `nodes` queue has higher priority (7) than `orchestrator` (3), ensuring node tasks are picked up quickly while orchestrators wait on results. The `code` queue has the same priority as `nodes` (7) and is processed by the coderunner binary.

---

## Execution Engine

The engine is the core computational component. It is designed as a **standalone Go package** that can be used independently of the API server.

**Orchestrator** -- The `graph:start` task handler. Walks the graph in topological order, dispatches network-bound operations (LLM calls, MCP calls) as separate asynq tasks, runs pure-computation nodes in-process, manages state, handles loops via back-edges, and collects results via Redis BRPOP.

**Node Executors** -- Pluggable executors for each node type:
- **LLM Node**: Sends prompts to an LLM provider, processes structured or unstructured responses
- **Tool Node**: Calls an MCP tool or a built-in function
- **Conditional Node**: Evaluates a condition and selects a branch
- **Transform Node**: Applies data transformations (mapping, filtering, aggregation)
- **Human-in-the-Loop Node**: Pauses execution and waits for external input

**LLM Providers** -- Abstraction layer over LLM APIs (OpenAI, Anthropic, Google, local models). Handles auth, rate limiting, retries, token counting, and response normalization.

**MCP Client** -- Connects to external MCP servers to discover and invoke tools. Manages tool schemas, connection lifecycle, and error handling.

---

## Data Flow

### Graph Definition Flow

1. User creates or edits a graph via any interface (UI, Terraform, CLI, MCP)
2. Interface sends requests to Management API
3. API Server validates the graph structure and schema compatibility
4. Validated graph is persisted to PostgreSQL

### Execution Flow

1. Client sends an invocation request to the Execution API with a graph ID and input data
2. API Server validates input against the graph's input schema
3. API Server creates an Execution record in PostgreSQL with status `pending`
4. API Server enqueues an asynq task via Redis -- task payload contains the **full self-contained graph** and inputs (worker doesn't need to read from PostgreSQL to start)
5. A worker picks up the `graph:start` task from Redis
6. Orchestrator walks the graph in topological order:
   a. For each group of independent nodes, dispatches tasks: `node:llm-call` for LLM nodes, `node:mcp-call` for tool nodes, `node:run` for forEach/subgraph nodes
   b. Pure-computation nodes (transform, conditional, input, output) execute in-process
   c. Orchestrator waits for distributed results via `BRPOP` on `exec:{id}:results`
   d. Progress events are published to Redis pub/sub channel `execution:{id}:events`
   e. Execution steps are sent to a background goroutine that writes to PostgreSQL (fire-and-forget, non-blocking)
7. On completion, orchestrator flushes all pending step writes to PostgreSQL, then writes the final Execution record synchronously
8. Orchestrator publishes `execution_completed` event to Redis pub/sub
9. Client retrieves results via polling (reads from PostgreSQL) or streaming (receives from Redis pub/sub)

### Streaming Flow

1. Client opens an SSE stream to `GET /api/v1/executions/{id}/stream`
2. API Server subscribes to Redis pub/sub channel `execution:{id}:events`
3. As nodes complete, worker publishes events to Redis -- server forwards them to the client in real time
4. Events include full node input/output data, timing, iteration index, LLM token usage
5. ForEach nodes emit per-item events with `item_index` and `item_total`
6. On `execution_completed` or `execution_failed` event, the stream closes
7. Multiple server instances work correctly -- they all subscribe to the same Redis channel

### Sync Execution Flow

1. Client sends `POST /api/v1/executions` with `mode: "sync"`
2. Server creates Execution, subscribes to Redis channel, then enqueues task (subscribe-before-enqueue prevents race conditions with fast workers)
3. Server blocks until `execution_completed` or `execution_failed` event
4. Server returns the result inline in the HTTP response

---

## Deployment Topology

### Local Development (Docker Compose)

```text
┌─────────────────────────────────────────┐
│  docker-compose                         │
│  ┌─────────────┐  ┌─────────────┐      │
│  │ api-server   │  │ worker (x1) │      │
│  │ :8000        │  │             │      │
│  └──────┬───────┘  └──────┬──────┘      │
│         │                 │             │
│  ┌──────┴───────┐  ┌──────┴──────┐      │
│  │ postgres:5432│  │ redis:6379  │      │
│  └──────────────┘  └─────────────┘      │
│                                         │
│  ┌─────────────┐                        │
│  │ web-ui:3000  │                        │
│  └─────────────┘                        │
└─────────────────────────────────────────┘
```

### Production (Kubernetes / Helm)

- API Server: Deployment with HPA, behind an Ingress
- Workers: Deployment with HPA, scaled based on queue depth
- Coderunner: Deployment with HPA, processes `node:code-exec` tasks from the `code` queue. Requires Python 3 runtime. Should run with restricted security context (no network access from user code, memory/CPU limits enforced).
- PostgreSQL: External managed PostgreSQL (RDS, Cloud SQL, etc.) or StatefulSet
- Redis: External managed service (ElastiCache) or StatefulSet
- Web UI: Static build served by Nginx or CDN

---

## Key Design Principles

### Engine Independence
The execution engine is usable as a standalone library without the API server, database, or worker infrastructure. This enables embedding, testing, and alternative deployment models.

### Interface Parity
All four interfaces (Web UI, Terraform, CLI, MCP) have equivalent capabilities for the operations they support. No critical workflow requires a specific interface.

### Schema-First Validation
Inputs, outputs, and inter-node data are validated against explicit schemas before execution. Fail early, fail clearly.

### Horizontal Scalability
The API server and workers are stateless and horizontally scalable. All shared state lives in PostgreSQL and Redis.

### Deterministic Execution
Given the same graph definition and inputs, execution produces the same control flow. LLM outputs are inherently non-deterministic, but routing, validation, and orchestration logic are deterministic.

### Extensibility
New node types, LLM providers, and MCP tool integrations are addable without modifying core engine code.

---

## Cross-Cutting Concerns

### Authentication
- API key-based authentication
- All auth is enforced at the API Server; the engine is auth-unaware

### Error Handling
- Structured error responses with error codes, messages, and context
- Node-level retries with configurable backoff
- Graph-level timeout enforcement
- Partial execution results preserved on failure

### Observability
- Structured logging (JSON) from all components
- Request-level tracing (correlation IDs propagated from API to worker to engine)
- Metrics: execution count, duration, failure rate, queue depth, LLM token usage

### Configuration
- Environment variables for infrastructure config (DB URIs, Redis URIs, ports)
- PostgreSQL-stored config for domain objects (graphs, schemas, templates)
- Provider configs stored encrypted at rest (API keys, secrets)

## See Also

- [Data Model](data-model.md) -- entity fields, relationships, validation rules
- [Graph Model](graph-model.md) -- execution model: ports, state, branching, loops
- [API Design](api-design.md) -- REST API endpoints
- [Expression Language](expression-language.md) -- expression language specification
- [Configuration Reference](../deployment/configuration.md) -- environment variables
- [Local Development](../deployment/local-dev.md) -- Docker Compose setup

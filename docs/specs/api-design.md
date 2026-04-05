# API Design

This document defines the REST API design for the Brockley API Server. The API is divided into two logical surfaces: the Management API (CRUD for domain objects) and the Execution API (invocation, status, cancellation, streaming).

---

## General Conventions

### Base URL

```
https://{host}/api/v1
```

### Content Type

All request and response bodies use `application/json` unless otherwise specified.

### Authentication

All requests must include an authentication header:

```
Authorization: Bearer {api_key}
```

### Pagination

List endpoints support cursor-based pagination:

```
GET /api/v1/graphs?limit=20&cursor={opaque_cursor}
```

Response includes:

```json
{
  "items": [...],
  "next_cursor": "abc123",
  "has_more": true
}
```

Default limit: 20. Maximum limit: 100.

### Filtering and Sorting

```
GET /api/v1/graphs?namespace=production&status=active
GET /api/v1/graphs?sort_by=created_at&sort_order=desc
```

Default sort: `created_at` descending.

### Error Responses

All errors follow a consistent structure:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Graph contains a cycle between nodes: node_a -> node_b -> node_a",
    "details": {
      "field": "edges",
      "nodes": ["node_a", "node_b"]
    },
    "request_id": "req_abc123"
  }
}
```

### Standard Error Codes

| HTTP Status | Error Code | Description |
|---|---|---|
| 400 | `VALIDATION_ERROR` | Request body or parameters are invalid |
| 400 | `SCHEMA_VALIDATION_ERROR` | Data does not match the expected schema |
| 401 | `UNAUTHORIZED` | Missing or invalid authentication |
| 403 | `FORBIDDEN` | Insufficient permissions |
| 404 | `NOT_FOUND` | Resource does not exist |
| 409 | `CONFLICT` | Resource version conflict or duplicate name |
| 422 | `GRAPH_INVALID` | Graph structure is invalid |
| 429 | `RATE_LIMITED` | Too many requests |
| 500 | `INTERNAL_ERROR` | Unexpected server error |
| 503 | `SERVICE_UNAVAILABLE` | Server is not ready |

### Request IDs

Every response includes a `X-Request-Id` header. If the client sends an `X-Request-Id` header, the server uses it; otherwise one is generated.

---

## Management API

### Graphs

#### List Graphs

```
GET /api/v1/graphs
```

Query parameters: `namespace`, `status`, `limit`, `cursor`, `sort_by`, `sort_order`

Response: `200 OK`

```json
{
  "items": [
    {
      "id": "graph_abc123",
      "name": "customer-support-agent",
      "namespace": "default",
      "version": 3,
      "status": "active",
      "description": "Handles customer support tickets",
      "node_count": 5,
      "edge_count": 6,
      "created_at": "2026-03-10T12:00:00Z",
      "updated_at": "2026-03-12T14:30:00Z"
    }
  ],
  "next_cursor": null,
  "has_more": false
}
```

#### Get Graph

```
GET /api/v1/graphs/{graph_id}
```

Response: `200 OK` -- full graph with embedded nodes and edges. The `api_key` field on LLM node configs is masked.

#### Create Graph

```
POST /api/v1/graphs
```

Request body: Full graph definition with nodes, edges, and optional state. See `data-model.md` for field definitions.

Response: `201 Created` with the full graph object.

#### Update Graph

```
PUT /api/v1/graphs/{graph_id}
```

Full replacement of the graph definition. Increments `version` automatically.

**API key masking:** The `api_key` field on LLM node configs is masked in GET responses (first 4 + `"..."` + last 4 characters for keys >= 8 chars; `"****"` for shorter keys). On PUT, if a submitted `api_key` matches the masked pattern, the server preserves the original key.

Response: `200 OK` with the updated graph.

#### Delete Graph

```
DELETE /api/v1/graphs/{graph_id}
```

Soft-deletes the graph. Returns `204 No Content`.

#### Validate Graph

```
POST /api/v1/graphs/{graph_id}/validate
```

Validates graph structure, port type compatibility, expression validity, strong typing rules, and node configurations without executing.

Response: `200 OK`

```json
{
  "valid": true,
  "warnings": [
    {
      "code": "UNUSED_NODE",
      "message": "Node 'debug_log' has no incoming edges and is not an entry node",
      "node_id": "debug_log"
    }
  ]
}
```

#### Export Graph

```
GET /api/v1/graphs/{graph_id}/export?format={format}
```

Supported formats: `json`, `terraform`, `yaml`

---

### Schemas (Library)

Reusable JSON Schema definitions that get copied into port definitions when building graphs. Not referenced at runtime.

```
GET    /api/v1/schemas                    # List (query: namespace, limit, cursor)
GET    /api/v1/schemas/{schema_id}        # Get
POST   /api/v1/schemas                    # Create
PUT    /api/v1/schemas/{schema_id}        # Update
DELETE /api/v1/schemas/{schema_id}        # Delete
```

---

### Prompt Templates (Library)

Reusable prompt templates that get copied into LLM node configs when building graphs. Not referenced at runtime.

```
GET    /api/v1/prompt-templates                     # List (query: namespace, limit, cursor)
GET    /api/v1/prompt-templates/{template_id}       # Get
POST   /api/v1/prompt-templates                     # Create
PUT    /api/v1/prompt-templates/{template_id}       # Update
DELETE /api/v1/prompt-templates/{template_id}       # Delete
```

---

### API Tool Definitions (Library)

Reusable REST/HTTP API tool definitions that catalog endpoints with shared configuration (base URL, auth, retry). Individual endpoints are cherry-picked per node -- definitions are not bulk-imported. Not referenced at runtime.

```
POST   /api/v1/api-tools                    # Create
GET    /api/v1/api-tools                    # List (query: namespace, limit, cursor)
GET    /api/v1/api-tools/{id}              # Get
PUT    /api/v1/api-tools/{id}              # Update
DELETE /api/v1/api-tools/{id}              # Delete
POST   /api/v1/api-tools/{id}/test         # Test endpoint call
```

Create/Update request body includes the full definition with endpoints, headers, retry config, etc. See `data-model.md` for field definitions.

Secret references in headers (e.g., `{{secret.stripe_api_key}}`) are resolved at execution time, not stored inline.

**Test endpoint** (`POST /api/v1/api-tools/{id}/test`): Executes a single endpoint call with sample input. Request body: `{"endpoint": "<name>", "input": {...}, "base_url_override": "optional"}`. Returns `{"success": bool, "result": any, "error": string, "is_error": bool, "duration_ms": int}`.

---

### Provider Configs (Library)

Reusable LLM provider settings that get copied into LLM node configs when building graphs. Not referenced at runtime.

```
GET    /api/v1/provider-configs                    # List (query: namespace, provider, limit, cursor)
GET    /api/v1/provider-configs/{config_id}        # Get
POST   /api/v1/provider-configs                    # Create
PUT    /api/v1/provider-configs/{config_id}        # Update
DELETE /api/v1/provider-configs/{config_id}        # Delete
```

The `api_key` field is accepted on create/update but stored as a secret reference. It is never returned in GET responses.

---

## Execution API

### Invoke Graph

```
POST /api/v1/executions
```

Request body:

```json
{
  "graph_id": "graph_abc123",
  "input": {
    "ticket_id": "T-1234",
    "subject": "Billing issue",
    "body": "I was charged twice for my subscription."
  },
  "mode": "async",
  "timeout_seconds": 300,
  "correlation_id": "ext-req-5678",
  "metadata": {
    "source": "api"
  }
}
```

**Mode options:**

- `sync` -- blocks until execution completes or times out. Response includes the full result.
- `sync` requests may remain open for the full execution timeout window.
- `async` -- returns immediately with execution ID. Client polls or streams for results.
- `stream` -- returns a streaming response (SSE) with real-time step events.

**Async response:** `202 Accepted`

```json
{
  "execution_id": "exec_xyz789",
  "status": "pending",
  "poll_url": "/api/v1/executions/exec_xyz789",
  "stream_url": "/api/v1/executions/exec_xyz789/stream"
}
```

### Get Execution

```
GET /api/v1/executions/{execution_id}
```

### List Executions

```
GET /api/v1/executions
```

Query parameters: `graph_id`, `status`, `trigger`, `limit`, `cursor`, `sort_by`, `sort_order`

### Get Execution Steps

```
GET /api/v1/executions/{execution_id}/steps
```

Returns all execution steps ordered by `created_at`.

### Cancel Execution

```
POST /api/v1/executions/{execution_id}/cancel
```

Best-effort cancellation. Returns `409 Conflict` if already in a terminal state.

### Stream Execution Events

```
GET /api/v1/executions/{execution_id}/stream
```

Returns a Server-Sent Events (SSE) stream.

Event types:

```
event: step_started
data: {"step_id": "step_001", "node_id": "classify", "node_type": "llm", "attempt": 1}

event: step_completed
data: {"step_id": "step_001", "node_id": "classify", "status": "completed", "duration_ms": 1250}

event: step_failed
data: {"step_id": "step_002", "node_id": "route", "error": {"code": "CONDITION_ERROR", "message": "..."}}

event: execution_completed
data: {"execution_id": "exec_xyz789", "status": "completed", "output": {...}}

event: execution_failed
data: {"execution_id": "exec_xyz789", "status": "failed", "error": {...}}

event: llm_token
data: {"step_id": "step_003", "node_id": "respond", "token": "I", "index": 0}
```

Clients should handle reconnection. The server supports `Last-Event-ID` for resuming streams.

#### Tool Loop Events

When an LLM node has `tool_loop` enabled, the stream emits additional event types to track tool call progress:

```
event: tool_call_started
data: {"execution_id": "exec_xyz789", "node_id": "agent", "tool_name": "search", "arguments": {"query": "latest pricing"}, "iteration": 1}

event: tool_call_completed
data: {"execution_id": "exec_xyz789", "node_id": "agent", "tool_name": "search", "result_preview": "Found 3 results...", "duration_ms": 450, "is_error": false}

event: tool_loop_iteration
data: {"execution_id": "exec_xyz789", "node_id": "agent", "iteration": 1, "tool_calls_this_round": 2, "total_tool_calls": 2}

event: tool_loop_completed
data: {"execution_id": "exec_xyz789", "node_id": "agent", "total_iterations": 3, "total_tool_calls": 5, "finish_reason": "stop"}
```

**Event descriptions:**

| Event | Description |
|---|---|
| `tool_call_started` | Emitted when the engine begins executing a tool call. Includes the tool name, arguments, and current loop iteration. |
| `tool_call_completed` | Emitted when a tool call finishes. Includes a truncated result preview, duration, and whether the tool returned an error. |
| `tool_loop_iteration` | Emitted at the end of each loop iteration (one LLM round-trip). Summarizes tool calls in this round and running total. |
| `tool_loop_completed` | Emitted when the tool loop finishes. Includes total iterations, total tool calls, and the finish reason (`stop`, `max_tool_calls`, or `max_iterations`). |

### Provide Human Input

```
POST /api/v1/executions/{execution_id}/steps/{step_id}/input
```

Request body:

```json
{
  "action": "approve",
  "data": {
    "comment": "Looks good, proceed."
  }
}
```

Returns `409 Conflict` if the step is not waiting for input.

---

## Health and Metadata

### Health Check

```
GET /api/v1/health
```

```json
{
  "status": "healthy",
  "version": "0.1.0",
  "components": {
    "postgresql": "connected",
    "redis": "connected",
    "workers": "available"
  }
}
```

### API Info

```
GET /api/v1/info
```

```json
{
  "name": "brockley",
  "version": "0.1.0",
  "api_version": "v1",
  "features": ["graphs", "schemas", "templates", "executions"]
}
```

---

## Rate Limiting

Default rate limits (configurable):

| Scope | Limit |
|---|---|
| Management API | 100 requests/minute per API key |
| Execution API (invoke) | 50 requests/minute per API key |
| Execution API (read) | 200 requests/minute per API key |
| Streaming | 10 concurrent streams per API key |

Rate limit headers on every response:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 87
X-RateLimit-Reset: 1710432000
```

---

## Versioning

- The API is versioned via URL path (`/api/v1`).
- Breaking changes require a new version (`/api/v2`).
- Non-breaking additions (new optional fields, new endpoints) are added within the current version.
- Deprecated fields are marked in responses with a `_deprecated` suffix.

---

## CORS

The API server supports configurable CORS for browser-based clients (Web UI).

Default allowed origins: `http://localhost:3000` (development). Production deployments should restrict origins to the deployed Web UI domain.

## See Also

- [API Overview](../api/overview.md) -- user-facing API reference
- [Data Model](data-model.md) -- entity fields, relationships, validation rules
- [Architecture](architecture.md) -- system overview, data flow
- [Configuration Reference](../deployment/configuration.md) -- `BROCKLEY_API_KEYS`, CORS, rate limiting

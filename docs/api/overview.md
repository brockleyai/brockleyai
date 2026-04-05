# API Overview

The Brockley REST API is the single gateway for all external access. Every interface (Web UI, Terraform, CLI, MCP server) communicates through it.

## Base URL

```
https://{host}/api/v1
```

Default local development: `http://localhost:8000/api/v1`

## Authentication

When `BROCKLEY_API_KEYS` is configured on the server, all API requests (except health endpoints) must include an API key:

```bash
curl -H "Authorization: Bearer your-api-key" http://localhost:8000/api/v1/graphs
```

Health endpoints (`/health`, `/health/ready`, `/version`, `/metrics`) do not require authentication.

When `BROCKLEY_API_KEYS` is not set, no authentication is required. This is the default for local development.

## Request IDs

Every response includes an `X-Request-Id` header. If the client sends an `X-Request-Id` header, the server uses it. Otherwise, one is generated. Use request IDs when reporting issues or correlating logs.

## Content Type

All request and response bodies use `application/json`.

## Pagination

List endpoints use cursor-based pagination:

```bash
GET /api/v1/graphs?limit=20&cursor=abc123
```

Response:

```json
{
  "items": [...],
  "next_cursor": "def456",
  "has_more": true
}
```

- Default limit: 20
- Maximum limit: 100
- Pass `next_cursor` as the `cursor` parameter to fetch the next page

## Filtering and Sorting

List endpoints accept query parameters for filtering and sorting:

```bash
GET /api/v1/graphs?namespace=production&status=active
GET /api/v1/executions?graph_id=graph_abc123&status=completed
GET /api/v1/graphs?sort_by=created_at&sort_order=desc
```

Default sort: `created_at` descending.

## Error Format

All errors use a consistent structure:

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

### Error Codes

| HTTP Status | Error Code | Description |
|-------------|------------|-------------|
| 400 | `VALIDATION_ERROR` | Request body or parameters are invalid |
| 400 | `SCHEMA_VALIDATION_ERROR` | Data does not match the expected schema |
| 401 | `UNAUTHORIZED` | Missing or invalid API key |
| 403 | `FORBIDDEN` | Insufficient permissions |
| 404 | `NOT_FOUND` | Resource does not exist |
| 409 | `CONFLICT` | Version conflict or duplicate name |
| 422 | `GRAPH_INVALID` | Graph structure is invalid |
| 429 | `RATE_LIMITED` | Too many requests |
| 500 | `INTERNAL_ERROR` | Unexpected server error |
| 503 | `SERVICE_UNAVAILABLE` | Server is not ready (e.g., database not connected) |

## Rate Limiting

Rate limit headers are included on every response:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 87
X-RateLimit-Reset: 1710432000
```

Default limits (configurable):

| Scope | Limit |
|-------|-------|
| Management API | 100 requests/minute per API key |
| Execution API (invoke) | 50 requests/minute per API key |
| Execution API (read) | 200 requests/minute per API key |
| Streaming | 10 concurrent streams per API key |

## API Endpoints

| Section | Endpoints |
|---------|-----------|
| [Graphs](graphs.md) | CRUD, validate, export |
| [Schemas](schemas.md) | CRUD for library schemas |
| [Prompt Templates](prompt-templates.md) | CRUD for library prompt templates |
| [Provider Configs](provider-configs.md) | CRUD for library provider configs |
| [Executions](executions.md) | Invoke, status, cancel, list, steps, streaming |
| [Health](health.md) | Health checks, version, metrics |

## See Also

- [Configuration Reference](../deployment/configuration.md) -- `BROCKLEY_API_KEYS` and other server config
- [API Design Spec](../specs/api-design.md) -- full internal specification
- [CLI Overview](../cli/overview.md) -- command-line access to the same API

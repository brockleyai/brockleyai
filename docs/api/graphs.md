# Graphs API

Manage agent graph definitions. Graphs are self-contained documents with embedded nodes, edges, and state.

## List Graphs

```
GET /api/v1/graphs
```

Query parameters: `namespace`, `status`, `limit`, `cursor`, `sort_by`, `sort_order`

```bash
curl http://localhost:8000/api/v1/graphs?status=active&namespace=production
```

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

## Get Graph

```
GET /api/v1/graphs/{graph_id}
```

Returns the full graph with embedded nodes and edges. The `api_key` field on LLM node configs is masked (e.g., `"sk-or...ab12"` or `"****"` for short keys).

```bash
curl http://localhost:8000/api/v1/graphs/graph_abc123
```

Response: `200 OK` -- full graph object.

## Create Graph

```
POST /api/v1/graphs
```

Request body: full graph definition with nodes, edges, and optional state.

```bash
curl -X POST http://localhost:8000/api/v1/graphs \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-agent",
    "nodes": [
      {
        "id": "input-1",
        "name": "Input",
        "type": "input",
        "input_ports": [],
        "output_ports": [{"name": "text", "schema": {"type": "string"}}],
        "config": {}
      },
      {
        "id": "output-1",
        "name": "Output",
        "type": "output",
        "input_ports": [{"name": "text", "schema": {"type": "string"}}],
        "output_ports": [{"name": "text", "schema": {"type": "string"}}],
        "config": {}
      }
    ],
    "edges": [
      {"id": "e1", "source_node_id": "input-1", "source_port": "text", "target_node_id": "output-1", "target_port": "text"}
    ]
  }'
```

Response: `201 Created` with the full graph object (includes generated `id` and `version: 1`).

## Update Graph

```
PUT /api/v1/graphs/{graph_id}
```

Full replacement of the graph definition. The `version` is auto-incremented.

If a submitted `api_key` matches the masked pattern from GET, the server preserves the original key.

```bash
curl -X PUT http://localhost:8000/api/v1/graphs/graph_abc123 \
  -H "Content-Type: application/json" \
  -d '{ ... updated graph definition ... }'
```

Response: `200 OK` with the updated graph.

## Delete Graph

```
DELETE /api/v1/graphs/{graph_id}
```

Soft-deletes the graph.

Response: `204 No Content`

## Validate Graph

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

If invalid:

```json
{
  "valid": false,
  "errors": [
    {
      "code": "UNWIRED_REQUIRED_PORT",
      "message": "required input port \"text\" on node \"transform-1\" is not wired",
      "node_id": "transform-1"
    }
  ]
}
```

## Export Graph

```
GET /api/v1/graphs/{graph_id}/export?format={format}
```

Supported formats: `json` (default), `terraform`, `yaml`

```bash
curl http://localhost:8000/api/v1/graphs/graph_abc123/export?format=terraform
```

## See Also

- [API Overview](overview.md) -- authentication, pagination, error format
- [Executions API](executions.md) -- invoking graphs
- [Data Model](../specs/data-model.md) -- full field definitions for graphs, nodes, edges
- [Common Errors](../troubleshooting/common-errors.md) -- validation error codes

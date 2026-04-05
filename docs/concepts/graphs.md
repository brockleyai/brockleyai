# Graphs

A **graph** is the fundamental unit in Brockley. It represents a complete, self-contained agent workflow -- a directed graph of [nodes](nodes.md) connected by [edges](edges.md), with optional typed [state](state.md).

## What Makes a Graph

A graph contains:

- **[Nodes](nodes.md)** -- the steps in the workflow (LLM calls, transforms, conditions, etc.)
- **[Edges](edges.md)** -- connections between node [ports](ports-and-typing.md) that define data flow
- **[State](state.md)** (optional) -- typed fields that persist and accumulate across execution
- **Metadata** -- name, description, namespace, version, status

Graphs are self-contained. Everything needed to validate and execute a graph is stored within it. There are no implicit dependencies on external definitions (though nodes may reference external services like LLM providers or MCP servers at runtime).

## Graph Structure (JSON)

```json
{
  "id": "g-abc123",
  "name": "customer-support-agent",
  "description": "Routes customer queries and generates responses",
  "namespace": "production",
  "version": 3,
  "status": "active",
  "nodes": [
    {
      "id": "input-1",
      "name": "Customer Query",
      "type": "input",
      "input_ports": [],
      "output_ports": [
        {
          "name": "query",
          "schema": {"type": "string"}
        },
        {
          "name": "customer_id",
          "schema": {"type": "string"}
        }
      ],
      "config": {}
    }
  ],
  "edges": [
    {
      "id": "edge-1",
      "source_node_id": "input-1",
      "source_port": "query",
      "target_node_id": "classify-1",
      "target_port": "text"
    }
  ],
  "state": {
    "fields": [
      {
        "name": "conversation_history",
        "schema": {
          "type": "array",
          "items": {"type": "string"}
        },
        "reducer": "append",
        "initial": []
      }
    ]
  },
  "metadata": {},
  "created_at": "2026-03-10T12:00:00Z",
  "updated_at": "2026-03-14T09:30:00Z"
}
```

## Namespaces

Every graph belongs to a **namespace**. Namespaces are simple string labels used to organize graphs into logical groups. Common patterns:

- `default` -- the default namespace
- `production`, `staging`, `development` -- environment-based separation
- `team-payments`, `team-support` -- team-based separation

Namespaces are an organizational tool -- they do not provide security isolation on their own. The architecture supports adding access control on top of namespaces via middleware extension points.

## Versioning

Graphs have an integer **version** that increments on each update. When you update a graph via the API, the version number increases automatically.

When you invoke a graph, the execution records the `graph_version` it ran against. This means you can always trace back which version of a graph produced a given result.

## Status Lifecycle

A graph has one of three statuses:

| Status | Meaning |
|--------|---------|
| `draft` | Work in progress. Can be edited. Can be executed for testing. |
| `active` | Ready for production use. Can be executed. |
| `archived` | Soft-deleted. Not shown in default listings. Cannot be executed. |

## Validation

Graphs are validated before execution and can be validated on demand via the API:

```bash
curl -s -X POST http://localhost:8000/api/v1/graphs/GRAPH_ID/validate | jq .
```

### Structural checks

The validator confirms the graph has at least one node and at least one `input` type node. Node IDs must be unique. Every node must be reachable from an input node -- orphaned nodes that can never execute are rejected.

### Port and typing checks

All [port](ports-and-typing.md) schemas must be valid JSON Schema with [strong typing](ports-and-typing.md#strong-typing-rules). No bare `{"type": "object"}` without `properties`, no bare `{"type": "array"}` without `items`. Port names must be unique within a node's input ports and within its output ports. The validator applies these rules recursively to nested schemas.

### Edge wiring checks

Every [edge](edges.md) must reference existing nodes and existing ports on those nodes. Required input ports must be wired -- either by an edge, a [state read](state.md#reading-state), or a default value. Multiple edges targeting the same input port trigger a `MULTI_EDGE_FAN_IN` warning (valid for [exclusive fan-in](branching.md#rejoining-after-branches-exclusive-fan-in) from conditional branches, but worth verifying).

### Cycle and loop checks

Unguarded cycles are not allowed. Any cycle in the graph must pass through a [back-edge](loops.md) that has both a `condition` [expression](expressions.md) and a `max_iterations` limit. This prevents infinite loops while still allowing controlled iteration.

### State checks

[State](state.md) field names must be unique and non-empty. State schemas follow the same strong typing rules as ports. Reducers must be compatible with their field types: `append` requires an array schema, `merge` requires an object schema. All `state_reads` and `state_writes` bindings must reference existing state fields and existing ports.

### Subgraph and ForEach checks

[Subgraph](subgraphs.md) port mappings must be complete and type-compatible. [ForEach](../nodes/foreach.md) inner graphs must have `item` and `index` input ports matching the expected contract.

### Expression checks

[Expressions](expressions.md) in templates, conditions, and transforms are parsed and checked for syntax errors during validation.

See [Troubleshooting: Common Errors](../troubleshooting/common-errors.md) for details on validation error codes.

## CRUD Operations

| Operation | Method | Endpoint |
|-----------|--------|----------|
| Create | `POST` | `/api/v1/graphs` |
| List | `GET` | `/api/v1/graphs` |
| Get | `GET` | `/api/v1/graphs/{id}` |
| Update | `PUT` | `/api/v1/graphs/{id}` |
| Delete | `DELETE` | `/api/v1/graphs/{id}` |
| Validate | `POST` | `/api/v1/graphs/{id}/validate` |

### Create Example

```bash
curl -s -X POST http://localhost:8000/api/v1/graphs \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-graph",
    "namespace": "default",
    "nodes": [...],
    "edges": [...]
  }' | jq .
```

### List with Filtering

```bash
# List all graphs in a namespace
curl -s "http://localhost:8000/api/v1/graphs?namespace=production" | jq .

# List with pagination
curl -s "http://localhost:8000/api/v1/graphs?limit=10&offset=20" | jq .
```

## Self-Contained Design

A key design principle: graphs are self-contained. The graph definition includes everything needed to validate it structurally. This means:

- Graphs can be exported, imported, and version-controlled as single JSON or YAML files
- Validation runs without database access or external service calls
- Graphs can be shared, reviewed, and tested independently
- The CLI can validate graphs locally without a server

Runtime dependencies (LLM API keys, MCP server URLs) are resolved at execution time, not at definition time. The graph references them by name (e.g., `api_key_ref: "OPENAI_API_KEY"`), and the execution environment resolves them.

## See Also

- [Nodes](nodes.md) -- the building blocks inside a graph
- [Edges](edges.md) -- how nodes are wired together
- [Ports and Typing](ports-and-typing.md) -- how data types work
- [State](state.md) -- persistent fields across execution
- [Expressions](expressions.md) -- the expression language used in templates, conditions, and transforms
- [Loops](loops.md) -- controlled iteration via back-edges
- [Execution](execution.md) -- how graphs run

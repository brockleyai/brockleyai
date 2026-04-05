# Common Errors

This page covers the most common errors you will encounter when building, validating, and executing graphs in Brockley, along with how to fix them.

## Validation Errors

Validation errors are returned when you create/update a graph or call the validate endpoint. They indicate structural or typing problems that must be fixed before the graph can execute.

### EMPTY_GRAPH

```json
{"code": "EMPTY_GRAPH", "message": "graph must have at least one node"}
```

**Cause**: The graph has no nodes.

**Fix**: Add at least one node to the graph. Every graph needs at minimum an `input` node.

---

### NO_INPUT_NODE

```json
{"code": "NO_INPUT_NODE", "message": "graph must have at least one input node"}
```

**Cause**: The graph has nodes but none of them is type `input`.

**Fix**: Add a node with `"type": "input"`. This defines the graph's external interface.

---

### MISSING_PORT_SCHEMA

```json
{"code": "MISSING_PORT_SCHEMA", "message": "port \"query\" must have a schema", "node_id": "llm-1"}
```

**Cause**: A port is defined without a `schema` field.

**Fix**: Add a JSON Schema to the port:

```json
{"name": "query", "schema": {"type": "string"}}
```

---

### SCHEMA_VIOLATION (bare object)

```json
{"code": "SCHEMA_VIOLATION", "message": "object schema must have 'properties' (bare {\"type\":\"object\"} not allowed)"}
```

**Cause**: A port or state field uses `{"type": "object"}` without specifying `properties`.

**Fix**: Define the object's properties:

```json
{
  "type": "object",
  "properties": {
    "name": {"type": "string"},
    "value": {"type": "number"}
  }
}
```

---

### SCHEMA_VIOLATION (bare array)

```json
{"code": "SCHEMA_VIOLATION", "message": "array schema must have 'items' (bare {\"type\":\"array\"} not allowed)"}
```

**Cause**: A port or state field uses `{"type": "array"}` without specifying `items`.

**Fix**: Define the array's item type:

```json
{
  "type": "array",
  "items": {"type": "string"}
}
```

---

### MISSING_TYPE

```json
{"code": "MISSING_TYPE", "message": "schema must have a 'type' field"}
```

**Cause**: A schema has no `type` field and no `oneOf`/`anyOf`/`enum`.

**Fix**: Add a `type` field:

```json
{"type": "string"}
```

Or use a union type:

```json
{"oneOf": [{"type": "string"}, {"type": "integer"}]}
```

---

### UNWIRED_REQUIRED_PORT

```json
{"code": "UNWIRED_REQUIRED_PORT", "message": "required input port \"text\" on node \"transform-1\" is not wired (no edge, state read, or default)"}
```

**Cause**: A required input port has no data source -- no edge connects to it, no state read feeds it, and it has no default value.

**Fix** (pick one):
1. Add an edge from an upstream node's output port to this input port
2. Add a state read binding: `"state_reads": [{"state_field": "my_field", "port": "text"}]`
3. Add a default value: `{"name": "text", "schema": {"type": "string"}, "default": ""}`
4. Make it optional: `{"name": "text", "schema": {"type": "string"}, "required": false}`

---

### INVALID_SOURCE_PORT / INVALID_TARGET_PORT

```json
{"code": "INVALID_SOURCE_PORT", "message": "source port \"result\" does not exist on node \"input-1\""}
```

**Cause**: An edge references a port name that does not exist on the specified node.

**Fix**: Check the port name in your edge definition against the node's actual `output_ports` (for source) or `input_ports` (for target). Port names are case-sensitive.

---

### UNGUARDED_CYCLE

```json
{"code": "UNGUARDED_CYCLE", "message": "graph contains a cycle not guarded by a back-edge (nodes: A → B → C → A)"}
```

**Cause**: The graph has a cycle in its edges, but none of the edges in the cycle are marked as `back_edge: true`.

**Fix**: Identify which edge should be the back-edge (the one that loops back) and mark it:

```json
{
  "id": "loop-edge",
  "source_node_id": "C",
  "source_port": "output",
  "target_node_id": "A",
  "target_port": "input",
  "back_edge": true,
  "condition": "state.count < 10",
  "max_iterations": 10
}
```

---

### BACKEDGE_NO_CONDITION

```json
{"code": "BACKEDGE_NO_CONDITION", "message": "back-edge must have a condition expression"}
```

**Cause**: An edge is marked `back_edge: true` but has no `condition` field.

**Fix**: Add a condition expression that determines whether to continue looping:

```json
{"condition": "state.iteration_count < 5"}
```

---

### BACKEDGE_NO_MAX_ITERATIONS

```json
{"code": "BACKEDGE_NO_MAX_ITERATIONS", "message": "back-edge must have max_iterations > 0"}
```

**Cause**: A back-edge has no `max_iterations` or it is set to 0 or negative.

**Fix**: Set a positive `max_iterations` to prevent infinite loops:

```json
{"max_iterations": 10}
```

---

### REDUCER_INCOMPATIBLE

```json
{"code": "REDUCER_INCOMPATIBLE", "message": "state field \"items\" uses 'append' reducer but schema type is \"string\" (must be 'array')"}
```

**Cause**: A state field's reducer does not match its schema type.

**Fix**: Match the reducer to the schema type:

| Reducer | Required Schema Type |
|---------|---------------------|
| `replace` | Any |
| `append` | `array` |
| `merge` | `object` |

---

### DUPLICATE_NODE_ID

```json
{"code": "DUPLICATE_NODE_ID", "message": "duplicate node ID: node-1"}
```

**Cause**: Two nodes have the same `id`.

**Fix**: Give each node a unique ID.

---

### DUPLICATE_PORT_NAME

```json
{"code": "DUPLICATE_PORT_NAME", "message": "duplicate input port name: text"}
```

**Cause**: A node has two input ports (or two output ports) with the same name.

**Fix**: Give each port within the same direction (input or output) a unique name.

## Validation Warnings

Warnings do not prevent execution but indicate potential issues.

### MULTI_EDGE_FAN_IN

```json
{"code": "MULTI_EDGE_FAN_IN", "message": "port node-5.text has 2 incoming edges -- ensure they are from mutually exclusive conditional branches"}
```

**Cause**: Multiple edges target the same input port. This is valid only if the edges come from mutually exclusive conditional branches (exclusive fan-in).

**Action**: Verify that the incoming edges are from branches of a conditional node, so only one edge will carry data at runtime.

### UNREACHABLE_NODE

```json
{"code": "UNREACHABLE_NODE", "message": "node \"orphan-1\" is not reachable from any input node"}
```

**Cause**: A node exists in the graph but no path of edges connects it to any input node.

**Action**: Either wire the node into the graph or remove it.

## Execution Errors

### Provider Errors

```json
{
  "error": {
    "code": "PROVIDER_ERROR",
    "message": "OpenAI API returned 429: rate limit exceeded",
    "node_id": "llm-1"
  }
}
```

**Common causes**:
- **429 Rate Limit**: you are sending too many requests. Add a retry policy with exponential backoff.
- **401 Unauthorized**: the `api_key_ref` references a secret that is missing, empty, or invalid.
- **500 Server Error**: the LLM provider is experiencing issues. Retry later.

**Fix**: Check the `api_key_ref` value, verify the secret exists in the environment, and consider adding a retry policy:

```json
{
  "retry_policy": {
    "max_retries": 3,
    "backoff_strategy": "exponential",
    "initial_delay_seconds": 1.0,
    "max_delay_seconds": 30.0
  }
}
```

### Timeout Errors

```json
{
  "error": {
    "code": "TIMEOUT",
    "message": "node exceeded timeout of 30 seconds",
    "node_id": "llm-1"
  }
}
```

**Cause**: A node took longer than its `timeout_seconds` to execute.

**Fix**: Increase the timeout, or investigate why the node is slow (large prompt, slow provider, slow MCP server).

### Schema Validation (Structured Output)

```json
{
  "error": {
    "code": "OUTPUT_VALIDATION_FAILED",
    "message": "LLM output does not match output_schema",
    "node_id": "llm-1"
  }
}
```

**Cause**: An LLM node with `response_format: "json"` and `validate_output: true` received a response that does not match the `output_schema`.

**Fix**: Improve the prompt to guide the LLM toward the correct schema, or relax the schema. Some models produce better structured output than others.

## Deployment Errors

### Database Connection Failed

```
FATAL: could not connect to database: connection refused
```

**Cause**: PostgreSQL is not running, not reachable, or the connection string is wrong.

**Fix**: Verify `BROCKLEY_DATABASE_URL` is correct and the database is accessible from the server.

### Redis Connection Failed

```
ERROR: could not connect to Redis: dial tcp: connection refused
```

**Cause**: Redis is not running or not reachable.

**Fix**: Verify `BROCKLEY_REDIS_URL` is correct. Redis is required for async execution and streaming. Without Redis, only the health endpoint works.

### Task Queue Not Configured

```json
{"error": {"code": "SERVICE_UNAVAILABLE", "message": "task queue not configured (REDIS_URL required)"}}
```

**Cause**: Attempting to invoke a graph execution but Redis is not configured.

**Fix**: Set `BROCKLEY_REDIS_URL` (or `REDIS_URL`) to a valid Redis connection string.

## Superagent Validation Errors

### SUPERAGENT_MISSING_CONFIG

```json
{"code": "SUPERAGENT_MISSING_CONFIG", "message": "superagent node missing required config: prompt, skills, provider, model"}
```

**Cause**: A superagent node is missing one or more required config fields.

**Fix**: Ensure the node config includes `prompt`, `skills` (non-empty array), `provider`, and `model`. Also provide `api_key` or `api_key_ref`.

### SUPERAGENT_INVALID_SKILL

```json
{"code": "SUPERAGENT_INVALID_SKILL", "message": "skill 'search' missing required field: mcp_url"}
```

**Cause**: A skill in the superagent config is missing `name`, `description`, or `mcp_url`.

**Fix**: Each skill must have all three fields. For API tool skills, use `api_tool_id` instead of `mcp_url`.

### SUPERAGENT_NO_OUTPUT

```json
{"code": "SUPERAGENT_NO_OUTPUT", "message": "superagent node must have at least one output port"}
```

**Cause**: The superagent node has no output ports defined.

**Fix**: Add at least one output port to the node.

### SUPERAGENT_INVALID_OVERRIDE

```json
{"code": "SUPERAGENT_INVALID_OVERRIDE", "message": "override evaluator specifies provider but not model"}
```

**Cause**: An override specifies a provider but no model, or has invalid numeric values.

**Fix**: If you specify a `provider` on an override, also specify `model`. Ensure numeric limits like `window_size` are > 0 and `compaction_threshold` is in range (0.0, 1.0].

## See Also

- [FAQ](faq.md) -- frequently asked questions
- [Graphs](../concepts/graphs.md) -- graph structure and validation rules
- [Ports and Typing](../concepts/ports-and-typing.md) -- port schemas and strong typing rules
- [Loops](../concepts/loops.md) -- back-edge conditions and max_iterations
- [Configuration Reference](../deployment/configuration.md) -- environment variables
- [Execution Model](../concepts/execution.md) -- how execution works
- [Graph Model](../specs/graph-model.md) -- validation rules
- [Data Model](../specs/data-model.md) -- entity fields and constraints

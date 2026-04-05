# Ports and Typing

Ports are the typed connection points on nodes. Every piece of data that flows between nodes passes through a port. Brockley uses JSON Schema to define port types and enforces strong typing rules at validation time.

## What is a Port?

A port has three fields:

```json
{
  "name": "query",
  "schema": {"type": "string"},
  "required": true
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | (required) | Unique identifier within the node's input or output ports |
| `schema` | object | (required) | JSON Schema defining the data type |
| `required` | boolean | `true` for input ports | Whether the port must be wired |
| `default` | any | (none) | Default value if the port is not wired |

## Input Ports vs Output Ports

- **Input ports** receive data from edges, state reads, or default values
- **Output ports** produce data that flows to downstream nodes via edges or writes to state

Every edge connects one output port to one input port:

```json
{
  "id": "edge-1",
  "source_node_id": "node-a",
  "source_port": "result",
  "target_node_id": "node-b",
  "target_port": "text"
}
```

## JSON Schema Types

Ports use standard JSON Schema. The supported primitive types are:

| Type | Example Schema | Example Value |
|------|---------------|---------------|
| `string` | `{"type": "string"}` | `"hello"` |
| `integer` | `{"type": "integer"}` | `42` |
| `number` | `{"type": "number"}` | `3.14` |
| `boolean` | `{"type": "boolean"}` | `true` |

### Object Types

Object schemas must include `properties`:

```json
{
  "type": "object",
  "properties": {
    "name": {"type": "string"},
    "age": {"type": "integer"},
    "email": {"type": "string"}
  },
  "required": ["name", "age"]
}
```

### Array Types

Array schemas must include `items`:

```json
{
  "type": "array",
  "items": {
    "type": "object",
    "properties": {
      "title": {"type": "string"},
      "score": {"type": "number"}
    }
  }
}
```

### Union Types

Use `oneOf` or `anyOf` for union types:

```json
{
  "oneOf": [
    {"type": "string"},
    {"type": "integer"}
  ]
}
```

### Enum Types

```json
{
  "enum": ["low", "medium", "high"]}
```

## Why Strong Typing?

Brockley rejects vague schemas on purpose. Three reasons:

**Catch errors before execution.** A bare `{"type": "object"}` tells the validator nothing. If an upstream node produces `{"name": "Alice"}` and a downstream node expects `{"title": "..."}`, a weakly-typed system discovers the mismatch at runtime -- possibly after spending money on LLM calls. Strong typing catches it during validation, before any node runs.

**Self-documenting graphs.** When every port declares its exact structure, the graph definition doubles as API documentation. Anyone reading the graph JSON can see what data flows where, without tracing through execution logs. The web UI also uses port schemas to render type-aware connection hints.

**Better LLM structured output.** When an LLM node has `response_format: "json"`, Brockley sends the output port's schema to the LLM provider as the expected JSON structure. A detailed schema with properties, types, and required fields produces much more reliable structured output than a bare object type. The schema becomes the contract between your graph and the LLM.

## Strong Typing Rules

Brockley enforces strong typing to catch errors before execution. These rules are checked during validation:

### Rule 1: Every Port Must Have a Schema

Ports without a `schema` field will fail validation with `MISSING_PORT_SCHEMA`.

### Rule 2: No Bare Object Schemas

This is **not allowed**:

```json
{"type": "object"}
```

You must specify `properties` (or use `oneOf`/`anyOf`):

```json
{
  "type": "object",
  "properties": {
    "key": {"type": "string"}
  }
}
```

**Error code**: `SCHEMA_VIOLATION` -- "object schema must have 'properties'"

### Rule 3: No Bare Array Schemas

This is **not allowed**:

```json
{"type": "array"}
```

You must specify `items`:

```json
{
  "type": "array",
  "items": {"type": "string"}
}
```

**Error code**: `SCHEMA_VIOLATION` -- "array schema must have 'items'"

### Rule 4: Every Schema Must Have a Type

This is **not allowed**:

```json
{}
```

Every schema needs a `type` field (or `oneOf`/`anyOf`/`enum` at the top level).

**Error code**: `MISSING_TYPE` -- "schema must have a 'type' field"

### Rule 5: Recursive Validation

Object property schemas and array item schemas are validated recursively. A bare `{"type": "object"}` nested inside a property is also rejected.

## Required Ports and Defaults

Input ports are **required by default**. If a required input port is not wired (no edge, no state read, no default value), validation fails with `UNWIRED_REQUIRED_PORT`.

You can make a port optional:

```json
{
  "name": "context",
  "schema": {"type": "string"},
  "required": false
}
```

Or provide a default value:

```json
{
  "name": "language",
  "schema": {"type": "string"},
  "default": "en"
}
```

A port with a `default` value does not need to be wired. If no data arrives through an edge or state read, the default is used.

## Port Resolution Priority

When a node executes, each input port's value is resolved in this order:

1. **Edge data** -- data arriving from an upstream node's output port
2. **State read** -- data read from a graph state field (via `state_reads`)
3. **Default value** -- the port's `default` field
4. **(missing)** -- if the port is optional and none of the above provide data, the port has no value

If both an edge and a state read target the same port, the edge takes priority.

## Unique Port Names

Within a single node, all input port names must be unique, and all output port names must be unique. Duplicate names cause a `DUPLICATE_PORT_NAME` validation error.

Input and output ports can share names (e.g., a transform node might have an input port `text` and an output port `text`).

## Examples

### Simple String In, String Out

```json
{
  "input_ports": [
    {"name": "text", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {"name": "result", "schema": {"type": "string"}}
  ]
}
```

### Structured Object with Optional Field

```json
{
  "input_ports": [
    {
      "name": "request",
      "schema": {
        "type": "object",
        "properties": {
          "query": {"type": "string"},
          "max_results": {"type": "integer"}
        },
        "required": ["query"]
      }
    },
    {
      "name": "language",
      "schema": {"type": "string"},
      "required": false,
      "default": "en"
    }
  ]
}
```

### Array of Objects

```json
{
  "output_ports": [
    {
      "name": "results",
      "schema": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "id": {"type": "string"},
            "title": {"type": "string"},
            "relevance": {"type": "number"}
          }
        }
      }
    }
  ]
}
```

## See Also

- [Edges](edges.md) -- how ports are connected via edges
- [State](state.md) -- how state fields interact with ports
- [Nodes](nodes.md) -- node types and their port conventions
- [Expressions](expressions.md) -- the expression language that reads from `input.*` ports
- [Branching](branching.md) -- how conditional nodes use ports for routing

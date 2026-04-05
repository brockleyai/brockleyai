# Input and Output Nodes

Input and output nodes define the external contract of a graph. Every graph must have at least one input node (entry point) and at least one output node (exit point). They act as the boundary between the outside world and the internal workflow.

## Input Node

**Type:** `input`

The input node is the entry point of every graph execution. When a graph is triggered (via API, CLI, UI, Terraform, MCP, or schedule), the caller provides a JSON payload. The input node receives that payload and makes it available to downstream nodes through its output ports.

The input node has no configuration. Its behavior is defined entirely by its **output ports**, which declare the shape of data the graph expects from callers.

### How It Works

1. The caller provides an input payload (e.g., `{"query": "hello", "user_id": "u-123"}`).
2. The orchestrator maps each top-level key in the payload to the matching output port on the input node.
3. Downstream nodes connected via edges receive these values on their input ports.

The input node is a passthrough: it copies every input it receives directly to its outputs, unchanged.

### Example

```json
{
  "id": "input-1",
  "name": "Graph Input",
  "type": "input",
  "input_ports": [],
  "output_ports": [
    {
      "name": "query",
      "schema": {"type": "string"},
      "required": true
    },
    {
      "name": "user_id",
      "schema": {"type": "string"},
      "required": true
    },
    {
      "name": "options",
      "schema": {"type": "object"},
      "required": false,
      "default": {}
    }
  ],
  "config": {}
}
```

When executed with:

```json
{
  "query": "What is the weather?",
  "user_id": "u-42"
}
```

The input node produces outputs `query = "What is the weather?"` and `user_id = "u-42"`. The `options` port receives its default value `{}` because the caller did not provide it.

## Output Node

**Type:** `output`

The output node is the terminal node of a graph. It collects results from upstream nodes and returns them as the execution result. Like the input node, it is a passthrough with no configuration.

The output node's behavior is defined by its **input ports**, which declare the shape of data the graph produces.

### How It Works

1. Upstream nodes send data to the output node's input ports via edges.
2. The output node passes all received inputs through as the execution output.
3. The orchestrator collects these values and returns them in the API response.

### Example

```json
{
  "id": "output-1",
  "name": "Graph Output",
  "type": "output",
  "input_ports": [
    {
      "name": "result",
      "schema": {"type": "string"},
      "required": true
    },
    {
      "name": "metadata",
      "schema": {"type": "object"},
      "required": false
    }
  ],
  "output_ports": [],
  "config": {}
}
```

If the execution produces `result = "Sunny, 22C"` and `metadata = {"source": "weather-api"}`, the API response contains:

```json
{
  "output": {
    "result": "Sunny, 22C",
    "metadata": {"source": "weather-api"}
  }
}
```

## Port Definitions

Both input and output nodes use the standard Port structure:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Port identifier. Must be unique within the node. |
| `schema` | JSON Schema | Yes | Validates the data flowing through the port. Must follow [strong typing rules](../specs/data-model.md). |
| `required` | boolean | No | Defaults to `true` for input ports. If `false`, the port may receive no value. |
| `default` | any | No | Default value used when no data arrives on this port. Must validate against the schema. |

### Schema Examples

String port:

```json
{"name": "query", "schema": {"type": "string"}}
```

Integer port with default:

```json
{"name": "limit", "schema": {"type": "integer"}, "required": false, "default": 10}
```

Array of objects:

```json
{
  "name": "items",
  "schema": {
    "type": "array",
    "items": {
      "type": "object",
      "properties": {
        "id": {"type": "string"},
        "value": {"type": "number"}
      },
      "required": ["id", "value"]
    }
  }
}
```

Typed object:

```json
{
  "name": "config",
  "schema": {
    "type": "object",
    "properties": {
      "mode": {"type": "string", "enum": ["fast", "thorough"]},
      "max_results": {"type": "integer"}
    },
    "required": ["mode"]
  }
}
```

## Complete Graph Example

A minimal graph with input and output nodes connected directly:

```json
{
  "name": "echo",
  "nodes": [
    {
      "id": "in",
      "name": "Input",
      "type": "input",
      "input_ports": [],
      "output_ports": [
        {"name": "message", "schema": {"type": "string"}}
      ],
      "config": {}
    },
    {
      "id": "out",
      "name": "Output",
      "type": "output",
      "input_ports": [
        {"name": "message", "schema": {"type": "string"}}
      ],
      "output_ports": [],
      "config": {}
    }
  ],
  "edges": [
    {
      "id": "e1",
      "source_node_id": "in",
      "source_port": "message",
      "target_node_id": "out",
      "target_port": "message"
    }
  ]
}
```

Executing with `{"message": "hello"}` returns `{"message": "hello"}`.

## See Also

- [Data Model: Node](../specs/data-model.md) -- port and schema definitions
- [Data Model: Strong Typing Rules](../specs/data-model.md) -- schema requirements
- [ForEach Node](foreach.md) -- inner graph input/output contract
- [Subgraph Node](subgraph.md) -- inner graph port mapping
- [Graph Model](../specs/graph-model.md) -- graph structure and execution flow

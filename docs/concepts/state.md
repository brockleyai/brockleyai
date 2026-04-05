# Graph State

Graph state provides persistent, typed fields that accumulate values across execution. While edges carry data between individual nodes, state lets you maintain information that spans the entire graph -- conversation history, running totals, collected results, and shared context.

## When to Use State vs Edges

| Use Case | Use Edges | Use State |
|----------|-----------|-----------|
| Pass data from node A to node B | Yes | No |
| Accumulate results across a loop | No | Yes (append reducer) |
| Build up a conversation history | No | Yes (append reducer) |
| Share context across many nodes | Possible but verbose | Yes (replace reducer) |
| Merge partial results from branches | No | Yes (merge reducer) |

**Rule of thumb**: if data flows from one specific node to another, use edges. If data needs to persist, accumulate, or be accessed by multiple unrelated nodes, use state.

## Defining State

State is defined at the graph level as an array of fields:

```json
{
  "state": {
    "fields": [
      {
        "name": "messages",
        "schema": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "role": {"type": "string"},
              "content": {"type": "string"}
            }
          }
        },
        "reducer": "append",
        "initial": []
      },
      {
        "name": "total_tokens",
        "schema": {"type": "integer"},
        "reducer": "replace",
        "initial": 0
      },
      {
        "name": "metadata",
        "schema": {
          "type": "object",
          "properties": {
            "model": {"type": "string"},
            "run_count": {"type": "integer"}
          }
        },
        "reducer": "merge",
        "initial": {}
      }
    ]
  }
}
```

### Field Properties

| Property | Type | Description |
|----------|------|-------------|
| `name` | string | Unique identifier for the state field |
| `schema` | object | JSON Schema defining the field's type (same strong typing rules as ports) |
| `reducer` | string | How writes accumulate: `replace`, `append`, or `merge` |
| `initial` | any | Starting value before any writes (optional) |

## Reducers

Reducers define what happens when a node writes to a state field.

### replace

The new value completely replaces the old value. Works with any type.

```
State before: "hello"
Write value:  "world"
State after:  "world"
```

### append

The write value is appended to an array. The state field's schema must be `type: "array"`.

```
State before: ["a", "b"]
Write value:  "c"
State after:  ["a", "b", "c"]
```

If the write value is itself an array, each element is appended individually:

```
State before: [1, 2]
Write value:  [3, 4]
State after:  [1, 2, 3, 4]
```

### merge

The write value is shallow-merged into an object. The state field's schema must be `type: "object"`.

```
State before: {"a": 1, "b": 2}
Write value:  {"b": 3, "c": 4}
State after:  {"a": 1, "b": 3, "c": 4}
```

## Reducer Compatibility

The validator checks that reducers are compatible with their field schemas:

| Reducer | Required Schema Type |
|---------|---------------------|
| `replace` | Any type |
| `append` | `type: "array"` (with `items`) |
| `merge` | `type: "object"` (with `properties`) |

Mismatches produce a `REDUCER_INCOMPATIBLE` validation error.

## Reading State

Nodes read state fields through **state read bindings**. A state read maps a state field to an input port on the node:

```json
{
  "id": "llm-1",
  "name": "Generate Response",
  "type": "llm",
  "input_ports": [
    {"name": "query", "schema": {"type": "string"}},
    {"name": "history", "schema": {"type": "array", "items": {"type": "string"}}}
  ],
  "state_reads": [
    {"state_field": "messages", "port": "history"}
  ]
}
```

When this node executes, the current value of the `messages` state field is injected into the `history` input port.

**Important**: if both an edge and a state read target the same input port, the edge value takes priority.

## Writing State

Nodes write to state fields through **state write bindings**. A state write maps an output port to a state field:

```json
{
  "id": "llm-1",
  "name": "Generate Response",
  "type": "llm",
  "output_ports": [
    {"name": "response_text", "schema": {"type": "string"}},
    {"name": "new_message", "schema": {"type": "string"}}
  ],
  "state_writes": [
    {"state_field": "messages", "port": "new_message"}
  ]
}
```

After this node executes, the value of the `new_message` output port is written to the `messages` state field using that field's reducer.

## Direct State Access in Expressions

In addition to `state_reads` bindings, state fields are directly accessible in all expression contexts via the `state.*` namespace. This means any node that evaluates expressions -- LLM prompt templates, transform expressions, conditional branch conditions, and back-edge conditions -- can reference state fields without declaring explicit `state_reads` bindings.

For example, an LLM node can reference state directly in its prompt template:

```
Conversation so far:
{{state.conversation_history}}

Now respond to the user's latest message: {{input.query}}
```

A transform node can combine state with input data:

```json
{
  "expressions": {
    "total": "state.running_total + input.new_value"
  }
}
```

A conditional branch can check state:

```json
{
  "condition": "input.priority == 'high' && state.attempt_count < 3"
}
```

### When to Use `state_reads` vs Direct Access

`state_reads` bindings are still useful when you want to map a state field to a specific input port name. This is helpful when:

- You want to rename the state field for clarity within the node (e.g., map `messages` state field to a `history` input port)
- You want the state value to participate in port-level schema validation
- You want to make the state dependency explicit in the graph structure

For simple access to state values in expressions, direct `state.*` references are more concise and require no additional configuration.

## State in Superagent Nodes

[Superagent](superagent.md) nodes interact with graph state through shared memory. When `shared_memory.enabled: true`, the superagent reads prior memory entries from a graph state field (`_superagent_memory`) at startup and writes new entries back at completion. This allows multiple superagent nodes in a pipeline to share persistent facts.

The state field must use the `merge` reducer:

```json
{
  "state": {
    "fields": [
      {"name": "_superagent_memory", "schema": {"type": "object"}, "reducer": "merge", "initial": {}}
    ]
  }
}
```

Each superagent node declares `state_reads` and `state_writes` bindings for this field. Memory entries survive context compaction within the agent -- they are flushed to state before the conversation is summarized. See [Superagent Built-In Tools](superagent-tools.md#shared-memory) for configuration details.

## State in Loops

State is especially useful in [loops](loops.md) (back-edges). Each iteration of a loop can append to or update state, building up results across iterations. See the [Loops](loops.md) concept page for back-edge conditions, max_iterations, and state accumulation patterns.

Example: a loop that collects items until a condition is met:

```json
{
  "state": {
    "fields": [
      {
        "name": "collected_items",
        "schema": {
          "type": "array",
          "items": {"type": "string"}
        },
        "reducer": "append",
        "initial": []
      },
      {
        "name": "iteration_count",
        "schema": {"type": "integer"},
        "reducer": "replace",
        "initial": 0
      }
    ]
  }
}
```

Each iteration, the LLM node appends a new item to `collected_items` and increments `iteration_count`. The back-edge condition checks whether enough items have been collected:

```json
{
  "back_edge": true,
  "condition": "state.iteration_count < 5",
  "max_iterations": 10
}
```

## Validation Rules

State validation checks:

| Rule | Error Code |
|------|-----------|
| Field names must be unique | `DUPLICATE_STATE_FIELD` |
| Field names must not be empty | `EMPTY_STATE_FIELD` |
| Field schemas follow strong typing rules | `SCHEMA_VIOLATION`, `MISSING_TYPE` |
| Reducer is compatible with schema type | `REDUCER_INCOMPATIBLE` |
| State reads reference existing fields | `INVALID_STATE_REF` |
| State reads map to existing input ports | `INVALID_STATE_PORT` |
| State writes reference existing fields | `INVALID_STATE_REF` |
| State writes map to existing output ports | `INVALID_STATE_PORT` |

## Complete Example

A graph with state that tracks a conversation:

```json
{
  "name": "conversational-agent",
  "state": {
    "fields": [
      {
        "name": "history",
        "schema": {
          "type": "array",
          "items": {"type": "string"}
        },
        "reducer": "append",
        "initial": []
      }
    ]
  },
  "nodes": [
    {
      "id": "input-1",
      "type": "input",
      "output_ports": [
        {"name": "user_message", "schema": {"type": "string"}}
      ]
    },
    {
      "id": "llm-1",
      "type": "llm",
      "input_ports": [
        {"name": "query", "schema": {"type": "string"}},
        {"name": "context", "schema": {"type": "array", "items": {"type": "string"}}}
      ],
      "output_ports": [
        {"name": "response_text", "schema": {"type": "string"}}
      ],
      "state_reads": [
        {"state_field": "history", "port": "context"}
      ],
      "state_writes": [
        {"state_field": "history", "port": "response_text"}
      ]
    },
    {
      "id": "output-1",
      "type": "output",
      "input_ports": [
        {"name": "response", "schema": {"type": "string"}}
      ]
    }
  ],
  "edges": [
    {"id": "e1", "source_node_id": "input-1", "source_port": "user_message", "target_node_id": "llm-1", "target_port": "query"},
    {"id": "e2", "source_node_id": "llm-1", "source_port": "response_text", "target_node_id": "output-1", "target_port": "response"}
  ]
}
```

## Managing State in the Web UI

The web UI provides a visual interface for managing state fields without editing JSON directly.

### Defining State Fields

State fields are managed from the left sidebar and the properties panel:

1. **Left sidebar**: The "STATE FIELDS" collapsible section shows all defined fields as compact cards. Click "+ Add State Field" to create a new field.
2. **Properties panel**: Click a state field card to open the full editor. Configure the field name, schema type, reducer, and initial value.
3. **No-selection view**: When nothing is selected on the canvas, the properties panel shows a graph overview including all state fields.

### Configuring State Bindings on Nodes

When a node is selected and the graph has state fields, a collapsible "State" section appears in the properties panel:

- **State Reads**: Map state fields to input ports. The state value is injected into the port before the node executes.
- **State Writes**: Map output ports to state fields. After execution, the port value is written to state using the field's reducer.

### State in Conditional Nodes

Conditional branch conditions do not directly receive the `state` namespace. To use state values in conditions, add a state read binding that maps the state field to an input port, then reference it as `input.port_name` in the condition expression. The UI displays a hint about this pattern when editing conditional nodes.

### Rename and Delete Propagation

- **Renaming** a state field automatically updates all `state_reads` and `state_writes` bindings across all nodes.
- **Deleting** a state field removes all bindings that reference it.

### Variable Browser

Expression-capable nodes (LLM, Transform, Conditional) show a Variable Browser that displays available `state.*` variables as blue pills. These can be used directly in expressions without explicit `state_reads` bindings.

## See Also

- [Execution](execution.md) -- how state updates are applied during execution
- [Loops](loops.md) -- state accumulation across loop iterations
- [Branching](branching.md) -- how state works with conditional branches
- [Ports and Typing](ports-and-typing.md) -- typing rules for state schemas
- [Expressions](expressions.md) -- accessing state via the `state.*` namespace
- [Superagent](superagent.md) -- shared memory across agent nodes

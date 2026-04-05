# Subgraphs

Subgraphs let you embed a complete graph inside a single node. The inner graph has its own nodes, edges, and state -- all hidden behind a clean port interface. This is how you build reusable workflow components, break large graphs into manageable pieces, and isolate state.

Two node types use subgraphs: the [**subgraph** node](../nodes/subgraph.md) (runs the inner graph once) and the [**foreach** node](../nodes/foreach.md) (runs the inner graph once per item in an array).

## Subgraph Nodes

A subgraph node wraps an inline graph definition and maps the outer node's [ports](ports-and-typing.md) to the inner graph's input and output nodes.

```json
{
  "id": "classify-block",
  "type": "subgraph",
  "input_ports": [
    {"name": "text", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {"name": "result", "schema": {"type": "string"}}
  ],
  "config": {
    "port_mapping": {
      "inputs": {"text": "text"},
      "outputs": {"classified_text": "result"}
    },
    "graph": {
      "nodes": [ ... ],
      "edges": [ ... ]
    }
  }
}
```

### Port Mapping

The `port_mapping` config connects the outer node's ports to the inner graph:

- **`inputs`**: Maps outer input port names to inner graph input port names. `{"text": "text"}` means the outer `text` port feeds into the inner graph's `text` input.
- **`outputs`**: Maps inner graph output port names to outer output port names. `{"classified_text": "result"}` means the inner graph's `classified_text` output becomes the outer node's `result` output.

You can optionally prefix inner port names with the node ID for clarity: `{"text": "inner-input.text"}`. The `nodeID.` prefix is stripped when resolving -- it just makes the mapping more explicit when the inner graph has multiple nodes.

### How Execution Works

1. Data arrives on the subgraph node's input ports.
2. Port mapping routes the input values into the inner graph's input node.
3. The inner graph executes as a standalone graph (same orchestrator, same execution model).
4. When the inner graph completes, port mapping routes the output values back to the outer node's output ports.

## ForEach Bodies

The [foreach](../nodes/foreach.md) node also uses an inline subgraph, but with a fixed contract instead of a configurable port mapping. The inner graph runs once per item in the input array.

The inner graph's input node must provide these ports:

| Port | Type | Description |
|------|------|-------------|
| `item` | (matches array element type) | The current element from the `items` array |
| `index` | integer | Zero-based position in the array |
| `context` | (matches `context` port type) | Shared data passed to every iteration (same value each time) |

The inner graph's output node produces the result for that item. All results are collected into the foreach node's `results` array, preserving the original input order.

```json
{
  "id": "process-all",
  "type": "foreach",
  "config": {
    "concurrency": 5,
    "on_item_error": "continue",
    "graph": {
      "nodes": [
        {"id": "in", "type": "input", "output_ports": [
          {"name": "item", "schema": {"type": "string"}},
          {"name": "index", "schema": {"type": "integer"}},
          {"name": "context", "schema": {"type": "object"}}
        ]},
        {"id": "process", "type": "llm", "...": "..."},
        {"id": "out", "type": "output", "input_ports": [
          {"name": "result", "schema": {"type": "string"}}
        ]}
      ],
      "edges": [
        {"source_node_id": "in", "source_port": "item", "target_node_id": "process", "target_port": "text"},
        {"source_node_id": "process", "source_port": "response_text", "target_node_id": "out", "target_port": "result"}
      ]
    }
  }
}
```

## State Scope Isolation

Inner graphs have completely isolated state. The inner graph's state fields exist only for the duration of that subgraph's execution. The inner graph cannot read or write the outer graph's state, and the outer graph cannot see the inner graph's state.

This isolation is deliberate:

- **Predictability.** A subgraph behaves the same regardless of what state exists in the outer graph. No hidden dependencies.
- **Reusability.** You can embed the same subgraph in different outer graphs without worrying about state field name collisions.
- **Encapsulation.** The only data path in and out of a subgraph is through its mapped ports.

If you need to pass accumulated state into a subgraph, read it from state into a port on the outer node, then map that port into the inner graph. If you need the subgraph to contribute to outer state, map an inner output to an outer port, then use a `state_writes` binding on the outer subgraph node.

## Reuse Patterns

### Shared Processing Block

Define a classify-and-format workflow once, then embed it in multiple graphs:

```
Graph A: [Input] --> [Classify Block (subgraph)] --> [Route] --> ...
Graph B: [Input] --> [Enrich] --> [Classify Block (subgraph)] --> [Output]
```

The subgraph is defined inline in each graph (graphs are self-contained), but the inner graph JSON can be copy-pasted or generated from a template. The Terraform provider and CLI both support graph templates to reduce duplication.

### Nested Composition

Subgraphs can contain other subgraphs. A top-level graph might contain a subgraph node that itself contains a foreach node with its own inner graph. The state isolation rule applies at each level -- each inner graph has its own state scope.

### Testing in Isolation

Because subgraphs are complete graphs, you can test them independently. Create the inner graph as a standalone graph, validate and execute it with test inputs, then embed it as a subgraph in the parent graph. The port mapping is the only integration point to verify.

## Subgraph vs ForEach

| | Subgraph | ForEach |
|---|----------|---------|
| **Runs** | Once per node execution | Once per array item |
| **Port mapping** | Configurable via `port_mapping` | Fixed contract (`item`, `index`, `context`) |
| **Output** | Mapped directly to outer output ports | Collected into `results` array |
| **Concurrency** | N/A (single execution) | Configurable (`concurrency` field) |
| **Error handling** | Node fails if inner graph fails | `continue` or `abort` policy |
| **State** | Isolated inner scope | Isolated inner scope |
| **Use for** | Reusable workflow blocks, encapsulation | Batch processing, fan-out/fan-in |

## Validation Rules

- Subgraph port mappings must be complete -- every outer port must map to an inner port.
- Port types must be compatible between the outer and inner graph.
- The inner graph is validated independently using the same rules as top-level graphs.
- ForEach inner graphs must have `item` and `index` input ports.

## See Also

- [Subgraph Node Reference](../nodes/subgraph.md) -- full configuration and examples
- [ForEach Node Reference](../nodes/foreach.md) -- full foreach configuration, error handling, and concurrency
- [Nodes](nodes.md) -- overview of all node types including subgraph and foreach
- [Branching](branching.md#foreach-fan-out) -- foreach as a fan-out pattern
- [Edges](edges.md) -- how inner graph edges work independently of outer edges
- [State](state.md) -- why inner state is isolated and how to pass state through ports

# Edges

Edges are the wiring between nodes. An edge connects one output [port](ports-and-typing.md) on a source node to one input port on a target node. Data produced by the source port flows along the edge to the target port.

## Edge Structure

```json
{
  "id": "edge-1",
  "source_node_id": "llm-1",
  "source_port": "response_text",
  "target_node_id": "transform-1",
  "target_port": "text"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique identifier within the graph |
| `source_node_id` | string | ID of the node that produces data |
| `source_port` | string | Name of the output port on the source node |
| `target_node_id` | string | ID of the node that receives data |
| `target_port` | string | Name of the input port on the target node |

Both the source port and target port must exist on their respective nodes. The validator checks this during [graph validation](graphs.md#validation).

## How Data Flows

When a node finishes executing, its output port values are available. The orchestrator walks each outgoing edge and delivers the source port's value to the target port. If the target node has all its required input ports satisfied (from edges, [state reads](state.md#reading-state), or defaults), it becomes ready to execute.

A single output port can have multiple outgoing edges -- this is how you fan data out to multiple downstream nodes. For example, one LLM node's `response_text` port might feed into both a transform node and a state write.

## Exclusive Fan-In

Multiple edges can target the same input port, but only when the edges come from mutually exclusive [conditional branches](branching.md). This pattern is called **exclusive fan-in** -- it is how you rejoin branches after a conditional node.

```
[Conditional]
  |-- billing  --> [Process Billing]  --result--> [Format] (target: text)
  |-- technical --> [Process Technical] --result--> [Format] (target: text)
  +-- general  --> [Process General]  --result--> [Format] (target: text)
```

Because only one branch fires, exactly one edge delivers real data and the others deliver skip signals. The `Format` node receives whichever branch produced output.

The validator emits a `MULTI_EDGE_FAN_IN` warning when multiple edges target the same port. This is expected for exclusive fan-in from conditional branches but worth double-checking to ensure the branches are truly mutually exclusive.

## Back-Edges (Loops)

A **back-edge** is an edge annotated with `back_edge: true`. It creates a controlled cycle in the graph, allowing nodes to re-execute. Back-edges require two additional fields:

```json
{
  "id": "loop-edge",
  "source_node_id": "evaluator",
  "source_port": "verdict",
  "target_node_id": "researcher",
  "target_port": "feedback",
  "back_edge": true,
  "condition": "input.verdict == 'needs_more'",
  "max_iterations": 5
}
```

| Field | Type | Description |
|-------|------|-------------|
| `back_edge` | boolean | Marks this edge as a back-edge (creates a cycle) |
| `condition` | string | [Expression](expressions.md) evaluated after each iteration. Loop continues while `true`. |
| `max_iterations` | integer | Hard cap on iterations. Prevents runaway loops. |

Back-edges are the only way to create cycles. Any cycle that does not pass through a back-edge is rejected during validation. See the [Loops](loops.md) concept page for detailed back-edge behavior, state accumulation, skip reset, and examples.

## Edges vs State

Edges and [state](state.md) are two different ways for data to move through a graph:

| | Edges | State |
|---|-------|-------|
| **Connection** | One specific output port to one specific input port | Any node can read; any node can write |
| **Scope** | Between two adjacent nodes | Graph-wide, persists across the full execution |
| **Accumulation** | No -- each edge delivers one value per execution | Yes -- reducers (append, merge) accumulate across writes |
| **Loops** | Data from the current iteration only | Accumulates across all iterations |

Use edges for direct node-to-node data flow. Use state when data needs to persist, accumulate, or be shared by unrelated nodes.

## Validation Rules

The validator checks these rules for edges:

- Both `source_node_id` and `target_node_id` must reference existing nodes
- `source_port` must be an output port on the source node
- `target_port` must be an input port on the target node
- Every required input port must be wired (via edge, state read, or default value)
- Every cycle must pass through a back-edge with a `condition` and `max_iterations`
- Multiple edges to the same port only from exclusive branches (`MULTI_EDGE_FAN_IN` warning)

## See Also

- [Ports and Typing](ports-and-typing.md) -- the typed connection points that edges connect
- [Loops](loops.md) -- back-edges, conditions, and iteration limits
- [Branching](branching.md) -- conditional routing and exclusive fan-in
- [State](state.md) -- the alternative to edges for graph-wide data
- [Execution](execution.md) -- how the orchestrator resolves edge data during execution
- [Graphs](graphs.md) -- the container for nodes, edges, and state

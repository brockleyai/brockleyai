# Branching and Joining

Brockley supports conditional routing, fork/join parallelism, and fan-out iteration. This page covers how to split execution into branches and rejoin results.

## Conditional Routing

A **conditional node** evaluates conditions against its input data and routes execution to exactly one branch.

### How It Works

1. The conditional node receives input data on its input ports
2. Each branch has a `condition` expression evaluated in order
3. The first branch whose condition evaluates to `true` fires
4. If no condition matches, the `default_label` branch fires
5. Non-matching branches produce **skip signals**

### Example: Route by Category

```json
{
  "id": "router-1",
  "name": "Route by Category",
  "type": "conditional",
  "input_ports": [
    {"name": "category", "schema": {"type": "string"}},
    {"name": "data", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {"name": "billing", "schema": {"type": "string"}},
    {"name": "technical", "schema": {"type": "string"}},
    {"name": "general", "schema": {"type": "string"}}
  ],
  "config": {
    "branches": [
      {"label": "billing", "condition": "input.category == 'billing'"},
      {"label": "technical", "condition": "input.category == 'technical'"}
    ],
    "default_label": "general"
  }
}
```

The output port names correspond to branch labels. When a branch fires, its output port carries the input data. Other output ports produce skip signals.

### Data Passthrough

The conditional node passes its input data through to the matching branch's output port. It does not transform data -- it just routes it.

## Skip Propagation

When a conditional branch does not fire, it produces a **skip signal**. This signal propagates downstream:

```
[Conditional]
  ├─ billing (fires)  → [Process Billing] → runs normally
  ├─ technical (skip) → [Process Technical] → SKIPPED
  └─ general (skip)   → [Process General] → SKIPPED
```

Any node that receives only skip signals on all its required input ports is itself skipped. This cascades through the graph.

**Key rule**: a node is skipped if and only if all of its required input ports receive skip signals. If any required input port receives real data (from any edge or state read), the node executes.

## Rejoining After Branches (Exclusive Fan-In)

After conditional branches diverge, you often want to rejoin into a single downstream node. This is called **exclusive fan-in**.

```
[Conditional]
  ├─ billing  → [Process Billing]  ─┐
  ├─ technical → [Process Technical] ─┼─→ [Format Response]
  └─ general  → [Process General]  ─┘
```

Multiple edges target the same input port on the join node (`Format Response`). Because the branches are mutually exclusive (only one fires), exactly one edge delivers real data and the others deliver skip signals.

### Wiring the Join

```json
[
  {
    "id": "e-billing-join",
    "source_node_id": "process-billing",
    "source_port": "result",
    "target_node_id": "format-response",
    "target_port": "text"
  },
  {
    "id": "e-technical-join",
    "source_node_id": "process-technical",
    "source_port": "result",
    "target_node_id": "format-response",
    "target_port": "text"
  },
  {
    "id": "e-general-join",
    "source_node_id": "process-general",
    "source_port": "result",
    "target_node_id": "format-response",
    "target_port": "text"
  }
]
```

The join node's `text` port receives whichever branch actually produced data.

### Validation Warning

When multiple edges target the same port, the validator emits a `MULTI_EDGE_FAN_IN` warning. This is expected for exclusive fan-in from conditional branches. The warning reminds you to ensure the branches are truly mutually exclusive.

## Fork/Join (Parallel Branches)

For unconditional parallel branches, simply wire multiple edges from a single node to different downstream nodes:

```
                    ┌─→ [Summarize]    ─┐
[Input] ─→ [Split] ─┤                   ├─→ [Combine]
                    └─→ [Classify]     ─┘
```

Both `Summarize` and `Classify` run in parallel (they have no dependency on each other). The `Combine` node waits for both to complete before executing.

Wire it like this:

```json
[
  {"id": "e1", "source_node_id": "split", "source_port": "text", "target_node_id": "summarize", "target_port": "input"},
  {"id": "e2", "source_node_id": "split", "source_port": "text", "target_node_id": "classify", "target_port": "input"},
  {"id": "e3", "source_node_id": "summarize", "source_port": "result", "target_node_id": "combine", "target_port": "summary"},
  {"id": "e4", "source_node_id": "classify", "source_port": "result", "target_node_id": "combine", "target_port": "category"}
]
```

The `combine` node has two different input ports (`summary` and `category`), so there is no fan-in ambiguity.

## ForEach Fan-Out

The [`foreach`](../nodes/foreach.md) node iterates over an array, executing a [subgraph](subgraphs.md) for each element. This is a structured fan-out/fan-in pattern.

```
[Input] ─→ [ForEach] ─→ [Output]
              │
              └── inner subgraph runs once per item
```

### Configuration

```json
{
  "id": "foreach-1",
  "type": "foreach",
  "input_ports": [
    {
      "name": "items",
      "schema": {"type": "array", "items": {"type": "string"}}
    },
    {
      "name": "context",
      "schema": {"type": "string"},
      "required": false
    }
  ],
  "output_ports": [
    {
      "name": "results",
      "schema": {"type": "array", "items": {"type": "string"}}
    },
    {
      "name": "errors",
      "schema": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "index": {"type": "integer"},
            "error": {"type": "string"}
          }
        }
      }
    }
  ],
  "config": {
    "concurrency": 5,
    "on_item_error": "continue"
  }
}
```

### Inner Subgraph Contract

The inner subgraph receives these inputs for each iteration:

| Input | Type | Description |
|-------|------|-------------|
| `item` | (matches `items` array element type) | The current element |
| `index` | integer | The zero-based position in the array |
| `context` | (matches `context` port type) | Shared context data (same for all iterations) |

### Error Handling

- `on_item_error: "continue"` (default) -- failed items are recorded in the `errors` output, but other items continue processing
- `on_item_error: "abort"` -- the entire foreach stops on the first item failure

### Concurrency

- `concurrency: 0` or omitted -- unlimited parallelism (all items process simultaneously)
- `concurrency: N` -- at most N items process at the same time

## Combining Patterns

These patterns compose naturally. A common real-world pattern:

```
[Input]
  │
  ▼
[Classify] (conditional)
  ├─ simple → [Quick Answer] ──────────────────┐
  └─ complex → [Research] → [ForEach Sources] → [Synthesize] ─┤
                                                                ▼
                                                          [Format Output]
                                                                │
                                                                ▼
                                                            [Output]
```

The conditional routes simple queries to a quick answer path and complex queries through research, iteration over sources, and synthesis. Both paths rejoin at `Format Output` using exclusive fan-in.

## Skip Propagation and State Writes

A skipped node does not execute, which means its `state_writes` bindings do not fire. If you have a node in a conditional branch that writes to [state](state.md), that write only happens when the branch actually fires.

This matters in [loops](loops.md): when a loop body contains conditional branches, the state fields updated by skipped branches keep their previous values. On the next iteration, skip states are cleared and all conditional branches are re-evaluated fresh.

## See Also

- [Execution](execution.md) -- how the orchestrator handles branches and parallelism
- [State](state.md) -- how state interacts with branching
- [Loops](loops.md) -- back-edges, skip reset on loop iteration
- [Edges](edges.md) -- edge wiring and exclusive fan-in
- [ForEach Node Reference](../nodes/foreach.md) -- full foreach configuration
- [Conditional Node Reference](../nodes/conditional.md) -- full conditional configuration
- [Nodes](nodes.md) -- all node types overview

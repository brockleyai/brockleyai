# Loops

Brockley supports controlled iteration through **back-edges** -- [edges](edges.md) that create cycles in the graph. Back-edges let you re-execute a group of nodes until a condition is met or an iteration limit is reached. This is how you build agent loops, retry patterns, iterative refinement, and accumulation workflows.

## Back-Edges

A back-edge is an edge with `back_edge: true`. It must also specify a `condition` [expression](expressions.md) and a `max_iterations` limit:

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
| `back_edge` | boolean | Marks this edge as a back-edge |
| `condition` | string | Expression evaluated after each iteration. Loop continues while `true`. |
| `max_iterations` | integer | Hard limit on total iterations. Prevents infinite loops. |

Both `condition` and `max_iterations` are required. A back-edge without them is rejected during validation.

## How Loops Execute

1. **First iteration.** The loop body executes normally. The orchestrator processes nodes in topological order, skipping the back-edge during the forward pass.

2. **Evaluate the back-edge condition.** After the source node of the back-edge completes, the condition expression is evaluated. The expression has access to `input.*` (the source node's output), `state.*` (current graph state), and `meta.*` (execution metadata).

3. **Continue or exit.** If the condition is `true` and the iteration count is below `max_iterations`, the loop body re-executes from the back-edge's target node. If the condition is `false` or the limit is reached, execution continues forward past the loop.

4. **Repeat.** Each iteration increments `meta.iteration` (0-based). State writes apply their reducers each time. The process repeats until the condition is false or the limit is hit.

```
[Input] --> [Researcher] --> [Evaluator]
                ^                |
                |                | (back-edge: condition = "input.verdict == 'needs_more'")
                +----------------+
                                 |
                                 v (when condition is false)
                            [Output]
```

## Conditions

Back-edge conditions are [expressions](expressions.md) that must return a truthy value for the loop to continue. Common patterns:

```
input.verdict == 'needs_more'                    -- check an explicit verdict
state.collected_items | length < 10              -- loop until enough items
state.iteration_count < 3 && input.quality < 0.9 -- combine limits with quality
input.retry_needed && meta.iteration < 5         -- use meta.iteration directly
```

The `input` namespace in a back-edge condition refers to the output of the back-edge's source node. This is the data that was just produced by the last step of the loop body.

## max_iterations

`max_iterations` is a hard safety limit. When the iteration count reaches this value, the back-edge condition is treated as `false` regardless of its actual evaluation. The loop exits and execution continues forward.

Set this to a reasonable value for your use case. For an LLM refinement loop, 3-5 iterations is typical. For a data collection loop, you might go higher. The point is to prevent runaway loops from burning LLM tokens or running indefinitely.

## State Accumulation in Loops

[State](state.md) is the primary mechanism for building up results across loop iterations. Each iteration, nodes execute their `state_writes` bindings, and the state field's reducer determines how the new value combines with the existing one.

### Example: Collecting Items

```json
{
  "state": {
    "fields": [
      {
        "name": "collected_items",
        "schema": {"type": "array", "items": {"type": "string"}},
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

Each iteration, the LLM node appends a new item to `collected_items` (via an `append` reducer) and writes the current count to `iteration_count` (via a `replace` reducer). After 5 iterations, `collected_items` has 5 entries and `iteration_count` is 5.

### Accessing State in Loop Conditions

Back-edge conditions can reference `state.*` directly:

```
state.collected_items | length < 10
```

This is often more useful than checking `input.*`, because state gives you the accumulated picture of the full loop so far.

## Accessing Loop State in Templates

Inside a loop body, LLM prompt templates can reference the current iteration and accumulated state:

```
This is iteration {{meta.iteration}} of the research loop.

Items collected so far:
{{#each state.collected_items}}
- {{this}}
{{/each}}

Find one more item and add it to the collection.
{{#if meta.iteration >= 3}}
Focus on quality over quantity -- we are near the limit.
{{/if}}
```

The `meta.iteration` field is 0-based: the first iteration is 0, the second is 1, and so on.

## Skip Reset

At the start of each loop iteration, skip states are cleared for all nodes in the loop body. This means [conditional branches](branching.md) are re-evaluated fresh each iteration.

Without skip reset, a conditional node that routed to branch A in iteration 0 would permanently skip branch B's downstream nodes for all future iterations. Skip reset ensures each iteration makes an independent routing decision based on the current data.

## Validation Rules

- Every cycle in the graph must pass through a back-edge. Unguarded cycles are rejected.
- Every back-edge must have both `condition` and `max_iterations`.
- `max_iterations` must be greater than 0.
- The condition expression must be syntactically valid.

## Example: Iterative Refinement

A common pattern is iterative refinement with an evaluator:

```json
{
  "nodes": [
    {
      "id": "writer",
      "type": "llm",
      "config": {
        "user_prompt": "{{#if meta.iteration == 0}}Write a summary of: {{input.text}}{{#else}}Revise this summary based on the feedback: {{input.feedback}}\n\nCurrent draft: {{state.current_draft}}{{/if}}"
      }
    },
    {
      "id": "evaluator",
      "type": "llm",
      "config": {
        "user_prompt": "Evaluate this summary. Respond with JSON.\n\nSummary: {{input.draft}}",
        "response_format": "json",
        "output_schema": {
          "type": "object",
          "properties": {
            "quality": {"type": "number"},
            "feedback": {"type": "string"},
            "verdict": {"type": "string", "enum": ["done", "needs_more"]}
          }
        }
      }
    }
  ],
  "edges": [
    {"source_node_id": "writer", "source_port": "response_text", "target_node_id": "evaluator", "target_port": "draft"},
    {
      "source_node_id": "evaluator",
      "source_port": "response",
      "target_node_id": "writer",
      "target_port": "feedback",
      "back_edge": true,
      "condition": "input.response.verdict == 'needs_more'",
      "max_iterations": 3
    }
  ]
}
```

The writer produces a draft. The evaluator scores it. If the verdict is `needs_more`, the loop continues and the writer receives the feedback. After at most 3 iterations (or when the evaluator says `done`), the loop exits.

## See Also

- [Edges](edges.md) -- edge structure and back-edge fields
- [State](state.md#state-in-loops) -- state accumulation with reducers across iterations
- [Branching](branching.md) -- conditional routing within loop bodies
- [Expressions](expressions.md) -- condition expression syntax and `meta.iteration`
- [Execution](execution.md#loop-execution-back-edges) -- how the orchestrator handles loops
- [Graphs](graphs.md#validation) -- validation rules for cycles and back-edges

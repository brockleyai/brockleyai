# Graph Model

This is a concise summary of how Brockley's graph execution model works. For full field definitions, see `data-model.md`. For the expression language, see `expression-language.md`.

---

## Core Concepts

Agent workflows need **iterative reasoning**, **accumulating context**, **dynamic routing**, and **state that evolves over loops**. Brockley's graph model is built around five ideas:

1. **Typed ports** -- every node has named, schema-typed input and output ports (strong typing: no bare objects/arrays)
2. **Graph state with reducers** -- a persistent, typed state bag that accumulates across execution (especially loops)
3. **Conditional back-edges** -- cycles are allowed via annotated back-edges with safety limits
4. **Branching primitives** -- conditional routing, static fork/join, and dynamic fan-out are all first-class
5. **Self-contained graphs** -- everything needed to execute is inline (schemas, prompts, LLM configs)

---

## Typed Ports

Every node declares its input and output ports. Each port has a name and a JSON Schema type. Edges connect a **source port** to a **target port**. Type compatibility is validated at design time.

```
Node: "classify_intent"
  type: llm
  input_ports:
    - name: "message"       schema: { type: "string" }
    - name: "history"       schema: { type: "array", items: { type: "object",
                              properties: { role: { type: "string" }, content: { type: "string" } },
                              required: ["role", "content"] } }
  output_ports:
    - name: "response"      schema: { type: "object",
                              properties: { category: { type: "string" }, confidence: { type: "number" } },
                              required: ["category", "confidence"] }
```

**Strong typing rule:** All schemas must be fully typed -- no bare `{ type: "object" }` or `{ type: "array" }`. Objects must declare `properties`, arrays must declare `items`, recursively.

---

## Graph State

The graph has a **state schema**: named, typed fields that persist across execution. Each state field has a **reducer** that controls how writes accumulate:

- `replace` -- new value overwrites old (default)
- `append` -- new value is appended to an array
- `merge` -- new object is shallow-merged with existing object

Nodes interact with state through explicit bindings:

```
Node: "researcher"
  state_reads:
    - state_field: "messages"      -> input_port: "history"
    - state_field: "attempt_count" -> input_port: "attempt"
  state_writes:
    - output_port: "response"     -> state_field: "messages"       # appended
    - output_port: "attempt_out"  -> state_field: "attempt_count"  # replaced
```

---

## Branching Patterns

### Conditional Branching (if/switch)

The `conditional` node evaluates branch conditions against its input. The first matching branch fires; non-matching branches are **skipped**. Multiple edges to the same target port are allowed from **mutually exclusive** branches (exclusive fan-in).

### Static Fork/Join (Parallel Branches)

No special construct needed. One output port wired to multiple downstream nodes creates a fork. A node with multiple required input ports from different upstream nodes creates a join (implicit barrier synchronization).

### Dynamic Fan-Out (ForEach)

The `foreach` node iterates over an array, running an inline subgraph per element. Concurrency is configurable. Results are collected in input order.

---

## Loops via Back-Edges

Cycles are allowed through **back-edges** -- edges annotated with a condition and a max iteration count.

```
Edge:
  source: evaluator.verdict
  target: researcher.feedback
  back_edge: true
  condition: "input.verdict == 'needs_more'"
  max_iterations: 5
```

Rules:
- Every back-edge must have a `condition` expression and `max_iterations`
- State writes from loop body nodes apply their reducers each iteration
- `meta.iteration` increments each time the loop head re-executes
- When max_iterations is reached, the back-edge condition is treated as false
- Skip tracking resets each loop iteration (conditional branches are re-evaluated fresh)

---

## Node Types

| Type | Purpose | Key Ports |
|------|---------|-----------|
| `input` | Graph entry point | Output: graph input data |
| `output` | Graph exit point | Input: graph output data |
| `llm` | Call an LLM provider | Input: prompt variables; Output: `response_text` or typed `response` |
| `tool` | Call an MCP tool | Input/Output: from MCP tool schema |
| `conditional` | Route to one of N branches | Input: `value`; Output: one port per branch |
| `transform` | Data transformation | Input/Output: user-defined via expressions |
| `foreach` | Fan-out over array | Input: `items`, `context`; Output: `results`, `errors` |
| `human_in_the_loop` | Pause for human input | Input: context; Output: `action`, `data` |
| `subgraph` | Execute an inline graph | Input/Output: mapped to inner graph |
| `superagent` | Autonomous agent loop | Input: prompt variables; Output: user-defined + meta outputs |

### LLM Node

LLM nodes carry full config inline: provider, model, API key reference, prompt templates, structured output schema. When `response_format: "json"`, the engine automatically instructs the LLM to return JSON matching the schema. Input ports are auto-generated from declared template variables.

### Superagent Node

Superagent nodes embed an autonomous agent loop: prompt assembly, multi-iteration tool calling (via MCP), task tracking, self-evaluation, reflection when stuck, context compaction, and structured output assembly. Dispatched as `node:superagent` asynq tasks; the coordinator stays alive and dispatches its own `node:llm-call`, `node:mcp-call`, and `node:code-exec` tasks. Built-in tools (`_task_*`, `_buffer_*`, `_memory_*`) run locally in the coordinator. When `code_execution.enabled`, the agent gains `_code_execute` and `_code_guidelines` tools; code runs in a separate coderunner process via `node:code-exec` tasks on the `code` queue. When `shared_memory.enabled`, shared memory entries are persisted to graph state via the `_superagent_memory` field (merge reducer).

### Subgraphs

Subgraph nodes embed an inline graph definition. The subgraph has its own state scope -- parent state is not visible inside. Data passes through port mappings.

---

## Execution Model

### Node Scheduling

The engine uses **topological ordering with barrier synchronization**:

1. Identify ready nodes (all required input ports satisfied)
2. Execute ready nodes in parallel
3. Propagate outputs along edges
4. Repeat until all nodes have executed or a terminal condition is met

### Expression Contexts

All expression-capable executors (LLM, transform, conditional) receive a `NodeContext` containing:
- **`state`** -- read-only snapshot of all graph state fields (available as `state.*` in expressions)
- **`meta`** -- execution metadata: `node_id`, `node_name`, `node_type`, `execution_id`, `graph_id`, `graph_name`, `iteration`

This means `state.*` and `meta.*` are directly accessible in all expression contexts (templates, conditions, transforms) without requiring `state_reads` bindings.

### Input Port Resolution (priority order)

1. **Incoming edge** -- value from connected source port
2. **State read** -- current value of bound state field (maps state to `input.*`)
3. **Default value** -- the port's declared default
4. If none of the above and the port is required -> validation error

### Skip Propagation

When a conditional node takes branch A, all nodes reachable **only** through other branches are marked skipped. Skip propagates forward. On loop restart, skip states are cleared for all nodes in the loop body.

---

## Validation Rules

At design time, the engine validates:

1. Port type compatibility on all edges
2. All required ports wired (edge, state read, or default)
3. Every back-edge has a condition and max_iterations
4. State reads/writes reference existing state fields
5. Reducer compatibility (append -> array, merge -> object)
6. Expression validity
7. All nodes reachable from an input node
8. Subgraph port mappings complete and type-compatible
9. Every cycle passes through a back-edge
10. Exclusive fan-in: multiple edges to one port only from exclusive branches
11. ForEach inner graph has `item` and `index` input ports
12. All schemas fully typed (no bare objects/arrays)

### Superagent Validation Rules

For nodes of type `superagent`:

13. Required config: `prompt`, `skills` (non-empty), `provider`, `model` -- `SUPERAGENT_MISSING_CONFIG`
14. Each skill must have `name`, `description`, `mcp_url` -- `SUPERAGENT_INVALID_SKILL`
15. API key: `api_key` or `api_key_ref` required -- `SUPERAGENT_MISSING_CONFIG`
16. At least one output port -- `SUPERAGENT_NO_OUTPUT`
17. If `shared_memory.enabled`, graph must have `_superagent_memory` state field (merge reducer) and node must have state_reads/writes -- `SUPERAGENT_MISSING_SHARED_MEMORY_STATE`
18. If `conversation_history_from_input` set, referenced input port must exist -- `SUPERAGENT_MISSING_CONFIG`
19. Override consistency: if provider specified, model must also be specified -- `SUPERAGENT_INVALID_OVERRIDE`
20. Template variables: `{{input.*}}` should correspond to declared input ports (warning)
21. Numeric limits (`max_iterations`, `max_total_tool_calls`, `max_tool_calls_per_iteration`, `timeout_seconds`) must be > 0 when set -- `SUPERAGENT_MISSING_CONFIG`
22. `overrides.stuck_detection.window_size` must be > 0 when set (prevents divide-by-zero) -- `SUPERAGENT_INVALID_OVERRIDE`
23. `overrides.context_compaction.compaction_threshold` must be in range (0.0, 1.0] -- `SUPERAGENT_INVALID_OVERRIDE`
24. Override with different provider than main node should have its own `api_key` or `api_key_ref` (warning) -- `SUPERAGENT_INVALID_OVERRIDE`
25. Malformed superagent config JSON is a validation error -- `SUPERAGENT_MISSING_CONFIG`

## See Also

- [Data Model](data-model.md) -- entity fields, relationships, validation rules
- [Architecture](architecture.md) -- system overview, distributed execution
- [Expression Language](expression-language.md) -- expression language specification
- [Common Errors](../troubleshooting/common-errors.md) -- validation error codes and fixes

# Execution Model

This page explains how Brockley executes a graph: the order nodes run in, how parallelism works, how data flows through ports, and what events are emitted.

## Execution Lifecycle

An execution goes through these statuses:

```
pending → running → completed
                  → failed
                  → cancelled
                  → timed_out
```

| Status | Meaning |
|--------|---------|
| `pending` | Created but not yet picked up by a worker |
| `running` | A worker is actively executing the graph |
| `completed` | All nodes finished successfully |
| `failed` | A node failed and the execution could not continue |
| `cancelled` | Manually cancelled via the API |
| `timed_out` | Exceeded the execution timeout |

## Invoking a Graph

### Synchronous Mode

The server waits for the execution to complete and returns the result in the response. A sync request may stay open for the full graph timeout window:

```bash
curl -s -X POST http://localhost:8000/api/v1/executions \
  -H "Content-Type: application/json" \
  -d '{
    "graph_id": "GRAPH_ID",
    "input": {"query": "What is Brockley?"},
    "mode": "sync"
  }'
```

Response (after execution completes):

```json
{
  "id": "exec-abc123",
  "status": "completed",
  "output": {
    "answer": "Brockley is an AI agent infrastructure platform."
  }
}
```

### Asynchronous Mode

The server returns immediately with the execution ID. You poll for results or stream events:

```bash
curl -s -X POST http://localhost:8000/api/v1/executions \
  -H "Content-Type: application/json" \
  -d '{
    "graph_id": "GRAPH_ID",
    "input": {"query": "What is Brockley?"},
    "mode": "async"
  }'
```

Response (immediate):

```json
{
  "id": "exec-abc123",
  "status": "pending"
}
```

Then poll:

```bash
curl -s http://localhost:8000/api/v1/executions/exec-abc123
```

### Streaming (SSE)

Connect to the streaming endpoint for real-time events:

```bash
curl -N http://localhost:8000/api/v1/executions/exec-abc123/stream
```

Events arrive as Server-Sent Events:

```
event: node_started
data: {"node_id": "llm-1", "node_type": "llm"}

event: llm_token
data: {"node_id": "llm-1", "token": "Brock"}

event: llm_token
data: {"node_id": "llm-1", "token": "ley"}

event: node_completed
data: {"node_id": "llm-1", "duration_ms": 1234}

event: execution_completed
data: {"output": {"answer": "..."}}
```

## Execution Order

The orchestrator determines execution order using **topological sorting**:

1. Build a dependency graph from edges (ignoring back-edges)
2. Compute the topological order
3. Execute nodes in that order

Nodes with no dependencies on each other can run **in parallel**. The orchestrator identifies independent nodes at each level of the topological order and dispatches them concurrently as separate asynq tasks.

### Distributed Execution

All network I/O happens in separate asynq tasks, distributed across workers. This design keeps the orchestrator lightweight and enables horizontal scaling.

| Operation | Task Type | Dispatched by |
|-----------|-----------|--------------|
| LLM call | `node:llm-call` | Orchestrator or superagent coordinator |
| MCP tool call | `node:mcp-call` | Orchestrator or superagent coordinator |
| ForEach / Subgraph | `node:run` | Orchestrator |
| Superagent loop | `node:superagent` | Orchestrator |
| Code execution | `node:code-exec` | Superagent coordinator |

The **orchestrator** dispatches the initial `graph:start` task, then dispatches individual node tasks. For LLM nodes, each `provider.Complete()` call is a separate `node:llm-call` task. For tool nodes, each `client.CallTool()` is a separate `node:mcp-call` task. These tasks can land on any worker in the pool.

The **superagent coordinator** is itself a long-lived task (`node:superagent`). It stays alive and dispatches its own `node:llm-call`, `node:mcp-call`, and `node:code-exec` tasks for each step in the agent loop. Built-in tools (`_task_*`, `_buffer_*`, `_memory_*`) run locally in the coordinator without dispatching tasks, since they involve no network I/O.

**Pure-computation nodes** (transform, conditional, input, output) execute in-process within the orchestrator. They are fast enough that the overhead of task dispatch would be wasteful.

### Example

```
[Input] → [A] → [C] → [Output]
        → [B] ↗
```

Topological order: `Input`, then `A` and `B` (parallel), then `C`, then `Output`.

## Node Execution Steps

Each node execution is recorded as a **step** with these fields:

| Field | Description |
|-------|-------------|
| `node_id` | Which node ran |
| `node_type` | The node's type (llm, transform, etc.) |
| `iteration` | Which iteration (0 for first run, >0 for loops) |
| `status` | `completed`, `failed`, `skipped`, `retrying` |
| `input` | The resolved input data |
| `output` | The produced output data |
| `state_before` | State snapshot before this node ran |
| `state_after` | State snapshot after state writes were applied |
| `duration_ms` | How long the node took to execute |
| `llm_usage` | Token counts and cost estimate (LLM nodes only) |

Retrieve steps for an execution:

```bash
curl -s http://localhost:8000/api/v1/executions/EXEC_ID/steps | jq .
```

## Port Resolution

When a node is about to execute, the orchestrator resolves each input port's value:

1. **Check edges**: if an upstream node's output port is wired to this input port, use that value
2. **Check state reads**: if a state read binding maps a state field to this port, use the current state value
3. **Check defaults**: if the port has a `default` value, use it
4. **Skip check**: if the port received a skip signal (from a skipped upstream node), propagate the skip

If a required port has no value and no skip signal, the node fails.

## Parallel Execution

Nodes at the same topological level that have no edges between them execute concurrently. This happens automatically -- you don't need to configure it.

Example: if `NodeA` and `NodeB` both depend only on `Input` and have no dependency on each other, they run in parallel.

The orchestrator waits for all nodes at a level to complete before moving to the next level.

## Conditional Skip Propagation

When a conditional node fires one branch, the other branches produce skip signals. These propagate downstream:

1. A non-matching branch's output port produces a skip signal
2. Any node whose **all** required input ports receive skip signals is itself skipped
3. The skipped node's output ports produce skip signals
4. This cascades until it reaches a join point where real data arrives on at least one port

## Loop Execution (Back-Edges)

Loops are created using **back-edges** -- edges marked with `back_edge: true`. A back-edge creates a cycle in the graph that is controlled by:

- A **condition** expression that determines whether to continue looping
- A **max_iterations** limit that prevents infinite loops

### Loop Execution Flow

1. Execute the loop body normally (first iteration)
2. Evaluate the back-edge condition
3. If condition is `true` and iteration count < `max_iterations`, re-execute the loop body
4. If condition is `false` or limit reached, exit the loop and continue forward

### State in Loops

State fields with `append` or `merge` reducers accumulate across iterations. This is the primary mechanism for building up results in a loop.

Each iteration can read the current accumulated state and write new values that get appended/merged.

## Events

The orchestrator emits events throughout execution. These are used for streaming (SSE) and observability:

| Event Type | When |
|------------|------|
| `execution_started` | Execution begins |
| `execution_completed` | Execution finishes successfully |
| `execution_failed` | Execution fails |
| `execution_cancelled` | Execution is cancelled |
| `node_started` | A node begins executing |
| `node_completed` | A node finishes successfully |
| `node_failed` | A node fails |
| `node_skipped` | A node is skipped (conditional branch) |
| `node_retrying` | A node is retrying after failure |
| `state_updated` | A state field was updated |
| `foreach_item_started` | A foreach iteration begins |
| `foreach_item_completed` | A foreach iteration completes |
| `foreach_item_failed` | A foreach iteration fails |
| `back_edge_evaluated` | A back-edge condition was evaluated |
| `llm_token` | An LLM provider produced a token (streaming) |
| `superagent_started` | A superagent node begins its agent loop |
| `superagent_iteration` | A superagent completes one outer loop iteration |
| `superagent_evaluation` | A superagent evaluator renders a verdict |
| `superagent_reflection` | A superagent triggers reflection (stuck recovery) |
| `superagent_stuck_warning` | A superagent detects repeated tool call patterns |
| `superagent_compaction` | A superagent compacts its context window |
| `superagent_memory_store` | A superagent writes to shared memory |
| `superagent_buffer_finalize` | A superagent maps a buffer to an output port |
| `superagent_tool_call` | A superagent executes a tool call |
| `superagent_completed` | A superagent finishes its agent loop |

## Trigger Sources

Executions track what triggered them:

| Trigger | Source |
|---------|--------|
| `api` | REST API call |
| `ui` | Web UI |
| `cli` | CLI tool |
| `terraform` | Terraform provider |
| `mcp` | MCP server |
| `scheduled` | Scheduled execution |

## Cancellation

Cancel a running execution:

```bash
curl -s -X POST http://localhost:8000/api/v1/executions/EXEC_ID/cancel
```

The worker will stop at the next safe point (between node executions) and set the status to `cancelled`.

## Error Handling

When a node fails:

1. If the node has a **retry policy**, the orchestrator retries according to that policy
2. If retries are exhausted or no retry policy exists, the node is marked as `failed`
3. The execution is marked as `failed` with error details including the node ID

Error details are available on the execution record:

```json
{
  "error": {
    "code": "PROVIDER_ERROR",
    "message": "OpenAI API returned 429: rate limit exceeded",
    "node_id": "llm-1",
    "step_id": "step-xyz"
  }
}
```

## LLM Usage Tracking

For LLM nodes, each step records token usage:

```json
{
  "llm_usage": {
    "provider": "openai",
    "model": "gpt-4o",
    "prompt_tokens": 150,
    "completion_tokens": 85,
    "total_tokens": 235,
    "cost_estimate_usd": 0.0047
  }
}
```

## See Also

- [Graphs](graphs.md) -- the container for nodes, edges, and state
- [Edges](edges.md) -- how data flows between nodes
- [Loops](loops.md) -- back-edge conditions and loop execution
- [Branching](branching.md) -- conditional routing patterns
- [State](state.md) -- how state accumulates during execution
- [Superagent](superagent.md) -- the distributed agent loop
- [Architecture Overview](../getting-started/architecture-overview.md) -- system-level view
- [Monitoring](../deployment/monitoring.md) -- metrics, traces, and execution observability

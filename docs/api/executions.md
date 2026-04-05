# Executions API

Invoke graph executions, check status, cancel, list history, get step details, and stream real-time events.

## Invoke Graph

```
POST /api/v1/executions
```

```bash
curl -X POST http://localhost:8000/api/v1/executions \
  -H "Content-Type: application/json" \
  -d '{
    "graph_id": "graph_abc123",
    "input": {
      "ticket_id": "T-1234",
      "subject": "Billing issue",
      "body": "I was charged twice."
    },
    "mode": "async",
    "timeout_seconds": 300,
    "correlation_id": "ext-req-5678"
  }'
```

### Mode Options

| Mode | Behavior |
|------|----------|
| `async` | Returns immediately with execution ID. Poll or stream for results. |
| `sync` | Blocks until execution completes or times out. Returns the full result inline. |
| `stream` | Returns an SSE stream with real-time step events. |

### Async Response

`202 Accepted`

```json
{
  "execution_id": "exec_xyz789",
  "status": "pending",
  "poll_url": "/api/v1/executions/exec_xyz789",
  "stream_url": "/api/v1/executions/exec_xyz789/stream"
}
```

### Sync Response

`200 OK` with the full execution result including `output`.

## Get Execution

```
GET /api/v1/executions/{execution_id}
```

```bash
curl http://localhost:8000/api/v1/executions/exec_xyz789
```

Returns the execution record with current status, input, output (if completed), and error (if failed).

## List Executions

```
GET /api/v1/executions
```

Query parameters: `graph_id`, `status`, `trigger`, `limit`, `cursor`, `sort_by`, `sort_order`

```bash
curl "http://localhost:8000/api/v1/executions?graph_id=graph_abc123&status=completed&limit=10"
```

## Get Execution Steps

```
GET /api/v1/executions/{execution_id}/steps
```

Returns all execution steps ordered by `created_at`. Each step records a single node execution (or one loop iteration of a node).

```bash
curl http://localhost:8000/api/v1/executions/exec_xyz789/steps
```

Response: `200 OK`

```json
{
  "items": [
    {
      "id": "step_001",
      "node_id": "classify",
      "node_type": "llm",
      "iteration": 0,
      "status": "completed",
      "duration_ms": 1250,
      "llm_usage": {
        "provider": "anthropic",
        "model": "claude-sonnet-4-20250514",
        "prompt_tokens": 340,
        "completion_tokens": 45,
        "total_tokens": 385
      }
    }
  ]
}
```

## Cancel Execution

```
POST /api/v1/executions/{execution_id}/cancel
```

Best-effort cancellation. Returns `200 OK` on success, `409 Conflict` if the execution is already in a terminal state (completed, failed, cancelled).

```bash
curl -X POST http://localhost:8000/api/v1/executions/exec_xyz789/cancel
```

## Stream Execution Events (SSE)

```
GET /api/v1/executions/{execution_id}/stream
```

Returns a Server-Sent Events stream with real-time execution events.

```bash
curl -N http://localhost:8000/api/v1/executions/exec_xyz789/stream
```

### Event Types

```
event: step_started
data: {"step_id": "step_001", "node_id": "classify", "node_type": "llm", "attempt": 1}

event: step_completed
data: {"step_id": "step_001", "node_id": "classify", "status": "completed", "duration_ms": 1250}

event: step_failed
data: {"step_id": "step_002", "node_id": "route", "error": {"code": "CONDITION_ERROR", "message": "..."}}

event: llm_token
data: {"step_id": "step_003", "node_id": "respond", "token": "I", "index": 0}

event: execution_completed
data: {"execution_id": "exec_xyz789", "status": "completed", "output": {...}}

event: execution_failed
data: {"execution_id": "exec_xyz789", "status": "failed", "error": {...}}
```

### Tool Loop Events

When an LLM node has `tool_loop` enabled:

```
event: tool_call_started
data: {"execution_id": "exec_xyz789", "node_id": "agent", "tool_name": "search", "arguments": {"query": "pricing"}, "iteration": 1}

event: tool_call_completed
data: {"execution_id": "exec_xyz789", "node_id": "agent", "tool_name": "search", "result_preview": "Found 3 results...", "duration_ms": 450, "is_error": false}

event: tool_loop_iteration
data: {"execution_id": "exec_xyz789", "node_id": "agent", "iteration": 1, "tool_calls_this_round": 2, "total_tool_calls": 2}

event: tool_loop_completed
data: {"execution_id": "exec_xyz789", "node_id": "agent", "total_iterations": 3, "total_tool_calls": 5, "finish_reason": "stop"}
```

### Reconnection

The server supports `Last-Event-ID` for resuming streams after disconnection.

## Provide Human Input

```
POST /api/v1/executions/{execution_id}/steps/{step_id}/input
```

For `human_in_the_loop` nodes that are waiting for input:

```bash
curl -X POST http://localhost:8000/api/v1/executions/exec_xyz789/steps/step_005/input \
  -H "Content-Type: application/json" \
  -d '{
    "action": "approve",
    "data": {"comment": "Looks good, proceed."}
  }'
```

Returns `409 Conflict` if the step is not waiting for input.

## See Also

- [API Overview](overview.md) -- authentication, pagination, error format
- [Graphs API](graphs.md) -- managing graph definitions
- [Health API](health.md) -- server health checks
- [CLI invoke](../cli/invoke.md) -- invoking from the command line

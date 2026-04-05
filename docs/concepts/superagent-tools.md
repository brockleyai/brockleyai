# Superagent Built-In Tools

The superagent provides three categories of built-in tools that run locally inside the coordinator -- zero-latency, no MCP call, no network round-trip. These tools are automatically available to the agent alongside any MCP tools from configured skills.

Built-in tool names are prefixed with `_` to distinguish them from MCP tools.

## Task Tracking

Task tracking prevents model drift during extended agent sessions. The agent creates tasks to plan its work, updates their status as it progresses, and receives periodic reminders of pending tasks.

### Tools

| Tool | Parameters | Returns |
|------|-----------|---------|
| `_task_create` | `description` (string, required), `priority` ("high", "medium", "low"; default: "medium") | Task ID |
| `_task_update` | `id` (string, required), `status` ("pending", "in_progress", "completed") | Confirmation |
| `_task_list` | *(none)* | All tasks with status and priority |

### Task Reminders

After every tool call, the current task list is injected as a system message:

```
[Tasks]
- [completed] #1: Search for recent papers on quantum computing (high)
- [in_progress] #2: Read top 3 papers and extract key findings (high)
- [pending] #3: Synthesize findings into report (medium)
```

The evaluator also receives the task list -- pending tasks signal that work is not done.

You can adjust the reminder frequency with `overrides.task_tracking.reminder_frequency` (default: 1 = every tool call). Set `overrides.task_tracking.enabled: false` to disable task tracking entirely.

### Meta Output

The final task list is available on the `_tasks` meta output port.

## Shared Memory

Shared memory provides persistent fact storage that survives context compaction and can be read by other superagent nodes in the same graph.

### Tools

| Tool | Parameters | Returns |
|------|-----------|---------|
| `_memory_store` | `key` (string, required), `content` (string, required), `tags` (string[], optional) | Confirmation |
| `_memory_recall` | `query` (string, required), `tags` (string[], optional) | Matching entries |
| `_memory_list` | `tags` (string[], optional) | All entries (optionally filtered by tags) |

### How It Works

- **Store:** Entries are stored with a key, content, optional tags, and an automatic namespace prefix (from `shared_memory.namespace` or the node ID).
- **Recall:** Searches by substring match on content and/or tag intersection. Returns all matching entries across all namespaces.
- **List:** Returns all entries, optionally filtered by tags.

### Cross-Node Memory Flow

When `shared_memory.enabled: true`:

1. At startup, prior memory entries are loaded from graph state (`_superagent_memory` field) and optionally injected into the system prompt (`inject_on_start: true`).
2. During execution, the agent stores facts via `_memory_store`.
3. Before context compaction, an automatic "memory flush" extracts key facts from the conversation and stores them (if `auto_flush: true`).
4. At completion, all memory entries are written to the `_memory_out` port and merged into graph state.
5. Downstream superagent nodes read these entries at their startup.

### Configuration

Shared memory requires graph state setup:

```json
{
  "state": {
    "fields": [
      {"name": "_superagent_memory", "schema": {"type": "object"}, "reducer": "merge", "initial": {}}
    ]
  }
}
```

Each superagent node with shared memory must declare:

```json
{
  "state_reads": [{"state_field": "_superagent_memory", "port": "_memory_in"}],
  "state_writes": [{"state_field": "_superagent_memory", "port": "_memory_out"}]
}
```

Shared memory tools are only available when `shared_memory.enabled: true`.

## Output Assembly (Buffers)

Buffers enable arbitrarily large outputs built incrementally across many iterations. Instead of relying on a single LLM response for the final output, the agent constructs content piece by piece using buffer operations, then maps the buffer to an output port.

### Tools

| Tool | Parameters | Returns |
|------|-----------|---------|
| `_buffer_create` | `name` (string, required) | Confirmation |
| `_buffer_append` | `name` (string, required), `content` (string, required) | Confirmation |
| `_buffer_prepend` | `name` (string, required), `content` (string, required) | Confirmation |
| `_buffer_insert` | `name` (string, required), `after` (string, required), `content` (string, required) | Confirmation |
| `_buffer_replace` | `name` (string, required), `old` (string, required), `new` (string, required), `count` (int, optional; 0 = all) | Confirmation |
| `_buffer_delete` | `name` (string, required), `start` (int, required), `end` (int, required) | Confirmation |
| `_buffer_read` | `name` (string, required), `start` (int, optional), `end` (int, optional) | Buffer content |
| `_buffer_length` | `name` (string, required) | Character count |
| `_buffer_finalize` | `name` (string, required), `output_port` (string, required) | Confirmation |

### Buffer Lifecycle

1. **Create** a named buffer (starts empty).
2. **Build** content using append, prepend, insert, replace, delete.
3. **Read** to verify content at any point.
4. **Finalize** to map the buffer to an output port.

Once finalized, a buffer is immutable -- no further modifications allowed.

### Output Resolution Priority

When the agent completes, outputs are resolved in this order:

1. **Buffer finalized to port** -- use buffer content directly (no extraction LLM call).
2. **Single string port, no buffer** -- use the agent's final `response_text`.
3. **Remaining ports** -- call an extraction LLM to map the conversation to the output schema.
4. **All ports have finalized buffers** -- no extraction call needed at all.

This means buffer-based outputs are the most efficient path -- they bypass the extraction step entirely.

### When to Use Buffers

Use buffers when:
- The output is large (reports, code files, documentation)
- The output is built iteratively (research → draft → revise)
- You need precise control over output structure

For simple, short outputs, the single-string fallback or extraction LLM is sufficient.

## Code Execution

Code execution tools allow the agent to write and run Python code during its task. This is useful for data processing, calculations, text manipulation, and any programmatic work that is more efficient in code than through LLM reasoning alone.

### Tools

| Tool | Parameters | Returns |
|------|-----------|---------|
| `_code_execute` | `code` (string, required), `timeout_sec` (integer, optional) | Stdout, stderr, return value, and any tool call results |
| `_code_guidelines` | *(none)* | Coding guidelines, available modules, and tool calling conventions |

### How It Works

1. The agent calls `_code_guidelines` to learn what modules are available and how to invoke tools from code.
2. The agent calls `_code_execute` with Python code. The superagent handler enqueues a `node:code-exec` task on the `code` queue.
3. The coderunner picks up the task and executes the code in a sandboxed Python subprocess with enforced time, memory, and output limits.
4. If the Python code calls tools (using a provided `call_tool()` function), those calls are relayed back to the superagent handler via Redis, which dispatches them as normal MCP/API tool calls and returns results to the Python process.
5. On completion, stdout, stderr, and the return value are sent back to the agent as the tool result.

### Configuration

Code execution is enabled via `code_execution.enabled: true` on the superagent node config. Key limits:

- `max_execution_time_sec` -- wall-clock timeout per execution (default: 30s)
- `max_memory_mb` -- memory cap per execution (default: 256 MB)
- `max_output_bytes` -- stdout/stderr capture limit (default: 1 MB)
- `max_code_bytes` -- code payload size limit (default: 64 KB)
- `max_tool_calls_per_execution` -- tool call cap within a single execution (default: 10)
- `max_executions_per_run` -- total code executions across the entire agent run (default: 50)
- `allowed_modules` -- Python module allowlist (default: safe set including json, math, re, datetime, collections, itertools, etc.)

### Security

Code runs in a subprocess with restricted capabilities:
- No network access from user code (tool calls are the only way to interact with external systems)
- Memory and CPU time are enforced at the process level
- Only allowlisted Python modules can be imported
- Output size is capped to prevent memory exhaustion

### When to Use Code Execution

Use code execution when the agent needs to:
- Process or transform structured data (CSV parsing, JSON manipulation)
- Perform calculations or statistical analysis
- Generate content programmatically (templates, formatted text)
- Run algorithms that are more reliable in code than LLM reasoning

Code execution tools are only available when `code_execution.enabled: true`.

## Disabling Built-In Tools

| Tool Category | Disable With |
|--------------|-------------|
| Task tracking | `overrides.task_tracking.enabled: false` |
| Shared memory | `shared_memory.enabled: false` (or omit `shared_memory`) |
| Buffers | Always available (no disable option) |
| Code execution | `code_execution.enabled: false` (or omit `code_execution`) |

## See Also

- [Superagent Concepts](superagent.md) -- the agent loop, evaluation, reflection, and termination layers
- [Superagent Node Reference](../nodes/superagent.md) -- full configuration reference
- [State](state.md#state-in-superagent-nodes) -- how shared memory persists via graph state
- [Build Your First Agent](../guides/superagent-tutorial.md) -- step-by-step tutorial
- [Multi-Agent Patterns](../guides/superagent-patterns.md) -- pipeline and fan-out patterns with shared memory

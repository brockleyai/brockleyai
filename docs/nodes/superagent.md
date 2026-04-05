# Superagent Node

**Type:** `superagent`

The Superagent node is an autonomous agent loop embeddable in any Brockley graph. Given a task prompt and a set of skills (tool sources), it plans, executes tool calls, tracks progress, evaluates completion, reflects when stuck, compacts context when it grows too large, and assembles structured output -- all inside a single node.

For conceptual background, see the [Superagent Concepts](../concepts/superagent.md) guide.

## When to Use Superagent vs LLM with Tool Loop

Use an **[LLM node](llm.md) with tool loop** when the task is straightforward, needs a few tool calls, and can be completed in a single pass (e.g., "look up X and format it").

Use a **Superagent node** when the task requires autonomous planning, multiple iterations, progress tracking, or large output assembly (e.g., "research this topic, read 5 sources, and produce a report").

## Configuration

### Required Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `prompt` | string | Yes | Task description. Supports `{{input.*}}` template variables. |
| `skills` | SuperagentSkill[] | Yes | Tool sources (MCP servers or API tool definitions). At least one required. |
| `provider` | string | Yes | Primary LLM provider: `openai`, `anthropic`, `google`, `openrouter`, `bedrock`, or `custom`. |
| `model` | string | Yes | Primary LLM model name (e.g., `claude-sonnet-4-6`, `gpt-4o`). |

### Authentication

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `api_key` | string | No | Inline API key. Takes priority over `api_key_ref`. |
| `api_key_ref` | string | No | Secret reference for the API key. At least one of `api_key` or `api_key_ref` is required. |
| `base_url` | string | No | Custom base URL for the LLM provider API. |

### Limits and Tuning

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `system_preamble` | string | No | Persona, tone, or guardrails. Prepended to the auto-assembled system prompt. |
| `max_iterations` | integer | No | Maximum outer loop iterations. Default: 25. |
| `max_total_tool_calls` | integer | No | Maximum tool calls across all iterations. Default: 200. |
| `max_tool_calls_per_iteration` | integer | No | Maximum tool calls per inner tool loop. Default: 25. |
| `max_tool_loop_rounds` | integer | No | Maximum LLM round-trips per inner tool loop. Default: 10. |
| `timeout_seconds` | integer | No | Wall-clock timeout for entire execution. Default: 600. |
| `temperature` | float | No | Sampling temperature. Provider default if omitted. |
| `max_tokens` | integer | No | Maximum tokens per LLM response. Provider default if omitted. |

### Advanced Features

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `shared_memory` | SharedMemoryConfig | No | Cross-node shared memory. See below. |
| `conversation_history_from_input` | string | No | Input port name containing prior conversation history (array of messages). Enables multi-turn. |
| `tool_policies` | ToolPolicies | No | Tool access control. See below. |
| `code_execution` | CodeExecutionConfig | No | Enable Python code execution. See below. |
| `overrides` | SuperagentOverrides | No | Override internal components. See below. |

### SuperagentSkill

Each skill connects the agent to a tool source. Exactly one of `mcp_url` or `api_tool_id` must be set per skill.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Skill name (used in prompt assembly and logging). |
| `description` | string | Yes | What this skill provides (included in the system prompt). |
| `mcp_url` | string | Conditional | URL of the MCP server. Required when using MCP routing. |
| `mcp_transport` | string | No | Transport protocol. Default: `"http"`. |
| `api_tool_id` | string | Conditional | Reference to an [API tool definition](api-tool.md). Required when using API routing. |
| `endpoints` | string[] | Conditional | Which API endpoints to expose as tools. Required when `api_tool_id` is set. |
| `headers` | HeaderConfig[] | No | Custom HTTP headers for the connection. |
| `prompt_fragment` | string | No | Extra context injected into the system prompt for this skill. |
| `tools` | string[] | No | Allowlist of tool names to expose. Empty = all tools from this server. |
| `timeout_seconds` | integer | No | Timeout for calls to this skill. Default: 30. |
| `compacted` | boolean | No | Enable compacted discovery mode. Default: false. |

### SharedMemoryConfig

Enables cross-node shared memory via graph state. Multiple superagent nodes can read and write to a shared memory store, allowing them to pass findings and context to each other.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `enabled` | boolean | Yes | Enable shared memory. |
| `namespace` | string | No | Key prefix for memory entries. Default: node ID. |
| `inject_on_start` | boolean | No | Inject prior memories into the system prompt at startup. Default: true. |
| `auto_flush` | boolean | No | Extract key facts before context compaction. Default: true. |

**State requirement:** When `enabled: true`, the graph must have a `_superagent_memory` state field with `reducer: "merge"`, and the node must declare `state_reads` and `state_writes` for it.

### ToolPolicies

| Field | Type | Description |
|-------|------|-------------|
| `allowed` | string[] | Allowlist mode -- only these tools are available. |
| `denied` | string[] | Denylist -- these tools are excluded. |
| `require_approval` | string[] | Tools excluded from routing; the agent asks for permission in text. |

### CodeExecutionConfig

Enables the agent to write and execute Python code during its task. When enabled, the agent gains two built-in tools: `_code_execute` (run Python code) and `_code_guidelines` (retrieve coding guidelines and available modules).

Code runs in a sandboxed subprocess managed by the coderunner component. Executions are dispatched as `node:code-exec` asynq tasks on the `code` queue.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `enabled` | boolean | Yes | Enable code execution tools. |
| `max_execution_time_sec` | integer | No | Wall-clock limit per execution. Default: 30. |
| `max_memory_mb` | integer | No | Memory limit per execution in MB. Default: 256. |
| `max_output_bytes` | integer | No | Maximum stdout/stderr capture. Default: 1048576 (1 MB). |
| `max_code_bytes` | integer | No | Maximum code payload size. Default: 65536 (64 KB). |
| `max_tool_calls_per_execution` | integer | No | Max tool calls from a single code execution. Default: 10. |
| `max_executions_per_run` | integer | No | Max code executions across the entire agent run. Default: 50. |
| `allowed_modules` | string[] | No | Python module allowlist. Empty = default safe set (json, math, re, datetime, etc.). |

### SuperagentOverrides

Override internal superagent components with different models, prompts, or parameters.

| Field | Type | Description |
|-------|------|-------------|
| `evaluator` | EvaluatorOverride | Override the evaluation LLM that decides if the agent is done. Has `provider`, `model`, `api_key`, `api_key_ref`, `prompt`, `disabled`. |
| `reflection` | ReflectionOverride | Override the reflection LLM that unsticks the agent. Has `provider`, `model`, `api_key`, `api_key_ref`, `prompt`, `max_reflections` (default: 3), `disabled`. |
| `context_compaction` | ContextCompactionOverride | Override compaction behavior. Has `enabled`, `provider`, `model`, `api_key`, `api_key_ref`, `prompt`, `context_window_limit` (default: 128000), `compaction_threshold` (default: 0.75), `preserve_recent_messages` (default: 5). |
| `stuck_detection` | StuckDetectionOverride | Override stuck detection. Has `enabled`, `window_size` (default: 20), `repeat_threshold` (default: 3). |
| `prompt_assembly` | PromptAssemblyOverride | Override prompt template and conventions. Has `template`, `tool_conventions`, `style`. |
| `output_extraction` | OutputExtractionOverride | Override the output extraction LLM. Has `prompt`, `provider`, `model`, `api_key`, `api_key_ref`. |
| `task_tracking` | TaskTrackingOverride | Override task tracking. Has `enabled`, `reminder_frequency` (default: 1). |

## Output Ports

### Developer-Defined Ports

You declare output ports on the node. The agent populates them via:

1. **Buffer finalization** -- call `_buffer_finalize(name, output_port)` to map a buffer to a port.
2. **Single string fallback** -- if there is one string output port and no buffer mapping, the agent's final `response_text` fills it.
3. **Extraction LLM** -- for remaining unmapped ports, a separate LLM call maps the conversation to the output schema.

### Automatic Meta Outputs

These are always available (prefixed with `_`):

| Port | Type | Description |
|------|------|-------------|
| `_conversation_history` | array | Full message history (pass to next invocation for multi-turn). |
| `_iterations` | integer | Outer loop iterations executed. |
| `_total_tool_calls` | integer | Total tool calls made. |
| `_finish_reason` | string | Why the agent stopped: `done`, `max_iterations`, `max_tool_calls`, `stuck`, `timeout`, `cancelled`. |
| `_tool_call_history` | array | Every tool call with name, args, result, duration, error flag. |
| `_working_memory` | object | Final plan, observations, completed steps. |
| `_tasks` | array | Final task list with statuses. |
| `_memory_out` | object | Shared memory entries written (when shared memory enabled). |

## Built-In Tools

The superagent has built-in tools that run locally (no MCP call). See the [Superagent Built-In Tools Guide](../concepts/superagent-tools.md) for full details.

**Task Tracking:** `_task_create`, `_task_update`, `_task_list` -- track progress and prevent drift.

**Shared Memory:** `_memory_store`, `_memory_recall`, `_memory_list` -- persist facts across compaction and across nodes.

**Output Assembly:** `_buffer_create`, `_buffer_append`, `_buffer_prepend`, `_buffer_insert`, `_buffer_replace`, `_buffer_delete`, `_buffer_read`, `_buffer_length`, `_buffer_finalize` -- build large outputs incrementally.

**Code Execution:** `_code_execute`, `_code_guidelines` -- write and run Python code. Available when `code_execution.enabled: true`.

## Architecture

The superagent runs an outer loop of up to `max_iterations`:

1. **Prompt assembly** -- system preamble + task + shared memory + skill descriptions + built-in tool guide + output requirements + working memory.
2. **Inner tool loop** -- LLM generates responses and tool calls; built-in tools handled locally, MCP/API tools dispatched as asynq tasks, code execution dispatched as `node:code-exec` tasks.
3. **Stuck detection** -- circular buffer tracks repeated tool calls. Escalation: warning -> reflection -> force exit.
4. **Evaluation** -- separate LLM call assesses completion: done, needs more work, stuck, or should compact.
5. **Reflection** (if stuck) -- LLM analyzes the situation and produces a new plan.
6. **Context compaction** (if needed) -- memory flush + conversation summarization + context reconstruction.

### Five-Layer Termination

The agent is guaranteed to terminate:

1. `max_tool_calls_per_iteration` -- caps each inner loop.
2. `max_total_tool_calls` -- caps total across all iterations.
3. `max_iterations` -- caps outer loop.
4. `timeout_seconds` -- wall-clock deadline.
5. Stuck detection escalation -- force exit after `max_reflections`.

## Examples

### Minimal: Research Agent

```json
{
  "id": "research-agent",
  "name": "Research Agent",
  "type": "superagent",
  "input_ports": [
    {"name": "topic", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {"name": "report", "schema": {"type": "string"}}
  ],
  "config": {
    "prompt": "Research '{{input.topic}}' thoroughly. Produce a comprehensive report with cited sources.",
    "skills": [
      {"name": "web_search", "description": "Search the web", "mcp_url": "http://search-mcp:9001"},
      {"name": "web_reader", "description": "Read web pages", "mcp_url": "http://reader-mcp:9002"}
    ],
    "provider": "anthropic",
    "model": "claude-sonnet-4-6",
    "api_key_ref": "anthropic_key"
  }
}
```

### Full Overrides: DevOps Incident Agent

```json
{
  "id": "incident-agent",
  "name": "Incident Agent",
  "type": "superagent",
  "input_ports": [
    {"name": "alert", "schema": {"type": "object"}}
  ],
  "output_ports": [
    {"name": "diagnosis", "schema": {"type": "string"}},
    {"name": "actions_taken", "schema": {"type": "array", "items": {"type": "string"}}},
    {"name": "resolved", "schema": {"type": "boolean"}}
  ],
  "config": {
    "prompt": "INCIDENT: {{input.alert.severity}} on {{input.alert.service}}: {{input.alert.message}}\n\nInvestigate, diagnose, and fix if possible.",
    "system_preamble": "You are Atlas, a senior SRE. Direct, technical, thorough.",
    "skills": [
      {"name": "k8s", "description": "Kubernetes cluster management", "mcp_url": "http://k8s-mcp:9001", "prompt_fragment": "Prod cluster: prod-us-east-1. Always specify namespace."},
      {"name": "monitoring", "description": "Prometheus and Grafana queries", "mcp_url": "http://monitoring-mcp:9002"},
      {"name": "logs", "description": "Loki log search", "mcp_url": "http://logs-mcp:9003"}
    ],
    "provider": "anthropic",
    "model": "claude-sonnet-4-6",
    "api_key_ref": "anthropic_key",
    "max_iterations": 25,
    "max_total_tool_calls": 300,
    "timeout_seconds": 900,
    "tool_policies": {
      "denied": ["kubectl_delete"],
      "require_approval": ["kubectl_apply"]
    },
    "shared_memory": {"enabled": true},
    "overrides": {
      "evaluator": {"provider": "google", "model": "gemini-2.5-flash", "api_key_ref": "google_key"},
      "reflection": {"provider": "anthropic", "model": "claude-opus-4-6", "api_key_ref": "anthropic_key"},
      "context_compaction": {"context_window_limit": 200000, "compaction_threshold": 0.8},
      "prompt_assembly": {"tool_conventions": "Always specify namespace with kubectl. Check metrics before and after changes."},
      "stuck_detection": {"window_size": 30, "repeat_threshold": 4}
    }
  }
}
```

### With Code Execution

```json
{
  "id": "data-analyst",
  "name": "Data Analyst",
  "type": "superagent",
  "input_ports": [
    {"name": "question", "schema": {"type": "string"}},
    {"name": "dataset_url", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {"name": "answer", "schema": {"type": "string"}},
    {"name": "code_used", "schema": {"type": "string"}}
  ],
  "config": {
    "prompt": "Answer this question about the dataset: {{input.question}}\n\nDataset URL: {{input.dataset_url}}",
    "skills": [
      {"name": "data_fetcher", "description": "Fetch datasets", "mcp_url": "http://data-mcp:9001"}
    ],
    "provider": "anthropic",
    "model": "claude-sonnet-4-6",
    "api_key_ref": "anthropic_key",
    "code_execution": {
      "enabled": true,
      "max_execution_time_sec": 60,
      "max_memory_mb": 512,
      "max_executions_per_run": 30,
      "allowed_modules": ["json", "math", "statistics", "re", "datetime", "csv"]
    }
  }
}
```

### Pipeline with Shared Memory

Two superagents in a pipeline, sharing findings:

```json
{
  "nodes": [
    {
      "id": "researcher",
      "type": "superagent",
      "config": {
        "prompt": "Research '{{input.topic}}'. Store key findings via _memory_store.",
        "skills": [{"name": "search", "description": "Web search", "mcp_url": "http://search:9001"}],
        "provider": "anthropic",
        "model": "claude-sonnet-4-6",
        "api_key_ref": "anthropic_key",
        "shared_memory": {"enabled": true, "namespace": "research"}
      },
      "state_reads": [{"state_field": "_superagent_memory", "port": "_memory_in"}],
      "state_writes": [{"state_field": "_superagent_memory", "port": "_memory_out"}]
    },
    {
      "id": "writer",
      "type": "superagent",
      "config": {
        "prompt": "Write a report on '{{input.topic}}' using research findings from shared memory.",
        "skills": [],
        "provider": "anthropic",
        "model": "claude-sonnet-4-6",
        "api_key_ref": "anthropic_key",
        "shared_memory": {"enabled": true, "namespace": "writing"}
      },
      "state_reads": [{"state_field": "_superagent_memory", "port": "_memory_in"}],
      "state_writes": [{"state_field": "_superagent_memory", "port": "_memory_out"}]
    }
  ],
  "state": {
    "fields": [
      {"name": "_superagent_memory", "schema": {"type": "object"}, "reducer": "merge", "initial": {}}
    ]
  }
}
```

## See Also

- [Superagent Concepts](../concepts/superagent.md) -- architecture and design rationale
- [Superagent Built-In Tools](../concepts/superagent-tools.md) -- task tracking, memory, buffers, code execution
- [Superagent Tutorial](../guides/superagent-tutorial.md) -- step-by-step walkthrough
- [Superagent Advanced Guide](../guides/superagent-advanced.md) -- overrides, shared memory, multi-turn
- [Superagent Patterns](../guides/superagent-patterns.md) -- common agent architectures
- [LLM Node](llm.md) -- simpler tool calling without full agent loop
- [API Tool Node](api-tool.md) -- API tool definitions used in skills
- [Data Model: Superagent Node Config](../specs/data-model.md) -- complete field reference

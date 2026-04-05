# Data Model

This document defines the core domain entities in Brockley, their fields, relationships, and validation rules. PostgreSQL is the primary data store. Graph documents are stored as JSONB columns, accessed through the Go `pgx` driver.

For a high-level overview of the graph execution model, see `graph-model.md`.
For the expression language used in templates, conditions, and transforms, see `expression-language.md`.

---

## Conventions

- All entities have an `id` field (string, globally unique, system-generated).
- All entities have `created_at` and `updated_at` timestamps (UTC, ISO 8601).
- Soft deletes use a `deleted_at` timestamp where applicable.
- All type definitions use JSON Schema (draft 2020-12) with strong typing rules (see below).
- Field names use `snake_case`.
- Table names use plural form (`graphs`, `schemas`, `executions`).
- Graph documents stored as JSONB columns. Relational fields (id, name, namespace, status, timestamps) as native columns for indexing.

---

## Strong Typing Rules

All JSON Schemas in Brockley -- port schemas, state field schemas, template variable schemas, structured output schemas -- must be **fully typed**. No bare container types.

| Rule | Valid | Invalid |
|------|-------|---------|
| Objects must have `properties` | `{ "type": "object", "properties": { "name": { "type": "string" } }, "required": ["name"] }` | `{ "type": "object" }` |
| Arrays must have `items` | `{ "type": "array", "items": { "type": "string" } }` | `{ "type": "array" }` |
| Nested objects must be typed recursively | `{ "type": "object", "properties": { "address": { "type": "object", "properties": { "city": { "type": "string" } }, "required": ["city"] } } }` | `{ "type": "object", "properties": { "address": { "type": "object" } } }` |
| Scalars are self-describing | `{ "type": "string" }`, `{ "type": "integer" }`, `{ "type": "number" }`, `{ "type": "boolean" }` | |
| Enums are valid | `{ "type": "string", "enum": ["a", "b", "c"] }` | |
| Union types via oneOf | `{ "oneOf": [{ "type": "string" }, { "type": "integer" }] }` | |

These rules are enforced at graph validation time. The engine rejects graphs with under-specified schemas.

**Exception:** The `metadata` field on graphs and nodes is exempt -- it is intentionally untyped (`{ "type": "object", "additionalProperties": true }`).

---

## Self-Contained Graphs

**Graphs are independent documents.** A graph contains everything needed to validate and execute it: all schemas are inline in ports, all prompts are inline in node configs, all LLM provider settings are inline in node configs. Graphs never reference other graphs' resources at runtime.

The **library collections** (Schema, PromptTemplate, ProviderConfig) exist as reusable building blocks. The UI and CLI use them to populate graph definitions. But at save/deploy time, library resources are **copied into** the graph -- they are not referenced at runtime.

---

## Multi-File Graph Definitions

The CLI supports composing a graph from multiple YAML/JSON files:

```bash
brockley validate -f graph.yaml                    # single file
brockley validate -f nodes.yaml -f edges.yaml      # multiple files merged
brockley validate -d ./my-graph/                   # directory mode
```

**Directory convention:**

```
my-graph/
  graph.yaml          # name, description, metadata, state
  nodes/
    classifier.yaml   # one node per file (or multiple)
    handler.yaml
  edges.yaml          # all edges
```

The CLI merges all files into a single Graph object before sending to the engine or server. The engine and server always receive **complete, single-document graphs**.

---

## Entity: Graph

A graph represents a complete, self-contained workflow.

### Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Unique identifier |
| `name` | string | yes | Human-readable name, unique within a namespace |
| `description` | string | no | Free-text description |
| `namespace` | string | yes | Logical grouping (default: `default`) |
| `version` | integer | yes | Monotonically increasing version number |
| `status` | enum | yes | `draft`, `active`, `archived` |
| `nodes` | array[Node] | yes | Embedded in JSONB -- node definitions |
| `edges` | array[Edge] | yes | Embedded in JSONB -- edge definitions |
| `state` | GraphState | no | Embedded in JSONB -- state schema with reducers |
| `metadata` | object | no | Arbitrary key-value metadata (exempt from strong typing) |
| `created_at` | datetime | yes | Creation timestamp |
| `updated_at` | datetime | yes | Last modification timestamp |
| `deleted_at` | datetime | no | Soft-delete timestamp |

### Validation Rules

- `name` must be 1-256 characters, alphanumeric plus hyphens and underscores.
- `nodes` must contain at least one node.
- All edges must reference node IDs and port names that exist in the graph.
- Every cycle must pass through at least one back-edge (no unguarded cycles).
- Each node ID must be unique within the graph.
- There must be at least one `input` node (entry point).
- Port type compatibility is checked on all edges.
- All required input ports must be satisfied (by an edge, a state read, or a default value).
- State reads/writes must reference fields defined in `state`.
- Multiple edges to a single target port only from mutually exclusive conditional branches (exclusive fan-in).
- ForEach inner graphs must have `item` and `index` input ports with typed schemas.
- Conditional nodes must have output ports matching all branch labels + default_label.
- All schemas must pass strong typing rules.
- Expressions in templates, conditions, and transforms must be valid per `expression-language.md`.

---

## Entity: GraphState

The state schema defines typed fields that persist across graph execution. State is the primary mechanism for accumulating data across loop iterations.

### StateField

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Field name (referenced as `state.<name>` in expressions) |
| `schema` | object | yes | JSON Schema (must pass strong typing rules) |
| `reducer` | enum | yes | How writes accumulate: `replace`, `append`, `merge` |
| `initial` | any | no | Initial value (must validate against schema; defaults to type zero value) |

**Reducer semantics:**
- `replace` -- new value overwrites the previous value
- `append` -- new value is appended to array (schema must be `type: "array"` with typed `items`)
- `merge` -- new object is shallow-merged with existing (schema must be `type: "object"` with typed `properties`)

---

## Entity: Node

A node represents a single step in a graph. Each node declares typed input and output ports, optional state bindings, and type-specific configuration.

### Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Unique within the parent graph |
| `name` | string | yes | Human-readable label |
| `type` | string | yes | Node type (built-in or custom) |
| `input_ports` | array[Port] | yes | Typed input port definitions |
| `output_ports` | array[Port] | yes | Typed output port definitions |
| `state_reads` | array[StateBinding] | no | Bind state fields into input ports |
| `state_writes` | array[StateBinding] | no | Push output port values into state fields |
| `config` | object | yes | Type-specific configuration (see below) |
| `retry_policy` | RetryPolicy | no | Retry configuration |
| `timeout_seconds` | integer | no | Maximum execution time for this node |
| `position` | object | no | `{ x, y }` for UI layout |
| `metadata` | object | no | Arbitrary key-value metadata |

### Port

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Port name (unique within a node's input or output ports) |
| `schema` | object | yes | JSON Schema (must pass strong typing rules) |
| `required` | boolean | no | Whether this port must be wired (default: `true` for inputs) |
| `default` | any | no | Default value if unwired (must validate against schema) |

### StateBinding

| Field | Type | Required | Description |
|---|---|---|---|
| `state_field` | string | yes | Name of the state field |
| `port` | string | yes | Name of the input port (for reads) or output port (for writes) |

---

### Built-in Node Types

**`input`** -- Graph entry point. Output ports define the graph's input contract.

**`output`** -- Graph exit point. Input ports define the graph's output contract.

**`llm`** -- Call an LLM provider with an inline prompt.

| Field | Type | Required | Description |
|---|---|---|---|
| `provider` | enum | yes | `openai`, `anthropic`, `google`, `openrouter`, `bedrock`, `custom` |
| `model` | string | yes | Model identifier |
| `api_key` | string | no | Inline API key (masked in API responses) |
| `api_key_ref` | string | no | Reference to secret store |
| `base_url` | string | no | API base URL |
| `system_prompt` | string | no | System message template (supports expression language) |
| `user_prompt` | string | yes | User message template (supports expression language) |
| `variables` | array[TemplateVariable] | yes | Declared template variables with typed schemas |
| `temperature` | float | no | Sampling temperature |
| `max_tokens` | integer | no | Maximum response tokens |
| `response_format` | enum | yes | `text` or `json` |
| `output_schema` | object | no | JSON Schema for structured JSON output (required if `response_format: "json"`) |

**`tool`** -- Call an MCP tool.

| Field | Type | Required | Description |
|---|---|---|---|
| `tool_name` | string | yes | MCP tool name |
| `mcp_url` | string | yes | MCP server URL |
| `mcp_transport` | enum | no | `sse` or `stdio` (default: `sse`) |
| `headers` | array[HeaderConfig] | no | Custom HTTP headers for the MCP connection |

**`conditional`** -- Route execution to one of N mutually exclusive branches.

| Field | Type | Required | Description |
|---|---|---|---|
| `branches` | array[Branch] | yes | Ordered list of label + condition pairs |
| `default_label` | string | yes | Label of the output port used when no branch matches |

**`transform`** -- Data transformation using expressions.

| Field | Type | Required | Description |
|---|---|---|---|
| `expressions` | map[string]string | yes | Map of output port name to expression |

**`foreach`** -- Fan-out: run an inline subgraph for each item in an array, collect results.

| Field | Type | Required | Description |
|---|---|---|---|
| `graph` | Graph | yes | Inline graph definition to execute per item |
| `concurrency` | integer | no | Max parallel iterations (default: 0 = unlimited) |
| `on_item_error` | enum | no | `continue` or `abort` (default: `continue`) |

**`human_in_the_loop`** -- Pause execution for human input.

| Field | Type | Required | Description |
|---|---|---|---|
| `prompt_text` | string | yes | Instructions for the reviewer |
| `timeout_seconds` | integer | no | How long to wait |
| `allowed_actions` | array[string] | no | Actions the reviewer can take (default: `["approve", "reject"]`) |

**`superagent`** -- Autonomous agent loop with built-in task tracking, memory, and output assembly.

| Field | Type | Required | Description |
|---|---|---|---|
| `prompt` | string | yes | Task description. Supports `{{input.*}}` templates. |
| `skills` | array[SuperagentSkill] | yes | MCP server connections providing tools. |
| `provider` | enum | yes | Primary LLM provider. |
| `model` | string | yes | Primary LLM model. |
| `api_key` | string | no | Inline API key (priority over `api_key_ref`). |
| `api_key_ref` | string | no | Secret store reference for API key. |
| `system_preamble` | string | no | Persona/tone/guardrails prepended to system prompt. |
| `max_iterations` | integer | no | Outer loop cap (default: 25). |
| `max_total_tool_calls` | integer | no | Aggregate tool call cap (default: 200). |
| `timeout_seconds` | integer | no | Wall-clock deadline (default: 600). |
| `shared_memory` | SharedMemoryConfig | no | Cross-node shared memory configuration. |
| `tool_policies` | ToolPolicies | no | Tool access policies (allowed/denied/require_approval). |
| `overrides` | SuperagentOverrides | no | Override evaluator, reflection, compaction, stuck detection, prompt, output extraction, task tracking. |
| `code_execution` | CodeExecutionConfig | no | Enable Python code execution for the agent. See below. |

**CodeExecutionConfig:**

| Field | Type | Required | Description |
|---|---|---|---|
| `enabled` | boolean | yes | Enable code execution tools (`_code_execute`, `_code_guidelines`). |
| `max_execution_time_sec` | integer | no | Maximum wall-clock time per code execution (default: 30). |
| `max_memory_mb` | integer | no | Maximum memory per code execution in megabytes (default: 256). |
| `max_output_bytes` | integer | no | Maximum stdout/stderr capture size (default: 1048576). |
| `max_code_bytes` | integer | no | Maximum code payload size (default: 65536). |
| `max_tool_calls_per_execution` | integer | no | Maximum tool calls a single code execution can make (default: 10). |
| `max_executions_per_run` | integer | no | Maximum code executions across the agent's entire run (default: 50). |
| `allowed_modules` | array[string] | no | Python module allowlist. Empty = default safe set. |

**SuperagentSkill:**

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Skill name. |
| `description` | string | yes | What this skill provides. |
| `mcp_url` | string | yes | MCP server URL. |
| `mcp_transport` | string | no | Transport protocol (default: `"http"`). |
| `headers` | array[HeaderConfig] | no | Custom HTTP headers. |
| `prompt_fragment` | string | no | Extra context for system prompt. |
| `tools` | array[string] | no | Allowlist of tool names (empty = all). |
| `timeout_seconds` | integer | no | MCP call timeout (default: 30). |

**SharedMemoryConfig:**

| Field | Type | Required | Description |
|---|---|---|---|
| `enabled` | boolean | yes | Enable shared memory. |
| `namespace` | string | no | Key prefix (default: node ID). |
| `inject_on_start` | boolean | no | Inject prior memories at startup (default: true). |
| `auto_flush` | boolean | no | Extract facts before compaction (default: true). |

**ToolPolicies:**

| Field | Type | Description |
|---|---|---|
| `allowed` | array[string] | Allowlist mode. |
| `denied` | array[string] | Denylist. |
| `require_approval` | array[string] | Tools excluded from routing; agent asks in text. |

**SuperagentOverrides:**

| Field | Type | Description |
|---|---|---|
| `evaluator` | EvaluatorOverride | Evaluation LLM config. Has `provider`, `model`, `api_key`, `api_key_ref`, `prompt`, `disabled`. |
| `reflection` | ReflectionOverride | Reflection LLM config. Has `provider`, `model`, `api_key`, `api_key_ref`, `prompt`, `max_reflections` (default: 3), `disabled`. |
| `context_compaction` | ContextCompactionOverride | Compaction config. Has `enabled`, `provider`, `model`, `api_key`, `api_key_ref`, `prompt`, `context_window_limit` (default: 128000), `compaction_threshold` (default: 0.75), `preserve_recent_messages` (default: 5). |
| `stuck_detection` | StuckDetectionOverride | Stuck detection config. Has `enabled`, `window_size` (default: 20), `repeat_threshold` (default: 3). |
| `prompt_assembly` | PromptAssemblyOverride | Prompt config. Has `template`, `tool_conventions`, `style`. |
| `output_extraction` | OutputExtractionOverride | Output extraction LLM config. Has `prompt`, `provider`, `model`, `api_key`, `api_key_ref`. |
| `task_tracking` | TaskTrackingOverride | Task tracking config. Has `enabled`, `reminder_frequency` (default: 1). |

**`subgraph`** -- Execute an inline graph as a node.

| Field | Type | Required | Description |
|---|---|---|---|
| `graph` | Graph | yes | Inline graph definition to execute |
| `port_mapping` | PortMapping | yes | Maps this node's ports to the inner graph's input/output nodes |

### Retry Policy

| Field | Type | Required | Description |
|---|---|---|---|
| `max_retries` | integer | no | Maximum retry attempts (default: 0) |
| `backoff_strategy` | enum | no | `fixed`, `exponential` (default: `exponential`) |
| `initial_delay_seconds` | float | no | Delay before first retry (default: 1.0) |
| `max_delay_seconds` | float | no | Maximum delay between retries (default: 60.0) |

---

## Entity: Edge

An edge connects an output port on one node to an input port on another node.

### Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Unique within the parent graph |
| `source_node_id` | string | yes | ID of the source node |
| `source_port` | string | yes | Name of the output port on the source node |
| `target_node_id` | string | yes | ID of the target node |
| `target_port` | string | yes | Name of the input port on the target node |
| `back_edge` | boolean | no | If true, this edge creates a loop (default: false) |
| `condition` | string | no | Expression (boolean) -- required if `back_edge: true` |
| `max_iterations` | integer | no | Maximum loop iterations -- required if `back_edge: true` |

---

## Library Collections

These collections are **building-block catalogs** -- reusable resources that the UI and CLI use to populate graph definitions. They are **not referenced at runtime.** When a library resource is used in a graph, it is copied into the graph document.

### Schema Library (`schemas`)

Reusable JSON Schema definitions for common data types.

### Prompt Library (`prompt_templates`)

Reusable prompt templates with declared variables.

### Provider Config Library (`provider_configs`)

Reusable LLM provider configurations (provider, model, base_url, api_key_ref, extra_headers).

---

## Entity: Execution

An execution represents a single invocation of a graph.

### Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Unique identifier |
| `graph_id` | string | yes | Reference to the graph being executed |
| `graph_version` | integer | yes | Version of the graph at invocation time |
| `status` | enum | yes | `pending`, `running`, `completed`, `failed`, `cancelled`, `timed_out` |
| `input` | object | yes | Input data (validated against graph's input port schemas) |
| `output` | object | no | Final output (populated on completion) |
| `state` | object | no | Final graph state at end of execution |
| `error` | ExecutionError | no | Error details (populated on failure) |
| `started_at` | datetime | no | When execution began |
| `completed_at` | datetime | no | When execution finished |
| `timeout_seconds` | integer | no | Maximum allowed execution time |
| `trigger` | enum | yes | `api`, `ui`, `cli`, `terraform`, `mcp`, `scheduled` |
| `correlation_id` | string | no | Caller-provided ID for tracing |
| `metadata` | object | no | Arbitrary key-value metadata |
| `created_at` | datetime | yes | |
| `updated_at` | datetime | yes | |

---

## Entity: ExecutionStep

An execution step records the result of executing a single node. In loops, a node produces multiple steps (one per iteration).

### Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Unique identifier |
| `execution_id` | string | yes | Reference to parent Execution |
| `node_id` | string | yes | Node within the graph |
| `node_type` | string | yes | Node type at execution time |
| `iteration` | integer | yes | Loop iteration (0 if not in loop) |
| `status` | enum | yes | `pending`, `running`, `completed`, `failed`, `skipped`, `retrying` |
| `input` | object | no | Input port values |
| `output` | object | no | Output port values |
| `error` | object | no | Error details |
| `attempt` | integer | yes | Retry attempt (1-based) |
| `started_at` | datetime | no | |
| `completed_at` | datetime | no | |
| `duration_ms` | integer | no | |
| `llm_usage` | LLMUsage | no | Token usage (LLM nodes only) |
| `created_at` | datetime | yes | |

### LLM Usage

| Field | Type | Description |
|---|---|---|
| `provider` | string | Provider name |
| `model` | string | Model used |
| `prompt_tokens` | integer | Tokens in prompt |
| `completion_tokens` | integer | Tokens in completion |
| `total_tokens` | integer | Total tokens |

---

## Entity: NodeTypeDefinition

Custom node types for extensibility.

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Unique identifier |
| `type_name` | string | yes | Type name used in node definitions |
| `display_name` | string | yes | Human-readable name for UI |
| `description` | string | no | What this node type does |
| `category` | string | no | UI grouping (e.g., `ai`, `data`, `integration`) |
| `input_ports` | array[Port] | yes | Default input port definitions |
| `output_ports` | array[Port] | yes | Default output port definitions |
| `config_schema` | object | yes | JSON Schema for type-specific config |
| `executor` | string | yes | Execution implementation ref |

---

## Distributed Task Types

These types are used by the distributed execution model. They are not persisted to PostgreSQL -- they exist as Redis task payloads and result values.

### NodeTaskResult

Pushed to Redis by all node task handlers. The orchestrator BRPOPs from `exec:{execution_id}:results`.

| Field | Type | Description |
|---|---|---|
| `request_id` | string | Correlates to the enqueued request |
| `node_id` | string | Node that produced this result |
| `status` | string | `"completed"` or `"failed"` |
| `outputs` | object | Node output port values (if completed) |
| `error` | string | Error message (if failed) |
| `attempt` | integer | Retry attempt number |

### ToolLoopState

Serializable state for tool loop iterations across task boundaries.

| Field | Type | Description |
|---|---|---|
| `max_calls` | integer | Total tool invocation limit |
| `max_iterations` | integer | LLM round-trip limit |
| `iteration` | integer | Current iteration counter |
| `total_tool_calls` | integer | Running total of tool calls |
| `history` | array[ToolCallHistoryEntry] | All tool invocations |
| `routing` | map[string]ToolRoute | Tool name → MCP route |

### MCPCallResult

Pushed to a tool-loop-specific Redis key (`exec:{id}:llm:{req_id}:mcp-results`).

| Field | Type | Description |
|---|---|---|
| `request_id` | string | Tool call ID |
| `tool_name` | string | MCP tool name |
| `content` | any | Tool result content |
| `error` | string | Error message |
| `is_error` | boolean | Whether the call failed |
| `duration_ms` | integer | Call duration |

---

## Tool Calling Types

These types support LLM tool calling and the tool loop feature.

### LLMToolDefinition

Defines a tool that an LLM can invoke during generation.

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Tool name (must match MCP tool name or custom definition) |
| `description` | string | yes | Human-readable description of what the tool does |
| `parameters` | object | yes | JSON Schema defining the tool's input parameters (must pass strong typing rules) |

### ToolCall

Represents a single tool invocation requested by an LLM.

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Unique identifier for this tool call (provider-generated) |
| `name` | string | yes | Name of the tool to invoke |
| `arguments` | object | yes | Arguments to pass to the tool, validated against the tool's parameter schema |

### ToolRoute

Defines how to reach an MCP server for tool execution.

| Field | Type | Required | Description |
|---|---|---|---|
| `mcp_url` | string | yes | MCP server URL |
| `mcp_transport` | enum | no | `sse` or `stdio` (default: `sse`) |
| `headers` | array[HeaderConfig] | no | Custom HTTP headers for the MCP connection |
| `timeout_seconds` | integer | no | Maximum time to wait for tool execution (default: 30) |

### Extended Message Fields

The `Message` type used in LLM conversations is extended with tool-related fields:

| Field | Type | Required | Description |
|---|---|---|---|
| `tool_calls` | array[ToolCall] | no | Tool calls requested by the assistant (present when role is `assistant`) |
| `tool_call_id` | string | no | ID of the tool call this message is a response to (present when role is `tool`) |
| `tool_result_error` | boolean | no | Whether the tool execution resulted in an error (present when role is `tool`) |

### Extended CompletionRequest Fields

The `CompletionRequest` sent to LLM providers is extended with:

| Field | Type | Required | Description |
|---|---|---|---|
| `tools` | array[LLMToolDefinition] | no | Tools available for the LLM to call |
| `tool_choice` | string | no | Tool selection strategy: `auto`, `none`, `required`, or a specific tool name |

### Extended CompletionResponse Fields

The `CompletionResponse` returned from LLM providers is extended with:

| Field | Type | Required | Description |
|---|---|---|---|
| `tool_calls` | array[ToolCall] | no | Tool calls requested by the model (present when `finish_reason` is `tool_calls`) |

### Extended StreamChunk Fields

The `StreamChunk` used in streaming LLM responses is extended with:

| Field | Type | Required | Description |
|---|---|---|---|
| `tool_calls` | array[ToolCall] | no | Partial or complete tool call data accumulated during streaming |

### Extended LLMNodeConfig Fields

The `llm` node config (see Built-in Node Types above) is extended with tool calling and tool loop fields:

| Field | Type | Required | Description |
|---|---|---|---|
| `tools` | array[LLMToolDefinition] | no | Statically defined tools available to the LLM |
| `tool_choice` | string | no | Tool selection strategy: `auto`, `none`, `required`, or a specific tool name |
| `tool_loop` | boolean | no | Enable the tool loop -- repeatedly call tools until the LLM stops requesting them (default: `false`) |
| `max_tool_calls` | integer | no | Maximum total tool calls across all loop iterations (default: 25) |
| `max_loop_iterations` | integer | no | Maximum number of LLM round-trips in the tool loop (default: 10) |
| `tool_routing` | map[string]ToolRoute | no | Maps tool names to MCP server routes for execution |
| `tool_routing_from_state` | string | no | Expression resolving to a tool routing map from graph state |
| `tool_routing_from_input` | string | no | Expression resolving to a tool routing map from node input |
| `messages_from_state` | string | no | State field name containing conversation history (array of Message objects) to prepend to the prompt |

---

## API Tool Definitions

These types support first-class REST/HTTP API tool integration. API tools allow LLMs to invoke REST endpoints directly, without requiring MCP server wrappers.

### APIToolDefinition (Library Resource)

A reusable library resource that catalogs REST endpoints with shared configuration. Like other library resources, definitions are copied into graphs at save/deploy time.

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Unique identifier |
| `tenant_id` | string | yes | Tenant scoping |
| `name` | string | yes | Human-readable name |
| `namespace` | string | no | Logical grouping (default: `default`) |
| `description` | string | no | What this API provides |
| `base_url` | string | yes | Base URL for all endpoints (e.g., `https://api.stripe.com/v1`) |
| `default_headers` | array[HeaderConfig] | no | Headers applied to all endpoints |
| `default_timeout_ms` | integer | no | Default timeout in milliseconds (0 = 30s) |
| `retry` | RetryConfig | no | Retry configuration for failed requests |
| `endpoints` | array[APIEndpoint] | yes | Catalog of available endpoints |
| `created_at` | datetime | yes | Creation timestamp |
| `updated_at` | datetime | yes | Last modification timestamp |

### APIEndpoint

A single REST endpoint within an API tool definition. Each endpoint maps 1:1 to a tool that LLMs can invoke.

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Endpoint name (used as tool name) |
| `description` | string | yes | What this endpoint does (shown to LLM) |
| `method` | string | yes | HTTP method: `GET`, `POST`, `PUT`, `PATCH`, `DELETE` |
| `path` | string | yes | URL path (supports `{{input.x}}` templates for path parameters) |
| `input_schema` | object | yes | JSON Schema for input parameters (must pass strong typing rules) |
| `output_schema` | object | no | JSON Schema for response validation |
| `headers` | array[HeaderConfig] | no | Endpoint-specific headers (merged with definition defaults) |
| `request_mapping` | RequestMapping | no | How input maps to HTTP request |
| `response_mapping` | ResponseMapping | no | How HTTP response maps to tool output |
| `timeout_ms` | integer | no | Overrides definition default timeout |

### RetryConfig

| Field | Type | Required | Description |
|---|---|---|---|
| `max_retries` | integer | yes | Maximum retry attempts |
| `backoff_ms` | integer | yes | Initial backoff delay in milliseconds |
| `retry_on_status` | array[integer] | no | HTTP status codes that trigger retry (e.g., `[429, 500, 502, 503]`) |

### RequestMapping

| Field | Type | Required | Description |
|---|---|---|---|
| `mode` | enum | yes | `json_body` (default), `form`, `query_params`, `path_and_body` |

### ResponseMapping

| Field | Type | Required | Description |
|---|---|---|---|
| `mode` | enum | yes | `json_body` (default), `text`, `jq`, `headers_and_body` |
| `expression` | string | no | JQ-like expression (required when mode is `jq`) |

### Extended ToolRoute (API + MCP routing)

The `ToolRoute` type is extended to support API endpoint routing alongside MCP routing. Exactly one of `mcp_url` or `api_tool_id`+`api_endpoint` must be set.

| Field | Type | Required | Description |
|---|---|---|---|
| `mcp_url` | string | conditional | MCP server URL (existing) |
| `mcp_transport` | enum | no | `sse` or `stdio` (default: `sse`) |
| `api_tool_id` | string | conditional | Reference to APIToolDefinition library resource (new) |
| `api_endpoint` | string | conditional | Endpoint name within the definition (new, required when `api_tool_id` is set) |
| `headers` | array[HeaderConfig] | no | Custom HTTP headers |
| `timeout_seconds` | integer | no | Per-call timeout (default: 30) |

### APIToolRef (on LLMNodeConfig)

References a specific endpoint from an API tool definition for auto-derivation of tool schemas and routing on LLM nodes.

| Field | Type | Required | Description |
|---|---|---|---|
| `api_tool_id` | string | yes | Library resource ID |
| `endpoint` | string | yes | Endpoint name within the definition |
| `tool_name` | string | no | Override tool name (default: endpoint name) |
| `headers` | array[HeaderConfig] | no | Per-ref header overrides |

### Extended LLMNodeConfig Fields (API Tools)

| Field | Type | Required | Description |
|---|---|---|---|
| `api_tools` | array[APIToolRef] | no | API tool references. Each ref selects a specific endpoint and auto-derives tool schema + routing. |

### Extended SuperagentSkill (API Tools)

The `SuperagentSkill` type is extended with API tool routing. Exactly one of `mcp_url` or `api_tool_id` must be set per skill.

| Field | Type | Required | Description |
|---|---|---|---|
| `api_tool_id` | string | conditional | Reference to APIToolDefinition (new, alternative to `mcp_url`) |
| `endpoints` | array[string] | conditional | Which endpoints to expose as tools (required when `api_tool_id` is set) |

### APIToolNodeConfig (Standalone Node)

Config for standalone `api_tool` nodes that make a direct HTTP call without LLM involvement.

| Field | Type | Required | Description |
|---|---|---|---|
| `api_tool_id` | string | conditional | Library resource ID |
| `endpoint` | string | yes | Endpoint name |
| `inline_endpoint` | InlineAPIEndpoint | conditional | Self-contained endpoint definition (alternative to `api_tool_id`) |
| `headers` | array[HeaderConfig] | no | Header overrides |

### NodeTypeAPITool Constant

```go
const NodeTypeAPITool = "api_tool"
```

The `api_tool` node type is a built-in node type alongside `tool` (MCP). It calls a single API endpoint and maps the response to output ports.

---

## Entity Relationship Summary

```text
Graph (self-contained document)
  |
  +-- embeds --> Node
  |                +-- has --> Port (input/output, each with inline typed schema)
  |                +-- has --> StateBinding (reads/writes)
  |                +-- has --> LLM config (inline: provider, model, prompt, variables)
  |                +-- has --> Inline subgraph (foreach, subgraph nodes)
  |
  +-- embeds --> Edge (port-to-port, optional back_edge with condition + max_iterations)
  |
  +-- has --> GraphState --> StateField (name, typed schema, reducer)

Library (building-block catalog, NOT runtime dependencies)
  +-- Schema Library --> reusable JSON Schemas
  +-- Prompt Library --> reusable prompt templates
  +-- Provider Config Library --> reusable LLM configs
  +-- API Tool Library --> reusable API tool definitions (base URL, endpoints, auth)

Graph <-- references -- Execution --> ExecutionStep (with iteration tracking)

NodeTypeDefinition (for custom node types)
```

## See Also

- [Graph Model](graph-model.md) -- execution model: ports, state, branching, loops
- [Architecture](architecture.md) -- system overview, distributed execution
- [API Design](api-design.md) -- REST API endpoints and conventions
- [Expression Language](expression-language.md) -- expression language specification

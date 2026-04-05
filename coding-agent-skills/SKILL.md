# Brockley Graph Authoring Skill

This file gives a coding agent everything it needs to write valid Brockley agent graph definitions in JSON. Brockley is an open-source AI agent infrastructure platform where workflows are defined as directed graphs with typed ports, graph-level state with reducers, conditional branching, loops via back-edges, and LLM/tool/superagent nodes. Graphs are self-contained JSON documents -- all schemas, prompts, and LLM configs are inline. You do not need any other file to write valid graphs.

---

## 1. Top-Level Graph Structure

Every graph is a single JSON object. When submitting to the API (`POST /api/v1/graphs`), you send this object directly.

| JSON key | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `name` | string | yes | -- | Human-readable name. 1-256 chars, alphanumeric + hyphens + underscores. |
| `description` | string | no | `""` | Free-text description. |
| `namespace` | string | yes | `"default"` | Logical grouping. |
| `version` | integer | yes | `1` | Monotonically increasing. The server auto-increments on update. |
| `status` | string | yes | `"draft"` | One of: `"draft"`, `"active"`, `"archived"`. |
| `nodes` | array[Node] | yes | -- | At least one node. Must include at least one `input` node. |
| `edges` | array[Edge] | yes | -- | Connections between node ports. Can be empty if graph has only one node. |
| `state` | object or null | no | `null` | Graph state definition. See section 5. |
| `metadata` | object | no | `null` | Arbitrary key-value pairs. Exempt from strong typing rules. |

**Minimal valid graph:**

```json
{
  "name": "hello",
  "namespace": "default",
  "version": 1,
  "status": "active",
  "nodes": [
    {
      "id": "input-1",
      "name": "Input",
      "type": "input",
      "input_ports": [],
      "output_ports": [
        {"name": "message", "schema": {"type": "string"}}
      ],
      "config": {}
    },
    {
      "id": "output-1",
      "name": "Output",
      "type": "output",
      "input_ports": [
        {"name": "message", "schema": {"type": "string"}}
      ],
      "output_ports": [
        {"name": "message", "schema": {"type": "string"}}
      ],
      "config": {}
    }
  ],
  "edges": [
    {
      "id": "e1",
      "source_node_id": "input-1",
      "source_port": "message",
      "target_node_id": "output-1",
      "target_port": "message"
    }
  ]
}
```

---

## 2. Node Structure (Common Fields)

Every node, regardless of type, has this shape:

| JSON key | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `id` | string | yes | -- | Unique within the graph. |
| `name` | string | yes | -- | Human-readable label. |
| `type` | string | yes | -- | One of the 11 built-in types (see below). |
| `input_ports` | array[Port] | yes | -- | Typed input port definitions. Can be `[]`. |
| `output_ports` | array[Port] | yes | -- | Typed output port definitions. Can be `[]`. |
| `state_reads` | array[StateBinding] | no | `[]` | Bind state fields to input ports. |
| `state_writes` | array[StateBinding] | no | `[]` | Push output port values into state fields. |
| `config` | object | yes | -- | Type-specific config (see per-type sections). Use `{}` for input/output. |
| `retry_policy` | object | no | `null` | See retry policy below. |
| `timeout_seconds` | integer | no | `null` | Max execution time for this node. |
| `position` | object | no | `null` | `{"x": number, "y": number}` for UI layout. |
| `metadata` | object | no | `null` | Arbitrary key-value pairs. |

### Port

| JSON key | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `name` | string | yes | -- | Unique within a node's input (or output) ports. |
| `schema` | object | yes | -- | JSON Schema. Must pass strong typing rules (see section 11). |
| `required` | boolean | no | `true` for inputs | Whether this port must be wired. |
| `default` | any | no | `null` | Default value if unwired. Must validate against schema. |

### StateBinding

| JSON key | Type | Required | Description |
|----------|------|----------|-------------|
| `state_field` | string | yes | Name of the state field (must exist in `state.fields`). |
| `port` | string | yes | Port name -- input port for `state_reads`, output port for `state_writes`. |

### RetryPolicy

| JSON key | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `max_retries` | integer | no | `0` | Maximum retry attempts. |
| `backoff_strategy` | string | no | `"exponential"` | `"fixed"` or `"exponential"`. |
| `initial_delay_seconds` | float | no | `1.0` | Delay before first retry. |
| `max_delay_seconds` | float | no | `60.0` | Maximum delay between retries. |

---

## 3. Node Types

There are 11 built-in node types.

### 3.1 `input` -- Graph Entry Point

The graph's entry point. Output ports define the shape of data the graph accepts. Input ports are typically `[]` (the graph caller provides data externally).

**Config:** `{}` (empty object)

**Output ports:** Define what the graph accepts as input. When invoked, the caller provides data matching these port schemas.

```json
{
  "id": "input-1",
  "name": "Input",
  "type": "input",
  "input_ports": [],
  "output_ports": [
    {"name": "query", "schema": {"type": "string"}},
    {"name": "context", "schema": {"type": "object", "properties": {"user_id": {"type": "string"}}, "required": ["user_id"]}}
  ],
  "config": {}
}
```

### 3.2 `output` -- Graph Exit Point

Collects final results. Input ports define the graph's output contract. Output ports must mirror the input ports (same names and schemas).

**Config:** `{}` (empty object)

```json
{
  "id": "output-1",
  "name": "Output",
  "type": "output",
  "input_ports": [
    {"name": "result", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {"name": "result", "schema": {"type": "string"}}
  ],
  "config": {}
}
```

### 3.3 `llm` -- Call an LLM Provider

Sends a prompt to an LLM and returns the response. Supports text and structured JSON output.

**Config:**

| JSON key | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `provider` | string | yes | -- | `"openai"`, `"anthropic"`, `"google"`, `"openrouter"`, `"bedrock"`, `"custom"` |
| `model` | string | yes | -- | Model identifier (e.g. `"gpt-4o"`, `"claude-sonnet-4-20250514"`) |
| `api_key` | string | no | -- | Inline API key. Supports env var syntax: `"${ENV_VAR}"`. |
| `api_key_ref` | string | no | -- | Reference to secret store. One of `api_key` or `api_key_ref` needed. |
| `base_url` | string | no | -- | Custom API base URL (for proxies, self-hosted models). |
| `system_prompt` | string | no | `""` | System message template. Supports `{{expression}}` syntax. |
| `user_prompt` | string | yes | -- | User message template. Supports `{{expression}}` syntax. |
| `variables` | array[TemplateVar] | yes | -- | Declared template variables with typed schemas. |
| `temperature` | float | no | provider default | Sampling temperature (0.0 - 2.0). |
| `max_tokens` | integer | no | provider default | Maximum response tokens. |
| `response_format` | string | yes | -- | `"text"` or `"json"`. |
| `output_schema` | object | no | -- | JSON Schema for structured output. Required if `response_format` is `"json"`. |
| `validate_output` | boolean | no | `true` | Validate LLM output against `output_schema`. |
| `extra_headers` | object | no | `null` | Extra HTTP headers (`{"key": "value"}`). |
| `messages` | array[PromptMessage] | no | -- | Full message chain. Overrides system_prompt/user_prompt if set. |

**TemplateVar:**

| JSON key | Type | Required | Description |
|----------|------|----------|-------------|
| `name` | string | yes | Variable name (matches an input port name). |
| `schema` | object | yes | JSON Schema for the variable. |
| `description` | string | no | Human-readable description. |

**PromptMessage:**

| JSON key | Type | Required | Description |
|----------|------|----------|-------------|
| `role` | string | yes | `"system"`, `"user"`, `"assistant"` |
| `content` | string | yes | Template string, supports `{{input.x}}`. |

**Output ports for text mode:** `response_text` (string)

**Output ports for JSON mode:** `response` (object matching output_schema)

**Additional output ports when tool_loop is enabled:** `finish_reason` (string), `total_tool_calls` (integer), `iterations` (integer)

**Example -- text response:**

```json
{
  "id": "responder",
  "name": "Generate Reply",
  "type": "llm",
  "input_ports": [
    {"name": "question", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {"name": "response_text", "schema": {"type": "string"}}
  ],
  "config": {
    "provider": "anthropic",
    "model": "claude-sonnet-4-20250514",
    "api_key_ref": "anthropic_key",
    "system_prompt": "You are a helpful assistant.",
    "user_prompt": "{{input.question}}",
    "variables": [
      {"name": "question", "schema": {"type": "string"}}
    ],
    "response_format": "text",
    "temperature": 0.7,
    "max_tokens": 1024
  }
}
```

**Example -- JSON structured output:**

```json
{
  "id": "classifier",
  "name": "Classify Intent",
  "type": "llm",
  "input_ports": [
    {"name": "message", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {
      "name": "response",
      "schema": {
        "type": "object",
        "properties": {
          "intent": {"type": "string"},
          "confidence": {"type": "number"}
        },
        "required": ["intent", "confidence"]
      }
    }
  ],
  "config": {
    "provider": "openai",
    "model": "gpt-4o",
    "api_key": "${OPENAI_API_KEY}",
    "system_prompt": "Classify the user's intent. Respond with valid JSON only.",
    "user_prompt": "Classify: {{input.message}}",
    "variables": [
      {"name": "message", "schema": {"type": "string"}}
    ],
    "response_format": "json",
    "output_schema": {
      "type": "object",
      "properties": {
        "intent": {"type": "string", "enum": ["question", "complaint", "feedback"]},
        "confidence": {"type": "number"}
      },
      "required": ["intent", "confidence"]
    },
    "temperature": 0.0
  }
}
```

### 3.4 `tool` -- Call an MCP Tool

Calls a single tool on an MCP server. Input ports map to tool input parameters. Output port `result` contains the tool response.

**Config:**

| JSON key | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `tool_name` | string | yes | -- | MCP tool name. |
| `mcp_url` | string | yes | -- | MCP server URL. |
| `mcp_transport` | string | no | `"sse"` | `"sse"` or `"stdio"`. |
| `headers` | array[HeaderConfig] | no | `[]` | Custom HTTP headers. |

**HeaderConfig:**

| JSON key | Type | Required | Description |
|----------|------|----------|-------------|
| `name` | string | yes | Header name. |
| `value` | string | no | Static value. |
| `from_input` | string | no | Dynamic: input port name. |
| `secret_ref` | string | no | Secret store reference. |

```json
{
  "id": "search",
  "name": "Search Tool",
  "type": "tool",
  "input_ports": [
    {"name": "query", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {"name": "result", "schema": {"type": "string"}}
  ],
  "config": {
    "tool_name": "web_search",
    "mcp_url": "http://localhost:8080",
    "headers": [
      {"name": "Authorization", "secret_ref": "mcp_api_key"}
    ]
  }
}
```

### 3.5 `api_tool` -- Call a REST API Endpoint

Calls a single REST endpoint without LLM involvement. Two modes: reference a library definition or inline the endpoint.

**Config (reference mode):**

| JSON key | Type | Required | Description |
|----------|------|----------|-------------|
| `api_tool_id` | string | yes | Library resource ID. |
| `endpoint` | string | yes | Endpoint name within the definition. |
| `headers` | array[HeaderConfig] | no | Header overrides. |

**Config (inline mode):**

| JSON key | Type | Required | Description |
|----------|------|----------|-------------|
| `inline_endpoint` | object | yes | Self-contained endpoint definition. |
| `headers` | array[HeaderConfig] | no | Header overrides. |

**InlineEndpoint:**

| JSON key | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `base_url` | string | yes | -- | Must start with `http://` or `https://`. |
| `method` | string | yes | -- | `GET`, `POST`, `PUT`, `PATCH`, `DELETE`. |
| `path` | string | yes | -- | URL path. Supports `{{input.x}}` templates. |
| `default_headers` | array[HeaderConfig] | no | `[]` | Default headers. |
| `input_schema` | object | no | -- | JSON Schema for input. |
| `output_schema` | object | no | -- | JSON Schema for response. |
| `request_mapping` | object | no | `{"mode": "json_body"}` | `mode`: `"json_body"`, `"form"`, `"query_params"`, `"path_and_body"`. |
| `response_mapping` | object | no | `{"mode": "json_body"}` | `mode`: `"json_body"`, `"text"`, `"jq"`, `"headers_and_body"`. |
| `retry` | object | no | -- | `{"max_retries": int, "backoff_ms": int, "retry_on_status": [int]}`. |
| `timeout_ms` | integer | no | `30000` | Request timeout in milliseconds. |

```json
{
  "id": "get-user",
  "name": "Get User",
  "type": "api_tool",
  "input_ports": [
    {"name": "user_id", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {"name": "result", "schema": {"type": "object", "properties": {"name": {"type": "string"}, "email": {"type": "string"}}}}
  ],
  "config": {
    "inline_endpoint": {
      "base_url": "https://api.example.com",
      "method": "GET",
      "path": "/users/{{input.user_id}}",
      "default_headers": [
        {"name": "Authorization", "secret_ref": "api_key"}
      ]
    }
  }
}
```

### 3.6 `conditional` -- Route to Branches

Evaluates branch conditions in order. The first match fires; others are skipped. Must have output ports matching all branch labels + the default label.

**Config:**

| JSON key | Type | Required | Description |
|----------|------|----------|-------------|
| `branches` | array[Branch] | yes | Ordered list of `{"label": string, "condition": string}`. |
| `default_label` | string | yes | Output port label used when no branch matches. |

**Branch:**

| JSON key | Type | Required | Description |
|----------|------|----------|-------------|
| `label` | string | yes | Must match an output port name. |
| `condition` | string | yes | Expression that evaluates to boolean. |

**Input port:** Conventionally named `value`. The conditional passes its input value through to the matching output port.

**Output ports:** One per branch label + one for default_label. All share the same schema as the input.

```json
{
  "id": "router",
  "name": "Route by Score",
  "type": "conditional",
  "input_ports": [
    {"name": "value", "schema": {"type": "object", "properties": {"score": {"type": "number"}}, "required": ["score"]}}
  ],
  "output_ports": [
    {"name": "high", "schema": {"type": "object", "properties": {"score": {"type": "number"}}, "required": ["score"]}},
    {"name": "medium", "schema": {"type": "object", "properties": {"score": {"type": "number"}}, "required": ["score"]}},
    {"name": "low", "schema": {"type": "object", "properties": {"score": {"type": "number"}}, "required": ["score"]}}
  ],
  "config": {
    "branches": [
      {"label": "high", "condition": "input.value.score > 0.8"},
      {"label": "medium", "condition": "input.value.score > 0.4"}
    ],
    "default_label": "low"
  }
}
```

### 3.7 `transform` -- Data Transformation

Evaluates expressions to compute output port values from inputs. No LLM calls, no side effects.

**Config:**

| JSON key | Type | Required | Description |
|----------|------|----------|-------------|
| `expressions` | object | yes | Map of output port name to expression string. |

Each key in `expressions` must match an output port name. The expression can reference `input.*`, `state.*`, and `meta.*`.

```json
{
  "id": "prep",
  "name": "Prepare Data",
  "type": "transform",
  "input_ports": [
    {"name": "text", "schema": {"type": "string"}},
    {"name": "items", "schema": {"type": "array", "items": {"type": "string"}}}
  ],
  "output_ports": [
    {"name": "upper_text", "schema": {"type": "string"}},
    {"name": "count", "schema": {"type": "number"}},
    {"name": "filtered", "schema": {"type": "array", "items": {"type": "string"}}}
  ],
  "config": {
    "expressions": {
      "upper_text": "input.text | trim | upper",
      "count": "input.items | length",
      "filtered": "input.items | filter(x => x != \"skip\")"
    }
  }
}
```

### 3.8 `foreach` -- Fan-Out Over Array

Runs an inline subgraph once per item in an input array. Collects results in order.

**Config:**

| JSON key | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `graph` | object | yes | -- | Complete inline graph definition. |
| `concurrency` | integer | no | `0` (unlimited) | Max parallel iterations. |
| `on_item_error` | string | no | `"continue"` | `"continue"` or `"abort"`. |

**Required input port:** `items` -- array to iterate over.

**Required output ports:** `results` (array) and `errors` (array).

**Inner graph requirements:**
- Must have an `input` node with output ports `item` (single element type) and `index` (number).
- Must have an `output` node that produces the per-item result.
- The inner graph is a complete graph definition (with `name`, `namespace`, `version`, `status`, `nodes`, `edges`).

```json
{
  "id": "foreach-1",
  "name": "Process Each",
  "type": "foreach",
  "input_ports": [
    {"name": "items", "schema": {"type": "array", "items": {"type": "string"}}}
  ],
  "output_ports": [
    {"name": "results", "schema": {"type": "array", "items": {"type": "string"}}},
    {"name": "errors", "schema": {"type": "array", "items": {"type": "object", "properties": {"index": {"type": "number"}, "error": {"type": "string"}}}}}
  ],
  "config": {
    "graph": {
      "name": "Inner Graph",
      "namespace": "default",
      "version": 1,
      "status": "active",
      "nodes": [
        {
          "id": "inner-input",
          "name": "Input",
          "type": "input",
          "input_ports": [],
          "output_ports": [
            {"name": "item", "schema": {"type": "string"}},
            {"name": "index", "schema": {"type": "number"}}
          ],
          "config": {}
        },
        {
          "id": "inner-transform",
          "name": "Process",
          "type": "transform",
          "input_ports": [{"name": "item", "schema": {"type": "string"}}],
          "output_ports": [{"name": "result", "schema": {"type": "string"}}],
          "config": {
            "expressions": {"result": "input.item | upper"}
          }
        },
        {
          "id": "inner-output",
          "name": "Output",
          "type": "output",
          "input_ports": [{"name": "result", "schema": {"type": "string"}}],
          "output_ports": [{"name": "result", "schema": {"type": "string"}}],
          "config": {}
        }
      ],
      "edges": [
        {"id": "ie1", "source_node_id": "inner-input", "source_port": "item", "target_node_id": "inner-transform", "target_port": "item"},
        {"id": "ie2", "source_node_id": "inner-transform", "source_port": "result", "target_node_id": "inner-output", "target_port": "result"}
      ]
    },
    "concurrency": 5
  }
}
```

### 3.9 `subgraph` -- Execute Inline Graph

Runs a complete inline graph as a single node. The subgraph has its own state scope -- parent state is not visible inside.

**Config:**

| JSON key | Type | Required | Description |
|----------|------|----------|-------------|
| `graph` | object | yes | Complete inline graph definition. |
| `port_mapping` | object | yes | Maps this node's ports to the inner graph's input/output node ports. |

**PortMapping:**

| JSON key | Type | Description |
|----------|------|-------------|
| `inputs` | object | Map of outer input port name to inner `"node_id.port_name"`. |
| `outputs` | object | Map of inner `"node_id.port_name"` to outer output port name. |

```json
{
  "id": "sub-1",
  "name": "Sub-workflow",
  "type": "subgraph",
  "input_ports": [{"name": "data", "schema": {"type": "string"}}],
  "output_ports": [{"name": "result", "schema": {"type": "string"}}],
  "config": {
    "graph": {
      "name": "Inner",
      "namespace": "default",
      "version": 1,
      "status": "active",
      "nodes": [
        {"id": "in", "name": "Input", "type": "input", "input_ports": [], "output_ports": [{"name": "data", "schema": {"type": "string"}}], "config": {}},
        {"id": "t1", "name": "Transform", "type": "transform", "input_ports": [{"name": "data", "schema": {"type": "string"}}], "output_ports": [{"name": "result", "schema": {"type": "string"}}], "config": {"expressions": {"result": "input.data | upper"}}},
        {"id": "out", "name": "Output", "type": "output", "input_ports": [{"name": "result", "schema": {"type": "string"}}], "output_ports": [{"name": "result", "schema": {"type": "string"}}], "config": {}}
      ],
      "edges": [
        {"id": "ie1", "source_node_id": "in", "source_port": "data", "target_node_id": "t1", "target_port": "data"},
        {"id": "ie2", "source_node_id": "t1", "source_port": "result", "target_node_id": "out", "target_port": "result"}
      ]
    },
    "port_mapping": {
      "inputs": {"data": "in.data"},
      "outputs": {"out.result": "result"}
    }
  }
}
```

### 3.10 `human_in_the_loop` -- Pause for Human Input

Pauses execution and waits for a human to approve, reject, or provide data.

**Config:**

| JSON key | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `prompt_text` | string | yes | -- | Instructions for the reviewer. |
| `timeout_seconds` | integer | no | `null` | How long to wait before timing out. |
| `allowed_actions` | array[string] | no | `["approve", "reject"]` | Actions the reviewer can take. |

**Output ports:** `action` (string -- which action was taken) and `data` (object -- any data the reviewer provided).

```json
{
  "id": "review",
  "name": "Human Review",
  "type": "human_in_the_loop",
  "input_ports": [
    {"name": "draft", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {"name": "action", "schema": {"type": "string"}},
    {"name": "data", "schema": {"type": "object", "properties": {"comment": {"type": "string"}}}}
  ],
  "config": {
    "prompt_text": "Review this draft before sending.",
    "timeout_seconds": 3600,
    "allowed_actions": ["approve", "reject", "edit"]
  }
}
```

### 3.11 `superagent` -- Autonomous Agent Loop

Runs an autonomous agent that iterates: prompt assembly, LLM call, tool calling, evaluation, reflection, and output extraction. This is the most powerful node type.

**Config (required):**

| JSON key | Type | Required | Description |
|----------|------|----------|-------------|
| `prompt` | string | yes | Task description. Supports `{{input.*}}` templates. |
| `skills` | array[SuperagentSkill] | yes | At least one skill (MCP server or API tool definition). |
| `provider` | string | yes | LLM provider. |
| `model` | string | yes | LLM model. |
| `api_key` | string | conditional | Inline API key. One of `api_key` or `api_key_ref` required. |
| `api_key_ref` | string | conditional | Secret store reference. |

**Config (optional):**

| JSON key | Type | Default | Description |
|----------|------|---------|-------------|
| `base_url` | string | -- | Custom API base URL. |
| `system_preamble` | string | `""` | Persona/guardrails prepended to system prompt. |
| `max_iterations` | integer | `25` | Outer loop cap. Must be > 0. |
| `max_total_tool_calls` | integer | `200` | Aggregate tool call cap. Must be > 0. |
| `max_tool_calls_per_iteration` | integer | -- | Per-iteration tool call cap. |
| `timeout_seconds` | integer | `600` | Wall-clock deadline. Must be > 0. |
| `temperature` | float | -- | Sampling temperature. |
| `max_tokens` | integer | -- | Max response tokens. |
| `shared_memory` | object | -- | Cross-node shared memory. See below. |
| `conversation_history_from_input` | string | -- | Input port name containing conversation history. |
| `tool_policies` | object | -- | Tool access policies. |
| `overrides` | object | -- | Override internal components. |
| `code_execution` | object | -- | Python code execution config. |

**SuperagentSkill (MCP mode):**

| JSON key | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `name` | string | yes | -- | Skill name. |
| `description` | string | yes | -- | What this skill provides. |
| `mcp_url` | string | yes (if no api_tool_id) | -- | MCP server URL. |
| `mcp_transport` | string | no | `"http"` | Transport protocol. |
| `headers` | array[HeaderConfig] | no | `[]` | Custom HTTP headers. |
| `prompt_fragment` | string | no | `""` | Extra context for system prompt. |
| `tools` | array[string] | no | `[]` | Allowlist of tool names (empty = all). |
| `timeout_seconds` | integer | no | `30` | MCP call timeout. |
| `compacted` | boolean | no | `false` | Use compacted tool discovery. |

**SuperagentSkill (API tool mode):**

| JSON key | Type | Required | Description |
|----------|------|----------|-------------|
| `name` | string | yes | Skill name. |
| `description` | string | yes | What this skill provides. |
| `api_tool_id` | string | yes | Library resource ID. |
| `endpoints` | array[string] | yes | Which endpoints to expose as tools. |

**SharedMemoryConfig:**

| JSON key | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `enabled` | boolean | yes | -- | Enable shared memory. |
| `namespace` | string | no | node ID | Key prefix. |
| `inject_on_start` | boolean | no | `true` | Inject prior memories at startup. |
| `auto_flush` | boolean | no | `true` | Extract facts before compaction. |

When `shared_memory.enabled` is true:
- Graph must have a `_superagent_memory` state field with `merge` reducer and object schema.
- Node must have `state_reads` and `state_writes` for `_superagent_memory`.

**ToolPolicies:**

| JSON key | Type | Description |
|----------|------|-------------|
| `allowed` | array[string] | Allowlist mode -- only these tools can be called. |
| `denied` | array[string] | Denylist -- these tools are blocked. |
| `require_approval` | array[string] | Tools that require human approval. |

**CodeExecutionConfig:**

| JSON key | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `enabled` | boolean | yes | -- | Enable `_code_execute` and `_code_guidelines` tools. |
| `max_execution_time_sec` | integer | no | `30` | Max wall-clock per execution (1-300). |
| `max_memory_mb` | integer | no | `256` | Max memory per execution (1-2048). |
| `max_output_bytes` | integer | no | `1048576` | Max stdout/stderr capture (1-10MB). |
| `max_code_bytes` | integer | no | `65536` | Max code payload (1-1MB). |
| `max_tool_calls_per_execution` | integer | no | `50` | Max tool calls per code execution (1-500). |
| `max_executions_per_run` | integer | no | `20` | Max code executions per agent run (1-100). |
| `allowed_modules` | array[string] | no | default safe set | Python module allowlist. |

**Overrides (all optional):**

| JSON key | Type | Description |
|----------|------|-------------|
| `evaluator` | object | `{provider, model, api_key, api_key_ref, prompt, disabled}` |
| `reflection` | object | `{provider, model, api_key, api_key_ref, prompt, max_reflections, disabled}` |
| `context_compaction` | object | `{enabled, provider, model, api_key, api_key_ref, prompt, context_window_limit, compaction_threshold, preserve_recent_messages}` |
| `stuck_detection` | object | `{enabled, window_size (> 0), repeat_threshold}` |
| `prompt_assembly` | object | `{template, tool_conventions, style}` |
| `output_extraction` | object | `{prompt, provider, model, api_key, api_key_ref}` |
| `task_tracking` | object | `{enabled, reminder_frequency}` |

Override consistency rules:
- If provider is set on an override, model must also be set.
- If override uses a different provider than the main node, it should have its own api_key or api_key_ref.
- `stuck_detection.window_size` must be > 0.
- `context_compaction.compaction_threshold` must be in (0.0, 1.0].

```json
{
  "id": "agent-1",
  "name": "Research Agent",
  "type": "superagent",
  "input_ports": [
    {"name": "topic", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {"name": "result", "schema": {"type": "string"}}
  ],
  "config": {
    "prompt": "Research the following topic and write a summary: {{input.topic}}",
    "provider": "anthropic",
    "model": "claude-sonnet-4-20250514",
    "api_key": "${ANTHROPIC_API_KEY}",
    "max_iterations": 10,
    "max_total_tool_calls": 50,
    "timeout_seconds": 120,
    "skills": [
      {
        "name": "web-search",
        "description": "Search the web for information",
        "mcp_url": "http://search-server:9090"
      }
    ],
    "overrides": {
      "evaluator": {"disabled": true}
    }
  }
}
```

---

## 4. Edges and Wiring

### Edge Structure

| JSON key | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `id` | string | yes | -- | Unique within the graph. |
| `source_node_id` | string | yes | -- | ID of the source node. |
| `source_port` | string | yes | -- | Output port name on the source node. |
| `target_node_id` | string | yes | -- | ID of the target node. |
| `target_port` | string | yes | -- | Input port name on the target node. |
| `back_edge` | boolean | no | `false` | If true, this edge creates a loop. |
| `condition` | string | conditional | -- | Expression (boolean). Required if `back_edge` is true. |
| `max_iterations` | integer | conditional | -- | Max loop iterations. Required if `back_edge` is true. Must be > 0. |

### Normal Edge

```json
{"id": "e1", "source_node_id": "nodeA", "source_port": "output", "target_node_id": "nodeB", "target_port": "input"}
```

### Back-Edge (Loop)

```json
{
  "id": "e-loop",
  "source_node_id": "evaluator",
  "source_port": "retry",
  "target_node_id": "worker",
  "target_port": "data",
  "back_edge": true,
  "condition": "input.retry != null",
  "max_iterations": 5
}
```

### Wiring Rules

- Every edge must reference existing node IDs and port names.
- Source port must be an output port. Target port must be an input port.
- No self-referencing edges unless `back_edge` is true.
- Multiple edges to the same target port are only allowed from mutually exclusive conditional branches (exclusive fan-in).
- Every required input port must be satisfied by: an edge, a state_read, or a default value.
- Every cycle in the graph must pass through at least one back-edge.

### Fork and Join

**Fork:** One output port wired to multiple downstream nodes creates parallel execution.

**Join:** A node with multiple required input ports from different upstream nodes creates a barrier -- the node waits for all inputs.

---

## 5. Graph State

State is a typed, persistent data bag that accumulates across execution -- especially useful in loops.

### State Definition

```json
{
  "state": {
    "fields": [
      {
        "name": "count",
        "schema": {"type": "number"},
        "reducer": "replace",
        "initial": 0
      },
      {
        "name": "messages",
        "schema": {"type": "array", "items": {"type": "object", "properties": {"role": {"type": "string"}, "content": {"type": "string"}}, "required": ["role", "content"]}},
        "reducer": "append",
        "initial": []
      },
      {
        "name": "context",
        "schema": {"type": "object", "properties": {"findings": {"type": "string"}, "status": {"type": "string"}}},
        "reducer": "merge",
        "initial": {}
      }
    ]
  }
}
```

### StateField

| JSON key | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `name` | string | yes | -- | Referenced as `state.<name>` in expressions. |
| `schema` | object | yes | -- | JSON Schema (must pass strong typing). |
| `reducer` | string | yes | -- | `"replace"`, `"append"`, or `"merge"`. |
| `initial` | any | no | type zero value | Initial value. Must validate against schema. |

### Reducer Semantics

| Reducer | Behavior | Schema requirement |
|---------|----------|-------------------|
| `replace` | New value overwrites previous. | Any type. |
| `append` | New value is appended to the array. | Schema must be `type: "array"` with `items`. |
| `merge` | New object is shallow-merged with existing. | Schema must be `type: "object"` with `properties`. |

### Reading and Writing State

**In expressions (direct):** All state fields are available as `state.<name>` in any expression context (templates, conditions, transforms) without needing `state_reads`.

**Via state_reads (explicit binding):** Maps a state field to a specific input port name.

```json
"state_reads": [
  {"state_field": "messages", "port": "history"}
]
```

Then `input.history` in expressions refers to the `messages` state field.

**Via state_writes:** Pushes an output port value into a state field using the field's reducer.

```json
"state_writes": [
  {"state_field": "count", "port": "new_count"},
  {"state_field": "messages", "port": "new_message"}
]
```

---

## 6. Expression Language

Brockley uses one expression language everywhere: prompt templates, conditional conditions, transform expressions, back-edge conditions.

### Namespaces

| Namespace | Description | Example |
|-----------|-------------|---------|
| `input` | Node's resolved input port values. | `input.query` |
| `state` | Read-only snapshot of all graph state fields. | `state.count` |
| `meta` | Execution metadata. | `meta.iteration` |

### Meta Fields

| Field | Type | Description |
|-------|------|-------------|
| `meta.node_id` | string | Current node ID. |
| `meta.node_name` | string | Current node name. |
| `meta.node_type` | string | Current node type. |
| `meta.execution_id` | string | Execution ID. |
| `meta.graph_id` | string | Graph ID. |
| `meta.graph_name` | string | Graph name. |
| `meta.iteration` | integer | Current loop iteration (only in loops). |

### Literals

| Type | Syntax | Examples |
|------|--------|---------|
| String | `"..."` or `'...'` | `"hello"`, `'world'` |
| Integer | digits | `42`, `-1`, `0` |
| Float | digits with `.` | `3.14`, `-0.5` |
| Boolean | `true`, `false` | |
| Null | `null` | |
| Array | `[a, b, c]` | `[1, 2, 3]` |
| Object | `{key: value}` | `{name: "Alice", age: 30}` |

### Operators

**Comparison:** `==`, `!=`, `>`, `>=`, `<`, `<=` (type-strict, no coercion)

**Logical:** `&&`, `||`, `!` (short-circuit)

**Arithmetic:** `+` (add/concat), `-`, `*`, `/`, `%` (division by zero returns null)

**Null handling:** `??` (null coalescing), `?.` (optional chaining)

**Ternary:** `condition ? value_if_true : value_if_false`

**Precedence (high to low):** `?.`, `!`, `*/%`, `+-`, `>>=<<=`, `==!=`, `&&`, `||`, `??`, `?:`

### Property Access

```
input.user.name          -- nested field
input.items[0]           -- array index (0-based)
input.items[-1]          -- negative index (from end)
input.data["key"]        -- bracket notation
input.user?.address      -- optional chaining (null if user is null)
```

### Pipe Filters

Used in templates and expressions. Pipe syntax: `value | filter`. Method syntax: `value.filter()`.

**Array:**

| Filter | Description | Example |
|--------|-------------|---------|
| `length` / `count` | Count elements | `input.items \| length` |
| `first` / `last` | First/last element | `input.items \| first` |
| `slice(start, end?)` | Sub-array | `items \| slice(1, 3)` |
| `take(n)` / `skip(n)` | First n / skip first n | `items \| take(2)` |
| `map(expr)` | Transform each | `items \| map(x => x.name)` |
| `map(field)` | Extract field | `items \| map("name")` |
| `filter(expr)` | Keep matching | `items \| filter(x => x.active)` |
| `filter(field, value)` | Keep by field value | `items \| filter("status", "done")` |
| `reject(expr)` | Remove matching | `items \| reject(x => x.draft)` |
| `flatten` | Flatten one level | `[[1,2],[3]] \| flatten` |
| `reverse` | Reverse order | `items \| reverse` |
| `sort` / `sort(field)` | Sort ascending | `items \| sort("name")` |
| `unique` | Deduplicate | `items \| unique` |
| `sum` / `min` / `max` / `avg` | Aggregation | `items \| sum` |
| `any(expr)` / `all(expr)` / `none(expr)` | Test | `items \| any(x => x > 5)` |
| `contains(value)` | Value exists | `items \| contains("x")` |
| `isEmpty` | True if empty | `items \| isEmpty` |
| `join(sep)` | Join to string | `items \| join(", ")` |
| `concat(other)` | Concatenate arrays | `a \| concat(b)` |
| `groupBy(field)` | Group into object | `items \| groupBy("type")` |

**String:**

| Filter | Description | Example |
|--------|-------------|---------|
| `length` | String length | `text \| length` |
| `trim` | Remove whitespace | `text \| trim` |
| `upper` / `lower` | Case conversion | `text \| upper` |
| `contains(sub)` | Contains substring | `text \| contains("err")` |
| `startsWith` / `endsWith` | Prefix/suffix | `text \| startsWith("http")` |
| `replace(old, new)` | Replace first | `text \| replace("a", "b")` |
| `replaceAll(old, new)` | Replace all | `text \| replaceAll("a", "b")` |
| `split(sep)` | Split to array | `text \| split(",")` |
| `truncate(n)` | Truncate with `...` | `text \| truncate(100)` |
| `matches(regex)` | Regex match | `text \| matches("[0-9]+")` |

**Object:**

| Filter | Description | Example |
|--------|-------------|---------|
| `keys` | Get keys | `obj \| keys` |
| `values` | Get values | `obj \| values` |
| `has(key)` | Check key exists | `obj \| has("name")` |
| `merge(other)` | Merge objects | `obj \| merge({extra: true})` |
| `omit(keys...)` | Remove keys | `obj \| omit("secret")` |
| `pick(keys...)` | Keep only keys | `obj \| pick("name", "email")` |

**Type:**

| Filter | Description | Example |
|--------|-------------|---------|
| `type` | Get type name | `val \| type` |
| `toInt` / `toFloat` / `toString` / `toBool` | Convert | `"42" \| toInt` |
| `json` | Serialize to JSON string | `obj \| json` |
| `parseJson` | Parse JSON string | `str \| parseJson` |
| `round(n?)` / `ceil` / `floor` / `abs` | Numeric | `3.14 \| round(1)` |
| `tokenEstimate` | Estimate token count (chars/4) | `text \| tokenEstimate` |

### Template Syntax (Prompts)

Used in `system_prompt`, `user_prompt`, and `prompt` fields.

**Interpolation:**
```
Hello, {{input.name}}!
Score: {{input.score | round(2)}}.
```

**Conditional blocks:**
```
{{#if input.history | length > 0}}
Previous conversation:
{{input.history | map("content") | join("\n")}}
{{#else}}
No previous conversation.
{{/if}}
```

**Iteration blocks:**
```
{{#each state.findings}}
- Finding {{@index}}: {{this.title}} (confidence: {{this.confidence}})
{{/each}}
```

Inside `#each`: `{{this}}` (current item), `{{@index}}` (0-based index), `{{@first}}` (boolean), `{{@last}}` (boolean).

**Raw output (escape `{{`):**
```
{{raw}}This is not {{an expression}}{{/raw}}
```

### Error Handling

- Missing field returns `null` (not an error).
- `null` propagates: `null + 1` returns `null`.
- Use `??` for defaults: `input.name ?? "unknown"`.
- Use `?.` for safe traversal: `input.user?.profile?.avatar`.
- Division by zero returns `null`.
- Type mismatches in comparisons return `false`.

---

## 7. Tool Calling (LLM Node)

LLM nodes can call tools via the tool loop. The LLM requests tool calls, the engine executes them, and feeds results back.

### Enabling the Tool Loop

Add these fields to the LLM node's config:

| JSON key | Type | Default | Description |
|----------|------|---------|-------------|
| `tool_loop` | boolean | `false` | Enable the tool loop. |
| `tools` | array[LLMToolDefinition] | `[]` | Statically defined tools the LLM can call. |
| `tool_choice` | string | `"auto"` | `"auto"`, `"none"`, `"required"`, or a specific tool name. |
| `max_tool_calls` | integer | `25` | Max total tool invocations across all iterations. |
| `max_loop_iterations` | integer | `10` | Max LLM round-trips. |
| `tool_routing` | object | `{}` | Maps tool names to MCP/API execution targets. |
| `tool_routing_from_state` | string | -- | State field name containing tool routing map. |
| `tool_routing_from_input` | boolean | `false` | If true, reads routing from `tool_routing` input port. |
| `messages_from_state` | string | -- | State field with conversation history to prepend. |
| `api_tools` | array[APIToolRef] | `[]` | API tool references (auto-derive schema + routing). |

**LLMToolDefinition:**

| JSON key | Type | Required | Description |
|----------|------|----------|-------------|
| `name` | string | yes | Tool name. |
| `description` | string | yes | What the tool does (shown to LLM). |
| `parameters` | object | yes | JSON Schema for tool input. |

**ToolRoute:**

| JSON key | Type | Required | Description |
|----------|------|----------|-------------|
| `mcp_url` | string | conditional | MCP server URL. Exactly one of mcp_url or api_tool_id required. |
| `mcp_transport` | string | no | Default `"http"`. |
| `api_tool_id` | string | conditional | API tool definition ID. |
| `api_endpoint` | string | conditional | Endpoint name (required with api_tool_id). |
| `headers` | array[HeaderConfig] | no | Custom headers. |
| `timeout_seconds` | integer | no | Per-call timeout (default 30). |
| `compacted` | boolean | no | Use compacted discovery mode. |

**APIToolRef:**

| JSON key | Type | Required | Description |
|----------|------|----------|-------------|
| `api_tool_id` | string | yes | Library resource ID. |
| `endpoint` | string | yes | Endpoint name. |
| `tool_name` | string | no | Override tool name (default: endpoint name). |
| `headers` | array[HeaderConfig] | no | Per-ref header overrides. |

### Output Ports for Tool Loop Nodes

When `tool_loop` is true, the LLM node produces additional output ports:
- `response_text` (string) -- the LLM's final text response
- `finish_reason` (string) -- `"stop"`, `"max_tool_calls"`, or `"max_iterations"`
- `total_tool_calls` (integer) -- total tool calls made
- `iterations` (integer) -- total LLM round-trips

**Example:**

```json
{
  "id": "assistant",
  "name": "Tool-Using Assistant",
  "type": "llm",
  "input_ports": [
    {"name": "prompt", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {"name": "response_text", "schema": {"type": "string"}},
    {"name": "finish_reason", "schema": {"type": "string"}},
    {"name": "total_tool_calls", "schema": {"type": "integer"}},
    {"name": "iterations", "schema": {"type": "integer"}}
  ],
  "config": {
    "provider": "openai",
    "model": "gpt-4o",
    "api_key": "${OPENAI_API_KEY}",
    "user_prompt": "{{input.prompt}}",
    "variables": [{"name": "prompt", "schema": {"type": "string"}}],
    "response_format": "text",
    "tool_loop": true,
    "max_tool_calls": 20,
    "max_loop_iterations": 5,
    "tool_routing": {
      "search": {"mcp_url": "http://search-server:9090"},
      "calculator": {"mcp_url": "http://calc-server:9090"}
    }
  }
}
```

---

## 8. Validation Rules

The engine enforces these rules. Violating any of them causes graph validation to fail.

### Structural Rules

| Code | Rule |
|------|------|
| `EMPTY_GRAPH` | Graph must have at least one node. |
| `NO_INPUT_NODE` | Graph must have at least one `input` node. |
| `DUPLICATE_NODE_ID` | All node IDs must be unique within the graph. |
| `EMPTY_NODE_ID` | Node ID must not be empty. |
| `EMPTY_NODE_NAME` | Node name must not be empty. |
| `DUPLICATE_PORT_NAME` | Port names must be unique within a node's input (or output) ports. |
| `EMPTY_PORT_NAME` | Port name must not be empty. |
| `MISSING_PORT_SCHEMA` | Every port must have a schema. |

### Schema Rules

| Code | Rule |
|------|------|
| `SCHEMA_VIOLATION` | Object schemas must have `properties` (no bare `{"type": "object"}`). |
| `SCHEMA_VIOLATION` | Array schemas must have `items` (no bare `{"type": "array"}`). |
| `MISSING_TYPE` | Every schema must have a `type` field (unless using `oneOf`/`anyOf`/`enum`). |
| `INVALID_SCHEMA` | Schema must be valid JSON. |

### Edge Rules

| Code | Rule |
|------|------|
| `EMPTY_EDGE_ID` | Edge ID must not be empty. |
| `INVALID_SOURCE_NODE` | Source node must exist. |
| `INVALID_TARGET_NODE` | Target node must exist. |
| `INVALID_SOURCE_PORT` | Source port must exist on the source node's output ports. |
| `INVALID_TARGET_PORT` | Target port must exist on the target node's input ports. |
| `SELF_REFERENCE` | Cannot self-reference without `back_edge: true`. |
| `UNGUARDED_CYCLE` | Every cycle must pass through a back-edge. |

### Back-Edge Rules

| Code | Rule |
|------|------|
| `BACKEDGE_NO_CONDITION` | Back-edge must have a `condition` expression. |
| `BACKEDGE_NO_MAX_ITERATIONS` | Back-edge must have `max_iterations` > 0. |

### State Rules

| Code | Rule |
|------|------|
| `EMPTY_STATE_FIELD` | State field name must not be empty. |
| `DUPLICATE_STATE_FIELD` | State field names must be unique. |
| `REDUCER_INCOMPATIBLE` | `append` requires array schema. `merge` requires object schema. |
| `INVALID_STATE_REF` | State reads/writes must reference existing state fields. |
| `INVALID_STATE_PORT` | State reads must reference existing input ports. State writes must reference existing output ports. |

### Port Wiring Rules

| Code | Rule |
|------|------|
| `UNWIRED_REQUIRED_PORT` | Required input port must be wired (edge, state read, or default). |
| `MULTI_EDGE_FAN_IN` | Multiple edges to one port -- must be from exclusive conditional branches (warning). |

### Reachability

| Code | Rule |
|------|------|
| `UNREACHABLE_NODE` | All nodes should be reachable from an input node (warning). |

### Tool Loop Rules

| Code | Rule |
|------|------|
| `TOOL_LOOP_NO_ROUTING` | `tool_loop` requires `tool_routing`, `tool_routing_from_state`, `tool_routing_from_input`, or `api_tools`. |
| `TOOL_ROUTE_NO_TARGET` | Each tool route must have `mcp_url` or `api_tool_id`. |
| `TOOL_ROUTE_AMBIGUOUS` | A route cannot have both `mcp_url` and `api_tool_id`. |
| `TOOL_ROUTE_INCOMPLETE` | `api_tool_id` requires `api_endpoint`. |
| `TOOL_LOOP_BAD_LIMIT` | `max_tool_calls` and `max_loop_iterations` must be positive. |
| `TOOL_CHOICE_INVALID` | `tool_choice` must be `"auto"`, `"none"`, `"required"`, or a defined tool name. |

### Superagent Rules

| Code | Rule |
|------|------|
| `SUPERAGENT_MISSING_CONFIG` | Requires `prompt`, `skills` (non-empty), `provider`, `model`, `api_key`/`api_key_ref`. |
| `SUPERAGENT_INVALID_SKILL` | Each skill needs `name`, `description`, and exactly one of `mcp_url` or `api_tool_id`. |
| `SUPERAGENT_NO_OUTPUT` | Must have at least one output port. |
| `SUPERAGENT_MISSING_SHARED_MEMORY_STATE` | When `shared_memory.enabled`, graph needs `_superagent_memory` state field (merge reducer) and the node needs state_reads/writes for it. |
| `SUPERAGENT_INVALID_OVERRIDE` | Override consistency (provider+model, threshold ranges, window_size > 0). |
| `SUPERAGENT_INVALID_CODE_EXEC` | Code execution limits out of range. |

### API Tool Rules

| Code | Rule |
|------|------|
| `API_TOOL_NO_DEFINITION` | Requires either `api_tool_id`+`endpoint` or `inline_endpoint`. |
| `API_TOOL_AMBIGUOUS` | Cannot have both `api_tool_id` and `inline_endpoint`. |
| `API_TOOL_INLINE_NO_BASE_URL` | Inline endpoint requires `base_url` starting with `http://` or `https://`. |
| `API_TOOL_INLINE_NO_METHOD` | Inline endpoint requires `method` (GET/POST/PUT/PATCH/DELETE). |
| `API_TOOL_INLINE_NO_PATH` | Inline endpoint requires `path`. |

---

## 9. Strong Typing Rules

All JSON Schemas in Brockley -- port schemas, state field schemas, output schemas -- must be fully typed. No bare container types.

**Rules:**

1. Object schemas must have `properties`:
   - Valid: `{"type": "object", "properties": {"name": {"type": "string"}}, "required": ["name"]}`
   - Invalid: `{"type": "object"}`

2. Array schemas must have `items`:
   - Valid: `{"type": "array", "items": {"type": "string"}}`
   - Invalid: `{"type": "array"}`

3. Nested objects must be typed recursively -- no bare objects at any depth.

4. Scalars are self-describing: `{"type": "string"}`, `{"type": "integer"}`, `{"type": "number"}`, `{"type": "boolean"}`

5. Enums are valid: `{"type": "string", "enum": ["a", "b", "c"]}`

6. Union types via `oneOf`: `{"oneOf": [{"type": "string"}, {"type": "integer"}]}`

**Exception:** The `metadata` field on graphs and nodes is exempt.

---

## 10. Enums Reference

| Enum | Values |
|------|--------|
| Graph status | `"draft"`, `"active"`, `"archived"` |
| Node types | `"input"`, `"output"`, `"llm"`, `"tool"`, `"api_tool"`, `"conditional"`, `"transform"`, `"foreach"`, `"subgraph"`, `"human_in_the_loop"`, `"superagent"` |
| Reducer | `"replace"`, `"append"`, `"merge"` |
| LLM provider | `"openai"`, `"anthropic"`, `"google"`, `"openrouter"`, `"bedrock"`, `"custom"` |
| Response format | `"text"`, `"json"` |
| Tool choice | `"auto"`, `"none"`, `"required"`, or a tool name |
| MCP transport | `"sse"`, `"stdio"`, `"http"` |
| Backoff strategy | `"fixed"`, `"exponential"` |
| ForEach error handling | `"continue"`, `"abort"` |
| HTTP methods | `"GET"`, `"POST"`, `"PUT"`, `"PATCH"`, `"DELETE"` |
| Request mapping mode | `"json_body"`, `"form"`, `"query_params"`, `"path_and_body"` |
| Response mapping mode | `"json_body"`, `"text"`, `"jq"`, `"headers_and_body"` |
| Execution status | `"pending"`, `"running"`, `"completed"`, `"failed"`, `"cancelled"`, `"timed_out"` |
| Execution trigger | `"api"`, `"ui"`, `"cli"`, `"terraform"`, `"mcp"`, `"scheduled"` |
| Execution mode | `"sync"`, `"async"` |

---

## 11. API Reference (Quick)

### Create Graph
```
POST /api/v1/graphs
Body: {graph JSON}
Response: 201 Created
```

### Validate Graph
```
POST /api/v1/graphs/{graph_id}/validate
Response: {"valid": true, "warnings": [...]}
```

### Execute Graph
```
POST /api/v1/executions
Body: {
  "graph_id": "...",
  "input": {...},
  "mode": "sync" | "async" | "stream"
}
```

### CLI
```bash
brockley validate -f graph.json
brockley run -f graph.json -i '{"key": "value"}'
brockley deploy -f graph.json --name my-graph
```

---

## 12. Complete Examples

Full, runnable example graphs are in the `examples/` directory alongside this file:

| File | Pattern | Key concepts |
|------|---------|-------------|
| `examples/simple-llm.json` | Minimal LLM call | input -> llm -> output |
| `examples/conditional-routing.json` | LLM classification + conditional branching | LLM JSON mode, conditional node, exclusive fan-in |
| `examples/stateful-loop.json` | Back-edge loop with state | replace + append reducers, state_writes, back-edge with condition |
| `examples/tool-calling.json` | LLM with tool_loop + MCP | tool_loop, tool_routing, MCP servers |
| `examples/superagent.json` | Autonomous agent with skills | superagent node, skills, overrides |
| `examples/foreach-parallel.json` | ForEach fan-out with inner graph | foreach node, inner graph, concurrency |
| `examples/multi-step-pipeline.json` | Full pipeline | classify -> merge -> route -> specialized responders |

Each file is valid Brockley graph JSON. Study them to see how the patterns in this document work together.

### Inline example: Simple LLM Call

Minimal graph that sends a prompt to an LLM and returns the response.

```json
{
  "name": "simple-llm",
  "namespace": "default",
  "version": 1,
  "status": "active",
  "nodes": [
    {
      "id": "input-1",
      "name": "Input",
      "type": "input",
      "input_ports": [],
      "output_ports": [
        {"name": "question", "schema": {"type": "string"}}
      ],
      "config": {}
    },
    {
      "id": "llm-1",
      "name": "Answer",
      "type": "llm",
      "input_ports": [
        {"name": "question", "schema": {"type": "string"}}
      ],
      "output_ports": [
        {"name": "response_text", "schema": {"type": "string"}}
      ],
      "config": {
        "provider": "openai",
        "model": "gpt-4o",
        "api_key": "${OPENAI_API_KEY}",
        "system_prompt": "You are a helpful assistant. Answer concisely.",
        "user_prompt": "{{input.question}}",
        "variables": [
          {"name": "question", "schema": {"type": "string"}}
        ],
        "response_format": "text",
        "temperature": 0.7,
        "max_tokens": 512
      }
    },
    {
      "id": "output-1",
      "name": "Output",
      "type": "output",
      "input_ports": [
        {"name": "answer", "schema": {"type": "string"}}
      ],
      "output_ports": [
        {"name": "answer", "schema": {"type": "string"}}
      ],
      "config": {}
    }
  ],
  "edges": [
    {"id": "e1", "source_node_id": "input-1", "source_port": "question", "target_node_id": "llm-1", "target_port": "question"},
    {"id": "e2", "source_node_id": "llm-1", "source_port": "response_text", "target_node_id": "output-1", "target_port": "answer"}
  ]
}
```

See the `examples/` directory for 6 more complete graphs covering conditional routing, stateful loops, tool calling, superagent, foreach, and multi-step pipelines.

```json
{
  "name": "conditional-routing",
  "namespace": "default",
  "version": 1,
  "status": "active",
  "nodes": [
    {
      "id": "input-1",
      "name": "Input",
      "type": "input",
      "input_ports": [],
      "output_ports": [{"name": "message", "schema": {"type": "string"}}],
      "config": {}
    },
    {
      "id": "classify",
      "name": "Classify",
      "type": "llm",
      "input_ports": [
        {"name": "message", "schema": {"type": "string"}}
      ],
      "output_ports": [
        {"name": "response", "schema": {"type": "object", "properties": {"category": {"type": "string"}}, "required": ["category"]}}
      ],
      "config": {
        "provider": "openai",
        "model": "gpt-4o-mini",
        "api_key": "${OPENAI_API_KEY}",
        "user_prompt": "Classify this message into one of: question, complaint, praise.\nMessage: {{input.message}}\nRespond as JSON.",
        "variables": [{"name": "message", "schema": {"type": "string"}}],
        "response_format": "json",
        "output_schema": {
          "type": "object",
          "properties": {"category": {"type": "string", "enum": ["question", "complaint", "praise"]}},
          "required": ["category"]
        },
        "temperature": 0.0
      }
    },
    {
      "id": "router",
      "name": "Route",
      "type": "conditional",
      "input_ports": [
        {"name": "value", "schema": {"type": "object", "properties": {"category": {"type": "string"}}, "required": ["category"]}}
      ],
      "output_ports": [
        {"name": "question", "schema": {"type": "object", "properties": {"category": {"type": "string"}}, "required": ["category"]}},
        {"name": "complaint", "schema": {"type": "object", "properties": {"category": {"type": "string"}}, "required": ["category"]}},
        {"name": "praise", "schema": {"type": "object", "properties": {"category": {"type": "string"}}, "required": ["category"]}}
      ],
      "config": {
        "branches": [
          {"label": "question", "condition": "input.value.category == \"question\""},
          {"label": "complaint", "condition": "input.value.category == \"complaint\""}
        ],
        "default_label": "praise"
      }
    },
    {
      "id": "handle-question",
      "name": "Handle Question",
      "type": "transform",
      "input_ports": [{"name": "value", "schema": {"type": "object", "properties": {"category": {"type": "string"}}, "required": ["category"]}}],
      "output_ports": [{"name": "result", "schema": {"type": "string"}}],
      "config": {"expressions": {"result": "\"Handling question: \" + input.value.category"}}
    },
    {
      "id": "handle-complaint",
      "name": "Handle Complaint",
      "type": "transform",
      "input_ports": [{"name": "value", "schema": {"type": "object", "properties": {"category": {"type": "string"}}, "required": ["category"]}}],
      "output_ports": [{"name": "result", "schema": {"type": "string"}}],
      "config": {"expressions": {"result": "\"Handling complaint: \" + input.value.category"}}
    },
    {
      "id": "handle-praise",
      "name": "Handle Praise",
      "type": "transform",
      "input_ports": [{"name": "value", "schema": {"type": "object", "properties": {"category": {"type": "string"}}, "required": ["category"]}}],
      "output_ports": [{"name": "result", "schema": {"type": "string"}}],
      "config": {"expressions": {"result": "\"Handling praise: \" + input.value.category"}}
    },
    {
      "id": "output-1",
      "name": "Output",
      "type": "output",
      "input_ports": [{"name": "result", "schema": {"type": "string"}}],
      "output_ports": [{"name": "result", "schema": {"type": "string"}}],
      "config": {}
    }
  ],
  "edges": [
    {"id": "e1", "source_node_id": "input-1", "source_port": "message", "target_node_id": "classify", "target_port": "message"},
    {"id": "e2", "source_node_id": "classify", "source_port": "response", "target_node_id": "router", "target_port": "value"},
    {"id": "e3", "source_node_id": "router", "source_port": "question", "target_node_id": "handle-question", "target_port": "value"},
    {"id": "e4", "source_node_id": "router", "source_port": "complaint", "target_node_id": "handle-complaint", "target_port": "value"},
    {"id": "e5", "source_node_id": "router", "source_port": "praise", "target_node_id": "handle-praise", "target_port": "value"},
    {"id": "e6", "source_node_id": "handle-question", "source_port": "result", "target_node_id": "output-1", "target_port": "result"},
    {"id": "e7", "source_node_id": "handle-complaint", "source_port": "result", "target_node_id": "output-1", "target_port": "result"},
    {"id": "e8", "source_node_id": "handle-praise", "source_port": "result", "target_node_id": "output-1", "target_port": "result"}
  ]
}
```

### Example 3: Stateful Loop with Back-Edge

A loop that processes items iteratively, using state to track progress.

```json
{
  "name": "stateful-loop",
  "namespace": "default",
  "version": 1,
  "status": "active",
  "state": {
    "fields": [
      {"name": "count", "schema": {"type": "number"}, "reducer": "replace", "initial": 0},
      {"name": "log", "schema": {"type": "array", "items": {"type": "string"}}, "reducer": "append", "initial": []}
    ]
  },
  "nodes": [
    {
      "id": "input-1",
      "name": "Input",
      "type": "input",
      "input_ports": [],
      "output_ports": [{"name": "data", "schema": {"type": "string"}}],
      "config": {}
    },
    {
      "id": "worker",
      "name": "Worker",
      "type": "transform",
      "input_ports": [{"name": "data", "schema": {"type": "string"}}],
      "output_ports": [
        {"name": "data", "schema": {"type": "string"}},
        {"name": "new_count", "schema": {"type": "number"}},
        {"name": "log_entry", "schema": {"type": "string"}}
      ],
      "state_writes": [
        {"state_field": "count", "port": "new_count"},
        {"state_field": "log", "port": "log_entry"}
      ],
      "config": {
        "expressions": {
          "data": "input.data",
          "new_count": "state.count + 1",
          "log_entry": "\"iteration-\" + (state.count + 1 | toString)"
        }
      }
    },
    {
      "id": "check",
      "name": "Check Done",
      "type": "conditional",
      "input_ports": [{"name": "value", "schema": {"type": "string"}}],
      "output_ports": [
        {"name": "continue", "schema": {"type": "string"}},
        {"name": "done", "schema": {"type": "string"}}
      ],
      "config": {
        "branches": [{"label": "continue", "condition": "state.count < 3"}],
        "default_label": "done"
      }
    },
    {
      "id": "output-1",
      "name": "Output",
      "type": "output",
      "input_ports": [{"name": "result", "schema": {"type": "string"}}],
      "output_ports": [{"name": "result", "schema": {"type": "string"}}],
      "config": {}
    }
  ],
  "edges": [
    {"id": "e1", "source_node_id": "input-1", "source_port": "data", "target_node_id": "worker", "target_port": "data"},
    {"id": "e2", "source_node_id": "worker", "source_port": "data", "target_node_id": "check", "target_port": "value"},
    {"id": "e3", "source_node_id": "check", "source_port": "continue", "target_node_id": "worker", "target_port": "data", "back_edge": true, "condition": "input.continue != null", "max_iterations": 10},
    {"id": "e4", "source_node_id": "check", "source_port": "done", "target_node_id": "output-1", "target_port": "result"}
  ]
}
```

### Example 4: LLM with Tool Loop

An LLM node that can call tools repeatedly until it has enough information.

```json
{
  "name": "tool-calling-agent",
  "namespace": "default",
  "version": 1,
  "status": "active",
  "nodes": [
    {
      "id": "input-1",
      "name": "Input",
      "type": "input",
      "input_ports": [],
      "output_ports": [{"name": "question", "schema": {"type": "string"}}],
      "config": {}
    },
    {
      "id": "agent",
      "name": "Agent",
      "type": "llm",
      "input_ports": [{"name": "question", "schema": {"type": "string"}}],
      "output_ports": [
        {"name": "response_text", "schema": {"type": "string"}},
        {"name": "finish_reason", "schema": {"type": "string"}},
        {"name": "total_tool_calls", "schema": {"type": "integer"}},
        {"name": "iterations", "schema": {"type": "integer"}}
      ],
      "config": {
        "provider": "anthropic",
        "model": "claude-sonnet-4-20250514",
        "api_key": "${ANTHROPIC_API_KEY}",
        "system_prompt": "You are a helpful assistant with access to tools. Use them to answer the user's question.",
        "user_prompt": "{{input.question}}",
        "variables": [{"name": "question", "schema": {"type": "string"}}],
        "response_format": "text",
        "tool_loop": true,
        "max_tool_calls": 10,
        "max_loop_iterations": 5,
        "tool_routing": {
          "web_search": {"mcp_url": "http://search-mcp:9090"},
          "calculator": {"mcp_url": "http://calc-mcp:9090"}
        }
      }
    },
    {
      "id": "output-1",
      "name": "Output",
      "type": "output",
      "input_ports": [
        {"name": "answer", "schema": {"type": "string"}},
        {"name": "tool_calls_used", "schema": {"type": "integer"}}
      ],
      "output_ports": [
        {"name": "answer", "schema": {"type": "string"}},
        {"name": "tool_calls_used", "schema": {"type": "integer"}}
      ],
      "config": {}
    }
  ],
  "edges": [
    {"id": "e1", "source_node_id": "input-1", "source_port": "question", "target_node_id": "agent", "target_port": "question"},
    {"id": "e2", "source_node_id": "agent", "source_port": "response_text", "target_node_id": "output-1", "target_port": "answer"},
    {"id": "e3", "source_node_id": "agent", "source_port": "total_tool_calls", "target_node_id": "output-1", "target_port": "tool_calls_used"}
  ]
}
```

### Example 5: Superagent

Full autonomous agent with MCP skills.

```json
{
  "name": "research-agent",
  "namespace": "default",
  "version": 1,
  "status": "active",
  "nodes": [
    {
      "id": "input-1",
      "name": "Input",
      "type": "input",
      "input_ports": [],
      "output_ports": [{"name": "topic", "schema": {"type": "string"}}],
      "config": {}
    },
    {
      "id": "agent",
      "name": "Research Agent",
      "type": "superagent",
      "input_ports": [{"name": "topic", "schema": {"type": "string"}}],
      "output_ports": [{"name": "report", "schema": {"type": "string"}}],
      "config": {
        "prompt": "Research the topic '{{input.topic}}' and produce a comprehensive report with citations.",
        "provider": "anthropic",
        "model": "claude-sonnet-4-20250514",
        "api_key": "${ANTHROPIC_API_KEY}",
        "max_iterations": 15,
        "max_total_tool_calls": 30,
        "timeout_seconds": 300,
        "skills": [
          {
            "name": "search",
            "description": "Web search for finding information",
            "mcp_url": "http://search-server:9090"
          },
          {
            "name": "reader",
            "description": "Read and extract content from web pages",
            "mcp_url": "http://reader-server:9090",
            "tools": ["read_page", "extract_text"]
          }
        ],
        "overrides": {
          "evaluator": {"disabled": true},
          "context_compaction": {
            "enabled": true,
            "context_window_limit": 100000,
            "compaction_threshold": 0.75
          }
        }
      }
    },
    {
      "id": "output-1",
      "name": "Output",
      "type": "output",
      "input_ports": [{"name": "report", "schema": {"type": "string"}}],
      "output_ports": [{"name": "report", "schema": {"type": "string"}}],
      "config": {}
    }
  ],
  "edges": [
    {"id": "e1", "source_node_id": "input-1", "source_port": "topic", "target_node_id": "agent", "target_port": "topic"},
    {"id": "e2", "source_node_id": "agent", "source_port": "report", "target_node_id": "output-1", "target_port": "report"}
  ]
}
```

### Example 6: Multi-Step Pipeline

A complex pipeline that classifies, transforms, branches, and assembles results.

```json
{
  "name": "support-pipeline",
  "namespace": "default",
  "version": 1,
  "status": "active",
  "nodes": [
    {
      "id": "input-1",
      "name": "Input",
      "type": "input",
      "input_ports": [],
      "output_ports": [
        {"name": "ticket", "schema": {"type": "object", "properties": {"subject": {"type": "string"}, "body": {"type": "string"}}, "required": ["subject", "body"]}}
      ],
      "config": {}
    },
    {
      "id": "classify",
      "name": "Classify",
      "type": "llm",
      "input_ports": [
        {"name": "ticket", "schema": {"type": "object", "properties": {"subject": {"type": "string"}, "body": {"type": "string"}}, "required": ["subject", "body"]}}
      ],
      "output_ports": [
        {"name": "response", "schema": {"type": "object", "properties": {"category": {"type": "string"}, "urgency": {"type": "string"}}, "required": ["category", "urgency"]}}
      ],
      "config": {
        "provider": "openai",
        "model": "gpt-4o-mini",
        "api_key": "${OPENAI_API_KEY}",
        "user_prompt": "Classify this ticket.\nSubject: {{input.ticket.subject}}\nBody: {{input.ticket.body}}\nReturn JSON with category (billing/technical/general) and urgency (high/medium/low).",
        "variables": [{"name": "ticket", "schema": {"type": "object", "properties": {"subject": {"type": "string"}, "body": {"type": "string"}}, "required": ["subject", "body"]}}],
        "response_format": "json",
        "output_schema": {
          "type": "object",
          "properties": {
            "category": {"type": "string", "enum": ["billing", "technical", "general"]},
            "urgency": {"type": "string", "enum": ["high", "medium", "low"]}
          },
          "required": ["category", "urgency"]
        },
        "temperature": 0.0
      }
    },
    {
      "id": "merge",
      "name": "Merge",
      "type": "transform",
      "input_ports": [
        {"name": "ticket", "schema": {"type": "object", "properties": {"subject": {"type": "string"}, "body": {"type": "string"}}, "required": ["subject", "body"]}},
        {"name": "classification", "schema": {"type": "object", "properties": {"category": {"type": "string"}, "urgency": {"type": "string"}}, "required": ["category", "urgency"]}}
      ],
      "output_ports": [
        {"name": "enriched", "schema": {"type": "object", "properties": {"subject": {"type": "string"}, "body": {"type": "string"}, "category": {"type": "string"}, "urgency": {"type": "string"}}, "required": ["subject", "body", "category", "urgency"]}}
      ],
      "config": {
        "expressions": {
          "enriched": "{subject: input.ticket.subject, body: input.ticket.body, category: input.classification.category, urgency: input.classification.urgency}"
        }
      }
    },
    {
      "id": "route",
      "name": "Route by Category",
      "type": "conditional",
      "input_ports": [
        {"name": "value", "schema": {"type": "object", "properties": {"subject": {"type": "string"}, "body": {"type": "string"}, "category": {"type": "string"}, "urgency": {"type": "string"}}, "required": ["category"]}}
      ],
      "output_ports": [
        {"name": "billing", "schema": {"type": "object", "properties": {"subject": {"type": "string"}, "body": {"type": "string"}, "category": {"type": "string"}, "urgency": {"type": "string"}}, "required": ["category"]}},
        {"name": "technical", "schema": {"type": "object", "properties": {"subject": {"type": "string"}, "body": {"type": "string"}, "category": {"type": "string"}, "urgency": {"type": "string"}}, "required": ["category"]}},
        {"name": "general", "schema": {"type": "object", "properties": {"subject": {"type": "string"}, "body": {"type": "string"}, "category": {"type": "string"}, "urgency": {"type": "string"}}, "required": ["category"]}}
      ],
      "config": {
        "branches": [
          {"label": "billing", "condition": "input.value.category == \"billing\""},
          {"label": "technical", "condition": "input.value.category == \"technical\""}
        ],
        "default_label": "general"
      }
    },
    {
      "id": "billing-reply",
      "name": "Billing Reply",
      "type": "llm",
      "input_ports": [{"name": "data", "schema": {"type": "object", "properties": {"subject": {"type": "string"}, "body": {"type": "string"}, "category": {"type": "string"}, "urgency": {"type": "string"}}, "required": ["subject", "body"]}}],
      "output_ports": [{"name": "response_text", "schema": {"type": "string"}}],
      "config": {
        "provider": "openai",
        "model": "gpt-4o",
        "api_key": "${OPENAI_API_KEY}",
        "system_prompt": "You are a billing support agent. Be empathetic.",
        "user_prompt": "Subject: {{input.data.subject}}\n{{input.data.body}}\nUrgency: {{input.data.urgency}}\n\nDraft a reply:",
        "variables": [{"name": "data", "schema": {"type": "object", "properties": {"subject": {"type": "string"}, "body": {"type": "string"}, "urgency": {"type": "string"}}, "required": ["subject", "body"]}}],
        "response_format": "text",
        "max_tokens": 512
      }
    },
    {
      "id": "technical-reply",
      "name": "Technical Reply",
      "type": "llm",
      "input_ports": [{"name": "data", "schema": {"type": "object", "properties": {"subject": {"type": "string"}, "body": {"type": "string"}, "category": {"type": "string"}, "urgency": {"type": "string"}}, "required": ["subject", "body"]}}],
      "output_ports": [{"name": "response_text", "schema": {"type": "string"}}],
      "config": {
        "provider": "openai",
        "model": "gpt-4o",
        "api_key": "${OPENAI_API_KEY}",
        "system_prompt": "You are a technical support engineer.",
        "user_prompt": "Subject: {{input.data.subject}}\n{{input.data.body}}\n\nProvide troubleshooting steps:",
        "variables": [{"name": "data", "schema": {"type": "object", "properties": {"subject": {"type": "string"}, "body": {"type": "string"}}, "required": ["subject", "body"]}}],
        "response_format": "text",
        "max_tokens": 512
      }
    },
    {
      "id": "general-reply",
      "name": "General Reply",
      "type": "llm",
      "input_ports": [{"name": "data", "schema": {"type": "object", "properties": {"subject": {"type": "string"}, "body": {"type": "string"}, "category": {"type": "string"}, "urgency": {"type": "string"}}, "required": ["subject", "body"]}}],
      "output_ports": [{"name": "response_text", "schema": {"type": "string"}}],
      "config": {
        "provider": "openai",
        "model": "gpt-4o",
        "api_key": "${OPENAI_API_KEY}",
        "system_prompt": "You are a friendly support agent.",
        "user_prompt": "Subject: {{input.data.subject}}\n{{input.data.body}}\n\nDraft a helpful reply:",
        "variables": [{"name": "data", "schema": {"type": "object", "properties": {"subject": {"type": "string"}, "body": {"type": "string"}}, "required": ["subject", "body"]}}],
        "response_format": "text",
        "max_tokens": 512
      }
    },
    {
      "id": "output-1",
      "name": "Output",
      "type": "output",
      "input_ports": [{"name": "reply", "schema": {"type": "string"}}],
      "output_ports": [{"name": "reply", "schema": {"type": "string"}}],
      "config": {}
    }
  ],
  "edges": [
    {"id": "e1", "source_node_id": "input-1", "source_port": "ticket", "target_node_id": "classify", "target_port": "ticket"},
    {"id": "e2", "source_node_id": "classify", "source_port": "response", "target_node_id": "merge", "target_port": "classification"},
    {"id": "e3", "source_node_id": "input-1", "source_port": "ticket", "target_node_id": "merge", "target_port": "ticket"},
    {"id": "e4", "source_node_id": "merge", "source_port": "enriched", "target_node_id": "route", "target_port": "value"},
    {"id": "e5", "source_node_id": "route", "source_port": "billing", "target_node_id": "billing-reply", "target_port": "data"},
    {"id": "e6", "source_node_id": "route", "source_port": "technical", "target_node_id": "technical-reply", "target_port": "data"},
    {"id": "e7", "source_node_id": "route", "source_port": "general", "target_node_id": "general-reply", "target_port": "data"},
    {"id": "e8", "source_node_id": "billing-reply", "source_port": "response_text", "target_node_id": "output-1", "target_port": "reply"},
    {"id": "e9", "source_node_id": "technical-reply", "source_port": "response_text", "target_node_id": "output-1", "target_port": "reply"},
    {"id": "e10", "source_node_id": "general-reply", "source_port": "response_text", "target_node_id": "output-1", "target_port": "reply"}
  ]
}
```

---

## 13. Common Mistakes

### Schema violations
- **Bare `{"type": "object"}`** -- always add `properties`.
- **Bare `{"type": "array"}`** -- always add `items`.
- **Missing `type` field** on a schema. Every schema needs `type` (or `oneOf`/`anyOf`/`enum`).

### Port wiring
- **Unwired required port** -- every required input must have an edge, state_read, or default.
- **Port name mismatch** -- edge `source_port`/`target_port` must match exact port names on the nodes.
- **Multiple edges to the same port** from non-exclusive branches -- only works with conditional branches.

### Loops
- **Cycle without back-edge** -- if nodes form a loop, one edge in the cycle must be `back_edge: true`.
- **Back-edge without condition or max_iterations** -- both are required.
- **max_iterations = 0** -- must be > 0.

### LLM nodes
- **Missing `variables`** -- the `variables` array must declare all template variables used in prompts.
- **`response_format: "json"` without `output_schema`** -- the schema is required for JSON mode.
- **Using `response_text` output port with JSON mode** -- JSON mode uses `response` port name, not `response_text`.
- **Using `response` output port with text mode** -- text mode uses `response_text`.

### Tool loop
- **`tool_loop: true` without `tool_routing`** -- you need to tell the engine where to send tool calls.
- **Tool in `tools` but not in `tool_routing`** -- the LLM will try to call it but it will fail.

### Superagent
- **Missing `api_key` and `api_key_ref`** -- at least one is required.
- **Skills with neither `mcp_url` nor `api_tool_id`** -- each skill needs exactly one.
- **Empty skills array** -- at least one skill is required.
- **`shared_memory.enabled` without `_superagent_memory` state field** -- the graph must define this state field with merge reducer.

### Conditional nodes
- **Missing output port for a branch label** -- each branch `label` and the `default_label` must have a corresponding output port.
- **Conditions that don't return boolean** -- branch conditions must evaluate to true/false.

### ForEach
- **Inner graph missing `item` and `index` output ports on its input node** -- these are required.
- **Inner graph missing its own `input` node** -- the inner graph is a complete graph, it needs its own input/output nodes.

### State
- **`append` reducer on non-array schema** -- append only works with `type: "array"`.
- **`merge` reducer on non-object schema** -- merge only works with `type: "object"`.
- **Referencing state field that doesn't exist** -- state_reads/writes must reference fields defined in `state.fields`.

### General
- **Node IDs not unique** -- every node must have a unique `id` within the graph.
- **Edge IDs not unique** -- every edge must have a unique `id` within the graph.
- **No `input` node** -- the graph must have at least one node of type `input`.
- **Self-referencing edge without back_edge** -- add `back_edge: true`.

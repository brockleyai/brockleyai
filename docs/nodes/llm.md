# LLM Node

**Type:** `llm`

The LLM node calls a language model provider and returns the response. It supports multiple providers, template-based prompt construction, text and JSON response modes, structured output validation with automatic retries, tool calling, and an autonomous tool loop.

## Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `provider` | string | Yes | Provider identifier: `openai`, `anthropic`, `google`, `openrouter`, `bedrock`, or `custom`. |
| `model` | string | Yes | Model name (e.g., `gpt-4o`, `claude-sonnet-4-20250514`, `gemini-2.0-flash`). |
| `api_key` | string | No | Inline API key. Takes priority over `api_key_ref`. Masked in API responses. |
| `api_key_ref` | string | No | Secret reference for the API key. At least one of `api_key` or `api_key_ref` is required. |
| `base_url` | string | No | Custom base URL for the provider API. Useful for proxies, self-hosted models, or `custom` provider. |
| `bedrock_region` | string | No | AWS region for the `bedrock` provider (e.g., `us-east-1`). |
| `system_prompt` | string | No | System prompt template. Supports `{{expression}}` interpolation. |
| `user_prompt` | string | Yes | User prompt template. Supports `{{expression}}` interpolation. |
| `messages` | PromptMessage[] | No | Explicit message chain. When set, takes priority over `system_prompt`/`user_prompt`. Each message has a `role` (`system`, `user`, `assistant`) and `content` (template string). |
| `variables` | TemplateVar[] | No | Declares template variables. Each variable becomes an input port on the node. |
| `temperature` | float | No | Sampling temperature (0.0 to 2.0). Provider default if omitted. |
| `max_tokens` | integer | No | Maximum tokens in the response. Provider default if omitted. |
| `response_format` | string | Yes | `"text"` or `"json"`. Controls output parsing and output port names. |
| `output_schema` | JSON Schema | No | When `response_format` is `"json"`, defines the expected JSON structure. |
| `validate_output` | boolean | No | When `true`, validates the LLM's JSON response against `output_schema`. If validation fails, the node retries automatically. |
| `max_validation_retries` | integer | No | Maximum validation retry attempts before failing. Default: 2. |
| `extra_headers` | map[string]string | No | Additional HTTP headers sent to the provider API. |

### Messages Chain

The `messages` field lets you define a full conversation chain instead of a single system + user prompt pair. Each message's `content` supports template expressions:

```json
{
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Previous question: {{input.prev_question}}"},
    {"role": "assistant", "content": "Previous answer: {{input.prev_answer}}"},
    {"role": "user", "content": "Follow-up: {{input.question}}"}
  ]
}
```

When `messages` is set, `system_prompt` and `user_prompt` are ignored.

### Variables and Input Port Auto-Generation

Each entry in the `variables` array creates an input port on the node:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Variable name. Used in templates as `{{input.name}}` and becomes the input port name. |
| `schema` | JSON Schema | Yes | Type of the variable, used for port validation. |
| `description` | string | No | Human-readable description. |

If you declare `variables: [{"name": "query", "schema": {"type": "string"}}]`, the node gets an input port named `query`. Upstream edges can connect to this port, and the value is accessible in templates as `{{input.query}}`.

## Output Ports

### Text Mode (`response_format: "text"` or omitted)

| Port | Type | Description |
|------|------|-------------|
| `response_text` | string | The raw text response from the LLM. |

### JSON Mode (`response_format: "json"`)

| Port | Type | Description |
|------|------|-------------|
| `response` | object/array | The parsed JSON response from the LLM. |

In JSON mode, the engine parses the LLM's response as JSON before passing it downstream. If the response is not valid JSON, the node fails.

### Tool Loop Output Ports

When `tool_loop` is enabled, the node exposes additional output ports:

| Port | Type | Description |
|------|------|-------------|
| `response_text` | string | Final text response after all tool calls resolve. |
| `messages` | array | Full conversation history including all tool calls and results. |
| `tool_call_history` | array | Array of all tool calls made, with their results. |
| `iterations` | integer | Number of loop iterations completed. |
| `total_tool_calls` | integer | Total number of tool calls executed. |
| `finish_reason` | string | Why the loop ended: `stop`, `max_tool_calls`, or `max_iterations`. |

## Template Expressions

Both `system_prompt` and `user_prompt` (and each message in `messages`) support template expressions using `{{...}}` syntax. Inside the braces, you can use the full [expression language](../expressions/overview.md).

The template context provides three namespaces:

- **`input`** -- values received on the node's input ports (corresponding to declared `variables`).
- **`state`** -- current values of all graph state fields.
- **`meta`** -- execution metadata (`meta.execution_id`, `meta.node_id`, `meta.iteration`, etc.).

```
Analyze the following text: {{input.query}}

The user's language preference is: {{input.language ?? "English"}}

Previous conversation:
{{state.conversation_history | last}}
```

Templates also support `{{#if}}`, `{{#each}}`, and `{{raw}}` block directives. See [Template Syntax](../expressions/templates.md) for details.

## Auto System Prompt for JSON Mode

When `response_format` is `"json"` and `output_schema` is provided, the engine automatically appends an instruction to the system prompt:

```
You MUST respond with valid JSON matching this schema:
<the output_schema JSON>
```

This happens transparently -- your `system_prompt` can focus on the task, and the schema instruction is appended automatically.

## Structured Output Validation

When `validate_output` is `true` and `output_schema` is set, the engine validates the LLM's JSON response against the schema after parsing. If validation fails:

1. The engine retries the LLM call with the validation error appended to the prompt.
2. This repeats up to `max_validation_retries` times (default: 2).
3. If all retries fail, the node fails with a validation error.

This is useful for enforcing strict output contracts without manual validation logic.

## Tool Calling

The LLM node supports tool calling, allowing the model to invoke external tools (via MCP servers or [API tool definitions](api-tool.md)) during generation.

### Mode 1: No Tools (Default)

The LLM generates a text or JSON response with no tool access.

### Mode 2: Tools Without Loop

Set `tools` and optionally `tool_choice`, but leave `tool_loop` as `false` (default). The LLM may request tool calls in its response, but the engine does not execute them automatically. The tool calls are returned in the output for downstream nodes to handle.

### Mode 3: Full Tool Loop

Set `tool_loop: true`. The engine enters an autonomous loop:

1. Send the prompt (with tools) to the LLM.
2. If the LLM requests tool calls, execute them via MCP servers or API endpoints.
3. Append tool results to the conversation and call the LLM again.
4. Repeat until the LLM responds without tool calls, or a safety limit is reached.

### Tool Loop Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `tools` | LLMToolDefinition[] | No | Statically defined tools available to the LLM. |
| `tool_choice` | string | No | `"auto"` (default), `"none"`, `"required"`, or a specific tool name. |
| `tool_loop` | boolean | No | Enable the autonomous tool loop. Default: `false`. |
| `max_tool_calls` | integer | No | Maximum total tool calls across all iterations. Default: 25. |
| `max_loop_iterations` | integer | No | Maximum LLM round-trips in the loop. Default: 10. |
| `tool_routing` | map[string]ToolRoute | No | Maps tool names to MCP server routes or API endpoints. |
| `tool_routing_from_state` | string | No | State field name resolving to a tool routing map. |
| `tool_routing_from_input` | boolean | No | When `true`, reads tool routing from node input. |
| `messages_from_state` | string | No | State field containing conversation history (array of messages) to prepend. |
| `api_tools` | APIToolRef[] | No | API tool references. Each ref selects an endpoint and auto-derives tool schema + routing. |

### Tool Routing

When `tool_loop` is enabled, the engine needs to know which MCP server or API endpoint handles each tool:

```json
{
  "config": {
    "tool_loop": true,
    "tool_routing": {
      "search": {
        "mcp_url": "http://localhost:8080/mcp",
        "timeout_seconds": 30
      },
      "create_ticket": {
        "mcp_url": "http://localhost:8081/mcp",
        "headers": [{"name": "Authorization", "value": "Bearer ${TICKET_API_KEY}"}]
      }
    }
  }
}
```

Each `ToolRoute` supports MCP routing (`mcp_url`, `mcp_transport`), API endpoint routing (`api_tool_id`, `api_endpoint`), custom headers, per-call timeout, and compacted discovery mode (`compacted: true`).

Tool routing can also be loaded dynamically from graph state (`tool_routing_from_state`) or node input (`tool_routing_from_input`).

### API Tool References

Instead of (or in addition to) defining MCP tool routes, you can reference endpoints from [API tool definitions](api-tool.md):

```json
{
  "config": {
    "api_tools": [
      {"api_tool_id": "stripe-api", "endpoint": "create_charge"},
      {"api_tool_id": "stripe-api", "endpoint": "get_customer", "tool_name": "lookup_customer"}
    ]
  }
}
```

Each `APIToolRef` auto-derives the tool definition (name, description, input schema) and routing from the API tool definition.

### Messages From State

The `messages_from_state` field specifies a graph state field containing an array of message objects. These messages are prepended to the conversation sent to the LLM, enabling multi-turn conversations that persist across loop iterations or graph executions.

```json
{
  "config": {
    "messages_from_state": "conversation_history",
    "tool_loop": true
  }
}
```

The state field must use `reducer: "append"` with a schema of `type: "array"`.

## API Key Handling

You can provide an API key in two ways:

1. **Inline (`api_key`)** -- stored directly in the node config. Simplest option for quick setups.
2. **Secret reference (`api_key_ref`)** -- resolved via the secret store at runtime.

If both are set, `api_key` takes priority.

**Masking:** When you retrieve a graph via the API, `api_key` values are masked. Keys of 8+ characters show as first 4 + `"..."` + last 4 (e.g., `"sk-or...ab12"`). Shorter keys show as `"****"`. When updating a graph, you can submit the masked value and the server preserves the original key.

## Common Patterns

### Classify-then-route

Use JSON mode with a structured schema to classify input, then wire the output to a [conditional node](conditional.md):

```json
{
  "config": {
    "response_format": "json",
    "output_schema": {
      "type": "object",
      "properties": {
        "category": {"type": "string", "enum": ["billing", "support", "sales"]},
        "confidence": {"type": "number"}
      },
      "required": ["category", "confidence"]
    },
    "validate_output": true
  }
}
```

### Multi-turn conversation with state

Store the conversation in graph state and feed it back on each iteration:

```json
{
  "config": {
    "messages_from_state": "chat_history",
    "tool_loop": true
  },
  "state_writes": [
    {"state_field": "chat_history", "port": "messages"}
  ]
}
```

### Cost-tiered model selection

Use a conditional node upstream to route simple queries to a cheap model and complex ones to a powerful model. Both LLM nodes connect to the same output via [exclusive fan-in](conditional.md#rejoining-with-exclusive-fan-in).

### Chained extraction

Run one LLM in JSON mode to extract structured data, then pipe that data into a second LLM for summarization in text mode.

## Examples

### Text Mode: Simple Question Answering

```json
{
  "id": "ask-llm",
  "name": "Ask LLM",
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
    "api_key_ref": "openai-key",
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

### JSON Mode: Structured Extraction with Validation

```json
{
  "id": "extract-entities",
  "name": "Extract Entities",
  "type": "llm",
  "input_ports": [
    {"name": "text", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {"name": "response", "schema": {"type": "object"}}
  ],
  "config": {
    "provider": "anthropic",
    "model": "claude-sonnet-4-20250514",
    "api_key_ref": "anthropic-key",
    "system_prompt": "Extract all named entities from the text.",
    "user_prompt": "Text: {{input.text}}",
    "variables": [
      {"name": "text", "schema": {"type": "string"}}
    ],
    "response_format": "json",
    "output_schema": {
      "type": "object",
      "properties": {
        "entities": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "name": {"type": "string"},
              "type": {"type": "string"}
            },
            "required": ["name", "type"]
          }
        }
      },
      "required": ["entities"]
    },
    "validate_output": true,
    "max_validation_retries": 3
  }
}
```

### Tool Loop: Agent with MCP Tools

```json
{
  "id": "support-agent",
  "name": "Support Agent",
  "type": "llm",
  "input_ports": [
    {"name": "user_message", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {"name": "response_text", "schema": {"type": "string"}},
    {"name": "messages", "schema": {"type": "array", "items": {"type": "object"}}},
    {"name": "finish_reason", "schema": {"type": "string"}}
  ],
  "config": {
    "provider": "anthropic",
    "model": "claude-sonnet-4-20250514",
    "api_key_ref": "anthropic-key",
    "system_prompt": "You are a helpful support agent. Use tools to look up information before answering.",
    "user_prompt": "{{input.user_message}}",
    "variables": [
      {"name": "user_message", "schema": {"type": "string"}}
    ],
    "response_format": "text",
    "tool_loop": true,
    "max_tool_calls": 10,
    "max_loop_iterations": 5,
    "tool_routing": {
      "search_kb": {
        "mcp_url": "http://localhost:9000/mcp"
      },
      "get_order_status": {
        "mcp_url": "http://localhost:9001/mcp"
      }
    }
  }
}
```

## See Also

- [Ports and Typing](../concepts/ports-and-typing.md) -- how input/output ports and schemas work
- [Providers Overview](../providers/overview.md) -- how providers work, secret resolution, supported providers
- [Expression Language Overview](../expressions/overview.md) -- template syntax and available operations
- [Template Syntax](../expressions/templates.md) -- `#if`, `#each`, `raw` block directives
- [Tool Node](tool.md) -- standalone MCP tool calls
- [API Tool Node](api-tool.md) -- standalone REST API calls
- [API Tools Guide](../guides/api-tools.md) -- API tool definitions and LLM integration
- [Tool Calling Guide](../guides/tool-calling.md) -- detailed tool calling patterns
- [Conditional Node](conditional.md) -- routing based on LLM output
- [Superagent Node](superagent.md) -- for tasks needing autonomous multi-step reasoning
- [Data Model: LLM Node Config](../specs/data-model.md) -- complete field reference

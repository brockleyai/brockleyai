# Anthropic Provider

Config value: `"anthropic"`

The Anthropic provider uses the Messages API (`/v1/messages`).

## Authentication

- **Header:** `x-api-key: <api_key>`
- **Version header:** `anthropic-version: 2023-06-01` (set automatically)
- **API Key:** Get one from [console.anthropic.com](https://console.anthropic.com)

Set the secret via environment variable:

```bash
export BROCKLEY_SECRET_ANTHROPIC_KEY="sk-ant-..."
```

Then reference it in your node config with `"api_key_ref": "anthropic-key"`.

## Base URL

Default: `https://api.anthropic.com`

Override with `base_url` in the node config for proxied or self-hosted deployments.

## Models

| Model | Context Window | Notes |
|-------|---------------|-------|
| `claude-sonnet-4-20250514` | 200k | Best balance of speed and quality |
| `claude-opus-4-20250514` | 200k | Most capable |
| `claude-haiku-3-20250414` | 200k | Fastest, most affordable |

Any model string the Anthropic API accepts works in Brockley.

## Max Tokens

Anthropic requires `max_tokens` on every request. If you do not set it in your node config, Brockley defaults to `4096`. Set it explicitly for longer outputs:

```json
{
  "max_tokens": 8192
}
```

## Structured Output (JSON Mode)

Anthropic does not have a native JSON mode parameter. When `response_format` is `"json"`:

- If `output_schema` is provided, the provider appends to the system prompt: `"Respond with valid JSON matching this schema: <schema>"`
- If no schema is provided, the provider appends: `"Respond with valid JSON."`

This works well in practice. Claude models follow schema instructions reliably.

```json
{
  "response_format": "json",
  "output_schema": {
    "type": "object",
    "properties": {
      "summary": {"type": "string"},
      "insights": {"type": "array", "items": {"type": "string"}}
    }
  },
  "max_tokens": 4096
}
```

## Streaming

Streaming is supported. The provider uses Anthropic's SSE format with `event: content_block_delta` for token delivery. Usage data arrives in the `message_start` and `message_delta` events.

## Tool Calling

Anthropic supports tool use. The provider translates Brockley's tool definitions to Anthropic's format:

- `tool_choice: "auto"` maps to `{"type": "auto"}`
- `tool_choice: "required"` maps to `{"type": "any"}`
- `tool_choice: "none"` removes tools from the request
- A specific tool name maps to `{"type": "tool", "name": "..."}`

Tool results are sent as `tool_result` content blocks on `user` messages, following Anthropic's alternating-role convention.

## Finish Reason Mapping

Anthropic uses different stop reason strings. The provider normalizes them:

| Anthropic | Brockley |
|-----------|----------|
| `end_turn` | `stop` |
| `tool_use` | `tool_calls` |
| `max_tokens` | `length` |

## Example Node Config

```json
{
  "config": {
    "provider": "anthropic",
    "model": "claude-sonnet-4-20250514",
    "api_key_ref": "anthropic-key",
    "system_prompt": "You are an expert data analyst.",
    "user_prompt": "Analyze this dataset:\n{{input.data | json}}",
    "variables": [
      {"name": "data", "schema": {"type": "array", "items": {"type": "object", "properties": {"value": {"type": "number"}}, "required": ["value"]}}}
    ],
    "response_format": "json",
    "output_schema": {
      "type": "object",
      "properties": {
        "summary": {"type": "string"},
        "insights": {"type": "array", "items": {"type": "string"}}
      }
    },
    "max_tokens": 4096
  }
}
```

## See Also

- [LLM Node Reference](../nodes/llm.md) -- full LLM node configuration and output modes
- [Providers Overview](overview.md) -- how providers work, secret resolution
- [Provider Interface](provider-interface.md) -- Complete/Stream methods, error codes
- [Supported Providers](supported.md) -- comparison of all providers
- [Tool Calling Guide](../guides/tool-calling.md) -- using tools with LLM nodes

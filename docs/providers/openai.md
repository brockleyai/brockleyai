# OpenAI Provider

Config value: `"openai"`

The OpenAI provider uses the Chat Completions API (`/v1/chat/completions`).

## Authentication

- **Header:** `Authorization: Bearer <api_key>`
- **API Key:** Get one from [platform.openai.com](https://platform.openai.com)

Set the secret via environment variable:

```bash
export BROCKLEY_SECRET_OPENAI_KEY="sk-..."
```

Then reference it in your node config with `"api_key_ref": "openai-key"`.

## Base URL

Default: `https://api.openai.com/v1`

Override with `base_url` in the node config to point at Azure OpenAI, local proxies, or any OpenAI-compatible API.

## Models

| Model | Context Window | Notes |
|-------|---------------|-------|
| `gpt-4o` | 128k | Flagship multimodal model |
| `gpt-4o-mini` | 128k | Cost-efficient, fast |
| `gpt-4-turbo` | 128k | Previous generation |
| `o1` | 200k | Reasoning model |
| `o3-mini` | 200k | Compact reasoning model |

Check [OpenAI models](https://platform.openai.com/docs/models) for the full list. Any model string the API accepts works in Brockley.

## Structured Output (JSON Mode)

OpenAI natively supports JSON mode. Set `response_format` to `"json"` in your node config:

```json
{
  "response_format": "json",
  "output_schema": {
    "type": "object",
    "properties": {
      "intent": {"type": "string"},
      "confidence": {"type": "number"}
    },
    "required": ["intent", "confidence"]
  }
}
```

The provider sends `{"response_format": {"type": "json_object"}}` to the API. Brockley also appends the output schema to the system prompt for schema-guided generation.

## Streaming

Streaming is supported. When you invoke with `mode: "stream"`, the provider uses OpenAI's SSE streaming format. Tokens arrive as `llm_token` events on the execution stream endpoint.

## Rate Limiting

Use client-side rate limiting to stay within your OpenAI quota:

```go
provider := NewRateLimitedProvider(openaiProvider, RateLimitConfig{
    RequestsPerMinute: 100,
})
```

The provider also returns structured `ProviderError` values with code `rate_limited` when the API returns 429. These are automatically retried if you wrap the provider with `NewRetryableProvider`.

## Tool Calling

OpenAI supports function calling. Define tools on your LLM node and enable the tool loop:

```json
{
  "provider": "openai",
  "model": "gpt-4o",
  "tool_loop": true,
  "tools": [
    {
      "name": "search",
      "description": "Search the knowledge base",
      "parameters": {
        "type": "object",
        "properties": {
          "query": {"type": "string"}
        },
        "required": ["query"]
      }
    }
  ]
}
```

The provider maps `tool_choice` values (`auto`, `none`, `required`, or a specific tool name) to the OpenAI format.

## Example Node Config

```json
{
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
    "max_tokens": 2048
  }
}
```

## See Also

- [LLM Node Reference](../nodes/llm.md) -- full LLM node configuration and output modes
- [Providers Overview](overview.md) -- how providers work, secret resolution
- [Provider Interface](provider-interface.md) -- Complete/Stream methods, error codes
- [Supported Providers](supported.md) -- comparison of all providers
- [Tool Calling Guide](../guides/tool-calling.md) -- using tools with LLM nodes

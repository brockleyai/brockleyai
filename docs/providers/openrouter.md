# OpenRouter Provider

Config value: `"openrouter"`

OpenRouter provides access to hundreds of models from multiple providers through a single OpenAI-compatible API.

## Authentication

- **Header:** `Authorization: Bearer <api_key>`
- **Additional header:** `HTTP-Referer: https://brockley.ai` (set automatically by the provider)
- **API Key:** Get one from [openrouter.ai](https://openrouter.ai)

Set the secret via environment variable:

```bash
export BROCKLEY_SECRET_OPENROUTER_KEY="sk-or-..."
```

Then reference it in your node config with `"api_key_ref": "openrouter-key"`.

## Base URL

Default: `https://openrouter.ai/api/v1`

## How It Works

The OpenRouter provider wraps the OpenAI provider internally. It sends requests to the OpenRouter API using the same OpenAI Chat Completions format (`/chat/completions`). The only differences are:

1. The base URL points to OpenRouter
2. An `HTTP-Referer` header is added automatically

This means all OpenAI features (JSON mode, tool calling, streaming) work identically through OpenRouter.

## Model Naming

OpenRouter uses a `provider/model` naming convention:

| Model | OpenRouter Name |
|-------|----------------|
| GPT-4o | `openai/gpt-4o` |
| Claude Sonnet | `anthropic/claude-sonnet-4-20250514` |
| Gemini Flash | `google/gemini-2.0-flash` |
| Llama 3.1 70B | `meta-llama/llama-3.1-70b-instruct` |
| Mixtral 8x7B | `mistralai/mixtral-8x7b-instruct` |

Check [openrouter.ai/models](https://openrouter.ai/models) for the full list. Use the exact model string from their catalog.

## Structured Output (JSON Mode)

JSON mode works the same as OpenAI since OpenRouter is OpenAI-compatible. The `response_format` parameter is forwarded to the underlying model.

Not all models behind OpenRouter support JSON mode. Check the model card on OpenRouter's site.

## Streaming

Streaming works identically to OpenAI. The provider name in usage metrics is reported as `"openrouter"`.

## Tool Calling

Tool calling is supported for models that support it. The OpenAI tool calling format is used. Check the individual model's capabilities on OpenRouter's model page.

## When to Use OpenRouter

- **Multi-provider access:** One API key for OpenAI, Anthropic, Google, Meta, Mistral, and more
- **Cost routing:** OpenRouter can route to the cheapest provider for a given model
- **Fallback:** If one provider is down, OpenRouter can route to alternatives
- **Model exploration:** Try models from different providers without separate API keys

## Example Node Config

```json
{
  "config": {
    "provider": "openrouter",
    "model": "anthropic/claude-sonnet-4-20250514",
    "api_key_ref": "openrouter-key",
    "system_prompt": "You are a code reviewer.",
    "user_prompt": "Review this code:\n```\n{{input.code}}\n```",
    "variables": [
      {"name": "code", "schema": {"type": "string"}}
    ],
    "response_format": "text",
    "temperature": 0.2
  }
}
```

## See Also

- [LLM Node Reference](../nodes/llm.md) -- full LLM node configuration and output modes
- [Providers Overview](overview.md) -- how providers work, secret resolution
- [Provider Interface](provider-interface.md) -- Complete/Stream methods, error codes
- [Supported Providers](supported.md) -- comparison of all providers

# Google (Gemini) Provider

Config value: `"google"`

The Google provider uses the Gemini API (`/v1beta/models/<model>:generateContent`).

## Authentication

- **Header:** `x-goog-api-key: <api_key>`
- **API Key:** Get one from [aistudio.google.com](https://aistudio.google.com)

Set the secret via environment variable:

```bash
export BROCKLEY_SECRET_GOOGLE_GEMINI_KEY="AIza..."
```

Then reference it in your node config with `"api_key_ref": "google-gemini-key"`.

## Base URL

Default: `https://generativelanguage.googleapis.com/v1beta`

Override with `base_url` for Vertex AI or other Google endpoints.

## Models

| Model | Context Window | Notes |
|-------|---------------|-------|
| `gemini-2.0-flash` | 1M | Fast, cost-effective |
| `gemini-2.0-pro` | 1M | More capable |
| `gemini-1.5-pro` | 2M | Long context |
| `gemini-1.5-flash` | 1M | Fast with long context |

Any model string the Gemini API accepts works in Brockley.

## Request Structure

Gemini uses a different API structure than OpenAI/Anthropic:

- System prompts are sent via the `systemInstruction` field
- User messages are in the `contents` array
- Generation parameters (`temperature`, `maxOutputTokens`) go in `generationConfig`
- The `assistant` role is mapped to `model`

## Structured Output (JSON Mode)

Gemini supports JSON mode natively via `responseMimeType`. When `response_format` is `"json"`, the provider sets `"responseMimeType": "application/json"` in `generationConfig`.

```json
{
  "response_format": "json",
  "output_schema": {
    "type": "object",
    "properties": {
      "category": {"type": "string"},
      "score": {"type": "number"}
    },
    "required": ["category", "score"]
  }
}
```

## Streaming

Streaming is supported. The provider calls the `streamGenerateContent` endpoint with `?alt=sse`. Tokens arrive as SSE `data:` lines containing Gemini response objects.

## Tool Calling

Gemini supports function calling. The provider translates Brockley tool definitions to Gemini's `functionDeclarations` format. Tool results are sent back as `functionResponse` parts.

## Finish Reason Mapping

| Gemini | Brockley |
|--------|----------|
| `STOP` | `stop` |
| `MAX_TOKENS` | `length` |
| (tool calls present) | `tool_calls` |

## Example Node Config

```json
{
  "config": {
    "provider": "google",
    "model": "gemini-2.0-flash",
    "api_key_ref": "google-gemini-key",
    "system_prompt": "You are a translation assistant.",
    "user_prompt": "Translate to {{input.target_language}}:\n{{input.text}}",
    "variables": [
      {"name": "text", "schema": {"type": "string"}},
      {"name": "target_language", "schema": {"type": "string"}}
    ],
    "response_format": "text",
    "temperature": 0.3
  }
}
```

## See Also

- [LLM Node Reference](../nodes/llm.md) -- full LLM node configuration and output modes
- [Providers Overview](overview.md) -- how providers work, secret resolution
- [Provider Interface](provider-interface.md) -- Complete/Stream methods, error codes
- [Supported Providers](supported.md) -- comparison of all providers

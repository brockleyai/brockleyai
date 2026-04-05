# Supported Providers

Detailed reference for each LLM provider supported by Brockley.

---

## OpenAI

**Config value:** `"openai"`

Uses the OpenAI Chat Completions API (`/v1/chat/completions`).

### Authentication

- **Header:** `Authorization: Bearer <api_key>`
- **API Key:** Obtained from [platform.openai.com](https://platform.openai.com)

### Base URL

Default: `https://api.openai.com/v1`

Override with `base_url` in the node config to point at OpenAI-compatible APIs (Azure OpenAI, local proxies, etc.).

### JSON Mode

OpenAI natively supports JSON mode via the `response_format` parameter. When `response_format` is `"json"` in the Brockley node config, the provider sends `{"response_format": {"type": "json_object"}}` to the API. The engine also appends the output schema to the system prompt for schema-guided generation.

### Key Models

| Model | Notes |
|-------|-------|
| `gpt-4o` | Flagship multimodal model |
| `gpt-4o-mini` | Cost-efficient, fast |
| `gpt-4-turbo` | Previous generation |
| `o1` | Reasoning model |
| `o3-mini` | Compact reasoning model |

### Example Node Config

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

### Environment Variable

```bash
export BROCKLEY_SECRET_OPENAI_KEY="sk-..."
```

---

## Anthropic

**Config value:** `"anthropic"`

Uses the Anthropic Messages API (`/v1/messages`).

### Authentication

- **Header:** `x-api-key: <api_key>`
- **Version header:** `anthropic-version: 2023-06-01` (set automatically)
- **API Key:** Obtained from [console.anthropic.com](https://console.anthropic.com)

### Base URL

Default: `https://api.anthropic.com`

### Structured Output

Anthropic does not have a native JSON mode parameter. Instead, Brockley achieves structured output via the system prompt. When `response_format` is `"json"`:

- If `output_schema` is provided, the provider appends to the system prompt: `"Respond with valid JSON matching this schema: <schema>"`
- If no schema is provided, the provider appends: `"Respond with valid JSON."`

### Max Tokens

Anthropic requires `max_tokens` on every request. If not specified in the node config, Brockley defaults to `4096`.

### Key Models

| Model | Notes |
|-------|-------|
| `claude-sonnet-4-20250514` | Best balance of speed and quality |
| `claude-opus-4-20250514` | Most capable |
| `claude-haiku-3-20250414` | Fastest, most affordable |

### Example Node Config

```json
{
  "config": {
    "provider": "anthropic",
    "model": "claude-sonnet-4-20250514",
    "api_key_ref": "anthropic-key",
    "system_prompt": "You are an expert data analyst.",
    "user_prompt": "Analyze this dataset:\n{{input.data | json}}",
    "variables": [
      {"name": "data", "schema": {"type": "array"}}
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

### Environment Variable

```bash
export BROCKLEY_SECRET_ANTHROPIC_KEY="sk-ant-..."
```

---

## Google Gemini

**Config value:** `"google"`

Uses the Google Gemini API (`/v1beta/models/<model>:generateContent`).

### Authentication

- **Header:** `x-goog-api-key: <api_key>`
- **API Key:** Obtained from [aistudio.google.com](https://aistudio.google.com)

### Base URL

Default: `https://generativelanguage.googleapis.com/v1beta`

### JSON Mode

Google Gemini supports JSON mode natively via the `responseMimeType` field in `generationConfig`. When `response_format` is `"json"`, the provider sets `"responseMimeType": "application/json"`.

### Request Structure

Gemini uses a different API structure than OpenAI/Anthropic:

- System prompts are sent via the `systemInstruction` field
- User messages are sent in the `contents` array
- Generation parameters (temperature, max tokens) go in `generationConfig`

### Key Models

| Model | Notes |
|-------|-------|
| `gemini-2.0-flash` | Fast, cost-effective |
| `gemini-2.0-pro` | More capable |
| `gemini-1.5-pro` | Long context (up to 2M tokens) |
| `gemini-1.5-flash` | Fast with long context |

### Example Node Config

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

### Environment Variable

```bash
export BROCKLEY_SECRET_GOOGLE_GEMINI_KEY="AIza..."
```

---

## OpenRouter

**Config value:** `"openrouter"`

Uses the OpenRouter API, which is OpenAI-compatible. OpenRouter provides access to hundreds of models from multiple providers through a single API.

### Authentication

- **Header:** `Authorization: Bearer <api_key>`
- **Additional header:** `HTTP-Referer: https://brockley.ai` (set automatically by the provider)
- **API Key:** Obtained from [openrouter.ai](https://openrouter.ai)

### Base URL

Default: `https://openrouter.ai/api/v1`

### How It Works

The OpenRouter provider reuses the OpenAI provider internally. It sends requests to the OpenRouter API using the same OpenAI Chat Completions format. The only differences are the base URL and the additional `HTTP-Referer` header.

### Model Naming

OpenRouter uses a `provider/model` naming convention:

| Model | OpenRouter Name |
|-------|----------------|
| GPT-4o | `openai/gpt-4o` |
| Claude Sonnet | `anthropic/claude-sonnet-4-20250514` |
| Gemini Flash | `google/gemini-2.0-flash` |
| Llama 3.1 | `meta-llama/llama-3.1-70b-instruct` |
| Mixtral | `mistralai/mixtral-8x7b-instruct` |

Check [openrouter.ai/models](https://openrouter.ai/models) for the full list.

### JSON Mode

JSON mode works the same as OpenAI since OpenRouter is OpenAI-compatible. The `response_format` parameter is forwarded to the underlying model.

### Example Node Config

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

### Environment Variable

```bash
export BROCKLEY_SECRET_OPENROUTER_KEY="sk-or-..."
```

---

## AWS Bedrock

**Config value:** `"bedrock"`

**Status: Stub implementation.** The provider is registered in the default registry but returns an error when called. See [Bedrock](bedrock.md) for workarounds.

### Planned Configuration

| Field | Description |
|-------|-------------|
| `bedrock_region` | AWS region (e.g., `us-east-1`) |
| API credentials | AWS access key and secret key, resolved via the secret store |

### Current Behavior

Calling an LLM node with `"provider": "bedrock"` will fail with:

```
bedrock provider requires AWS credentials configuration -- not yet fully implemented
```

### Example Node Config (for future use)

```json
{
  "config": {
    "provider": "bedrock",
    "model": "anthropic.claude-sonnet-4-20250514-v1:0",
    "api_key_ref": "aws-bedrock",
    "bedrock_region": "us-east-1",
    "system_prompt": "You are a helpful assistant.",
    "user_prompt": "{{input.question}}",
    "variables": [
      {"name": "question", "schema": {"type": "string"}}
    ]
  }
}
```

### Environment Variables (planned)

```bash
export BROCKLEY_SECRET_AWS_BEDROCK="<access_key>:<secret_key>"
```

---

## Provider Comparison

| Feature | OpenAI | Anthropic | Google | OpenRouter | Bedrock |
|---------|--------|-----------|--------|------------|---------|
| Status | Implemented | Implemented | Implemented | Implemented | Stub |
| Native JSON mode | Yes | No (via prompt) | Yes | Yes | -- |
| Streaming | Yes | Yes | Yes | Yes | -- |
| Tool calling | Yes | Yes | Yes | Yes (model-dependent) | -- |
| Auth method | Bearer token | x-api-key header | x-goog-api-key | Bearer token | AWS SigV4 (planned) |
| Custom base URL | Yes | Yes | Yes | Yes | N/A |
| Default max_tokens | Provider default | 4096 | Provider default | Provider default | -- |

## See Also

- [LLM Node Reference](../nodes/llm.md) -- full LLM node configuration and output modes
- [Providers Overview](overview.md) -- how providers work, secret resolution
- [Provider Interface](provider-interface.md) -- Complete/Stream methods, error codes
- [OpenAI](openai.md) | [Anthropic](anthropic.md) | [Google](google.md) | [OpenRouter](openrouter.md) | [Bedrock](bedrock.md) | [Custom](custom.md)

# Providers Overview

Providers connect Brockley's LLM nodes to external language model APIs. Each provider implements a common interface that handles authentication, request formatting, and response parsing for a specific LLM service.

## How Providers Work in LLM Nodes

When an LLM node executes:

1. The engine reads the `provider` field from the node's config to determine which provider to use.
2. It looks up the provider in the **Provider Registry** by name.
3. It resolves the API key using the `api_key_ref` field via the **Secret Store**.
4. It builds a `CompletionRequest` with the rendered prompts, model name, and parameters.
5. The provider translates this into the API-specific format, makes the HTTP call, and returns a normalized `CompletionResponse`.

The LLM node does not know the details of any specific API. All provider-specific logic (endpoint URLs, auth headers, request/response schemas) is encapsulated in the provider implementation.

## Provider Registry

The provider registry is a thread-safe map of provider names to `LLMProvider` implementations. Providers are registered at application startup.

```go
registry := provider.NewRegistry()
registry.Register("openai", provider.NewOpenAIProvider(apiKey, ""))
registry.Register("anthropic", provider.NewAnthropicProvider(apiKey, ""))
```

The LLM executor looks up providers by the string in the node's `provider` config field.

## Secret Resolution (`api_key_ref`)

API keys are never stored directly in graph definitions. Instead, LLM nodes reference secrets by name using the `api_key_ref` field.

The default secret store resolves references via environment variables using this convention:

```
api_key_ref value  ->  environment variable
-------------------------------------------------
openai-key         ->  BROCKLEY_SECRET_OPENAI_KEY
anthropic-key      ->  BROCKLEY_SECRET_ANTHROPIC_KEY
google-gemini      ->  BROCKLEY_SECRET_GOOGLE_GEMINI
my-custom-key      ->  BROCKLEY_SECRET_MY_CUSTOM_KEY
```

The transformation rule is:
1. Convert to uppercase
2. Replace hyphens (`-`) with underscores (`_`)
3. Prepend `BROCKLEY_SECRET_`

### Example

For a node with `"api_key_ref": "anthropic-primary"`:

```bash
export BROCKLEY_SECRET_ANTHROPIC_PRIMARY="sk-ant-..."
```

The engine calls `SecretStore.GetSecret(ctx, "anthropic-primary")`, which reads `BROCKLEY_SECRET_ANTHROPIC_PRIMARY` from the environment.

## Supported Providers

| Provider | Config Value | API Style | Status |
|----------|-------------|-----------|--------|
| OpenAI | `"openai"` | OpenAI Chat Completions | Fully implemented |
| Anthropic | `"anthropic"` | Anthropic Messages API | Fully implemented |
| Google Gemini | `"google"` | Gemini generateContent | Fully implemented |
| OpenRouter | `"openrouter"` | OpenAI-compatible | Fully implemented |
| AWS Bedrock | `"bedrock"` | AWS Bedrock | Stub (not yet fully implemented) |
| Custom | `"custom"` | User-defined | Via provider registry |

See the [Supported Providers](supported.md) page for detailed configuration examples and model lists for each provider.

## Choosing a Provider

| Consideration | Recommendation |
|---------------|----------------|
| Widest model selection | OpenAI or OpenRouter |
| Best structured output | OpenAI (native JSON mode) or Anthropic |
| Self-hosted / on-premise | OpenRouter (as a proxy) or Custom |
| Cost optimization | OpenRouter (multi-provider routing) |
| Google ecosystem | Google Gemini |
| AWS ecosystem | Bedrock (stub -- use OpenRouter or custom proxy for now) |
| Multiple providers in one graph | Use different `api_key_ref` values per LLM node |

## LLMProvider Interface

All providers implement this interface:

```go
type LLMProvider interface {
    Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
    Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error)
    Name() string
}
```

- **Complete** -- synchronous call that returns the full response.
- **Stream** -- returns a channel of incremental chunks for streaming responses.
- **Name** -- returns the provider identifier string.

## Custom Providers

You can register custom providers that implement the `LLMProvider` interface. This is useful for:

- Internal LLM deployments (vLLM, TGI, Ollama)
- API gateways with custom auth
- Mock providers for testing
- Providers not yet built into Brockley

```go
registry.Register("my-llm", &MyCustomProvider{
    baseURL: "https://internal-llm.company.com/v1",
})
```

Then reference it in a node config:

```json
{
  "provider": "my-llm",
  "model": "internal-model-v2",
  "api_key_ref": "internal-llm-key"
}
```

## See Also

- [LLM Node Reference](../nodes/llm.md) -- full LLM node configuration, tool calling, and output modes
- [Provider Interface](provider-interface.md) -- Complete/Stream methods, error codes, retry behavior
- [Supported Providers](supported.md) -- detailed config for each provider
- [OpenAI](openai.md) | [Anthropic](anthropic.md) | [Google](google.md) | [OpenRouter](openrouter.md) | [Bedrock](bedrock.md) | [Custom](custom.md)
- [Configuration Reference](../deployment/configuration.md) -- `BROCKLEY_SECRET_*` variables

# Provider Interface

The provider layer abstracts LLM API differences behind a common Go interface. Each provider implements the same interface, so the engine does not need to know which LLM service is being called.

## Interface

```go
type LLMProvider interface {
    Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
    Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error)
    Name() string
}
```

## Error Handling

All providers return structured `ProviderError` values on failure:

| Code | Description | Retryable |
|------|-------------|-----------|
| `rate_limited` | Provider rate limit exceeded | Yes |
| `auth_failed` | Invalid API key or forbidden | No |
| `model_not_found` | Requested model does not exist | No |
| `context_length` | Input exceeds model context window | No |
| `content_filtered` | Content policy violation | No |
| `server_error` | Provider server error (5xx) | Yes |
| `invalid_request` | Malformed request | No |

## Retry Behavior

Retryable errors are automatically retried with exponential backoff:

- **Max retries**: 3 (configurable)
- **Initial backoff**: 1 second
- **Max backoff**: 30 seconds
- **Backoff factor**: 2x with 10-30% jitter
- **Retry-After header**: Respected when provided by the provider

Use `NewRetryableProvider` to wrap any provider with retry logic:

```go
provider := NewRetryableProvider(openaiProvider, DefaultRetryConfig())
```

## Rate Limiting

Client-side rate limiting prevents exceeding provider quotas:

```go
provider := NewRateLimitedProvider(openaiProvider, RateLimitConfig{
    RequestsPerMinute: 100,
})
```

Rate limiting and retry can be composed:

```go
provider := NewRetryableProvider(
    NewRateLimitedProvider(openaiProvider, rateLimitConfig),
    retryConfig,
)
```

## API Key Handling

LLM nodes support two ways to provide API keys:

1. **Inline (`api_key`)** -- the key is set directly on the node config. Takes priority.
2. **Secret reference (`api_key_ref`)** -- a named reference resolved via the secret store at runtime. Used as fallback.

```json
{
  "config": {
    "api_key": "sk-or-v1-abc123...",
    "api_key_ref": "openrouter-fallback"
  }
}
```

At least one must be provided. If both are set, `api_key` is used.

At execution time, the resolved key is passed to the provider via `CompletionRequest.APIKey`. Each provider applies the correct auth header for its API (Bearer token, `x-api-key`, etc.). If `APIKey` is non-empty, it takes priority over the key the provider was constructed with.

The `SecretStore` interface resolves `api_key_ref` values:

```go
type SecretStore interface {
    GetSecret(ctx context.Context, ref string) (string, error)
}
```

The default `EnvSecretStore` maps references to environment variables:
- `"anthropic-primary"` resolves to `BROCKLEY_SECRET_ANTHROPIC_PRIMARY`

**Masking:** When graphs are returned via the API, `api_key` values are masked (e.g., `"sk-or...ab12"`). Short keys show as `"****"`. On update, submitting the masked value preserves the stored key.

## Default Registry

`NewDefaultRegistry()` returns a `ProviderRegistry` pre-populated with all 5 built-in providers (openai, anthropic, google, openrouter, bedrock). Providers are constructed with empty API keys since actual keys are supplied at runtime via `CompletionRequest.APIKey`. The worker uses this at startup.

## Adding a New Provider

1. Implement `LLMProvider` in `engine/provider/`
2. Handle auth, request/response translation, streaming
3. Return `ProviderError` for all API errors
4. Register in the `ProviderRegistry` (and add to `NewDefaultRegistry()`)
5. Document in this spec

## See Also

- [Providers Overview](overview.md) -- how providers work, secret resolution, provider registry
- [Supported Providers](supported.md) -- config details for each provider
- [Custom Providers](custom.md) -- building your own provider implementation

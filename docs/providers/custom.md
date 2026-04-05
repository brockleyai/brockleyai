# Custom Providers

You can register custom providers that implement the `LLMProvider` interface. This is useful for internal LLM deployments (vLLM, TGI, Ollama), API gateways with custom auth, mock providers for testing, and providers not yet built into Brockley.

## The LLMProvider Interface

```go
type LLMProvider interface {
    Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
    Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error)
    Name() string
}
```

- **Complete** -- synchronous call that returns the full response
- **Stream** -- returns a channel of incremental chunks
- **Name** -- returns the provider identifier string (used in metrics, logs, and node config)

## Implementing a Custom Provider

Here is a minimal example that wraps an OpenAI-compatible API:

```go
package myprovider

import (
    "context"
    "github.com/brockleyai/brockleyai/engine/provider"
    "github.com/brockleyai/brockleyai/internal/model"
)

type MyProvider struct {
    inner *provider.OpenAIProvider
}

func NewMyProvider() *MyProvider {
    return &MyProvider{
        inner: provider.NewOpenAIProvider("", "https://my-llm.internal.company.com/v1"),
    }
}

func (p *MyProvider) Name() string {
    return "my-llm"
}

func (p *MyProvider) Complete(ctx context.Context, req *model.CompletionRequest) (*model.CompletionResponse, error) {
    resp, err := p.inner.Complete(ctx, req)
    if err != nil {
        return nil, err
    }
    resp.Usage.Provider = p.Name()
    return resp, nil
}

func (p *MyProvider) Stream(ctx context.Context, req *model.CompletionRequest) (<-chan model.StreamChunk, error) {
    return p.inner.Stream(ctx, req)
}
```

## Registering the Provider

Register your provider at application startup:

```go
registry := provider.NewDefaultRegistry()
registry.Register("my-llm", myprovider.NewMyProvider())
```

Then reference it in a node config:

```json
{
  "provider": "my-llm",
  "model": "internal-model-v2",
  "api_key_ref": "internal-llm-key"
}
```

## Error Handling

Return `ProviderError` values for all API errors so the retry wrapper handles them correctly:

| Code | Retryable | When |
|------|-----------|------|
| `rate_limited` | Yes | 429 responses |
| `auth_failed` | No | 401/403 responses |
| `model_not_found` | No | Model does not exist |
| `context_length` | No | Input too long |
| `content_filtered` | No | Content policy violation |
| `server_error` | Yes | 5xx responses |
| `invalid_request` | No | Malformed request |

Use the `classifyHTTPError` helper if your provider makes HTTP calls.

## Adding Retry and Rate Limiting

Wrap your provider with the built-in retry and rate limit decorators:

```go
base := myprovider.NewMyProvider()
withRetry := provider.NewRetryableProvider(base, provider.DefaultRetryConfig())
withRateLimit := provider.NewRateLimitedProvider(withRetry, provider.RateLimitConfig{
    RequestsPerMinute: 60,
})
registry.Register("my-llm", withRateLimit)
```

## OpenAI-Compatible APIs

If your target API is OpenAI-compatible (vLLM, Ollama with `/v1/chat/completions`, LiteLLM), you do not need a custom provider. Use the `openai` provider with a custom `base_url`:

```json
{
  "provider": "openai",
  "model": "my-local-model",
  "api_key_ref": "local-key",
  "base_url": "http://localhost:11434/v1"
}
```

Only build a custom provider when the target API is not OpenAI-compatible or you need custom logic (non-standard auth, request transformation, response post-processing).

## Testing Custom Providers

Write round-trip tests for your provider:

```go
func TestMyProvider_Complete(t *testing.T) {
    // Use httptest.NewServer to create a fake API endpoint
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Verify request format
        // Return a canned response
    }))
    defer server.Close()

    p := NewMyProvider()
    // Override base URL to point at test server
    resp, err := p.Complete(context.Background(), &model.CompletionRequest{
        Model:      "test-model",
        UserPrompt: "Hello",
        BaseURL:    server.URL,
    })
    require.NoError(t, err)
    require.NotEmpty(t, resp.Content)
}
```

## See Also

- [LLM Node Reference](../nodes/llm.md) -- full LLM node configuration and output modes
- [Provider Interface](provider-interface.md) -- Complete/Stream methods, error codes, retry behavior
- [Providers Overview](overview.md) -- how providers work, secret resolution
- [Contributing Guide](../contributing/internal-guide.md) -- development conventions

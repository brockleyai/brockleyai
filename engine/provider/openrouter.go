package provider

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/brockleyai/brockleyai/internal/model"
)

const defaultOpenRouterBaseURL = "https://openrouter.ai/api/v1"

// OpenRouterProvider implements model.LLMProvider using the OpenRouter API.
// It reuses the OpenAI HTTP logic with a different base URL and additional headers.
type OpenRouterProvider struct {
	inner *OpenAIProvider
}

var _ model.LLMProvider = (*OpenRouterProvider)(nil)

// NewOpenRouterProvider creates a new OpenRouter provider.
// If baseURL is empty, the default OpenRouter API URL is used.
func NewOpenRouterProvider(apiKey, baseURL string) *OpenRouterProvider {
	if baseURL == "" {
		baseURL = defaultOpenRouterBaseURL
	}
	inner := &OpenAIProvider{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 30 * time.Second},
	}
	return &OpenRouterProvider{inner: inner}
}

func (p *OpenRouterProvider) Name() string {
	return "openrouter"
}

// mergeHeaders returns a copy of extraHeaders with the OpenRouter-specific
// HTTP-Referer header added.
func (p *OpenRouterProvider) mergeHeaders(extraHeaders map[string]string) map[string]string {
	merged := make(map[string]string, len(extraHeaders)+1)
	for k, v := range extraHeaders {
		merged[k] = v
	}
	merged["HTTP-Referer"] = "https://brockley.ai"
	return merged
}

func (p *OpenRouterProvider) Complete(ctx context.Context, req *model.CompletionRequest) (*model.CompletionResponse, error) {
	// Create a copy of the request with merged headers.
	reqCopy := *req
	reqCopy.ExtraHeaders = p.mergeHeaders(req.ExtraHeaders)

	resp, err := p.inner.Complete(ctx, &reqCopy)
	if err != nil {
		return nil, err
	}
	// Override the provider name in the usage.
	resp.Usage.Provider = p.Name()
	return resp, nil
}

func (p *OpenRouterProvider) Stream(ctx context.Context, req *model.CompletionRequest) (<-chan model.StreamChunk, error) {
	reqCopy := *req
	reqCopy.ExtraHeaders = p.mergeHeaders(req.ExtraHeaders)

	ch, err := p.inner.Stream(ctx, &reqCopy)
	if err != nil {
		return nil, err
	}

	// Wrap the channel to fix the provider name in usage.
	out := make(chan model.StreamChunk)
	go func() {
		defer close(out)
		for chunk := range ch {
			if chunk.Usage != nil {
				usageCopy := *chunk.Usage
				usageCopy.Provider = p.Name()
				chunk.Usage = &usageCopy
			}
			out <- chunk
		}
	}()

	return out, nil
}

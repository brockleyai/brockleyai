package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brockleyai/brockleyai/engine/mock"
	"github.com/brockleyai/brockleyai/internal/model"
)

// Compile-time checks that each provider satisfies model.LLMProvider.
var (
	_ model.LLMProvider = (*OpenAIProvider)(nil)
	_ model.LLMProvider = (*AnthropicProvider)(nil)
	_ model.LLMProvider = (*GoogleProvider)(nil)
	_ model.LLMProvider = (*OpenRouterProvider)(nil)
	_ model.LLMProvider = (*BedrockProvider)(nil)
)

func TestRegistryRegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	m := &mock.MockLLMProvider{Responses: []string{"hello"}}

	reg.Register("mock", m)

	got, err := reg.Get("mock")
	if err != nil {
		t.Fatalf("Get returned unexpected error: %v", err)
	}
	if got.Name() != "mock" {
		t.Errorf("expected provider name %q, got %q", "mock", got.Name())
	}
}

func TestRegistryGetUnknown(t *testing.T) {
	reg := NewRegistry()

	_, err := reg.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
}

func TestRegistryList(t *testing.T) {
	reg := NewRegistry()

	m1 := &mock.MockLLMProvider{Responses: []string{"a"}}
	m2 := &mock.MockLLMProvider{Responses: []string{"b"}}

	reg.Register("beta", m1)
	reg.Register("alpha", m2)

	names := reg.List()
	if len(names) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(names))
	}
	// List returns sorted names.
	if names[0] != "alpha" || names[1] != "beta" {
		t.Errorf("expected [alpha beta], got %v", names)
	}
}

func TestRegistryOverwrite(t *testing.T) {
	reg := NewRegistry()

	m1 := &mock.MockLLMProvider{Responses: []string{"first"}}
	m2 := &mock.MockLLMProvider{Responses: []string{"second"}}

	reg.Register("test", m1)
	reg.Register("test", m2)

	names := reg.List()
	if len(names) != 1 {
		t.Fatalf("expected 1 provider after overwrite, got %d", len(names))
	}
}

func TestProviderNames(t *testing.T) {
	tests := []struct {
		name     string
		provider model.LLMProvider
		want     string
	}{
		{"openai", NewOpenAIProvider("key", ""), "openai"},
		{"anthropic", NewAnthropicProvider("key", ""), "anthropic"},
		{"google", NewGoogleProvider("key", ""), "google"},
		{"openrouter", NewOpenRouterProvider("key", ""), "openrouter"},
		{"bedrock", NewBedrockProvider("us-east-1", "", ""), "bedrock"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.provider.Name(); got != tt.want {
				t.Errorf("Name() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBedrockStubReturnsError(t *testing.T) {
	p := NewBedrockProvider("us-east-1", "", "")

	_, err := p.Complete(context.TODO(), &model.CompletionRequest{})
	if err == nil {
		t.Fatal("expected error from bedrock stub Complete, got nil")
	}

	_, err = p.Stream(context.TODO(), &model.CompletionRequest{})
	if err == nil {
		t.Fatal("expected error from bedrock stub Stream, got nil")
	}
}

func TestOpenAIDefaultBaseURL(t *testing.T) {
	p := NewOpenAIProvider("key", "")
	if p.baseURL != "https://api.openai.com/v1" {
		t.Errorf("expected default OpenAI base URL, got %q", p.baseURL)
	}
}

func TestAnthropicDefaultBaseURL(t *testing.T) {
	p := NewAnthropicProvider("key", "")
	if p.baseURL != "https://api.anthropic.com" {
		t.Errorf("expected default Anthropic base URL, got %q", p.baseURL)
	}
}

func TestGoogleDefaultBaseURL(t *testing.T) {
	p := NewGoogleProvider("key", "")
	if p.baseURL != "https://generativelanguage.googleapis.com/v1beta" {
		t.Errorf("expected default Google base URL, got %q", p.baseURL)
	}
}

func TestOpenRouterDefaultBaseURL(t *testing.T) {
	p := NewOpenRouterProvider("key", "")
	if p.inner.baseURL != "https://openrouter.ai/api/v1" {
		t.Errorf("expected default OpenRouter base URL, got %q", p.inner.baseURL)
	}
}

func TestCustomBaseURL(t *testing.T) {
	p := NewOpenAIProvider("key", "https://custom.api.com/v1/")
	// Trailing slash should be trimmed.
	if p.baseURL != "https://custom.api.com/v1" {
		t.Errorf("expected trimmed base URL, got %q", p.baseURL)
	}
}

func TestOpenAIProvider_ReqAPIKeyOverridesConstructor(t *testing.T) {
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openAIResponse{
			Model: "test",
			Choices: []struct {
				Message      openAIMessage `json:"message"`
				FinishReason string        `json:"finish_reason"`
			}{
				{Message: openAIMessage{Role: "assistant", Content: "ok"}, FinishReason: "stop"},
			},
		})
	}))
	defer srv.Close()

	p := NewOpenAIProvider("constructor-key", srv.URL)
	req := &model.CompletionRequest{
		APIKey:     "request-key",
		Model:      "test",
		UserPrompt: "hi",
	}

	_, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedAuth != "Bearer request-key" {
		t.Errorf("expected 'Bearer request-key', got %q", capturedAuth)
	}
}

func TestOpenAIProvider_FallsBackToConstructorKey(t *testing.T) {
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openAIResponse{
			Model: "test",
			Choices: []struct {
				Message      openAIMessage `json:"message"`
				FinishReason string        `json:"finish_reason"`
			}{
				{Message: openAIMessage{Role: "assistant", Content: "ok"}, FinishReason: "stop"},
			},
		})
	}))
	defer srv.Close()

	p := NewOpenAIProvider("constructor-key", srv.URL)
	req := &model.CompletionRequest{
		Model:      "test",
		UserPrompt: "hi",
	}

	_, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedAuth != "Bearer constructor-key" {
		t.Errorf("expected 'Bearer constructor-key', got %q", capturedAuth)
	}
}

func TestAnthropicProvider_ReqAPIKeyOverridesConstructor(t *testing.T) {
	var capturedKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedKey = r.Header.Get("x-api-key")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(anthropicResponse{
			ID:         "test",
			Model:      "test",
			StopReason: "end_turn",
			Content: []struct {
				Type  string          `json:"type"`
				Text  string          `json:"text,omitempty"`
				ID    string          `json:"id,omitempty"`
				Name  string          `json:"name,omitempty"`
				Input json.RawMessage `json:"input,omitempty"`
			}{{Type: "text", Text: "ok"}},
		})
	}))
	defer srv.Close()

	p := NewAnthropicProvider("constructor-key", srv.URL)
	req := &model.CompletionRequest{
		APIKey:     "request-key",
		Model:      "test",
		UserPrompt: "hi",
	}

	_, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedKey != "request-key" {
		t.Errorf("expected 'request-key', got %q", capturedKey)
	}
}

func TestGoogleProvider_ReqAPIKeyOverridesConstructor(t *testing.T) {
	var capturedKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedKey = r.Header.Get("x-goog-api-key")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(geminiResponse{
			Candidates: []struct {
				Content      geminiContent `json:"content"`
				FinishReason string        `json:"finishReason"`
			}{
				{
					Content:      geminiContent{Parts: []geminiPart{{Text: "ok"}}},
					FinishReason: "STOP",
				},
			},
		})
	}))
	defer srv.Close()

	p := NewGoogleProvider("constructor-key", srv.URL)
	req := &model.CompletionRequest{
		APIKey:     "request-key",
		Model:      "test-model",
		UserPrompt: "hi",
	}

	_, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedKey != "request-key" {
		t.Errorf("expected 'request-key', got %q", capturedKey)
	}
}

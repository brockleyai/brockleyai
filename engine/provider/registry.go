package provider

import (
	"fmt"
	"sort"
	"sync"

	"github.com/brockleyai/brockleyai/internal/model"
)

// Registry holds named LLM providers and allows lookup by name.
type Registry struct {
	providers map[string]model.LLMProvider
	mu        sync.RWMutex
}

// NewRegistry creates an empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]model.LLMProvider),
	}
}

// Register adds a provider to the registry under the given name.
// If a provider with the same name already exists, it is replaced.
func (r *Registry) Register(name string, provider model.LLMProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = provider
}

// Get returns the provider registered under the given name.
// Returns an error if no provider is registered with that name.
func (r *Registry) Get(name string) (model.LLMProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %q not found in registry", name)
	}
	return p, nil
}

// NewDefaultRegistry creates a registry pre-loaded with all built-in providers.
// Providers are created without constructor API keys — callers pass keys
// at runtime via CompletionRequest.APIKey.
func NewDefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register("openai", NewOpenAIProvider("", ""))
	r.Register("anthropic", NewAnthropicProvider("", ""))
	r.Register("google", NewGoogleProvider("", ""))
	r.Register("openrouter", NewOpenRouterProvider("", ""))
	r.Register("bedrock", NewBedrockProvider("", "", ""))
	return r
}

// List returns the names of all registered providers in sorted order.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

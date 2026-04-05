package executor

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"sync"

	"github.com/brockleyai/brockleyai/engine/mcp"
	"github.com/brockleyai/brockleyai/internal/model"
)

// MCPClientCache is scoped to a single graph execution.
// It caches MCP clients by URL + headers hash to avoid redundant connections.
type MCPClientCache struct {
	mu      sync.Mutex
	clients map[string]model.MCPClient
}

// NewMCPClientCache creates a new empty cache.
func NewMCPClientCache() *MCPClientCache {
	return &MCPClientCache{
		clients: make(map[string]model.MCPClient),
	}
}

// GetOrCreate returns an existing MCP client for the given URL and headers,
// or creates a new one if none exists.
func (c *MCPClientCache) GetOrCreate(url string, headers map[string]string) model.MCPClient {
	key := cacheKey(url, headers)
	c.mu.Lock()
	defer c.mu.Unlock()

	if client, ok := c.clients[key]; ok {
		return client
	}

	client := mcp.NewClient(url, headers)
	c.clients[key] = client
	return client
}

// ListToolsCached calls ListTools on the MCP client at the given URL,
// caching the result for the lifetime of this cache.
func (c *MCPClientCache) ListToolsCached(ctx context.Context, url string, headers map[string]string) ([]model.ToolDefinition, error) {
	client := c.GetOrCreate(url, headers)
	return client.ListTools(ctx)
}

// cacheKey creates a deterministic key from URL and sorted headers.
func cacheKey(url string, headers map[string]string) string {
	if len(headers) == 0 {
		return url
	}
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	h.Write([]byte(url))
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte(headers[k]))
	}
	return fmt.Sprintf("%s#%x", url, h.Sum(nil)[:8])
}

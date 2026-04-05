package executor

import (
	"encoding/json"
	"testing"

	"github.com/brockleyai/brockleyai/engine/mock"
	"github.com/brockleyai/brockleyai/internal/model"
)

func TestMCPToolInterceptor_ListTools(t *testing.T) {
	mcpClient := &mock.MockMCPClient{
		Tools: map[string]mock.MockTool{
			"search": {Definition: model.ToolDefinition{Name: "search", Description: "Search things"}},
			"fetch":  {Definition: model.ToolDefinition{Name: "fetch", Description: "Fetch data"}},
		},
	}

	cache := NewMCPClientCache()
	cache.clients["http://test-mcp:9000/mcp"] = mcpClient

	interceptor := NewMCPToolInterceptor(cache, []string{"http://test-mcp:9000/mcp"})

	args, _ := json.Marshal(map[string]string{"mcp_url": "http://test-mcp:9000/mcp"})
	result, handled := interceptor("_list_mcp_tools", args)
	if !handled {
		t.Fatal("expected interceptor to handle _list_mcp_tools")
	}

	var names []string
	if err := json.Unmarshal([]byte(result), &names); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if len(names) != 2 {
		t.Errorf("expected 2 tool names, got %d", len(names))
	}

	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	if !nameSet["search"] || !nameSet["fetch"] {
		t.Errorf("expected tool names [search, fetch], got %v", names)
	}
}

func TestMCPToolInterceptor_DescribeTool(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{"type": "string"},
		},
	}
	mcpClient := &mock.MockMCPClient{
		Tools: map[string]mock.MockTool{
			"search": {Definition: model.ToolDefinition{
				Name:        "search",
				Description: "Search the knowledge base",
				InputSchema: schema,
			}},
		},
	}

	cache := NewMCPClientCache()
	cache.clients["http://test-mcp:9000/mcp"] = mcpClient

	interceptor := NewMCPToolInterceptor(cache, []string{"http://test-mcp:9000/mcp"})

	args, _ := json.Marshal(map[string]string{
		"mcp_url":   "http://test-mcp:9000/mcp",
		"tool_name": "search",
	})
	result, handled := interceptor("_describe_mcp_tool", args)
	if !handled {
		t.Fatal("expected interceptor to handle _describe_mcp_tool")
	}

	var desc map[string]any
	if err := json.Unmarshal([]byte(result), &desc); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if desc["name"] != "search" {
		t.Errorf("expected name=search, got %v", desc["name"])
	}
	if desc["description"] != "Search the knowledge base" {
		t.Errorf("expected description='Search the knowledge base', got %v", desc["description"])
	}
	if desc["input_schema"] == nil {
		t.Error("expected input_schema to be present")
	}
}

func TestMCPToolInterceptor_DescribeTool_NotFound(t *testing.T) {
	mcpClient := &mock.MockMCPClient{
		Tools: map[string]mock.MockTool{
			"search": {Definition: model.ToolDefinition{Name: "search"}},
		},
	}

	cache := NewMCPClientCache()
	cache.clients["http://test-mcp:9000/mcp"] = mcpClient

	interceptor := NewMCPToolInterceptor(cache, []string{"http://test-mcp:9000/mcp"})

	args, _ := json.Marshal(map[string]string{
		"mcp_url":   "http://test-mcp:9000/mcp",
		"tool_name": "nonexistent",
	})
	result, handled := interceptor("_describe_mcp_tool", args)
	if !handled {
		t.Fatal("expected interceptor to handle _describe_mcp_tool")
	}
	if result == "" || result[0] != 'E' {
		t.Errorf("expected error result, got %q", result)
	}
}

func TestMCPToolInterceptor_ScopeEnforcement(t *testing.T) {
	mcpClient := &mock.MockMCPClient{
		Tools: map[string]mock.MockTool{
			"search": {Definition: model.ToolDefinition{Name: "search"}},
		},
	}

	cache := NewMCPClientCache()
	cache.clients["http://test-mcp:9000/mcp"] = mcpClient

	// Only allow a different URL
	interceptor := NewMCPToolInterceptor(cache, []string{"http://other-mcp:9000/mcp"})

	args, _ := json.Marshal(map[string]string{"mcp_url": "http://test-mcp:9000/mcp"})
	result, handled := interceptor("_list_mcp_tools", args)
	if !handled {
		t.Fatal("expected interceptor to handle _list_mcp_tools")
	}
	if result == "" || result[0] != 'E' {
		t.Errorf("expected error for out-of-scope URL, got %q", result)
	}
}

func TestMCPToolInterceptor_UnknownTool(t *testing.T) {
	cache := NewMCPClientCache()
	interceptor := NewMCPToolInterceptor(cache, []string{"http://test-mcp:9000/mcp"})

	_, handled := interceptor("some_other_tool", json.RawMessage(`{}`))
	if handled {
		t.Error("expected interceptor to NOT handle unknown tool names")
	}
}

func TestMCPToolInterceptor_MissingArgs(t *testing.T) {
	cache := NewMCPClientCache()
	interceptor := NewMCPToolInterceptor(cache, []string{"http://test-mcp:9000/mcp"})

	// _list_mcp_tools with empty mcp_url
	result, handled := interceptor("_list_mcp_tools", json.RawMessage(`{"mcp_url":""}`))
	if !handled {
		t.Fatal("expected handled")
	}
	if result == "" || result[0] != 'E' {
		t.Errorf("expected error for empty mcp_url, got %q", result)
	}

	// _describe_mcp_tool with empty tool_name
	result, handled = interceptor("_describe_mcp_tool", json.RawMessage(`{"mcp_url":"http://test-mcp:9000/mcp","tool_name":""}`))
	if !handled {
		t.Fatal("expected handled")
	}
	if result == "" || result[0] != 'E' {
		t.Errorf("expected error for empty tool_name, got %q", result)
	}
}

func TestMCPToolInterceptorFromCache_ListTools(t *testing.T) {
	toolCache := map[string][]model.ToolDefinition{
		"http://test-mcp:9000/mcp": {
			{Name: "search", Description: "Search things"},
			{Name: "fetch", Description: "Fetch data"},
		},
	}

	interceptor := NewMCPToolInterceptorFromCache(toolCache, []string{"http://test-mcp:9000/mcp"})

	args, _ := json.Marshal(map[string]string{"mcp_url": "http://test-mcp:9000/mcp"})
	result, handled := interceptor("_list_mcp_tools", args)
	if !handled {
		t.Fatal("expected interceptor to handle _list_mcp_tools")
	}

	var names []string
	if err := json.Unmarshal([]byte(result), &names); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if len(names) != 2 {
		t.Errorf("expected 2 tool names, got %d", len(names))
	}
}

func TestMCPToolInterceptorFromCache_DescribeTool(t *testing.T) {
	toolCache := map[string][]model.ToolDefinition{
		"http://test-mcp:9000/mcp": {
			{Name: "search", Description: "Search the KB", InputSchema: map[string]any{"type": "object"}},
		},
	}

	interceptor := NewMCPToolInterceptorFromCache(toolCache, []string{"http://test-mcp:9000/mcp"})

	args, _ := json.Marshal(map[string]string{
		"mcp_url":   "http://test-mcp:9000/mcp",
		"tool_name": "search",
	})
	result, handled := interceptor("_describe_mcp_tool", args)
	if !handled {
		t.Fatal("expected interceptor to handle _describe_mcp_tool")
	}

	var desc map[string]any
	if err := json.Unmarshal([]byte(result), &desc); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if desc["name"] != "search" {
		t.Errorf("expected name=search, got %v", desc["name"])
	}
}

func TestCollectCompactedMCPURLs(t *testing.T) {
	routing := map[string]model.ToolRoute{
		"tool1": {MCPURL: "http://mcp-a:9000/mcp", Compacted: true},
		"tool2": {MCPURL: "http://mcp-a:9000/mcp", Compacted: true}, // duplicate
		"tool3": {MCPURL: "http://mcp-b:9000/mcp", Compacted: false},
		"tool4": {MCPURL: "http://mcp-c:9000/mcp", Compacted: true},
	}

	urls := collectCompactedMCPURLs(routing)
	if len(urls) != 2 {
		t.Errorf("expected 2 unique compacted URLs, got %d: %v", len(urls), urls)
	}

	urlSet := make(map[string]bool)
	for _, u := range urls {
		urlSet[u] = true
	}
	if !urlSet["http://mcp-a:9000/mcp"] || !urlSet["http://mcp-c:9000/mcp"] {
		t.Errorf("unexpected URLs: %v", urls)
	}
}

func TestCollectCompactedSkillURLs(t *testing.T) {
	skills := []model.SuperagentSkill{
		{Name: "s1", MCPURL: "http://mcp-a:9000/mcp", Compacted: true},
		{Name: "s2", MCPURL: "http://mcp-a:9000/mcp", Compacted: true},
		{Name: "s3", MCPURL: "http://mcp-b:9000/mcp", Compacted: false},
		{Name: "s4", APIToolID: "api-1"},
	}

	urls := CollectCompactedSkillURLs(skills)
	if len(urls) != 1 {
		t.Errorf("expected 1 unique compacted URL, got %d: %v", len(urls), urls)
	}
	if urls[0] != "http://mcp-a:9000/mcp" {
		t.Errorf("expected http://mcp-a:9000/mcp, got %s", urls[0])
	}
}

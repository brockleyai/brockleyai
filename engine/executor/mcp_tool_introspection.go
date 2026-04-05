package executor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/brockleyai/brockleyai/internal/model"
)

// mcpToolIntrospectionTools are built-in tools that let the LLM discover MCP
// tools at runtime when compacted mode is enabled. They are injected into the
// tool list when any compacted MCP route exists.
var mcpToolIntrospectionTools = []model.LLMToolDefinition{
	{
		Name:        "_list_mcp_tools",
		Description: "List the available tool names from a compacted MCP server. Returns an array of tool names that can be passed to _describe_mcp_tool for full schema details.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"mcp_url": {
					"type": "string",
					"description": "The URL of the MCP server to list tools from."
				}
			},
			"required": ["mcp_url"]
		}`),
	},
	{
		Name:        "_describe_mcp_tool",
		Description: "Describe a specific tool from a compacted MCP server, including its full description and input schema.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"mcp_url": {
					"type": "string",
					"description": "The URL of the MCP server."
				},
				"tool_name": {
					"type": "string",
					"description": "The name of the tool to describe."
				}
			},
			"required": ["mcp_url", "tool_name"]
		}`),
	},
}

// NewMCPToolInterceptor creates a ToolInterceptor that handles the
// _list_mcp_tools and _describe_mcp_tool built-in introspection tools.
// scopedURLs limits which MCP server URLs the LLM is allowed to introspect.
func NewMCPToolInterceptor(cache *MCPClientCache, scopedURLs []string) ToolInterceptor {
	allowed := make(map[string]bool, len(scopedURLs))
	for _, url := range scopedURLs {
		allowed[url] = true
	}

	return func(toolName string, argsRaw json.RawMessage) (string, bool) {
		switch toolName {
		case "_list_mcp_tools":
			return handleListMCPTools(cache, allowed, argsRaw), true
		case "_describe_mcp_tool":
			return handleDescribeMCPTool(cache, allowed, argsRaw), true
		default:
			return "", false
		}
	}
}

func handleListMCPTools(cache *MCPClientCache, allowed map[string]bool, argsRaw json.RawMessage) string {
	var args struct {
		MCPURL string `json:"mcp_url"`
	}
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		return fmt.Sprintf("Error: invalid arguments: %v", err)
	}
	if args.MCPURL == "" {
		return "Error: mcp_url is required"
	}
	if !allowed[args.MCPURL] {
		return fmt.Sprintf("Error: MCP server %q is not available in this context", args.MCPURL)
	}

	tools, err := cache.ListToolsCached(context.Background(), args.MCPURL, nil)
	if err != nil {
		return fmt.Sprintf("Error: listing tools from %s: %v", args.MCPURL, err)
	}

	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}

	out, _ := json.Marshal(names)
	return string(out)
}

func handleDescribeMCPTool(cache *MCPClientCache, allowed map[string]bool, argsRaw json.RawMessage) string {
	var args struct {
		MCPURL   string `json:"mcp_url"`
		ToolName string `json:"tool_name"`
	}
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		return fmt.Sprintf("Error: invalid arguments: %v", err)
	}
	if args.MCPURL == "" {
		return "Error: mcp_url is required"
	}
	if args.ToolName == "" {
		return "Error: tool_name is required"
	}
	if !allowed[args.MCPURL] {
		return fmt.Sprintf("Error: MCP server %q is not available in this context", args.MCPURL)
	}

	tools, err := cache.ListToolsCached(context.Background(), args.MCPURL, nil)
	if err != nil {
		return fmt.Sprintf("Error: listing tools from %s: %v", args.MCPURL, err)
	}

	for _, t := range tools {
		if t.Name == args.ToolName {
			desc := map[string]any{
				"name":         t.Name,
				"description":  t.Description,
				"input_schema": t.InputSchema,
			}
			out, _ := json.Marshal(desc)
			return string(out)
		}
	}

	return fmt.Sprintf("Error: tool %q not found on MCP server %q", args.ToolName, args.MCPURL)
}

// collectCompactedMCPURLs returns unique MCP URLs from routes that have Compacted=true.
func collectCompactedMCPURLs(routing map[string]model.ToolRoute) []string {
	seen := make(map[string]bool)
	var urls []string
	for _, route := range routing {
		if route.Compacted && route.MCPURL != "" && !seen[route.MCPURL] {
			seen[route.MCPURL] = true
			urls = append(urls, route.MCPURL)
		}
	}
	return urls
}

// CollectCompactedSkillURLs returns unique MCP URLs from skills that have Compacted=true.
func CollectCompactedSkillURLs(skills []model.SuperagentSkill) []string {
	seen := make(map[string]bool)
	var urls []string
	for _, skill := range skills {
		if skill.Compacted && skill.MCPURL != "" && !seen[skill.MCPURL] {
			seen[skill.MCPURL] = true
			urls = append(urls, skill.MCPURL)
		}
	}
	return urls
}

// MCPToolIntrospectionTools returns the built-in introspection tool definitions.
func MCPToolIntrospectionTools() []model.LLMToolDefinition {
	return mcpToolIntrospectionTools
}

// NewMCPToolInterceptorFromCache creates a ToolInterceptor using a pre-populated
// tool definition cache (map of MCP URL → tool definitions). This is used in
// distributed mode where tools were already listed during skill resolution.
func NewMCPToolInterceptorFromCache(toolCache map[string][]model.ToolDefinition, scopedURLs []string) ToolInterceptor {
	allowed := make(map[string]bool, len(scopedURLs))
	for _, url := range scopedURLs {
		allowed[url] = true
	}

	return func(toolName string, argsRaw json.RawMessage) (string, bool) {
		switch toolName {
		case "_list_mcp_tools":
			return handleListMCPToolsFromCache(toolCache, allowed, argsRaw), true
		case "_describe_mcp_tool":
			return handleDescribeMCPToolFromCache(toolCache, allowed, argsRaw), true
		default:
			return "", false
		}
	}
}

func handleListMCPToolsFromCache(toolCache map[string][]model.ToolDefinition, allowed map[string]bool, argsRaw json.RawMessage) string {
	var args struct {
		MCPURL string `json:"mcp_url"`
	}
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		return fmt.Sprintf("Error: invalid arguments: %v", err)
	}
	if args.MCPURL == "" {
		return "Error: mcp_url is required"
	}
	if !allowed[args.MCPURL] {
		return fmt.Sprintf("Error: MCP server %q is not available in this context", args.MCPURL)
	}

	tools, ok := toolCache[args.MCPURL]
	if !ok {
		return fmt.Sprintf("Error: no cached tools for MCP server %q", args.MCPURL)
	}

	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}

	out, _ := json.Marshal(names)
	return string(out)
}

func handleDescribeMCPToolFromCache(toolCache map[string][]model.ToolDefinition, allowed map[string]bool, argsRaw json.RawMessage) string {
	var args struct {
		MCPURL   string `json:"mcp_url"`
		ToolName string `json:"tool_name"`
	}
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		return fmt.Sprintf("Error: invalid arguments: %v", err)
	}
	if args.MCPURL == "" {
		return "Error: mcp_url is required"
	}
	if args.ToolName == "" {
		return "Error: tool_name is required"
	}
	if !allowed[args.MCPURL] {
		return fmt.Sprintf("Error: MCP server %q is not available in this context", args.MCPURL)
	}

	tools, ok := toolCache[args.MCPURL]
	if !ok {
		return fmt.Sprintf("Error: no cached tools for MCP server %q", args.MCPURL)
	}

	for _, t := range tools {
		if t.Name == args.ToolName {
			desc := map[string]any{
				"name":         t.Name,
				"description":  t.Description,
				"input_schema": t.InputSchema,
			}
			out, _ := json.Marshal(desc)
			return string(out)
		}
	}

	return fmt.Sprintf("Error: tool %q not found on MCP server %q", args.ToolName, args.MCPURL)
}

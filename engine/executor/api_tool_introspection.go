package executor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/brockleyai/brockleyai/internal/model"
)

// apiToolIntrospectionTools are built-in tools that let the LLM discover API
// tool endpoints at runtime. They are injected into the tool list when
// api_tools refs are present in the node config.
var apiToolIntrospectionTools = []model.LLMToolDefinition{
	{
		Name:        "_list_api_tools",
		Description: "List the available endpoints for a given API tool. Returns an array of endpoint names that can be passed to _describe_api_tool for full schema details.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"api_tool_id": {
					"type": "string",
					"description": "The ID of the API tool definition to list endpoints for."
				}
			},
			"required": ["api_tool_id"]
		}`),
	},
	{
		Name:        "_describe_api_tool",
		Description: "Describe a specific endpoint of an API tool, including its full input schema, HTTP method, path, and description.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"api_tool_id": {
					"type": "string",
					"description": "The ID of the API tool definition."
				},
				"endpoint": {
					"type": "string",
					"description": "The name of the endpoint to describe."
				}
			},
			"required": ["api_tool_id", "endpoint"]
		}`),
	},
}

// NewAPIToolInterceptor creates a ToolInterceptor that handles the
// _list_api_tools and _describe_api_tool built-in introspection tools.
// scopedIDs limits which API tool IDs the LLM is allowed to introspect.
func NewAPIToolInterceptor(dispatcher *APIToolDispatcher, tenantID string, scopedIDs []string) ToolInterceptor {
	allowed := make(map[string]bool, len(scopedIDs))
	for _, id := range scopedIDs {
		allowed[id] = true
	}

	return func(toolName string, argsRaw json.RawMessage) (string, bool) {
		switch toolName {
		case "_list_api_tools":
			return handleListAPITools(dispatcher, tenantID, allowed, argsRaw), true
		case "_describe_api_tool":
			return handleDescribeAPITool(dispatcher, tenantID, allowed, argsRaw), true
		default:
			return "", false
		}
	}
}

func handleListAPITools(dispatcher *APIToolDispatcher, tenantID string, allowed map[string]bool, argsRaw json.RawMessage) string {
	var args struct {
		APIToolID string `json:"api_tool_id"`
	}
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		return fmt.Sprintf("Error: invalid arguments: %v", err)
	}
	if args.APIToolID == "" {
		return "Error: api_tool_id is required"
	}
	if !allowed[args.APIToolID] {
		return fmt.Sprintf("Error: API tool %q is not available in this context", args.APIToolID)
	}

	def, err := dispatcher.ResolveDefinition(context.Background(), tenantID, args.APIToolID)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	names := make([]string, len(def.Endpoints))
	for i, ep := range def.Endpoints {
		names[i] = ep.Name
	}

	out, _ := json.Marshal(names)
	return string(out)
}

func handleDescribeAPITool(dispatcher *APIToolDispatcher, tenantID string, allowed map[string]bool, argsRaw json.RawMessage) string {
	var args struct {
		APIToolID string `json:"api_tool_id"`
		Endpoint  string `json:"endpoint"`
	}
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		return fmt.Sprintf("Error: invalid arguments: %v", err)
	}
	if args.APIToolID == "" {
		return "Error: api_tool_id is required"
	}
	if args.Endpoint == "" {
		return "Error: endpoint is required"
	}
	if !allowed[args.APIToolID] {
		return fmt.Sprintf("Error: API tool %q is not available in this context", args.APIToolID)
	}

	def, err := dispatcher.ResolveDefinition(context.Background(), tenantID, args.APIToolID)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	ep := FindEndpoint(def, args.Endpoint)
	if ep == nil {
		return fmt.Sprintf("Error: endpoint %q not found in API tool %q", args.Endpoint, args.APIToolID)
	}

	desc := map[string]any{
		"name":         ep.Name,
		"description":  ep.Description,
		"method":       ep.Method,
		"path":         ep.Path,
		"input_schema": json.RawMessage(ep.InputSchema),
	}
	out, _ := json.Marshal(desc)
	return string(out)
}

// ChainInterceptors combines multiple ToolInterceptors into one.
// The first interceptor that handles the call (returns handled=true) wins.
func ChainInterceptors(interceptors ...ToolInterceptor) ToolInterceptor {
	return func(toolName string, args json.RawMessage) (string, bool) {
		for _, ic := range interceptors {
			if ic == nil {
				continue
			}
			if result, handled := ic(toolName, args); handled {
				return result, true
			}
		}
		return "", false
	}
}

// uniqueAPIToolIDs collects the unique api_tool_id values from a slice of APIToolRefs.
func uniqueAPIToolIDs(refs []model.APIToolRef) []string {
	seen := make(map[string]bool, len(refs))
	var ids []string
	for _, ref := range refs {
		if !seen[ref.APIToolID] {
			seen[ref.APIToolID] = true
			ids = append(ids, ref.APIToolID)
		}
	}
	return ids
}

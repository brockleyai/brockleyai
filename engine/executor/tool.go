package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/brockleyai/brockleyai/engine/expression"
	"github.com/brockleyai/brockleyai/engine/mcp"
	"github.com/brockleyai/brockleyai/internal/model"
)

// ToolExecutor handles nodes of type "tool".
// It creates an MCP client, calls the configured tool, and maps the result to output ports.
type ToolExecutor struct{}

var _ NodeExecutor = (*ToolExecutor)(nil)

func (e *ToolExecutor) Execute(ctx context.Context, node *model.Node, inputs map[string]any, nctx *NodeContext, deps *ExecutorDeps) (*NodeResult, error) {
	var cfg model.ToolNodeConfig
	if err := json.Unmarshal(node.Config, &cfg); err != nil {
		return nil, fmt.Errorf("tool executor: invalid config: %w", err)
	}

	if cfg.MCPURL == "" {
		return nil, fmt.Errorf("tool executor: mcp_url is required")
	}
	if cfg.ToolName == "" {
		return nil, fmt.Errorf("tool executor: tool_name is required")
	}

	// Resolve headers.
	headers := make(map[string]string)
	for _, hc := range cfg.Headers {
		val, err := resolveHeaderValue(ctx, hc, inputs, nctx, deps)
		if err != nil {
			return nil, fmt.Errorf("tool executor: resolving header %q: %w", hc.Name, err)
		}
		headers[hc.Name] = val
	}

	// Create MCP client and call the tool.
	client := mcp.NewClient(cfg.MCPURL, headers)
	result, err := client.CallTool(ctx, cfg.ToolName, inputs)
	if err != nil {
		return nil, fmt.Errorf("tool executor: calling tool %q: %w", cfg.ToolName, err)
	}

	if result.IsError {
		return nil, fmt.Errorf("tool executor: tool %q returned error: %s", cfg.ToolName, result.Error)
	}

	outputs := map[string]any{
		"result": result.Content,
	}
	return &NodeResult{Outputs: outputs}, nil
}

// resolveHeaderValue resolves a HeaderConfig to its string value.
// It supports static values, dynamic values from input ports, secret references,
// and template values containing {{}} expressions.
func resolveHeaderValue(ctx context.Context, hc model.HeaderConfig, inputs map[string]any, nctx *NodeContext, deps *ExecutorDeps) (string, error) {
	if hc.SecretRef != "" {
		if deps == nil || deps.SecretStore == nil {
			return "", fmt.Errorf("secret store not available for secret_ref %q", hc.SecretRef)
		}
		return deps.SecretStore.GetSecret(ctx, hc.SecretRef)
	}

	if hc.FromInput != "" {
		val, ok := inputs[hc.FromInput]
		if !ok {
			return "", fmt.Errorf("input port %q not found", hc.FromInput)
		}
		return fmt.Sprintf("%v", val), nil
	}

	// Support template expressions in header values.
	if strings.Contains(hc.Value, "{{") {
		exprCtx := &expression.Context{Input: inputs}
		if nctx != nil {
			exprCtx.State = nctx.State
			exprCtx.Meta = nctx.Meta
		}
		return expression.RenderTemplate(hc.Value, exprCtx)
	}

	return hc.Value, nil
}

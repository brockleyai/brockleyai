package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/brockleyai/brockleyai/internal/model"
)

// APIToolNodeExecutor handles nodes of type "api_tool".
// It calls a single REST API endpoint (like ToolExecutor calls a single MCP tool).
type APIToolNodeExecutor struct{}

var _ NodeExecutor = (*APIToolNodeExecutor)(nil)

func (e *APIToolNodeExecutor) Execute(ctx context.Context, node *model.Node, inputs map[string]any, nctx *NodeContext, deps *ExecutorDeps) (*NodeResult, error) {
	var cfg model.APIToolNodeConfig
	if err := json.Unmarshal(node.Config, &cfg); err != nil {
		return nil, fmt.Errorf("api_tool executor: invalid config: %w", err)
	}

	if cfg.InlineEndpoint != nil {
		return executeInlineAPITool(ctx, cfg, inputs, nctx, deps)
	}

	// Referenced definition.
	if deps.APIToolDispatcher == nil {
		return nil, fmt.Errorf("api_tool executor: APIToolDispatcher is required")
	}

	tenantID := ""
	if nctx != nil {
		tenantID, _ = nctx.Meta["tenant_id"].(string)
	}

	route := model.ToolRoute{
		APIToolID:   cfg.APIToolID,
		APIEndpoint: cfg.Endpoint,
		Headers:     cfg.Headers,
	}

	result, err := deps.APIToolDispatcher.CallEndpoint(ctx, tenantID, route, cfg.Endpoint, inputs)
	if err != nil {
		return nil, fmt.Errorf("api_tool executor: %w", err)
	}
	if result.IsError {
		return nil, fmt.Errorf("api_tool executor: endpoint returned error: %s", result.Error)
	}

	return &NodeResult{Outputs: map[string]any{"result": result.Content}}, nil
}

// executeInlineAPITool handles inline endpoint definitions (self-contained graphs).
func executeInlineAPITool(ctx context.Context, cfg model.APIToolNodeConfig, inputs map[string]any, nctx *NodeContext, deps *ExecutorDeps) (*NodeResult, error) {
	ep := cfg.InlineEndpoint

	// Build a temporary definition for the dispatcher to use.
	def := &model.APIToolDefinition{
		BaseURL:        ep.BaseURL,
		DefaultHeaders: ep.DefaultHeaders,
		Retry:          ep.Retry,
		Endpoints: []model.APIEndpoint{
			{
				Name:            "inline",
				Method:          ep.Method,
				Path:            ep.Path,
				InputSchema:     ep.InputSchema,
				OutputSchema:    ep.OutputSchema,
				RequestMapping:  ep.RequestMapping,
				ResponseMapping: ep.ResponseMapping,
				TimeoutMs:       ep.TimeoutMs,
			},
		},
	}

	endpoint := &def.Endpoints[0]

	// Create a minimal dispatcher for inline execution.
	logger := slog.Default()
	if deps != nil && deps.Logger != nil {
		logger = deps.Logger
	}
	dispatcher := &APIToolDispatcher{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     logger,
	}

	route := model.ToolRoute{Headers: cfg.Headers}
	result, err := dispatcher.executeHTTPCall(ctx, def, endpoint, route, inputs)
	if err != nil {
		return nil, fmt.Errorf("api_tool executor (inline): %w", err)
	}
	if result.IsError {
		return nil, fmt.Errorf("api_tool executor (inline): endpoint returned error: %s", result.Error)
	}

	return &NodeResult{Outputs: map[string]any{"result": result.Content}}, nil
}

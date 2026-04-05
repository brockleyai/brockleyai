// Package mcp implements the MCP (Model Context Protocol) client for Brockley.
// It communicates with MCP tool servers over HTTP using JSON-RPC 2.0.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/brockleyai/brockleyai/internal/model"
)

// Client is an HTTP-based MCP client that speaks JSON-RPC 2.0 to an MCP server.
type Client struct {
	url        string
	headers    map[string]string
	httpClient *http.Client
}

var _ model.MCPClient = (*Client)(nil)

// NewClient creates a new MCP client targeting the given URL.
// Custom headers are applied to every request.
func NewClient(url string, headers map[string]string) *Client {
	return &Client{
		url:        url,
		headers:    headers,
		httpClient: &http.Client{},
	}
}

// jsonRPCRequest is a JSON-RPC 2.0 request envelope.
type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// jsonRPCResponse is a JSON-RPC 2.0 response envelope.
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

// jsonRPCError represents a JSON-RPC 2.0 error object.
type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// listToolsResult is the typed result for tools/list.
type listToolsResult struct {
	Tools []toolDef `json:"tools"`
}

// toolDef is a single tool in the tools/list response.
type toolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

// callToolParams is the params envelope for tools/call.
type callToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// callToolResult is the typed result for tools/call.
type callToolResult struct {
	Content []contentBlock `json:"content"`
	IsError bool           `json:"isError"`
}

// contentBlock is a single content element in a tool call response.
type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ListTools queries the MCP server for available tools.
func (c *Client) ListTools(ctx context.Context) ([]model.ToolDefinition, error) {
	resp, err := c.call(ctx, "tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("mcp list tools: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("mcp list tools: server error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	var result listToolsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("mcp list tools: parsing result: %w", err)
	}

	defs := make([]model.ToolDefinition, len(result.Tools))
	for i, t := range result.Tools {
		defs[i] = model.ToolDefinition{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}
	return defs, nil
}

// CallTool invokes a named tool on the MCP server with the given arguments.
func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]any) (*model.ToolResult, error) {
	params := callToolParams{
		Name:      name,
		Arguments: arguments,
	}

	resp, err := c.call(ctx, "tools/call", params)
	if err != nil {
		return nil, fmt.Errorf("mcp call tool %q: %w", name, err)
	}

	if resp.Error != nil {
		return &model.ToolResult{
			IsError: true,
			Error:   fmt.Sprintf("server error %d: %s", resp.Error.Code, resp.Error.Message),
		}, nil
	}

	var result callToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("mcp call tool %q: parsing result: %w", name, err)
	}

	if result.IsError {
		// Concatenate all text content blocks as the error message.
		errMsg := ""
		for _, block := range result.Content {
			if block.Type == "text" {
				if errMsg != "" {
					errMsg += "\n"
				}
				errMsg += block.Text
			}
		}
		return &model.ToolResult{
			IsError: true,
			Error:   errMsg,
		}, nil
	}

	// Build content from response blocks.
	if len(result.Content) == 1 && result.Content[0].Type == "text" {
		return &model.ToolResult{
			Content: result.Content[0].Text,
			IsError: false,
		}, nil
	}

	// Multiple content blocks: return as a slice of maps.
	blocks := make([]map[string]string, len(result.Content))
	for i, b := range result.Content {
		blocks[i] = map[string]string{
			"type": b.Type,
			"text": b.Text,
		}
	}
	return &model.ToolResult{
		Content: blocks,
		IsError: false,
	}, nil
}

// call sends a JSON-RPC 2.0 request and returns the parsed response.
func (c *Client) call(ctx context.Context, method string, params any) (*jsonRPCResponse, error) {
	reqBody := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", httpResp.StatusCode, string(respBody))
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return &rpcResp, nil
}

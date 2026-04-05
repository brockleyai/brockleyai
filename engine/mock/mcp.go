package mock

import (
	"context"
	"fmt"

	"github.com/brockleyai/brockleyai/internal/model"
)

// MockTool defines a tool that the MockMCPClient exposes.
type MockTool struct {
	Definition model.ToolDefinition
	// Handler is called when the tool is invoked. If nil, a default
	// successful result is returned.
	Handler func(args map[string]any) (*model.ToolResult, error)
}

// MCPCall records a single tool invocation.
type MCPCall struct {
	Name string
	Args map[string]any
}

// MockMCPClient is a test double for model.MCPClient.
type MockMCPClient struct {
	// Tools maps tool name to MockTool definition and behavior.
	Tools map[string]MockTool

	// Calls records every tool invocation in order.
	Calls []MCPCall
}

var _ model.MCPClient = (*MockMCPClient)(nil)

func (m *MockMCPClient) ListTools(ctx context.Context) ([]model.ToolDefinition, error) {
	defs := make([]model.ToolDefinition, 0, len(m.Tools))
	for _, t := range m.Tools {
		defs = append(defs, t.Definition)
	}
	return defs, nil
}

func (m *MockMCPClient) CallTool(ctx context.Context, name string, arguments map[string]any) (*model.ToolResult, error) {
	m.Calls = append(m.Calls, MCPCall{Name: name, Args: arguments})

	tool, ok := m.Tools[name]
	if !ok {
		return nil, fmt.Errorf("mock mcp: unknown tool %q", name)
	}

	if tool.Handler != nil {
		return tool.Handler(arguments)
	}

	return &model.ToolResult{
		Content: map[string]any{"status": "ok"},
		IsError: false,
	}, nil
}

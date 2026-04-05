package executor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/brockleyai/brockleyai/engine/mock"
	"github.com/brockleyai/brockleyai/internal/model"
)

func newToolLoopDeps(provider *mock.MockLLMProvider, mcpClient *mock.MockMCPClient) *ExecutorDeps {
	reg := &mockProviderRegistry{
		providers: map[string]model.LLMProvider{"mock": provider},
	}
	cache := NewMCPClientCache()
	return &ExecutorDeps{
		ProviderRegistry: reg,
		SecretStore:      mock.NewMockSecretStore(),
		MCPClient:        mcpClient,
		MCPClientCache:   cache,
		EventEmitter:     &mock.MockEventEmitter{},
	}
}

func TestToolLoop_BasicMultiStep(t *testing.T) {
	// Simulate: LLM calls echo, gets result, then responds with text.
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			{
				// Turn 1: LLM requests tool call
				ToolCalls: []model.ToolCall{
					{ID: "call_1", Name: "echo", Arguments: json.RawMessage(`{"text":"hello"}`)},
				},
				FinishReason: "tool_calls",
			},
			{
				// Turn 2: LLM responds with text
				Content:      "The echo returned: hello",
				FinishReason: "stop",
			},
		},
	}

	mcpClient := &mock.MockMCPClient{
		Tools: map[string]mock.MockTool{
			"echo": {
				Definition: model.ToolDefinition{Name: "echo", Description: "Echo text"},
				Handler: func(args map[string]any) (*model.ToolResult, error) {
					return &model.ToolResult{Content: args["text"], IsError: false}, nil
				},
			},
		},
	}

	deps := newToolLoopDeps(provider, mcpClient)
	// Pre-populate the cache with the mock MCP client
	deps.MCPClientCache.clients["http://test-mcp:9001"] = mcpClient

	cfg := model.LLMNodeConfig{
		Provider:   "mock",
		Model:      "test-model",
		UserPrompt: "Echo hello",
		ToolLoop:   true,
		Tools: []model.LLMToolDefinition{
			{Name: "echo", Description: "Echo text", Parameters: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}}}`)},
		},
		ToolRouting: map[string]model.ToolRoute{
			"echo": {MCPURL: "http://test-mcp:9001"},
		},
		ResponseFormat: model.ResponseFormatText,
	}

	node := &model.Node{
		ID: "llm-loop", Name: "test-loop", Type: model.NodeTypeLLM,
		Config: mustJSON(cfg),
	}

	exec := &LLMExecutor{}
	result, err := exec.Execute(context.Background(), node, nil, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify outputs
	if result.Outputs["response_text"] != "The echo returned: hello" {
		t.Errorf("expected 'The echo returned: hello', got %v", result.Outputs["response_text"])
	}
	if result.Outputs["finish_reason"] != "stop" {
		t.Errorf("expected finish_reason=stop, got %v", result.Outputs["finish_reason"])
	}
	if result.Outputs["total_tool_calls"] != 1 {
		t.Errorf("expected total_tool_calls=1, got %v", result.Outputs["total_tool_calls"])
	}

	// Verify tool call history
	history, ok := result.Outputs["tool_call_history"].([]ToolCallHistoryEntry)
	if !ok {
		t.Fatalf("expected []ToolCallHistoryEntry, got %T", result.Outputs["tool_call_history"])
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
	if history[0].Name != "echo" {
		t.Errorf("expected tool name=echo, got %s", history[0].Name)
	}
	if history[0].IsError {
		t.Error("expected is_error=false")
	}
}

func TestToolLoop_SafetyLimits(t *testing.T) {
	// LLM keeps requesting tool calls — should stop at limit.
	maxCalls := 3
	var responses []mock.MockCompletionResponse
	for i := 0; i < 10; i++ {
		responses = append(responses, mock.MockCompletionResponse{
			ToolCalls: []model.ToolCall{
				{ID: "call_" + string(rune('a'+i)), Name: "echo", Arguments: json.RawMessage(`{"text":"loop"}`)},
			},
			FinishReason: "tool_calls",
		})
	}

	provider := &mock.MockLLMProvider{CompletionResponses: responses}

	mcpClient := &mock.MockMCPClient{
		Tools: map[string]mock.MockTool{
			"echo": {
				Definition: model.ToolDefinition{Name: "echo"},
				Handler: func(args map[string]any) (*model.ToolResult, error) {
					return &model.ToolResult{Content: "ok"}, nil
				},
			},
		},
	}

	deps := newToolLoopDeps(provider, mcpClient)
	deps.MCPClientCache.clients["http://test-mcp:9001"] = mcpClient

	cfg := model.LLMNodeConfig{
		Provider:     "mock",
		Model:        "test",
		UserPrompt:   "Loop forever",
		ToolLoop:     true,
		MaxToolCalls: &maxCalls,
		Tools: []model.LLMToolDefinition{
			{Name: "echo", Description: "Echo", Parameters: json.RawMessage(`{"type":"object"}`)},
		},
		ToolRouting: map[string]model.ToolRoute{
			"echo": {MCPURL: "http://test-mcp:9001"},
		},
	}

	node := &model.Node{ID: "llm-limit", Name: "test-limit", Type: model.NodeTypeLLM, Config: mustJSON(cfg)}

	exec := &LLMExecutor{}
	result, err := exec.Execute(context.Background(), node, nil, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outputs["finish_reason"] != "limit_reached" {
		t.Errorf("expected finish_reason=limit_reached, got %v", result.Outputs["finish_reason"])
	}

	totalCalls, ok := result.Outputs["total_tool_calls"].(int)
	if !ok {
		t.Fatalf("expected int total_tool_calls, got %T", result.Outputs["total_tool_calls"])
	}
	if totalCalls > maxCalls {
		t.Errorf("expected total_tool_calls <= %d, got %d", maxCalls, totalCalls)
	}
}

func TestToolLoop_UnknownToolRecovery(t *testing.T) {
	// LLM calls an unknown tool, gets error, then calls a known tool.
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			{
				// Turn 1: Call unknown tool
				ToolCalls: []model.ToolCall{
					{ID: "call_1", Name: "nonexistent", Arguments: json.RawMessage(`{}`)},
				},
				FinishReason: "tool_calls",
			},
			{
				// Turn 2: Call known tool after error
				ToolCalls: []model.ToolCall{
					{ID: "call_2", Name: "echo", Arguments: json.RawMessage(`{"text":"hello"}`)},
				},
				FinishReason: "tool_calls",
			},
			{
				// Turn 3: Final response
				Content:      "Done!",
				FinishReason: "stop",
			},
		},
	}

	mcpClient := &mock.MockMCPClient{
		Tools: map[string]mock.MockTool{
			"echo": {
				Definition: model.ToolDefinition{Name: "echo"},
				Handler: func(args map[string]any) (*model.ToolResult, error) {
					return &model.ToolResult{Content: "hello"}, nil
				},
			},
		},
	}

	deps := newToolLoopDeps(provider, mcpClient)
	deps.MCPClientCache.clients["http://test-mcp:9001"] = mcpClient

	cfg := model.LLMNodeConfig{
		Provider:   "mock",
		Model:      "test",
		UserPrompt: "Try tools",
		ToolLoop:   true,
		Tools: []model.LLMToolDefinition{
			{Name: "echo", Description: "Echo", Parameters: json.RawMessage(`{"type":"object"}`)},
		},
		ToolRouting: map[string]model.ToolRoute{
			"echo": {MCPURL: "http://test-mcp:9001"},
		},
	}

	node := &model.Node{ID: "llm-unknown", Name: "test-unknown", Type: model.NodeTypeLLM, Config: mustJSON(cfg)}

	exec := &LLMExecutor{}
	result, err := exec.Execute(context.Background(), node, nil, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outputs["finish_reason"] != "stop" {
		t.Errorf("expected finish_reason=stop, got %v", result.Outputs["finish_reason"])
	}
	if result.Outputs["total_tool_calls"] != 2 {
		t.Errorf("expected total_tool_calls=2, got %v", result.Outputs["total_tool_calls"])
	}

	history := result.Outputs["tool_call_history"].([]ToolCallHistoryEntry)
	if !history[0].IsError {
		t.Error("expected first tool call to be an error (unknown tool)")
	}
	if history[1].IsError {
		t.Error("expected second tool call to succeed")
	}
}

func TestToolLoop_MultipleToolCallsPerIteration(t *testing.T) {
	// LLM requests two tool calls in a single response.
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			{
				ToolCalls: []model.ToolCall{
					{ID: "call_1", Name: "echo", Arguments: json.RawMessage(`{"text":"a"}`)},
					{ID: "call_2", Name: "echo", Arguments: json.RawMessage(`{"text":"b"}`)},
				},
				FinishReason: "tool_calls",
			},
			{
				Content:      "Both done",
				FinishReason: "stop",
			},
		},
	}

	mcpClient := &mock.MockMCPClient{
		Tools: map[string]mock.MockTool{
			"echo": {
				Definition: model.ToolDefinition{Name: "echo"},
				Handler: func(args map[string]any) (*model.ToolResult, error) {
					return &model.ToolResult{Content: args["text"]}, nil
				},
			},
		},
	}

	deps := newToolLoopDeps(provider, mcpClient)
	deps.MCPClientCache.clients["http://test-mcp:9001"] = mcpClient

	cfg := model.LLMNodeConfig{
		Provider:   "mock",
		Model:      "test",
		UserPrompt: "Call two tools",
		ToolLoop:   true,
		Tools: []model.LLMToolDefinition{
			{Name: "echo", Description: "Echo", Parameters: json.RawMessage(`{"type":"object"}`)},
		},
		ToolRouting: map[string]model.ToolRoute{
			"echo": {MCPURL: "http://test-mcp:9001"},
		},
	}

	node := &model.Node{ID: "llm-multi", Name: "test-multi", Type: model.NodeTypeLLM, Config: mustJSON(cfg)}

	exec := &LLMExecutor{}
	result, err := exec.Execute(context.Background(), node, nil, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outputs["total_tool_calls"] != 2 {
		t.Errorf("expected total_tool_calls=2, got %v", result.Outputs["total_tool_calls"])
	}
	if result.Outputs["iterations"] != 1 {
		t.Errorf("expected iterations=1, got %v", result.Outputs["iterations"])
	}
}

func TestToolLoop_MessagesFromState(t *testing.T) {
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			{
				Content:      "Hello again!",
				FinishReason: "stop",
			},
		},
	}

	mcpClient := &mock.MockMCPClient{
		Tools: map[string]mock.MockTool{
			"echo": {Definition: model.ToolDefinition{Name: "echo"}},
		},
	}
	deps := newToolLoopDeps(provider, mcpClient)
	deps.MCPClientCache.clients["http://test-mcp:9001"] = mcpClient

	cfg := model.LLMNodeConfig{
		Provider:          "mock",
		Model:             "test",
		UserPrompt:        "Continue our conversation",
		ToolLoop:          true,
		MessagesFromState: "conversation",
		Tools: []model.LLMToolDefinition{
			{Name: "echo", Description: "Echo", Parameters: json.RawMessage(`{"type":"object"}`)},
		},
		ToolRouting: map[string]model.ToolRoute{
			"echo": {MCPURL: "http://test-mcp:9001"},
		},
	}

	node := &model.Node{ID: "llm-state-msgs", Name: "test-state-msgs", Type: model.NodeTypeLLM, Config: mustJSON(cfg)}

	nctx := &NodeContext{
		State: map[string]any{
			"conversation": []any{
				map[string]any{"role": "user", "content": "Hello!"},
				map[string]any{"role": "assistant", "content": "Hi there!"},
			},
		},
	}

	exec := &LLMExecutor{}
	result, err := exec.Execute(context.Background(), node, nil, nctx, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outputs["response_text"] != "Hello again!" {
		t.Errorf("expected 'Hello again!', got %v", result.Outputs["response_text"])
	}

	// Verify the messages included state messages
	msgs := result.Outputs["messages"].([]model.Message)
	// Should have: 2 from state + system + user = at least 3
	if len(msgs) < 3 {
		t.Errorf("expected at least 3 messages (2 from state + 1 user), got %d", len(msgs))
	}
}

func TestToolLoop_EventEmission(t *testing.T) {
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			{
				ToolCalls: []model.ToolCall{
					{ID: "call_1", Name: "echo", Arguments: json.RawMessage(`{"text":"hi"}`)},
				},
				FinishReason: "tool_calls",
			},
			{
				Content:      "Done",
				FinishReason: "stop",
			},
		},
	}

	mcpClient := &mock.MockMCPClient{
		Tools: map[string]mock.MockTool{
			"echo": {
				Definition: model.ToolDefinition{Name: "echo"},
				Handler: func(args map[string]any) (*model.ToolResult, error) {
					return &model.ToolResult{Content: "hi"}, nil
				},
			},
		},
	}

	emitter := &mock.MockEventEmitter{}
	deps := newToolLoopDeps(provider, mcpClient)
	deps.MCPClientCache.clients["http://test-mcp:9001"] = mcpClient
	deps.EventEmitter = emitter

	cfg := model.LLMNodeConfig{
		Provider:   "mock",
		Model:      "test",
		UserPrompt: "Test events",
		ToolLoop:   true,
		Tools: []model.LLMToolDefinition{
			{Name: "echo", Description: "Echo", Parameters: json.RawMessage(`{"type":"object"}`)},
		},
		ToolRouting: map[string]model.ToolRoute{
			"echo": {MCPURL: "http://test-mcp:9001"},
		},
	}

	node := &model.Node{ID: "llm-events", Name: "test-events", Type: model.NodeTypeLLM, Config: mustJSON(cfg)}

	exec := &LLMExecutor{}
	_, err := exec.Execute(context.Background(), node, nil, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have events: tool_call_started, tool_call_completed, tool_loop_iteration
	eventTypes := make(map[model.EventType]int)
	for _, ev := range emitter.Events {
		eventTypes[ev.Type]++
	}

	if eventTypes[model.EventToolCallStarted] != 1 {
		t.Errorf("expected 1 tool_call_started event, got %d", eventTypes[model.EventToolCallStarted])
	}
	if eventTypes[model.EventToolCallCompleted] != 1 {
		t.Errorf("expected 1 tool_call_completed event, got %d", eventTypes[model.EventToolCallCompleted])
	}
	if eventTypes[model.EventToolLoopIteration] != 1 {
		t.Errorf("expected 1 tool_loop_iteration event, got %d", eventTypes[model.EventToolLoopIteration])
	}
}

func TestRunToolLoop_InterceptorHandled(t *testing.T) {
	// LLM returns a tool call for _test_builtin. Interceptor handles it.
	// On second call, LLM returns text.
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			{
				ToolCalls: []model.ToolCall{
					{ID: "call_1", Name: "_test_builtin", Arguments: json.RawMessage(`{"key":"val"}`)},
				},
				FinishReason: "tool_calls",
			},
			{
				Content:      "Got the intercepted result",
				FinishReason: "stop",
			},
		},
	}

	deps := &ExecutorDeps{
		EventEmitter: &mock.MockEventEmitter{},
	}

	interceptor := func(toolName string, args json.RawMessage) (string, bool) {
		if toolName == "_test_builtin" {
			return "intercepted result", true
		}
		return "", false
	}

	result, err := RunToolLoop(context.Background(), ToolLoopConfig{
		Provider:      provider,
		Request:       &model.CompletionRequest{Model: "test"},
		Routing:       map[string]model.ToolRoute{},
		MaxCalls:      25,
		MaxIterations: 10,
		Interceptor:   interceptor,
	}, deps, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.FinishReason != "stop" {
		t.Errorf("expected finish_reason=stop, got %s", result.FinishReason)
	}
	if result.TotalToolCalls != 1 {
		t.Errorf("expected total_tool_calls=1, got %d", result.TotalToolCalls)
	}
	if len(result.History) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(result.History))
	}
	if result.History[0].Name != "_test_builtin" {
		t.Errorf("expected tool name=_test_builtin, got %s", result.History[0].Name)
	}
	if result.History[0].Result != "intercepted result" {
		t.Errorf("expected result='intercepted result', got %s", result.History[0].Result)
	}
	if result.History[0].IsError {
		t.Error("expected is_error=false for intercepted tool")
	}

	// Verify tool result message was appended for the LLM.
	foundToolMsg := false
	for _, msg := range result.Messages {
		if msg.Role == "tool" && msg.Content == "intercepted result" {
			foundToolMsg = true
			break
		}
	}
	if !foundToolMsg {
		t.Error("expected tool result message with intercepted content in messages")
	}
}

func TestRunToolLoop_InterceptorPassthrough(t *testing.T) {
	// Interceptor returns ("", false) — normal MCP dispatch should happen.
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			{
				ToolCalls: []model.ToolCall{
					{ID: "call_1", Name: "echo", Arguments: json.RawMessage(`{"text":"hello"}`)},
				},
				FinishReason: "tool_calls",
			},
			{
				Content:      "Done",
				FinishReason: "stop",
			},
		},
	}

	mcpClient := &mock.MockMCPClient{
		Tools: map[string]mock.MockTool{
			"echo": {
				Definition: model.ToolDefinition{Name: "echo"},
				Handler: func(args map[string]any) (*model.ToolResult, error) {
					return &model.ToolResult{Content: args["text"], IsError: false}, nil
				},
			},
		},
	}

	cache := NewMCPClientCache()
	cache.clients["http://test-mcp:9001"] = mcpClient

	deps := &ExecutorDeps{
		MCPClientCache: cache,
		EventEmitter:   &mock.MockEventEmitter{},
	}

	interceptor := func(toolName string, args json.RawMessage) (string, bool) {
		return "", false // always pass through
	}

	result, err := RunToolLoop(context.Background(), ToolLoopConfig{
		Provider: provider,
		Request:  &model.CompletionRequest{Model: "test"},
		Routing: map[string]model.ToolRoute{
			"echo": {MCPURL: "http://test-mcp:9001"},
		},
		MaxCalls:      25,
		MaxIterations: 10,
		Interceptor:   interceptor,
	}, deps, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.FinishReason != "stop" {
		t.Errorf("expected finish_reason=stop, got %s", result.FinishReason)
	}
	if result.TotalToolCalls != 1 {
		t.Errorf("expected total_tool_calls=1, got %d", result.TotalToolCalls)
	}
	// Verify MCP was actually called.
	if len(mcpClient.Calls) != 1 {
		t.Errorf("expected 1 MCP call, got %d", len(mcpClient.Calls))
	}
	if mcpClient.Calls[0].Name != "echo" {
		t.Errorf("expected MCP call to 'echo', got %s", mcpClient.Calls[0].Name)
	}
}

func TestRunToolLoop_InterceptorMixed(t *testing.T) {
	// LLM returns two tool calls: _builtin_tool (intercepted) and mcp_tool (MCP dispatch).
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			{
				ToolCalls: []model.ToolCall{
					{ID: "call_1", Name: "_builtin_tool", Arguments: json.RawMessage(`{"x":1}`)},
					{ID: "call_2", Name: "mcp_tool", Arguments: json.RawMessage(`{"y":2}`)},
				},
				FinishReason: "tool_calls",
			},
			{
				Content:      "Both handled",
				FinishReason: "stop",
			},
		},
	}

	mcpClient := &mock.MockMCPClient{
		Tools: map[string]mock.MockTool{
			"mcp_tool": {
				Definition: model.ToolDefinition{Name: "mcp_tool"},
				Handler: func(args map[string]any) (*model.ToolResult, error) {
					return &model.ToolResult{Content: "mcp_result", IsError: false}, nil
				},
			},
		},
	}

	cache := NewMCPClientCache()
	cache.clients["http://test-mcp:9001"] = mcpClient

	deps := &ExecutorDeps{
		MCPClientCache: cache,
		EventEmitter:   &mock.MockEventEmitter{},
	}

	interceptor := func(toolName string, args json.RawMessage) (string, bool) {
		if toolName == "_builtin_tool" {
			return "builtin_result", true
		}
		return "", false
	}

	result, err := RunToolLoop(context.Background(), ToolLoopConfig{
		Provider: provider,
		Request:  &model.CompletionRequest{Model: "test"},
		Routing: map[string]model.ToolRoute{
			"mcp_tool": {MCPURL: "http://test-mcp:9001"},
		},
		MaxCalls:      25,
		MaxIterations: 10,
		Interceptor:   interceptor,
	}, deps, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.FinishReason != "stop" {
		t.Errorf("expected finish_reason=stop, got %s", result.FinishReason)
	}
	if result.TotalToolCalls != 2 {
		t.Errorf("expected total_tool_calls=2, got %d", result.TotalToolCalls)
	}
	if len(result.History) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(result.History))
	}

	// First entry: intercepted builtin tool.
	if result.History[0].Name != "_builtin_tool" {
		t.Errorf("expected first tool=_builtin_tool, got %s", result.History[0].Name)
	}
	if result.History[0].Result != "builtin_result" {
		t.Errorf("expected first result='builtin_result', got %s", result.History[0].Result)
	}

	// Second entry: MCP-dispatched tool.
	if result.History[1].Name != "mcp_tool" {
		t.Errorf("expected second tool=mcp_tool, got %s", result.History[1].Name)
	}
	if result.History[1].Result != `"mcp_result"` {
		t.Errorf("expected second result='\"mcp_result\"', got %s", result.History[1].Result)
	}

	// Verify MCP was called only for mcp_tool.
	if len(mcpClient.Calls) != 1 {
		t.Errorf("expected 1 MCP call, got %d", len(mcpClient.Calls))
	}

	// Verify both tool result messages are in the conversation.
	toolMsgs := 0
	for _, msg := range result.Messages {
		if msg.Role == "tool" {
			toolMsgs++
		}
	}
	if toolMsgs != 2 {
		t.Errorf("expected 2 tool messages, got %d", toolMsgs)
	}
}

func TestToolLoop_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			{Content: "never reached", FinishReason: "stop"},
		},
	}

	deps := newToolLoopDeps(provider, nil)

	cfg := model.LLMNodeConfig{
		Provider:   "mock",
		Model:      "test",
		UserPrompt: "Cancelled",
		ToolLoop:   true,
		Tools: []model.LLMToolDefinition{
			{Name: "echo", Description: "Echo", Parameters: json.RawMessage(`{"type":"object"}`)},
		},
		ToolRouting: map[string]model.ToolRoute{
			"echo": {MCPURL: "http://test-mcp:9001"},
		},
	}

	node := &model.Node{ID: "llm-cancel", Name: "test-cancel", Type: model.NodeTypeLLM, Config: mustJSON(cfg)}

	exec := &LLMExecutor{}
	result, err := exec.Execute(ctx, node, nil, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outputs["finish_reason"] != "cancelled" {
		t.Errorf("expected finish_reason=cancelled, got %v", result.Outputs["finish_reason"])
	}
}

func TestToolLoop_CompactedMCP(t *testing.T) {
	// Compacted MCP route: the echo tool's MCP URL is marked compacted.
	// The LLM first calls _list_mcp_tools, then _describe_mcp_tool, then
	// calls the actual tool, then responds with text.
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			{
				// Turn 1: LLM calls _list_mcp_tools to discover tools
				ToolCalls: []model.ToolCall{
					{ID: "call_1", Name: "_list_mcp_tools", Arguments: json.RawMessage(`{"mcp_url":"http://test-mcp:9001"}`)},
				},
				FinishReason: "tool_calls",
			},
			{
				// Turn 2: LLM calls _describe_mcp_tool to get schema
				ToolCalls: []model.ToolCall{
					{ID: "call_2", Name: "_describe_mcp_tool", Arguments: json.RawMessage(`{"mcp_url":"http://test-mcp:9001","tool_name":"echo"}`)},
				},
				FinishReason: "tool_calls",
			},
			{
				// Turn 3: LLM calls the actual tool
				ToolCalls: []model.ToolCall{
					{ID: "call_3", Name: "echo", Arguments: json.RawMessage(`{"text":"hello"}`)},
				},
				FinishReason: "tool_calls",
			},
			{
				// Turn 4: Final text response
				Content:      "Echo said: hello",
				FinishReason: "stop",
			},
		},
	}

	mcpClient := &mock.MockMCPClient{
		Tools: map[string]mock.MockTool{
			"echo": {
				Definition: model.ToolDefinition{
					Name:        "echo",
					Description: "Echo text back",
					InputSchema: map[string]any{"type": "object", "properties": map[string]any{"text": map[string]any{"type": "string"}}},
				},
				Handler: func(args map[string]any) (*model.ToolResult, error) {
					return &model.ToolResult{Content: args["text"], IsError: false}, nil
				},
			},
		},
	}

	deps := newToolLoopDeps(provider, mcpClient)
	deps.MCPClientCache.clients["http://test-mcp:9001"] = mcpClient

	cfg := model.LLMNodeConfig{
		Provider:     "mock",
		Model:        "test-model",
		UserPrompt:   "Echo hello using the compacted MCP",
		ToolLoop:     true,
		SystemPrompt: "You have access to an MCP server with echo capabilities.",
		// No explicit tools — compacted mode relies on introspection
		ToolRouting: map[string]model.ToolRoute{
			"echo": {MCPURL: "http://test-mcp:9001", Compacted: true},
		},
		ResponseFormat: model.ResponseFormatText,
	}

	node := &model.Node{
		ID: "llm-compacted", Name: "test-compacted", Type: model.NodeTypeLLM,
		Config: mustJSON(cfg),
	}

	exec := &LLMExecutor{}
	result, err := exec.Execute(context.Background(), node, nil, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outputs["response_text"] != "Echo said: hello" {
		t.Errorf("expected 'Echo said: hello', got %v", result.Outputs["response_text"])
	}
	if result.Outputs["finish_reason"] != "stop" {
		t.Errorf("expected finish_reason=stop, got %v", result.Outputs["finish_reason"])
	}
	// 3 tool calls: _list_mcp_tools, _describe_mcp_tool, echo
	if result.Outputs["total_tool_calls"] != 3 {
		t.Errorf("expected total_tool_calls=3, got %v", result.Outputs["total_tool_calls"])
	}
}

func TestToolLoop_MixedCompactedAndNonCompacted(t *testing.T) {
	// Two MCP routes: one compacted (search), one non-compacted (echo).
	// The non-compacted tools should be auto-discovered normally.
	// The compacted tools should only be available via introspection.
	provider := &mock.MockLLMProvider{
		CompletionResponses: []mock.MockCompletionResponse{
			{
				// LLM calls the non-compacted echo tool directly (auto-discovered)
				ToolCalls: []model.ToolCall{
					{ID: "call_1", Name: "echo", Arguments: json.RawMessage(`{"text":"hi"}`)},
				},
				FinishReason: "tool_calls",
			},
			{
				// LLM calls _list_mcp_tools for the compacted server
				ToolCalls: []model.ToolCall{
					{ID: "call_2", Name: "_list_mcp_tools", Arguments: json.RawMessage(`{"mcp_url":"http://search-mcp:9002"}`)},
				},
				FinishReason: "tool_calls",
			},
			{
				// Final response
				Content:      "Done",
				FinishReason: "stop",
			},
		},
	}

	echoClient := &mock.MockMCPClient{
		Tools: map[string]mock.MockTool{
			"echo": {
				Definition: model.ToolDefinition{
					Name:        "echo",
					Description: "Echo text",
					InputSchema: map[string]any{"type": "object"},
				},
				Handler: func(args map[string]any) (*model.ToolResult, error) {
					return &model.ToolResult{Content: "echoed"}, nil
				},
			},
		},
	}

	searchClient := &mock.MockMCPClient{
		Tools: map[string]mock.MockTool{
			"search": {
				Definition: model.ToolDefinition{
					Name:        "search",
					Description: "Search the KB",
					InputSchema: map[string]any{"type": "object"},
				},
			},
		},
	}

	deps := newToolLoopDeps(provider, echoClient)
	deps.MCPClientCache.clients["http://echo-mcp:9001"] = echoClient
	deps.MCPClientCache.clients["http://search-mcp:9002"] = searchClient

	cfg := model.LLMNodeConfig{
		Provider:     "mock",
		Model:        "test-model",
		UserPrompt:   "Use tools",
		SystemPrompt: "You have echo and search capabilities.",
		ToolLoop:     true,
		ToolRouting: map[string]model.ToolRoute{
			"echo":   {MCPURL: "http://echo-mcp:9001", Compacted: false},
			"search": {MCPURL: "http://search-mcp:9002", Compacted: true},
		},
		ResponseFormat: model.ResponseFormatText,
	}

	node := &model.Node{
		ID: "llm-mixed", Name: "test-mixed", Type: model.NodeTypeLLM,
		Config: mustJSON(cfg),
	}

	exec := &LLMExecutor{}
	result, err := exec.Execute(context.Background(), node, nil, nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outputs["finish_reason"] != "stop" {
		t.Errorf("expected finish_reason=stop, got %v", result.Outputs["finish_reason"])
	}
	// 2 tool calls: echo + _list_mcp_tools
	if result.Outputs["total_tool_calls"] != 2 {
		t.Errorf("expected total_tool_calls=2, got %v", result.Outputs["total_tool_calls"])
	}
}

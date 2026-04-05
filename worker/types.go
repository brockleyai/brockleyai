package worker

import (
	"encoding/json"
	"fmt"

	"github.com/brockleyai/brockleyai/internal/model"
)

// Task type constants for distributed execution.
const (
	TaskTypeGraphStart = "graph:start"     // Orchestrator — stays alive, drives execution
	TaskTypeNodeRun    = "node:run"        // Execute one non-LLM node that needs a task (forEach, subgraph, tool)
	TaskTypeLLMCall    = "node:llm-call"   // One provider.Complete() call
	TaskTypeMCPCall    = "node:mcp-call"   // One MCP tool call OR tools/list call
	TaskTypeAPICall    = "node:api-call"   // One API tool HTTP call
	TaskTypeSuperagent = "node:superagent" // Superagent coordinator — stays alive, dispatches LLM/MCP as tasks
	TaskTypeCodeExec   = "node:code-exec"  // One Python code execution — handled by coderunner
)

// Queue names for distributed execution.
const (
	QueueOrchestrator = "orchestrator" // graph:start tasks
	QueueNodes        = "nodes"        // node:run, node:llm-call, node:mcp-call, node:api-call tasks
	QueueCode         = "code"         // node:code-exec tasks — processed by coderunner
)

// GraphStartTask is the payload for graph:start tasks.
// The orchestrator stays alive, walks the graph, and dispatches node tasks.
type GraphStartTask struct {
	ExecutionID string          `json:"execution_id"`
	GraphID     string          `json:"graph_id"`
	GraphName   string          `json:"graph_name"`
	TenantID    string          `json:"tenant_id"`
	Graph       json.RawMessage `json:"graph"`
	Input       json.RawMessage `json:"input"`
	Timeout     int             `json:"timeout,omitempty"`
}

// LLMCallTask is the payload for node:llm-call tasks.
// Executes one provider.Complete() call. Handles tool loop MCP dispatch.
type LLMCallTask struct {
	ExecutionID string                   `json:"execution_id"`
	RequestID   string                   `json:"request_id"`
	NodeID      string                   `json:"node_id"`
	Provider    string                   `json:"provider"`
	APIKey      string                   `json:"api_key"` // carried separately because CompletionRequest.APIKey is json:"-"
	Request     *model.CompletionRequest `json:"request"`
	ToolLoop    *ToolLoopState           `json:"tool_loop,omitempty"`
	ResultKey   string                   `json:"result_key,omitempty"` // If set, push result here instead of exec:{id}:results
	Attempt     int                      `json:"attempt"`
	RetryPolicy *model.RetryPolicy       `json:"retry_policy,omitempty"`
	Debug       bool                     `json:"debug,omitempty"`
}

// MCPCallTask is the payload for node:mcp-call tasks.
// Executes one client.CallTool() or client.ListTools() call.
type MCPCallTask struct {
	ExecutionID    string             `json:"execution_id"`
	RequestID      string             `json:"request_id"`
	NodeID         string             `json:"node_id"`
	Operation      string             `json:"operation"` // "call_tool" or "list_tools"
	MCPURL         string             `json:"mcp_url"`
	Headers        map[string]string  `json:"headers,omitempty"`
	ToolName       string             `json:"tool_name,omitempty"`
	Arguments      map[string]any     `json:"arguments,omitempty"`
	TimeoutSeconds int                `json:"timeout_seconds,omitempty"`
	ResultKey      string             `json:"result_key"`    // Redis key to push result to
	ForToolLoop    bool               `json:"for_tool_loop"` // true when dispatched from an LLM tool loop (result format differs)
	Attempt        int                `json:"attempt"`
	RetryPolicy    *model.RetryPolicy `json:"retry_policy,omitempty"`
}

// APICallTask is the payload for node:api-call tasks.
// Executes one API tool HTTP endpoint call.
type APICallTask struct {
	ExecutionID    string               `json:"execution_id"`
	RequestID      string               `json:"request_id"`
	NodeID         string               `json:"node_id"`
	TenantID       string               `json:"tenant_id"`
	APIToolID      string               `json:"api_tool_id"`
	APIEndpoint    string               `json:"api_endpoint"`
	Headers        []model.HeaderConfig `json:"headers,omitempty"`
	ToolName       string               `json:"tool_name"`
	Arguments      map[string]any       `json:"arguments,omitempty"`
	TimeoutSeconds int                  `json:"timeout_seconds,omitempty"`
	ResultKey      string               `json:"result_key"`
	ForToolLoop    bool                 `json:"for_tool_loop"`
	Attempt        int                  `json:"attempt"`
	RetryPolicy    *model.RetryPolicy   `json:"retry_policy,omitempty"`
}

// NodeRunTask is the payload for node:run tasks.
// Used for complex nodes like forEach, subgraph, and superagent.
type NodeRunTask struct {
	ExecutionID string          `json:"execution_id"`
	RequestID   string          `json:"request_id"`
	NodeID      string          `json:"node_id"`
	NodeType    string          `json:"node_type"`
	NodeConfig  json.RawMessage `json:"node_config"`
	Inputs      map[string]any  `json:"inputs"`
	State       map[string]any  `json:"state,omitempty"`
	Meta        map[string]any  `json:"meta,omitempty"`
	OutputPorts []model.Port    `json:"output_ports,omitempty"` // Needed by superagent for output resolution
	ResultKey   string          `json:"result_key"`             // Redis key to push result to
	Debug       bool            `json:"debug,omitempty"`
}

// NodeTaskResult is the result pushed to Redis by all node task handlers.
// The orchestrator BRPOPs from exec:{execution_id}:results to collect these.
type NodeTaskResult struct {
	RequestID string               `json:"request_id"`
	NodeID    string               `json:"node_id"`
	Status    string               `json:"status"` // "completed" or "failed"
	Outputs   map[string]any       `json:"outputs,omitempty"`
	Error     string               `json:"error,omitempty"`
	Attempt   int                  `json:"attempt"`
	LLMUsage  *model.LLMUsage      `json:"llm_usage,omitempty"`
	LLMDebug  *model.LLMDebugTrace `json:"llm_debug,omitempty"`
}

// ToolLoopState holds the serializable state for a tool loop iteration.
// Extracted from the inline state in llm_tool_loop.go for distributed execution.
type ToolLoopState struct {
	// MaxCalls is the total tool invocation limit.
	MaxCalls int `json:"max_calls"`
	// MaxIterations is the LLM round-trip limit.
	MaxIterations int `json:"max_iterations"`
	// Iteration is the current iteration counter.
	Iteration int `json:"iteration"`
	// TotalToolCalls is the running total of tool calls made.
	TotalToolCalls int `json:"total_tool_calls"`
	// History records all tool invocations.
	History []ToolCallHistoryEntry `json:"history,omitempty"`
	// Routing maps tool name → ToolRoute for MCP dispatch.
	Routing map[string]model.ToolRoute `json:"routing"`
	// FinishReason from the last LLM response (used if the loop exits early).
	FinishReason string `json:"finish_reason,omitempty"`
	// NodeConfig is the original LLM node config (needed for MCP header resolution).
	NodeConfig json.RawMessage `json:"node_config,omitempty"`
	// NodeInputs are the original node inputs (needed for header resolution).
	NodeInputs map[string]any `json:"node_inputs,omitempty"`
	// NodeState is the state snapshot (needed for header resolution).
	NodeState map[string]any `json:"node_state,omitempty"`
	// NodeMeta is the execution metadata.
	NodeMeta map[string]any `json:"node_meta,omitempty"`
	// DebugTraces accumulates per-call LLM traces across tool loop iterations.
	DebugTraces []model.LLMCallTrace `json:"debug_traces,omitempty"`
}

// ToolCallHistoryEntry records a single tool invocation during a tool loop.
// Mirrors executor.ToolCallHistoryEntry for serialization across task boundaries.
type ToolCallHistoryEntry struct {
	Name       string          `json:"name"`
	Arguments  json.RawMessage `json:"arguments"`
	Result     string          `json:"result"`
	DurationMs int64           `json:"duration_ms"`
	IsError    bool            `json:"is_error"`
}

// MCPCallResult is the result of an MCP tool call, pushed to a tool-loop-specific Redis key.
type MCPCallResult struct {
	RequestID  string `json:"request_id"`
	ToolCallID string `json:"tool_call_id"`
	ToolName   string `json:"tool_name"`
	Content    any    `json:"content,omitempty"`
	Error      string `json:"error,omitempty"`
	IsError    bool   `json:"is_error"`
	DurationMs int64  `json:"duration_ms"`
}

// APICallResult is the result of an API tool call, pushed to a tool-loop-specific Redis key.
type APICallResult struct {
	RequestID  string `json:"request_id"`
	ToolCallID string `json:"tool_call_id"`
	ToolName   string `json:"tool_name"`
	Content    any    `json:"content,omitempty"`
	Error      string `json:"error,omitempty"`
	IsError    bool   `json:"is_error"`
	DurationMs int64  `json:"duration_ms"`
}

// ResultKeyForExecution returns the Redis key for the main execution result queue.
func ResultKeyForExecution(executionID string) string {
	return "exec:" + executionID + ":results"
}

// ResultKeyForLLMCall returns the Redis key for MCP results within a tool loop.
func ResultKeyForLLMCall(executionID, requestID string) string {
	return "exec:" + executionID + ":llm:" + requestID + ":mcp-results"
}

// ResultKeyForSuperagent returns a scoped Redis key for superagent internal LLM/MCP calls.
// Each call gets a unique key so results don't mix with the orchestrator's main result queue.
func ResultKeyForSuperagent(executionID, nodeID string, seq int64) string {
	return fmt.Sprintf("sa:%s:%s:%d", executionID, nodeID, seq)
}

// --- Code Execution Types ---

// CodeExecTask is the payload for node:code-exec tasks.
// Sent by the superagent handler, consumed by the coderunner.
type CodeExecTask struct {
	ExecutionID         string   `json:"execution_id"`
	NodeID              string   `json:"node_id"`
	Seq                 int64    `json:"seq"`
	Code                string   `json:"code"`
	MaxExecutionTimeSec int      `json:"max_execution_time_sec"`
	MaxMemoryMB         int      `json:"max_memory_mb"`
	MaxOutputBytes      int      `json:"max_output_bytes"`
	MaxCodeBytes        int      `json:"max_code_bytes"`
	MaxToolCalls        int      `json:"max_tool_calls"`
	AllowedModules      []string `json:"allowed_modules,omitempty"`
	CallbackKey         string   `json:"callback_key"`
	ResponseKey         string   `json:"response_key"`
	ResultKey           string   `json:"result_key"`
}

// CodeExecResult is the final result pushed by the coderunner to the result key.
type CodeExecResult struct {
	Status     string `json:"status"` // "completed", "error", "timeout", "cancelled"
	Output     string `json:"output"` // from brockley.output()
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	Error      string `json:"error,omitempty"`
	Traceback  string `json:"traceback,omitempty"`
	ToolCalls  int    `json:"tool_calls"`
	DurationMs int64  `json:"duration_ms"`
}

// CodeToolRequest is a tool call request from Python code, relayed by the coderunner.
type CodeToolRequest struct {
	Type      string         `json:"type"` // "tool_call" or "cancel"
	Name      string         `json:"name,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Seq       int            `json:"seq"`
}

// CodeToolResponse is a tool call response from the superagent handler.
type CodeToolResponse struct {
	Type    string `json:"type"` // "result", "error", "cancel"
	Content string `json:"content,omitempty"`
	IsError bool   `json:"is_error,omitempty"`
	Seq     int    `json:"seq"`
}

// CodeExecCallbackKey returns the Redis key for coderunner → handler tool requests.
func CodeExecCallbackKey(executionID, nodeID string, seq int64) string {
	return fmt.Sprintf("codeexec:%s:%s:%d:callbacks", executionID, nodeID, seq)
}

// CodeExecResponseKey returns the Redis key for handler → coderunner tool responses.
func CodeExecResponseKey(executionID, nodeID string, seq int64) string {
	return fmt.Sprintf("codeexec:%s:%s:%d:responses", executionID, nodeID, seq)
}

// CodeExecResultKey returns the Redis key for coderunner → handler final result.
func CodeExecResultKey(executionID, nodeID string, seq int64) string {
	return fmt.Sprintf("codeexec:%s:%s:%d:result", executionID, nodeID, seq)
}

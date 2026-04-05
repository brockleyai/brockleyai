package provider

import (
	"encoding/json"

	"github.com/brockleyai/brockleyai/internal/model"
)

// ToolCallCodec handles translation between unified tool types and provider wire format.
// Each provider implements this to handle its specific JSON structure.
type ToolCallCodec interface {
	// EncodeTools converts unified LLMToolDefinitions to the provider's request format.
	EncodeTools(tools []model.LLMToolDefinition, toolChoice string) (json.RawMessage, error)

	// EncodeMessages converts unified Messages (including tool calls/results)
	// to the provider's message format.
	EncodeMessages(messages []model.Message) (json.RawMessage, error)

	// DecodeToolCalls extracts tool calls from the provider's response body.
	// Returns nil tool calls if the response contains none.
	DecodeToolCalls(responseBody json.RawMessage) ([]model.ToolCall, string, error) // tools, finishReason, err

	// DecodeStreamToolCalls accumulates tool call fragments from streaming chunks.
	// Call with each chunk; returns complete ToolCalls when the stream ends.
	DecodeStreamToolCalls(chunk json.RawMessage) ([]model.ToolCall, bool, error) // tools, complete, err
}

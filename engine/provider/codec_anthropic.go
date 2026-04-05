package provider

import (
	"encoding/json"
	"fmt"

	"github.com/brockleyai/brockleyai/internal/model"
)

// AnthropicCodec translates between unified tool types and Anthropic's wire format.
// Also used by Bedrock (same content block format).
type AnthropicCodec struct {
	// streamAccum accumulates tool use blocks during streaming.
	streamBlocks []anthropicToolUseBlock
	currentBlock *anthropicToolUseBlock
}

var _ ToolCallCodec = (*AnthropicCodec)(nil)

// anthropicToolDef is the wire format for a tool definition in Anthropic requests.
type anthropicToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// anthropicToolUseBlock is a tool_use content block in Anthropic responses.
type anthropicToolUseBlock struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// anthropicToolChoiceRequest is the tool_choice field for Anthropic requests.
type anthropicToolChoiceRequest struct {
	Type string `json:"type"`           // "auto", "any", "tool"
	Name string `json:"name,omitempty"` // only for type="tool"
}

func (c *AnthropicCodec) EncodeTools(tools []model.LLMToolDefinition, toolChoice string) (json.RawMessage, error) {
	type encoded struct {
		Tools      []anthropicToolDef          `json:"tools"`
		ToolChoice *anthropicToolChoiceRequest `json:"tool_choice,omitempty"`
	}
	e := encoded{}
	for _, t := range tools {
		e.Tools = append(e.Tools, anthropicToolDef{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Parameters,
		})
	}
	switch toolChoice {
	case "", "auto":
		e.ToolChoice = &anthropicToolChoiceRequest{Type: "auto"}
	case "none":
		// Anthropic doesn't have "none" — omit tools instead.
		// We still encode them but the caller should handle this.
		e.ToolChoice = nil
	case "required":
		e.ToolChoice = &anthropicToolChoiceRequest{Type: "any"}
	default:
		e.ToolChoice = &anthropicToolChoiceRequest{Type: "tool", Name: toolChoice}
	}
	return json.Marshal(e)
}

func (c *AnthropicCodec) EncodeMessages(messages []model.Message) (json.RawMessage, error) {
	type contentBlock struct {
		Type      string          `json:"type"`
		Text      string          `json:"text,omitempty"`
		ID        string          `json:"id,omitempty"`
		Name      string          `json:"name,omitempty"`
		Input     json.RawMessage `json:"input,omitempty"`
		ToolUseID string          `json:"tool_use_id,omitempty"`
		Content   string          `json:"content,omitempty"`
		IsError   bool            `json:"is_error,omitempty"`
	}
	type aMsg struct {
		Role    string         `json:"role"`
		Content []contentBlock `json:"content"`
	}
	var msgs []aMsg
	for _, m := range messages {
		if m.Role == "system" {
			continue // Anthropic handles system separately
		}
		if m.Role == "tool" {
			// Tool result → tool_result content block on a "user" message
			block := contentBlock{
				Type:      "tool_result",
				ToolUseID: m.ToolCallID,
				Content:   m.Content,
				IsError:   m.ToolResultError,
			}
			// Try to merge with previous user message
			if len(msgs) > 0 && msgs[len(msgs)-1].Role == "user" {
				msgs[len(msgs)-1].Content = append(msgs[len(msgs)-1].Content, block)
			} else {
				msgs = append(msgs, aMsg{Role: "user", Content: []contentBlock{block}})
			}
			continue
		}
		msg := aMsg{Role: m.Role}
		if len(m.ToolCalls) > 0 {
			// Assistant message with tool_use blocks
			if m.Content != "" {
				msg.Content = append(msg.Content, contentBlock{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				msg.Content = append(msg.Content, contentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: tc.Arguments,
				})
			}
		} else {
			msg.Content = []contentBlock{{Type: "text", Text: m.Content}}
		}
		msgs = append(msgs, msg)
	}
	return json.Marshal(msgs)
}

func (c *AnthropicCodec) DecodeToolCalls(responseBody json.RawMessage) ([]model.ToolCall, string, error) {
	var resp struct {
		Content []struct {
			Type  string          `json:"type"`
			ID    string          `json:"id,omitempty"`
			Name  string          `json:"name,omitempty"`
			Input json.RawMessage `json:"input,omitempty"`
			Text  string          `json:"text,omitempty"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
	}
	if err := json.Unmarshal(responseBody, &resp); err != nil {
		return nil, "", fmt.Errorf("anthropic codec: decoding response: %w", err)
	}

	var calls []model.ToolCall
	for _, block := range resp.Content {
		if block.Type == "tool_use" {
			calls = append(calls, model.ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: block.Input,
			})
		}
	}

	// Map Anthropic stop reasons to unified finish reasons
	finishReason := resp.StopReason
	switch finishReason {
	case "end_turn":
		finishReason = "stop"
	case "tool_use":
		finishReason = "tool_calls"
	case "max_tokens":
		finishReason = "length"
	}

	return calls, finishReason, nil
}

func (c *AnthropicCodec) DecodeStreamToolCalls(chunk json.RawMessage) ([]model.ToolCall, bool, error) {
	var event struct {
		Type         string `json:"type"`
		ContentBlock *struct {
			Type string `json:"type"`
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"content_block,omitempty"`
		Delta *struct {
			Type        string          `json:"type"`
			PartialJSON string          `json:"partial_json,omitempty"`
			StopReason  string          `json:"stop_reason,omitempty"`
			Input       json.RawMessage `json:"input,omitempty"`
		} `json:"delta,omitempty"`
	}
	if err := json.Unmarshal(chunk, &event); err != nil {
		return nil, false, fmt.Errorf("anthropic codec: decoding stream chunk: %w", err)
	}

	switch event.Type {
	case "content_block_start":
		if event.ContentBlock != nil && event.ContentBlock.Type == "tool_use" {
			c.currentBlock = &anthropicToolUseBlock{
				ID:   event.ContentBlock.ID,
				Name: event.ContentBlock.Name,
			}
		}
	case "content_block_delta":
		if c.currentBlock != nil && event.Delta != nil && event.Delta.Type == "input_json_delta" {
			c.currentBlock.Input = append(c.currentBlock.Input, []byte(event.Delta.PartialJSON)...)
		}
	case "content_block_stop":
		if c.currentBlock != nil {
			if len(c.currentBlock.Input) == 0 {
				c.currentBlock.Input = json.RawMessage("{}")
			}
			c.streamBlocks = append(c.streamBlocks, *c.currentBlock)
			c.currentBlock = nil
		}
	case "message_delta":
		if event.Delta != nil && event.Delta.StopReason == "tool_use" {
			var calls []model.ToolCall
			for _, block := range c.streamBlocks {
				calls = append(calls, model.ToolCall{
					ID:        block.ID,
					Name:      block.Name,
					Arguments: block.Input,
				})
			}
			c.streamBlocks = nil
			c.currentBlock = nil
			return calls, true, nil
		}
	}

	return nil, false, nil
}

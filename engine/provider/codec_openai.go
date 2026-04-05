package provider

import (
	"encoding/json"
	"fmt"

	"github.com/brockleyai/brockleyai/internal/model"
)

// OpenAICodec translates between unified tool types and OpenAI's wire format.
// Also used by OpenRouter (identical format).
type OpenAICodec struct {
	// streamAccum accumulates tool call fragments during streaming.
	streamAccum map[int]*openAIStreamToolCall
}

var _ ToolCallCodec = (*OpenAICodec)(nil)

// openAIToolDef is the wire format for a tool definition in OpenAI requests.
type openAIToolDef struct {
	Type     string            `json:"type"`
	Function openAIFunctionDef `json:"function"`
}

type openAIFunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// openAIToolCall is the wire format for a tool call in OpenAI responses.
type openAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// openAIStreamToolCall accumulates streaming tool call fragments.
type openAIStreamToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// openAIToolChoice is the wire format for tool_choice in OpenAI requests.
type openAIToolChoice struct {
	Type     string              `json:"type"`
	Function *openAIToolChoiceFC `json:"function,omitempty"`
}

type openAIToolChoiceFC struct {
	Name string `json:"name"`
}

func (c *OpenAICodec) EncodeTools(tools []model.LLMToolDefinition, toolChoice string) (json.RawMessage, error) {
	type encoded struct {
		Tools      []openAIToolDef `json:"tools"`
		ToolChoice any             `json:"tool_choice,omitempty"`
	}
	e := encoded{}
	for _, t := range tools {
		e.Tools = append(e.Tools, openAIToolDef{
			Type: "function",
			Function: openAIFunctionDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	switch toolChoice {
	case "", "auto":
		e.ToolChoice = "auto"
	case "none":
		e.ToolChoice = "none"
	case "required":
		e.ToolChoice = "required"
	default:
		// Specific tool name
		e.ToolChoice = openAIToolChoice{
			Type:     "function",
			Function: &openAIToolChoiceFC{Name: toolChoice},
		}
	}
	return json.Marshal(e)
}

func (c *OpenAICodec) EncodeMessages(messages []model.Message) (json.RawMessage, error) {
	type oaiMsg struct {
		Role       string           `json:"role"`
		Content    string           `json:"content,omitempty"`
		ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
		ToolCallID string           `json:"tool_call_id,omitempty"`
	}
	var msgs []oaiMsg
	for _, m := range messages {
		msg := oaiMsg{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
		for _, tc := range m.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, openAIToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
					Name:      tc.Name,
					Arguments: string(tc.Arguments),
				},
			})
		}
		msgs = append(msgs, msg)
	}
	return json.Marshal(msgs)
}

func (c *OpenAICodec) DecodeToolCalls(responseBody json.RawMessage) ([]model.ToolCall, string, error) {
	var resp struct {
		Choices []struct {
			Message struct {
				ToolCalls []openAIToolCall `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(responseBody, &resp); err != nil {
		return nil, "", fmt.Errorf("openai codec: decoding response: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, "", nil
	}
	choice := resp.Choices[0]
	if len(choice.Message.ToolCalls) == 0 {
		return nil, choice.FinishReason, nil
	}
	var calls []model.ToolCall
	for _, tc := range choice.Message.ToolCalls {
		calls = append(calls, model.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: json.RawMessage(tc.Function.Arguments),
		})
	}
	return calls, choice.FinishReason, nil
}

func (c *OpenAICodec) DecodeStreamToolCalls(chunk json.RawMessage) ([]model.ToolCall, bool, error) {
	var sc struct {
		Choices []struct {
			Delta struct {
				ToolCalls []struct {
					Index    int    `json:"index"`
					ID       string `json:"id,omitempty"`
					Type     string `json:"type,omitempty"`
					Function struct {
						Name      string `json:"name,omitempty"`
						Arguments string `json:"arguments,omitempty"`
					} `json:"function"`
				} `json:"tool_calls,omitempty"`
			} `json:"delta"`
			FinishReason *string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(chunk, &sc); err != nil {
		return nil, false, fmt.Errorf("openai codec: decoding stream chunk: %w", err)
	}
	if len(sc.Choices) == 0 {
		return nil, false, nil
	}

	if c.streamAccum == nil {
		c.streamAccum = make(map[int]*openAIStreamToolCall)
	}

	for _, tc := range sc.Choices[0].Delta.ToolCalls {
		existing, ok := c.streamAccum[tc.Index]
		if !ok {
			existing = &openAIStreamToolCall{}
			c.streamAccum[tc.Index] = existing
		}
		if tc.ID != "" {
			existing.ID = tc.ID
		}
		if tc.Function.Name != "" {
			existing.Name = tc.Function.Name
		}
		existing.Arguments += tc.Function.Arguments
	}

	// Check if stream is done
	if sc.Choices[0].FinishReason != nil && *sc.Choices[0].FinishReason == "tool_calls" {
		var calls []model.ToolCall
		for i := 0; i < len(c.streamAccum); i++ {
			tc := c.streamAccum[i]
			calls = append(calls, model.ToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: json.RawMessage(tc.Arguments),
			})
		}
		c.streamAccum = nil
		return calls, true, nil
	}

	return nil, false, nil
}

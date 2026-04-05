package provider

import (
	"encoding/json"
	"fmt"

	"github.com/brockleyai/brockleyai/internal/model"
)

// GoogleCodec translates between unified tool types and Google Gemini's wire format.
type GoogleCodec struct{}

var _ ToolCallCodec = (*GoogleCodec)(nil)

// googleToolDef is the wire format for tool definitions in Google requests.
type googleToolDef struct {
	FunctionDeclarations []googleFunctionDecl `json:"function_declarations"`
}

type googleFunctionDecl struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// googleFunctionCall and googleFunctionResponse are defined in google.go.

func (c *GoogleCodec) EncodeTools(tools []model.LLMToolDefinition, toolChoice string) (json.RawMessage, error) {
	type encoded struct {
		Tools []googleToolDef `json:"tools"`
	}
	e := encoded{}
	td := googleToolDef{}
	for _, t := range tools {
		td.FunctionDeclarations = append(td.FunctionDeclarations, googleFunctionDecl{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		})
	}
	e.Tools = []googleToolDef{td}
	return json.Marshal(e)
}

func (c *GoogleCodec) EncodeMessages(messages []model.Message) (json.RawMessage, error) {
	type part struct {
		Text             string                  `json:"text,omitempty"`
		FunctionCall     *googleFunctionCall     `json:"functionCall,omitempty"`
		FunctionResponse *googleFunctionResponse `json:"functionResponse,omitempty"`
	}
	type content struct {
		Role  string `json:"role"`
		Parts []part `json:"parts"`
	}
	var contents []content
	for _, m := range messages {
		if m.Role == "system" {
			continue // Google handles system separately via systemInstruction
		}
		if m.Role == "tool" {
			// Tool result → functionResponse part on a "user" content
			var resp map[string]any
			if err := json.Unmarshal([]byte(m.Content), &resp); err != nil {
				resp = map[string]any{"result": m.Content}
			}
			// Find the tool name from the tool_call_id
			toolName := m.ToolCallID // Google uses function name, not ID
			p := part{FunctionResponse: &googleFunctionResponse{Name: toolName, Response: resp}}
			contents = append(contents, content{Role: "user", Parts: []part{p}})
			continue
		}
		c := content{Role: m.Role}
		if m.Role == "assistant" {
			c.Role = "model"
		}
		if len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				c.Parts = append(c.Parts, part{
					FunctionCall: &googleFunctionCall{Name: tc.Name, Args: tc.Arguments},
				})
			}
			if m.Content != "" {
				c.Parts = append([]part{{Text: m.Content}}, c.Parts...)
			}
		} else {
			c.Parts = []part{{Text: m.Content}}
		}
		contents = append(contents, c)
	}
	return json.Marshal(contents)
}

func (c *GoogleCodec) DecodeToolCalls(responseBody json.RawMessage) ([]model.ToolCall, string, error) {
	var resp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text         string              `json:"text,omitempty"`
					FunctionCall *googleFunctionCall `json:"functionCall,omitempty"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(responseBody, &resp); err != nil {
		return nil, "", fmt.Errorf("google codec: decoding response: %w", err)
	}
	if len(resp.Candidates) == 0 {
		return nil, "", nil
	}

	var calls []model.ToolCall
	for i, part := range resp.Candidates[0].Content.Parts {
		if part.FunctionCall != nil {
			calls = append(calls, model.ToolCall{
				ID:        fmt.Sprintf("call_%d", i),
				Name:      part.FunctionCall.Name,
				Arguments: part.FunctionCall.Args,
			})
		}
	}

	// Map Google finish reasons to unified
	finishReason := resp.Candidates[0].FinishReason
	if len(calls) > 0 {
		finishReason = "tool_calls"
	} else {
		switch finishReason {
		case "STOP":
			finishReason = "stop"
		case "MAX_TOKENS":
			finishReason = "length"
		}
	}

	return calls, finishReason, nil
}

func (c *GoogleCodec) DecodeStreamToolCalls(chunk json.RawMessage) ([]model.ToolCall, bool, error) {
	// Google streams full candidates per chunk, so we can reuse DecodeToolCalls
	calls, finishReason, err := c.DecodeToolCalls(chunk)
	if err != nil {
		return nil, false, err
	}
	if len(calls) > 0 {
		return calls, true, nil
	}
	if finishReason == "stop" || finishReason == "length" {
		return nil, true, nil
	}
	return nil, false, nil
}

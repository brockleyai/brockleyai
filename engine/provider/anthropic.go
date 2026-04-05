package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/brockleyai/brockleyai/internal/model"
)

const defaultAnthropicBaseURL = "https://api.anthropic.com"

// AnthropicProvider implements model.LLMProvider for the Anthropic Messages API.
type AnthropicProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

var _ model.LLMProvider = (*AnthropicProvider)(nil)

// NewAnthropicProvider creates a new Anthropic provider.
// If baseURL is empty, the default Anthropic API URL is used.
func NewAnthropicProvider(apiKey, baseURL string) *AnthropicProvider {
	if baseURL == "" {
		baseURL = defaultAnthropicBaseURL
	}
	return &AnthropicProvider{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// anthropicRequest is the request body for the Anthropic messages API.
type anthropicRequest struct {
	Model      string                      `json:"model"`
	MaxTokens  int                         `json:"max_tokens"`
	System     string                      `json:"system,omitempty"`
	Messages   []anthropicMessage          `json:"messages"`
	Stream     bool                        `json:"stream,omitempty"`
	Tools      []anthropicToolDef          `json:"tools,omitempty"`
	ToolChoice *anthropicToolChoiceRequest `json:"tool_choice,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []anthropicContentBlock
}

// anthropicContentBlock is a typed content block in Anthropic messages.
type anthropicContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

// anthropicResponse is the response body from the Anthropic messages API.
type anthropicResponse struct {
	ID         string `json:"id"`
	Model      string `json:"model"`
	StopReason string `json:"stop_reason"`
	Content    []struct {
		Type  string          `json:"type"`
		Text  string          `json:"text,omitempty"`
		ID    string          `json:"id,omitempty"`
		Name  string          `json:"name,omitempty"`
		Input json.RawMessage `json:"input,omitempty"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type anthropicErrorResponse struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

type anthropicContentDelta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicMessageDelta struct {
	StopReason string `json:"stop_reason"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func (p *AnthropicProvider) buildSystemPrompt(req *model.CompletionRequest) string {
	system := req.SystemPrompt
	if req.ResponseFormat == model.ResponseFormatJSON && req.OutputSchema != nil {
		schemaJSON, err := json.Marshal(req.OutputSchema)
		if err == nil {
			suffix := "\n\nRespond with valid JSON matching this schema: " + string(schemaJSON)
			system += suffix
		}
	} else if req.ResponseFormat == model.ResponseFormatJSON {
		if system != "" {
			system += "\n\n"
		}
		system += "Respond with valid JSON."
	}
	return system
}

func (p *AnthropicProvider) buildBody(req *model.CompletionRequest, stream bool) *anthropicRequest {
	maxTokens := 4096
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}

	// Build messages: use chain if provided, otherwise single user message.
	var msgs []anthropicMessage
	systemPrompt := p.buildSystemPrompt(req)

	if len(req.Messages) > 0 {
		for _, m := range req.Messages {
			if m.Role == "system" {
				systemPrompt = m.Content
				continue
			}
			if m.Role == "tool" {
				// Tool result → tool_result content block on a "user" message
				block := anthropicContentBlock{
					Type:      "tool_result",
					ToolUseID: m.ToolCallID,
					Content:   m.Content,
					IsError:   m.ToolResultError,
				}
				// Merge with previous user message if possible
				if len(msgs) > 0 && msgs[len(msgs)-1].Role == "user" {
					if blocks, ok := msgs[len(msgs)-1].Content.([]anthropicContentBlock); ok {
						msgs[len(msgs)-1].Content = append(blocks, block)
					} else {
						msgs = append(msgs, anthropicMessage{Role: "user", Content: []anthropicContentBlock{block}})
					}
				} else {
					msgs = append(msgs, anthropicMessage{Role: "user", Content: []anthropicContentBlock{block}})
				}
				continue
			}
			if len(m.ToolCalls) > 0 {
				// Assistant message with tool_use blocks
				var blocks []anthropicContentBlock
				if m.Content != "" {
					blocks = append(blocks, anthropicContentBlock{Type: "text", Text: m.Content})
				}
				for _, tc := range m.ToolCalls {
					blocks = append(blocks, anthropicContentBlock{
						Type:  "tool_use",
						ID:    tc.ID,
						Name:  tc.Name,
						Input: tc.Arguments,
					})
				}
				msgs = append(msgs, anthropicMessage{Role: m.Role, Content: blocks})
			} else {
				msgs = append(msgs, anthropicMessage{Role: m.Role, Content: m.Content})
			}
		}
		if len(msgs) == 0 {
			msgs = []anthropicMessage{{Role: "user", Content: req.UserPrompt}}
		}
	} else {
		msgs = []anthropicMessage{{Role: "user", Content: req.UserPrompt}}
	}

	aReq := &anthropicRequest{
		Model:     req.Model,
		MaxTokens: maxTokens,
		System:    systemPrompt,
		Messages:  msgs,
		Stream:    stream,
	}

	// Add tool definitions if present.
	if len(req.Tools) > 0 {
		for _, t := range req.Tools {
			aReq.Tools = append(aReq.Tools, anthropicToolDef{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.Parameters,
			})
		}
		switch req.ToolChoice {
		case "", "auto":
			aReq.ToolChoice = &anthropicToolChoiceRequest{Type: "auto"}
		case "none":
			aReq.ToolChoice = nil
			aReq.Tools = nil
		case "required":
			aReq.ToolChoice = &anthropicToolChoiceRequest{Type: "any"}
		default:
			aReq.ToolChoice = &anthropicToolChoiceRequest{Type: "tool", Name: req.ToolChoice}
		}
	}

	return aReq
}

func (p *AnthropicProvider) doHTTPRequest(ctx context.Context, jsonBody []byte, apiKey string, extraHeaders map[string]string, stream bool) (*http.Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("anthropic: failed to create request: %w", err)
	}

	key := apiKey
	if key == "" {
		key = p.apiKey
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", key)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	for k, v := range extraHeaders {
		httpReq.Header.Set(k, v)
	}

	client := p.client
	if stream {
		client = &http.Client{}
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: request failed: %w", err)
	}

	return resp, nil
}

func parseAnthropicError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	var errResp anthropicErrorResponse
	message := string(body)
	errType := ""
	if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
		message = errResp.Error.Message
		errType = errResp.Error.Type
	}
	return classifyHTTPError("anthropic", resp.StatusCode, message, errType, "", resp.Header)
}

func (p *AnthropicProvider) Complete(ctx context.Context, req *model.CompletionRequest) (*model.CompletionResponse, error) {
	body := p.buildBody(req, false)
	requestJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: failed to marshal request: %w", err)
	}

	resp, err := p.doHTTPRequest(ctx, requestJSON, req.APIKey, req.ExtraHeaders, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseAnthropicError(resp)
	}

	responseJSON, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: failed to read response: %w", err)
	}

	var aResp anthropicResponse
	if err := json.Unmarshal(responseJSON, &aResp); err != nil {
		return nil, fmt.Errorf("anthropic: failed to decode response: %w", err)
	}

	var content string
	var toolCalls []model.ToolCall
	for _, block := range aResp.Content {
		switch block.Type {
		case "text":
			content += block.Text
		case "tool_use":
			toolCalls = append(toolCalls, model.ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: block.Input,
			})
		}
	}

	totalTokens := aResp.Usage.InputTokens + aResp.Usage.OutputTokens

	// Map Anthropic stop reasons to unified finish reasons.
	finishReason := aResp.StopReason
	switch finishReason {
	case "end_turn":
		finishReason = "stop"
	case "tool_use":
		finishReason = "tool_calls"
	case "max_tokens":
		finishReason = "length"
	}

	return &model.CompletionResponse{
		Content:      content,
		Model:        aResp.Model,
		FinishReason: finishReason,
		ToolCalls:    toolCalls,
		RawRequest:   requestJSON,
		RawResponse:  responseJSON,
		Usage: model.LLMUsage{
			Provider:         p.Name(),
			Model:            aResp.Model,
			PromptTokens:     aResp.Usage.InputTokens,
			CompletionTokens: aResp.Usage.OutputTokens,
			TotalTokens:      totalTokens,
		},
	}, nil
}

func (p *AnthropicProvider) Stream(ctx context.Context, req *model.CompletionRequest) (<-chan model.StreamChunk, error) {
	body := p.buildBody(req, true)
	requestJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: failed to marshal request: %w", err)
	}

	resp, err := p.doHTTPRequest(ctx, requestJSON, req.APIKey, req.ExtraHeaders, true)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, parseAnthropicError(resp)
	}

	ch := make(chan model.StreamChunk)
	go func() {
		defer resp.Body.Close()
		defer close(ch)

		var totalInputTokens, totalOutputTokens int

		scanner := bufio.NewScanner(resp.Body)
		var currentEvent string
		for scanner.Scan() {
			line := scanner.Text()

			if strings.HasPrefix(line, "event: ") {
				currentEvent = strings.TrimPrefix(line, "event: ")
				continue
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")

			switch currentEvent {
			case "content_block_delta":
				var delta struct {
					Delta anthropicContentDelta `json:"delta"`
				}
				if json.Unmarshal([]byte(data), &delta) == nil {
					ch <- model.StreamChunk{Content: delta.Delta.Text}
				}

			case "message_delta":
				var delta struct {
					Delta anthropicMessageDelta `json:"delta"`
					Usage *anthropicUsage       `json:"usage,omitempty"`
				}
				if json.Unmarshal([]byte(data), &delta) == nil {
					if delta.Usage != nil {
						totalOutputTokens = delta.Usage.OutputTokens
					}
				}

			case "message_start":
				var msg struct {
					Message struct {
						Usage anthropicUsage `json:"usage"`
					} `json:"message"`
				}
				if json.Unmarshal([]byte(data), &msg) == nil {
					totalInputTokens = msg.Message.Usage.InputTokens
				}

			case "message_stop":
				ch <- model.StreamChunk{
					Done: true,
					Usage: &model.LLMUsage{
						Provider:         p.Name(),
						Model:            req.Model,
						PromptTokens:     totalInputTokens,
						CompletionTokens: totalOutputTokens,
						TotalTokens:      totalInputTokens + totalOutputTokens,
					},
				}
				return
			}

			currentEvent = ""
		}
	}()

	return ch, nil
}

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

const defaultOpenAIBaseURL = "https://api.openai.com/v1"

// OpenAIProvider implements model.LLMProvider for the OpenAI API.
type OpenAIProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

var _ model.LLMProvider = (*OpenAIProvider)(nil)

// NewOpenAIProvider creates a new OpenAI provider.
// If baseURL is empty, the default OpenAI API URL is used.
func NewOpenAIProvider(apiKey, baseURL string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	return &OpenAIProvider{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

// openAIRequest is the request body for the OpenAI chat completions API.
type openAIRequest struct {
	Model          string            `json:"model"`
	Messages       []openAIMessage   `json:"messages"`
	Temperature    *float64          `json:"temperature,omitempty"`
	MaxTokens      *int              `json:"max_tokens,omitempty"`
	Stream         bool              `json:"stream,omitempty"`
	ResponseFormat *openAIRespFormat `json:"response_format,omitempty"`
	Tools          []openAIToolDef   `json:"tools,omitempty"`
	ToolChoice     any               `json:"tool_choice,omitempty"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAIRespFormat struct {
	Type string `json:"type"`
}

// openAIResponse is the response body from the OpenAI chat completions API.
type openAIResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message      openAIMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

type openAIErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func (p *OpenAIProvider) buildMessages(req *model.CompletionRequest) []openAIMessage {
	// Use explicit messages chain if provided.
	if len(req.Messages) > 0 {
		msgs := make([]openAIMessage, 0, len(req.Messages))
		for _, m := range req.Messages {
			msg := openAIMessage{
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
		return msgs
	}
	// Fallback: build from system_prompt + user_prompt.
	var msgs []openAIMessage
	if req.SystemPrompt != "" {
		msgs = append(msgs, openAIMessage{Role: "system", Content: req.SystemPrompt})
	}
	msgs = append(msgs, openAIMessage{Role: "user", Content: req.UserPrompt})
	return msgs
}

func (p *OpenAIProvider) buildRequest(req *model.CompletionRequest, stream bool) *openAIRequest {
	oReq := &openAIRequest{
		Model:       req.Model,
		Messages:    p.buildMessages(req),
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      stream,
	}
	if req.ResponseFormat == model.ResponseFormatJSON {
		oReq.ResponseFormat = &openAIRespFormat{Type: "json_object"}
	}
	// Add tool definitions if present.
	if len(req.Tools) > 0 {
		for _, t := range req.Tools {
			oReq.Tools = append(oReq.Tools, openAIToolDef{
				Type: "function",
				Function: openAIFunctionDef{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.Parameters,
				},
			})
		}
		switch req.ToolChoice {
		case "", "auto":
			oReq.ToolChoice = "auto"
		case "none":
			oReq.ToolChoice = "none"
		case "required":
			oReq.ToolChoice = "required"
		default:
			oReq.ToolChoice = openAIToolChoice{
				Type:     "function",
				Function: &openAIToolChoiceFC{Name: req.ToolChoice},
			}
		}
	}
	return oReq
}

func (p *OpenAIProvider) doHTTPRequest(ctx context.Context, jsonBody []byte, apiKey string, extraHeaders map[string]string, baseURLOverride string, stream bool) (*http.Response, error) {
	baseURL := p.baseURL
	if baseURLOverride != "" {
		baseURL = strings.TrimRight(baseURLOverride, "/")
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("openai: failed to create request: %w", err)
	}

	key := apiKey
	if key == "" {
		key = p.apiKey
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+key)
	for k, v := range extraHeaders {
		httpReq.Header.Set(k, v)
	}

	client := p.client
	if stream {
		// Streaming requests should not have a fixed timeout; rely on context.
		client = &http.Client{}
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: request failed: %w", err)
	}

	return resp, nil
}

func parseOpenAIError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	var errResp openAIErrorResponse
	message := string(body)
	errType := ""
	errCode := ""
	if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
		message = errResp.Error.Message
		errType = errResp.Error.Type
		errCode = errResp.Error.Code
	}
	return classifyHTTPError("openai", resp.StatusCode, message, errType, errCode, resp.Header)
}

func (p *OpenAIProvider) Complete(ctx context.Context, req *model.CompletionRequest) (*model.CompletionResponse, error) {
	oReq := p.buildRequest(req, false)
	requestJSON, err := json.Marshal(oReq)
	if err != nil {
		return nil, fmt.Errorf("openai: failed to marshal request: %w", err)
	}

	resp, err := p.doHTTPRequest(ctx, requestJSON, req.APIKey, req.ExtraHeaders, req.BaseURL, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseOpenAIError(resp)
	}

	responseJSON, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai: failed to read response: %w", err)
	}

	var oResp openAIResponse
	if err := json.Unmarshal(responseJSON, &oResp); err != nil {
		return nil, fmt.Errorf("openai: failed to decode response: %w", err)
	}

	if len(oResp.Choices) == 0 {
		return nil, fmt.Errorf("openai: empty choices in response")
	}

	result := &model.CompletionResponse{
		Content:      oResp.Choices[0].Message.Content,
		Model:        oResp.Model,
		FinishReason: oResp.Choices[0].FinishReason,
		RawRequest:   requestJSON,
		RawResponse:  responseJSON,
		Usage: model.LLMUsage{
			Provider:         p.Name(),
			Model:            oResp.Model,
			PromptTokens:     oResp.Usage.PromptTokens,
			CompletionTokens: oResp.Usage.CompletionTokens,
			TotalTokens:      oResp.Usage.TotalTokens,
		},
	}

	// Extract tool calls from the response.
	for _, tc := range oResp.Choices[0].Message.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, model.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: json.RawMessage(tc.Function.Arguments),
		})
	}

	return result, nil
}

func (p *OpenAIProvider) Stream(ctx context.Context, req *model.CompletionRequest) (<-chan model.StreamChunk, error) {
	oReq := p.buildRequest(req, true)
	requestJSON, err := json.Marshal(oReq)
	if err != nil {
		return nil, fmt.Errorf("openai: failed to marshal request: %w", err)
	}

	resp, err := p.doHTTPRequest(ctx, requestJSON, req.APIKey, req.ExtraHeaders, req.BaseURL, true)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, parseOpenAIError(resp)
	}

	ch := make(chan model.StreamChunk)
	go func() {
		defer resp.Body.Close()
		defer close(ch)
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				ch <- model.StreamChunk{Done: true}
				return
			}

			var chunk openAIStreamChunk
			if json.Unmarshal([]byte(data), &chunk) != nil {
				continue
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			sc := model.StreamChunk{
				Content: chunk.Choices[0].Delta.Content,
			}

			if chunk.Choices[0].FinishReason != nil && *chunk.Choices[0].FinishReason != "" {
				sc.Done = true
				if chunk.Usage != nil {
					sc.Usage = &model.LLMUsage{
						Provider:         p.Name(),
						Model:            req.Model,
						PromptTokens:     chunk.Usage.PromptTokens,
						CompletionTokens: chunk.Usage.CompletionTokens,
						TotalTokens:      chunk.Usage.TotalTokens,
					}
				}
			}

			ch <- sc
		}
	}()

	return ch, nil
}

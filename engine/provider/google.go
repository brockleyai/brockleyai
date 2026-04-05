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

const defaultGoogleBaseURL = "https://generativelanguage.googleapis.com/v1beta"

// GoogleProvider implements model.LLMProvider for the Google Gemini API.
type GoogleProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

var _ model.LLMProvider = (*GoogleProvider)(nil)

// NewGoogleProvider creates a new Google/Gemini provider.
// If baseURL is empty, the default Gemini API URL is used.
func NewGoogleProvider(apiKey, baseURL string) *GoogleProvider {
	if baseURL == "" {
		baseURL = defaultGoogleBaseURL
	}
	return &GoogleProvider{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *GoogleProvider) Name() string {
	return "google"
}

// Gemini request types.
type geminiRequest struct {
	Contents          []geminiContent  `json:"contents"`
	SystemInstruction *geminiContent   `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenConfig `json:"generationConfig,omitempty"`
	Tools             []googleToolDef  `json:"tools,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string                  `json:"text,omitempty"`
	FunctionCall     *googleFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *googleFunctionResponse `json:"functionResponse,omitempty"`
}

type geminiGenConfig struct {
	Temperature      *float64 `json:"temperature,omitempty"`
	MaxOutputTokens  *int     `json:"maxOutputTokens,omitempty"`
	ResponseMimeType string   `json:"responseMimeType,omitempty"`
}

// Gemini response types.
type geminiResponse struct {
	Candidates []struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
	ModelVersion string `json:"modelVersion,omitempty"`
}

// googleFunctionCall is a function call in Google responses.
type googleFunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

// googleFunctionResponse is a function response in Google messages.
type googleFunctionResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type geminiErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func (p *GoogleProvider) buildBody(req *model.CompletionRequest) *geminiRequest {
	gReq := &geminiRequest{}

	// Build contents from messages chain if provided.
	if len(req.Messages) > 0 {
		for _, m := range req.Messages {
			if m.Role == "system" {
				gReq.SystemInstruction = &geminiContent{
					Parts: []geminiPart{{Text: m.Content}},
				}
				continue
			}
			if m.Role == "tool" {
				// Tool result → functionResponse part
				var respData map[string]any
				if err := json.Unmarshal([]byte(m.Content), &respData); err != nil {
					respData = map[string]any{"result": m.Content}
				}
				gReq.Contents = append(gReq.Contents, geminiContent{
					Role: "user",
					Parts: []geminiPart{{
						FunctionResponse: &googleFunctionResponse{
							Name:     m.ToolCallID, // Google uses tool name as ID
							Response: respData,
						},
					}},
				})
				continue
			}
			role := m.Role
			if role == "assistant" {
				role = "model"
			}
			if len(m.ToolCalls) > 0 {
				var parts []geminiPart
				if m.Content != "" {
					parts = append(parts, geminiPart{Text: m.Content})
				}
				for _, tc := range m.ToolCalls {
					parts = append(parts, geminiPart{
						FunctionCall: &googleFunctionCall{Name: tc.Name, Args: tc.Arguments},
					})
				}
				gReq.Contents = append(gReq.Contents, geminiContent{Role: role, Parts: parts})
			} else {
				gReq.Contents = append(gReq.Contents, geminiContent{
					Role:  role,
					Parts: []geminiPart{{Text: m.Content}},
				})
			}
		}
		if len(gReq.Contents) == 0 {
			gReq.Contents = []geminiContent{{Role: "user", Parts: []geminiPart{{Text: req.UserPrompt}}}}
		}
	} else {
		gReq.Contents = []geminiContent{{Role: "user", Parts: []geminiPart{{Text: req.UserPrompt}}}}
		if req.SystemPrompt != "" {
			gReq.SystemInstruction = &geminiContent{
				Parts: []geminiPart{{Text: req.SystemPrompt}},
			}
		}
	}

	genConfig := &geminiGenConfig{
		Temperature:     req.Temperature,
		MaxOutputTokens: req.MaxTokens,
	}
	if req.ResponseFormat == model.ResponseFormatJSON {
		genConfig.ResponseMimeType = "application/json"
	}
	if genConfig.Temperature != nil || genConfig.MaxOutputTokens != nil || genConfig.ResponseMimeType != "" {
		gReq.GenerationConfig = genConfig
	}

	// Add tool definitions if present.
	if len(req.Tools) > 0 {
		td := googleToolDef{}
		for _, t := range req.Tools {
			td.FunctionDeclarations = append(td.FunctionDeclarations, googleFunctionDecl{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			})
		}
		gReq.Tools = []googleToolDef{td}
	}

	return gReq
}

func (p *GoogleProvider) doHTTPRequest(ctx context.Context, reqModel string, body any, apiKey string, extraHeaders map[string]string, stream bool) (*http.Response, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("google: failed to marshal request: %w", err)
	}

	var endpoint string
	if stream {
		endpoint = fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse", p.baseURL, reqModel)
	} else {
		endpoint = fmt.Sprintf("%s/models/%s:generateContent", p.baseURL, reqModel)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("google: failed to create request: %w", err)
	}

	key := apiKey
	if key == "" {
		key = p.apiKey
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", key)
	for k, v := range extraHeaders {
		httpReq.Header.Set(k, v)
	}

	client := p.client
	if stream {
		client = &http.Client{}
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("google: request failed: %w", err)
	}

	return resp, nil
}

func parseGoogleError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	var errResp geminiErrorResponse
	if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
		return fmt.Errorf("google: API error (status %d): %s (code: %d, status: %s)",
			resp.StatusCode, errResp.Error.Message, errResp.Error.Code, errResp.Error.Status)
	}
	return fmt.Errorf("google: API error (status %d): %s", resp.StatusCode, string(body))
}

func (p *GoogleProvider) Complete(ctx context.Context, req *model.CompletionRequest) (*model.CompletionResponse, error) {
	body := p.buildBody(req)

	resp, err := p.doHTTPRequest(ctx, req.Model, body, req.APIKey, req.ExtraHeaders, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseGoogleError(resp)
	}

	var gResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&gResp); err != nil {
		return nil, fmt.Errorf("google: failed to decode response: %w", err)
	}

	if len(gResp.Candidates) == 0 {
		return nil, fmt.Errorf("google: empty candidates in response")
	}

	var content string
	var toolCalls []model.ToolCall
	for i, part := range gResp.Candidates[0].Content.Parts {
		if part.Text != "" {
			content += part.Text
		}
		if part.FunctionCall != nil {
			toolCalls = append(toolCalls, model.ToolCall{
				ID:        fmt.Sprintf("call_%d", i),
				Name:      part.FunctionCall.Name,
				Arguments: part.FunctionCall.Args,
			})
		}
	}

	// Map Google finish reasons to unified.
	finishReason := gResp.Candidates[0].FinishReason
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	} else {
		switch finishReason {
		case "STOP":
			finishReason = "stop"
		case "MAX_TOKENS":
			finishReason = "length"
		}
	}

	return &model.CompletionResponse{
		Content:      content,
		Model:        req.Model,
		FinishReason: finishReason,
		ToolCalls:    toolCalls,
		Usage: model.LLMUsage{
			Provider:         p.Name(),
			Model:            req.Model,
			PromptTokens:     gResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: gResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      gResp.UsageMetadata.TotalTokenCount,
		},
	}, nil
}

func (p *GoogleProvider) Stream(ctx context.Context, req *model.CompletionRequest) (<-chan model.StreamChunk, error) {
	body := p.buildBody(req)

	resp, err := p.doHTTPRequest(ctx, req.Model, body, req.APIKey, req.ExtraHeaders, true)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, parseGoogleError(resp)
	}

	ch := make(chan model.StreamChunk)
	go func() {
		defer resp.Body.Close()
		defer close(ch)

		var lastUsage *model.LLMUsage
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")

			var gResp geminiResponse
			if json.Unmarshal([]byte(data), &gResp) != nil {
				continue
			}

			var content string
			if len(gResp.Candidates) > 0 {
				for _, part := range gResp.Candidates[0].Content.Parts {
					content += part.Text
				}
			}

			if gResp.UsageMetadata.TotalTokenCount > 0 {
				lastUsage = &model.LLMUsage{
					Provider:         p.Name(),
					Model:            req.Model,
					PromptTokens:     gResp.UsageMetadata.PromptTokenCount,
					CompletionTokens: gResp.UsageMetadata.CandidatesTokenCount,
					TotalTokens:      gResp.UsageMetadata.TotalTokenCount,
				}
			}

			done := false
			if len(gResp.Candidates) > 0 && gResp.Candidates[0].FinishReason != "" && gResp.Candidates[0].FinishReason != "FINISH_REASON_UNSPECIFIED" {
				done = true
			}

			sc := model.StreamChunk{
				Content: content,
				Done:    done,
			}
			if done {
				sc.Usage = lastUsage
			}

			ch <- sc
		}
	}()

	return ch, nil
}

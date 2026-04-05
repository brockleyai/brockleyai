package executor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/brockleyai/brockleyai/engine/expression"
	"github.com/brockleyai/brockleyai/internal/model"
)

// LLMExecutor handles nodes of type "llm".
type LLMExecutor struct{}

var _ NodeExecutor = (*LLMExecutor)(nil)

func (e *LLMExecutor) Execute(ctx context.Context, node *model.Node, inputs map[string]any, nctx *NodeContext, deps *ExecutorDeps) (*NodeResult, error) {
	var cfg model.LLMNodeConfig
	if err := json.Unmarshal(node.Config, &cfg); err != nil {
		return nil, fmt.Errorf("llm executor: invalid config: %w", err)
	}

	// Build template context from inputs, state, and meta.
	exprCtx := &expression.Context{
		Input: inputs,
	}
	if nctx != nil {
		exprCtx.State = nctx.State
		exprCtx.Meta = nctx.Meta
	}

	var err error

	// Build messages: use messages chain if provided, otherwise fall back to system_prompt/user_prompt.
	var messages []model.Message
	var systemPrompt, userPrompt string

	if len(cfg.Messages) > 0 {
		// Render each message in the chain.
		for i, msg := range cfg.Messages {
			rendered, err := expression.RenderTemplate(msg.Content, exprCtx)
			if err != nil {
				return nil, fmt.Errorf("llm executor: rendering message[%d]: %w", i, err)
			}
			messages = append(messages, model.Message{Role: msg.Role, Content: rendered})
		}
		// Extract system/user for backward-compat fields on CompletionRequest.
		for _, m := range messages {
			if m.Role == "system" && systemPrompt == "" {
				systemPrompt = m.Content
			}
			if m.Role == "user" {
				userPrompt = m.Content
			}
		}
	} else {
		// Legacy: single system_prompt + user_prompt.
		userPrompt, err = expression.RenderTemplate(cfg.UserPrompt, exprCtx)
		if err != nil {
			return nil, fmt.Errorf("llm executor: rendering user_prompt: %w", err)
		}

		if cfg.SystemPrompt != "" {
			systemPrompt, err = expression.RenderTemplate(cfg.SystemPrompt, exprCtx)
			if err != nil {
				return nil, fmt.Errorf("llm executor: rendering system_prompt: %w", err)
			}
		}
	}

	// If response_format is JSON and output_schema is set, append schema instruction.
	if cfg.ResponseFormat == model.ResponseFormatJSON && len(cfg.OutputSchema) > 0 {
		schemaStr := string(cfg.OutputSchema)
		instruction := "\n\nYou MUST respond with valid JSON matching this schema:\n" + schemaStr
		systemPrompt += instruction
	}

	// Resolve API key: inline api_key takes priority over api_key_ref.
	apiKey := cfg.APIKey
	if apiKey == "" && cfg.APIKeyRef != "" && deps.SecretStore != nil {
		apiKey, err = deps.SecretStore.GetSecret(ctx, cfg.APIKeyRef)
		if err != nil {
			return nil, fmt.Errorf("llm executor: resolving api_key_ref %q: %w", cfg.APIKeyRef, err)
		}
	}

	// Look up provider.
	if deps.ProviderRegistry == nil {
		return nil, fmt.Errorf("llm executor: no provider registry configured")
	}
	provider, err := deps.ProviderRegistry.Get(string(cfg.Provider))
	if err != nil {
		return nil, fmt.Errorf("llm executor: looking up provider %q: %w", cfg.Provider, err)
	}

	// Build completion request.
	req := &model.CompletionRequest{
		APIKey:         apiKey,
		Model:          cfg.Model,
		BaseURL:        cfg.BaseURL,
		Messages:       messages,
		SystemPrompt:   systemPrompt,
		UserPrompt:     userPrompt,
		Temperature:    cfg.Temperature,
		MaxTokens:      cfg.MaxTokens,
		ResponseFormat: cfg.ResponseFormat,
		ExtraHeaders:   cfg.ExtraHeaders,
		Tools:          cfg.Tools,
		ToolChoice:     cfg.ToolChoice,
	}

	// Set output schema on the request if present.
	if len(cfg.OutputSchema) > 0 {
		var schema any
		if err := json.Unmarshal(cfg.OutputSchema, &schema); err == nil {
			req.OutputSchema = schema
		}
	}

	// If tool_loop is enabled, delegate to the tool loop executor.
	if cfg.ToolLoop {
		// For tool loop, ensure messages include the initial conversation.
		if len(req.Messages) == 0 {
			if systemPrompt != "" {
				req.Messages = append(req.Messages, model.Message{Role: "system", Content: systemPrompt})
			}
			req.Messages = append(req.Messages, model.Message{Role: "user", Content: userPrompt})
		}
		return executeToolLoop(ctx, &cfg, req, provider, deps, nctx)
	}

	// Call provider (non-tool-loop path).
	resp, err := provider.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("llm executor: provider call failed: %w", err)
	}

	// Build outputs based on response format.
	outputs := make(map[string]any)

	if cfg.ResponseFormat == model.ResponseFormatJSON {
		content := resp.Content

		// Validate against output schema if configured.
		if shouldValidateOutput(cfg.ValidateOutput, cfg.OutputSchema) {
			retries := maxValidationRetries(cfg.MaxValidationRetries)
			validated, valErr := validateAndRetryJSON(ctx, content, cfg.OutputSchema, retries, provider, req)
			if valErr != nil {
				return nil, valErr
			}
			content = validated
		}

		// Parse JSON response.
		var parsed any
		if err := json.Unmarshal([]byte(content), &parsed); err != nil {
			return nil, fmt.Errorf("llm executor: response is not valid JSON: %w", err)
		}
		outputs["response"] = parsed
	} else {
		// Text format.
		outputs["response_text"] = resp.Content
	}

	// Include tool calls and finish reason when tools are configured.
	if len(req.Tools) > 0 {
		outputs["finish_reason"] = resp.FinishReason
		if len(resp.ToolCalls) > 0 {
			outputs["tool_calls"] = resp.ToolCalls
		}
	}

	return &NodeResult{Outputs: outputs}, nil
}

package executor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/brockleyai/brockleyai/engine/mock"
	"github.com/brockleyai/brockleyai/internal/model"
)

func TestValidateJSONOutput(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer"}
		},
		"required": ["name", "age"]
	}`)

	tests := []struct {
		name    string
		output  string
		wantErr bool
	}{
		{
			name:    "valid output matches schema",
			output:  `{"name": "Alice", "age": 30}`,
			wantErr: false,
		},
		{
			name:    "valid output with extra fields",
			output:  `{"name": "Bob", "age": 25, "email": "bob@test.com"}`,
			wantErr: false,
		},
		{
			name:    "missing required field",
			output:  `{"name": "Charlie"}`,
			wantErr: true,
		},
		{
			name:    "wrong type for field",
			output:  `{"name": "Dave", "age": "not-a-number"}`,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			output:  `not json at all`,
			wantErr: true,
		},
		{
			name:    "empty object missing required fields",
			output:  `{}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateJSONOutput(tt.output, schema)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

func TestShouldValidateOutput(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name           string
		validateOutput *bool
		outputSchema   json.RawMessage
		want           bool
	}{
		{
			name:           "schema set and validate nil (default true)",
			validateOutput: nil,
			outputSchema:   json.RawMessage(`{"type": "object"}`),
			want:           true,
		},
		{
			name:           "schema set and validate true",
			validateOutput: boolPtr(true),
			outputSchema:   json.RawMessage(`{"type": "object"}`),
			want:           true,
		},
		{
			name:           "schema set and validate false",
			validateOutput: boolPtr(false),
			outputSchema:   json.RawMessage(`{"type": "object"}`),
			want:           false,
		},
		{
			name:           "no schema set",
			validateOutput: nil,
			outputSchema:   nil,
			want:           false,
		},
		{
			name:           "empty schema",
			validateOutput: nil,
			outputSchema:   json.RawMessage(``),
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldValidateOutput(tt.validateOutput, tt.outputSchema)
			if got != tt.want {
				t.Errorf("shouldValidateOutput() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMaxValidationRetries(t *testing.T) {
	intPtr := func(n int) *int { return &n }

	tests := []struct {
		name       string
		configured *int
		want       int
	}{
		{"nil defaults to 2", nil, 2},
		{"explicit 0", intPtr(0), 0},
		{"explicit 3", intPtr(3), 3},
		{"explicit 1", intPtr(1), 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maxValidationRetries(tt.configured)
			if got != tt.want {
				t.Errorf("maxValidationRetries() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestValidateAndRetryJSON_ValidOnFirstAttempt(t *testing.T) {
	schema := json.RawMessage(`{"type": "object", "properties": {"x": {"type": "integer"}}, "required": ["x"]}`)
	provider := &mock.MockLLMProvider{}
	req := &model.CompletionRequest{Model: "test"}

	result, err := validateAndRetryJSON(context.Background(), `{"x": 1}`, schema, 2, provider, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != `{"x": 1}` {
		t.Errorf("expected original content, got %q", result)
	}
	// Provider should not have been called since first attempt was valid.
	if len(provider.Calls) != 0 {
		t.Errorf("expected 0 provider calls, got %d", len(provider.Calls))
	}
}

func TestValidateAndRetryJSON_InvalidThenValid(t *testing.T) {
	schema := json.RawMessage(`{"type": "object", "properties": {"x": {"type": "integer"}}, "required": ["x"]}`)

	provider := &mock.MockLLMProvider{
		Responses: []string{`{"x": 42}`},
	}
	req := &model.CompletionRequest{
		Model:      "test",
		UserPrompt: "give me json",
		Messages: []model.Message{
			{Role: "user", Content: "give me json"},
		},
	}

	// First attempt is invalid (missing required field), retry should succeed.
	result, err := validateAndRetryJSON(context.Background(), `{"y": "wrong"}`, schema, 2, provider, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != `{"x": 42}` {
		t.Errorf("expected corrected content, got %q", result)
	}
	if len(provider.Calls) != 1 {
		t.Errorf("expected 1 retry call, got %d", len(provider.Calls))
	}

	// Verify the retry request contains the validation error context.
	retryReq := provider.Calls[0]
	lastMsg := retryReq.Messages[len(retryReq.Messages)-1]
	if lastMsg.Role != "user" {
		t.Errorf("expected last message role 'user', got %q", lastMsg.Role)
	}
}

func TestValidateAndRetryJSON_ExhaustedRetries(t *testing.T) {
	schema := json.RawMessage(`{"type": "object", "properties": {"x": {"type": "integer"}}, "required": ["x"]}`)

	// All retry responses are still invalid.
	provider := &mock.MockLLMProvider{
		Responses: []string{`{"y": "still wrong"}`, `{"z": "also wrong"}`},
	}
	req := &model.CompletionRequest{
		Model:    "test",
		Messages: []model.Message{{Role: "user", Content: "give me json"}},
	}

	_, err := validateAndRetryJSON(context.Background(), `{"bad": true}`, schema, 2, provider, req)
	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
	if len(provider.Calls) != 2 {
		t.Errorf("expected 2 retry calls, got %d", len(provider.Calls))
	}
}

func TestValidateAndRetryJSON_NoRetries(t *testing.T) {
	schema := json.RawMessage(`{"type": "object", "properties": {"x": {"type": "integer"}}, "required": ["x"]}`)
	provider := &mock.MockLLMProvider{}
	req := &model.CompletionRequest{Model: "test"}

	_, err := validateAndRetryJSON(context.Background(), `{"bad": true}`, schema, 0, provider, req)
	if err == nil {
		t.Fatal("expected error with 0 retries")
	}
	// Provider should not have been called.
	if len(provider.Calls) != 0 {
		t.Errorf("expected 0 provider calls, got %d", len(provider.Calls))
	}
}

// TestLLMExecutor_JSONValidation_EndToEnd tests the full LLM executor path
// with schema validation and re-prompting.
func TestLLMExecutor_JSONValidation_EndToEnd(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"result": map[string]any{"type": "integer"},
			"status": map[string]any{"type": "string"},
		},
		"required": []string{"result", "status"},
	}

	t.Run("valid response passes validation", func(t *testing.T) {
		provider := &mock.MockLLMProvider{
			Responses: []string{`{"result": 42, "status": "ok"}`},
		}
		deps := newTestDeps(provider, map[string]string{"my-key": "sk-test"})

		cfg := model.LLMNodeConfig{
			Provider:       "mock",
			Model:          "test-model",
			APIKeyRef:      "my-key",
			UserPrompt:     "Compute something",
			ResponseFormat: model.ResponseFormatJSON,
			OutputSchema:   mustJSON(schema),
			// ValidateOutput defaults to true when OutputSchema is set.
		}

		node := &model.Node{
			ID: "llm-val-1", Name: "test-validation-pass", Type: model.NodeTypeLLM,
			Config: mustJSON(cfg),
		}

		exec := &LLMExecutor{}
		result, err := exec.Execute(context.Background(), node, nil, nil, deps)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := result.Outputs["response"]; !ok {
			t.Fatal("expected output port 'response'")
		}
		// Only the initial call, no retries.
		if len(provider.Calls) != 1 {
			t.Errorf("expected 1 provider call, got %d", len(provider.Calls))
		}
	})

	t.Run("invalid then valid triggers re-prompt", func(t *testing.T) {
		provider := &mock.MockLLMProvider{
			Responses: []string{
				`{"result": "not_a_number"}`,        // fails: result should be integer
				`{"result": 42, "status": "fixed"}`, // succeeds
			},
		}
		deps := newTestDeps(provider, map[string]string{"my-key": "sk-test"})

		cfg := model.LLMNodeConfig{
			Provider:       "mock",
			Model:          "test-model",
			APIKeyRef:      "my-key",
			UserPrompt:     "Compute something",
			ResponseFormat: model.ResponseFormatJSON,
			OutputSchema:   mustJSON(schema),
		}

		node := &model.Node{
			ID: "llm-val-2", Name: "test-validation-retry", Type: model.NodeTypeLLM,
			Config: mustJSON(cfg),
		}

		exec := &LLMExecutor{}
		result, err := exec.Execute(context.Background(), node, nil, nil, deps)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		resp, ok := result.Outputs["response"]
		if !ok {
			t.Fatal("expected output port 'response'")
		}
		m := resp.(map[string]any)
		if m["status"] != "fixed" {
			t.Errorf("expected status 'fixed', got %v", m["status"])
		}
		// 1 initial call + 1 retry = 2 calls.
		if len(provider.Calls) != 2 {
			t.Errorf("expected 2 provider calls, got %d", len(provider.Calls))
		}
	})

	t.Run("validate_output false skips validation", func(t *testing.T) {
		validateFalse := false
		provider := &mock.MockLLMProvider{
			Responses: []string{`{"result": "not_a_number"}`},
		}
		deps := newTestDeps(provider, map[string]string{"my-key": "sk-test"})

		cfg := model.LLMNodeConfig{
			Provider:       "mock",
			Model:          "test-model",
			APIKeyRef:      "my-key",
			UserPrompt:     "Compute something",
			ResponseFormat: model.ResponseFormatJSON,
			OutputSchema:   mustJSON(schema),
			ValidateOutput: &validateFalse,
		}

		node := &model.Node{
			ID: "llm-val-3", Name: "test-validation-skip", Type: model.NodeTypeLLM,
			Config: mustJSON(cfg),
		}

		exec := &LLMExecutor{}
		result, err := exec.Execute(context.Background(), node, nil, nil, deps)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := result.Outputs["response"]; !ok {
			t.Fatal("expected output port 'response'")
		}
		// Only 1 call, no validation retries.
		if len(provider.Calls) != 1 {
			t.Errorf("expected 1 provider call, got %d", len(provider.Calls))
		}
	})

	t.Run("no schema means no validation", func(t *testing.T) {
		provider := &mock.MockLLMProvider{
			Responses: []string{`{"anything": "goes"}`},
		}
		deps := newTestDeps(provider, map[string]string{"my-key": "sk-test"})

		cfg := model.LLMNodeConfig{
			Provider:       "mock",
			Model:          "test-model",
			APIKeyRef:      "my-key",
			UserPrompt:     "Do something",
			ResponseFormat: model.ResponseFormatJSON,
			// No OutputSchema set.
		}

		node := &model.Node{
			ID: "llm-val-4", Name: "test-no-schema", Type: model.NodeTypeLLM,
			Config: mustJSON(cfg),
		}

		exec := &LLMExecutor{}
		result, err := exec.Execute(context.Background(), node, nil, nil, deps)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := result.Outputs["response"]; !ok {
			t.Fatal("expected output port 'response'")
		}
		if len(provider.Calls) != 1 {
			t.Errorf("expected 1 provider call, got %d", len(provider.Calls))
		}
	})

	t.Run("exhausted retries returns error", func(t *testing.T) {
		retries := 1
		provider := &mock.MockLLMProvider{
			Responses: []string{
				`{"result": "bad1"}`, // initial response (invalid)
				`{"result": "bad2"}`, // retry 1 (also invalid)
			},
		}
		deps := newTestDeps(provider, map[string]string{"my-key": "sk-test"})

		cfg := model.LLMNodeConfig{
			Provider:             "mock",
			Model:                "test-model",
			APIKeyRef:            "my-key",
			UserPrompt:           "Compute something",
			ResponseFormat:       model.ResponseFormatJSON,
			OutputSchema:         mustJSON(schema),
			MaxValidationRetries: &retries,
		}

		node := &model.Node{
			ID: "llm-val-5", Name: "test-exhausted-retries", Type: model.NodeTypeLLM,
			Config: mustJSON(cfg),
		}

		exec := &LLMExecutor{}
		_, err := exec.Execute(context.Background(), node, nil, nil, deps)
		if err == nil {
			t.Fatal("expected error after exhausted retries")
		}
	})
}

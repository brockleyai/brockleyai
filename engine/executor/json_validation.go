package executor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/brockleyai/brockleyai/internal/model"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const defaultMaxValidationRetries = 2

// validateJSONOutput validates a JSON string against a JSON Schema.
// Returns nil if valid, or an error describing why validation failed.
func validateJSONOutput(output string, schema json.RawMessage) error {
	// Parse the schema.
	var schemaObj any
	if err := json.Unmarshal(schema, &schemaObj); err != nil {
		return fmt.Errorf("json validation: invalid schema: %w", err)
	}

	// Compile the schema.
	c := jsonschema.NewCompiler()
	if err := c.AddResource("schema.json", schemaObj); err != nil {
		return fmt.Errorf("json validation: adding schema resource: %w", err)
	}
	sch, err := c.Compile("schema.json")
	if err != nil {
		return fmt.Errorf("json validation: compiling schema: %w", err)
	}

	// Parse the output.
	var outputObj any
	if err := json.Unmarshal([]byte(output), &outputObj); err != nil {
		return fmt.Errorf("json validation: output is not valid JSON: %w", err)
	}

	// Validate.
	if err := sch.Validate(outputObj); err != nil {
		return fmt.Errorf("json validation failed: %w", err)
	}

	return nil
}

// shouldValidateOutput returns true when schema validation should be performed.
// Validation is on by default when an output_schema is set; ValidateOutput=false opts out.
func shouldValidateOutput(validateOutput *bool, outputSchema json.RawMessage) bool {
	if len(outputSchema) == 0 {
		return false
	}
	if validateOutput != nil {
		return *validateOutput
	}
	// Default: validate when schema is present.
	return true
}

// maxValidationRetries returns the configured retry count or the default (2).
func maxValidationRetries(configured *int) int {
	if configured != nil && *configured >= 0 {
		return *configured
	}
	return defaultMaxValidationRetries
}

// validateAndRetryJSON validates the LLM JSON response against the output schema.
// If validation fails, it re-prompts the LLM with the error details and retries
// up to maxRetries times. Returns the final valid response or the last error.
func validateAndRetryJSON(
	ctx context.Context,
	content string,
	schema json.RawMessage,
	maxRetries int,
	provider model.LLMProvider,
	req *model.CompletionRequest,
) (string, error) {
	// Validate the initial response.
	if err := validateJSONOutput(content, schema); err == nil {
		return content, nil
	} else if maxRetries <= 0 {
		return "", fmt.Errorf("llm executor: schema validation failed (no retries): %w", err)
	} else {
		// Retry loop.
		lastErr := err
		for attempt := 0; attempt < maxRetries; attempt++ {
			// Build a retry request with the validation error context.
			retryReq := *req
			retryMessages := make([]model.Message, len(req.Messages))
			copy(retryMessages, req.Messages)

			retryMessages = append(retryMessages, model.Message{
				Role:    "assistant",
				Content: content,
			})
			retryMessages = append(retryMessages, model.Message{
				Role: "user",
				Content: fmt.Sprintf(
					"Your previous JSON response failed schema validation:\n%s\n\nPlease fix the response to match the required schema and respond with valid JSON only.",
					lastErr.Error(),
				),
			})
			retryReq.Messages = retryMessages

			resp, err := provider.Complete(ctx, &retryReq)
			if err != nil {
				return "", fmt.Errorf("llm executor: re-prompt call failed (attempt %d): %w", attempt+1, err)
			}

			content = resp.Content
			if err := validateJSONOutput(content, schema); err == nil {
				return content, nil
			} else {
				lastErr = err
			}
		}
		return "", fmt.Errorf("llm executor: schema validation failed after %d retries: %w", maxRetries, lastErr)
	}
}

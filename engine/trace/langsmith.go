package trace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/brockleyai/brockleyai/internal/model"
)

const langSmithBatchURL = "https://api.smith.langchain.com/runs/batch"

// LangSmithExporter sends trace spans to LangSmith using its custom REST API.
// LangSmith does not support OTLP, so this exporter maps TraceSpan to
// LangSmith's Run model and posts them in batches.
type LangSmithExporter struct {
	apiKey  string
	project string
	client  *http.Client
	runs    []langSmithRun
	mu      sync.Mutex
}

var _ model.TraceExporter = (*LangSmithExporter)(nil)

// langSmithRun is the LangSmith run payload format.
type langSmithRun struct {
	ID          string         `json:"id"`
	TraceID     string         `json:"trace_id"`
	ParentRunID string         `json:"parent_run_id,omitempty"`
	Name        string         `json:"name"`
	RunType     string         `json:"run_type"`
	Inputs      map[string]any `json:"inputs"`
	Outputs     map[string]any `json:"outputs"`
	StartTime   string         `json:"start_time"`
	EndTime     string         `json:"end_time"`
	Extra       map[string]any `json:"extra"`
}

type langSmithBatchPayload struct {
	Post []langSmithRun `json:"post"`
}

// NewLangSmithExporter creates a LangSmith exporter.
// apiKey is the LangSmith API key (x-api-key header).
// project is the LangSmith project name for organizing traces.
func NewLangSmithExporter(apiKey, project string) *LangSmithExporter {
	return &LangSmithExporter{
		apiKey:  apiKey,
		project: project,
		client:  &http.Client{},
	}
}

// ExportSpan converts a TraceSpan to a LangSmith run and buffers it.
func (e *LangSmithExporter) ExportSpan(ctx context.Context, span model.TraceSpan) {
	run := langSmithRun{
		ID:          span.SpanID,
		TraceID:     span.TraceID,
		ParentRunID: span.ParentSpanID,
		Name:        span.Name,
		RunType:     spanKindToRunType(span.Kind),
		Inputs:      extractInputs(span.Attributes),
		Outputs:     extractOutputs(span.Attributes),
		StartTime:   span.StartTime.UTC().Format("2006-01-02T15:04:05.000000Z"),
		EndTime:     span.EndTime.UTC().Format("2006-01-02T15:04:05.000000Z"),
		Extra:       buildExtra(span.Attributes, e.project),
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.runs = append(e.runs, run)
}

// Flush sends all buffered runs to LangSmith.
func (e *LangSmithExporter) Flush(ctx context.Context) error {
	e.mu.Lock()
	if len(e.runs) == 0 {
		e.mu.Unlock()
		return nil
	}
	runs := e.runs
	e.runs = nil
	e.mu.Unlock()

	payload := langSmithBatchPayload{Post: runs}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("langsmith: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, langSmithBatchURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("langsmith: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("langsmith: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("langsmith: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// Shutdown flushes remaining runs.
func (e *LangSmithExporter) Shutdown(ctx context.Context) error {
	return e.Flush(ctx)
}

// spanKindToRunType maps TraceSpan.Kind to LangSmith run_type.
func spanKindToRunType(kind string) string {
	switch kind {
	case "LLM":
		return "llm"
	case "TOOL":
		return "tool"
	case "CHAIN":
		return "chain"
	default:
		return "chain"
	}
}

// extractInputs pulls input-related attributes for the LangSmith inputs field.
func extractInputs(attrs map[string]any) map[string]any {
	inputs := make(map[string]any)
	if v, ok := attrs["input.value"]; ok {
		inputs["input"] = v
	}
	if v, ok := attrs["llm.input_messages"]; ok {
		inputs["messages"] = v
	}
	if v, ok := attrs["tool.parameters"]; ok {
		inputs["parameters"] = v
	}
	if len(inputs) == 0 {
		inputs["input"] = ""
	}
	return inputs
}

// extractOutputs pulls output-related attributes for the LangSmith outputs field.
func extractOutputs(attrs map[string]any) map[string]any {
	outputs := make(map[string]any)
	if v, ok := attrs["output.value"]; ok {
		outputs["output"] = v
	}
	if v, ok := attrs["llm.output_messages"]; ok {
		outputs["messages"] = v
	}
	if v, ok := attrs["tool.result"]; ok {
		outputs["result"] = v
	}
	if len(outputs) == 0 {
		outputs["output"] = ""
	}
	return outputs
}

// buildExtra creates the LangSmith extra field containing metadata.
func buildExtra(attrs map[string]any, project string) map[string]any {
	metadata := make(map[string]any)
	for k, v := range attrs {
		metadata[k] = v
	}
	metadata["project"] = project
	return map[string]any{
		"metadata": metadata,
	}
}

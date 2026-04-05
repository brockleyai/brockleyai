package trace

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/brockleyai/brockleyai/internal/model"
)

func sampleSpan(name string) model.TraceSpan {
	return model.TraceSpan{
		TraceID:      "trace-001",
		SpanID:       "span-001",
		ParentSpanID: "span-000",
		Name:         name,
		Kind:         "LLM",
		StartTime:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndTime:      time.Date(2025, 1, 1, 0, 0, 1, 0, time.UTC),
		Status:       "OK",
		Attributes: map[string]any{
			"llm.model_name":         "gpt-4",
			"llm.token_count.prompt": 100,
		},
	}
}

// --- ExporterRegistry tests ---

func TestExporterRegistry_FanOut(t *testing.T) {
	reg := NewExporterRegistry()

	exp1 := &recordingExporter{}
	exp2 := &recordingExporter{}
	reg.Register(exp1)
	reg.Register(exp2)

	ctx := context.Background()
	span := sampleSpan("test-span")
	reg.ExportSpan(ctx, span)

	if len(exp1.spans) != 1 {
		t.Fatalf("expected exp1 to receive 1 span, got %d", len(exp1.spans))
	}
	if len(exp2.spans) != 1 {
		t.Fatalf("expected exp2 to receive 1 span, got %d", len(exp2.spans))
	}
	if exp1.spans[0].Name != "test-span" {
		t.Errorf("expected span name 'test-span', got %q", exp1.spans[0].Name)
	}
	if exp2.spans[0].Name != "test-span" {
		t.Errorf("expected span name 'test-span', got %q", exp2.spans[0].Name)
	}
}

func TestExporterRegistry_FlushAll(t *testing.T) {
	reg := NewExporterRegistry()
	exp1 := &recordingExporter{}
	exp2 := &recordingExporter{}
	reg.Register(exp1)
	reg.Register(exp2)

	if err := reg.Flush(context.Background()); err != nil {
		t.Fatalf("flush error: %v", err)
	}
	if exp1.flushCount != 1 || exp2.flushCount != 1 {
		t.Errorf("expected both exporters flushed once, got %d and %d", exp1.flushCount, exp2.flushCount)
	}
}

func TestExporterRegistry_ShutdownAll(t *testing.T) {
	reg := NewExporterRegistry()
	exp1 := &recordingExporter{}
	reg.Register(exp1)

	if err := reg.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}
	if exp1.shutdownCount != 1 {
		t.Errorf("expected shutdown called once, got %d", exp1.shutdownCount)
	}
}

func TestExporterRegistry_EmptyIsValid(t *testing.T) {
	reg := NewExporterRegistry()
	ctx := context.Background()

	// Should not panic with no exporters.
	reg.ExportSpan(ctx, sampleSpan("no-op"))
	if err := reg.Flush(ctx); err != nil {
		t.Fatalf("flush on empty registry should not error: %v", err)
	}
	if err := reg.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown on empty registry should not error: %v", err)
	}
}

// --- OTLPExporter tests ---

func TestOTLPExporter_BufferAndFlush(t *testing.T) {
	var receivedBody []byte
	var receivedContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		receivedBody = body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	exp := NewOTLPExporter(server.URL, map[string]string{"X-Custom": "test-val"})
	ctx := context.Background()

	span := sampleSpan("otlp-test")
	exp.ExportSpan(ctx, span)

	if err := exp.Flush(ctx); err != nil {
		t.Fatalf("flush error: %v", err)
	}

	if receivedContentType != "application/json" {
		t.Errorf("expected application/json, got %q", receivedContentType)
	}

	var payload otlpPayload
	if err := json.Unmarshal(receivedBody, &payload); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(payload.ResourceSpans) != 1 {
		t.Fatalf("expected 1 resourceSpan, got %d", len(payload.ResourceSpans))
	}
	rs := payload.ResourceSpans[0]

	// Check service.name resource attribute.
	if len(rs.Resource.Attributes) < 1 {
		t.Fatal("expected resource attributes")
	}
	if rs.Resource.Attributes[0].Key != "service.name" {
		t.Errorf("expected service.name attribute, got %q", rs.Resource.Attributes[0].Key)
	}
	if *rs.Resource.Attributes[0].Value.StringValue != "brockley" {
		t.Errorf("expected 'brockley', got %q", *rs.Resource.Attributes[0].Value.StringValue)
	}

	if len(rs.ScopeSpans) != 1 || len(rs.ScopeSpans[0].Spans) != 1 {
		t.Fatal("expected 1 scope span with 1 span")
	}
	s := rs.ScopeSpans[0].Spans[0]
	if s.TraceID != "trace-001" {
		t.Errorf("expected traceId 'trace-001', got %q", s.TraceID)
	}
	if s.Name != "otlp-test" {
		t.Errorf("expected name 'otlp-test', got %q", s.Name)
	}
}

func TestOTLPExporter_FlushEmpty(t *testing.T) {
	exp := NewOTLPExporter("http://unused", nil)
	// Flushing with no spans should not make any HTTP calls.
	if err := exp.Flush(context.Background()); err != nil {
		t.Fatalf("flush empty should not error: %v", err)
	}
}

func TestOTLPExporter_CustomHeaders(t *testing.T) {
	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	exp := NewOTLPExporter(server.URL, map[string]string{"Authorization": "Bearer my-token"})
	exp.ExportSpan(context.Background(), sampleSpan("header-test"))
	if err := exp.Flush(context.Background()); err != nil {
		t.Fatalf("flush error: %v", err)
	}

	if authHeader != "Bearer my-token" {
		t.Errorf("expected 'Bearer my-token', got %q", authHeader)
	}
}

// --- LangfuseExporter tests ---

func TestLangfuseExporter_EndpointAndAuth(t *testing.T) {
	var requestURL string
	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURL = r.URL.Path
		authHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	exp := NewLangfuseExporter(server.URL, "pk-test", "sk-test")
	// Override the internal OTLP endpoint to point at our test server.
	exp.otlp.endpoint = server.URL + "/api/public/otel/v1/traces"

	exp.ExportSpan(context.Background(), sampleSpan("langfuse-test"))
	if err := exp.Flush(context.Background()); err != nil {
		t.Fatalf("flush error: %v", err)
	}

	if requestURL != "/api/public/otel/v1/traces" {
		t.Errorf("expected /api/public/otel/v1/traces, got %q", requestURL)
	}

	expectedCreds := base64.StdEncoding.EncodeToString([]byte("pk-test:sk-test"))
	expectedAuth := "Basic " + expectedCreds
	if authHeader != expectedAuth {
		t.Errorf("expected auth %q, got %q", expectedAuth, authHeader)
	}
}

func TestLangfuseExporter_EndpointConstruction(t *testing.T) {
	exp := NewLangfuseExporter("https://cloud.langfuse.com", "pk", "sk")
	expected := "https://cloud.langfuse.com/api/public/otel/v1/traces"
	if exp.otlp.endpoint != expected {
		t.Errorf("expected endpoint %q, got %q", expected, exp.otlp.endpoint)
	}

	// Trailing slash should be handled.
	exp2 := NewLangfuseExporter("https://cloud.langfuse.com/", "pk", "sk")
	if exp2.otlp.endpoint != expected {
		t.Errorf("expected endpoint %q, got %q", expected, exp2.otlp.endpoint)
	}
}

// --- OpikExporter tests ---

func TestOpikExporter_EndpointAndHeaders(t *testing.T) {
	exp := NewOpikExporter("https://www.comet.com/opik", "sk-opik-123", "my-workspace")
	expected := "https://www.comet.com/opik/api/v1/private/otel/v1/traces"
	if exp.otlp.endpoint != expected {
		t.Errorf("expected endpoint %q, got %q", expected, exp.otlp.endpoint)
	}
	if exp.otlp.headers["Authorization"] != "sk-opik-123" {
		t.Errorf("expected Authorization 'sk-opik-123', got %q", exp.otlp.headers["Authorization"])
	}
	if exp.otlp.headers["Comet-Workspace"] != "my-workspace" {
		t.Errorf("expected Comet-Workspace 'my-workspace', got %q", exp.otlp.headers["Comet-Workspace"])
	}
}

// --- PhoenixExporter tests ---

func TestPhoenixExporter_EndpointAndOptionalAuth(t *testing.T) {
	// With API key.
	exp := NewPhoenixExporter("http://localhost:6006", "my-key")
	if exp.otlp.endpoint != "http://localhost:6006/v1/traces" {
		t.Errorf("unexpected endpoint: %q", exp.otlp.endpoint)
	}
	if exp.otlp.headers["Authorization"] != "Bearer my-key" {
		t.Errorf("expected Bearer auth, got %q", exp.otlp.headers["Authorization"])
	}

	// Without API key.
	exp2 := NewPhoenixExporter("http://localhost:6006", "")
	if _, ok := exp2.otlp.headers["Authorization"]; ok {
		t.Error("expected no Authorization header when apiKey is empty")
	}
}

// --- LangSmithExporter tests ---

func TestLangSmithExporter_RunFormat(t *testing.T) {
	var receivedBody []byte
	var apiKeyHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKeyHeader = r.Header.Get("x-api-key")
		body, _ := io.ReadAll(r.Body)
		receivedBody = body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	exp := NewLangSmithExporter("ls-test-key", "my-project")
	// Override the batch URL to point at our test server.
	origClient := exp.client
	exp.client = origClient
	// We need to intercept the URL. Create a transport wrapper.
	exp.client = server.Client()
	// Manually set the batch URL via a custom approach: we'll replace the
	// client with a redirect-aware one. Instead, let's just test through
	// the httptest server by temporarily changing the const.
	// Since langSmithBatchURL is a const, we use a workaround: create a
	// custom HTTP client that redirects to the test server.
	exp.client = &http.Client{
		Transport: &rewriteTransport{
			targetURL: server.URL + "/runs/batch",
			wrapped:   http.DefaultTransport,
		},
	}

	span := model.TraceSpan{
		TraceID:      "trace-100",
		SpanID:       "span-200",
		ParentSpanID: "span-100",
		Name:         "llm.anthropic.complete",
		Kind:         "LLM",
		StartTime:    time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC),
		EndTime:      time.Date(2025, 6, 1, 12, 0, 2, 0, time.UTC),
		Status:       "OK",
		Attributes: map[string]any{
			"llm.model_name":         "claude-3",
			"llm.input_messages":     []any{map[string]any{"role": "user", "content": "hello"}},
			"llm.output_messages":    []any{map[string]any{"role": "assistant", "content": "hi"}},
			"llm.token_count.prompt": 10,
		},
	}

	exp.ExportSpan(context.Background(), span)
	if err := exp.Flush(context.Background()); err != nil {
		t.Fatalf("flush error: %v", err)
	}

	if apiKeyHeader != "ls-test-key" {
		t.Errorf("expected x-api-key 'ls-test-key', got %q", apiKeyHeader)
	}

	var batch langSmithBatchPayload
	if err := json.Unmarshal(receivedBody, &batch); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(batch.Post) != 1 {
		t.Fatalf("expected 1 run, got %d", len(batch.Post))
	}
	run := batch.Post[0]

	if run.ID != "span-200" {
		t.Errorf("expected id 'span-200', got %q", run.ID)
	}
	if run.TraceID != "trace-100" {
		t.Errorf("expected trace_id 'trace-100', got %q", run.TraceID)
	}
	if run.ParentRunID != "span-100" {
		t.Errorf("expected parent_run_id 'span-100', got %q", run.ParentRunID)
	}
	if run.RunType != "llm" {
		t.Errorf("expected run_type 'llm', got %q", run.RunType)
	}
	if run.Name != "llm.anthropic.complete" {
		t.Errorf("expected name 'llm.anthropic.complete', got %q", run.Name)
	}
	if _, ok := run.Inputs["messages"]; !ok {
		t.Error("expected inputs to have 'messages' key")
	}
	if _, ok := run.Outputs["messages"]; !ok {
		t.Error("expected outputs to have 'messages' key")
	}
	metadata, ok := run.Extra["metadata"].(map[string]any)
	if !ok {
		t.Fatal("expected extra.metadata to be a map")
	}
	if metadata["project"] != "my-project" {
		t.Errorf("expected project 'my-project' in metadata, got %v", metadata["project"])
	}
}

func TestLangSmithExporter_SpanKindMapping(t *testing.T) {
	tests := []struct {
		kind     string
		expected string
	}{
		{"LLM", "llm"},
		{"TOOL", "tool"},
		{"CHAIN", "chain"},
		{"UNKNOWN", "chain"},
	}
	for _, tc := range tests {
		got := spanKindToRunType(tc.kind)
		if got != tc.expected {
			t.Errorf("spanKindToRunType(%q) = %q, want %q", tc.kind, got, tc.expected)
		}
	}
}

// --- SetupFromEnv tests ---

func TestSetupFromEnv_NoEnvVars(t *testing.T) {
	// Ensure no BROCKLEY_TRACE_* vars are set.
	registry := SetupFromEnv()
	if registry == nil {
		t.Fatal("expected non-nil registry")
		return
	}
	if len(registry.exporters) != 0 {
		t.Errorf("expected 0 exporters with no env vars, got %d", len(registry.exporters))
	}
}

func TestSetupFromEnv_LangfuseEnabled(t *testing.T) {
	t.Setenv("BROCKLEY_TRACE_LANGFUSE_ENABLED", "true")
	t.Setenv("BROCKLEY_TRACE_LANGFUSE_HOST", "https://cloud.langfuse.com")
	t.Setenv("BROCKLEY_TRACE_LANGFUSE_PUBLIC_KEY", "pk-test")
	t.Setenv("BROCKLEY_TRACE_LANGFUSE_SECRET_KEY", "sk-test")

	registry := SetupFromEnv()
	if len(registry.exporters) != 1 {
		t.Fatalf("expected 1 exporter, got %d", len(registry.exporters))
	}
	if _, ok := registry.exporters[0].(*LangfuseExporter); !ok {
		t.Errorf("expected LangfuseExporter, got %T", registry.exporters[0])
	}
}

func TestSetupFromEnv_MultipleEnabled(t *testing.T) {
	t.Setenv("BROCKLEY_TRACE_LANGFUSE_ENABLED", "true")
	t.Setenv("BROCKLEY_TRACE_LANGFUSE_HOST", "https://cloud.langfuse.com")
	t.Setenv("BROCKLEY_TRACE_LANGFUSE_PUBLIC_KEY", "pk")
	t.Setenv("BROCKLEY_TRACE_LANGFUSE_SECRET_KEY", "sk")

	t.Setenv("BROCKLEY_TRACE_LANGSMITH_ENABLED", "true")
	t.Setenv("BROCKLEY_TRACE_LANGSMITH_API_KEY", "ls-key")

	registry := SetupFromEnv()
	if len(registry.exporters) != 2 {
		t.Fatalf("expected 2 exporters, got %d", len(registry.exporters))
	}
}

func TestParseHeaders(t *testing.T) {
	headers := parseHeaders("Authorization=Bearer token123,X-Custom=value")
	if headers["Authorization"] != "Bearer token123" {
		t.Errorf("expected 'Bearer token123', got %q", headers["Authorization"])
	}
	if headers["X-Custom"] != "value" {
		t.Errorf("expected 'value', got %q", headers["X-Custom"])
	}

	empty := parseHeaders("")
	if len(empty) != 0 {
		t.Errorf("expected empty map, got %d entries", len(empty))
	}
}

// --- helpers ---

// recordingExporter is a test helper that records calls.
type recordingExporter struct {
	spans         []model.TraceSpan
	flushCount    int
	shutdownCount int
}

func (e *recordingExporter) ExportSpan(ctx context.Context, span model.TraceSpan) {
	e.spans = append(e.spans, span)
}

func (e *recordingExporter) Flush(ctx context.Context) error {
	e.flushCount++
	return nil
}

func (e *recordingExporter) Shutdown(ctx context.Context) error {
	e.shutdownCount++
	return nil
}

// rewriteTransport rewrites all request URLs to a target URL for testing.
type rewriteTransport struct {
	targetURL string
	wrapped   http.RoundTripper
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newReq := req.Clone(req.Context())
	target, _ := http.NewRequest(req.Method, t.targetURL, req.Body)
	newReq.URL = target.URL
	return t.wrapped.RoundTrip(newReq)
}

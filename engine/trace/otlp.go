package trace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/brockleyai/brockleyai/internal/model"
)

// OTLPExporter sends trace spans to any OTLP-compatible HTTP endpoint
// using the simplified OTLP JSON format.
type OTLPExporter struct {
	endpoint string
	headers  map[string]string
	client   *http.Client
	spans    []model.TraceSpan
	mu       sync.Mutex
}

var _ model.TraceExporter = (*OTLPExporter)(nil)

// NewOTLPExporter creates an exporter that sends spans to the given OTLP HTTP endpoint.
// Custom headers (e.g. for auth) are sent with every request.
func NewOTLPExporter(endpoint string, headers map[string]string) *OTLPExporter {
	return &OTLPExporter{
		endpoint: endpoint,
		headers:  headers,
		client:   &http.Client{},
	}
}

// ExportSpan buffers a span for later flushing.
func (e *OTLPExporter) ExportSpan(ctx context.Context, span model.TraceSpan) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.spans = append(e.spans, span)
}

// Flush sends all buffered spans to the OTLP endpoint.
func (e *OTLPExporter) Flush(ctx context.Context) error {
	e.mu.Lock()
	if len(e.spans) == 0 {
		e.mu.Unlock()
		return nil
	}
	spans := e.spans
	e.spans = nil
	e.mu.Unlock()

	payload := buildOTLPPayload(spans)

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("otlp: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("otlp: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range e.headers {
		req.Header.Set(k, v)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("otlp: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("otlp: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// Shutdown flushes remaining spans and marks the exporter as done.
func (e *OTLPExporter) Shutdown(ctx context.Context) error {
	return e.Flush(ctx)
}

// OTLP JSON payload types.

type otlpPayload struct {
	ResourceSpans []otlpResourceSpan `json:"resourceSpans"`
}

type otlpResourceSpan struct {
	Resource   otlpResource    `json:"resource"`
	ScopeSpans []otlpScopeSpan `json:"scopeSpans"`
}

type otlpResource struct {
	Attributes []otlpAttribute `json:"attributes"`
}

type otlpScopeSpan struct {
	Spans []otlpSpan `json:"spans"`
}

type otlpSpan struct {
	TraceID           string          `json:"traceId"`
	SpanID            string          `json:"spanId"`
	ParentSpanID      string          `json:"parentSpanId,omitempty"`
	Name              string          `json:"name"`
	Kind              int             `json:"kind"`
	StartTimeUnixNano string          `json:"startTimeUnixNano"`
	EndTimeUnixNano   string          `json:"endTimeUnixNano"`
	Attributes        []otlpAttribute `json:"attributes"`
}

type otlpAttribute struct {
	Key   string        `json:"key"`
	Value otlpAttrValue `json:"value"`
}

type otlpAttrValue struct {
	StringValue *string `json:"stringValue,omitempty"`
	IntValue    *string `json:"intValue,omitempty"`
	DoubleValue *string `json:"doubleValue,omitempty"`
	BoolValue   *bool   `json:"boolValue,omitempty"`
}

func buildOTLPPayload(spans []model.TraceSpan) otlpPayload {
	otlpSpans := make([]otlpSpan, 0, len(spans))
	for _, s := range spans {
		otlpSpans = append(otlpSpans, otlpSpan{
			TraceID:           s.TraceID,
			SpanID:            s.SpanID,
			ParentSpanID:      s.ParentSpanID,
			Name:              s.Name,
			Kind:              spanKindToInt(s.Kind),
			StartTimeUnixNano: strconv.FormatInt(s.StartTime.UnixNano(), 10),
			EndTimeUnixNano:   strconv.FormatInt(s.EndTime.UnixNano(), 10),
			Attributes:        convertAttributes(s.Attributes),
		})
	}

	return otlpPayload{
		ResourceSpans: []otlpResourceSpan{
			{
				Resource: otlpResource{
					Attributes: []otlpAttribute{
						{
							Key:   "service.name",
							Value: otlpAttrValue{StringValue: strPtr("brockley")},
						},
					},
				},
				ScopeSpans: []otlpScopeSpan{
					{Spans: otlpSpans},
				},
			},
		},
	}
}

func spanKindToInt(kind string) int {
	switch kind {
	case "LLM", "CHAIN":
		return 1 // SPAN_KIND_INTERNAL
	case "TOOL":
		return 3 // SPAN_KIND_CLIENT
	default:
		return 1
	}
}

func convertAttributes(attrs map[string]any) []otlpAttribute {
	if len(attrs) == 0 {
		return nil
	}
	result := make([]otlpAttribute, 0, len(attrs))
	for k, v := range attrs {
		result = append(result, otlpAttribute{
			Key:   k,
			Value: toOTLPValue(v),
		})
	}
	return result
}

func toOTLPValue(v any) otlpAttrValue {
	switch val := v.(type) {
	case string:
		return otlpAttrValue{StringValue: strPtr(val)}
	case int:
		return otlpAttrValue{IntValue: strPtr(strconv.Itoa(val))}
	case int64:
		return otlpAttrValue{IntValue: strPtr(strconv.FormatInt(val, 10))}
	case float64:
		return otlpAttrValue{DoubleValue: strPtr(strconv.FormatFloat(val, 'f', -1, 64))}
	case bool:
		return otlpAttrValue{BoolValue: &val}
	default:
		// For complex types, marshal to JSON string.
		b, _ := json.Marshal(val)
		s := string(b)
		return otlpAttrValue{StringValue: &s}
	}
}

func strPtr(s string) *string {
	return &s
}

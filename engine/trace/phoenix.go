package trace

import (
	"context"
	"strings"

	"github.com/brockleyai/brockleyai/internal/model"
)

// PhoenixExporter sends trace spans to Arize Phoenix via its OTLP-compatible endpoint.
// It is a thin wrapper around OTLPExporter with Phoenix-specific endpoint and optional auth.
type PhoenixExporter struct {
	otlp *OTLPExporter
}

var _ model.TraceExporter = (*PhoenixExporter)(nil)

// NewPhoenixExporter creates a Phoenix exporter.
// host is the Phoenix base URL (e.g. "http://localhost:6006").
// apiKey is optional; if empty, no Authorization header is sent (common for self-hosted).
func NewPhoenixExporter(host, apiKey string) *PhoenixExporter {
	endpoint := strings.TrimRight(host, "/") + "/v1/traces"
	headers := map[string]string{}
	if apiKey != "" {
		headers["Authorization"] = "Bearer " + apiKey
	}
	return &PhoenixExporter{
		otlp: NewOTLPExporter(endpoint, headers),
	}
}

func (e *PhoenixExporter) ExportSpan(ctx context.Context, span model.TraceSpan) {
	e.otlp.ExportSpan(ctx, span)
}

func (e *PhoenixExporter) Flush(ctx context.Context) error {
	return e.otlp.Flush(ctx)
}

func (e *PhoenixExporter) Shutdown(ctx context.Context) error {
	return e.otlp.Shutdown(ctx)
}

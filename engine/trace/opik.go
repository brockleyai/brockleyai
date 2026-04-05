package trace

import (
	"context"
	"strings"

	"github.com/brockleyai/brockleyai/internal/model"
)

// OpikExporter sends trace spans to Opik (Comet) via its OTLP-compatible endpoint.
// It is a thin wrapper around OTLPExporter with Opik-specific endpoint and auth headers.
type OpikExporter struct {
	otlp *OTLPExporter
}

var _ model.TraceExporter = (*OpikExporter)(nil)

// NewOpikExporter creates an Opik exporter.
// host is the Opik base URL (e.g. "https://www.comet.com/opik").
// apiKey is the Opik API key, workspace is the Comet workspace name.
func NewOpikExporter(host, apiKey, workspace string) *OpikExporter {
	endpoint := strings.TrimRight(host, "/") + "/api/v1/private/otel/v1/traces"
	headers := map[string]string{
		"Authorization":   apiKey,
		"Comet-Workspace": workspace,
	}
	return &OpikExporter{
		otlp: NewOTLPExporter(endpoint, headers),
	}
}

func (e *OpikExporter) ExportSpan(ctx context.Context, span model.TraceSpan) {
	e.otlp.ExportSpan(ctx, span)
}

func (e *OpikExporter) Flush(ctx context.Context) error {
	return e.otlp.Flush(ctx)
}

func (e *OpikExporter) Shutdown(ctx context.Context) error {
	return e.otlp.Shutdown(ctx)
}

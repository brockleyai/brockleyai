package trace

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/brockleyai/brockleyai/internal/model"
)

// LangfuseExporter sends trace spans to Langfuse via its OTLP-compatible endpoint.
// It is a thin wrapper around OTLPExporter with Langfuse-specific endpoint and auth.
type LangfuseExporter struct {
	otlp *OTLPExporter
}

var _ model.TraceExporter = (*LangfuseExporter)(nil)

// NewLangfuseExporter creates a Langfuse exporter.
// host is the Langfuse base URL (e.g. "https://cloud.langfuse.com").
// publicKey and secretKey are Langfuse API credentials.
func NewLangfuseExporter(host, publicKey, secretKey string) *LangfuseExporter {
	endpoint := strings.TrimRight(host, "/") + "/api/public/otel/v1/traces"
	creds := base64.StdEncoding.EncodeToString([]byte(publicKey + ":" + secretKey))
	headers := map[string]string{
		"Authorization": "Basic " + creds,
	}
	return &LangfuseExporter{
		otlp: NewOTLPExporter(endpoint, headers),
	}
}

func (e *LangfuseExporter) ExportSpan(ctx context.Context, span model.TraceSpan) {
	e.otlp.ExportSpan(ctx, span)
}

func (e *LangfuseExporter) Flush(ctx context.Context) error {
	return e.otlp.Flush(ctx)
}

func (e *LangfuseExporter) Shutdown(ctx context.Context) error {
	return e.otlp.Shutdown(ctx)
}

package trace

import (
	"context"
	"errors"
	"sync"

	"github.com/brockleyai/brockleyai/internal/model"
)

// ExporterRegistry fans out trace spans to multiple registered exporters.
// It implements model.TraceExporter itself, so it can be used wherever
// a single TraceExporter is expected.
type ExporterRegistry struct {
	exporters []model.TraceExporter
	mu        sync.RWMutex
}

var _ model.TraceExporter = (*ExporterRegistry)(nil)

// NewExporterRegistry creates an empty registry with no exporters.
func NewExporterRegistry() *ExporterRegistry {
	return &ExporterRegistry{}
}

// Register adds a trace exporter to the registry.
func (r *ExporterRegistry) Register(exporter model.TraceExporter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.exporters = append(r.exporters, exporter)
}

// ExportSpan sends the span to all registered exporters.
func (r *ExporterRegistry) ExportSpan(ctx context.Context, span model.TraceSpan) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, exp := range r.exporters {
		exp.ExportSpan(ctx, span)
	}
}

// Flush flushes all registered exporters. Returns a joined error if any fail.
func (r *ExporterRegistry) Flush(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var errs []error
	for _, exp := range r.exporters {
		if err := exp.Flush(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Shutdown shuts down all registered exporters. Returns a joined error if any fail.
func (r *ExporterRegistry) Shutdown(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var errs []error
	for _, exp := range r.exporters {
		if err := exp.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

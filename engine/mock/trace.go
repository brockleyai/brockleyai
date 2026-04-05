package mock

import (
	"context"
	"sync"

	"github.com/brockleyai/brockleyai/internal/model"
)

// NoopTraceExporter discards all trace spans. Used when no external
// observability platform is configured.
type NoopTraceExporter struct{}

var _ model.TraceExporter = (*NoopTraceExporter)(nil)

func (NoopTraceExporter) ExportSpan(ctx context.Context, span model.TraceSpan) {}
func (NoopTraceExporter) Flush(ctx context.Context) error                      { return nil }
func (NoopTraceExporter) Shutdown(ctx context.Context) error                   { return nil }

// MockTraceExporter records all exported spans for test assertions.
type MockTraceExporter struct {
	mu sync.Mutex

	// Spans records all exported spans in order.
	Spans []model.TraceSpan

	// FlushCount tracks how many times Flush was called.
	FlushCount int

	// ShutdownCount tracks how many times Shutdown was called.
	ShutdownCount int
}

var _ model.TraceExporter = (*MockTraceExporter)(nil)

func (m *MockTraceExporter) ExportSpan(ctx context.Context, span model.TraceSpan) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Spans = append(m.Spans, span)
}

func (m *MockTraceExporter) Flush(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.FlushCount++
	return nil
}

func (m *MockTraceExporter) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ShutdownCount++
	return nil
}

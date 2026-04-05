package mock

import (
	"sync"

	"github.com/brockleyai/brockleyai/internal/model"
)

// NoopMetricsCollector discards all metrics. Zero overhead for production
// paths where Prometheus is not configured.
type NoopMetricsCollector struct{}

var _ model.MetricsCollector = (*NoopMetricsCollector)(nil)

func (NoopMetricsCollector) ExecutionStarted(graphID, graphName string) {}
func (NoopMetricsCollector) ExecutionCompleted(graphID, graphName string, durationMs int64, status string) {
}
func (NoopMetricsCollector) NodeStarted(graphID, nodeID, nodeType string) {}
func (NoopMetricsCollector) NodeCompleted(graphID, nodeID, nodeType string, durationMs int64, status string) {
}
func (NoopMetricsCollector) ProviderCallCompleted(provider, model string, durationMs int64, promptTokens, completionTokens int, status string) {
}
func (NoopMetricsCollector) MCPCallCompleted(toolName, mcpURL string, durationMs int64, status string) {
}
func (NoopMetricsCollector) HTTPRequestCompleted(method, path string, statusCode int, durationMs int64) {
}

// MetricsCall records a single metrics collector invocation.
type MetricsCall struct {
	Method string
	Args   []any
}

// TestMetricsCollector records all metrics calls for test assertions.
type TestMetricsCollector struct {
	mu    sync.Mutex
	Calls []MetricsCall
}

var _ model.MetricsCollector = (*TestMetricsCollector)(nil)

func (t *TestMetricsCollector) record(method string, args ...any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Calls = append(t.Calls, MetricsCall{Method: method, Args: args})
}

func (t *TestMetricsCollector) ExecutionStarted(graphID, graphName string) {
	t.record("ExecutionStarted", graphID, graphName)
}

func (t *TestMetricsCollector) ExecutionCompleted(graphID, graphName string, durationMs int64, status string) {
	t.record("ExecutionCompleted", graphID, graphName, durationMs, status)
}

func (t *TestMetricsCollector) NodeStarted(graphID, nodeID, nodeType string) {
	t.record("NodeStarted", graphID, nodeID, nodeType)
}

func (t *TestMetricsCollector) NodeCompleted(graphID, nodeID, nodeType string, durationMs int64, status string) {
	t.record("NodeCompleted", graphID, nodeID, nodeType, durationMs, status)
}

func (t *TestMetricsCollector) ProviderCallCompleted(provider, modelName string, durationMs int64, promptTokens, completionTokens int, status string) {
	t.record("ProviderCallCompleted", provider, modelName, durationMs, promptTokens, completionTokens, status)
}

func (t *TestMetricsCollector) MCPCallCompleted(toolName, mcpURL string, durationMs int64, status string) {
	t.record("MCPCallCompleted", toolName, mcpURL, durationMs, status)
}

func (t *TestMetricsCollector) HTTPRequestCompleted(method, path string, statusCode int, durationMs int64) {
	t.record("HTTPRequestCompleted", method, path, statusCode, durationMs)
}

package mock

import (
	"sync"

	"github.com/brockleyai/brockleyai/internal/model"
)

// MockEventEmitter is a test double for model.EventEmitter that collects all emitted events.
type MockEventEmitter struct {
	mu sync.Mutex

	// Events records all emitted events in order.
	Events []model.ExecutionEvent
}

var _ model.EventEmitter = (*MockEventEmitter)(nil)

func (m *MockEventEmitter) Emit(event model.ExecutionEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Events = append(m.Events, event)
}

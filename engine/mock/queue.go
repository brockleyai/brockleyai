package mock

import (
	"context"
	"sync"

	"github.com/brockleyai/brockleyai/internal/model"
)

// MockTaskQueue is a test double for model.TaskQueue that records enqueued tasks.
type MockTaskQueue struct {
	mu sync.Mutex

	// Tasks records all enqueued tasks in order.
	Tasks []*model.ExecutionTask
}

var _ model.TaskQueue = (*MockTaskQueue)(nil)

func (m *MockTaskQueue) Enqueue(ctx context.Context, task *model.ExecutionTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Tasks = append(m.Tasks, task)
	return nil
}

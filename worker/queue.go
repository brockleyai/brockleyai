package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/brockleyai/brockleyai/internal/model"
)

// AsynqTaskQueue implements model.TaskQueue using asynq backed by Redis.
// It enqueues graph:start tasks (the distributed orchestrator entry point).
type AsynqTaskQueue struct {
	client *asynq.Client
}

var _ model.TaskQueue = (*AsynqTaskQueue)(nil)

// NewAsynqTaskQueue creates an AsynqTaskQueue connected to the given Redis address.
func NewAsynqTaskQueue(redisAddr string) *AsynqTaskQueue {
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
	return &AsynqTaskQueue{client: client}
}

// Enqueue serializes an ExecutionTask and enqueues it as a graph:start task.
// The asynq task timeout is set to the graph timeout plus a buffer for cleanup.
func (q *AsynqTaskQueue) Enqueue(_ context.Context, task *model.ExecutionTask) error {
	payload, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal execution task: %w", err)
	}

	opts := []asynq.Option{asynq.Queue(QueueOrchestrator)}

	// Set asynq task timeout to graph timeout + 60s buffer.
	if task.Timeout > 0 {
		opts = append(opts, asynq.Timeout(time.Duration(task.Timeout+60)*time.Second))
	} else {
		// Default: 30 minutes for graph execution.
		opts = append(opts, asynq.Timeout(30*time.Minute))
	}

	// Disable asynq-level retries — the orchestrator handles retries internally.
	opts = append(opts, asynq.MaxRetry(0))

	asynqTask := asynq.NewTask(TaskTypeGraphStart, payload, opts...)
	_, err = q.client.Enqueue(asynqTask)
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	return nil
}

// Close closes the underlying asynq client.
func (q *AsynqTaskQueue) Close() error {
	return q.client.Close()
}

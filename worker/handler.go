// Package worker implements the asynq task handlers for Brockley graph execution.
//
// The distributed execution model uses these task types:
//   - graph:start     — Orchestrator that walks the graph, dispatches node tasks, collects results
//   - node:run        — Executes complex nodes (forEach, subgraph) that need a separate task
//   - node:llm-call   — Executes one LLM provider.Complete() call, handles tool loop MCP dispatch
//   - node:mcp-call   — Executes one MCP tool call or list_tools call
//   - node:api-call   — Executes one API tool HTTP call
//   - node:superagent — Superagent coordinator that stays alive and dispatches LLM/MCP as tasks
//   - node:code-exec  — One Python code execution, handled by the coderunner component
//
// All node tasks push results to a Redis list (exec:{execution_id}:results) that
// the orchestrator BRPOPs from. Tool loop MCP results use a scoped key.
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/brockleyai/brockleyai/internal/model"
)

// RedisEventEmitter publishes execution events to Redis pub/sub
// and sends step data to the StepWriter for PostgreSQL persistence.
type RedisEventEmitter struct {
	client      *redis.Client
	executionID string
	stepWriter  *StepWriter
	logger      *slog.Logger
}

// Emit publishes an event to Redis pub/sub and optionally writes
// to PostgreSQL via the StepWriter (for node_completed/node_failed events).
func (e *RedisEventEmitter) Emit(event model.ExecutionEvent) {
	// Publish to Redis pub/sub.
	channel := fmt.Sprintf("execution:%s:events", e.executionID)
	eventJSON, err := json.Marshal(event)
	if err != nil {
		e.logger.Error("failed to marshal event", "error", err, "event_type", event.Type)
		return
	}

	if err := e.client.Publish(context.Background(), channel, string(eventJSON)).Err(); err != nil {
		e.logger.Error("failed to publish event to Redis", "error", err, "event_type", event.Type)
	}

	// Write to StepWriter for PostgreSQL persistence (only for node completion events).
	if event.Type == model.EventNodeCompleted || event.Type == model.EventNodeFailed {
		status := model.StepStatusCompleted
		if event.Type == model.EventNodeFailed {
			status = model.StepStatusFailed
		}

		now := time.Now().UTC()
		step := &model.ExecutionStep{
			ID:          fmt.Sprintf("step_%s_%s_%d", e.executionID, event.NodeID, event.Iteration),
			ExecutionID: e.executionID,
			NodeID:      event.NodeID,
			NodeType:    event.NodeType,
			Iteration:   event.Iteration,
			Status:      status,
			Input:       event.Input,
			Output:      event.Output,
			Attempt:     event.Attempt,
			DurationMs:  &event.DurationMs,
			LLMUsage:    event.LLMUsage,
			LLMDebug:    event.LLMDebug,
			CreatedAt:   now,
		}
		if event.Error != nil {
			errJSON, _ := json.Marshal(event.Error)
			step.Error = errJSON
		}
		e.stepWriter.Write(step)
	}
}

// StepWriter writes execution steps to PostgreSQL in a background goroutine.
// Writes are non-blocking to avoid slowing down graph execution.
type StepWriter struct {
	store model.Store
	ch    chan *model.ExecutionStep
	wg    sync.WaitGroup
	done  chan struct{}
}

// NewStepWriter creates a StepWriter with the given buffer size and starts the background writer.
func NewStepWriter(store model.Store, bufSize int) *StepWriter {
	w := &StepWriter{
		store: store,
		ch:    make(chan *model.ExecutionStep, bufSize),
		done:  make(chan struct{}),
	}
	go w.run()
	return w
}

// Write queues a step for background writing to PostgreSQL. Non-blocking.
func (w *StepWriter) Write(step *model.ExecutionStep) {
	w.wg.Add(1)
	select {
	case w.ch <- step:
	default:
		// Buffer full; drop the step to avoid blocking execution.
		w.wg.Done()
	}
}

// Flush blocks until all queued writes are completed.
func (w *StepWriter) Flush() {
	w.wg.Wait()
}

// Close flushes remaining writes and stops the background writer.
func (w *StepWriter) Close() {
	w.Flush()
	close(w.ch)
	<-w.done
}

func (w *StepWriter) run() {
	defer close(w.done)
	for step := range w.ch {
		if err := w.store.InsertExecutionStep(context.Background(), step); err != nil {
			// Best-effort: log but don't fail the execution.
			_ = err
		}
		w.wg.Done()
	}
}

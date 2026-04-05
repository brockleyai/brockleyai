package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/brockleyai/brockleyai/internal/model"
	"github.com/brockleyai/brockleyai/server/middleware"
)

// ExecutionHandler handles /api/v1/executions endpoints.
type ExecutionHandler struct {
	store     model.Store
	queue     model.TaskQueue
	logger    *slog.Logger
	redisAddr string
}

// NewExecutionHandler creates a new ExecutionHandler.
func NewExecutionHandler(store model.Store, queue model.TaskQueue, redisAddr string, logger *slog.Logger) *ExecutionHandler {
	return &ExecutionHandler{
		store:     store,
		queue:     queue,
		logger:    logger,
		redisAddr: redisAddr,
	}
}

// Invoke handles POST /api/v1/executions.
func (h *ExecutionHandler) Invoke(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())

	if h.queue == nil {
		writeError(w, http.StatusServiceUnavailable, ErrCodeServiceUnavail, "task queue not configured (REDIS_URL required)", requestID)
		return
	}

	var body InvokeRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "invalid request body: "+err.Error(), requestID)
		return
	}

	if body.GraphID == "" {
		writeError(w, http.StatusBadRequest, ErrCodeValidation, "graph_id is required", requestID)
		return
	}

	// Load graph from store.
	graph, err := h.store.GetGraph(r.Context(), tenantID, body.GraphID)
	if err != nil {
		h.logger.Error("failed to get graph", "error", err, "graph_id", body.GraphID, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to load graph", requestID)
		return
	}
	if graph == nil {
		writeError(w, http.StatusNotFound, ErrCodeNotFound, "graph not found", requestID)
		return
	}

	// Serialize graph for the task payload.
	graphJSON, err := json.Marshal(graph)
	if err != nil {
		h.logger.Error("failed to marshal graph", "error", err, "graph_id", body.GraphID, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to serialize graph", requestID)
		return
	}

	// Serialize input.
	var inputJSON json.RawMessage
	if body.Input != nil {
		inputJSON, err = json.Marshal(body.Input)
		if err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeValidation, "invalid input: "+err.Error(), requestID)
			return
		}
	}

	// Determine mode.
	mode := model.ExecutionModeAsync
	if body.Mode == string(model.ExecutionModeSync) {
		mode = model.ExecutionModeSync
	}

	// Create Execution record.
	now := time.Now().UTC()
	executionID := generateExecutionID()
	exec := &model.Execution{
		ID:            executionID,
		TenantID:      tenantID,
		GraphID:       body.GraphID,
		GraphVersion:  graph.Version,
		Status:        model.ExecutionStatusPending,
		Input:         inputJSON,
		Trigger:       model.TriggerAPI,
		CorrelationID: body.CorrelationID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if body.Timeout > 0 {
		exec.TimeoutSeconds = &body.Timeout
	}
	if body.Debug {
		exec.Metadata = json.RawMessage(`{"debug":true}`)
	}

	if err := h.store.CreateExecution(r.Context(), exec); err != nil {
		h.logger.Error("failed to create execution", "error", err, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to create execution", requestID)
		return
	}

	// Create and enqueue task.
	task := &model.ExecutionTask{
		ExecutionID: executionID,
		GraphID:     body.GraphID,
		GraphName:   graph.Name,
		TenantID:    tenantID,
		Graph:       graphJSON,
		Input:       inputJSON,
		Timeout:     body.Timeout,
		Debug:       body.Debug,
	}

	// For sync mode, subscribe to Redis BEFORE enqueuing to avoid
	// missing the completion event if the worker finishes quickly.
	var syncSub *redis.PubSub
	if mode == model.ExecutionModeSync {
		rdb := redis.NewClient(&redis.Options{Addr: h.redisAddr})
		channel := fmt.Sprintf("execution:%s:events", executionID)
		syncSub = rdb.Subscribe(r.Context(), channel)
		// Ensure subscription is established before proceeding.
		if _, err := syncSub.Receive(r.Context()); err != nil {
			rdb.Close()
			syncSub.Close()
			h.logger.Error("failed to subscribe for sync execution", "error", err, "execution_id", executionID, "request_id", requestID)
			writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to set up sync execution", requestID)
			return
		}
		defer rdb.Close()
		defer syncSub.Close()
	}

	if err := h.queue.Enqueue(r.Context(), task); err != nil {
		h.logger.Error("failed to enqueue task", "error", err, "execution_id", executionID, "request_id", requestID)
		// Update execution to failed since we couldn't enqueue.
		exec.Status = model.ExecutionStatusFailed
		exec.Error = &model.ExecutionError{Code: "ENQUEUE_FAILED", Message: err.Error()}
		exec.UpdatedAt = time.Now().UTC()
		_ = h.store.UpdateExecution(r.Context(), exec)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to enqueue execution", requestID)
		return
	}

	h.logger.Info("execution enqueued", "execution_id", executionID, "graph_id", body.GraphID, "mode", mode, "request_id", requestID)

	// Sync mode: block until completion.
	if mode == model.ExecutionModeSync {
		h.handleSyncExecutionWithSub(w, r, exec, syncSub, requestID)
		return
	}

	// Async mode: return 202 with execution ID.
	writeJSON(w, http.StatusAccepted, exec)
}

// handleSyncExecutionWithSub blocks on a pre-established Redis subscription
// until the execution completes, fails, or times out.
func (h *ExecutionHandler) handleSyncExecutionWithSub(w http.ResponseWriter, r *http.Request, exec *model.Execution, sub *redis.PubSub, requestID string) {
	timeout := 300 * time.Second // default 5 minutes
	if exec.TimeoutSeconds != nil && *exec.TimeoutSeconds > 0 {
		timeout = time.Duration(*exec.TimeoutSeconds) * time.Second
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// Use ReceiveMessage in a loop instead of Channel() to avoid
	// goroutine startup races.
	for {
		msg, err := sub.ReceiveMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				writeError(w, http.StatusRequestTimeout, "TIMEOUT", "execution timed out", requestID)
				return
			}
			continue
		}

		var event model.ExecutionEvent
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			continue
		}

		if event.Type == model.EventExecutionCompleted || event.Type == model.EventExecutionFailed || event.Type == model.EventExecutionCancelled {
			// Build the response from the event + original exec to avoid a
			// DB read race (the worker publishes to Redis before the DB write
			// is visible to other connections).
			exec.Status = model.ExecutionStatus(event.Status)
			exec.Output = event.Output
			exec.State = event.State
			now := event.Timestamp
			exec.CompletedAt = &now
			exec.UpdatedAt = now
			if event.Error != nil {
				exec.Error = event.Error
			}
			writeJSON(w, http.StatusOK, exec)
			return
		}
	}
}

// Get handles GET /api/v1/executions/{id}.
func (h *ExecutionHandler) Get(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())
	id := r.PathValue("id")

	exec, err := h.store.GetExecution(r.Context(), tenantID, id)
	if err != nil {
		h.logger.Error("failed to get execution", "error", err, "execution_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to get execution", requestID)
		return
	}
	if exec == nil {
		writeError(w, http.StatusNotFound, ErrCodeNotFound, "execution not found", requestID)
		return
	}

	writeJSON(w, http.StatusOK, exec)
}

// List handles GET /api/v1/executions.
func (h *ExecutionHandler) List(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())

	graphID := r.URL.Query().Get("graph_id")
	status := r.URL.Query().Get("status")
	cursor := r.URL.Query().Get("cursor")
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	executions, nextCursor, err := h.store.ListExecutions(r.Context(), tenantID, graphID, status, cursor, limit)
	if err != nil {
		h.logger.Error("failed to list executions", "error", err, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to list executions", requestID)
		return
	}

	writeJSON(w, http.StatusOK, ListResponse[*model.Execution]{
		Items:      executions,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	})
}

// GetSteps handles GET /api/v1/executions/{id}/steps.
func (h *ExecutionHandler) GetSteps(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())
	id := r.PathValue("id")

	// Verify the execution belongs to this tenant before returning steps.
	exec, err := h.store.GetExecution(r.Context(), tenantID, id)
	if err != nil {
		h.logger.Error("failed to get execution for steps", "error", err, "execution_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to get execution", requestID)
		return
	}
	if exec == nil {
		writeError(w, http.StatusNotFound, ErrCodeNotFound, "execution not found", requestID)
		return
	}

	steps, err := h.store.ListExecutionSteps(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to list execution steps", "error", err, "execution_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to list execution steps", requestID)
		return
	}

	writeJSON(w, http.StatusOK, ListResponse[*model.ExecutionStep]{
		Items:   steps,
		HasMore: false,
	})
}

// Cancel handles POST /api/v1/executions/{id}/cancel.
func (h *ExecutionHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	tenantID := middleware.GetTenantID(r.Context())
	id := r.PathValue("id")

	exec, err := h.store.GetExecution(r.Context(), tenantID, id)
	if err != nil {
		h.logger.Error("failed to get execution for cancel", "error", err, "execution_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to get execution", requestID)
		return
	}
	if exec == nil {
		writeError(w, http.StatusNotFound, ErrCodeNotFound, "execution not found", requestID)
		return
	}

	if exec.Status != model.ExecutionStatusPending && exec.Status != model.ExecutionStatusRunning {
		writeError(w, http.StatusConflict, ErrCodeConflict, fmt.Sprintf("cannot cancel execution in status %q", exec.Status), requestID)
		return
	}

	now := time.Now().UTC()
	exec.Status = model.ExecutionStatusCancelled
	exec.CompletedAt = &now
	exec.UpdatedAt = now
	if err := h.store.UpdateExecution(r.Context(), exec); err != nil {
		h.logger.Error("failed to cancel execution", "error", err, "execution_id", id, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to cancel execution", requestID)
		return
	}

	// Publish cancel event to Redis so the worker can check.
	if h.redisAddr != "" {
		rdb := redis.NewClient(&redis.Options{Addr: h.redisAddr})
		defer rdb.Close()

		cancelEvent := model.ExecutionEvent{
			Type:        model.EventExecutionCancelled,
			ExecutionID: id,
			Timestamp:   now,
			Status:      string(model.ExecutionStatusCancelled),
		}
		eventJSON, _ := json.Marshal(cancelEvent)
		channel := fmt.Sprintf("execution:%s:events", id)
		rdb.Publish(r.Context(), channel, string(eventJSON))
	}

	h.logger.Info("execution cancelled", "execution_id", id, "request_id", requestID)
	writeJSON(w, http.StatusOK, exec)
}

// Stream handles GET /api/v1/executions/{id}/stream using Server-Sent Events.
func (h *ExecutionHandler) Stream(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	id := r.PathValue("id")

	if h.redisAddr == "" {
		writeError(w, http.StatusServiceUnavailable, ErrCodeServiceUnavail, "streaming not available (REDIS_URL required)", requestID)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "streaming not supported", requestID)
		return
	}

	// Set SSE headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Request-Id", requestID)

	rdb := redis.NewClient(&redis.Options{Addr: h.redisAddr})
	defer rdb.Close()

	channel := fmt.Sprintf("execution:%s:events", id)
	sub := rdb.Subscribe(r.Context(), channel)
	defer sub.Close()

	ch := sub.Channel()

	for {
		select {
		case <-r.Context().Done():
			// Client disconnected.
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}

			// Parse to determine event type for SSE event field.
			var event model.ExecutionEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				continue
			}

			// Write SSE event.
			fmt.Fprintf(w, "event: %s\n", event.Type)
			fmt.Fprintf(w, "data: %s\n\n", msg.Payload)
			flusher.Flush()

			// Close stream on terminal events.
			if event.Type == model.EventExecutionCompleted ||
				event.Type == model.EventExecutionFailed ||
				event.Type == model.EventExecutionCancelled {
				return
			}
		}
	}
}

func generateExecutionID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return "exec_" + hex.EncodeToString(b)
}

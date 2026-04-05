package worker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/brockleyai/brockleyai/internal/model"
)

// RedisPublisher abstracts Redis Publish for testability.
// *redis.Client satisfies this interface.
type RedisPublisher interface {
	Publish(ctx context.Context, channel string, message interface{}) *redis.IntCmd
}

// emitRetryingEvent publishes an EventNodeRetrying event to Redis pub/sub
// so SSE consumers see retry activity. These events are transient and not
// persisted to PostgreSQL.
func emitRetryingEvent(pub RedisPublisher, executionID, nodeID, nodeType string, attempt int, errMsg string) {
	event := model.ExecutionEvent{
		Type:        model.EventNodeRetrying,
		ExecutionID: executionID,
		NodeID:      nodeID,
		NodeType:    nodeType,
		Timestamp:   time.Now(),
		Attempt:     attempt,
		Error: &model.ExecutionError{
			Code:    "RETRY",
			Message: errMsg,
			NodeID:  nodeID,
		},
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		return
	}

	channel := "execution:" + executionID + ":events"
	pub.Publish(context.Background(), channel, string(eventJSON))
}

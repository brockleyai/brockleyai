package mock

import (
	"context"
	"fmt"
	"sync"

	"github.com/brockleyai/brockleyai/internal/model"
)

// MockCompletionResponse holds a scripted response for the mock provider.
type MockCompletionResponse struct {
	Content      string
	ToolCalls    []model.ToolCall
	FinishReason string // defaults to "stop" if empty
}

// MockLLMProvider is a test double for model.LLMProvider.
// It returns sequential responses, supports error injection, and records all calls.
type MockLLMProvider struct {
	mu sync.Mutex

	// Responses are consumed in order by Complete calls.
	// If exhausted, Complete returns an error.
	Responses []string

	// CompletionResponses are consumed in order by Complete calls.
	// Takes priority over Responses when present at a given index.
	CompletionResponses []MockCompletionResponse

	// Errors are consumed in parallel with Responses.
	// A nil entry means no error for that call.
	Errors []error

	// StreamChunks are consumed in order by Stream calls.
	// Each element is a slice of content strings; the last chunk in each
	// slice is sent with Done=true.
	StreamChunks [][]string

	// Calls records every CompletionRequest passed to Complete.
	Calls []*model.CompletionRequest

	// StreamCalls records every CompletionRequest passed to Stream.
	StreamCalls []*model.CompletionRequest

	callIdx   int
	streamIdx int
}

var _ model.LLMProvider = (*MockLLMProvider)(nil)

func (m *MockLLMProvider) Name() string {
	return "mock"
}

func (m *MockLLMProvider) Complete(ctx context.Context, req *model.CompletionRequest) (*model.CompletionResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Calls = append(m.Calls, req)
	idx := m.callIdx
	m.callIdx++

	// Check for injected error.
	if idx < len(m.Errors) && m.Errors[idx] != nil {
		return nil, m.Errors[idx]
	}

	// Use CompletionResponses if available at this index.
	if idx < len(m.CompletionResponses) {
		cr := m.CompletionResponses[idx]
		finishReason := cr.FinishReason
		if finishReason == "" {
			if len(cr.ToolCalls) > 0 {
				finishReason = "tool_calls"
			} else {
				finishReason = "stop"
			}
		}
		return &model.CompletionResponse{
			Content:      cr.Content,
			Model:        req.Model,
			FinishReason: finishReason,
			ToolCalls:    cr.ToolCalls,
		}, nil
	}

	if idx >= len(m.Responses) {
		return nil, fmt.Errorf("mock provider: no more responses (call index %d)", idx)
	}

	return &model.CompletionResponse{
		Content:      m.Responses[idx],
		Model:        req.Model,
		FinishReason: "stop",
	}, nil
}

func (m *MockLLMProvider) Stream(ctx context.Context, req *model.CompletionRequest) (<-chan model.StreamChunk, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StreamCalls = append(m.StreamCalls, req)
	idx := m.streamIdx
	m.streamIdx++

	if idx >= len(m.StreamChunks) {
		return nil, fmt.Errorf("mock provider: no more stream chunks (stream index %d)", idx)
	}

	chunks := m.StreamChunks[idx]
	ch := make(chan model.StreamChunk, len(chunks))
	for i, content := range chunks {
		done := i == len(chunks)-1
		chunk := model.StreamChunk{Content: content, Done: done}
		if done {
			chunk.Usage = &model.LLMUsage{
				Provider: "mock",
				Model:    req.Model,
			}
		}
		ch <- chunk
	}
	close(ch)
	return ch, nil
}

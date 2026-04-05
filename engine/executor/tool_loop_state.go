package executor

import (
	"encoding/json"

	"github.com/brockleyai/brockleyai/internal/model"
)

// SerializableToolLoopState holds the complete state needed to resume a tool loop
// across task boundaries in the distributed execution model.
type SerializableToolLoopState struct {
	MaxCalls       int                        `json:"max_calls"`
	MaxIterations  int                        `json:"max_iterations"`
	Iteration      int                        `json:"iteration"`
	TotalToolCalls int                        `json:"total_tool_calls"`
	History        []ToolCallHistoryEntry     `json:"history,omitempty"`
	Routing        map[string]model.ToolRoute `json:"routing"`
	FinishReason   string                     `json:"finish_reason,omitempty"`
	NodeConfig     json.RawMessage            `json:"node_config,omitempty"`
	NodeInputs     map[string]any             `json:"node_inputs,omitempty"`
	NodeState      map[string]any             `json:"node_state,omitempty"`
	NodeMeta       map[string]any             `json:"node_meta,omitempty"`
}

// BuildInitialToolLoopState creates the initial ToolLoopState for a new tool loop.
func BuildInitialToolLoopState(
	cfg *model.LLMNodeConfig,
	routing map[string]model.ToolRoute,
	inputs map[string]any,
	nctx *NodeContext,
) *SerializableToolLoopState {
	maxCalls := defaultMaxToolCalls
	if cfg.MaxToolCalls != nil {
		maxCalls = *cfg.MaxToolCalls
	}
	maxIterations := defaultMaxLoopIterations
	if cfg.MaxLoopIterations != nil {
		maxIterations = *cfg.MaxLoopIterations
	}

	cfgJSON, _ := json.Marshal(cfg)

	state := &SerializableToolLoopState{
		MaxCalls:      maxCalls,
		MaxIterations: maxIterations,
		Routing:       routing,
		NodeConfig:    cfgJSON,
		NodeInputs:    inputs,
	}

	if nctx != nil {
		state.NodeState = nctx.State
		state.NodeMeta = nctx.Meta
	}

	return state
}

package model

// GraphStatus represents the lifecycle state of a graph.
type GraphStatus string

const (
	GraphStatusDraft    GraphStatus = "draft"
	GraphStatusActive   GraphStatus = "active"
	GraphStatusArchived GraphStatus = "archived"
)

// Reducer defines how state field writes accumulate.
type Reducer string

const (
	ReducerReplace Reducer = "replace"
	ReducerAppend  Reducer = "append"
	ReducerMerge   Reducer = "merge"
)

// ExecutionStatus represents the lifecycle state of an execution.
type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusCancelled ExecutionStatus = "cancelled"
	ExecutionStatusTimedOut  ExecutionStatus = "timed_out"
)

// StepStatus represents the lifecycle state of an execution step.
type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"
	StepStatusRunning   StepStatus = "running"
	StepStatusCompleted StepStatus = "completed"
	StepStatusFailed    StepStatus = "failed"
	StepStatusSkipped   StepStatus = "skipped"
	StepStatusRetrying  StepStatus = "retrying"
)

// ExecutionTrigger identifies what initiated an execution.
type ExecutionTrigger string

const (
	TriggerAPI       ExecutionTrigger = "api"
	TriggerUI        ExecutionTrigger = "ui"
	TriggerCLI       ExecutionTrigger = "cli"
	TriggerTerraform ExecutionTrigger = "terraform"
	TriggerMCP       ExecutionTrigger = "mcp"
	TriggerScheduled ExecutionTrigger = "scheduled"
)

// ExecutionMode determines how the client receives results.
type ExecutionMode string

const (
	ExecutionModeSync  ExecutionMode = "sync"
	ExecutionModeAsync ExecutionMode = "async"
)

// ResponseFormat determines LLM response handling.
type ResponseFormat string

const (
	ResponseFormatText ResponseFormat = "text"
	ResponseFormatJSON ResponseFormat = "json"
)

// ProviderType identifies the LLM provider.
type ProviderType string

const (
	ProviderOpenAI     ProviderType = "openai"
	ProviderAnthropic  ProviderType = "anthropic"
	ProviderGoogle     ProviderType = "google"
	ProviderOpenRouter ProviderType = "openrouter"
	ProviderBedrock    ProviderType = "bedrock"
	ProviderCustom     ProviderType = "custom"
)

// Built-in node type constants.
const (
	NodeTypeInput          = "input"
	NodeTypeOutput         = "output"
	NodeTypeLLM            = "llm"
	NodeTypeTool           = "tool"
	NodeTypeConditional    = "conditional"
	NodeTypeTransform      = "transform"
	NodeTypeForEach        = "foreach"
	NodeTypeSubgraph       = "subgraph"
	NodeTypeHumanInTheLoop = "human_in_the_loop"
	NodeTypeSuperagent     = "superagent"
	NodeTypeAPITool        = "api_tool"
)

// EventType identifies the type of execution event.
type EventType string

const (
	EventExecutionStarted     EventType = "execution_started"
	EventExecutionCompleted   EventType = "execution_completed"
	EventExecutionFailed      EventType = "execution_failed"
	EventExecutionCancelled   EventType = "execution_cancelled"
	EventNodeStarted          EventType = "node_started"
	EventNodeCompleted        EventType = "node_completed"
	EventNodeFailed           EventType = "node_failed"
	EventNodeSkipped          EventType = "node_skipped"
	EventNodeRetrying         EventType = "node_retrying"
	EventStateUpdated         EventType = "state_updated"
	EventForEachItemStarted   EventType = "foreach_item_started"
	EventForEachItemCompleted EventType = "foreach_item_completed"
	EventForEachItemFailed    EventType = "foreach_item_failed"
	EventBackEdgeEvaluated    EventType = "back_edge_evaluated"
	EventLLMToken             EventType = "llm_token"
	EventToolCallStarted      EventType = "tool_call_started"
	EventToolCallCompleted    EventType = "tool_call_completed"
	EventToolLoopIteration    EventType = "tool_loop_iteration"
	EventToolLoopCompleted    EventType = "tool_loop_completed"

	// Superagent event types
	EventSuperagentStarted        EventType = "superagent_started"
	EventSuperagentIteration      EventType = "superagent_iteration"
	EventSuperagentEvaluation     EventType = "superagent_evaluation"
	EventSuperagentReflection     EventType = "superagent_reflection"
	EventSuperagentStuckWarning   EventType = "superagent_stuck_warning"
	EventSuperagentCompaction     EventType = "superagent_compaction"
	EventSuperagentMemoryStore    EventType = "superagent_memory_store"
	EventSuperagentBufferFinalize EventType = "superagent_buffer_finalize"
	EventSuperagentToolCall       EventType = "superagent_tool_call"
	EventSuperagentCompleted      EventType = "superagent_completed"
)

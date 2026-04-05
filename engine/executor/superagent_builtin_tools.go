package executor

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/brockleyai/brockleyai/internal/model"
)

// --- Task Tracker ---

// TaskStatus represents the status of a task.
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted  TaskStatus = "completed"
)

// TaskPriority represents the priority of a task.
type TaskPriority string

const (
	TaskPriorityHigh   TaskPriority = "high"
	TaskPriorityMedium TaskPriority = "medium"
	TaskPriorityLow    TaskPriority = "low"
)

// Task represents a tracked task in the superagent loop.
type Task struct {
	ID          string       `json:"id"`
	Description string       `json:"description"`
	Priority    TaskPriority `json:"priority"`
	Status      TaskStatus   `json:"status"`
}

// TaskTracker manages tasks for a superagent execution.
type TaskTracker struct {
	tasks  []Task
	nextID int
}

// NewTaskTracker creates a new TaskTracker.
func NewTaskTracker() *TaskTracker {
	return &TaskTracker{nextID: 1}
}

// Create adds a new task with an auto-incremented ID.
func (t *TaskTracker) Create(description string, priority TaskPriority) *Task {
	if priority == "" {
		priority = TaskPriorityMedium
	}
	task := Task{
		ID:          fmt.Sprintf("%d", t.nextID),
		Description: description,
		Priority:    priority,
		Status:      TaskStatusPending,
	}
	t.nextID++
	t.tasks = append(t.tasks, task)
	return &t.tasks[len(t.tasks)-1]
}

// Update changes the status of a task by ID.
func (t *TaskTracker) Update(id string, status TaskStatus) error {
	for i := range t.tasks {
		if t.tasks[i].ID == id {
			t.tasks[i].Status = status
			return nil
		}
	}
	return fmt.Errorf("task %q not found", id)
}

// List returns a copy of all tasks.
func (t *TaskTracker) List() []Task {
	result := make([]Task, len(t.tasks))
	copy(result, t.tasks)
	return result
}

// BuildTaskReminder returns a formatted string of all tasks for injection into prompts.
func (t *TaskTracker) BuildTaskReminder() string {
	if len(t.tasks) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("[Tasks]")
	for _, task := range t.tasks {
		fmt.Fprintf(&sb, "\n- [%s] #%s: %s (%s)", task.Status, task.ID, task.Description, task.Priority)
	}
	return sb.String()
}

// --- Buffer Manager ---

// Buffer represents a named content buffer for structured output assembly.
type Buffer struct {
	Name        string
	content     strings.Builder
	FinalizedTo string // output port name, empty if not finalized
}

// BufferManager manages named content buffers.
type BufferManager struct {
	buffers map[string]*Buffer
}

// NewBufferManager creates a new BufferManager.
func NewBufferManager() *BufferManager {
	return &BufferManager{buffers: make(map[string]*Buffer)}
}

// Create creates a new named buffer.
func (bm *BufferManager) Create(name string) error {
	if _, exists := bm.buffers[name]; exists {
		return fmt.Errorf("buffer %q already exists", name)
	}
	bm.buffers[name] = &Buffer{Name: name}
	return nil
}

// Append appends content to a buffer.
func (bm *BufferManager) Append(name, content string) error {
	buf, err := bm.getWritable(name)
	if err != nil {
		return err
	}
	buf.content.WriteString(content)
	return nil
}

// Prepend prepends content to a buffer.
func (bm *BufferManager) Prepend(name, content string) error {
	buf, err := bm.getWritable(name)
	if err != nil {
		return err
	}
	current := buf.content.String()
	buf.content.Reset()
	buf.content.WriteString(content)
	buf.content.WriteString(current)
	return nil
}

// Insert inserts content after the first occurrence of the 'after' marker.
func (bm *BufferManager) Insert(name, after, content string) error {
	buf, err := bm.getWritable(name)
	if err != nil {
		return err
	}
	current := buf.content.String()
	idx := strings.Index(current, after)
	if idx < 0 {
		return fmt.Errorf("marker %q not found in buffer %q", after, name)
	}
	insertPos := idx + len(after)
	result := current[:insertPos] + content + current[insertPos:]
	buf.content.Reset()
	buf.content.WriteString(result)
	return nil
}

// Replace replaces occurrences of old with new in a buffer. count=0 means replace all.
func (bm *BufferManager) Replace(name, old, new string, count int) error {
	buf, err := bm.getWritable(name)
	if err != nil {
		return err
	}
	current := buf.content.String()
	if count == 0 {
		count = -1
	}
	result := strings.Replace(current, old, new, count)
	buf.content.Reset()
	buf.content.WriteString(result)
	return nil
}

// Delete removes characters in the range [start, end) from a buffer.
func (bm *BufferManager) Delete(name string, start, end int) error {
	buf, err := bm.getWritable(name)
	if err != nil {
		return err
	}
	current := buf.content.String()
	if start < 0 || end > len(current) || start > end {
		return fmt.Errorf("delete range [%d, %d) out of bounds for buffer %q (length %d)", start, end, name, len(current))
	}
	result := current[:start] + current[end:]
	buf.content.Reset()
	buf.content.WriteString(result)
	return nil
}

// Read returns the content of a buffer, optionally sliced by start/end character positions.
func (bm *BufferManager) Read(name string, start, end *int) (string, error) {
	buf, ok := bm.buffers[name]
	if !ok {
		return "", fmt.Errorf("buffer %q not found", name)
	}
	content := buf.content.String()
	s := 0
	e := len(content)
	if start != nil {
		s = *start
	}
	if end != nil {
		e = *end
	}
	if s < 0 || e > len(content) || s > e {
		return "", fmt.Errorf("read range [%d, %d) out of bounds for buffer %q (length %d)", s, e, name, len(content))
	}
	return content[s:e], nil
}

// Length returns the character count of a buffer's content.
func (bm *BufferManager) Length(name string) (int, error) {
	buf, ok := bm.buffers[name]
	if !ok {
		return 0, fmt.Errorf("buffer %q not found", name)
	}
	return buf.content.Len(), nil
}

// Finalize marks a buffer as immutable and maps it to an output port.
func (bm *BufferManager) Finalize(name, outputPort string) error {
	buf, ok := bm.buffers[name]
	if !ok {
		return fmt.Errorf("buffer %q not found", name)
	}
	if buf.FinalizedTo != "" {
		return fmt.Errorf("buffer %q already finalized to port %q", name, buf.FinalizedTo)
	}
	buf.FinalizedTo = outputPort
	return nil
}

// GetFinalizedBuffers returns a map of outputPort -> content for all finalized buffers.
func (bm *BufferManager) GetFinalizedBuffers() map[string]string {
	result := make(map[string]string)
	for _, buf := range bm.buffers {
		if buf.FinalizedTo != "" {
			result[buf.FinalizedTo] = buf.content.String()
		}
	}
	return result
}

// getWritable returns a buffer if it exists and is not finalized.
func (bm *BufferManager) getWritable(name string) (*Buffer, error) {
	buf, ok := bm.buffers[name]
	if !ok {
		return nil, fmt.Errorf("buffer %q not found", name)
	}
	if buf.FinalizedTo != "" {
		return nil, fmt.Errorf("buffer %q is finalized and immutable", name)
	}
	return buf, nil
}

// --- Memory Store ---

// MemoryEntry represents an entry in the shared memory store.
type MemoryEntry struct {
	Key       string   `json:"key"`
	Content   string   `json:"content"`
	Tags      []string `json:"tags,omitempty"`
	Namespace string   `json:"namespace"`
}

// MemoryStore provides cross-node shared memory for superagent nodes.
type MemoryStore struct {
	entries   []MemoryEntry
	namespace string
}

// NewMemoryStore creates a new MemoryStore with the given namespace.
func NewMemoryStore(namespace string) *MemoryStore {
	return &MemoryStore{namespace: namespace}
}

// Store stores a key-value pair with optional tags. If the key already exists in
// the same namespace, it is overwritten.
func (ms *MemoryStore) Store(key, content string, tags []string) {
	fullKey := ms.namespace + "/" + key
	for i, e := range ms.entries {
		if e.Key == fullKey {
			ms.entries[i].Content = content
			ms.entries[i].Tags = tags
			return
		}
	}
	ms.entries = append(ms.entries, MemoryEntry{
		Key:       fullKey,
		Content:   content,
		Tags:      tags,
		Namespace: ms.namespace,
	})
}

// Recall searches all entries by substring match on content AND tag intersection.
func (ms *MemoryStore) Recall(query string, tags []string) []MemoryEntry {
	var results []MemoryEntry
	for _, e := range ms.entries {
		if !strings.Contains(e.Content, query) {
			continue
		}
		if len(tags) > 0 && !hasTagIntersection(e.Tags, tags) {
			continue
		}
		results = append(results, e)
	}
	return results
}

// List returns entries, optionally filtered by tags. Empty/nil tags returns all.
func (ms *MemoryStore) List(tags []string) []MemoryEntry {
	if len(tags) == 0 {
		result := make([]MemoryEntry, len(ms.entries))
		copy(result, ms.entries)
		return result
	}
	var results []MemoryEntry
	for _, e := range ms.entries {
		if hasTagIntersection(e.Tags, tags) {
			results = append(results, e)
		}
	}
	return results
}

// LoadFromState hydrates the memory store from a serialized state value.
func (ms *MemoryStore) LoadFromState(state map[string]any) {
	if state == nil {
		return
	}
	b, err := json.Marshal(state)
	if err != nil {
		return
	}
	var entries map[string]MemoryEntry
	if err := json.Unmarshal(b, &entries); err != nil {
		return
	}
	ms.entries = nil
	for _, entry := range entries {
		ms.entries = append(ms.entries, entry)
	}
	// Sort for deterministic ordering.
	sort.Slice(ms.entries, func(i, j int) bool {
		return ms.entries[i].Key < ms.entries[j].Key
	})
}

// ToStateValue serializes entries as a map[key]entry for merge reducer compatibility.
func (ms *MemoryStore) ToStateValue() map[string]any {
	result := make(map[string]any)
	for _, e := range ms.entries {
		result[e.Key] = e
	}
	return result
}

// hasTagIntersection returns true if the two slices share at least one element.
func hasTagIntersection(a, b []string) bool {
	set := make(map[string]struct{}, len(a))
	for _, t := range a {
		set[t] = struct{}{}
	}
	for _, t := range b {
		if _, ok := set[t]; ok {
			return true
		}
	}
	return false
}

// --- Tool Definitions ---

// GetBuiltInToolDefinitions returns LLM tool definitions for all enabled built-in tools.
func GetBuiltInToolDefinitions(cfg *model.SuperagentNodeConfig) []model.LLMToolDefinition {
	var tools []model.LLMToolDefinition

	// Task tools: always included unless explicitly disabled.
	tasksEnabled := cfg.Overrides == nil || cfg.Overrides.TaskTracking == nil ||
		cfg.Overrides.TaskTracking.Enabled == nil || *cfg.Overrides.TaskTracking.Enabled
	if tasksEnabled {
		tools = append(tools, taskToolDefinitions()...)
	}

	// Memory tools: only when shared memory is enabled.
	if cfg.SharedMemory != nil && cfg.SharedMemory.Enabled {
		tools = append(tools, memoryToolDefinitions()...)
	}

	// Buffer tools: always included.
	tools = append(tools, bufferToolDefinitions()...)

	// Code execution tools: only when enabled.
	if cfg.CodeExecution != nil && cfg.CodeExecution.Enabled {
		tools = append(tools, codeExecutionToolDefinitions()...)
	}

	return tools
}

func codeExecutionToolDefinitions() []model.LLMToolDefinition {
	return []model.LLMToolDefinition{
		{
			Name:        "_code_execute",
			Description: "Execute Python code. Use for data transformation, computation, assembling large outputs, or batching tool calls. Call _code_guidelines() first for usage instructions.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"code":{"type":"string","description":"Python 3 code to execute. Has access to the brockley module for tool calls and output."},"timeout":{"type":"integer","description":"Max execution seconds. Defaults to config value (usually 30)."}},"required":["code"]}`),
		},
		{
			Name:        "_code_guidelines",
			Description: "Get usage guidelines and the list of tools available from code. Call this before writing non-trivial code.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		},
	}
}

// IsCodeExecTool returns true if the tool name is a code execution tool.
func IsCodeExecTool(name string) bool {
	return name == "_code_execute" || name == "_code_guidelines"
}

func taskToolDefinitions() []model.LLMToolDefinition {
	return []model.LLMToolDefinition{
		{
			Name:        "_task_create",
			Description: "Create a new task to track progress toward the goal. Use this to break down complex work into manageable pieces.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"description":{"type":"string","description":"What needs to be done"},"priority":{"type":"string","enum":["high","medium","low"],"default":"medium","description":"Task priority level"}},"required":["description"]}`),
		},
		{
			Name:        "_task_update",
			Description: "Update the status of an existing task.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","description":"Task ID to update"},"status":{"type":"string","enum":["pending","in_progress","completed"],"description":"New task status"}},"required":["id","status"]}`),
		},
		{
			Name:        "_task_list",
			Description: "List all tracked tasks and their current status.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		},
	}
}

func memoryToolDefinitions() []model.LLMToolDefinition {
	return []model.LLMToolDefinition{
		{
			Name:        "_memory_store",
			Description: "Store a piece of information in shared memory for later recall or use by other agents.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"key":{"type":"string","description":"Unique key for this memory entry"},"content":{"type":"string","description":"The content to store"},"tags":{"type":"array","items":{"type":"string"},"description":"Optional tags for categorization"}},"required":["key","content"]}`),
		},
		{
			Name:        "_memory_recall",
			Description: "Search shared memory by content substring and optional tags.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"Substring to search for in memory content"},"tags":{"type":"array","items":{"type":"string"},"description":"Optional tags to filter by"}},"required":["query"]}`),
		},
		{
			Name:        "_memory_list",
			Description: "List all entries in shared memory, optionally filtered by tags.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"tags":{"type":"array","items":{"type":"string"},"description":"Optional tags to filter by"}}}`),
		},
	}
}

func bufferToolDefinitions() []model.LLMToolDefinition {
	return []model.LLMToolDefinition{
		{
			Name:        "_buffer_create",
			Description: "Create a new named buffer for assembling structured output content.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Name for the buffer"}},"required":["name"]}`),
		},
		{
			Name:        "_buffer_append",
			Description: "Append content to the end of a buffer.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Buffer name"},"content":{"type":"string","description":"Content to append"}},"required":["name","content"]}`),
		},
		{
			Name:        "_buffer_prepend",
			Description: "Prepend content to the beginning of a buffer.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Buffer name"},"content":{"type":"string","description":"Content to prepend"}},"required":["name","content"]}`),
		},
		{
			Name:        "_buffer_insert",
			Description: "Insert content after the first occurrence of a marker string in a buffer.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Buffer name"},"after":{"type":"string","description":"Marker string to insert after"},"content":{"type":"string","description":"Content to insert"}},"required":["name","after","content"]}`),
		},
		{
			Name:        "_buffer_replace",
			Description: "Replace occurrences of a string in a buffer. Set count to 0 to replace all occurrences.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Buffer name"},"old":{"type":"string","description":"String to find"},"new":{"type":"string","description":"Replacement string"},"count":{"type":"integer","description":"Number of replacements (0 = all)","default":0}},"required":["name","old","new"]}`),
		},
		{
			Name:        "_buffer_delete",
			Description: "Delete a character range [start, end) from a buffer.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Buffer name"},"start":{"type":"integer","description":"Start character index (inclusive)"},"end":{"type":"integer","description":"End character index (exclusive)"}},"required":["name","start","end"]}`),
		},
		{
			Name:        "_buffer_read",
			Description: "Read the content of a buffer, optionally a specific character range.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Buffer name"},"start":{"type":"integer","description":"Start character index (optional)"},"end":{"type":"integer","description":"End character index (optional)"}},"required":["name"]}`),
		},
		{
			Name:        "_buffer_length",
			Description: "Get the character count of a buffer's content.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Buffer name"}},"required":["name"]}`),
		},
		{
			Name:        "_buffer_finalize",
			Description: "Finalize a buffer and map it to an output port. After finalization, the buffer becomes immutable.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Buffer name"},"output_port":{"type":"string","description":"Output port name to map the buffer content to"}},"required":["name","output_port"]}`),
		},
	}
}

// --- Dispatch Router ---

// BuiltInToolDispatcher routes built-in tool calls to the appropriate subsystem.
type BuiltInToolDispatcher struct {
	Tasks   *TaskTracker
	Buffers *BufferManager
	Memory  *MemoryStore
}

// NewBuiltInToolDispatcher creates a new dispatcher with the given subsystems.
func NewBuiltInToolDispatcher(tasks *TaskTracker, buffers *BufferManager, memory *MemoryStore) *BuiltInToolDispatcher {
	return &BuiltInToolDispatcher{
		Tasks:   tasks,
		Buffers: buffers,
		Memory:  memory,
	}
}

// IsBuiltIn returns true if the tool name is a built-in tool.
func (d *BuiltInToolDispatcher) IsBuiltIn(name string) bool {
	return strings.HasPrefix(name, "_task_") ||
		strings.HasPrefix(name, "_buffer_") ||
		strings.HasPrefix(name, "_memory_")
}

// Dispatch routes a built-in tool call to the appropriate handler.
func (d *BuiltInToolDispatcher) Dispatch(name string, args json.RawMessage) (string, error) {
	var params map[string]any
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("invalid arguments for %s: %w", name, err)
		}
	}
	if params == nil {
		params = make(map[string]any)
	}

	switch name {
	// Task tools
	case "_task_create":
		return d.handleTaskCreate(params)
	case "_task_update":
		return d.handleTaskUpdate(params)
	case "_task_list":
		return d.handleTaskList()

	// Buffer tools
	case "_buffer_create":
		return d.handleBufferCreate(params)
	case "_buffer_append":
		return d.handleBufferAppend(params)
	case "_buffer_prepend":
		return d.handleBufferPrepend(params)
	case "_buffer_insert":
		return d.handleBufferInsert(params)
	case "_buffer_replace":
		return d.handleBufferReplace(params)
	case "_buffer_delete":
		return d.handleBufferDelete(params)
	case "_buffer_read":
		return d.handleBufferRead(params)
	case "_buffer_length":
		return d.handleBufferLength(params)
	case "_buffer_finalize":
		return d.handleBufferFinalize(params)

	// Memory tools
	case "_memory_store":
		return d.handleMemoryStore(params)
	case "_memory_recall":
		return d.handleMemoryRecall(params)
	case "_memory_list":
		return d.handleMemoryList(params)

	default:
		return "", fmt.Errorf("unknown built-in tool: %s", name)
	}
}

// AsInterceptor returns a ToolInterceptor that routes built-in tools through Dispatch.
func (d *BuiltInToolDispatcher) AsInterceptor() ToolInterceptor {
	return func(toolName string, args json.RawMessage) (string, bool) {
		if !d.IsBuiltIn(toolName) {
			return "", false
		}
		result, err := d.Dispatch(toolName, args)
		if err != nil {
			errResult, _ := json.Marshal(map[string]string{"error": err.Error()})
			return string(errResult), true
		}
		return result, true
	}
}

// --- Dispatch Handlers ---

func (d *BuiltInToolDispatcher) handleTaskCreate(params map[string]any) (string, error) {
	desc, _ := params["description"].(string)
	if desc == "" {
		return "", fmt.Errorf("_task_create: description is required")
	}
	priority, _ := params["priority"].(string)
	task := d.Tasks.Create(desc, TaskPriority(priority))
	return jsonResult(task)
}

func (d *BuiltInToolDispatcher) handleTaskUpdate(params map[string]any) (string, error) {
	id, _ := params["id"].(string)
	if id == "" {
		return "", fmt.Errorf("_task_update: id is required")
	}
	status, _ := params["status"].(string)
	if status == "" {
		return "", fmt.Errorf("_task_update: status is required")
	}
	if err := d.Tasks.Update(id, TaskStatus(status)); err != nil {
		return "", err
	}
	return jsonResult(map[string]string{"status": "updated"})
}

func (d *BuiltInToolDispatcher) handleTaskList() (string, error) {
	return jsonResult(d.Tasks.List())
}

func (d *BuiltInToolDispatcher) handleBufferCreate(params map[string]any) (string, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return "", fmt.Errorf("_buffer_create: name is required")
	}
	if err := d.Buffers.Create(name); err != nil {
		return "", err
	}
	return jsonResult(map[string]string{"status": "created", "name": name})
}

func (d *BuiltInToolDispatcher) handleBufferAppend(params map[string]any) (string, error) {
	name, _ := params["name"].(string)
	content, _ := params["content"].(string)
	if err := d.Buffers.Append(name, content); err != nil {
		return "", err
	}
	return jsonResult(map[string]string{"status": "appended"})
}

func (d *BuiltInToolDispatcher) handleBufferPrepend(params map[string]any) (string, error) {
	name, _ := params["name"].(string)
	content, _ := params["content"].(string)
	if err := d.Buffers.Prepend(name, content); err != nil {
		return "", err
	}
	return jsonResult(map[string]string{"status": "prepended"})
}

func (d *BuiltInToolDispatcher) handleBufferInsert(params map[string]any) (string, error) {
	name, _ := params["name"].(string)
	after, _ := params["after"].(string)
	content, _ := params["content"].(string)
	if err := d.Buffers.Insert(name, after, content); err != nil {
		return "", err
	}
	return jsonResult(map[string]string{"status": "inserted"})
}

func (d *BuiltInToolDispatcher) handleBufferReplace(params map[string]any) (string, error) {
	name, _ := params["name"].(string)
	old, _ := params["old"].(string)
	newStr, _ := params["new"].(string)
	count := 0
	if c, ok := params["count"].(float64); ok {
		count = int(c)
	}
	if err := d.Buffers.Replace(name, old, newStr, count); err != nil {
		return "", err
	}
	return jsonResult(map[string]string{"status": "replaced"})
}

func (d *BuiltInToolDispatcher) handleBufferDelete(params map[string]any) (string, error) {
	name, _ := params["name"].(string)
	start, _ := params["start"].(float64)
	end, _ := params["end"].(float64)
	if err := d.Buffers.Delete(name, int(start), int(end)); err != nil {
		return "", err
	}
	return jsonResult(map[string]string{"status": "deleted"})
}

func (d *BuiltInToolDispatcher) handleBufferRead(params map[string]any) (string, error) {
	name, _ := params["name"].(string)
	var startPtr, endPtr *int
	if s, ok := params["start"].(float64); ok {
		si := int(s)
		startPtr = &si
	}
	if e, ok := params["end"].(float64); ok {
		ei := int(e)
		endPtr = &ei
	}
	content, err := d.Buffers.Read(name, startPtr, endPtr)
	if err != nil {
		return "", err
	}
	return jsonResult(map[string]string{"content": content})
}

func (d *BuiltInToolDispatcher) handleBufferLength(params map[string]any) (string, error) {
	name, _ := params["name"].(string)
	length, err := d.Buffers.Length(name)
	if err != nil {
		return "", err
	}
	return jsonResult(map[string]int{"length": length})
}

func (d *BuiltInToolDispatcher) handleBufferFinalize(params map[string]any) (string, error) {
	name, _ := params["name"].(string)
	outputPort, _ := params["output_port"].(string)
	if err := d.Buffers.Finalize(name, outputPort); err != nil {
		return "", err
	}
	return jsonResult(map[string]string{"status": "finalized", "output_port": outputPort})
}

func (d *BuiltInToolDispatcher) handleMemoryStore(params map[string]any) (string, error) {
	key, _ := params["key"].(string)
	if key == "" {
		return "", fmt.Errorf("_memory_store: key is required")
	}
	content, _ := params["content"].(string)
	if content == "" {
		return "", fmt.Errorf("_memory_store: content is required")
	}
	var tags []string
	if rawTags, ok := params["tags"].([]any); ok {
		for _, t := range rawTags {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
	}
	d.Memory.Store(key, content, tags)
	return jsonResult(map[string]string{"status": "stored"})
}

func (d *BuiltInToolDispatcher) handleMemoryRecall(params map[string]any) (string, error) {
	query, _ := params["query"].(string)
	var tags []string
	if rawTags, ok := params["tags"].([]any); ok {
		for _, t := range rawTags {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
	}
	entries := d.Memory.Recall(query, tags)
	return jsonResult(entries)
}

func (d *BuiltInToolDispatcher) handleMemoryList(params map[string]any) (string, error) {
	var tags []string
	if rawTags, ok := params["tags"].([]any); ok {
		for _, t := range rawTags {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
	}
	entries := d.Memory.List(tags)
	return jsonResult(entries)
}

// jsonResult marshals a value to a JSON string.
func jsonResult(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshaling result: %w", err)
	}
	return string(b), nil
}

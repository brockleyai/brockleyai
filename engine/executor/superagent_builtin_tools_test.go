package executor

import (
	"encoding/json"
	"testing"

	"github.com/brockleyai/brockleyai/internal/model"
)

// --- Task Tracker Tests ---

func TestTaskTracker_Create(t *testing.T) {
	tracker := NewTaskTracker()
	task := tracker.Create("Write tests", TaskPriorityHigh)

	if task.ID != "1" {
		t.Errorf("expected ID=1, got %s", task.ID)
	}
	if task.Status != TaskStatusPending {
		t.Errorf("expected status=pending, got %s", task.Status)
	}
	if task.Priority != TaskPriorityHigh {
		t.Errorf("expected priority=high, got %s", task.Priority)
	}
	if task.Description != "Write tests" {
		t.Errorf("expected description='Write tests', got %s", task.Description)
	}

	// Second task should have ID=2
	task2 := tracker.Create("Review code", TaskPriorityLow)
	if task2.ID != "2" {
		t.Errorf("expected ID=2, got %s", task2.ID)
	}
}

func TestTaskTracker_Create_DefaultPriority(t *testing.T) {
	tracker := NewTaskTracker()
	task := tracker.Create("Some task", "")

	if task.Priority != TaskPriorityMedium {
		t.Errorf("expected default priority=medium, got %s", task.Priority)
	}
}

func TestTaskTracker_Update(t *testing.T) {
	tracker := NewTaskTracker()
	tracker.Create("Task 1", TaskPriorityMedium)

	if err := tracker.Update("1", TaskStatusInProgress); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tasks := tracker.List()
	if tasks[0].Status != TaskStatusInProgress {
		t.Errorf("expected status=in_progress, got %s", tasks[0].Status)
	}

	if err := tracker.Update("1", TaskStatusCompleted); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tasks = tracker.List()
	if tasks[0].Status != TaskStatusCompleted {
		t.Errorf("expected status=completed, got %s", tasks[0].Status)
	}
}

func TestTaskTracker_Update_NotFound(t *testing.T) {
	tracker := NewTaskTracker()
	err := tracker.Update("999", TaskStatusCompleted)
	if err == nil {
		t.Fatal("expected error for non-existent task")
	}
}

func TestTaskTracker_List(t *testing.T) {
	tracker := NewTaskTracker()
	tracker.Create("Task 1", TaskPriorityHigh)
	tracker.Create("Task 2", TaskPriorityLow)
	tracker.Create("Task 3", TaskPriorityMedium)

	tasks := tracker.List()
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	// Verify list returns a copy (mutating doesn't affect original)
	tasks[0].Description = "mutated"
	original := tracker.List()
	if original[0].Description == "mutated" {
		t.Error("List should return a copy, not a reference")
	}
}

func TestTaskTracker_BuildTaskReminder(t *testing.T) {
	tracker := NewTaskTracker()
	tracker.Create("Write code", TaskPriorityHigh)
	tracker.Create("Write tests", TaskPriorityMedium)

	reminder := tracker.BuildTaskReminder()
	expected := "[Tasks]\n- [pending] #1: Write code (high)\n- [pending] #2: Write tests (medium)"
	if reminder != expected {
		t.Errorf("unexpected reminder:\ngot:  %q\nwant: %q", reminder, expected)
	}
}

func TestTaskTracker_BuildTaskReminder_Empty(t *testing.T) {
	tracker := NewTaskTracker()
	if reminder := tracker.BuildTaskReminder(); reminder != "" {
		t.Errorf("expected empty string, got %q", reminder)
	}
}

// --- Buffer Manager Tests ---

func TestBufferManager_CreateAndAppend(t *testing.T) {
	bm := NewBufferManager()
	if err := bm.Create("main"); err != nil {
		t.Fatal(err)
	}
	if err := bm.Append("main", "hello "); err != nil {
		t.Fatal(err)
	}
	if err := bm.Append("main", "world"); err != nil {
		t.Fatal(err)
	}
	content, err := bm.Read("main", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if content != "hello world" {
		t.Errorf("expected 'hello world', got %q", content)
	}
}

func TestBufferManager_Prepend(t *testing.T) {
	bm := NewBufferManager()
	if err := bm.Create("buf"); err != nil {
		t.Fatal(err)
	}
	if err := bm.Append("buf", "world"); err != nil {
		t.Fatal(err)
	}
	if err := bm.Prepend("buf", "hello "); err != nil {
		t.Fatal(err)
	}
	content, err := bm.Read("buf", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if content != "hello world" {
		t.Errorf("expected 'hello world', got %q", content)
	}
}

func TestBufferManager_Insert(t *testing.T) {
	bm := NewBufferManager()
	if err := bm.Create("buf"); err != nil {
		t.Fatal(err)
	}
	if err := bm.Append("buf", "hello world"); err != nil {
		t.Fatal(err)
	}
	if err := bm.Insert("buf", "hello", " beautiful"); err != nil {
		t.Fatal(err)
	}
	content, err := bm.Read("buf", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if content != "hello beautiful world" {
		t.Errorf("expected 'hello beautiful world', got %q", content)
	}
}

func TestBufferManager_Insert_MarkerNotFound(t *testing.T) {
	bm := NewBufferManager()
	if err := bm.Create("buf"); err != nil {
		t.Fatal(err)
	}
	if err := bm.Append("buf", "hello world"); err != nil {
		t.Fatal(err)
	}
	err := bm.Insert("buf", "notfound", "content")
	if err == nil {
		t.Fatal("expected error for missing marker")
	}
}

func TestBufferManager_Replace(t *testing.T) {
	bm := NewBufferManager()
	if err := bm.Create("buf"); err != nil {
		t.Fatal(err)
	}
	if err := bm.Append("buf", "foo bar foo baz foo"); err != nil {
		t.Fatal(err)
	}

	// Replace all
	if err := bm.Replace("buf", "foo", "qux", 0); err != nil {
		t.Fatal(err)
	}
	content, _ := bm.Read("buf", nil, nil)
	if content != "qux bar qux baz qux" {
		t.Errorf("replace all: expected 'qux bar qux baz qux', got %q", content)
	}

	// Replace with count=1
	bm2 := NewBufferManager()
	if err := bm2.Create("buf"); err != nil {
		t.Fatal(err)
	}
	if err := bm2.Append("buf", "aaa bbb aaa"); err != nil {
		t.Fatal(err)
	}
	if err := bm2.Replace("buf", "aaa", "ccc", 1); err != nil {
		t.Fatal(err)
	}
	content2, _ := bm2.Read("buf", nil, nil)
	if content2 != "ccc bbb aaa" {
		t.Errorf("replace count=1: expected 'ccc bbb aaa', got %q", content2)
	}
}

func TestBufferManager_Delete(t *testing.T) {
	bm := NewBufferManager()
	if err := bm.Create("buf"); err != nil {
		t.Fatal(err)
	}
	if err := bm.Append("buf", "hello world"); err != nil {
		t.Fatal(err)
	}
	// Delete " world" (chars 5-11)
	if err := bm.Delete("buf", 5, 11); err != nil {
		t.Fatal(err)
	}
	content, _ := bm.Read("buf", nil, nil)
	if content != "hello" {
		t.Errorf("expected 'hello', got %q", content)
	}
}

func TestBufferManager_Delete_OutOfRange(t *testing.T) {
	bm := NewBufferManager()
	if err := bm.Create("buf"); err != nil {
		t.Fatal(err)
	}
	if err := bm.Append("buf", "hi"); err != nil {
		t.Fatal(err)
	}
	err := bm.Delete("buf", 0, 10)
	if err == nil {
		t.Fatal("expected error for out-of-range delete")
	}
}

func TestBufferManager_Read_Partial(t *testing.T) {
	bm := NewBufferManager()
	if err := bm.Create("buf"); err != nil {
		t.Fatal(err)
	}
	if err := bm.Append("buf", "hello world"); err != nil {
		t.Fatal(err)
	}
	start := 6
	end := 11
	content, err := bm.Read("buf", &start, &end)
	if err != nil {
		t.Fatal(err)
	}
	if content != "world" {
		t.Errorf("expected 'world', got %q", content)
	}
}

func TestBufferManager_Length(t *testing.T) {
	bm := NewBufferManager()
	if err := bm.Create("buf"); err != nil {
		t.Fatal(err)
	}
	if err := bm.Append("buf", "hello"); err != nil {
		t.Fatal(err)
	}
	length, err := bm.Length("buf")
	if err != nil {
		t.Fatal(err)
	}
	if length != 5 {
		t.Errorf("expected length=5, got %d", length)
	}
}

func TestBufferManager_Finalize(t *testing.T) {
	bm := NewBufferManager()
	if err := bm.Create("output"); err != nil {
		t.Fatal(err)
	}
	if err := bm.Append("output", "final content"); err != nil {
		t.Fatal(err)
	}
	if err := bm.Finalize("output", "result"); err != nil {
		t.Fatal(err)
	}

	// Writes should be rejected after finalization
	if err := bm.Append("output", "more"); err == nil {
		t.Error("expected error when appending to finalized buffer")
	}
	if err := bm.Prepend("output", "more"); err == nil {
		t.Error("expected error when prepending to finalized buffer")
	}
}

func TestBufferManager_Finalize_AlreadyFinalized(t *testing.T) {
	bm := NewBufferManager()
	if err := bm.Create("buf"); err != nil {
		t.Fatal(err)
	}
	if err := bm.Finalize("buf", "port1"); err != nil {
		t.Fatal(err)
	}
	err := bm.Finalize("buf", "port2")
	if err == nil {
		t.Fatal("expected error for double finalization")
	}
}

func TestBufferManager_GetFinalizedBuffers(t *testing.T) {
	bm := NewBufferManager()
	if err := bm.Create("a"); err != nil {
		t.Fatal(err)
	}
	if err := bm.Append("a", "content a"); err != nil {
		t.Fatal(err)
	}
	if err := bm.Create("b"); err != nil {
		t.Fatal(err)
	}
	if err := bm.Append("b", "content b"); err != nil {
		t.Fatal(err)
	}
	if err := bm.Create("c"); err != nil {
		t.Fatal(err)
	}
	if err := bm.Append("c", "not finalized"); err != nil {
		t.Fatal(err)
	}
	if err := bm.Finalize("a", "port_a"); err != nil {
		t.Fatal(err)
	}
	if err := bm.Finalize("b", "port_b"); err != nil {
		t.Fatal(err)
	}

	finalized := bm.GetFinalizedBuffers()
	if len(finalized) != 2 {
		t.Fatalf("expected 2 finalized buffers, got %d", len(finalized))
	}
	if finalized["port_a"] != "content a" {
		t.Errorf("port_a: expected 'content a', got %q", finalized["port_a"])
	}
	if finalized["port_b"] != "content b" {
		t.Errorf("port_b: expected 'content b', got %q", finalized["port_b"])
	}
}

func TestBufferManager_NotFound(t *testing.T) {
	bm := NewBufferManager()
	if err := bm.Append("nope", "x"); err == nil {
		t.Error("expected error for missing buffer on Append")
	}
	if err := bm.Prepend("nope", "x"); err == nil {
		t.Error("expected error for missing buffer on Prepend")
	}
	if _, err := bm.Read("nope", nil, nil); err == nil {
		t.Error("expected error for missing buffer on Read")
	}
	if _, err := bm.Length("nope"); err == nil {
		t.Error("expected error for missing buffer on Length")
	}
	if err := bm.Finalize("nope", "p"); err == nil {
		t.Error("expected error for missing buffer on Finalize")
	}
}

func TestBufferManager_Create_Duplicate(t *testing.T) {
	bm := NewBufferManager()
	if err := bm.Create("buf"); err != nil {
		t.Fatal(err)
	}
	err := bm.Create("buf")
	if err == nil {
		t.Fatal("expected error for duplicate buffer name")
	}
}

// --- Memory Store Tests ---

func TestMemoryStore_StoreAndList(t *testing.T) {
	ms := NewMemoryStore("agent1")
	ms.Store("key1", "value1", nil)
	ms.Store("key2", "value2", []string{"tag1"})

	entries := ms.List(nil)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestMemoryStore_Store_Overwrite(t *testing.T) {
	ms := NewMemoryStore("ns")
	ms.Store("k", "old", nil)
	ms.Store("k", "new", []string{"updated"})

	entries := ms.List(nil)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after overwrite, got %d", len(entries))
	}
	if entries[0].Content != "new" {
		t.Errorf("expected content='new', got %q", entries[0].Content)
	}
}

func TestMemoryStore_Recall_BySubstring(t *testing.T) {
	ms := NewMemoryStore("ns")
	ms.Store("a", "the quick brown fox", nil)
	ms.Store("b", "lazy dog", nil)

	results := ms.Recall("quick", nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Key != "ns/a" {
		t.Errorf("expected key 'ns/a', got %q", results[0].Key)
	}
}

func TestMemoryStore_Recall_ByTags(t *testing.T) {
	ms := NewMemoryStore("ns")
	ms.Store("a", "content a", []string{"tag1", "tag2"})
	ms.Store("b", "content b", []string{"tag3"})

	// Both have "content" substring; filter by tag1
	results := ms.Recall("content", []string{"tag1"})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestMemoryStore_Recall_Combined(t *testing.T) {
	ms := NewMemoryStore("ns")
	ms.Store("a", "alpha data", []string{"important"})
	ms.Store("b", "beta data", []string{"important"})
	ms.Store("c", "alpha info", []string{"trivial"})

	// Substring "alpha" AND tag "important"
	results := ms.Recall("alpha", []string{"important"})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Key != "ns/a" {
		t.Errorf("expected key 'ns/a', got %q", results[0].Key)
	}
}

func TestMemoryStore_Recall_NoMatch(t *testing.T) {
	ms := NewMemoryStore("ns")
	ms.Store("a", "hello", nil)

	results := ms.Recall("nonexistent", nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestMemoryStore_List_ByTags(t *testing.T) {
	ms := NewMemoryStore("ns")
	ms.Store("a", "content a", []string{"tag1"})
	ms.Store("b", "content b", []string{"tag2"})
	ms.Store("c", "content c", []string{"tag1", "tag2"})

	results := ms.List([]string{"tag1"})
	if len(results) != 2 {
		t.Fatalf("expected 2 entries with tag1, got %d", len(results))
	}
}

func TestMemoryStore_Namespace(t *testing.T) {
	ms := NewMemoryStore("agent1")
	ms.Store("key", "agent1 data", nil)

	entries := ms.List(nil)
	if entries[0].Key != "agent1/key" {
		t.Errorf("expected namespaced key 'agent1/key', got %q", entries[0].Key)
	}
	if entries[0].Namespace != "agent1" {
		t.Errorf("expected namespace 'agent1', got %q", entries[0].Namespace)
	}

	// Recall searches all entries regardless of namespace
	results := ms.Recall("agent1", nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestMemoryStore_LoadFromState(t *testing.T) {
	state := map[string]any{
		"ns/key1": map[string]any{
			"key":       "ns/key1",
			"content":   "value1",
			"tags":      []any{"t1"},
			"namespace": "ns",
		},
		"ns/key2": map[string]any{
			"key":       "ns/key2",
			"content":   "value2",
			"namespace": "ns",
		},
	}

	ms := NewMemoryStore("ns")
	ms.LoadFromState(state)

	entries := ms.List(nil)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries after load, got %d", len(entries))
	}
}

func TestMemoryStore_ToStateValue(t *testing.T) {
	ms := NewMemoryStore("ns")
	ms.Store("a", "value a", []string{"tag1"})
	ms.Store("b", "value b", nil)

	stateVal := ms.ToStateValue()
	if len(stateVal) != 2 {
		t.Fatalf("expected 2 entries in state, got %d", len(stateVal))
	}
	if _, ok := stateVal["ns/a"]; !ok {
		t.Error("expected key 'ns/a' in state")
	}
	if _, ok := stateVal["ns/b"]; !ok {
		t.Error("expected key 'ns/b' in state")
	}
}

// --- Dispatch Router Tests ---

func TestDispatcher_TaskCreate(t *testing.T) {
	d := NewBuiltInToolDispatcher(NewTaskTracker(), NewBufferManager(), NewMemoryStore("test"))
	args, _ := json.Marshal(map[string]any{"description": "do something", "priority": "high"})
	result, err := d.Dispatch("_task_create", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var task Task
	if err := json.Unmarshal([]byte(result), &task); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if task.ID != "1" {
		t.Errorf("expected ID=1, got %s", task.ID)
	}
	if task.Priority != TaskPriorityHigh {
		t.Errorf("expected priority=high, got %s", task.Priority)
	}
}

func TestDispatcher_BufferCreate(t *testing.T) {
	d := NewBuiltInToolDispatcher(NewTaskTracker(), NewBufferManager(), NewMemoryStore("test"))
	args, _ := json.Marshal(map[string]any{"name": "output"})
	result, err := d.Dispatch("_buffer_create", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var res map[string]string
	if err := json.Unmarshal([]byte(result), &res); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if res["status"] != "created" {
		t.Errorf("expected status=created, got %s", res["status"])
	}
}

func TestDispatcher_MemoryStore(t *testing.T) {
	d := NewBuiltInToolDispatcher(NewTaskTracker(), NewBufferManager(), NewMemoryStore("test"))
	args, _ := json.Marshal(map[string]any{"key": "fact", "content": "Go is great", "tags": []string{"lang"}})
	result, err := d.Dispatch("_memory_store", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var res map[string]string
	if err := json.Unmarshal([]byte(result), &res); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if res["status"] != "stored" {
		t.Errorf("expected status=stored, got %s", res["status"])
	}

	// Verify it was actually stored
	entries := d.Memory.List(nil)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestDispatcher_UnknownTool(t *testing.T) {
	d := NewBuiltInToolDispatcher(NewTaskTracker(), NewBufferManager(), NewMemoryStore("test"))
	_, err := d.Dispatch("_unknown_tool", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestDispatcher_IsBuiltIn(t *testing.T) {
	d := NewBuiltInToolDispatcher(NewTaskTracker(), NewBufferManager(), NewMemoryStore("test"))

	if !d.IsBuiltIn("_task_create") {
		t.Error("expected _task_create to be built-in")
	}
	if !d.IsBuiltIn("_buffer_read") {
		t.Error("expected _buffer_read to be built-in")
	}
	if !d.IsBuiltIn("_memory_store") {
		t.Error("expected _memory_store to be built-in")
	}
	if d.IsBuiltIn("web_search") {
		t.Error("expected web_search to NOT be built-in")
	}
	if d.IsBuiltIn("task_create") {
		t.Error("expected task_create (no underscore) to NOT be built-in")
	}
}

func TestDispatcher_AsInterceptor(t *testing.T) {
	d := NewBuiltInToolDispatcher(NewTaskTracker(), NewBufferManager(), NewMemoryStore("test"))
	interceptor := d.AsInterceptor()

	// Built-in tool should be handled
	args, _ := json.Marshal(map[string]any{"description": "test task"})
	result, handled := interceptor("_task_create", args)
	if !handled {
		t.Error("expected interceptor to handle _task_create")
	}
	if result == "" {
		t.Error("expected non-empty result for handled tool")
	}

	// Non-built-in should pass through
	result, handled = interceptor("web_search", json.RawMessage(`{}`))
	if handled {
		t.Error("expected interceptor to pass through web_search")
	}
	if result != "" {
		t.Error("expected empty result for non-built-in tool")
	}

	// Error case: built-in but returns error should still be handled
	badArgs, _ := json.Marshal(map[string]any{})
	result, handled = interceptor("_task_create", badArgs)
	if !handled {
		t.Error("expected interceptor to handle _task_create even with error")
	}
	// Should contain error JSON
	var errResult map[string]string
	if err := json.Unmarshal([]byte(result), &errResult); err != nil {
		t.Fatalf("expected JSON error result, got %q", result)
	}
	if errResult["error"] == "" {
		t.Error("expected non-empty error message")
	}
}

// --- Tool Definition Tests ---

func TestGetBuiltInToolDefinitions_AllEnabled(t *testing.T) {
	cfg := &model.SuperagentNodeConfig{
		SharedMemory: &model.SharedMemoryConfig{Enabled: true},
	}
	tools := GetBuiltInToolDefinitions(cfg)
	// 3 task + 3 memory + 9 buffer = 15
	if len(tools) != 15 {
		t.Errorf("expected 15 tools with all enabled, got %d", len(tools))
		for _, tool := range tools {
			t.Logf("  - %s", tool.Name)
		}
	}
}

func TestGetBuiltInToolDefinitions_NoMemory(t *testing.T) {
	cfg := &model.SuperagentNodeConfig{}
	tools := GetBuiltInToolDefinitions(cfg)
	// 3 task + 9 buffer = 12
	if len(tools) != 12 {
		t.Errorf("expected 12 tools without memory, got %d", len(tools))
	}

	// Verify no memory tools
	for _, tool := range tools {
		if tool.Name == "_memory_store" || tool.Name == "_memory_recall" || tool.Name == "_memory_list" {
			t.Errorf("unexpected memory tool %q when shared memory disabled", tool.Name)
		}
	}
}

func TestGetBuiltInToolDefinitions_TasksDisabled(t *testing.T) {
	disabled := false
	cfg := &model.SuperagentNodeConfig{
		SharedMemory: &model.SharedMemoryConfig{Enabled: true},
		Overrides: &model.SuperagentOverrides{
			TaskTracking: &model.TaskTrackingOverride{
				Enabled: &disabled,
			},
		},
	}
	tools := GetBuiltInToolDefinitions(cfg)
	// 3 memory + 9 buffer = 12
	if len(tools) != 12 {
		t.Errorf("expected 12 tools with tasks disabled, got %d", len(tools))
	}

	// Verify no task tools
	for _, tool := range tools {
		if tool.Name == "_task_create" || tool.Name == "_task_update" || tool.Name == "_task_list" {
			t.Errorf("unexpected task tool %q when tasks disabled", tool.Name)
		}
	}
}

func TestGetBuiltInToolDefinitions_ValidJSON(t *testing.T) {
	cfg := &model.SuperagentNodeConfig{
		SharedMemory: &model.SharedMemoryConfig{Enabled: true},
	}
	tools := GetBuiltInToolDefinitions(cfg)

	for _, tool := range tools {
		if tool.Name == "" {
			t.Error("tool has empty name")
		}
		if tool.Description == "" {
			t.Errorf("tool %q has empty description", tool.Name)
		}
		var schema map[string]any
		if err := json.Unmarshal(tool.Parameters, &schema); err != nil {
			t.Errorf("tool %q has invalid JSON parameters: %v", tool.Name, err)
		}
		if schema["type"] != "object" {
			t.Errorf("tool %q parameters should have type=object", tool.Name)
		}
	}
}

func TestGetBuiltInToolDefinitions_CodeExecution(t *testing.T) {
	cfg := &model.SuperagentNodeConfig{
		CodeExecution: &model.CodeExecutionConfig{Enabled: true},
	}
	tools := GetBuiltInToolDefinitions(cfg)
	// 3 task + 9 buffer + 2 code = 14
	if len(tools) != 14 {
		t.Errorf("expected 14 tools with code execution, got %d", len(tools))
		for _, tool := range tools {
			t.Logf("  - %s", tool.Name)
		}
	}

	// Verify code tools present.
	hasExecute := false
	hasGuidelines := false
	for _, tool := range tools {
		if tool.Name == "_code_execute" {
			hasExecute = true
		}
		if tool.Name == "_code_guidelines" {
			hasGuidelines = true
		}
	}
	if !hasExecute {
		t.Error("expected _code_execute tool")
	}
	if !hasGuidelines {
		t.Error("expected _code_guidelines tool")
	}
}

func TestGetBuiltInToolDefinitions_CodeExecutionDisabled(t *testing.T) {
	cfg := &model.SuperagentNodeConfig{
		CodeExecution: &model.CodeExecutionConfig{Enabled: false},
	}
	tools := GetBuiltInToolDefinitions(cfg)
	// 3 task + 9 buffer = 12
	if len(tools) != 12 {
		t.Errorf("expected 12 tools with code execution disabled, got %d", len(tools))
	}
	for _, tool := range tools {
		if tool.Name == "_code_execute" || tool.Name == "_code_guidelines" {
			t.Errorf("unexpected code tool %q when code execution disabled", tool.Name)
		}
	}
}

func TestGetBuiltInToolDefinitions_CodeExecutionNil(t *testing.T) {
	cfg := &model.SuperagentNodeConfig{}
	tools := GetBuiltInToolDefinitions(cfg)
	for _, tool := range tools {
		if tool.Name == "_code_execute" || tool.Name == "_code_guidelines" {
			t.Errorf("unexpected code tool %q when code execution nil", tool.Name)
		}
	}
}

func TestIsCodeExecTool(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"_code_execute", true},
		{"_code_guidelines", true},
		{"_task_create", false},
		{"_buffer_create", false},
		{"echo", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCodeExecTool(tt.name); got != tt.want {
				t.Errorf("IsCodeExecTool(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

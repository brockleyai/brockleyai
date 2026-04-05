package worker

import (
	"encoding/json"
	"testing"

	"github.com/brockleyai/brockleyai/engine/codeexec"
)

func TestCodeExecKeyBuilders(t *testing.T) {
	tests := []struct {
		name     string
		fn       func(string, string, int64) string
		execID   string
		nodeID   string
		seq      int64
		expected string
	}{
		{
			name:     "callback key",
			fn:       CodeExecCallbackKey,
			execID:   "exec-1",
			nodeID:   "node-1",
			seq:      42,
			expected: "codeexec:exec-1:node-1:42:callbacks",
		},
		{
			name:     "response key",
			fn:       CodeExecResponseKey,
			execID:   "exec-1",
			nodeID:   "node-1",
			seq:      42,
			expected: "codeexec:exec-1:node-1:42:responses",
		},
		{
			name:     "result key",
			fn:       CodeExecResultKey,
			execID:   "exec-1",
			nodeID:   "node-1",
			seq:      42,
			expected: "codeexec:exec-1:node-1:42:result",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn(tt.execID, tt.nodeID, tt.seq)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestCodeExecKeyBuilders_UniquePerSeq(t *testing.T) {
	k1 := CodeExecCallbackKey("exec-1", "node-1", 1)
	k2 := CodeExecCallbackKey("exec-1", "node-1", 2)
	if k1 == k2 {
		t.Errorf("expected unique keys, both got %q", k1)
	}
}

func TestCodeExecTask_Serialization(t *testing.T) {
	task := CodeExecTask{
		ExecutionID:         "exec-1",
		NodeID:              "node-1",
		Seq:                 1,
		Code:                "print('hello')",
		MaxExecutionTimeSec: 30,
		MaxMemoryMB:         256,
		MaxOutputBytes:      1048576,
		MaxCodeBytes:        65536,
		MaxToolCalls:        50,
		AllowedModules:      []string{"json", "math"},
		CallbackKey:         "codeexec:exec-1:node-1:1:callbacks",
		ResponseKey:         "codeexec:exec-1:node-1:1:responses",
		ResultKey:           "codeexec:exec-1:node-1:1:result",
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded CodeExecTask
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Code != task.Code {
		t.Errorf("expected code %q, got %q", task.Code, decoded.Code)
	}
	if decoded.MaxExecutionTimeSec != 30 {
		t.Errorf("expected max_execution_time_sec 30, got %d", decoded.MaxExecutionTimeSec)
	}
	if decoded.CallbackKey != task.CallbackKey {
		t.Errorf("expected callback_key %q, got %q", task.CallbackKey, decoded.CallbackKey)
	}
	if len(decoded.AllowedModules) != 2 || decoded.AllowedModules[0] != "json" {
		t.Errorf("expected allowed_modules [json, math], got %v", decoded.AllowedModules)
	}
}

func TestCodeExecResult_Serialization(t *testing.T) {
	result := CodeExecResult{
		Status:     "completed",
		Output:     `{"result": 42}`,
		Stdout:     "debug output",
		Stderr:     "",
		Error:      "",
		Traceback:  "",
		ToolCalls:  3,
		DurationMs: 150,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded CodeExecResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Status != "completed" {
		t.Errorf("expected status completed, got %s", decoded.Status)
	}
	if decoded.Output != `{"result": 42}` {
		t.Errorf("expected output %q, got %q", `{"result": 42}`, decoded.Output)
	}
	if decoded.ToolCalls != 3 {
		t.Errorf("expected tool_calls 3, got %d", decoded.ToolCalls)
	}
}

func TestCodeToolRequest_Serialization(t *testing.T) {
	req := CodeToolRequest{
		Type:      "tool_call",
		Name:      "echo",
		Arguments: map[string]any{"message": "hello"},
		Seq:       1,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded CodeToolRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Name != "echo" {
		t.Errorf("expected name echo, got %s", decoded.Name)
	}
	if decoded.Arguments["message"] != "hello" {
		t.Errorf("expected arguments.message hello, got %v", decoded.Arguments["message"])
	}
}

func TestCodeToolResponse_Serialization(t *testing.T) {
	resp := CodeToolResponse{
		Type:    "result",
		Content: "tool output",
		IsError: false,
		Seq:     1,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded CodeToolResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Type != "result" {
		t.Errorf("expected type result, got %s", decoded.Type)
	}
	if decoded.Content != "tool output" {
		t.Errorf("expected content 'tool output', got %q", decoded.Content)
	}
}

func TestFormatCodeExecResult_Completed(t *testing.T) {
	result := codeexec.FormatCodeExecResult("completed", `{"answer": 42}`, "debug", "", "", "", 2, 150)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if parsed["status"] != "completed" {
		t.Errorf("expected status completed, got %v", parsed["status"])
	}
	if parsed["output"] != `{"answer": 42}` {
		t.Errorf("expected output, got %v", parsed["output"])
	}
	if parsed["stdout"] != "debug" {
		t.Errorf("expected stdout debug, got %v", parsed["stdout"])
	}
}

func TestFormatCodeExecResult_StdoutTruncation(t *testing.T) {
	// Create stdout larger than 8KB
	bigStdout := make([]byte, 10000)
	for i := range bigStdout {
		bigStdout[i] = 'x'
	}

	result := codeexec.FormatCodeExecResult("completed", "", string(bigStdout), "", "", "", 0, 100)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	stdout, ok := parsed["stdout"].(string)
	if !ok {
		t.Fatal("expected stdout to be string")
	}
	if len(stdout) > 8192 {
		t.Errorf("stdout should be truncated to 8192, got %d", len(stdout))
	}

	note, ok := parsed["stdout_note"].(string)
	if !ok || note == "" {
		t.Error("expected stdout_note for truncated output")
	}
}

func TestFormatCodeExecResult_Error(t *testing.T) {
	result := codeexec.FormatCodeExecResult("error", "", "", "", "Traceback (most recent call last):\n  ...", "NameError: name 'x' is not defined", 0, 50)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed["status"] != "error" {
		t.Errorf("expected status error, got %v", parsed["status"])
	}
	if parsed["traceback"] == nil || parsed["traceback"] == "" {
		t.Error("expected traceback to be present")
	}
	if parsed["error"] == nil || parsed["error"] == "" {
		t.Error("expected error to be present")
	}
}

func TestFormatCodeExecResult_Timeout(t *testing.T) {
	result := codeexec.FormatCodeExecResult("timeout", "", "", "", "", "execution timed out", 0, 30000)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed["status"] != "timeout" {
		t.Errorf("expected status timeout, got %v", parsed["status"])
	}
}

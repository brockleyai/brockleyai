package codeexec

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGuidelines_ReturnsContent(t *testing.T) {
	g := Guidelines()
	if g == "" {
		t.Fatal("expected non-empty guidelines")
	}
	if !strings.Contains(g, "brockley.output") {
		t.Error("guidelines should mention brockley.output")
	}
	if !strings.Contains(g, "_code_execute") {
		t.Error("guidelines should mention _code_execute")
	}
}

func TestGuidelinesWithTools_NoTools(t *testing.T) {
	g := GuidelinesWithTools("base guidelines", nil)
	if !strings.Contains(g, "No tools are available") {
		t.Error("expected 'No tools are available' when no tools")
	}
}

func TestGuidelinesWithTools_WithTools(t *testing.T) {
	tools := []string{"echo", "word_count", "lookup"}
	g := GuidelinesWithTools("base", tools)
	for _, tool := range tools {
		if !strings.Contains(g, tool) {
			t.Errorf("expected tool %q in guidelines", tool)
		}
	}
}

func TestIntOrDefault(t *testing.T) {
	v := 42
	if got := IntOrDefault(&v, 10); got != 42 {
		t.Errorf("expected 42, got %d", got)
	}
	if got := IntOrDefault(nil, 10); got != 10 {
		t.Errorf("expected 10, got %d", got)
	}
}

func TestDefaultLimits(t *testing.T) {
	limits := DefaultLimits()
	if limits["max_execution_time_sec"] != 30 {
		t.Errorf("expected 30, got %d", limits["max_execution_time_sec"])
	}
	if limits["max_memory_mb"] != 256 {
		t.Errorf("expected 256, got %d", limits["max_memory_mb"])
	}
}

func TestFormatCodeExecResult_EmptyFields(t *testing.T) {
	result := FormatCodeExecResult("completed", "", "", "", "", "", 0, 100)
	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed["status"] != "completed" {
		t.Errorf("expected status completed, got %v", parsed["status"])
	}
	// stdout/stderr should not be present when empty.
	if _, ok := parsed["stdout"]; ok {
		t.Error("expected stdout to be omitted when empty")
	}
}

func TestEmbeddedAssets(t *testing.T) {
	// Verify all expected files exist.
	for _, name := range []string{"assets/run.py", "assets/brockley.py", "assets/guidelines.md"} {
		data, err := Assets.ReadFile(name)
		if err != nil {
			t.Errorf("failed to read embedded %s: %v", name, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("embedded %s is empty", name)
		}
	}
}

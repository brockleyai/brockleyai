package codeexec

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Guidelines returns the static guidelines content for the _code_guidelines tool.
func Guidelines() string {
	data, err := Assets.ReadFile("assets/guidelines.md")
	if err != nil {
		return "Code execution guidelines are not available."
	}
	return string(data)
}

// GuidelinesWithTools returns guidelines plus the dynamic list of tools
// available from code. Tools with require_approval policy are excluded.
func GuidelinesWithTools(guidelines string, availableTools []string) string {
	if len(availableTools) == 0 {
		return guidelines + "\n\n## Available Tools\n\nNo tools are available from code."
	}

	var sb strings.Builder
	sb.WriteString(guidelines)
	sb.WriteString("\n\n## Available Tools\n\nThe following tools can be called from code via `brockley.tools.call(name, **kwargs)`:\n")
	for _, t := range availableTools {
		fmt.Fprintf(&sb, "- `%s`\n", t)
	}
	return sb.String()
}

// DefaultLimits returns default values for code execution limits.
func DefaultLimits() map[string]int {
	return map[string]int{
		"max_execution_time_sec":       30,
		"max_memory_mb":                256,
		"max_output_bytes":             1048576,
		"max_code_bytes":               65536,
		"max_tool_calls_per_execution": 50,
		"max_executions_per_run":       20,
	}
}

// IntOrDefault returns the value pointed to by p, or the default if p is nil.
func IntOrDefault(p *int, def int) int {
	if p != nil {
		return *p
	}
	return def
}

// FormatCodeExecResult shapes a CodeExecResult for the LLM context window.
// It applies truncation limits to stdout/stderr while preserving structured output and tracebacks.
func FormatCodeExecResult(status, output, stdout, stderr, traceback, errMsg string, toolCalls int, durationMs int64) string {
	const maxStdout = 8192
	const maxStderr = 4096

	result := map[string]any{
		"status":      status,
		"duration_ms": durationMs,
	}

	if output != "" {
		result["output"] = output
	}
	if errMsg != "" {
		result["error"] = errMsg
	}
	if traceback != "" {
		result["traceback"] = traceback
	}
	if toolCalls > 0 {
		result["tool_calls"] = toolCalls
	}

	if stdout != "" {
		if len(stdout) > maxStdout {
			result["stdout"] = stdout[:maxStdout]
			result["stdout_note"] = fmt.Sprintf("[stdout truncated, %d bytes total — use brockley.output() for structured results]", len(stdout))
		} else {
			result["stdout"] = stdout
		}
	}
	if stderr != "" {
		if len(stderr) > maxStderr {
			result["stderr"] = stderr[:maxStderr]
			result["stderr_note"] = fmt.Sprintf("[stderr truncated, %d bytes total]", len(stderr))
		} else {
			result["stderr"] = stderr
		}
	}

	b, _ := json.Marshal(result)
	return string(b)
}

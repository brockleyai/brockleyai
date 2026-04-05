package executor

import (
	"fmt"
	"strings"

	"github.com/brockleyai/brockleyai/engine/expression"
	"github.com/brockleyai/brockleyai/internal/model"
)

// Default prompt templates.
const (
	defaultSuperagentPreamble = `You are an autonomous agent executing a task. You have access to tools and should use them to complete your work systematically.

Guidelines:
- Break complex work into tasks using _task_create
- Update task status as you work using _task_update
- Think step by step before acting
- When done, ensure all tasks are marked completed`

	defaultBuiltInToolGuide = `## Built-In Tools

### Task Management
- _task_create: Create a tracked task. Use to plan and organize your work.
- _task_update: Update task status (pending -> in_progress -> completed).
- _task_list: View all tasks and their current status.

### Output Assembly (Buffers)
Use buffers to build large outputs incrementally:
- _buffer_create: Create a named buffer.
- _buffer_append/prepend: Add content to buffer.
- _buffer_insert: Insert after a marker string.
- _buffer_replace: Find and replace in buffer.
- _buffer_delete: Delete a character range.
- _buffer_read: Read buffer contents.
- _buffer_length: Get buffer size.
- _buffer_finalize: Map buffer to an output port (required to use buffer content as output).`

	defaultMemoryToolGuide = `### Shared Memory
- _memory_store: Save a finding or fact for other agents to access.
- _memory_recall: Search shared memory by content substring or tags.
- _memory_list: List all shared memory entries, optionally filtered by tags.
Use memory to persist important findings that should survive context compaction or be shared with other superagent nodes.`

	defaultCodeExecutionGuide = `### Code Execution (Python)
- _code_guidelines: Get usage guidelines and available tools list. Call this before writing non-trivial code.
- _code_execute: Execute Python 3 code with access to the brockley module for tool calls and structured output.
Use code execution for data transformation, computation, assembling large outputs, or batching multiple tool calls efficiently. The code runs in an isolated environment with Python stdlib access.`

	// Used in P5 - define here so all prompts are in one file.
	DefaultEvaluatorPrompt = `You are evaluating whether an autonomous agent has completed its task.

Task: {{task}}

Output Requirements:
{{output_requirements}}

Current Task List:
{{task_list}}

Working Memory:
- Iteration: {{iteration}}
- Plan: {{plan}}

Analyze the conversation and determine:
1. Has the agent completed all required work?
2. Is the agent stuck in a loop?
3. Is the context getting too large and needs compaction?

Respond with JSON only:
{"needs_more_work": bool, "stuck_detected": bool, "should_compact": bool, "reasoning": "brief explanation"}`

	DefaultReflectionPrompt = `You are reflecting on an autonomous agent's progress. The agent appears to be stuck or making insufficient progress.

Task: {{task}}

Current Task List:
{{task_list}}

Working Memory:
- Plan: {{plan}}
- Iteration: {{iteration}}

Analyze what went wrong and propose a new approach.

Respond with JSON only:
{"reflection_text": "what went wrong and why", "new_plan": "revised step-by-step plan"}`

	DefaultCompactionPrompt = `Summarize the following conversation history, preserving:
1. Key facts and decisions made
2. Important tool results and findings
3. Current task status and progress
4. Errors encountered and lessons learned
5. Context needed to continue the work

Be concise but preserve all actionable information.`

	DefaultMemoryFlushPrompt = `Review the conversation and extract key facts, findings, and decisions that should be preserved in long-term memory. For each fact, provide a key, content, and relevant tags.

Respond with JSON array:
[{"key": "descriptive-key", "content": "the fact or finding", "tags": ["relevant", "tags"]}]`

	DefaultExtractionPrompt = `Extract structured output from the conversation to match the required schema.

Output Schema:
{{schema}}

Based on the conversation, produce a JSON object matching the schema above. Use only information from the conversation.`
)

// AssembleSystemPrompt builds the full system prompt for a superagent execution.
func AssembleSystemPrompt(
	cfg *model.SuperagentNodeConfig,
	inputs map[string]any,
	sharedMemory []MemoryEntry,
	workingMemory map[string]any,
	iteration int,
	outputPorts []model.Port,
) (string, error) {
	var sections []string

	// 1. System preamble (T0 identity).
	preamble := defaultSuperagentPreamble
	if cfg.SystemPreamble != "" {
		preamble = cfg.SystemPreamble
	}
	sections = append(sections, preamble)

	// 2. Task prompt (rendered with input variables).
	exprCtx := &expression.Context{Input: inputs}
	rendered, err := expression.RenderTemplate(cfg.Prompt, exprCtx)
	if err != nil {
		return "", fmt.Errorf("rendering prompt template: %w", err)
	}
	sections = append(sections, "## Task\n\n"+rendered)

	// 3. Shared memory context (T1).
	if len(sharedMemory) > 0 {
		var memLines []string
		memLines = append(memLines, "## Shared Memory (from previous agents)")
		for _, entry := range sharedMemory {
			tagStr := ""
			if len(entry.Tags) > 0 {
				tagStr = " [" + strings.Join(entry.Tags, ", ") + "]"
			}
			memLines = append(memLines, fmt.Sprintf("- **%s**%s: %s", entry.Key, tagStr, entry.Content))
		}
		sections = append(sections, strings.Join(memLines, "\n"))
	}

	// 4. Skill descriptions.
	if len(cfg.Skills) > 0 {
		var skillLines []string
		skillLines = append(skillLines, "## Available Skills")
		for _, skill := range cfg.Skills {
			line := fmt.Sprintf("- **%s**: %s", skill.Name, skill.Description)
			skillLines = append(skillLines, line)
			if skill.PromptFragment != "" {
				skillLines = append(skillLines, "  "+skill.PromptFragment)
			}
		}
		sections = append(sections, strings.Join(skillLines, "\n"))
	}

	// 5. Built-in tool guide.
	guide := defaultBuiltInToolGuide
	if cfg.SharedMemory != nil && cfg.SharedMemory.Enabled {
		guide += "\n\n" + defaultMemoryToolGuide
	}
	if cfg.CodeExecution != nil && cfg.CodeExecution.Enabled {
		guide += "\n\n" + defaultCodeExecutionGuide
	}
	sections = append(sections, guide)

	// 6. Tool conventions/style (from overrides).
	if cfg.Overrides != nil && cfg.Overrides.PromptAssembly != nil {
		if cfg.Overrides.PromptAssembly.ToolConventions != "" {
			sections = append(sections, "## Tool Conventions\n\n"+cfg.Overrides.PromptAssembly.ToolConventions)
		}
		if cfg.Overrides.PromptAssembly.Style != "" {
			sections = append(sections, "## Style Guidelines\n\n"+cfg.Overrides.PromptAssembly.Style)
		}
	}

	// 7. Output requirements.
	if len(outputPorts) > 0 {
		var outLines []string
		outLines = append(outLines, "## Output Requirements")
		outLines = append(outLines, "You must produce the following outputs:")
		for _, port := range outputPorts {
			if strings.HasPrefix(port.Name, "_") {
				continue // skip meta outputs
			}
			schemaStr := string(port.Schema)
			outLines = append(outLines, fmt.Sprintf("- **%s** (schema: %s)", port.Name, schemaStr))
		}
		outLines = append(outLines, "\nUse _buffer_create and _buffer_finalize to map large outputs to ports, or provide them naturally in your final response.")
		sections = append(sections, strings.Join(outLines, "\n"))
	}

	// 8. Working memory state.
	if workingMemory != nil && iteration > 0 {
		var wmLines []string
		wmLines = append(wmLines, "## Current State")
		wmLines = append(wmLines, fmt.Sprintf("- Iteration: %d", iteration))
		if plan, ok := workingMemory["plan"].(string); ok && plan != "" {
			wmLines = append(wmLines, fmt.Sprintf("- Current Plan: %s", plan))
		}
		sections = append(sections, strings.Join(wmLines, "\n"))
	}

	return strings.Join(sections, "\n\n"), nil
}

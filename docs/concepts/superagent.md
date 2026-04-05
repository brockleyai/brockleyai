# Superagent

A Superagent is an autonomous agent loop packaged as a first-class Brockley node type. You define what the agent should do (a task prompt), what tools it has access to (MCP skills), and what outputs you expect -- the superagent handles the rest: planning, tool calling, progress tracking, self-evaluation, reflection when stuck, context management, and structured output assembly.

## Why a Node Type?

The superagent is a single node, not a graph template or subgraph. This means:

- **Composable** -- drop a superagent into any graph alongside other node types (LLM, transform, conditional, foreach).
- **Shared memory** -- multiple superagents in a pipeline share persistent facts via graph state.
- **Observable** -- 10 event types stream progress in real-time (started, iteration, evaluation, reflection, stuck warning, compaction, memory store, buffer finalize, tool call, completed).
- **Bounded** -- five-layer termination guarantees the agent always stops.

## What You Define

A superagent node has four main inputs:

1. **Prompt** -- the task, with `{{input.*}}` template variables for dynamic values.
2. **Skills** -- MCP server connections that provide tools.
3. **Output ports** -- the structured outputs you expect from the agent.
4. **Limits and overrides** (optional) -- tune iterations, timeouts, evaluation strategy, and more.

## How It Works

### The Agent Loop

```
┌─────────────────────────────────────────────────────────┐
│ Outer Loop (up to max_iterations)                       │
│                                                         │
│  1. Assemble system prompt (from config + state)        │
│  2. ┌─ Inner Tool Loop ──────────────────────────────┐  │
│     │  LLM call → tool calls → results → LLM call…  │  │
│     │  (built-in tools handled locally)              │  │
│     └────────────────────────────────────────────────┘  │
│  3. Stuck detection (repeated tool call patterns)       │
│  4. Evaluation (separate LLM: done? stuck? compact?)    │
│  5. If stuck → Reflection (LLM produces new plan)       │
│  6. If context too large → Compaction (summarize)       │
│                                                         │
│  → Loop continues or exits based on evaluation          │
└─────────────────────────────────────────────────────────┘
```

### Prompt Assembly

The system prompt is automatically composed from multiple sections:

1. **System preamble** -- persona, tone, guardrails (from `system_preamble`)
2. **Task** -- the rendered prompt with input values
3. **Shared memory** -- prior facts from other nodes (if enabled)
4. **Skill descriptions** -- what each MCP server provides, with prompt fragments
5. **Built-in tool guide** -- how to use task tracking, memory, and buffer tools
6. **Output requirements** -- what output ports exist and their schemas
7. **Working memory** -- current plan, observations, iteration count

After context compaction, the system prompt is reassembled fresh from config -- ensuring instructions are never lost.

### Evaluation

After each iteration, a separate LLM call (the evaluator) assesses the agent's progress:

- **Done** -- all tasks complete, outputs ready. Agent exits.
- **Needs more work** -- continue to next iteration.
- **Stuck** -- trigger reflection.
- **Should compact** -- context is getting large, trigger compaction.

The evaluator receives the task list, working memory, and conversation history. Pending tasks mean work is not done.

You can disable evaluation (`overrides.evaluator.disabled: true`) to run the agent for exactly `max_iterations` iterations.

### Reflection

When the evaluator detects the agent is stuck (or stuck detection triggers), a reflection LLM:

1. Analyzes what went wrong (repeated actions, wrong assumptions)
2. Produces a `new_plan` with alternative approaches
3. The plan is injected into the conversation as a system message
4. The stuck detection window resets

After `max_reflections` (default: 3) attempts, the agent force-exits with `_finish_reason: "stuck"`.

### Context Compaction

When the conversation history grows too large (exceeds `compaction_threshold` of `context_window_limit`):

1. **Memory flush** -- LLM extracts key facts and stores them via `_memory_store` (preserving knowledge)
2. **Summarization** -- LLM condenses the conversation into a summary
3. **Reconstruction** -- fresh system prompt + summary + last N messages

This is analogous to how Claude Code reloads CLAUDE.md after context compaction.

### Five-Layer Termination

The agent is guaranteed to stop. No matter what the LLM decides, one of these limits will trigger. They are checked in order from innermost to outermost:

| Layer | Limit | Default | What Happens | Finish Reason |
|-------|-------|---------|-------------|---------------|
| 1 | `max_tool_calls_per_iteration` | 25 | The inner tool loop ends for this iteration. The agent proceeds to evaluation. | (continues to evaluation) |
| 2 | `max_total_tool_calls` | 200 | The agent stops immediately. All tool calls across all iterations are counted. | `max_tool_calls` |
| 3 | `max_iterations` | 25 | The outer loop stops after this many iterations. Each iteration includes one inner tool loop plus evaluation. | `max_iterations` |
| 4 | `timeout_seconds` | 600 | Wall-clock timeout. Checked between steps. If the agent has been running longer than this, it exits. | `timeout` |
| 5 | Stuck escalation | 3 reflections | After the evaluator triggers `max_reflections` reflection attempts, the agent gives up. | `stuck` |

Layer 1 is a soft limit -- it just ends the current iteration's tool loop and proceeds to evaluation. Layers 2-5 are hard exits. The `_finish_reason` meta output tells you which layer triggered.

## Memory Tiers

The superagent uses three tiers of memory:

- **Working memory (T0)** -- plan, observations, iteration count. Lives in the conversation. Lost on compaction (but plan is regenerated via reflection).
- **Shared memory (T1)** -- persistent facts stored via `_memory_store`. Survives compaction (auto-flushed before summarization). Shared across nodes via graph state.
- **Buffers (T2)** -- structured output content built incrementally. Persists across iterations. Mapped to output ports via `_buffer_finalize`.

## Code Execution

When `code_execution.enabled: true`, the agent gains two additional built-in tools: `_code_execute` and `_code_guidelines`. The agent can write and run Python code during its task -- useful for data processing, calculations, text manipulation, and any work that is more reliable or efficient in code than through LLM reasoning.

Code runs in a sandboxed Python subprocess via a separate coderunner process. The sandbox enforces time limits, memory limits, and an allowlist of Python modules (json, math, re, datetime, collections, itertools, and more). There is no network access from user code -- the only way to reach external systems is through tool calls.

If the Python code needs to call tools (e.g., MCP tools), it uses a provided `call_tool()` function. These calls are relayed back to the superagent coordinator, which dispatches them as normal MCP tasks. Results are returned to the Python process.

See [Superagent Built-In Tools: Code Execution](superagent-tools.md#code-execution) for the full tool reference, configuration options, and security details.

## Distributed Execution

In production, the superagent runs as a distributed coordinator:

1. The orchestrator dispatches a `node:superagent` asynq task.
2. The `SuperagentHandler` stays alive and coordinates the agent loop.
3. Each LLM call is dispatched as a separate `node:llm-call` task.
4. Each MCP tool call is dispatched as a separate `node:mcp-call` task.
5. Built-in tools (`_task_*`, `_buffer_*`, `_memory_*`) are handled locally by the coordinator.

This ensures all network I/O is distributed across workers.

## See Also

- [Superagent Node Reference](../nodes/superagent.md) -- full configuration reference
- [Built-In Tools](superagent-tools.md) -- task tracking, memory, buffer, and code execution tools
- [State](state.md#state-in-superagent-nodes) -- shared memory via graph state
- [Build Your First Agent](../guides/superagent-tutorial.md) -- step-by-step tutorial
- [Customizing the Superagent](../guides/superagent-advanced.md) -- overrides and advanced configuration
- [Multi-Agent Patterns](../guides/superagent-patterns.md) -- pipeline, fan-out, and outer loop patterns
- [Execution](execution.md) -- how superagent tasks are distributed across workers

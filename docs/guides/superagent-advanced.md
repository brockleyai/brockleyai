# Customizing the Superagent

The superagent's internal components (evaluator, reflection, compaction, stuck detection, prompt assembly, output extraction, task tracking) are all configurable via the `overrides` field. This guide covers common customization patterns.

## Multi-Model Configurations

Use a fast, cheap model for the main agent loop and a stronger model for evaluation:

```json
{
  "config": {
    "provider": "openrouter",
    "model": "anthropic/claude-sonnet-4-6",
    "api_key_ref": "openrouter_key",
    "overrides": {
      "evaluator": {
        "provider": "anthropic",
        "model": "claude-opus-4-6",
        "api_key_ref": "anthropic_key"
      }
    }
  }
}
```

Common patterns:
- **Main loop:** fast model (Sonnet, GPT-4o, Gemini Flash)
- **Evaluator:** strong model (Opus, GPT-4o) for accurate completion assessment
- **Reflection:** strong model for better strategy revision
- **Compaction:** fast model (summarization doesn't need top-tier reasoning)

When an override specifies a different `provider` than the main node, include `api_key` or `api_key_ref` on the override.

## Disabling the Evaluator

For simple tasks where you know the agent should run for a fixed number of iterations:

```json
{
  "config": {
    "max_iterations": 3,
    "overrides": {
      "evaluator": {"disabled": true}
    }
  }
}
```

The agent runs exactly `max_iterations` iterations, then stops with `_finish_reason: "max_iterations"`. This is useful for one-shot tasks or when you want predictable execution time.

## Custom Evaluation Prompt

Override the evaluator prompt to change how completion is assessed:

```json
{
  "overrides": {
    "evaluator": {
      "prompt": "You are evaluating a code review agent. Consider it done ONLY if: (1) all files have been reviewed, (2) at least one comment was left per file, (3) a summary was written. Be strict."
    }
  }
}
```

## Tuning Stuck Detection

Adjust when the agent is considered stuck:

```json
{
  "overrides": {
    "stuck_detection": {
      "window_size": 30,
      "repeat_threshold": 4
    }
  }
}
```

- **`window_size`** -- how many recent tool calls to track (default: 20). Larger values are more tolerant of repeated patterns.
- **`repeat_threshold`** -- how many identical calls trigger stuck (default: 3). Higher values give the agent more chances.

To disable stuck detection entirely:

```json
{
  "overrides": {
    "stuck_detection": {"enabled": false}
  }
}
```

## Context Compaction Settings

Control when and how context compaction happens:

```json
{
  "overrides": {
    "context_compaction": {
      "context_window_limit": 200000,
      "compaction_threshold": 0.8,
      "preserve_recent_messages": 10
    }
  }
}
```

- **`context_window_limit`** -- token budget (default: 128000). Set this to match your model's context window.
- **`compaction_threshold`** -- fraction that triggers compaction (default: 0.75). At 0.8, compaction triggers when the conversation reaches 80% of the limit.
- **`preserve_recent_messages`** -- messages kept after compaction (default: 5). Higher values preserve more recent context.

## Tool Policies

Control which tools the agent can use:

### Denylist (block specific tools)

```json
{
  "config": {
    "tool_policies": {
      "denied": ["dangerous_tool", "expensive_api_call"]
    }
  }
}
```

### Allowlist (only these tools)

```json
{
  "config": {
    "tool_policies": {
      "allowed": ["search", "read_file", "list_directory"]
    }
  }
}
```

### Require Approval

Tools in `require_approval` are excluded from routing. The agent sees them in tool descriptions but cannot call them directly -- it must ask for permission in its text response:

```json
{
  "config": {
    "tool_policies": {
      "require_approval": ["kubectl_apply", "deploy_service"]
    }
  }
}
```

## Custom Prompt Assembly

Add tool usage conventions or writing style instructions:

```json
{
  "overrides": {
    "prompt_assembly": {
      "tool_conventions": "Always check metrics before and after making changes. Never run destructive operations without confirmation.",
      "style": "Be concise. Use bullet points for findings. Include confidence levels."
    }
  }
}
```

For full control over the prompt template:

```json
{
  "overrides": {
    "prompt_assembly": {
      "template": "You are {{system_preamble}}.\n\nTask: {{task}}\n\nTools: {{skills}}\n\nRules: {{tool_conventions}}"
    }
  }
}
```

## Conversation History (Multi-Turn)

Enable multi-turn conversations by passing prior history via an input port:

```json
{
  "input_ports": [
    {"name": "query", "schema": {"type": "string"}},
    {"name": "history", "schema": {"type": "array"}}
  ],
  "config": {
    "prompt": "Answer the user's question: {{input.query}}",
    "conversation_history_from_input": "history"
  }
}
```

The `history` input port receives an array of message objects from a previous execution's `_conversation_history` meta output. The agent resumes the conversation from where it left off.

## Reflection Configuration

Control how the agent recovers from being stuck:

```json
{
  "overrides": {
    "reflection": {
      "max_reflections": 5,
      "prompt": "Analyze what went wrong. The agent has been repeating the same actions. Identify: (1) what assumption is wrong, (2) what alternative approach to try, (3) a concrete new plan."
    }
  }
}
```

To disable reflection (agent will force-exit when stuck instead of trying to recover):

```json
{
  "overrides": {
    "reflection": {"disabled": true}
  }
}
```

## Task Tracking Configuration

Adjust task reminder frequency or disable task tracking:

```json
{
  "overrides": {
    "task_tracking": {
      "reminder_frequency": 3
    }
  }
}
```

This injects task reminders every 3 tool calls instead of every call. For short tasks where reminders add noise:

```json
{
  "overrides": {
    "task_tracking": {"enabled": false}
  }
}
```

## See Also

- [Multi-Agent Patterns](superagent-patterns.md) -- pipeline, fan-out, and outer loop patterns
- [Superagent Node Reference](../nodes/superagent.md) -- complete configuration reference
- [Superagent Tutorial](superagent-tutorial.md) -- getting started with superagents
- [Data Model](../specs/data-model.md) -- SuperagentOverrides field definitions

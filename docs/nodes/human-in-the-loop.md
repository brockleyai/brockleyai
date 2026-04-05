# Human-in-the-Loop Node

**Type:** `human_in_the_loop`

**Status: Not yet implemented.** The executor is registered but returns an error at runtime. Full implementation requires an external input mechanism (webhook callback, UI integration, or API polling) planned for a future release.

The human-in-the-loop (HITL) node pauses graph execution and waits for a human decision before continuing. It is designed for workflows that require human approval, review, or manual input at specific points.

## Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `prompt_text` | string | Yes | Text displayed to the human reviewer describing what action is needed. |
| `timeout_seconds` | integer | No | Maximum time (in seconds) to wait for a human response. If omitted, waits indefinitely (subject to the graph-level execution timeout). |
| `allowed_actions` | string[] | No | Valid action names the human can take (e.g., `["approve", "reject", "escalate"]`). Defaults to `["approve", "reject"]`. |

## Current Behavior

When the engine encounters a HITL node, it returns an error:

```
human-in-the-loop execution not yet implemented -- requires external input mechanism
```

You can include HITL nodes in graph definitions for planning and validation purposes, but executing a graph that reaches a HITL node will fail.

## Planned Behavior

When fully implemented, the HITL node will:

1. Pause the execution at the HITL node.
2. Emit an event with the `prompt_text` and `allowed_actions`.
3. Transition the execution status to a waiting/paused state.
4. An external system (UI, webhook, API) submits the human's response.
5. The execution resumes with the human's response on the output ports.
6. If `timeout_seconds` is exceeded, the node fails with a timeout error.

## Example Configuration

```json
{
  "id": "approval",
  "name": "Manager Approval",
  "type": "human_in_the_loop",
  "input_ports": [
    {"name": "proposal", "schema": {"type": "object"}}
  ],
  "output_ports": [
    {"name": "decision", "schema": {"type": "string"}},
    {"name": "comments", "schema": {"type": "string"}}
  ],
  "config": {
    "prompt_text": "Please review this proposal and approve or reject it.",
    "timeout_seconds": 86400,
    "allowed_actions": ["approve", "reject", "request_changes"]
  }
}
```

## Use Cases

- **Content moderation** -- pause before publishing AI-generated content for human review.
- **Financial approvals** -- require sign-off before processing transactions above a threshold.
- **Escalation handling** -- route complex support cases to a human when AI confidence is low.
- **Quality assurance** -- sample and review a percentage of automated decisions.

## See Also

- [Conditional Node](conditional.md) -- route based on automated conditions
- [Superagent Node](superagent.md) -- `tool_policies.require_approval` for tool-level human oversight
- [Data Model: HITL Node Config](../specs/data-model.md) -- complete field reference

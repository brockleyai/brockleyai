# Conditional Node

**Type:** `conditional`

The conditional node routes data down different paths based on [expression language](../expressions/overview.md) conditions. It evaluates branches in order and fires the first matching branch's output port. If no branch matches, the default port fires.

## Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `branches` | Branch[] | Yes | Ordered list of branches. Each has a `label` and a `condition` expression. |
| `default_label` | string | Yes | Output port name used when no branch condition matches. |

### Branch

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `label` | string | Yes | The name of the output port that fires if this condition is true. |
| `condition` | string | Yes | An expression evaluated against the node's inputs. Must return a truthy or falsy result. |

## How It Works

1. The conditional node receives data on its `value` input port.
2. Each branch condition is evaluated in order. The expression context provides `input.value` (the received data), `state.*` (graph state fields), and `meta.*` (execution metadata).
3. The first branch whose condition evaluates to a truthy value wins. The input value is emitted on that branch's output port.
4. If no branch matches, the input value is emitted on the `default_label` output port.
5. Only one output port fires per execution. All other output ports produce no value.

Since branches are evaluated in order, put more specific conditions first. For example, with thresholds: check `>= 0.8` before `>= 0.5` so that `0.9` matches the higher threshold.

## Truth Evaluation Rules

A condition result is **truthy** if it is:

| Value | Truthy? |
|-------|---------|
| `true` | Yes |
| Non-empty string (`"hello"`) | Yes |
| Non-zero number (`1`, `3.14`, `-1`) | Yes |
| Non-empty array (`[1, 2]`) | Yes |
| Any object, including empty (`{}`) | Yes |
| `false` | No |
| `null` | No |
| Empty string `""` | No |
| Zero (`0`, `0.0`) | No |
| Empty array `[]` | No |

These rules match the [expression language truthiness semantics](../expressions/operators.md#truthiness-rules).

## Input Ports

| Port | Type | Description |
|------|------|-------------|
| `value` | any | The data to route. Passed through to whichever branch matches. |

Additional input ports can be declared if state values or other data need to be available in conditions.

## Output Ports

Output ports are defined dynamically by the configuration. Each branch `label` and the `default_label` becomes an output port. The engine validates that the node's declared output ports match the branch labels.

## Skip Propagation

When a conditional node fires one branch, downstream nodes connected to other branches are **skipped**. The engine tracks which paths are active and does not execute nodes on inactive paths.

## Rejoining with Exclusive Fan-In

If branches converge back to a single downstream node, the engine uses **exclusive fan-in**: the downstream node executes once, receiving data from whichever branch was active. The node does not wait for all branches -- it proceeds as soon as the active branch delivers data.

## Examples

### Simple Type Routing

```json
{
  "config": {
    "branches": [
      {"label": "billing", "condition": "input.value.category == 'billing'"},
      {"label": "support", "condition": "input.value.category == 'support'"}
    ],
    "default_label": "other"
  }
}
```

### Numeric Thresholds (Order Matters)

```json
{
  "config": {
    "branches": [
      {"label": "high", "condition": "input.value.score >= 0.8"},
      {"label": "medium", "condition": "input.value.score >= 0.5"}
    ],
    "default_label": "low"
  }
}
```

Score of 0.9 matches `high`. Score of 0.6 matches `medium`. Score of 0.3 falls through to `low`.

### Complex Conditions with Logical Operators

```json
{
  "config": {
    "branches": [
      {
        "label": "urgent",
        "condition": "input.value.priority == 'high' && input.value.age_hours > 24"
      },
      {
        "label": "needs_review",
        "condition": "input.value.confidence < 0.7 || input.value.flagged == true"
      }
    ],
    "default_label": "auto_process"
  }
}
```

### Null/Existence Checks

```json
{
  "config": {
    "branches": [
      {"label": "has_context", "condition": "input.value.context != null"}
    ],
    "default_label": "no_context"
  }
}
```

### Array-Based Conditions

```json
{
  "config": {
    "branches": [
      {"label": "has_items", "condition": "input.value.items | length > 0"},
      {"label": "empty", "condition": "input.value.items | isEmpty"}
    ],
    "default_label": "unknown"
  }
}
```

### String Pattern Matching

```json
{
  "config": {
    "branches": [
      {"label": "email", "condition": "input.value.contact | contains('@')"},
      {"label": "phone", "condition": "input.value.contact | contains('+')"}
    ],
    "default_label": "other_contact"
  }
}
```

### Using Ternary and Null Coalescing

```json
{
  "config": {
    "branches": [
      {
        "label": "premium",
        "condition": "(input.value.tier ?? 'free') == 'premium' && input.value.active == true"
      }
    ],
    "default_label": "standard"
  }
}
```

### Using State in Conditions

The `state` namespace is available in condition expressions:

```json
{
  "config": {
    "branches": [
      {
        "label": "retry",
        "condition": "state.attempt_count < 3 && input.value.status == 'failed'"
      }
    ],
    "default_label": "give_up"
  }
}
```

You can also use `state_reads` to bind state fields to named input ports for clarity:

```json
{
  "state_reads": [
    {"state_field": "retry_count", "port": "retries"}
  ],
  "config": {
    "branches": [
      {"label": "retry", "condition": "input.retries < 3"}
    ],
    "default_label": "give_up"
  }
}
```

### Full Graph Example with Rejoin

```json
{
  "nodes": [
    {
      "id": "in", "type": "input",
      "output_ports": [{"name": "request", "schema": {"type": "object"}}],
      "config": {}
    },
    {
      "id": "route", "type": "conditional",
      "input_ports": [{"name": "value", "schema": {"type": "object"}}],
      "output_ports": [
        {"name": "simple", "schema": {"type": "object"}},
        {"name": "complex", "schema": {"type": "object"}}
      ],
      "config": {
        "branches": [{"label": "simple", "condition": "input.value.tokens < 100"}],
        "default_label": "complex"
      }
    },
    {
      "id": "fast-llm", "type": "llm",
      "input_ports": [{"name": "query", "schema": {"type": "object"}}],
      "output_ports": [{"name": "response_text", "schema": {"type": "string"}}],
      "config": {
        "provider": "openai", "model": "gpt-4o-mini",
        "api_key_ref": "openai-key",
        "user_prompt": "{{input.query}}",
        "variables": [{"name": "query", "schema": {"type": "object"}}],
        "response_format": "text"
      }
    },
    {
      "id": "power-llm", "type": "llm",
      "input_ports": [{"name": "query", "schema": {"type": "object"}}],
      "output_ports": [{"name": "response_text", "schema": {"type": "string"}}],
      "config": {
        "provider": "anthropic", "model": "claude-sonnet-4-20250514",
        "api_key_ref": "anthropic-key",
        "user_prompt": "{{input.query}}",
        "variables": [{"name": "query", "schema": {"type": "object"}}],
        "response_format": "text"
      }
    },
    {
      "id": "out", "type": "output",
      "input_ports": [{"name": "answer", "schema": {"type": "string"}}],
      "config": {}
    }
  ],
  "edges": [
    {"id": "e1", "source_node_id": "in", "source_port": "request", "target_node_id": "route", "target_port": "value"},
    {"id": "e2", "source_node_id": "route", "source_port": "simple", "target_node_id": "fast-llm", "target_port": "query"},
    {"id": "e3", "source_node_id": "route", "source_port": "complex", "target_node_id": "power-llm", "target_port": "query"},
    {"id": "e4", "source_node_id": "fast-llm", "source_port": "response_text", "target_node_id": "out", "target_port": "answer"},
    {"id": "e5", "source_node_id": "power-llm", "source_port": "response_text", "target_node_id": "out", "target_port": "answer"}
  ]
}
```

Both LLM nodes connect to the same output node. Only one path is active per execution, so the output node receives data from exactly one upstream node via exclusive fan-in.

## See Also

- [Branching and Joining](../concepts/branching.md) -- conditional routing patterns, skip propagation, exclusive fan-in
- [Expression Language Overview](../expressions/overview.md) -- full expression syntax
- [Operators Reference](../expressions/operators.md) -- comparison, logical, and null handling operators
- [Transform Node](transform.md) -- data transformation using expressions
- [ForEach Node](foreach.md) -- iteration over arrays
- [Data Model: Conditional Node Config](../specs/data-model.md) -- complete field reference
- [Graph Model](../specs/graph-model.md) -- skip propagation and exclusive fan-in details

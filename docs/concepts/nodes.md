# Nodes

Nodes are the building blocks of a graph. Each node represents a single step in the workflow: an LLM call, a data transformation, a conditional branch, or another operation.

## Node Structure

Every node has these fields:

```json
{
  "id": "transform-1",
  "name": "Format Response",
  "type": "transform",
  "input_ports": [
    {
      "name": "raw_text",
      "schema": {"type": "string"}
    }
  ],
  "output_ports": [
    {
      "name": "formatted",
      "schema": {"type": "string"}
    }
  ],
  "state_reads": [],
  "state_writes": [],
  "config": {
    "expressions": {
      "formatted": "input.raw_text.trim().upper()"
    }
  },
  "retry_policy": null,
  "timeout_seconds": null,
  "position": {"x": 400, "y": 200}
}
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique identifier within the graph |
| `name` | string | Human-readable label |
| `type` | string | Determines the node's behavior |
| `input_ports` | array | Typed inputs the node receives |
| `output_ports` | array | Typed outputs the node produces |
| `state_reads` | array | State fields read into input ports |
| `state_writes` | array | Output ports written to state fields |
| `config` | object | Type-specific configuration |
| `retry_policy` | object | Optional retry behavior |
| `timeout_seconds` | integer | Optional per-node timeout |
| `position` | object | X/Y coordinates for the visual editor |

## When to Use Which Node Type

| I need to... | Use this node |
|-------------|--------------|
| Accept external data into the graph | [`input`](#input) |
| Return results from the graph | [`output`](#output) |
| Call an LLM to generate text or structured JSON | [`llm`](#llm) |
| Call an external tool via MCP | [`tool`](#tool) |
| Route data down different paths based on conditions | [`conditional`](#conditional) |
| Reshape, combine, or compute data without an LLM | [`transform`](#transform) |
| Process each item in an array | [`foreach`](#foreach) |
| Embed a reusable workflow as a single node | [`subgraph`](#subgraph) |
| Run an autonomous agent loop (planning, tool use, self-evaluation) | [`superagent`](#superagent) |
| Pause for human review or approval | [`human_in_the_loop`](#human_in_the_loop) |

A common question: should I use a **transform** or an **LLM** node? Use transform for deterministic operations (string manipulation, math, restructuring JSON). Use an LLM when the task requires language understanding, generation, or judgment. Transforms are instant and free; LLM calls cost time and tokens.

Another common question: should I use a **subgraph** or a **foreach**? Use subgraph to encapsulate a reusable workflow that runs once. Use foreach when you need to run the same workflow once per item in an array. Both execute inner graphs with isolated state. See [Subgraphs](subgraphs.md) for more detail.

## Built-in Node Types

### input

> Full reference: [Input/Output Node](../nodes/input-output.md)

The entry point of a graph. Defines the external inputs the graph accepts.

- **Input ports**: none (it receives data from the caller)
- **Output ports**: one per input the graph accepts
- **Config**: none

Every graph must have at least one input node.

```json
{
  "id": "input-1",
  "name": "User Input",
  "type": "input",
  "input_ports": [],
  "output_ports": [
    {"name": "query", "schema": {"type": "string"}},
    {"name": "context", "schema": {"type": "string"}}
  ],
  "config": {}
}
```

### output

> Full reference: [Input/Output Node](../nodes/input-output.md)

The exit point of a graph. Defines the external outputs the graph produces.

- **Input ports**: one per output the graph produces
- **Output ports**: none
- **Config**: none

```json
{
  "id": "output-1",
  "name": "Result",
  "type": "output",
  "input_ports": [
    {"name": "answer", "schema": {"type": "string"}},
    {"name": "confidence", "schema": {"type": "number"}}
  ],
  "output_ports": [],
  "config": {}
}
```

### llm

> Full reference: [LLM Node](../nodes/llm.md)

Calls an LLM provider to generate text or structured JSON output.

- **Input ports**: auto-generated from template variables, plus any explicit ports
- **Output ports**: `response_text` (text mode) or `response` (JSON mode)
- **Config**: provider, model, prompts, response format, output schema

```json
{
  "id": "llm-1",
  "name": "Classify Intent",
  "type": "llm",
  "input_ports": [
    {"name": "query", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {
      "name": "response",
      "schema": {
        "type": "object",
        "properties": {
          "intent": {"type": "string"},
          "confidence": {"type": "number"}
        }
      }
    }
  ],
  "config": {
    "provider": "openai",
    "model": "gpt-4o",
    "api_key_ref": "OPENAI_API_KEY",
    "system_prompt": "You are an intent classifier.",
    "user_prompt": "Classify this query: {{query}}",
    "variables": [
      {"name": "query", "schema": {"type": "string"}}
    ],
    "response_format": "json",
    "output_schema": {
      "type": "object",
      "properties": {
        "intent": {"type": "string"},
        "confidence": {"type": "number"}
      }
    },
    "temperature": 0.0
  }
}
```

**Supported providers**: `openai`, `anthropic`, `google`, `openrouter`, `bedrock`, `custom`

### tool

> Full reference: [Tool Node](../nodes/tool.md)

Invokes a tool on an MCP (Model Context Protocol) server.

- **Input ports**: derived from the tool's input schema
- **Output ports**: derived from the tool's output schema
- **Config**: MCP server URL, tool name, transport, custom headers

```json
{
  "id": "tool-1",
  "name": "Search Database",
  "type": "tool",
  "input_ports": [
    {"name": "query", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {
      "name": "results",
      "schema": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "title": {"type": "string"},
            "score": {"type": "number"}
          }
        }
      }
    }
  ],
  "config": {
    "tool_name": "search",
    "mcp_url": "http://my-mcp-server:3001/sse",
    "mcp_transport": "sse"
  }
}
```

### conditional

> Full reference: [Conditional Node](../nodes/conditional.md)

Routes execution down different branches based on conditions evaluated against input data.

- **Input ports**: the data to evaluate conditions against
- **Output ports**: one per branch label, plus the default label
- **Config**: branches (label + condition pairs) and a default label

```json
{
  "id": "cond-1",
  "name": "Route by Intent",
  "type": "conditional",
  "input_ports": [
    {"name": "intent", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {"name": "billing", "schema": {"type": "string"}},
    {"name": "technical", "schema": {"type": "string"}},
    {"name": "other", "schema": {"type": "string"}}
  ],
  "config": {
    "branches": [
      {"label": "billing", "condition": "input.intent == 'billing'"},
      {"label": "technical", "condition": "input.intent == 'technical'"}
    ],
    "default_label": "other"
  }
}
```

Only one branch fires. Non-matching branches produce skip signals that propagate to downstream nodes. See [Branching](branching.md) for details.

### transform

> Full reference: [Transform Node](../nodes/transform.md)

Reshapes data using [expressions](expressions.md). Maps output port names to expression strings. Expressions have access to `input.*`, `state.*`, and `meta.*` namespaces.

- **Input ports**: data to transform
- **Output ports**: transformed results
- **Config**: expression map (output port name to expression)

```json
{
  "id": "transform-1",
  "name": "Build Summary",
  "type": "transform",
  "input_ports": [
    {"name": "name", "schema": {"type": "string"}},
    {"name": "score", "schema": {"type": "number"}}
  ],
  "output_ports": [
    {"name": "summary", "schema": {"type": "string"}}
  ],
  "config": {
    "expressions": {
      "summary": "'User ' + input.name + ' scored ' + toString(input.score)"
    }
  }
}
```

### foreach

> Full reference: [ForEach Node](../nodes/foreach.md)

Iterates over an array, executing an inline [subgraph](subgraphs.md) for each item. Supports concurrency control and error handling.

- **Input ports**: `items` (array to iterate), `context` (optional data passed to every iteration)
- **Output ports**: `results` (array of results), `errors` (array of errors)
- **Config**: inline subgraph, concurrency, error handling strategy

```json
{
  "id": "foreach-1",
  "name": "Process Items",
  "type": "foreach",
  "input_ports": [
    {
      "name": "items",
      "schema": {
        "type": "array",
        "items": {"type": "string"}
      }
    }
  ],
  "output_ports": [
    {
      "name": "results",
      "schema": {
        "type": "array",
        "items": {"type": "string"}
      }
    }
  ],
  "config": {
    "graph": { "...inline subgraph..." },
    "concurrency": 5,
    "on_item_error": "continue"
  }
}
```

The inner subgraph receives `item` (current element), `index` (position), and `context` (shared data) as inputs.

### subgraph

> Full reference: [Subgraph Node](../nodes/subgraph.md)

Embeds another graph as a single node. Useful for composition and reuse. See [Subgraphs concept page](subgraphs.md) for patterns and state isolation details.

- **Input ports**: mapped to inner graph inputs
- **Output ports**: mapped from inner graph outputs
- **Config**: inline graph definition and port mapping

```json
{
  "id": "sub-1",
  "name": "Summarize",
  "type": "subgraph",
  "input_ports": [
    {"name": "text", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {"name": "summary", "schema": {"type": "string"}}
  ],
  "config": {
    "graph": { "...inline subgraph..." },
    "port_mapping": {
      "inputs": {"text": "inner-input.text"},
      "outputs": {"inner-output.summary": "summary"}
    }
  }
}
```

### human_in_the_loop

> Full reference: [Human-in-the-Loop Node](../nodes/human-in-the-loop.md)

Pauses execution and waits for human input before continuing.

- **Input ports**: data to present to the human
- **Output ports**: the human's response and chosen action
- **Config**: prompt text, allowed actions, timeout

```json
{
  "id": "hitl-1",
  "name": "Approval",
  "type": "human_in_the_loop",
  "input_ports": [
    {"name": "proposal", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {"name": "decision", "schema": {"type": "string"}},
    {"name": "action", "schema": {"type": "string"}}
  ],
  "config": {
    "prompt_text": "Please review and approve this proposal.",
    "allowed_actions": ["approve", "reject", "revise"],
    "timeout_seconds": 3600
  }
}
```

### superagent

> Full reference: [Superagent Node](../nodes/superagent.md)

An autonomous agent loop that plans, executes tool calls, tracks progress, evaluates completion, and assembles structured output -- all inside a single node.

- **Input ports**: template variables referenced in the prompt (e.g., `{{input.topic}}` creates a `topic` port)
- **Output ports**: developer-defined (populated via buffer finalization, single-string fallback, or extraction LLM), plus automatic meta outputs (`_conversation_history`, `_iterations`, `_finish_reason`, etc.)
- **Config**: prompt, skills (MCP servers), provider, model, limits, overrides

```json
{
  "id": "agent-1",
  "name": "Research Agent",
  "type": "superagent",
  "input_ports": [
    {"name": "topic", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {"name": "report", "schema": {"type": "string"}}
  ],
  "config": {
    "prompt": "Research '{{input.topic}}' and produce a report.",
    "skills": [
      {"name": "search", "description": "Web search", "mcp_url": "http://search:9001"}
    ],
    "provider": "anthropic",
    "model": "claude-sonnet-4-6",
    "api_key_ref": "anthropic_key",
    "max_iterations": 10,
    "timeout_seconds": 300
  }
}
```

The superagent features built-in task tracking, shared memory (across nodes), output buffer assembly, stuck detection with reflection, and context compaction. See the [Superagent concept page](superagent.md) and [full node reference](../nodes/superagent.md) for details.

## Retry Policy

Any node can have a retry policy for handling transient failures:

```json
{
  "retry_policy": {
    "max_retries": 3,
    "backoff_strategy": "exponential",
    "initial_delay_seconds": 1.0,
    "max_delay_seconds": 30.0
  }
}
```

| Field | Description |
|-------|-------------|
| `max_retries` | Maximum number of retry attempts |
| `backoff_strategy` | `"fixed"` or `"exponential"` |
| `initial_delay_seconds` | Delay before first retry |
| `max_delay_seconds` | Maximum delay between retries |

## Timeout

Set `timeout_seconds` on a node to limit how long it can run:

```json
{
  "timeout_seconds": 30
}
```

If the node exceeds the timeout, it fails with a timeout error.

## See Also

- [Ports and Typing](ports-and-typing.md) -- how input/output ports and schemas work
- [Edges](edges.md) -- how nodes are wired together
- [Branching](branching.md) -- conditional routing and fork/join patterns
- [Subgraphs](subgraphs.md) -- reuse and composition via subgraph nodes
- [Superagent](superagent.md) -- the autonomous agent node in depth
- [Execution](execution.md) -- how nodes are scheduled and run
- [Node Reference](../nodes/) -- full configuration reference for each node type

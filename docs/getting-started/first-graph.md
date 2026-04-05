# Build Your First Graph

This tutorial walks through building a graph that takes text input, transforms it to uppercase, and outputs the result. You will learn the graph structure, how nodes and ports connect, and how to execute the graph.

We cover three approaches:
1. **API approach** -- create and execute via `curl`
2. **YAML approach** -- define the graph as a YAML file
3. **Coding agent approach** -- describe the graph in natural language and let your coding agent generate it

## Prerequisites

Brockley must be running locally. See the [Quickstart](quickstart.md) if you haven't set it up yet.

## What We're Building

A three-node graph:

```
[Input] --text--> [Transform (uppercase)] --result--> [Output]
```

- **Input node**: accepts a `text` field (string)
- **Transform node**: converts the text to uppercase using an expression
- **Output node**: returns the `result` field (string)

## Approach 1: API with curl

### Step 1: Define the Graph

```bash
curl -s -X POST http://localhost:8000/api/v1/graphs \
  -H "Content-Type: application/json" \
  -d '{
    "name": "text-uppercaser",
    "description": "Transforms input text to uppercase",
    "namespace": "default",
    "nodes": [
      {
        "id": "input-1",
        "name": "Text Input",
        "type": "input",
        "input_ports": [],
        "output_ports": [
          {
            "name": "text",
            "schema": {"type": "string"}
          }
        ],
        "config": {}
      },
      {
        "id": "transform-1",
        "name": "To Uppercase",
        "type": "transform",
        "input_ports": [
          {
            "name": "text",
            "schema": {"type": "string"}
          }
        ],
        "output_ports": [
          {
            "name": "result",
            "schema": {"type": "string"}
          }
        ],
        "config": {
          "expressions": {
            "result": "input.text.upper()"
          }
        }
      },
      {
        "id": "output-1",
        "name": "Result Output",
        "type": "output",
        "input_ports": [
          {
            "name": "result",
            "schema": {"type": "string"}
          }
        ],
        "output_ports": [],
        "config": {}
      }
    ],
    "edges": [
      {
        "id": "edge-1",
        "source_node_id": "input-1",
        "source_port": "text",
        "target_node_id": "transform-1",
        "target_port": "text"
      },
      {
        "id": "edge-2",
        "source_node_id": "transform-1",
        "source_port": "result",
        "target_node_id": "output-1",
        "target_port": "result"
      }
    ]
  }' | jq .
```

Note the `id` field in the response. You will need it to execute the graph.

### Step 2: Validate the Graph

Before executing, you can validate the graph to catch structural or typing errors:

```bash
curl -s -X POST http://localhost:8000/api/v1/graphs/GRAPH_ID/validate | jq .
```

Expected response:

```json
{
  "valid": true,
  "errors": [],
  "warnings": []
}
```

### Step 3: Execute the Graph

```bash
curl -s -X POST http://localhost:8000/api/v1/executions \
  -H "Content-Type: application/json" \
  -d '{
    "graph_id": "GRAPH_ID",
    "input": {
      "text": "hello brockley"
    },
    "mode": "sync"
  }' | jq .
```

Expected output:

```json
{
  "id": "exec-...",
  "status": "completed",
  "output": {
    "result": "HELLO BROCKLEY"
  }
}
```

### Step 4: Inspect the Execution Steps

To see what happened at each node:

```bash
curl -s http://localhost:8000/api/v1/executions/EXECUTION_ID/steps | jq .
```

This returns an array of step records showing the input, output, status, and duration for each node that ran.

## Approach 2: YAML Definition

You can also define graphs as YAML files. This is useful for version control, code review, and CI/CD pipelines.

### Step 1: Create the YAML File

Save this as `text-uppercaser.yaml`:

```yaml
name: text-uppercaser
description: Transforms input text to uppercase
namespace: default

nodes:
  - id: input-1
    name: Text Input
    type: input
    input_ports: []
    output_ports:
      - name: text
        schema:
          type: string
    config: {}

  - id: transform-1
    name: To Uppercase
    type: transform
    input_ports:
      - name: text
        schema:
          type: string
    output_ports:
      - name: result
        schema:
          type: string
    config:
      expressions:
        result: "input.text.upper()"

  - id: output-1
    name: Result Output
    type: output
    input_ports:
      - name: result
        schema:
          type: string
    output_ports: []
    config: {}

edges:
  - id: edge-1
    source_node_id: input-1
    source_port: text
    target_node_id: transform-1
    target_port: text

  - id: edge-2
    source_node_id: transform-1
    source_port: result
    target_node_id: output-1
    target_port: result
```

### Step 2: Create via API

Convert YAML to JSON and post it:

```bash
# Using yq to convert YAML to JSON, then curl to create the graph
yq -o=json text-uppercaser.yaml | \
  curl -s -X POST http://localhost:8000/api/v1/graphs \
    -H "Content-Type: application/json" \
    -d @- | jq .
```

Or, if you have the Brockley CLI installed:

```bash
brockley create -f text-uppercaser.yaml
```

### Step 3: Validate with the CLI

```bash
brockley validate text-uppercaser.yaml
```

Expected output:

```
text-uppercaser.yaml: valid
```

## Approach 3: Coding Agent

If you use Claude Code, Cursor, Copilot, or another coding agent, you can generate graphs from natural language instead of writing JSON or YAML by hand. Brockley ships a [coding agent skill](../../coding-agent-skills/SKILL.md) that gives your agent complete knowledge of every node type, port schema rule, expression operator, and validation constraint.

### Step 1: Set Up the Skill (One-Time)

**Claude Code:**

```bash
cp coding-agent-skills/SKILL.md .claude/commands/brockley.md
```

**Cursor** -- add to `.cursorrules`:

```
When writing Brockley graphs, follow the spec in coding-agent-skills/SKILL.md
```

See the [coding agent skills README](../../coding-agent-skills/README.md) for other agents.

### Step 2: Ask Your Agent to Build the Graph

Prompt your coding agent:

> Build me a Brockley graph that takes a text string as input, transforms it to uppercase using the expression language, and outputs the result.

Your agent reads the skill file and produces valid graph JSON -- the same structure as Approach 1 above, but generated from a natural language description.

### Step 3: Validate and Deploy

```bash
# Validate the generated graph
brockley validate -f text-uppercaser.json

# Deploy it
brockley deploy -f text-uppercaser.json
```

### When This Approach Shines

The text-uppercaser is simple enough to write by hand. Coding agents become especially powerful for complex graphs -- multi-step pipelines with LLM nodes, conditional routing, tool calling, superagent loops, and state management. Instead of manually wiring dozens of nodes and edges, describe the workflow and let your agent handle the structural details while you review and iterate.

## Understanding the Graph Structure

Let's break down what each part does:

### Nodes

Each node has:
- **`id`**: unique identifier within the graph
- **`name`**: human-readable label
- **`type`**: determines behavior (`input`, `output`, `transform`, `llm`, etc.)
- **`input_ports`**: typed inputs the node receives
- **`output_ports`**: typed outputs the node produces
- **`config`**: type-specific configuration

### Ports

Ports are the connection points. Every port has:
- **`name`**: identifier used in edges and expressions
- **`schema`**: JSON Schema defining the data type

Brockley enforces strong typing. You cannot use bare `{"type": "object"}` without `properties`, or bare `{"type": "array"}` without `items`.

### Edges

Edges wire output ports to input ports:
- **`source_node_id`** + **`source_port`**: where data comes from
- **`target_node_id`** + **`target_port`**: where data goes to

### Transform Expressions

In the transform node's config, `expressions` maps output port names to [expression language](../concepts/expressions.md) strings:

```json
{
  "expressions": {
    "result": "input.text.upper()"
  }
}
```

The expression `input.text.upper()` accesses the `text` input port and calls the `upper()` string method. The expression language supports string operations, array processing, object manipulation, arithmetic, conditionals, and more. See the [full expression reference](../expressions/overview.md) for all available operations.

### What Else Could You Use Here?

The transform node is the right choice for pure data reshaping. But you could substitute other node types depending on your goal:

- An **[LLM node](../nodes/llm.md)** if you wanted an AI model to rewrite the text (e.g., "rewrite this in a formal tone") rather than a deterministic transformation.
- A **[tool node](../nodes/tool.md)** if the transformation requires calling an external service (e.g., a translation API exposed via MCP).
- A **[conditional node](../nodes/conditional.md)** if you wanted to route the text down different paths based on its content before transforming it.

## Common Mistakes

| Mistake | Error Code | Fix |
|---------|-----------|-----|
| Missing port schema | `MISSING_PORT_SCHEMA` | Every port needs a `schema` field |
| Bare object schema | `SCHEMA_VIOLATION` | Use `{"type": "object", "properties": {...}}` not just `{"type": "object"}` |
| Unwired required port | `UNWIRED_REQUIRED_PORT` | Connect an edge to every required input port, or add a `default` |
| Missing input node | `NO_INPUT_NODE` | Every graph needs at least one `input` type node |
| Edge referencing missing port | `INVALID_SOURCE_PORT` | Check that the port name matches exactly |

## Next Steps

- [Architecture Overview](architecture-overview.md) -- understand how the system works
- [Concepts: Nodes](../concepts/nodes.md) -- learn about all node types
- [Concepts: Ports and Typing](../concepts/ports-and-typing.md) -- deep dive into the type system
- [Concepts: Edges](../concepts/edges.md) -- how data flows between nodes
- [Concepts: Expressions](../concepts/expressions.md) -- the expression language used in transforms, conditions, and templates
- [Concepts: Execution](../concepts/execution.md) -- understand how graphs execute
- [Build with Coding Agents](../../coding-agent-skills/README.md) -- full setup guide and examples for coding agent graph authoring

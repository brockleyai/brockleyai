# Build Your First Autonomous Agent

This tutorial walks you through creating a superagent node that autonomously uses tools to complete a task. By the end, you will have a working graph with an agent that searches for information and produces a structured report.

## Prerequisites

- A running Brockley instance ([quickstart guide](../README.md))
- An MCP server with tools (we'll use a simple search tool)
- An LLM provider API key (OpenAI, Anthropic, Google, OpenRouter, or Bedrock)

## Step 1: Minimal Superagent Graph

Create a graph with three nodes: input, superagent, and output.

```json
{
  "name": "Simple Research Agent",
  "nodes": [
    {
      "id": "input",
      "name": "Input",
      "type": "input",
      "output_ports": [
        {"name": "topic", "schema": {"type": "string"}}
      ]
    },
    {
      "id": "agent",
      "name": "Research Agent",
      "type": "superagent",
      "input_ports": [
        {"name": "topic", "schema": {"type": "string"}}
      ],
      "output_ports": [
        {"name": "report", "schema": {"type": "string"}}
      ],
      "config": {
        "prompt": "Research '{{input.topic}}' and produce a concise report with key findings.",
        "skills": [
          {
            "name": "search",
            "description": "Search the web for information",
            "mcp_url": "http://localhost:9001/mcp"
          }
        ],
        "provider": "anthropic",
        "model": "claude-sonnet-4-6",
        "api_key_ref": "anthropic_key",
        "max_iterations": 5,
        "timeout_seconds": 120
      }
    },
    {
      "id": "output",
      "name": "Output",
      "type": "output",
      "input_ports": [
        {"name": "report", "schema": {"type": "string"}}
      ]
    }
  ],
  "edges": [
    {"id": "e1", "source_node_id": "input", "source_port": "topic", "target_node_id": "agent", "target_port": "topic"},
    {"id": "e2", "source_node_id": "agent", "source_port": "report", "target_node_id": "output", "target_port": "report"}
  ]
}
```

Key points:
- `prompt` uses `{{input.topic}}` to inject the input value.
- `skills` connects to one MCP server.
- `max_iterations: 5` and `timeout_seconds: 120` keep the agent bounded for testing.
- The agent will use its single `report` output port and the single-string fallback to produce output.

## Step 2: Run It

Create and execute the graph:

```bash
# Create the graph
brockley graph create -f graph.json

# Execute with an input
brockley graph execute <graph-id> --input '{"topic": "recent advances in battery technology"}'
```

The agent will:
1. Read the prompt and understand its task
2. Use the search tool to find relevant information
3. Synthesize findings into a report
4. Return the report on the `report` output port

## Step 3: Add Task Tracking

Task tracking helps the agent stay organized. No config changes needed -- it is enabled by default. The agent automatically gets access to `_task_create`, `_task_update`, and `_task_list` tools.

To see what tasks the agent created, check the `_tasks` meta output:

```bash
brockley graph execution <execution-id> --output _tasks
```

## Step 4: Use Buffers for Large Output

For longer outputs, use buffer tools instead of the single-string fallback. Update the prompt to guide the agent:

```json
{
  "prompt": "Research '{{input.topic}}' thoroughly. Use _buffer_create to create a 'report' buffer, build it incrementally with _buffer_append, then finalize it to the 'report' output port with _buffer_finalize.",
  "max_iterations": 10
}
```

The agent will now build the report piece by piece across multiple iterations, then finalize the buffer to the output port. This bypasses the extraction LLM entirely.

## Step 5: Add Shared Memory

To share knowledge between multiple agents in a pipeline, enable shared memory:

```json
{
  "config": {
    "shared_memory": {
      "enabled": true,
      "namespace": "research"
    }
  },
  "state_reads": [{"state_field": "_superagent_memory", "port": "_memory_in"}],
  "state_writes": [{"state_field": "_superagent_memory", "port": "_memory_out"}]
}
```

And add the state field to your graph:

```json
{
  "state": {
    "fields": [
      {"name": "_superagent_memory", "schema": {"type": "object"}, "reducer": "merge", "initial": {}}
    ]
  }
}
```

Now the agent can store key findings via `_memory_store`, and downstream agents can read them.

## Step 6: Inspect the Execution

Check meta outputs to understand what the agent did:

```bash
# How many iterations did it run?
brockley graph execution <id> --output _iterations

# Why did it stop?
brockley graph execution <id> --output _finish_reason

# What tools did it call?
brockley graph execution <id> --output _tool_call_history

# Full conversation history (for debugging)
brockley graph execution <id> --output _conversation_history
```

## See Also

- [Superagent Node Reference](../nodes/superagent.md) -- all configuration fields and defaults
- [Customizing the Superagent](superagent-advanced.md) -- overrides, multi-model configs, tool policies
- [Multi-Agent Patterns](superagent-patterns.md) -- pipeline, fan-out, and outer loop patterns
- [Superagent Concepts](../concepts/superagent.md) -- architecture and design principles

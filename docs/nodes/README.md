# Node Type Reference

Every node in a graph has a type that determines what it does. This section is the complete reference for all built-in node types -- configuration fields, port schemas, behavior, and examples.

If you haven't read [Nodes](../concepts/nodes.md) and [Ports and Typing](../concepts/ports-and-typing.md) yet, start there for the conceptual foundation.

## Structural nodes

Every graph starts and ends with these.

- **[Input and Output](input-output.md)** -- Define the graph's external interface: what data goes in and what comes out.

## AI nodes

Nodes that call language models.

- **[LLM](llm.md)** -- Call any supported LLM provider. Supports template prompts, text and JSON response modes, structured output validation, tool calling, and the autonomous tool loop.
- **[Superagent](superagent.md)** -- An autonomous agent loop that plans, calls tools, executes code, and self-evaluates until a task is complete.

## Tool nodes

Nodes that invoke external tools and APIs.

- **[Tool (MCP)](tool.md)** -- Invoke tools served by MCP (Model Context Protocol) servers.
- **[API Tool](api-tool.md)** -- Call REST APIs directly, with configurable method, headers, body, and response extraction.

## Flow control nodes

Nodes that control how execution moves through the graph.

- **[Conditional](conditional.md)** -- Route execution down different branches based on expression conditions.
- **[ForEach](foreach.md)** -- Fan out over an array, executing a subgraph once per item, then collect the results.
- **[Subgraph](subgraph.md)** -- Embed another graph as a single node for composition and reuse.

## Data nodes

- **[Transform](transform.md)** -- Reshape data using expressions. No external calls -- pure computation.

## Interaction nodes

- **[Human-in-the-Loop](human-in-the-loop.md)** -- Pause execution and wait for human input before continuing.

## Extension

- **[Custom Node Types](custom.md)** -- Build your own node types to extend Brockley's capabilities.

## Where to go next

- **[Guides](../guides/)** -- Hands-on tutorials that show these nodes in action.
- **[LLM Providers](../providers/)** -- Configure the LLM backends used by LLM and Superagent nodes.
- **[Core Concepts](../concepts/)** -- Understand the model that these nodes operate within.

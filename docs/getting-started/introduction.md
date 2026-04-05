# What is Brockley?

Brockley is an open-source AI agent infrastructure platform. It lets you define, manage, execute, and operationalize agent workflows as structured [graphs](../concepts/graphs.md) -- through a visual web UI, a CLI, Terraform/OpenTofu, or an MCP server for AI coding agents.

## Why Brockley?

Most agent-building tools fall into one of two camps:

- **Demo-oriented visual tools** that look nice but lack the engineering discipline needed for production use.
- **Enterprise black boxes** that are hard to self-host, extend, or reason about.

Brockley takes a different approach. It treats agent workflows as **software infrastructure**: typed, validated, version-controlled, and deployable through the same pipelines you use for everything else.

## Key Concepts

### Graphs

A [**graph**](../concepts/graphs.md) is a self-contained agent workflow. It defines a directed graph of [nodes](../concepts/nodes.md) connected by [edges](../concepts/edges.md), with optional controlled [loops](../concepts/loops.md). Graphs are the unit of deployment, versioning, and execution.

### Nodes

[**Nodes**](../concepts/nodes.md) are the steps in a graph. Each node has a type (LLM call, tool invocation, conditional branch, transform, etc.), typed input [ports](../concepts/ports-and-typing.md), typed output ports, and configuration specific to its type.

Built-in node types:

| Type | Purpose |
|------|---------|
| `input` | Entry point -- defines the graph's external inputs |
| `output` | Exit point -- defines the graph's external outputs |
| `llm` | Calls an LLM provider (OpenAI, Anthropic, Google, etc.) |
| `tool` | Invokes an MCP tool server |
| `conditional` | Routes execution down different branches based on [conditions](../concepts/branching.md) |
| `transform` | Reshapes data using [expressions](../concepts/expressions.md) |
| `foreach` | Fans out over an array, executing a [subgraph](../concepts/subgraphs.md) per item |
| `subgraph` | Embeds another graph as a single node |
| `superagent` | Autonomous [agent loop](../concepts/superagent.md) with planning, tool use, and self-evaluation |
| `human_in_the_loop` | Pauses execution for human input |

### Ports

[**Ports**](../concepts/ports-and-typing.md) are the typed connection points on nodes. Every port has a name and a JSON Schema. Edges connect an output port on one node to an input port on another. Brockley enforces strong typing: no bare `{"type": "object"}` or `{"type": "array"}` schemas are allowed.

### State

[**Graph state**](../concepts/state.md) provides named, typed fields that persist across execution and accumulate values through reducers (`replace`, `append`, `merge`). State is how you build memory, accumulate results, or track context across a multi-step workflow.

### Edges

[**Edges**](../concepts/edges.md) wire output ports to input ports. Data flows along edges. Back-edges create controlled [loops](../concepts/loops.md) with conditions and iteration limits.

## Who is Brockley For?

- **Individual developers** who want to prototype and run agent workflows quickly, self-host, and define them with a GUI, CLI, or Terraform.
- **Engineering teams** who want version-controlled, testable, reviewable agent workflows with API access and deployment discipline.
- **Platform teams** who want to standardize how agent systems are defined and operated across environments.
- **AI coding agent users** who want tools like Claude Code or Cursor to build and test workflows programmatically through MCP.

## What You Can Do With Brockley

**Build agent workflows visually or as code.** The web UI provides a drag-and-drop graph editor. You can also define graphs as JSON or YAML files, version them in Git, and create them through the API or CLI.

**Connect to any LLM provider.** OpenAI, Anthropic, Google, AWS Bedrock, OpenRouter, or bring your own via the custom provider interface. Switch providers by changing a config field -- no code changes.

**Integrate MCP tools.** Any [Model Context Protocol](https://modelcontextprotocol.io/) server works as a tool source. Point a tool node or superagent skill at an MCP endpoint, and Brockley handles discovery, invocation, and error handling.

**Validate before execution.** The engine checks graph structure, port types, edge wiring, cycle safety, state reducer compatibility, and reachability -- all before a single node runs. Catch misconfigurations at design time, not at 2am.

**Execute synchronously, asynchronously, or with streaming.** Sync mode blocks until completion. Async returns an execution ID immediately. SSE streaming delivers real-time events: node start/complete, LLM tokens, state updates, superagent iterations.

**Run autonomous agents.** The [superagent](../concepts/superagent.md) node type is a full agent loop in a single node -- planning, multi-step tool calling, self-evaluation, reflection when stuck, context compaction, and structured output assembly. Drop it into any graph alongside other nodes.

**Manage everything through a REST API.** Create, list, update, delete, validate, and execute graphs. Inspect execution steps with full input/output/timing data. Cancel running executions. All responses are JSON.

**Deploy with Terraform/OpenTofu.** Define graphs, schemas, and provider configs as Terraform resources. Plan and apply through standard IaC workflows. Diff changes before deployment.

**Script with the CLI.** Create, validate, execute, and manage graphs from the command line. Useful for CI/CD pipelines, scripting, and local development without the web UI.

**Build graphs with coding agents.** Describe the workflow you want in natural language and let Claude Code, Cursor, Copilot, or any coding agent produce valid, deployable graph JSON. Brockley ships a self-contained [coding agent skill](../../coding-agent-skills/README.md) that documents every node type, config field, expression operator, and validation rule -- so your agent never guesses.

**Self-host anywhere.** Docker Compose for local dev. Helm chart for Kubernetes. The full stack is five containers: server, worker, web UI, PostgreSQL, and Redis.

## Next Steps

- [Quickstart](quickstart.md) -- get Brockley running locally in under 5 minutes
- [Build Your First Graph](first-graph.md) -- step-by-step tutorial (API, YAML, or coding agent)
- [Architecture Overview](architecture-overview.md) -- understand how the system works
- [Build with Coding Agents](../../coding-agent-skills/README.md) -- set up Claude Code, Cursor, or Copilot to author graphs

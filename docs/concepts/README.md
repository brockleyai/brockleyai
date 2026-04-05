# Core Concepts

This section explains the foundational model behind Brockley -- how graphs, nodes, edges, ports, state, and execution work together. Read these before diving into the node type reference or guides.

## Foundational model

Read these in order. Each builds on the previous.

1. **[Graphs](graphs.md)** -- The fundamental unit in Brockley: a self-contained agent workflow defined as a directed graph of nodes and edges.

2. **[Nodes](nodes.md)** -- The steps in a graph. Each node has a type, typed input and output ports, and configuration specific to its type.

3. **[Edges](edges.md)** -- Connections between node ports that define how data flows through a graph. Edges enforce type compatibility.

4. **[Ports and Typing](ports-and-typing.md)** -- Every port has a JSON Schema type. The type system catches mismatches at validation time, not runtime.

5. **[State](state.md)** -- Persistent graph-level fields that accumulate across execution. State uses reducers (append, merge, replace) to control how values evolve.

## Execution and flow control

Read these as you need them. They cover how graphs run and how to control flow.

6. **[Execution Model](execution.md)** -- How Brockley runs a graph: distributed async execution, step-level tracking, the orchestrator-worker model.

7. **[Expressions](expressions.md)** -- Reference upstream outputs, apply filters, and build templates. Used in prompts, conditionals, and transforms. For the full language reference, see [Expression Language](../expressions/).

8. **[Branching and Joining](branching.md)** -- Conditional nodes, fork/join patterns, exclusive fan-in, and skip propagation.

9. **[Loops](loops.md)** -- ForEach nodes for iterating over arrays, back-edges for controlled cycles, and loop termination.

10. **[Subgraphs](subgraphs.md)** -- Nest one graph inside another as a single node. Useful for composition and reuse.

## Superagent

Read these when you're working with autonomous agents.

11. **[Superagent](superagent.md)** -- The autonomous agent loop: planning, tool calling, code execution, task management, and configurable termination.

12. **[Superagent Built-in Tools](superagent-tools.md)** -- The tools available to superagents out of the box: code execution, file operations, web search, and more.

## Where to go next

- **[Node Types](../nodes/)** -- Detailed reference for each node type introduced here.
- **[Expression Language](../expressions/)** -- Full reference for operators, filters, and template syntax.
- **[Guides](../guides/)** -- Hands-on tutorials that put these concepts into practice.

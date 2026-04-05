# Multi-Agent Patterns

This guide covers three canonical patterns for composing multiple superagent nodes in a single graph. Each pattern uses shared memory to pass knowledge between agents beyond what flows through edges.

## Pattern 1: Pipeline with Shared Memory

A sequential chain of agents where each builds on the previous one's findings.

```
[input] -> [research (SA)] -> [analysis (SA)] -> [writing (SA)] -> [output]
           stores findings    recalls findings    recalls everything
           in memory          adds analysis       builds final report
```

Each agent adds facts to shared memory. Later agents have richer context than what passes through edges alone.

### Graph

```json
{
  "name": "Research Pipeline",
  "nodes": [
    {
      "id": "input", "type": "input",
      "output_ports": [{"name": "topic", "schema": {"type": "string"}}]
    },
    {
      "id": "researcher", "type": "superagent",
      "input_ports": [{"name": "topic", "schema": {"type": "string"}}],
      "output_ports": [{"name": "summary", "schema": {"type": "string"}}],
      "config": {
        "prompt": "Research '{{input.topic}}'. Use _memory_store to save key findings with tags like 'finding', 'source', 'statistic'. Produce a brief summary.",
        "skills": [
          {"name": "search", "description": "Web search", "mcp_url": "http://search:9001"},
          {"name": "reader", "description": "Read web pages", "mcp_url": "http://reader:9002"}
        ],
        "provider": "anthropic", "model": "claude-sonnet-4-6", "api_key_ref": "anthropic_key",
        "shared_memory": {"enabled": true, "namespace": "research"},
        "max_iterations": 10
      },
      "state_reads": [{"state_field": "_superagent_memory", "port": "_memory_in"}],
      "state_writes": [{"state_field": "_superagent_memory", "port": "_memory_out"}]
    },
    {
      "id": "analyst", "type": "superagent",
      "input_ports": [{"name": "summary", "schema": {"type": "string"}}],
      "output_ports": [{"name": "analysis", "schema": {"type": "string"}}],
      "config": {
        "prompt": "Analyze the research on '{{input.summary}}'. Use _memory_recall to find all findings, then identify patterns, gaps, and insights. Store your analysis in memory with 'analysis' tag.",
        "skills": [],
        "provider": "anthropic", "model": "claude-sonnet-4-6", "api_key_ref": "anthropic_key",
        "shared_memory": {"enabled": true, "namespace": "analysis"},
        "max_iterations": 5
      },
      "state_reads": [{"state_field": "_superagent_memory", "port": "_memory_in"}],
      "state_writes": [{"state_field": "_superagent_memory", "port": "_memory_out"}]
    },
    {
      "id": "writer", "type": "superagent",
      "input_ports": [{"name": "analysis", "schema": {"type": "string"}}],
      "output_ports": [{"name": "report", "schema": {"type": "string"}}],
      "config": {
        "prompt": "Write a comprehensive report. Use _memory_recall to retrieve all research findings and analysis. Use _buffer_create and _buffer_append to build the report, then _buffer_finalize to the 'report' port.",
        "skills": [],
        "provider": "anthropic", "model": "claude-sonnet-4-6", "api_key_ref": "anthropic_key",
        "shared_memory": {"enabled": true, "namespace": "writing"},
        "max_iterations": 5
      },
      "state_reads": [{"state_field": "_superagent_memory", "port": "_memory_in"}],
      "state_writes": [{"state_field": "_superagent_memory", "port": "_memory_out"}]
    },
    {
      "id": "output", "type": "output",
      "input_ports": [{"name": "report", "schema": {"type": "string"}}]
    }
  ],
  "edges": [
    {"id": "e1", "source_node_id": "input", "source_port": "topic", "target_node_id": "researcher", "target_port": "topic"},
    {"id": "e2", "source_node_id": "researcher", "source_port": "summary", "target_node_id": "analyst", "target_port": "summary"},
    {"id": "e3", "source_node_id": "analyst", "source_port": "analysis", "target_node_id": "writer", "target_port": "analysis"},
    {"id": "e4", "source_node_id": "writer", "source_port": "report", "target_node_id": "output", "target_port": "report"}
  ],
  "state": {
    "fields": [
      {"name": "_superagent_memory", "schema": {"type": "object"}, "reducer": "merge", "initial": {}}
    ]
  }
}
```

### Key Points

- Each agent has a different `namespace` so you can filter by origin when recalling.
- The writer uses buffers for large output assembly.
- Edges carry brief summaries; shared memory carries detailed findings.

## Pattern 2: Fan-Out with Memory Merge

Parallel agents that work independently, with their memories merged after completion.

```
            +-> [security-review (SA)] --+
[input] -> [split]                        -> [merge] -> [output]
            +-> [perf-review (SA)] ------+
            +-> [style-review (SA)] -----+
```

### When to Use

- Parallel code reviews (security, performance, style)
- Multi-perspective analysis (technical, business, legal)
- Competitive analysis from multiple sources

### Graph (Simplified)

```json
{
  "nodes": [
    {
      "id": "security-review", "type": "superagent",
      "config": {
        "prompt": "Review the code for security vulnerabilities. Store each finding via _memory_store with 'security' tag.",
        "skills": [{"name": "code", "description": "Read source code", "mcp_url": "http://code:9001"}],
        "provider": "anthropic", "model": "claude-sonnet-4-6", "api_key_ref": "anthropic_key",
        "shared_memory": {"enabled": true, "namespace": "security"},
        "system_preamble": "You are a security expert. Focus on OWASP Top 10, injection, auth issues."
      },
      "state_reads": [{"state_field": "_superagent_memory", "port": "_memory_in"}],
      "state_writes": [{"state_field": "_superagent_memory", "port": "_memory_out"}]
    },
    {
      "id": "perf-review", "type": "superagent",
      "config": {
        "prompt": "Review the code for performance issues. Store each finding via _memory_store with 'performance' tag.",
        "skills": [{"name": "code", "description": "Read source code", "mcp_url": "http://code:9001"}],
        "provider": "anthropic", "model": "claude-sonnet-4-6", "api_key_ref": "anthropic_key",
        "shared_memory": {"enabled": true, "namespace": "performance"},
        "system_preamble": "You are a performance engineer. Focus on algorithmic complexity, memory usage, I/O patterns."
      },
      "state_reads": [{"state_field": "_superagent_memory", "port": "_memory_in"}],
      "state_writes": [{"state_field": "_superagent_memory", "port": "_memory_out"}]
    }
  ],
  "state": {
    "fields": [
      {"name": "_superagent_memory", "schema": {"type": "object"}, "reducer": "merge", "initial": {}}
    ]
  }
}
```

### Key Points

- Parallel agents don't see each other's memory during execution.
- After all complete, the `merge` reducer combines their memories.
- A downstream agent can use `_memory_recall` with tag filters to read findings by category (e.g., `tags: ["security"]`).

## Pattern 3: Outer Loop with Back-Edges

An iterative refinement loop where a review agent feeds critique back to a planning agent.

```
[input] -> [plan (SA)] -> [execute (SA)] -> [review (SA)] -+
              ^                                             |
              +---- back-edge (review says "revise") ------+
```

### When to Use

- Iterative code generation (plan -> implement -> review -> revise)
- Document drafting with feedback loops
- Any task where quality improves with revision

### How It Works

1. **Plan agent** reads the task (and any critique from previous iterations) and produces a plan.
2. **Execute agent** carries out the plan using tools.
3. **Review agent** evaluates the result. If it's not good enough, it writes critique to shared memory and the graph loops back.
4. On the next iteration, the **plan agent** reads the critique from shared memory and revises.

### Key Points

- The review agent's output drives a conditional node that decides whether to loop back or proceed to output.
- Shared memory accumulates critique across iterations, giving the plan agent full revision history.
- Set `max_iterations` conservatively on each agent to prevent the outer graph loop from running too long.
- Use `system_preamble` to give each agent a clear role (planner, executor, reviewer).

## Choosing a Pattern

| Pattern | Use When | Agents Run |
|---------|----------|-----------|
| **Pipeline** | Tasks are sequential and each stage builds on the previous | Sequentially |
| **Fan-Out** | Tasks are independent and can run in parallel | In parallel |
| **Outer Loop** | Quality improves with iterative refinement | Sequentially, repeating |

All three patterns can be combined. For example, a pipeline where the middle stage fans out to parallel reviewers, followed by an outer loop for refinement.

## See Also

- [Superagent Concepts](../concepts/superagent.md) -- architecture and design principles
- [Customizing the Superagent](superagent-advanced.md) -- overrides, multi-model configs
- [Superagent Node Reference](../nodes/superagent.md) -- complete configuration reference
- [Graph State](../concepts/state.md) -- state reducers and shared state
- [Branching](../concepts/branching.md) -- conditional routing and back-edges

# Brockley Coding Agent Skills

This directory gives coding agents (Claude Code, Cursor, Copilot, Aider, etc.) everything they need to write valid Brockley agent graph definitions in JSON.

## Contents

| File | What it is |
|------|-----------|
| `SKILL.md` | Complete reference for writing Brockley graphs. Self-contained -- an LLM never needs another file. Covers all 11 node types, the expression language, validation rules, state/reducers, and common mistakes. |
| `schema.json` | JSON Schema (draft 2020-12) for the Graph type. Use for editor validation and auto-complete. |
| `examples/` | Seven annotated example graphs covering every major pattern. |

## Example Files

| File | Pattern |
|------|---------|
| `examples/simple-llm.json` | Minimal LLM call (input -> LLM -> output) |
| `examples/conditional-routing.json` | LLM classification + conditional branching |
| `examples/stateful-loop.json` | Back-edge loop with state reducers (replace, append) |
| `examples/tool-calling.json` | LLM with tool_loop + MCP tool routing |
| `examples/superagent.json` | Autonomous superagent with skills |
| `examples/foreach-parallel.json` | ForEach fan-out with inner graph |
| `examples/multi-step-pipeline.json` | Complex multi-node pipeline (classify + route + respond) |

## How to Use

### Claude Code / Cursor / Copilot

Point your agent at `SKILL.md`:

```
Read coding-agent-skills/SKILL.md and use it to generate a Brockley graph that [describe your workflow].
```

Or add it to your project's context configuration:

**Claude Code** -- add to `.claude/settings.json`:
```json
{
  "context": ["brockleyai/coding-agent-skills/SKILL.md"]
}
```

**Cursor** -- add to `.cursorrules`:
```
When writing Brockley graphs, follow the spec in coding-agent-skills/SKILL.md
```

### Validation

Validate generated graphs with the Brockley CLI:

```bash
brockley validate -f my-graph.json
```

Or via the API:

```bash
curl -X POST http://localhost:8080/api/v1/graphs \
  -H "Content-Type: application/json" \
  -d @my-graph.json
```

### JSON Schema

Point your editor at `schema.json` for auto-complete and inline validation:

```json
{
  "$schema": "./coding-agent-skills/schema.json",
  "name": "my-graph",
  ...
}
```

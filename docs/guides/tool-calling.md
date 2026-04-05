# Tool Calling Guide

This guide explains how to configure tool calling on LLM nodes in Brockley. Tool calling allows an LLM to invoke external tools (served via MCP) during generation, enabling agent-like behavior where the model can look up information, take actions, and reason over results.

---

## Overview

Brockley's LLM node supports three modes of tool interaction:

1. **No tools** -- standard LLM call with no tool access.
2. **Tools without loop** -- the LLM can request tool calls, but the engine returns them as output without executing. Useful when downstream nodes handle tool execution.
3. **Full tool loop** -- the engine autonomously executes tool calls and feeds results back to the LLM until it produces a final response.

Most users will want the full tool loop (mode 3).

---

## Simple Example: One MCP Server, Two Tools

This example configures an LLM node with two tools served by a single MCP server.

### Graph State

Define a state field to accumulate conversation history:

```yaml
state:
  fields:
    - name: conversation_history
      schema:
        type: array
        items:
          type: object
          properties:
            role:
              type: string
            content:
              type: string
          required: [role, content]
      reducer: append
      initial: []
```

### LLM Node Configuration

```yaml
- id: agent
  name: Support Agent
  type: llm
  config:
    provider: openai
    model: gpt-4o
    api_key_ref: openai_key
    system_prompt: |
      You are a customer support agent. Use the available tools
      to look up information before answering questions.
    user_prompt: "{{input.user_message}}"
    variables:
      - name: user_message
        schema:
          type: string
    response_format: text
    tool_loop: true
    max_tool_calls: 10
    max_loop_iterations: 5
    messages_from_state: conversation_history
    tools:
      - name: search_kb
        description: Search the knowledge base for relevant articles
        parameters:
          type: object
          properties:
            query:
              type: string
          required: [query]
      - name: get_order
        description: Look up a customer order by ID
        parameters:
          type: object
          properties:
            order_id:
              type: string
          required: [order_id]
    tool_routing:
      search_kb:
        mcp_url: http://localhost:9000/mcp
        mcp_transport: sse
      get_order:
        mcp_url: http://localhost:9000/mcp
        mcp_transport: sse
```

When the LLM decides it needs to search the knowledge base, it emits a `search_kb` tool call. The engine executes it against the MCP server, appends the result to the conversation, and calls the LLM again. This continues until the LLM produces a final text response.

---

## Advanced Example: Multiple MCP Servers, Auto-Discovery

In more complex setups, tools may be spread across multiple MCP servers. You can use auto-discovery to let the engine fetch tool definitions from each server at execution time, instead of defining them statically.

```yaml
- id: research-agent
  name: Research Agent
  type: llm
  config:
    provider: anthropic
    model: claude-sonnet-4-20250514
    api_key_ref: anthropic_key
    system_prompt: |
      You are a research agent. Use the available tools to gather
      information from multiple sources, then synthesize a report.
    user_prompt: "Research topic: {{input.topic}}"
    variables:
      - name: topic
        schema:
          type: string
    response_format: text
    tool_loop: true
    max_tool_calls: 25
    max_loop_iterations: 10
    tool_routing:
      search_web:
        mcp_url: http://web-search-mcp:8080/mcp
        mcp_transport: sse
        timeout_seconds: 15
      fetch_page:
        mcp_url: http://web-search-mcp:8080/mcp
        mcp_transport: sse
        timeout_seconds: 30
      query_database:
        mcp_url: http://db-mcp:8081/mcp
        mcp_transport: sse
        headers:
          - name: Authorization
            value: "Bearer ${DB_API_KEY}"
        timeout_seconds: 10
      create_document:
        mcp_url: http://docs-mcp:8082/mcp
        mcp_transport: sse
```

With auto-discovery, you can omit the `tools` array entirely and let the engine query each MCP server's `tools/list` endpoint. The engine builds the tool definitions and routing table automatically from the discovered tools.

### Dynamic Tool Routing from State

For advanced use cases where tool routing changes at runtime (e.g., multi-tenant deployments with per-tenant MCP endpoints), use `tool_routing_from_state`:

```yaml
config:
  tool_routing_from_state: "dynamic_tool_routes"
```

The referenced state field must contain a map with the same structure as `tool_routing`.

---

## API Tools: REST APIs Without MCP Wrappers

API tools let LLM nodes call REST/HTTP endpoints directly, without requiring MCP server wrappers. You define your API contracts (endpoints, methods, auth, schemas) as a reusable library resource, then select individual endpoints per node via `api_tools` refs.

This is the recommended approach when your tools are standard REST APIs. For MCP-native servers, continue using `tool_routing` as shown above.

For a full guide on API tool definitions, request/response mapping, auth configuration, and superagent integration, see `docs/guides/api-tools.md`.

### Quick Example

```yaml
- id: payment-agent
  name: Payment Agent
  type: llm
  config:
    provider: openai
    model: gpt-4o
    api_key_ref: openai_key
    system_prompt: |
      You are a payment processing agent. You can create payments
      and look up customer information via the Stripe API.
    user_prompt: "{{input.user_message}}"
    variables:
      - name: user_message
        schema:
          type: string
    response_format: text
    tool_loop: true
    max_tool_calls: 10
    max_loop_iterations: 5
    api_tools:
      - api_tool_id: stripe-api
        endpoint: create_payment_intent
      - api_tool_id: stripe-api
        endpoint: get_customer
```

The `api_tools` refs auto-derive tool schemas and routing from the API tool definition. Only the selected endpoints' schemas appear in the LLM's context -- not all endpoints in the definition. This makes API tools significantly more token-efficient than MCP auto-discovery.

API tools and MCP tools can be mixed on the same node. Use `api_tools` for REST endpoints and `tool_routing` for MCP servers.

---

## Compacted MCP Mode

When an MCP server exposes many tools (10+), auto-discovery dumps all their schemas into the LLM's context, wasting thousands of tokens on schemas the LLM may never use. **Compacted mode** is an opt-in alternative that uses progressive discovery instead.

### How It Works

Compacted mode uses a three-tier discovery model:

1. **Tier 1 -- High-level description (always in context).** You describe what the MCP server does in `system_prompt` (LLM nodes) or `description` (superagent skills). No tool schemas are injected automatically.

2. **Tier 2 -- Tool listing (on-demand).** Two built-in introspection tools become available:
   - `_list_mcp_tools(mcp_url)` -- Returns tool names only. Very cheap (~1 token per tool).
   - `_describe_mcp_tool(mcp_url, tool_name)` -- Returns the full description and input schema for one tool.

3. **Tier 3 -- Full schemas (explicit allowlist).** Tools listed in the `tools` field get full callable schemas in context, just like non-compacted mode.

### When to Use Compacted Mode

| Scenario | Recommendation |
|----------|---------------|
| MCP server with 1-5 tools | Use default auto-discover -- overhead is minimal |
| MCP server with 10+ tools, node uses 2-3 | Use `compacted: true` -- saves thousands of tokens |
| Multiple MCP servers, mixed sizes | Mix compacted and non-compacted per route |

### Configuration on LLM Nodes

Add `compacted: true` to any `tool_routing` entry:

```yaml
- id: research-agent
  type: llm
  config:
    provider: anthropic
    model: claude-sonnet-4-20250514
    api_key_ref: anthropic_key
    system_prompt: |
      You have access to a knowledge base MCP server with tools for searching,
      retrieving, and summarizing documents. Use _list_mcp_tools to discover
      available tools, and _describe_mcp_tool to learn their schemas before calling.
    user_prompt: "{{input.query}}"
    tool_loop: true
    tool_routing:
      search_kb:
        mcp_url: http://kb-mcp:9000/mcp
        compacted: true
      simple_echo:
        mcp_url: http://simple-mcp:9000/mcp
        # compacted not set -- default auto-discover
```

In this example, `search_kb`'s MCP server uses compacted mode (schemas not auto-discovered), while `simple_echo`'s server uses normal auto-discover.

### Configuration on Superagent Nodes

Add `compacted: true` to any skill:

```yaml
- id: agent
  type: superagent
  config:
    prompt: "Research the topic using available tools."
    skills:
      - name: knowledge-base
        description: "Search and retrieve from the knowledge base"
        mcp_url: http://kb-mcp:9000/mcp
        compacted: true
        tools: [search_kb, get_document]  # Only these get full schemas
      - name: calculator
        mcp_url: http://calc-mcp:9000/mcp
        description: "Simple calculator"
        # compacted not set -- all tools auto-discovered
```

When `compacted: true` is set on a skill:
- Only tools listed in `tools` get full callable schemas in context
- If `tools` is empty, no tool schemas are included -- the LLM discovers tools entirely via `_list_mcp_tools` and `_describe_mcp_tool`
- The `description` field becomes the LLM's primary context about the server's capabilities

### Behavior Matrix

| `compacted` | Behavior |
|-------------|----------|
| not set / `false` (default) | Auto-discover all tools from MCP server. All schemas in context. No change from existing behavior. |
| `true` | Skip auto-discover. Only tools in explicit `tools` list get full schemas. `_list_mcp_tools` and `_describe_mcp_tool` injected for on-demand discovery. |

### Token Savings

| Scenario | Default (auto-discover) | Compacted mode |
|----------|------------------------|----------------|
| 20-tool MCP server, node needs 2 | ~2,000 tokens (all schemas) | ~250 tokens (2 schemas + 2 introspection tools) |
| LLM discovers it needs a 3rd tool | Already has schema | Calls `_describe_mcp_tool` on-demand (~50 tokens) |

---

## Safety Limits

Tool loops include built-in safety limits to prevent runaway execution:

| Setting | Default | Description |
|---------|---------|-------------|
| `max_tool_calls` | 25 | Maximum total tool calls across all iterations. |
| `max_loop_iterations` | 10 | Maximum LLM round-trips in the loop. |

When a limit is reached, the loop terminates gracefully. The `finish_reason` output port indicates why:

| Finish Reason | Meaning |
|---------------|---------|
| `stop` | The LLM finished naturally (no more tool calls). |
| `max_tool_calls` | The total tool call limit was reached. |
| `max_iterations` | The iteration limit was reached. |

### Choosing Limits

- For simple lookup agents (1-3 tools), `max_tool_calls: 10` and `max_loop_iterations: 5` is typically sufficient.
- For complex research agents that may chain many tool calls, increase to `max_tool_calls: 50` and `max_loop_iterations: 15`.
- Always set limits. Unbounded loops risk high LLM costs and long execution times.

---

## Troubleshooting

### Tool Not Found

**Symptom:** The LLM requests a tool call, but the engine returns an error: `tool not found in routing table`.

**Cause:** The tool name in the LLM's response does not match any key in `tool_routing`.

**Fix:**
- Verify that the `name` in your `tools` array exactly matches the key in `tool_routing`.
- If using auto-discovery, confirm the MCP server's `tools/list` response includes the tool.
- Check for typos and case sensitivity -- tool names are case-sensitive.

### MCP Server Timeout

**Symptom:** Tool calls fail with a timeout error.

**Cause:** The MCP server took longer than `timeout_seconds` to respond.

**Fix:**
- Increase `timeout_seconds` in the tool route configuration (default is 30 seconds).
- Check the MCP server's health and response times.
- For long-running tools, consider breaking them into smaller operations.

### Safety Limit Reached

**Symptom:** The `finish_reason` output is `max_tool_calls` or `max_iterations` instead of `stop`.

**Cause:** The LLM kept requesting tool calls beyond the configured limits.

**Fix:**
- Review the conversation history (`messages` output port) to understand why the LLM kept looping.
- Improve the system prompt to give clearer instructions about when to stop.
- If the task genuinely requires more tool calls, increase `max_tool_calls` or `max_loop_iterations`.
- Consider whether the tool responses are giving the LLM enough information to reach a conclusion.

### MCP Connection Refused

**Symptom:** Tool calls fail with a connection error.

**Cause:** The MCP server is not running or not reachable at the configured URL.

**Fix:**
- Verify the MCP server is running and listening on the expected port.
- Check that the `mcp_url` is correct and reachable from the Brockley engine.
- If using Docker, ensure the containers are on the same network.

### Tool Returns Error

**Symptom:** The tool executes but returns an error result. The LLM receives the error and may retry or adjust its approach.

**Cause:** The MCP tool itself failed (e.g., invalid arguments, external API error).

**Fix:**
- Check the tool call arguments in the `tool_call_history` output.
- Review the MCP server logs for error details.
- Improve tool descriptions so the LLM passes valid arguments.

## See Also

- [LLM Node Reference](../nodes/llm.md) -- full config reference for LLM nodes
- [Tool Node Reference](../nodes/tool.md) -- standalone MCP tool calls
- [API Tool Node Reference](../nodes/api-tool.md) -- standalone REST API calls
- [Nodes Overview](../concepts/nodes.md) -- all node types including tool and api_tool
- [API Tools Guide](api-tools.md) -- REST APIs as tools without MCP wrappers
- [Superagent Tutorial](superagent-tutorial.md) -- autonomous agents with tools
- [Data Model](../specs/data-model.md) -- tool calling types (LLMToolDefinition, ToolRoute)
- [Common Errors](../troubleshooting/common-errors.md) -- provider and timeout errors

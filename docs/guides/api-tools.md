# API Tools Guide

API tools let you use REST APIs as tools in Brockley graphs -- without building MCP servers. You describe your API endpoints (method, path, input schema, auth), and Brockley handles the HTTP calls. Each endpoint becomes a discrete tool that LLM nodes and superagent nodes can invoke via function calling, or that you can call directly from a standalone `api_tool` node.

---

## API Tools vs MCP Tools

| | API Tools | MCP Tools |
|---|-----------|-----------|
| **What you provide** | JSON definition of your REST API | A running MCP server |
| **Protocol** | Direct HTTP (REST) | JSON-RPC 2.0 over MCP |
| **Code required** | None -- declarative | MCP server implementation |
| **Token efficiency** | Only selected endpoints appear in LLM context | All tools from a server are discovered at once |
| **Best for** | Existing REST APIs, third-party APIs | Custom tool logic, stateful tools, complex I/O |

Use API tools when you already have a REST API. Use MCP tools when you need custom server-side logic or bidirectional communication.

---

## Creating API Tool Definitions

An API tool definition is a reusable library resource that catalogs REST endpoints with shared config (base URL, auth headers, retry policy). Individual endpoints are selected per-node -- the definition itself is just a registry.

### JSON Definition

```json
{
  "name": "stripe-api",
  "namespace": "payments",
  "description": "Stripe payment processing API",
  "base_url": "https://api.stripe.com/v1",
  "default_headers": [
    {"name": "Authorization", "secret_ref": "stripe_api_key"},
    {"name": "Content-Type", "value": "application/json"}
  ],
  "default_timeout_ms": 30000,
  "retry": {
    "max_retries": 3,
    "backoff_ms": 1000,
    "retry_on_status": [429, 500, 502, 503]
  },
  "endpoints": [
    {
      "name": "get_customer",
      "description": "Retrieve a customer by their Stripe ID.",
      "method": "GET",
      "path": "/customers/{{input.customer_id}}",
      "input_schema": {
        "type": "object",
        "properties": {
          "customer_id": {"type": "string", "description": "Stripe customer ID (cus_...)"}
        },
        "required": ["customer_id"]
      }
    },
    {
      "name": "create_charge",
      "description": "Create a payment charge.",
      "method": "POST",
      "path": "/charges",
      "input_schema": {
        "type": "object",
        "properties": {
          "amount": {"type": "integer", "description": "Amount in cents"},
          "currency": {"type": "string", "description": "Three-letter currency code"}
        },
        "required": ["amount", "currency"]
      }
    }
  ]
}
```

### CLI

```bash
# Create from a JSON file
brockley api-tool create -f stripe-api.json

# List definitions
brockley api-tool list

# Get a specific definition
brockley api-tool get stripe-api

# Update
brockley api-tool update stripe-api -f stripe-api-updated.json

# Delete
brockley api-tool delete stripe-api
```

### Terraform

```hcl
resource "brockley_api_tool" "stripe" {
  name        = "stripe-api"
  namespace   = "payments"
  description = "Stripe payment processing API"
  base_url    = "https://api.stripe.com/v1"

  default_header {
    name       = "Authorization"
    secret_ref = "stripe_api_key"
  }

  default_header {
    name  = "Content-Type"
    value = "application/json"
  }

  retry {
    max_retries     = 3
    backoff_ms      = 1000
    retry_on_status = [429, 500, 502, 503]
  }

  endpoint {
    name        = "get_customer"
    description = "Retrieve a customer by their Stripe ID."
    method      = "GET"
    path        = "/customers/{{input.customer_id}}"
    input_schema = jsonencode({
      type = "object"
      properties = {
        customer_id = { type = "string", description = "Stripe customer ID" }
      }
      required = ["customer_id"]
    })
  }

  endpoint {
    name        = "create_charge"
    description = "Create a payment charge."
    method      = "POST"
    path        = "/charges"
    input_schema = jsonencode({
      type = "object"
      properties = {
        amount   = { type = "integer", description = "Amount in cents" }
        currency = { type = "string", description = "Three-letter currency code" }
      }
      required = ["amount", "currency"]
    })
  }
}
```

---

## Using API Tools on LLM Nodes

LLM nodes can invoke API endpoints as tools during function calling (with or without tool loop). Use the `api_tools` array on the LLM node config to select specific endpoints from a definition.

### Selecting Endpoints

```json
{
  "id": "billing-agent",
  "type": "llm",
  "config": {
    "provider": "anthropic",
    "model": "claude-sonnet-4-20250514",
    "api_key_ref": "anthropic_key",
    "system_prompt": "You are a billing assistant. Use tools to look up customer data and process charges.",
    "user_prompt": "{{input.request}}",
    "variables": [{"name": "request", "schema": {"type": "string"}}],
    "tool_loop": true,
    "api_tools": [
      {"api_tool_id": "stripe-api", "endpoint": "get_customer"},
      {"api_tool_id": "stripe-api", "endpoint": "create_charge"}
    ]
  }
}
```

Each `api_tools` entry selects one endpoint. The engine auto-derives the LLM tool schema (name, description, parameters) from the endpoint definition and creates the corresponding tool routing entry. The LLM sees these as normal function-calling tools.

### How Auto-Derive Works

For each `api_tools` entry, the engine:

1. Loads the API tool definition from the library store.
2. Finds the named endpoint.
3. Builds a tool definition with `name` = endpoint name (or `tool_name` override), `description` = endpoint description, and `parameters` = endpoint `input_schema`.
4. Creates a `ToolRoute` entry pointing to the API tool dispatcher instead of an MCP server.

These derived tools are merged with any explicit `tools` and `tool_routing` on the same node. You can mix API tools and MCP tools on one LLM node.

### Mixing with MCP Tools

```json
{
  "config": {
    "tool_loop": true,
    "tools": [
      {"name": "search_kb", "description": "Search knowledge base", "parameters": {"type": "object", "properties": {"query": {"type": "string"}}, "required": ["query"]}}
    ],
    "tool_routing": {
      "search_kb": {"mcp_url": "http://kb-mcp:9000/mcp"}
    },
    "api_tools": [
      {"api_tool_id": "stripe-api", "endpoint": "get_customer"}
    ]
  }
}
```

The LLM sees `search_kb` (MCP) and `get_customer` (API) as peers. Tool calls are routed to the correct backend automatically.

---

## Using API Tools on Superagent Nodes

Superagent nodes use API tools via skills. Set `api_tool_id` and `endpoints` on a skill instead of `mcp_url`.

```json
{
  "id": "billing-agent",
  "type": "superagent",
  "config": {
    "prompt": "Handle this billing request: {{input.request}}",
    "provider": "anthropic",
    "model": "claude-sonnet-4-20250514",
    "api_key_ref": "anthropic_key",
    "skills": [
      {
        "name": "stripe",
        "description": "Stripe payment processing -- look up customers and create charges.",
        "api_tool_id": "stripe-api",
        "endpoints": ["get_customer", "create_charge"]
      },
      {
        "name": "knowledge_base",
        "description": "Search the support knowledge base.",
        "mcp_url": "http://kb-mcp:9000/mcp"
      }
    ]
  }
}
```

Each endpoint listed in `endpoints` becomes a tool available to the agent. The skill's `description` is included in the system prompt, giving the agent context about what the API can do.

---

## Using API Tools as Standalone Nodes

The `api_tool` node type calls a single API endpoint directly, without an LLM. Use this for deterministic API calls in a graph -- no function calling, no LLM, just a straight HTTP call.

### Referenced

```json
{
  "id": "get-customer",
  "type": "api_tool",
  "input_ports": [
    {"name": "customer_id", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {"name": "result", "schema": {"type": "object"}}
  ],
  "config": {
    "api_tool_id": "stripe-api",
    "endpoint": "get_customer"
  }
}
```

### Inline

For self-contained graphs that don't need a library definition:

```json
{
  "id": "get-user",
  "type": "api_tool",
  "input_ports": [
    {"name": "user_id", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {"name": "result", "schema": {"type": "object"}}
  ],
  "config": {
    "inline_endpoint": {
      "base_url": "https://api.example.com",
      "method": "GET",
      "path": "/users/{{input.user_id}}"
    }
  }
}
```

See the [api_tool node reference](../nodes/api-tool.md) for full config options.

---

## Token Efficiency

API tools are designed to be token-efficient compared to MCP tools.

**MCP tools** use auto-discovery: the engine calls `tools/list` on each MCP server and sends all tool schemas to the LLM. If an MCP server exposes 50 tools, all 50 schemas appear in context -- even if the LLM only needs 2.

**API tools** use explicit selection: you choose exactly which endpoints appear as tools on each node. Only those endpoints' schemas are sent to the LLM.

For small APIs (1-5 tools), the difference is negligible. For large APIs (50+ endpoints), API tools can save thousands of context tokens.

---

## Three-Tier Progressive Discovery

For superagent nodes with large API surface areas, API tools support three-tier progressive discovery. This gives the agent awareness of all capabilities without paying upfront token costs for every endpoint schema.

### Tier 1: Skill Description (Always Present)

The skill's `description` field is included in the system prompt. Write it to give a high-level overview:

```json
{
  "name": "stripe",
  "description": "Stripe payment API -- manage customers, charges, subscriptions, and refunds.",
  "api_tool_id": "stripe-api",
  "endpoints": ["get_customer", "create_charge", "list_subscriptions", "create_refund"]
}
```

### Tier 2: List Endpoint Names (`_list_api_tools`)

The superagent's built-in `_list_api_tools` tool returns all available endpoint names grouped by skill. This costs minimal tokens -- just names, no schemas.

```
Agent: I need to find a specific Stripe endpoint. Let me check what's available.
Tool call: _list_api_tools({})
Result: {"stripe": ["get_customer", "create_charge", "list_subscriptions", "create_refund"]}
```

### Tier 3: Full Schema (`_describe_api_tool`)

The `_describe_api_tool` built-in returns the full schema for a specific endpoint. The agent calls this only when it needs the details.

```
Agent: I need to create a refund. Let me get the schema.
Tool call: _describe_api_tool({"tool_name": "create_refund"})
Result: {"name": "create_refund", "description": "Create a refund for a charge.", "method": "POST", "path": "/refunds", "input_schema": {"type": "object", "properties": {"charge": {"type": "string"}, "amount": {"type": "integer"}}, "required": ["charge"]}}
```

This three-tier approach means an agent with access to 100 endpoints only loads full schemas for the 2-3 it actually needs.

---

## Examples

### Full Pipeline: Lookup + Transform + Output

A graph that looks up a customer, extracts their email, and formats a greeting:

```json
{
  "nodes": [
    {
      "id": "input-1",
      "type": "input",
      "output_ports": [{"name": "customer_id", "schema": {"type": "string"}}]
    },
    {
      "id": "lookup",
      "type": "api_tool",
      "input_ports": [{"name": "customer_id", "schema": {"type": "string"}}],
      "output_ports": [{"name": "result", "schema": {"type": "object"}}],
      "config": {
        "api_tool_id": "stripe-api",
        "endpoint": "get_customer"
      }
    },
    {
      "id": "format",
      "type": "transform",
      "input_ports": [{"name": "customer", "schema": {"type": "object"}}],
      "output_ports": [{"name": "greeting", "schema": {"type": "string"}}],
      "config": {
        "expressions": {
          "greeting": "\"Hello, \" + input.customer.name + \"! Your email is \" + input.customer.email"
        }
      }
    },
    {
      "id": "output-1",
      "type": "output",
      "input_ports": [{"name": "greeting", "schema": {"type": "string"}}],
      "output_ports": [{"name": "greeting", "schema": {"type": "string"}}]
    }
  ],
  "edges": [
    {"source_node_id": "input-1", "source_port": "customer_id", "target_node_id": "lookup", "target_port": "customer_id"},
    {"source_node_id": "lookup", "source_port": "result", "target_node_id": "format", "target_port": "customer"},
    {"source_node_id": "format", "source_port": "greeting", "target_node_id": "output-1", "target_port": "greeting"}
  ]
}
```

### Billing Agent with API + MCP Tools

An LLM agent that uses Stripe (API tool) for payment operations and an MCP knowledge base for policy lookups:

```json
{
  "nodes": [
    {
      "id": "input-1",
      "type": "input",
      "output_ports": [{"name": "request", "schema": {"type": "string"}}]
    },
    {
      "id": "agent",
      "type": "llm",
      "input_ports": [{"name": "request", "schema": {"type": "string"}}],
      "output_ports": [
        {"name": "response_text", "schema": {"type": "string"}},
        {"name": "tool_call_history", "schema": {"type": "array"}}
      ],
      "config": {
        "provider": "anthropic",
        "model": "claude-sonnet-4-20250514",
        "api_key_ref": "anthropic_key",
        "system_prompt": "You are a billing support agent. Look up customer info and refund policies before answering.",
        "user_prompt": "{{input.request}}",
        "variables": [{"name": "request", "schema": {"type": "string"}}],
        "tool_loop": true,
        "max_tool_calls": 10,
        "api_tools": [
          {"api_tool_id": "stripe-api", "endpoint": "get_customer"},
          {"api_tool_id": "stripe-api", "endpoint": "create_charge"}
        ],
        "tools": [
          {"name": "search_policy", "description": "Search refund and billing policies", "parameters": {"type": "object", "properties": {"query": {"type": "string"}}, "required": ["query"]}}
        ],
        "tool_routing": {
          "search_policy": {"mcp_url": "http://kb-mcp:9000/mcp"}
        }
      }
    },
    {
      "id": "output-1",
      "type": "output",
      "input_ports": [{"name": "reply", "schema": {"type": "string"}}],
      "output_ports": [{"name": "reply", "schema": {"type": "string"}}]
    }
  ],
  "edges": [
    {"source_node_id": "input-1", "source_port": "request", "target_node_id": "agent", "target_port": "request"},
    {"source_node_id": "agent", "source_port": "response_text", "target_node_id": "output-1", "target_port": "reply"}
  ]
}
```

## See Also

- [API Tool Node Reference](../nodes/api-tool.md) -- standalone api_tool node config
- [Tool Calling Guide](tool-calling.md) -- MCP tools and tool loop
- [Data Model](../specs/data-model.md) -- APIToolDefinition, APIEndpoint types
- [API Design](../specs/api-design.md) -- API tool definition endpoints
```

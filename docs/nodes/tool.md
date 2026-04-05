# Tool Node (MCP)

**Type:** `tool`

The tool node calls an external tool via the [Model Context Protocol](https://modelcontextprotocol.io/) (MCP). It connects to an MCP server, discovers the tool's input schema, and invokes the tool with arguments received from upstream nodes.

## Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `tool_name` | string | Yes | The name of the tool to invoke on the MCP server. |
| `mcp_url` | string | Yes | URL of the MCP server endpoint. |
| `mcp_transport` | string | No | Transport protocol: `"sse"` (default) or `"stdio"`. |
| `headers` | HeaderConfig[] | No | Custom HTTP headers sent with every request to the MCP server. |

## Headers

Each header supports three modes for setting its value. Exactly one of `value`, `from_input`, or `secret_ref` must be set per header.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | HTTP header name (e.g., `Authorization`, `X-Api-Key`). |
| `value` | string | No | **Static** -- header value set directly in the config. |
| `from_input` | string | No | **Dynamic** -- reads the header value from the named input port at runtime. |
| `secret_ref` | string | No | **Secret** -- resolved via the secret store at runtime (same mechanism as `api_key_ref`). |

### Static Headers

The value is hardcoded in the graph definition. Good for fixed identifiers or non-sensitive configuration:

```json
{"name": "X-Database", "value": "analytics"}
```

### Dynamic Headers from Input Ports

The header value comes from an upstream node at runtime. This is the primary way to pass per-user or per-request auth tokens through a graph:

```json
{"name": "Authorization", "from_input": "user_token"}
```

At runtime, the engine reads the value of the `user_token` input port and uses it as the `Authorization` header. The input port must be declared on the node and wired from upstream.

### Secret Reference Headers

The header value is resolved from the secret store. With the default environment-based secret store, `"mcp-service-key"` maps to the environment variable `BROCKLEY_SECRET_MCP_SERVICE_KEY`:

```json
{"name": "X-Api-Key", "secret_ref": "mcp-service-key"}
```

Secrets never appear in the graph definition or API responses.

## How Tool Schemas Are Discovered

The MCP client communicates with the MCP server using JSON-RPC 2.0. When the tool node executes:

1. The client sends a `tools/list` request to the MCP server.
2. The server responds with all available tools, including each tool's `name`, `description`, and `inputSchema`.
3. The engine matches the configured `tool_name` against the listed tools.
4. The tool's `inputSchema` determines what arguments the tool expects.
5. The engine sends a `tools/call` request with the tool name and arguments built from the node's input port values.

## Input and Output Ports

**Input ports** are defined by the graph author. They should correspond to the arguments the MCP tool expects. Values received on input ports are passed as the `arguments` map in the `tools/call` request.

**Output ports:**

| Port | Type | Description |
|------|------|-------------|
| `result` | any | The content returned by the tool. For single text responses, this is a string. For multi-block responses, this is an array of `{type, text}` objects. |

If the tool returns an error, the node fails with the error message from the MCP server.

## Examples

### Basic Tool Call

```json
{
  "id": "search-tool",
  "name": "Web Search",
  "type": "tool",
  "input_ports": [
    {"name": "query", "schema": {"type": "string"}},
    {"name": "max_results", "schema": {"type": "integer"}}
  ],
  "output_ports": [
    {"name": "result", "schema": {"type": "string"}}
  ],
  "config": {
    "tool_name": "web_search",
    "mcp_url": "http://localhost:3001/mcp",
    "mcp_transport": "sse"
  }
}
```

### Dynamic Auth via Input Port

Pass a per-user OAuth token from the graph input through to the MCP server:

```json
{
  "id": "user-api",
  "name": "User API Call",
  "type": "tool",
  "input_ports": [
    {"name": "endpoint", "schema": {"type": "string"}},
    {"name": "user_token", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {"name": "result", "schema": {"type": "string"}}
  ],
  "config": {
    "tool_name": "http_request",
    "mcp_url": "http://http-mcp:3000/mcp",
    "headers": [
      {"name": "Authorization", "from_input": "user_token"}
    ]
  }
}
```

The graph's input node exposes a `user_token` port. The caller provides the token at execution time, and it flows through to the MCP server as the `Authorization` header.

### Secret-Based Auth

```json
{
  "id": "secure-tool",
  "name": "Secure Tool",
  "type": "tool",
  "input_ports": [
    {"name": "data", "schema": {"type": "object"}}
  ],
  "output_ports": [
    {"name": "result", "schema": {"type": "string"}}
  ],
  "config": {
    "tool_name": "process_data",
    "mcp_url": "http://secure-mcp:8443/mcp",
    "headers": [
      {"name": "X-Api-Key", "secret_ref": "mcp-service-key"}
    ]
  }
}
```

### Multiple Headers (Mixed Modes)

You can combine all three header modes on the same node:

```json
{
  "config": {
    "tool_name": "query",
    "mcp_url": "http://db-mcp:8080/mcp",
    "headers": [
      {"name": "X-Database", "value": "analytics"},
      {"name": "Authorization", "from_input": "user_token"},
      {"name": "X-Internal-Key", "secret_ref": "db-internal-key"}
    ]
  }
}
```

### MCP Protocol Details

The tool node uses JSON-RPC 2.0 over HTTP. A `tools/call` request looks like:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "web_search",
    "arguments": {
      "query": "Brockley AI platform",
      "max_results": 5
    }
  }
}
```

A successful response:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {"type": "text", "text": "Search results: ..."}
    ],
    "isError": false
  }
}
```

## See Also

- [API Tool Node](api-tool.md) -- call REST APIs directly without an MCP wrapper
- [LLM Node](llm.md) -- tool calling from within LLM responses
- [Tool Calling Guide](../guides/tool-calling.md) -- patterns for tool integration
- [Superagent Node](superagent.md) -- autonomous agent with tool access
- [Data Model: Tool Node Config](../specs/data-model.md) -- complete field reference

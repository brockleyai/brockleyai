# API Tool Node

**Type:** `api_tool`

The API tool node calls a single REST API endpoint directly from a graph. It works like the [tool node](tool.md) (which calls MCP tools), but targets HTTP APIs described by API tool definitions instead of MCP servers. No MCP wrapper is needed -- describe your API once and Brockley handles the HTTP call.

For using API tools with LLM nodes and Superagent nodes, see the [API Tools Guide](../guides/api-tools.md).

## Configuration

The node supports two modes: **referenced** (points to a library definition) and **inline** (self-contained endpoint config). Exactly one must be set.

### Referenced Mode

Points to a pre-created API tool definition (via CLI, API, or Terraform):

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `api_tool_id` | string | Yes | ID or name of the API tool definition (library resource). |
| `endpoint` | string | Yes | Name of the endpoint within that definition. |
| `headers` | HeaderConfig[] | No | Per-node header overrides, merged on top of the definition's defaults. |

The engine looks up the definition, finds the named endpoint, and makes the HTTP request using the definition's base URL, default headers, and endpoint configuration.

### Inline Mode

Self-contained endpoint definition, no library resource needed:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `inline_endpoint` | InlineAPIEndpoint | Yes | Self-contained endpoint definition. |
| `headers` | HeaderConfig[] | No | Per-node header overrides. |

### InlineAPIEndpoint

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `base_url` | string | Yes | Base URL of the API (e.g., `https://api.example.com`). |
| `method` | string | Yes | HTTP method: `GET`, `POST`, `PUT`, `PATCH`, `DELETE`. |
| `path` | string | Yes | URL path. Supports `{{input.x}}` template expressions for path parameters. |
| `default_headers` | HeaderConfig[] | No | Default headers sent with the request. |
| `input_schema` | JSON Schema | No | Schema describing the endpoint's expected input. |
| `output_schema` | JSON Schema | No | Schema describing the expected response. |
| `request_mapping` | RequestMapping | No | How input fields map to the HTTP request. |
| `response_mapping` | ResponseMapping | No | How the HTTP response maps to tool output. |
| `retry` | RetryConfig | No | Retry configuration for transient failures. |
| `timeout_ms` | integer | No | Request timeout in milliseconds. |

### RequestMapping

Controls how input port values are sent in the HTTP request:

| Mode | Behavior |
|------|----------|
| `json_body` | Input fields are sent as a JSON request body (default). |
| `form` | Input fields are sent as `application/x-www-form-urlencoded`. |
| `query_params` | Input fields are sent as URL query parameters. |
| `path_and_body` | Path template params extracted from input; remaining fields sent as JSON body. |

### ResponseMapping

Controls how the HTTP response is parsed:

| Mode | Behavior |
|------|----------|
| `json_body` | Parse the response body as JSON (default). |
| `text` | Return the response body as a plain string. |
| `jq` | Apply a jq-like expression to the JSON response. Requires `expression` field. |
| `headers_and_body` | Return both headers and body as a structured object. |

### RetryConfig

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `max_retries` | integer | Yes | Maximum retry attempts. |
| `backoff_ms` | integer | Yes | Initial backoff delay in milliseconds. |
| `retry_on_status` | integer[] | No | HTTP status codes that trigger retry (e.g., `[429, 500, 502, 503]`). |

## Input and Output Ports

**Input ports** are defined by the graph author. They should correspond to the fields in the endpoint's `input_schema`. Values received on input ports populate path templates, query parameters, or the request body (depending on `request_mapping`).

**Output ports:**

| Port | Type | Description |
|------|------|-------------|
| `result` | any | The parsed response from the API endpoint. |

If the API returns an HTTP error, the node fails with the error details.

## Examples

### Referenced: Call a Library-Defined Endpoint

```json
{
  "id": "get-customer",
  "name": "Get Customer",
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

### Inline: Self-Contained POST Endpoint

```json
{
  "id": "create-charge",
  "name": "Create Charge",
  "type": "api_tool",
  "input_ports": [
    {"name": "amount", "schema": {"type": "integer"}},
    {"name": "currency", "schema": {"type": "string"}}
  ],
  "output_ports": [
    {"name": "result", "schema": {"type": "object"}}
  ],
  "config": {
    "inline_endpoint": {
      "base_url": "https://api.stripe.com/v1",
      "method": "POST",
      "path": "/charges",
      "default_headers": [
        {"name": "Authorization", "secret_ref": "stripe_key"}
      ],
      "input_schema": {
        "type": "object",
        "properties": {
          "amount": {"type": "integer"},
          "currency": {"type": "string"}
        },
        "required": ["amount", "currency"]
      }
    }
  }
}
```

### Inline: GET with Query Parameters

```json
{
  "id": "search",
  "name": "Search API",
  "type": "api_tool",
  "input_ports": [
    {"name": "q", "schema": {"type": "string"}},
    {"name": "limit", "schema": {"type": "integer"}}
  ],
  "output_ports": [
    {"name": "result", "schema": {"type": "object"}}
  ],
  "config": {
    "inline_endpoint": {
      "base_url": "https://api.example.com",
      "method": "GET",
      "path": "/search",
      "request_mapping": {"mode": "query_params"},
      "input_schema": {
        "type": "object",
        "properties": {
          "q": {"type": "string"},
          "limit": {"type": "integer"}
        },
        "required": ["q"]
      }
    }
  }
}
```

At runtime, the engine builds `GET https://api.example.com/search?q=hello&limit=10` from the input port values.

### Inline: Path Parameters

```json
{
  "id": "get-user",
  "name": "Get User",
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

### Inline: With Retry and Custom Response Mapping

```json
{
  "config": {
    "inline_endpoint": {
      "base_url": "https://api.example.com",
      "method": "POST",
      "path": "/process",
      "retry": {
        "max_retries": 3,
        "backoff_ms": 1000,
        "retry_on_status": [429, 500, 502, 503]
      },
      "response_mapping": {
        "mode": "jq",
        "expression": ".data.results"
      },
      "timeout_ms": 10000
    }
  }
}
```

### Referenced: With Header Overrides

Add graph-specific headers on top of the definition's defaults:

```json
{
  "config": {
    "api_tool_id": "my-api",
    "endpoint": "search",
    "headers": [
      {"name": "X-Custom-Header", "value": "graph-specific-value"},
      {"name": "Authorization", "from_input": "user_token"}
    ]
  }
}
```

Headers use the same three modes as the [tool node](tool.md#headers): `value` (static), `from_input` (dynamic from input port), and `secret_ref` (from secret store).

## See Also

- [API Tools Guide](../guides/api-tools.md) -- creating and managing API tool definitions
- [Tool Node](tool.md) -- MCP tool calls (for tools behind MCP servers)
- [LLM Node](llm.md) -- API tool references in LLM tool calling (`api_tools` field)
- [Superagent Node](superagent.md) -- API tool skills in autonomous agents
- [Data Model: API Tool Definitions](../specs/data-model.md) -- complete API tool type reference

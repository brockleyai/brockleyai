# Terraform Resources

## brockley_graph

Manages a Brockley agent graph.

### Arguments

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | Graph name |
| `namespace` | string | No | Namespace (default: "default") |
| `description` | string | No | Description |
| `status` | string | No | Status: draft, active, archived |
| `nodes` | string | Yes | JSON-encoded array of node definitions |
| `edges` | string | No | JSON-encoded array of edge definitions |
| `state` | string | No | JSON-encoded graph state definition |

### Read-Only

| Name | Type | Description |
|------|------|-------------|
| `id` | string | Graph ID |
| `version` | number | Version (auto-incremented) |

### Example

```hcl
resource "brockley_graph" "classifier" {
  name        = "customer-classifier"
  namespace   = "production"
  description = "Classifies customer tickets"

  nodes = jsonencode([
    {
      id           = "input-1"
      name         = "Input"
      type         = "input"
      input_ports  = [{ name = "text", schema = { type = "string" } }]
      output_ports = [{ name = "text", schema = { type = "string" } }]
      config       = {}
    }
  ])

  edges = jsonencode([])
}
```

---

## brockley_schema

Manages a Brockley library schema.

### Arguments

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | Schema name |
| `namespace` | string | No | Namespace |
| `description` | string | No | Description |
| `json_schema` | string | Yes | JSON Schema definition (JSON-encoded) |

### Example

```hcl
resource "brockley_schema" "ticket" {
  name = "ticket-input"

  json_schema = jsonencode({
    type = "object"
    properties = {
      id      = { type = "string" }
      subject = { type = "string" }
    }
    required = ["id", "subject"]
  })
}
```

---

## brockley_prompt_template

Manages a Brockley prompt template.

### Arguments

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | Template name |
| `namespace` | string | No | Namespace |
| `description` | string | No | Description |
| `system_prompt` | string | No | System prompt text |
| `user_prompt` | string | Yes | User prompt template |
| `variables` | string | No | JSON-encoded variable definitions |
| `response_format` | string | No | Response format: text or json |

### Example

```hcl
resource "brockley_prompt_template" "classify" {
  name          = "classify-intent"
  system_prompt = "You are a classifier."
  user_prompt   = "Classify: {{subject}}"
}
```

---

## brockley_provider_config

Manages a Brockley LLM provider configuration.

### Arguments

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | Config name |
| `namespace` | string | No | Namespace |
| `provider_type` | string | Yes | Provider: openai, anthropic, google, openrouter, bedrock |
| `api_key_ref` | string | Yes | Secret store reference for the API key |
| `base_url` | string | No | Custom base URL |
| `default_model` | string | No | Default model |

### Example

```hcl
resource "brockley_provider_config" "anthropic" {
  name          = "anthropic-primary"
  provider_type = "anthropic"
  api_key_ref   = "anthropic-key"
  default_model = "claude-sonnet-4-20250514"
}
```

## brockley_api_tool

Manages a Brockley API tool definition.

### Arguments

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | API tool name |
| `namespace` | string | No | Namespace |
| `description` | string | No | Description |
| `base_url` | string | Yes | Base URL for all endpoints |
| `default_header` | block | No | Repeatable block: `name`, `value` or `secret_ref` |
| `retry` | block | No | Retry config: `max_retries`, `backoff_ms`, `retry_on_status` |
| `endpoint` | block | Yes | Repeatable block (see below) |

### Endpoint Block

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | Endpoint name (used as tool name) |
| `description` | string | Yes | What this endpoint does |
| `method` | string | Yes | HTTP method |
| `path` | string | Yes | URL path (supports `{{input.x}}` templates) |
| `input_schema` | string | Yes | JSON-encoded input schema |

### Example

```hcl
resource "brockley_api_tool" "stripe" {
  name     = "stripe-api"
  base_url = "https://api.stripe.com/v1"

  default_header {
    name       = "Authorization"
    secret_ref = "stripe_api_key"
  }

  endpoint {
    name        = "get_customer"
    description = "Retrieve a customer by ID"
    method      = "GET"
    path        = "/customers/{{input.customer_id}}"
    input_schema = jsonencode({
      type = "object"
      properties = {
        customer_id = { type = "string" }
      }
      required = ["customer_id"]
    })
  }
}
```

## See Also

- [Terraform Overview](overview.md) -- provider setup
- [Data Sources](data-sources.md) -- reading existing resources
- [Import Guide](import.md) -- importing existing resources
- [Examples](examples.md) -- complete configurations
```

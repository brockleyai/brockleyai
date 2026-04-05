# Terraform Examples

## Complete Agent Setup

```hcl
terraform {
  required_providers {
    brockley = {
      source = "brockleyai/brockley"
    }
  }
}

provider "brockley" {
  server_url = "http://localhost:8000"
  api_key    = var.brockley_api_key
}

variable "brockley_api_key" {
  type      = string
  sensitive = true
}

# Provider config for Anthropic
resource "brockley_provider_config" "anthropic" {
  name          = "anthropic-primary"
  provider_type = "anthropic"
  api_key_ref   = "anthropic-key"
  default_model = "claude-sonnet-4-20250514"
}

# Reusable schema
resource "brockley_schema" "ticket_input" {
  name = "ticket-input"

  json_schema = jsonencode({
    type = "object"
    properties = {
      ticket_id = { type = "string" }
      subject   = { type = "string" }
      body      = { type = "string" }
    }
    required = ["ticket_id", "subject", "body"]
  })
}

# Agent graph
resource "brockley_graph" "classifier" {
  name        = "ticket-classifier"
  namespace   = "production"
  description = "Classifies support tickets"
  status      = "active"

  nodes = jsonencode([
    {
      id           = "input-1"
      name         = "Input"
      type         = "input"
      input_ports  = [
        { name = "ticket", schema = { type = "object", properties = { subject = { type = "string" }, body = { type = "string" } }, required = ["subject", "body"] } }
      ]
      output_ports = [
        { name = "ticket", schema = { type = "object", properties = { subject = { type = "string" }, body = { type = "string" } }, required = ["subject", "body"] } }
      ]
      config = {}
    },
    {
      id           = "output-1"
      name         = "Output"
      type         = "output"
      input_ports  = [{ name = "result", schema = { type = "string" } }]
      output_ports = [{ name = "result", schema = { type = "string" } }]
      config       = {}
    }
  ])

  edges = jsonencode([])
}

output "graph_id" {
  value = brockley_graph.classifier.id
}
```

## API Tool with LLM Agent

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
    description = "Retrieve a customer by Stripe ID"
    method      = "GET"
    path        = "/customers/{{input.customer_id}}"
    input_schema = jsonencode({
      type = "object"
      properties = {
        customer_id = { type = "string", description = "Stripe customer ID (cus_...)" }
      }
      required = ["customer_id"]
    })
  }
}

resource "brockley_graph" "billing_agent" {
  name        = "billing-agent"
  namespace   = "production"
  description = "Handles billing inquiries using Stripe API"
  status      = "active"

  nodes = jsonencode([
    {
      id           = "input-1"
      name         = "Input"
      type         = "input"
      input_ports  = []
      output_ports = [{ name = "request", schema = { type = "string" } }]
      config       = {}
    },
    {
      id          = "agent"
      name        = "Billing Agent"
      type        = "llm"
      input_ports = [{ name = "request", schema = { type = "string" } }]
      output_ports = [
        { name = "response_text", schema = { type = "string" } }
      ]
      config = {
        provider      = "anthropic"
        model         = "claude-sonnet-4-20250514"
        api_key_ref   = "anthropic-key"
        system_prompt = "You are a billing support agent."
        user_prompt   = "{{input.request}}"
        variables     = [{ name = "request", schema = { type = "string" } }]
        tool_loop     = true
        api_tools = [
          { api_tool_id = "stripe-api", endpoint = "get_customer" }
        ]
      }
    },
    {
      id           = "output-1"
      name         = "Output"
      type         = "output"
      input_ports  = [{ name = "reply", schema = { type = "string" } }]
      output_ports = [{ name = "reply", schema = { type = "string" } }]
      config       = {}
    }
  ])

  edges = jsonencode([
    { id = "e1", source_node_id = "input-1", source_port = "request", target_node_id = "agent", target_port = "request" },
    { id = "e2", source_node_id = "agent", source_port = "response_text", target_node_id = "output-1", target_port = "reply" }
  ])
}
```

## See Also

- [Terraform Overview](overview.md) -- provider setup
- [Resources](resources.md) -- all resource types and attributes
- [API Tools Guide](../guides/api-tools.md) -- API tools in depth
- [CLI export](../cli/export.md) -- export graphs as Terraform HCL
```

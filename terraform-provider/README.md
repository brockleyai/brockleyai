# Brockley Terraform Provider

Terraform/OpenTofu provider for managing Brockley AI agent infrastructure.

## Requirements

- Terraform >= 1.0
- Go >= 1.24 (for building from source)

## Building

```bash
go build -o terraform-provider-brockley ./terraform-provider
```

## Usage

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
```

## Resources

### brockley_graph

```hcl
resource "brockley_graph" "classifier" {
  name        = "customer-classifier"
  namespace   = "production"
  description = "Classifies customer tickets"
  status      = "active"

  nodes = jsonencode([
    {
      id          = "input-1"
      name        = "Input"
      type        = "input"
      input_ports = [{ name = "text", schema = { type = "string" } }]
      output_ports = [{ name = "text", schema = { type = "string" } }]
      config      = {}
    }
  ])

  edges = jsonencode([])
}
```

### brockley_schema

```hcl
resource "brockley_schema" "ticket_input" {
  name        = "ticket-input"
  namespace   = "default"
  description = "Schema for support ticket input"

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
```

### brockley_prompt_template

```hcl
resource "brockley_prompt_template" "classify" {
  name          = "classify-intent"
  namespace     = "default"
  system_prompt = "You are a classifier."
  user_prompt   = "Classify: {{subject}}"

  variables = jsonencode([
    { name = "subject", schema = { type = "string" } }
  ])

  response_format = "json"
}
```

### brockley_provider_config

```hcl
resource "brockley_provider_config" "anthropic" {
  name          = "anthropic-primary"
  namespace     = "default"
  provider_type = "anthropic"
  api_key       = var.anthropic_api_key
  default_model = "claude-sonnet-4-20250514"
}
```

## Data Sources

All resources have corresponding data sources for reading:

```hcl
data "brockley_graph" "existing" {
  id = "graph_abc123"
}

data "brockley_schema" "existing" {
  id = "schema_001"
}
```

## Import

All resources support `terraform import`:

```bash
terraform import brockley_graph.classifier graph_abc123
terraform import brockley_schema.ticket_input schema_001
terraform import brockley_prompt_template.classify pt_001
terraform import brockley_provider_config.anthropic pc_001
```

## Configuration

| Attribute | Environment Variable | Description |
|-----------|---------------------|-------------|
| `server_url` | `BROCKLEY_SERVER_URL` | Brockley server URL |
| `api_key` | `BROCKLEY_API_KEY` | API key for authentication |

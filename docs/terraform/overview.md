# Terraform Provider Overview

The Brockley Terraform provider lets you manage agent graphs, schemas, prompt templates, and provider configs as infrastructure-as-code.

## Installation

```hcl
terraform {
  required_providers {
    brockley = {
      source = "brockleyai/brockley"
    }
  }
}
```

## Provider Configuration

```hcl
provider "brockley" {
  server_url = "http://localhost:8000"  # or BROCKLEY_SERVER_URL
  api_key    = var.brockley_api_key     # or BROCKLEY_API_KEY
}
```

## Resources

| Resource | Description |
|----------|-------------|
| [brockley_graph](resources.md#brockley_graph) | Agent graph definition |
| [brockley_schema](resources.md#brockley_schema) | Library JSON schema |
| [brockley_prompt_template](resources.md#brockley_prompt_template) | Prompt template |
| [brockley_provider_config](resources.md#brockley_provider_config) | LLM provider configuration |

## Data Sources

| Data Source | Description |
|-------------|-------------|
| [brockley_graph](data-sources.md#brockley_graph) | Read a graph |
| [brockley_schema](data-sources.md#brockley_schema) | Read a schema |
| [brockley_prompt_template](data-sources.md#brockley_prompt_template) | Read a prompt template |
| [brockley_provider_config](data-sources.md#brockley_provider_config) | Read a provider config |

## Import

All resources support `terraform import`. See [Import Guide](import.md).

## Authentication

The provider reads credentials from:

1. Provider block attributes (`server_url`, `api_key`)
2. Environment variables (`BROCKLEY_SERVER_URL`, `BROCKLEY_API_KEY`)

```hcl
# Using environment variables (recommended for CI)
provider "brockley" {}
```

```bash
export BROCKLEY_SERVER_URL=http://localhost:8000
export BROCKLEY_API_KEY=your-api-key
terraform plan
```

## See Also

- [Resources](resources.md) -- all resource types and attributes
- [Data Sources](data-sources.md) -- reading existing resources
- [Import Guide](import.md) -- importing existing resources
- [Examples](examples.md) -- complete Terraform configurations
- [CLI Overview](../cli/overview.md) -- CLI equivalents for graph management
- [API Overview](../api/overview.md) -- REST API equivalents
- [CLI export](../cli/export.md) -- export graphs as Terraform HCL

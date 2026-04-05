# Terraform Data Sources

Data sources allow reading existing Brockley resources without managing them.

## brockley_graph

```hcl
data "brockley_graph" "existing" {
  id = "graph_abc123"
}

output "graph_name" {
  value = data.brockley_graph.existing.name
}
```

### Attributes

| Name | Type | Description |
|------|------|-------------|
| `id` | string | Graph ID (required) |
| `name` | string | Graph name |
| `namespace` | string | Namespace |
| `description` | string | Description |
| `status` | string | Status |
| `version` | number | Version |
| `nodes` | string | JSON-encoded nodes |
| `edges` | string | JSON-encoded edges |

## brockley_schema

```hcl
data "brockley_schema" "existing" {
  id = "schema_001"
}
```

### Attributes

| Name | Type | Description |
|------|------|-------------|
| `id` | string | Schema ID (required) |
| `name` | string | Schema name |
| `namespace` | string | Namespace |
| `description` | string | Description |
| `json_schema` | string | JSON Schema definition |

## brockley_prompt_template

```hcl
data "brockley_prompt_template" "existing" {
  id = "pt_001"
}
```

### Attributes

| Name | Type | Description |
|------|------|-------------|
| `id` | string | Template ID (required) |
| `name` | string | Template name |
| `namespace` | string | Namespace |
| `description` | string | Description |
| `system_prompt` | string | System prompt |
| `user_prompt` | string | User prompt |
| `variables` | string | JSON-encoded variables |
| `response_format` | string | Response format |

## brockley_provider_config

```hcl
data "brockley_provider_config" "existing" {
  id = "pc_001"
}
```

### Attributes

| Name | Type | Description |
|------|------|-------------|
| `id` | string | Config ID (required) |
| `name` | string | Config name |
| `namespace` | string | Namespace |
| `provider_type` | string | Provider type |
| `base_url` | string | Base URL |
| `api_key_ref` | string | API key reference |
| `default_model` | string | Default model |

## See Also

- [Terraform Overview](overview.md) -- provider setup
- [Resources](resources.md) -- managing resources
- [Import Guide](import.md) -- importing existing resources

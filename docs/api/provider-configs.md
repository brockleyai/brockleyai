# Provider Configs API

Manage library LLM provider configurations. Provider configs are reusable building blocks that get copied into LLM node configs when building graphs. They are not referenced at runtime.

## Endpoints

```
POST   /api/v1/provider-configs              # Create
GET    /api/v1/provider-configs              # List
GET    /api/v1/provider-configs/{id}         # Get
PUT    /api/v1/provider-configs/{id}         # Update
DELETE /api/v1/provider-configs/{id}         # Delete
```

## List Provider Configs

```bash
curl http://localhost:8000/api/v1/provider-configs?namespace=default&provider=anthropic
```

Query parameters: `namespace`, `provider`, `limit`, `cursor`

Response: `200 OK`

```json
{
  "items": [
    {
      "id": "pc_001",
      "name": "anthropic-primary",
      "namespace": "default",
      "provider_type": "anthropic",
      "api_key_ref": "anthropic-key",
      "default_model": "claude-sonnet-4-20250514",
      "created_at": "2026-03-10T12:00:00Z",
      "updated_at": "2026-03-10T12:00:00Z"
    }
  ],
  "next_cursor": null,
  "has_more": false
}
```

The `api_key` field is never returned in GET responses. Only `api_key_ref` (the secret store reference) is visible.

## Create Provider Config

```bash
curl -X POST http://localhost:8000/api/v1/provider-configs \
  -H "Content-Type: application/json" \
  -d '{
    "name": "anthropic-primary",
    "namespace": "default",
    "provider_type": "anthropic",
    "api_key_ref": "anthropic-key",
    "default_model": "claude-sonnet-4-20250514"
  }'
```

Response: `201 Created`

## Get Provider Config

```bash
curl http://localhost:8000/api/v1/provider-configs/pc_001
```

Response: `200 OK`

## Update Provider Config

```bash
curl -X PUT http://localhost:8000/api/v1/provider-configs/pc_001 \
  -H "Content-Type: application/json" \
  -d '{ ... updated config ... }'
```

Response: `200 OK`

## Delete Provider Config

```bash
curl -X DELETE http://localhost:8000/api/v1/provider-configs/pc_001
```

Response: `204 No Content`

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Config name |
| `namespace` | string | No | Namespace (default: `"default"`) |
| `provider_type` | string | Yes | Provider: `openai`, `anthropic`, `google`, `openrouter`, `bedrock` |
| `api_key_ref` | string | Yes | Secret store reference for the API key |
| `base_url` | string | No | Custom base URL |
| `default_model` | string | No | Default model |

## See Also

- [API Overview](overview.md) -- authentication, pagination, error format
- [Providers Overview](../providers/overview.md) -- how providers work
- [Terraform Resources](../terraform/resources.md) -- `brockley_provider_config` resource

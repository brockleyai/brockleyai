# Prompt Templates API

Manage library prompt templates. Templates are reusable building blocks that get copied into LLM node configs when building graphs. They are not referenced at runtime.

## Endpoints

```
POST   /api/v1/prompt-templates              # Create
GET    /api/v1/prompt-templates              # List
GET    /api/v1/prompt-templates/{id}         # Get
PUT    /api/v1/prompt-templates/{id}         # Update
DELETE /api/v1/prompt-templates/{id}         # Delete
```

## List Prompt Templates

```bash
curl http://localhost:8000/api/v1/prompt-templates?namespace=default
```

Query parameters: `namespace`, `limit`, `cursor`

Response: `200 OK`

```json
{
  "items": [
    {
      "id": "pt_001",
      "name": "classify-intent",
      "namespace": "default",
      "description": "Classifies user intent from a message",
      "system_prompt": "You are an intent classifier.",
      "user_prompt": "Classify the intent of this message: {{subject}}",
      "variables": [
        {"name": "subject", "schema": {"type": "string"}}
      ],
      "response_format": "json",
      "created_at": "2026-03-10T12:00:00Z",
      "updated_at": "2026-03-10T12:00:00Z"
    }
  ],
  "next_cursor": null,
  "has_more": false
}
```

## Create Prompt Template

```bash
curl -X POST http://localhost:8000/api/v1/prompt-templates \
  -H "Content-Type: application/json" \
  -d '{
    "name": "classify-intent",
    "namespace": "default",
    "description": "Classifies user intent from a message",
    "system_prompt": "You are an intent classifier.",
    "user_prompt": "Classify the intent of this message: {{subject}}",
    "variables": [
      {"name": "subject", "schema": {"type": "string"}}
    ],
    "response_format": "json"
  }'
```

Response: `201 Created`

## Get Prompt Template

```bash
curl http://localhost:8000/api/v1/prompt-templates/pt_001
```

Response: `200 OK`

## Update Prompt Template

```bash
curl -X PUT http://localhost:8000/api/v1/prompt-templates/pt_001 \
  -H "Content-Type: application/json" \
  -d '{ ... updated template ... }'
```

Response: `200 OK`

## Delete Prompt Template

```bash
curl -X DELETE http://localhost:8000/api/v1/prompt-templates/pt_001
```

Response: `204 No Content`

## See Also

- [API Overview](overview.md) -- authentication, pagination, error format
- [Data Model](../specs/data-model.md) -- prompt template fields
- [Terraform Resources](../terraform/resources.md) -- `brockley_prompt_template` resource
- [Expression Language](../specs/expression-language.md) -- template syntax

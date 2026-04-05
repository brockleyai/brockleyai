# Schemas API

Manage library JSON Schema definitions. Schemas are reusable building blocks that get copied into port definitions when building graphs. They are not referenced at runtime.

## Endpoints

```
POST   /api/v1/schemas              # Create
GET    /api/v1/schemas              # List
GET    /api/v1/schemas/{id}         # Get
PUT    /api/v1/schemas/{id}         # Update
DELETE /api/v1/schemas/{id}         # Delete
```

## List Schemas

```bash
curl http://localhost:8000/api/v1/schemas?namespace=default&limit=20
```

Query parameters: `namespace`, `limit`, `cursor`

Response: `200 OK`

```json
{
  "items": [
    {
      "id": "schema_001",
      "name": "ticket-input",
      "namespace": "default",
      "description": "Customer support ticket input",
      "json_schema": {
        "type": "object",
        "properties": {
          "id": {"type": "string"},
          "subject": {"type": "string"},
          "body": {"type": "string"}
        },
        "required": ["id", "subject", "body"]
      },
      "created_at": "2026-03-10T12:00:00Z",
      "updated_at": "2026-03-10T12:00:00Z"
    }
  ],
  "next_cursor": null,
  "has_more": false
}
```

## Create Schema

```bash
curl -X POST http://localhost:8000/api/v1/schemas \
  -H "Content-Type: application/json" \
  -d '{
    "name": "ticket-input",
    "namespace": "default",
    "description": "Customer support ticket input",
    "json_schema": {
      "type": "object",
      "properties": {
        "id": {"type": "string"},
        "subject": {"type": "string"},
        "body": {"type": "string"}
      },
      "required": ["id", "subject", "body"]
    }
  }'
```

Response: `201 Created`

Schemas must pass strong typing rules: no bare `{"type": "object"}` or `{"type": "array"}`. Objects must have `properties`, arrays must have `items`.

## Get Schema

```bash
curl http://localhost:8000/api/v1/schemas/schema_001
```

Response: `200 OK` with the full schema object.

## Update Schema

```bash
curl -X PUT http://localhost:8000/api/v1/schemas/schema_001 \
  -H "Content-Type: application/json" \
  -d '{ ... updated schema ... }'
```

Response: `200 OK`

## Delete Schema

```bash
curl -X DELETE http://localhost:8000/api/v1/schemas/schema_001
```

Response: `204 No Content`

## See Also

- [API Overview](overview.md) -- authentication, pagination, error format
- [Data Model](../specs/data-model.md) -- strong typing rules
- [Terraform Resources](../terraform/resources.md) -- `brockley_schema` resource

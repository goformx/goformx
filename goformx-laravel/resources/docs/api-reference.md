---
title: API Reference
order: 4
---

# API Reference

GoFormX exposes public endpoints for fetching form schemas and submitting responses. These endpoints require no authentication and are rate-limited to 60 requests per minute.

## Get form schema

Retrieve the JSON schema for a form.

```
GET /api/forms/{form-id}/schema
```

**Response** `200 OK`

```json
{
  "id": "abc123",
  "title": "Contact Form",
  "schema": {
    "components": [...]
  }
}
```

## Submit a form

Submit a response to a form.

```
POST /api/forms/{form-id}/submit
Content-Type: application/json
```

**Request body**

```json
{
    "data": {
        "name": "Jane Doe",
        "email": "jane@example.com",
        "message": "Hello!"
    }
}
```

**Response** `201 Created`

```json
{
    "id": "sub_xyz",
    "form_id": "abc123",
    "created_at": "2026-02-27T12:00:00Z"
}
```

**Error responses**

| Status | Meaning                                            |
| ------ | -------------------------------------------------- |
| `404`  | Form not found                                     |
| `422`  | Validation failed — response includes field errors |
| `429`  | Rate limit exceeded — wait and retry               |

## Get embed HTML

Returns a standalone HTML page for embedding in an iframe.

```
GET /api/forms/{form-id}/embed
```

Returns `text/html` with the rendered form.

# Form Lifecycle Spec

**Owns:** End-to-end form CRUD flow across Waaseyaa frontend and Go API

---

## Overview

Forms are the core domain object in GoFormX. The form lifecycle spans two services:
- **Waaseyaa frontend** — UI, user auth, plan enforcement, rendering
- **Go API** — form storage, schema validation, submission handling

The frontend never stores form data. It delegates all persistence to the Go API via `GoFormsClient`.

---

## Form CRUD Flow

### Create

```
1. User navigates to /forms (Forms/Index.vue)
2. Clicks "Create Form" → POST /forms
3. AppServiceProvider route handler:
   a. Validates auth (session check)
   b. Calls FormController::store(userId, planTier, data)
   c. FormController calls GoFormsClient::post('/api/forms', ...)
4. Go API:
   a. Assertion middleware verifies HMAC
   b. FormHandler validates schema
   c. GORM creates form in PostgreSQL
   d. Returns 201 with form JSON
5. Frontend redirects to /forms
```

### Read (List)

```
1. User navigates to /forms
2. AppServiceProvider calls FormController::index(userId, planTier)
3. FormController calls GoFormsClient::get('/api/forms', ...)
4. Go returns paginated form list (filtered by user_id ownership)
5. Frontend renders Forms/Index.vue with form data
```

### Read (Single / Edit)

```
1. User navigates to /forms/{id}/edit
2. AppServiceProvider calls FormController::edit(id, userId, planTier)
3. FormController calls GoFormsClient::get('/api/forms/{id}', ...)
4. Go returns form with full schema
5. Frontend renders Forms/Edit.vue with Form.io builder
```

### Update

```
1. User modifies form in Form.io builder
2. Builder emits schema change → PUT /forms/{id}
3. AppServiceProvider parses JSON body, calls FormController::update(...)
4. FormController calls GoFormsClient::put('/api/forms/{id}', ...)
5. Go validates and updates form in PostgreSQL
6. Frontend redirects to /forms/{id}/edit
```

### Delete

```
1. User clicks delete on form
2. DELETE /forms/{id}
3. AppServiceProvider calls FormController::destroy(id, userId, planTier)
4. FormController calls GoFormsClient::delete('/api/forms/{id}', ...)
5. Go deletes form and associated submissions
6. Frontend redirects to /forms
```

---

## Submission Flow (Public)

```
1. Anonymous user loads /forms/{id} (public fill page)
2. Vue renders Forms/Fill.vue with Form.io renderer
3. Form schema fetched from Go: GET /forms/{id}/schema (public, no auth)
4. User fills and submits form
5. POST /forms/{id}/submit (public, rate limited)
6. Go validates submission against schema, stores in PostgreSQL
7. Returns 201 success
```

---

## Plan Enforcement

Plan tier determines form limits (enforced at the Go API level):

| Tier | Max Forms | Max Submissions/Month |
|------|-----------|----------------------|
| free | 3 | 100 |
| pro | 25 | 5,000 |
| business | 100 | 50,000 |
| growth | 500 | 250,000 |
| enterprise | unlimited | unlimited |

Plan tier is read from `UserRepository::getPlanTier()` and passed to Go via request headers.

---

## Key Files

| File | Role |
|------|------|
| `goformx-web/src/Controller/FormController.php` | Thin controller, delegates to GoFormsClient |
| `goformx-web/src/Service/GoFormsClient.php` | HTTP client with HMAC signing |
| `goformx-web/src/AppServiceProvider.php` | Route definitions (forms.index, forms.create, etc.) |
| `goformx-web/frontend/src/pages/Forms/*.vue` | Vue pages (Index, Edit, Fill, Submissions) |
| `goforms/internal/application/handlers/web/` | Go REST handlers |
| `goforms/internal/domain/form/` | Go form domain (model, service, repository) |

---

## Error Handling

The Waaseyaa frontend maps Go API errors:

| Go Status | Frontend Behavior |
|-----------|------------------|
| 401 | Redirect to /login (assertion failed) |
| 403 | Flash "Access denied" |
| 404 | Redirect to /forms |
| 422 | Display validation errors |
| 429 | Flash "Rate limit exceeded" |
| 5xx | Flash "Something went wrong" |

`FormController` catches `\RuntimeException` from `GoFormsClient` and redirects to `/forms` as a fallback.

# GoFormX API

GoFormX is an AI-first, schema-driven forms service. JSON Schema Draft 2020-12 is the form definition, OpenAPI 3.1 is the HTTP contract, and PostgreSQL stores immutable schema versions and the submissions validated against them.

The supported runtime is this Go service. Retired dashboard, renderer fork, browser-session, and plan-tier code remains available in Git history but is absent from the build and scan graph.

## v1 flow

1. A server-side caller uses a scoped `gfst_` service token to create a form and schema versions.
2. Publishing a version makes its unguessable `gfpk_` public key and exact schema version available to approved browser origins.
3. The browser fetches the published JSON Schema without a credential.
4. The browser submits `{ "data": ... }` with an idempotency key and optional exact schema-version header.
5. GoFormX validates and persists the submission. If a webhook is enabled, the same transaction creates its encrypted delivery intent. A safe retry returns the same submission and does not duplicate delivery.

The machine-readable contract is [`contracts/openapi.v1.yaml`](contracts/openapi.v1.yaml). Webhook configuration, receiver verification, retries, status, and replay are documented in [`docs/webhooks.md`](../docs/webhooks.md).

## Development

Prerequisites are Go 1.26.7, Node.js 22, Docker Compose, and Task. PostgreSQL is disposable and provisioned by verification.

```bash
task bootstrap
task migrate:up
task verify
task dev
```

`task verify` is the same complete gate used by CI. It starts a disposable PostgreSQL 17.11 instance, verifies migrations, lints the OpenAPI contract, checks generated/module drift, vets and lints Go, runs race and behavioral-coverage gates, scans reachable dependencies, and removes the database even after failure. It does not deploy, alter DNS, or touch development/production data.

## Delivery and production ownership

This repository builds and attests multi-architecture GHCR images; it does not deploy them. Production Compose, secrets, migrations, reverse-proxy configuration, external smoke tests, backup/restore, and rollback are owned by [`jonesrussell/waaseyaa-infra`](https://github.com/jonesrussell/waaseyaa-infra). The retired SSH, Supervisor, Nginx, and in-repository Compose deploy paths were removed so there is only one operational source of truth.

## Provision a service token

Token plaintext is returned once; only its SHA-256 hash is stored. The owner must already exist in the `users` table.

```bash
export DATABASE_URL='postgres://goformx:password@localhost:5432/goformx?sslmode=disable'
go run ./cmd/goformx-token issue \
  --owner 11111111-1111-4111-8111-111111111111 \
  --scopes forms:read,forms:write,forms:publish,submissions:read \
  --ttl 24h
```

The command emits JSON suitable for a secret manager or an agent tool. Never place the returned token in browser JavaScript. Revoke it by its non-secret token ID:

```bash
go run ./cmd/goformx-token revoke --token-id TOKEN_ID
```

Rotate an active token atomically. The replacement inherits the owner's scopes, the previous token is revoked with auditable lineage, and the replacement plaintext is emitted only once:

```bash
go run ./cmd/goformx-token rotate --token-id TOKEN_ID --ttl 24h
```

## API surface

| Route | Access | Purpose |
| --- | --- | --- |
| `POST /v1/forms` | `forms:write` | Create a form and initial draft schema |
| `POST /v1/forms/{id}/versions` | `forms:write` | Append an immutable draft version |
| `POST /v1/forms/{id}/versions/{version}/publish` | `forms:publish` | Publish an exact version |
| `GET /v1/forms/{id}/submissions` | `submissions:read` | Retrieve accepted submissions |
| `PUT /v1/forms/{id}/webhook` | `forms:write` | Store an encrypted destination and signing configuration |
| `GET /v1/forms/{id}/deliveries` | `submissions:read` | Inspect recent delivery state without secret material |
| `POST /v1/forms/{id}/deliveries/{deliveryId}/replay` | `forms:write` | Requeue a dead-letter delivery |
| `GET /v1/public/forms/{publicKey}/schema` | Public key | Fetch a published schema |
| `POST /v1/public/forms/{publicKey}/submissions` | Public key | Validate and accept an idempotent submission |

Only `/v1` is supported. Renderer-specific schemas and the former `/api` assertion-auth routes have been removed.

## License

AGPL-3.0-or-later; see [`LICENSE`](LICENSE).

# ADR 0001: JSON Schema is the canonical form contract

- Status: accepted
- Date: 2026-08-27
- Decision owners: GoFormX maintainers

## Context

GoFormX previously mixed a Go domain model, Form.io component JSON, and frontend assumptions. That made the data contract renderer-dependent and difficult for agents or other clients to discover safely.

## Decision

JSON Schema Draft 2020-12 is the single canonical form-definition format. OpenAPI 3.1 describes the HTTP contract and uses the same dialect. Form definitions are immutable once published; edits create a new numbered version. Form.io and any future renderer are adapters, never alternate sources of truth.

The API has two explicit planes:

- The authenticated control plane manages forms, immutable schema versions, publication, and submissions.
- The public data plane resolves a published schema by a rotatable non-secret public key and accepts idempotent submissions.

Server integrations use short-lived scoped bearer tokens. Public browser clients receive only public keys. Errors use a stable envelope and validation errors use JSON Pointers. Clients can pin a published schema version; omitting it selects the current version.

## Consequences

The committed OpenAPI document and canonical schema are executable contracts and must pass independent contract tests. Runtime handlers and persistence must implement them rather than redefining resource shapes. Breaking changes require a new API version and an ADR.

## V1 non-goals

- A dashboard or form-builder UI
- Billing and plan enforcement
- Migrating Laravel or Waaseyaa applications
- Form.io schema compatibility
- Automated production deployment or DNS changes

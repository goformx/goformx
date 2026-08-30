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

## Canonical validation authority

The shared application validator embeds `contracts/schema/form-definition.schema.json` directly, validates the form-definition envelope against those exact published bytes, then compiles the submitted definition with the maintained Draft 2020-12 implementation. Initial creation, immutable version creation, publication, and submission validation use this authority. Schema reference, depth, node, and pattern budgets run before compilation. Validation does not fetch the contract from the deployment filesystem or network.

`Form.Validate` requires the existing `DefinitionValidator` boundary, as `NewSchemaVersion` does. The form model retains metadata invariants but must not apply a separate property-type whitelist. An empty property schema, boolean property schemas, local references, `null`, and union types are valid when the canonical contract permits them. The root form envelope still requires a nonempty properties object; a generic valid JSON Schema is not automatically a valid GoFormX form definition.

This removes the creation/version contradiction found in #140 without changing published contract bytes. Before deploying the correction, check persisted versions for envelopes that earlier runtime paths incorrectly accepted (such as missing or empty properties). Do not silently rewrite immutable historical schemas: create and explicitly publish a conforming replacement where needed, and record compatibility evidence in the deployment release gate.

## V1 non-goals

- A dashboard or form-builder UI
- Billing and plan enforcement
- Migrating Laravel or Waaseyaa applications
- Form.io schema compatibility
- Automated production deployment or DNS changes

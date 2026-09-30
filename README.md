# GoFormX

GoFormX is an AI-first, schema-driven forms service. The only supported runtime is [`goforms/`](goforms/), with JSON Schema Draft 2020-12 as the canonical form definition, OpenAPI 3.1 as the machine contract, and PostgreSQL as the persistence layer.

The [product vision](docs/product/PRODUCT-VISION.md) and [delivery roadmap](docs/product/ROADMAP.md), adopted 2026-09-29, focus on forms created from a developer's existing AI tools and one authorized human inbox across sites. The personal-site contact flow remains a regression gate, not the complete release milestone. The Go service owns the data and API contract; the Waaseyaa control plane owns accounts, sessions and the human inbox.

Start with the [service README](goforms/README.md), the [architecture boundary](docs/architecture/schema-first-runtime.md), and the [active roadmap](https://github.com/goformx/goformx/issues/84).

For agents and custom dashboards, use the [published API contract and tested client guide](docs/api-clients.md). No dashboard login is needed to download the machine interface.

Database runtime, migration, operator and backup authority is specified in the [tested PostgreSQL permission contract](docs/database-permissions.md); production provisioning remains infrastructure-owned.

The former human-first web runtime and renderer fork were retired under [issue #83](https://github.com/goformx/goformx/issues/83). Recovery links and removal decisions are recorded in [the archive index](docs/archive/legacy-runtime.md).

# Legacy runtime archive

Issue [#83](https://github.com/goformx/goformx/issues/83) removed the accidental second product from the active repository. The last monorepo commit containing the complete former web tree and Form.io submodule pointer is [`2e9cf151`](https://github.com/goformx/goformx/tree/2e9cf151b1276036845d49d04c4d9d1508ad460a).

| Retired path | Disposition | Recovery / active direction |
| --- | --- | --- |
| `goformx-web/` PHP/Waaseyaa and Vue runtime | Deleted from the working tree | Recover from [`2e9cf151/goformx-web`](https://github.com/goformx/goformx/tree/2e9cf151b1276036845d49d04c4d9d1508ad460a/goformx-web); new work follows [roadmap #84](https://github.com/goformx/goformx/issues/84) |
| `goformx-formio` submodule | Removed; upstream repository archived | Recover from the [`goformx/formio`](https://github.com/goformx/formio) archive; renderer-specific schemas are not a supported contract |
| Browser session, CSRF, HMAC assertion, shadow-user, plan-tier, duplicate middleware, and Fx composition packages | Deleted | Git history only; any reintroduction requires a new ADR and roadmap decision |
| Legacy implementation plans, framework specs, demo seed, and plan-tier migration | Deleted | Git history only; active contracts live in OpenAPI, ADR 0001, and `docs/architecture/` |
| Production/development source paths | Consolidated | `goforms/cmd/api`, `goforms/cmd/goformx-token`, `goforms/contracts`, and PostgreSQL migrations are the complete supported graph |

Git history is the archive: no abandoned package remains compilable or visible to dependency and security scans, while prior evidence stays recoverable by commit.

# GoFormX roadmap for the greenfield release

Authoritative local roadmap revised 2026-09-30: Russell confirmed no existing users or data, allows ordinary signup and unverified login, and defers mail. [Previous scope](history/release-before-greenfield-2026-09-30/ROADMAP.md) is historical. [PRODUCT-VISION.md](PRODUCT-VISION.md) supplies direction; the single [release checklist](release-package/README.md) owns acceptance.

Russell can add a form to his next real site through Codex, explicitly publish it, receive signed webhook notification and see its submissions beside his other sites in one authorized inbox. [The two-site journey](local-journey/RESULT.md) passed recorded Go/PHP heads. Keep tenancy, separate submission-read permission, durable retries, explicit publication and revocation.

## Remaining work

1. Finish actual signup/login without verification or a mail provider, preserving normal organization authorization. Document that forgot-password requires mail later; do not add a recovery feature.
2. Freeze/test the final pair and build tracked API/migration/maintenance/PHP/web artifacts. Verify fresh PostgreSQL/PHP startup, supported preflight, routed login, form submission and inbox behavior.
3. Prepare the minimal single-host configuration and DNS diff. Start with fresh PostgreSQL and SQLite. No populated production migration, account preservation or Pi recovery dependency exists.
4. Present the actual source/image/configuration package for approval. After approval, deploy, prove the real-site journey and take a consistent backup. Retain current databases when reverting application images after new writes.

WSL native runtime is available. DevLake was stopped at Russell's request without deleting volumes. Restricted old local-journey files remain untouched; that workspace is not reused.

Mail/emailed recovery, MCP, a second harness, billing, broad integrations, builder expansion, AI submission analysis, scaling and launch claims are deferred. Do not invent gates for nonexistent users/data. A second harness is required only before portability claims.

No merge, production deployment, DNS/credential mutation, decommissioning or spending is authorized here. [EVIDENCE.md](release-package/EVIDENCE.md) records actual results; historical green checks qualify only their recorded heads.

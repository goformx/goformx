# GoFormX roadmap: forms managed from your AI workflow

Authoritative roadmap, adopted 2026-09-29. Supersedes the previous schema-first resurrection/dashboard-launch sequence. Read [PRODUCT-VISION.md](PRODUCT-VISION.md) first. Phases describe acceptance order, not promised dates. Local planning files live under `C:/projects/GoFormX/product`; centrally tracked work remains in `goformx/goformx`, including control-plane work.

## R0. Qualify the foundation

Next implementation slice: upgrade the control plane from alpha.299 to the latest published Waaseyaa alpha, verified as alpha.302 on September 29. Recheck release availability at execution time and pin exact released packages.

- Complete #169's operation-specific bounded PHP/browser reads and completeness tests. The upstream fix is released; waiting for it is no longer the blocker.
- Complete #118 account/reset/session gates on installed packages, with no source overlays.
- Reconcile #120 residual token-mint authority and #123 integration acceptance.
- Pair the candidate control plane with the exact candidate Go SHA in all cross-service suites. The existing CI Go pin predates main.
- Keep `task verify`, `composer check`, site-contract diagnostics and all five boundary suites as the baseline.

Exit: exact package/source pair passes clean bootstrap, authorization, response integrity and the full real-service lifecycle. No production rollout is implied.

## R1. Prove one assistant-driven form workflow

Write the adapter/connection contract before implementation. Start with a documented API-backed path in Russell's chosen development harness, with MCP as the preferred portable adapter where supported.

- Define secure connection setup, per-connection identity, granted operations, revocation and local credential custody. Decide local versus remote adapter transport from actual client constraints; do not prescribe one universal connection mechanism without a compatibility spike.
- Reuse existing API operations. If a capability is missing, change OpenAPI and shared behavior first.
- Discover/reuse existing forms; create a draft; set/validate schema; prepare site integration code; submit a test; request explicit publication; report the endpoint and result.
- Define retry/idempotency and uncertain-outcome handling for each operation. Do not promise idempotency for operations that lack a contract.
- Keep management and submission-read permissions separate. Test foreign tenants, wrong scopes, revocation and attempted instruction injection from tool data.
- Choose the smallest notification path that satisfies the contact-form workflow. Existing webhooks may be reused; email product delivery must be deliberately implemented and tested before advertising it.

Exit: a recorded, reproducible contact-form journey on a real development site, without hidden server/database interventions or manual dashboard setup beyond necessary account consent. No secrets in frontend code, logs or recorded evidence.

## R2. Deliver one useful inbox across sites

- Define site identity and form association, including ownership and authorization. Keep organizations as tenancy, not an assumed synonym for a site.
- Add authorized cross-form reads through Go's API where missing, with bounded pagination, explicit filters and stable ordering.
- Provide a human inbox with site/form filters, submission detail and links back to the relevant form/site. Preserve exact data and safe exports.
- Make account connections, scopes and revocation visible. Keep current form management as a fallback and recovery surface.
- Support contact, consent-bearing newsletter capture and short intake examples. Avoid campaign-management scope.

Exit: Russell launches forms on two distinct sites and sees both sets of submissions in one authorized inbox. A user cannot infer or retrieve another tenant's forms or submissions. An assistant without submission-read permission remains unable to read the inbox.

## R3. Prove portability and developer onboarding

- Test Hermes, ChatGPT and Claude connection paths individually against their actual available client/edition/version. Record transport, authentication, allowed operations, limitations, date and evidence.
- Provide copyable setup instructions and tested examples for each supported path. API/script support is valid when MCP is unavailable; label the path accurately.
- Continue management of the same form from at least two harnesses without recreating its identity or moving data.
- Document disconnect/reconnect, expired credentials, publication confirmation, failed requests and recovery.
- Test first-run onboarding with a small invited group of developers who maintain multiple sites. Record friction and second-site reuse before growing the scope.

Exit: two harnesses pass the same end-to-end conformance journey. Publish support claims only for verified paths. A third target stays explicitly experimental until it passes.

## R4. Deploy and launch the focused product

The infrastructure preparation can proceed alongside R1-R3 after R0. Public positioning as an assistant-managed product requires the workflow, inbox and compatibility evidence, not merely healthy containers. A separately approved private dogfood deployment may happen earlier if its relevant safety gates pass.

- Carry forward #125 and private infra #75/#62/#66 acceptance: separate database roles, controlled image contexts, attested immutable images, key custody, consistent encrypted offsite backups, restore proof, capacity, migration and rollback rehearsals.
- Proposed host remains fetder-droplet, subject to capacity verification. www.goformx.com is the canonical UI; api.goformx.com preserves public API consumers; plan apex compatibility explicitly.
- Use the deployment handoff for operational detail, amended by this roadmap. The old ARM64-only target and dashboard-led public launch criteria are superseded.
- With explicit approval, move the authoritative data plane from the Pi without split writers or stale-database rollback. Preserve public form keys and existing consumers.
- Demonstrate account connection, assistant creation/integration, explicit publishing, public submission, notification, shared inbox and recovery on the deployed release.

Exit: reproducible release evidence, operational monitoring, restore/rollback proof, two real sites and verified multi-harness use. No DNS, deployment, merge or spending authority is granted by this roadmap.

## Retained work and superseded assumptions

| Existing item | New disposition |
| --- | --- |
| #118, #169 | Immediate R0 installed-release and integrity gates. |
| #120, #123 | Finish acceptance; do not expand dashboards merely to justify closing them. |
| #125 | R4 operations plus the new assistant-to-inbox release journey. |
| #57 and existing personal-site tests | Retained regression/reliability gate; no longer the entire product acceptance definition. |
| #110 | Deferred until multiple API instances or measured distributed admission need. |
| #111 | Evidence-triggered abuse control; no blanket CAPTCHA-first product requirement. |
| #165 | Measure before optimization. |
| #171 and dependency PRs | Reassess with candidate dependencies and reachable-vulnerability evidence; do not blindly waive license/security checks. |
| Broad visual builder, hosted AI chat, billing-first launch | Outside initial scope. |
| September handoffs and review ledger | Historical evidence; neither future priorities nor proof of current release qualification. |
| Deployment plan of September 29 | Operational reference subordinate to this vision; infrastructure choice is retained provisionally. |

## Measurement and release records

Record time from successful connection to first valid submission, completion and failure by step, reuse on a second site, harness switching, duplicates/unauthorized mutations, delivery errors and restore outcomes. Establish a baseline before inventing conversion or speed claims. Store only necessary event metadata; exclude submitted content, tokens and prompts.

For every slice, record owner, issue, exact source/package versions, verification commands/results, outstanding constraints and next gate. Closed legacy implementation issues do not establish completion of the new workflow. No schedule or pricing commitment has been made.

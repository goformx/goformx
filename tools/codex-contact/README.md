# Local Codex contact-form client

This is an API/script path for Codex running in Russell's workspace. It is an experimental local client, not an MCP server or a claim of a completed real-service journey. It uses only Python's standard library. The Go OpenAPI contract remains authoritative.

## Connection

Select an organization and issue a revocable `gfst_` token through the authorized control-plane flow. The default connection needs `forms:read` and `forms:write`. Keep the token in a user-only local secret store. Load it into `GOFORMX_SERVICE_TOKEN` only for the client process. Never put it in a repository, shell history, browser bundle, generated code, or test evidence. Pass an HTTPS API origin, or loopback HTTP for local development, and the selected organization ID on every command. Revoking the token in the control-plane UI causes the next call to report 401.

The command examples below use placeholder IDs. Set `$cli` to the absolute path of `goformx_contact.py` and provide the token environment variable from your configured secret store without displaying it.

```powershell
python $cli --api-origin http://127.0.0.1:8080 --organization-id $org ensure-site --name 'Site A' --site-origin https://site-a.example
python $cli --api-origin http://127.0.0.1:8080 --organization-id $org discover --site-id $site
python $cli --api-origin http://127.0.0.1:8080 --organization-id $org discover --form-id $form
python $cli --api-origin http://127.0.0.1:8080 --organization-id $org create-draft --intent site-a-contact-v1 --spec .\contact-form.json
python $cli --api-origin http://127.0.0.1:8080 --organization-id $org generate --form-id $form --output .\src\goformx-contact.js
python $cli --api-origin http://127.0.0.1:8080 --organization-id $org publish --form-id $form --version 1 --confirm
python $cli --api-origin http://127.0.0.1:8080 --organization-id $org test-submission --form-id $form --intent site-a-test-v1 --synthetic-data .\synthetic-data.json
```

`ensure-site` creates or reuses the site with the same normalized origin and name, then reads it back. A changed name for the same origin is a server conflict and needs human reconciliation. `contact-form.json` contains `name`, `title`, `schema`, the returned `siteId`, and `allowedOrigins` containing that site's browser-serialized origin exactly. Go compares the raw browser `Origin` header with the stored allowed value. The client reads back the exact accepted form, site association, origin allowance, and schema after creating a draft. `discover` uses exact form or site IDs and does not infer uniqueness from names. A generated browser module includes the `gfpk_` public key, public API origin, selected site origin, accepted schema and version. It refuses to submit if loaded on another origin. Its caller must provide a stable 16 to 128 character idempotency key and reuse it if a submission outcome is uncertain. It contains no management token.

Before mutation, the client verifies that the token's organization from `GET /v1/sites` matches the selected organization. `create-draft` commits an operation key to a local SQLite database with `synchronous=FULL` before the first POST. Two processes using the same intent reuse the same key. Repeating the same `--intent` and spec retries with that key, including after an uncertain network response. It refuses to reuse an intent for a changed origin, organization or spec. Existing JSON intent keys are migrated without changing the key. An incomplete JSON file or unusable database is a hard stop because replacing its key could duplicate a form. `test-submission` likewise keeps a key per synthetic payload and intent. State defaults to the user profile at `~/.goformx/codex-intents`, contains no credential or submitted content, and should not be placed in source control. Keep this directory private and on reliable local storage; an operating system or device that does not honor SQLite sync cannot provide a power-loss guarantee. A 401 requires reauthorization; a 403 requires an appropriate grant. The client does not read submissions and never requests `submissions:read`.

Publication is a distinct command requiring both `--confirm` and a connection with `forms:publish`. Codex should run it only after Russell explicitly names the form and schema version to publish. No create, discover, generate or test command publishes automatically. The current public submission endpoint accepts only published schemas, so the synthetic public test runs after explicit publication. The test sends the selected site's `Origin`, checks browser CORS preflight and response headers, pins `X-GoFormX-Schema-Version`, and rejects a result accepted under another version. There is no supported draft-only server submission preview yet. Generating code before publication does not establish a working live form.

## Hosted setup status

The client accepts an HTTPS API origin and identifies its HTTP requests as
`GoFormX-Codex-Contact/1.0`, rather than Python's default user agent. This brings
the shipped request path in line with the identified client used in the separate
personal-site integration. It does not bypass edge challenges, change grants or
establish hosted end-to-end qualification. A 403 may come from the service grant
or the hosting edge; inspect the failure before changing permissions.

The verified two-site rehearsal was local. Hosted setup still requires normal
account sign-in, a selected organization, an explicitly issued expiring token,
and a controlled site. Publication must separately name the exact form/version.
Qualification must use synthetic data, preserve retries, check the human inbox
and revoke temporary grants. Email notifications remain unverified.

## Historical integration dependencies

- The server branch must add `Idempotency-Key` to `POST /v1/forms` and enforce one draft per logical retry. The local client persists and sends the header but this checkout alone cannot prove server behavior.
- The server branch must add the documented site endpoints, `siteId` on form create and form readback, same-origin/name site-create reconciliation, and `meta.organizationId` on `GET /v1/sites` even for an empty page. This client implements those calls against the candidate contract but cannot prove them until integration.
- A real two-site journey, revocation, default submission-read denial, cross-site inbox and browser submission must be exercised against the paired service after its contract lands. Fake-server tests only verify this client's behavior.
- Notification setup and signed delivery verification are not part of this client slice. Do not claim notification works from an accepted submission alone.

Run the client tests with `python -m unittest discover -s tools/codex-contact -v` from the repository root.

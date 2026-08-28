# Schema-first abuse-case gate

This gate is a release requirement for the public form API. It is exercised by `task verify` and must be updated whenever an API route, schema rule, persistence transaction, credential, or outbound network boundary changes.

| Abuse case | Required invariant | Automated evidence |
| --- | --- | --- |
| Oversized or malformed JSON | At most one JSON object and no more than 1 MiB enters a handler | v1 HTTP tests |
| External or unsafe schema reference | `$ref`, `$dynamicRef`, and `$recursiveRef` stay inside the same document | validation security tests |
| Schema evaluation exhaustion | Depth, node count, and pattern length are bounded; compiled immutable schemas use a bounded concurrent cache | validation security and race tests |
| Anonymous storage flooding | Each form has an independent burst limit and a transactionally serialized rolling daily quota | HTTP and PostgreSQL integration tests |
| Replay or changed retry body | Same form/key/body returns the original record; changed content returns `409` | v1 vertical-slice and repository tests |
| Cross-owner access or enumeration | Another owner's form has the same external absence response as an unknown form | v1 authorization tests |
| Large submission history | Reads use an opaque cursor, deterministic order, default page size 25, and maximum 100 | handler and PostgreSQL page tests |
| Token theft or overreach | Plaintext is never stored; expiry, revocation, owner, and exact route scope are enforced | domain, middleware, and PostgreSQL token tests |
| SQL injection or quota race | Values are bound parameters; per-form admission holds a PostgreSQL transaction advisory lock | PostgreSQL integration tests |
| Sensitive logging | Tokens, webhook secrets, and submission payloads never enter request logs | logger sanitization and write-only webhook API tests |
| Webhook SSRF, header injection, forgery, or replay | Destinations resolve only to public addresses at connect time; secrets are encrypted; reserved headers are rejected; requests carry a timestamped HMAC and stable delivery ID | webhook policy, cipher, signature, dispatcher, handler, and PostgreSQL tests |

High or critical findings block release. Lower findings must either be fixed in the same change or appear in the residual-risk table of the threat model with a named owner and removal condition.

# GoFormX product vision

Status: authoritative product direction. Adopted by Russell on 2026-09-29.

This decision supersedes all earlier product visions, feature priorities, launch definitions and marketing plans. Historical documents remain evidence of past work, not competing instructions. The operational deployment plan remains subordinate to this direction. Existing security contracts, data preservation obligations and explicit production-approval boundaries remain in force until deliberately replaced with reviewed equivalents.

## The promise

**One home for your forms. Managed from the AI tools you already use.**

GoFormX helps developers who repeatedly launch websites add reliable forms without rebuilding a backend or moving into another authoring workflow. Forms belong to their GoFormX account, stay usable when they switch AI assistants, and send their submissions to one place.

The core job: “While I am building this site, help me create and connect the form it needs, then let me manage it alongside the forms on all my other sites.”

## Why this exists

Russell started GoFormX because he repeatedly needed simple forms on new sites and wanted all the results in one place. The initial ambition to compete broadly with established form-building platforms is retired. The opportunity is to make the form backend a natural part of the developer's AI-assisted workflow.

AI is the management interface. Reliable collection is the core service. A model call is not required to accept, validate, store or deliver a submission.

## Who we serve first

The initial user is Russell, followed by independent developers and small teams who launch and maintain multiple sites using AI coding assistants. Their frequent needs are contact forms, newsletter capture and short intake forms.

Agencies may become a useful audience after organization isolation and repeat use are proven. Complex enterprise surveys, visual form-design teams and general workflow automation are outside the initial focus.

## The defining experience

During a site build, the developer asks:

> Add a contact form to this site. Collect name, email and message, notify me when someone submits, and use my existing GoFormX account.

The assistant resolves the intended account/site, creates a draft with an explicit schema, prepares integration code, tests it, and reports what is ready. Publication is a separate, explicit operation authorized by the user. An ambiguous timeout must not silently create another form or publish twice.

The developer opens GoFormX to see submissions across authorized sites, filter to a site or form, inspect the exact accepted data, manage access and recover from failures. The UI supports the workflow even when the assistant is unavailable.

The developer can later use another supported harness to find and manage that same form. The account, endpoint, public form key and data do not belong to the original assistant.

## Product principles

1. Stay in the developer's existing workflow. Do not require a GoFormX-specific chat interface to manage forms.
2. Support portable contracts. A documented API is the foundation; MCP is a proposed adapter for compatible clients, with API/script/skill paths where appropriate. Do not invent a second set of business rules for agents.
3. Make simple forms simple. Optimize the contact-form journey before expanding the catalog.
4. Centralize visibility without weakening tenancy. “One inbox” means submissions the current user is authorized to see, never an unrestricted cross-organization view.
5. Keep people in control. Show meaningful proposed changes, keep publication explicit, expose failures and uncertain outcomes, and make credentials revocable.
6. Keep collection independent of AI providers. No model is on the critical path for submissions.
7. Earn compatibility claims. Hermes, ChatGPT and Claude are target harnesses, not a claim that all current editions and connection modes already work.

## Authority and privacy

- Go owns forms, schemas, immutable versions, publication, submissions, tokens, webhooks and their canonical API contract.
- The Waaseyaa control plane owns human accounts, browser sessions, organizations/memberships, connections, navigation and the human inbox. It does not directly read Go's PostgreSQL database or keep a second submission store.
- An agent adapter acts through the same public business contract and scoped authorization as other integrations. No agent superuser, database shortcuts or privileged MCP-only mutation path.
- Form-management permission and submission-reading permission are separate. The default site-building connection does not need access to people's submitted messages.
- Public site code receives only browser-safe submission identifiers. Management tokens and signing material remain outside generated client code and logs.
- Submission content is untrusted data. It cannot grant permissions, issue tool instructions or authorize publication. Sending submissions into an AI harness requires a separate explicit access/privacy decision, including what data leaves GoFormX and the receiving provider's handling.
- Existing audit, replay, tenant isolation, validation, revocation, no-store and key-custody requirements survive this product change.

## Initial scope

Include account connection and revocation; discover existing forms; create/reuse a draft; set a schema; produce working frontend integration code; test; explicitly publish; inspect configuration; basic notification delivery; and a shared human inbox filtered by site and form.

Before implementation, decide how site identity and form association are represented. Do not overload organization IDs as site IDs or duplicate authoritative submission records to build an inbox. Newsletter capture means storing a consent-bearing signup and delivering it to a chosen destination, not building an email-campaign platform.

Retain existing useful dashboards, tokens and webhooks. Expand them only where the core workflow or release safety requires it.

## Explicit non-goals

- Competing on an exhaustive drag-and-drop builder, survey catalog or theme marketplace.
- Building a GoFormX-hosted general-purpose AI assistant.
- Automatic analysis of every submission, embeddings or background model enrichment.
- Autonomous publication, broad default submission access or unrestricted assistant credentials.
- Billing, enterprise administration, horizontal scaling or a large integration catalog before the core workflow is proven.
- A launch checklist defined by dashboard feature count.

## Success and prioritization

The primary outcome is a developer's next real site accepting its first valid submission, with that submission visible in the same inbox as their other sites.

Track connection-to-first-valid-submission time, completion/failure by step, reuse on a second site, cross-harness continuation, publication mistakes/duplicates, submission reliability and inbox usefulness. Collect event metadata without recording form contents, secrets or raw assistant prompts.

Initial validation targets, not public claims: Russell completes the journey on at least two real sites; the same form is managed from two independently tested harnesses; default assistant credentials cannot read submissions; a revoked connection fails immediately according to the documented contract; existing contact/newsletter consumers continue to work.

A feature earns priority if it shortens that journey, improves collection/recovery, makes switching harnesses practical, or is necessary for privacy and authorization. Otherwise defer it explicitly.

## Companion documents

- [Delivery roadmap](ROADMAP.md)
- [Positioning and launch messaging](MARKETING.md)
- [Codex local connection contract](CODEX-CONNECTION-CONTRACT.md)
- [Central GitHub roadmap](https://github.com/goformx/goformx/issues/84)

Russell owns changes to the product direction. Every implementation agent must check this vision and the central roadmap before treating historical plans as work orders.

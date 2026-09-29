# GoFormX positioning and marketing brief

Adopted 2026-09-29 under [PRODUCT-VISION.md](PRODUCT-VISION.md). This replaces previous form-builder positioning. Copy below is a working messaging system; future-capability copy must not be published as current availability.

## Positioning

Audience: developers who repeatedly ship websites using AI coding tools and want simple, reliable forms without maintaining a backend for each site.

Category: a form backend for AI-assisted development.

Promise: **One home for your forms. Managed from the AI tools you already use.**

Short explanation: GoFormX connects the forms on your websites to one account and inbox. Create and manage them from a supported AI assistant, while GoFormX handles validation, storage and delivery.

The differentiating product bet is continuity across sites and assistants. It must be proven through onboarding and daily use. We make no claim to be the first, only, fastest or cheapest service in this category.

## Message pillars

| Message | User benefit | Required proof |
| --- | --- | --- |
| Build where you already work | Add the form during the site-building conversation | Recorded end-to-end integration in each advertised harness |
| Every site's forms in one place | Find results without opening a separate backend per site | Two-site inbox with real authorization/filtering tests |
| Change assistants, keep your forms | Account and data outlive the tool used to create them | Same form managed from two independently tested harnesses |
| Simple collection you can depend on | Submissions still arrive without a model call | Service reliability, notification and restore evidence |
| Give assistants only the access they need | Setup need not expose submitted messages | Scope separation, revocation and browser-secret tests |

## Homepage copy for the current development stage

Eyebrow: Forms for developers building with AI

Headline: **Your next site needs a form. Your assistant should be able to set it up.**

Subhead: We're building GoFormX so you can create and manage forms from your AI coding tools, with submissions from all your sites in one place.

Primary CTA: Request early access

Secondary CTA: Follow development

Only use the primary CTA when an actual monitored request flow exists. Otherwise link to the public roadmap. Do not invent customer counts, endorsements or a waiting list.

## Homepage copy after the launch gates pass

Headline: **One home for your forms.**

Subhead: Create and manage forms from your supported AI assistant. Collect submissions from every site in one inbox, without building another backend.

Primary CTA: Connect your assistant

Secondary CTA: See a contact form go live

Three supporting blocks:

- **Add a form while you build.** Ask your assistant to create the form, connect your site and test a submission. Review and publish when you're ready.
- **Keep every site in view.** See your authorized submissions together and filter by site or form.
- **Keep your forms when you switch tools.** Your GoFormX account and endpoints stay with you. Choose another supported assistant whenever you need to.

Show a compatibility table with verified connection modes. Do not replace it with an unqualified “works with any AI” claim.

## Product demo

Start with a real small website project and an account with another site's form already present.

1. Ask the assistant to add a contact form collecting name, email and message.
2. Show the proposed schema, notification destination and requested access.
3. Generate the frontend integration and run a synthetic test submission.
4. Give explicit publication approval and show the working form.
5. Open the shared inbox and filter between the two sites.
6. Use a second supported harness to find the same form and inspect its configuration.
7. Show that the setup connection cannot read submission contents without separate permission.

Use synthetic data. Never expose management tokens or one-time reveals in recordings. Label cuts and prerequisites honestly; do not imply a shorter measured setup time than the demo establishes.

## Founder story

“I kept launching websites, and most of them needed the same thing: a simple form and somewhere reliable for the results to go. I wanted those results together instead of scattered across projects. GoFormX started there. Now that I do more of my development through AI tools, I want to manage my forms in that same workflow, without tying them to one assistant.”

## Launch announcement draft

Pre-release version:

“I'm building GoFormX for developers who keep launching sites. The goal is simple: ask your coding assistant to set up a form, then see submissions from all your sites in one place. I'm starting with contact forms and testing the workflow across AI tools. The current roadmap shows what works and what's still being built.”

Release version, only after evidence:

“GoFormX lets you manage your website forms from supported AI coding tools and collect the results in one inbox. Create a form, connect it to your site, test it and explicitly publish it. Your forms stay in your account when you switch assistants. See the demo and compatibility guide to try it.”

Name only the harnesses and notification paths that passed release acceptance.

## Early go-to-market sequence

1. Dogfood on Russell's next two sites. Capture real setup friction and repeat use.
2. Publish a technical walkthrough with code, an honest demo and the compatibility matrix.
3. Invite a small group of developers maintaining several sites. Observe first setup and the next-site experience; ask what they otherwise would have used.
4. Turn successful workflows into maintained quickstarts: contact, consent-bearing signup, short intake, and continuing from another harness.
5. Share useful walkthroughs in relevant developer communities where participation is welcome. Posting or outreach requires separate authorization; this document authorizes neither.
6. Expand distribution only after activation and repeat use are credible. Avoid paid acquisition until setup is repeatable and service cost is understood.

Potential educational topics: adding a contact form during an AI-assisted site build; keeping form credentials out of frontend code; managing the same form from two assistants; collecting submissions across multiple sites. These are editorial topics, not keyword-volume or ranking claims.

## Commercial hypothesis

Start with a controlled developer preview. Do not announce a permanent free tier or a price yet. Investigate pricing by account usage, active forms and submission/storage costs while keeping multi-site usefulness intact. Model quotas, abuse and delivery costs before promising limits. Do not introduce per-model token billing for ordinary form collection, which does not require inference.

## Claim discipline

Available foundation: schema-first Go API and substantial account/dashboard/integration implementations, subject to remaining release gates.

Planned until verified: portable assistant connection, MCP adapter, complete assistant-driven setup, unified cross-site inbox, per-harness compatibility and the revised launch journey.

Never claim automatic privacy compliance, guaranteed delivery, universal compatibility, unlimited usage, production readiness or measured setup speed without specific evidence. ChatGPT, Claude and Hermes names describe intended compatibility, not endorsement or affiliation. Do not use provider logos or fabricate testimonials.

Marketing succeeds when it attracts developers who complete the workflow and use GoFormX on their next site, not merely when the AI language increases clicks.

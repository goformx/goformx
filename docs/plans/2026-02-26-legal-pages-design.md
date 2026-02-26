# Legal Pages Design — Privacy Policy & Terms of Service

**Date:** 2026-02-26
**Scope:** Static Vue pages for Privacy Policy and Terms of Service
**Regulatory coverage:** PIPEDA + Quebec Law 25 + GDPR + CCPA (global)

## Approach

Static Vue pages (Approach A) — hardcoded content in Vue templates with Tailwind prose styling. Matches the existing public page pattern (Home, Pricing, Demo). No database, no markdown parser, no CMS.

## Routes & Controllers

**Routes** (public group in `web.php`):
- `GET /privacy` → `PrivacyController` (invokable)
- `GET /terms` → `TermsController` (invokable)

**Controllers** — two invokable controllers matching existing pattern (`DemoController`, `PricingController`). Render Inertia pages with no props beyond global shared data.

**Sitemap** — add `/privacy` and `/terms` to `sitemap.xml` route.

## Vue Pages

### Privacy.vue (`resources/js/pages/Privacy.vue`)

- Imports `PublicHeader` + new `PublicFooter`
- `<Head>` with title, meta description, canonical URL
- Sticky table of contents sidebar for navigation (policy is long with global coverage)
- Structured prose content with Tailwind typography utilities
- "Last updated" date displayed prominently
- Cross-link to Terms page

**Sections:**
1. Information We Collect
2. How We Use Your Information
3. Legal Bases for Processing (GDPR)
4. Sharing & Third Parties
5. International Data Transfers
6. Data Retention
7. Your Rights (subsections by jurisdiction: PIPEDA, Law 25, GDPR, CCPA)
8. Cookies & Tracking
9. Children's Privacy
10. Changes to This Policy
11. Contact Us

### Terms.vue (`resources/js/pages/Terms.vue`)

- Same layout pattern as Privacy
- Cross-link to Privacy page

**Sections:**
1. Acceptance of Terms
2. Description of Service
3. Accounts
4. Acceptable Use
5. Intellectual Property
6. Payment & Billing
7. Data & Privacy
8. Service Availability
9. Limitation of Liability
10. Termination
11. Governing Law & Disputes
12. Changes to Terms
13. Contact Us

## PublicFooter Component

**`PublicFooter.vue`** (`resources/js/components/PublicFooter.vue`)

- Added to all public pages (Home, Pricing, Demo, Privacy, Terms, Fill)
- Single-row layout: copyright left, legal links right
- Muted text, top border, responsive (stacks on mobile)
- Minimal — just copyright + Privacy + Terms links

## Privacy Policy Content Scope

**Data collected:** account info (name, email), billing (via Stripe — no card numbers stored), form data (schemas, submissions), usage data (IP, browser, device), cookies.

**Legal bases (GDPR):** contract performance, legitimate interest, consent, legal obligation.

**Third-party processors:** Stripe (payments, US-based), hosting provider, email service. Each named with purpose and country.

**International transfers:** Canadian-hosted, Stripe processes in US. SCCs/adequacy decisions referenced for EU transfers.

**Rights by jurisdiction:**
- **All users:** access, correction, deletion, data portability
- **PIPEDA (Canada):** withdraw consent, file complaint with OPC
- **Law 25 (Quebec):** de-indexation, incident notification
- **GDPR (EU/EEA):** restriction, object, supervisory authority complaint
- **CCPA (California):** right to know, delete, opt-out of sale (explicitly state we don't sell), non-discrimination

**Retention:** account data while active + 30 days post-deletion, billing records 7 years (tax law), form submissions deleted with account, logs rotated 90 days.

**Cookies:** session (functional), analytics (if any). No third-party marketing cookies.

**Children:** not directed at under-16, no knowing collection.

**Contact:** email for privacy requests. Response timelines: 30 days (PIPEDA/GDPR), 45 days (CCPA).

## Terms of Service Content Scope

- **Acceptance:** account creation or use = agreement, 16+ required
- **Service:** forms management platform, create/collect/embed
- **Accounts:** accurate info, one person per account, responsible for credentials, 2FA recommended
- **Acceptable use:** no illegal content, phishing, spam, sensitive data without safeguards, API abuse, reverse engineering
- **IP:** GoFormX owns platform, user owns their content/submissions, license to host/process granted
- **Billing:** Stripe, subscriptions, free tier, monthly/annual, cancellation at period end, no partial refunds
- **Privacy:** reference to Privacy Policy
- **Availability:** best-effort uptime, no SLA, planned maintenance with notice
- **Liability:** as-is, limited to 12 months of fees, no indirect/consequential damages
- **Termination:** user can delete anytime, we can terminate for violations with notice, data deleted per retention
- **Governing law:** Ontario, Canada (doesn't limit GDPR/CCPA statutory rights)
- **Changes:** 30 days email notice for material changes, continued use = acceptance
- **Contact:** same email as privacy policy

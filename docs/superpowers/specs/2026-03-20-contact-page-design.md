# Contact Page Design

**Date:** 2026-03-20
**Status:** Approved

## Summary

Add a `/contact` page to goformx.com that embeds the existing Contact Form (`442dd7f7-95a2-4709-91c9-3d4cd4996212`) using Form.io's JS renderer. Also add a `source` field to the `/me` site's existing contact form submission payload to distinguish which site submissions come from.

## Goals

- Dogfood GoFormX on the marketing site (demonstrates the Form.io embed workflow to potential customers)
- Distinguish submission sources across sites via a `source` field injected at render/submit time
- Add "Contact" link to goformx.com nav and footer

## Design

### 1. goformx.com/contact (Form.io renderer)

**Route:** `GET /contact` → renders `contact.html.twig`

**Template structure:**
- Extends `layout.html.twig` (same as pricing, privacy, terms)
- Heading: "Contact us" with subtitle
- `<div id="goformx-contact">` container for Form.io
- `<script>` block loads Form.io JS from CDN (`https://cdn.form.io/formiojs/formio.full.min.js`)
- On load: `Formio.createForm()` with schema URL `https://api.goformx.com/forms/442dd7f7-95a2-4709-91c9-3d4cd4996212/schema`
- Submission override injects `source: 'goformx.com'` into the data
- Custom submit handler POSTs to `https://api.goformx.com/forms/442dd7f7-95a2-4709-91c9-3d4cd4996212/submit`
- On success: container replaced with "Thanks! We'll get back to you soon."

**Form.io configuration:**
```javascript
Formio.createForm(document.getElementById('goformx-contact'), schemaUrl).then(form => {
    form.submission = { data: { source: 'goformx.com' } };
    form.on('submit', (submission) => {
        // Replace form with success message
    });
});
```

**Nav/footer changes:**
- Nav: add "Contact" link between "Docs" and "Sign in"
- Footer: add "Contact" after "Terms"

### 2. /me site (source field addition)

**File:** `src/lib/components/forms/ContactForm.svelte`

**Change:** Add `source: 'jonesrussell.github.io/me'` to the submit payload:
```typescript
await service.submitForm(config.formIds.contact, {
    email,
    message,
    ...(referral ? { referral } : {}),
    source: 'jonesrussell.github.io/me',
});
```

No other changes to the `/me` site. The existing custom Svelte form stays as-is.

## Files to Change

| File | Repo | Change |
|------|------|--------|
| `goformx-web/templates/contact.html.twig` | goformx | New template |
| `goformx-web/src/AppServiceProvider.php` | goformx | Add `/contact` route |
| `goformx-web/templates/layout.html.twig` | goformx | Add Contact to nav + footer |
| `src/lib/components/forms/ContactForm.svelte` | me | Add source field to payload |

## Not Doing

- No Go API changes (public submit endpoint already exists)
- No Contact Form schema modifications (source injected at embed time)
- No new Vue/Inertia pages (contact is a public Twig page)
- No CAPTCHA (Go API has rate limiting)
- No Form.io renderer on /me site (custom Svelte form matches the terminal aesthetic)

## Submission Source Tracking

Each submission's `data.source` field tells you which site it came from:

| Site | Source Value |
|------|-------------|
| goformx.com | `goformx.com` |
| jonesrussell.github.io/me | `jonesrussell.github.io/me` |
| Future embeds | Set at embed time |

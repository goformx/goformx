# Contact Page Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `/contact` page to goformx.com that embeds the Contact Form via Form.io renderer, and add a `source` field to the `/me` site's existing contact form.

**Architecture:** Two independent changes across two repos. GoFormX gets a new Twig template with Form.io JS that loads the form schema from the Go API at runtime. The `/me` site adds one field to an existing submit payload.

**Tech Stack:** Twig templates, Form.io JS (CDN), SvelteKit (Svelte 5)

**Spec:** `docs/superpowers/specs/2026-03-20-contact-page-design.md`

---

## File Structure

| File | Repo | Action | Responsibility |
|------|------|--------|----------------|
| `goformx-web/templates/contact.html.twig` | goformx | Create | Contact page with Form.io embed |
| `goformx-web/templates/layout.html.twig` | goformx | Modify | Add Contact to nav and footer |
| `goformx-web/src/AppServiceProvider.php` | goformx | Modify | Add `/contact` route |
| `src/lib/components/forms/ContactForm.svelte` | me | Modify | Add source field to payload |

---

### Task 1: Add /contact route to AppServiceProvider

**Files:**
- Modify: `goformx-web/src/AppServiceProvider.php` (in `registerPublicRoutes()`)

- [ ] **Step 1: Add the route**

In `registerPublicRoutes()`, after the `terms` route, add:

```php
$router->addRoute('contact', new Route('/contact', defaults: ['_controller' => $this->twig('contact.html.twig')]));
```

- [ ] **Step 2: Run tests to verify nothing breaks**

Run: `cd goformx-web && vendor/bin/phpunit`
Expected: 37 tests, all pass

- [ ] **Step 3: Commit**

```bash
git add goformx-web/src/AppServiceProvider.php
git commit -m "feat(web): add /contact route"
```

---

### Task 2: Add Contact link to nav and footer

**Files:**
- Modify: `goformx-web/templates/layout.html.twig`

- [ ] **Step 1: Add Contact to nav**

In the `.nav-links` div, add a Contact link between the Docs link and the Sign in button:

```html
<a href="/contact">Contact</a>
```

So the nav links become: Pricing, Docs, Contact, Sign in, Get started.

- [ ] **Step 2: Add Contact to footer**

In the footer, add Contact after Terms:

```html
&middot; <a href="/contact">Contact</a>
```

So the footer becomes: &copy; 2026 GoFormX &middot; Privacy &middot; Terms &middot; Contact

- [ ] **Step 3: Verify locally**

Open any marketing page and confirm Contact appears in both nav and footer.

- [ ] **Step 4: Commit**

```bash
git add goformx-web/templates/layout.html.twig
git commit -m "feat(web): add Contact link to nav and footer"
```

---

### Task 3: Create contact.html.twig template

**Files:**
- Create: `goformx-web/templates/contact.html.twig`

- [ ] **Step 1: Create the template**

```twig
{% extends "layout.html.twig" %}

{% block title %}Contact — GoFormX{% endblock %}

{% block head %}
<link rel="stylesheet" href="https://cdn.form.io/formiojs/formio.full.min.css">
<style>
    .contact-page { max-width: 40rem; margin: 0 auto; padding: 3rem 0; }
    .contact-page h1 { font-size: 2rem; font-weight: 800; margin-bottom: 0.5rem; }
    .contact-page .subtitle { color: #6b7280; margin-bottom: 2rem; }
    .contact-form-container { background: #fff; border: 1px solid #e5e7eb; border-radius: 0.5rem; padding: 2rem; }
    .contact-success { text-align: center; padding: 3rem 2rem; }
    .contact-success h2 { font-size: 1.5rem; font-weight: 700; margin-bottom: 0.5rem; color: #059669; }
    .contact-success p { color: #6b7280; }
</style>
{% endblock %}

{% block content %}
<div class="contact-page">
    <h1>Contact us</h1>
    <p class="subtitle">Have a question or want to learn more? We'd love to hear from you.</p>
    <div class="contact-form-container">
        <div id="goformx-contact"></div>
    </div>
</div>

<script src="https://cdn.form.io/formiojs/formio.full.min.js"></script>
<script>
(function() {
    var FORM_ID = '442dd7f7-95a2-4709-91c9-3d4cd4996212';
    var API_URL = 'https://api.goformx.com';
    var container = document.getElementById('goformx-contact');

    fetch(API_URL + '/forms/' + FORM_ID + '/schema')
        .then(function(res) { return res.json(); })
        .then(function(response) {
            var schema = response.data || response;
            return Formio.createForm(container, schema, {
                noAlerts: true,
                hooks: {
                    beforeSubmit: function(submission, next) {
                        submission.data = submission.data || {};
                        submission.data.source = 'goformx.com';
                        next();
                    }
                }
            });
        })
        .then(function(form) {
            form.nosubmit = true;

            form.on('submit', function(submission) {
                fetch(API_URL + '/forms/' + FORM_ID + '/submit', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(submission.data)
                })
                .then(function(res) {
                    if (!res.ok) throw new Error('Submission failed');
                    showSuccess();
                })
                .catch(function() {
                    alert('Something went wrong. Please try again.');
                });
            });
        })
        .catch(function() {
            container.textContent = 'Unable to load contact form. Please email us directly.';
        });

    function showSuccess() {
        var wrapper = container.closest('.contact-form-container');
        while (wrapper.firstChild) wrapper.removeChild(wrapper.firstChild);

        var div = document.createElement('div');
        div.className = 'contact-success';

        var h2 = document.createElement('h2');
        h2.textContent = 'Message sent!';
        div.appendChild(h2);

        var p = document.createElement('p');
        p.textContent = "Thanks for reaching out. We'll get back to you soon.";
        div.appendChild(p);

        wrapper.appendChild(div);
    }
})();
</script>
{% endblock %}
```

- [ ] **Step 2: Run tests**

Run: `cd goformx-web && vendor/bin/phpunit`
Expected: 37 tests, all pass

- [ ] **Step 3: Commit**

```bash
git add goformx-web/templates/contact.html.twig
git commit -m "feat(web): add contact page with Form.io embed"
```

---

### Task 4: Deploy and smoke test goformx.com/contact

- [ ] **Step 1: Push all goformx commits**

```bash
git push
```

- [ ] **Step 2: Wait for CI deploy to complete**

```bash
gh run list --repo goformx/goformx --workflow "Deploy GoFormX Web" --limit 1
gh run watch <run-id> --repo goformx/goformx --exit-status
```

- [ ] **Step 3: Smoke test with Playwright**

Navigate to `https://goformx.com/contact` and verify:
- Page loads with "Contact us" heading
- Form.io renders Email, Message, Referral fields
- Contact link appears in nav and footer
- No console errors (except favicon 404)

- [ ] **Step 4: Submit a test submission**

Fill out the form and submit. Verify:
- Success message appears: "Message sent!"
- Check submissions in dashboard — new submission should have `source: 'goformx.com'` in its data

---

### Task 5: Add source field to /me site ContactForm

**Files:**
- Modify: `/home/fsd42/dev/me/src/lib/components/forms/ContactForm.svelte`

- [ ] **Step 1: Add source to submit payload**

In the `handleSubmit` function, add `source` to the data object:

```typescript
await service.submitForm(config.formIds.contact, {
    email,
    message,
    ...(referral ? { referral } : {}),
    source: 'jonesrussell.github.io/me',
});
```

- [ ] **Step 2: Run tests**

Run: `cd /home/fsd42/dev/me && npm test`
Expected: All tests pass

- [ ] **Step 3: Commit and push**

```bash
cd /home/fsd42/dev/me
git add src/lib/components/forms/ContactForm.svelte
git commit -m "feat: add source field to contact form submissions"
git push
```

# Legal Pages Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add Privacy Policy and Terms of Service pages with a PublicFooter component to all public pages.

**Architecture:** Static Vue pages following existing public page pattern (PublicHeader + Head + content). Two invokable controllers. PublicFooter added to all 5 public pages. Global regulatory coverage (PIPEDA, Law 25, GDPR, CCPA).

**Tech Stack:** Laravel 12, Inertia v2, Vue 3, TypeScript, Tailwind CSS v4, Pest v4

---

### Task 1: PrivacyController + route + test

**Files:**
- Create: `goformx-laravel/app/Http/Controllers/PrivacyController.php`
- Modify: `goformx-laravel/routes/web.php`
- Create: `goformx-laravel/tests/Feature/PrivacyPageTest.php`

**Step 1: Create the test file**

```bash
cd goformx-laravel && ddev exec php artisan make:test PrivacyPageTest --pest --no-interaction
```

**Step 2: Write the failing tests**

Replace `tests/Feature/PrivacyPageTest.php` with:

```php
<?php

use Inertia\Testing\AssertableInertia as Assert;

it('renders privacy page for guests', function () {
    $this->get(route('privacy'))
        ->assertOk()
        ->assertInertia(fn (Assert $page) => $page->component('Privacy'));
});

it('renders privacy page for authenticated users', function () {
    $user = \App\Models\User::factory()->create();

    $this->actingAs($user)
        ->get(route('privacy'))
        ->assertOk()
        ->assertInertia(fn (Assert $page) => $page->component('Privacy'));
});
```

**Step 3: Run tests to verify they fail**

Run: `cd goformx-laravel && ddev exec php artisan test --compact --filter=PrivacyPageTest`
Expected: FAIL (route not defined)

**Step 4: Create the controller**

```bash
cd goformx-laravel && ddev exec php artisan make:controller PrivacyController --invokable --no-interaction
```

Edit `app/Http/Controllers/PrivacyController.php`:

```php
<?php

namespace App\Http\Controllers;

use Illuminate\Http\Request;
use Inertia\Inertia;
use Inertia\Response;

class PrivacyController extends Controller
{
    public function __invoke(Request $request): Response
    {
        return Inertia::render('Privacy');
    }
}
```

**Step 5: Add route to web.php**

After the `Route::get('pricing', ...)` line, add:

```php
use App\Http\Controllers\PrivacyController;
use App\Http\Controllers\TermsController;
```

(Add imports at top of file)

```php
Route::get('privacy', PrivacyController::class)->name('privacy');
Route::get('terms', TermsController::class)->name('terms');
```

(Add routes after the pricing route)

**Step 6: Create a minimal Privacy.vue placeholder**

Create `resources/js/pages/Privacy.vue`:

```vue
<script setup lang="ts">
import { Head } from '@inertiajs/vue3';
import PublicHeader from '@/components/PublicHeader.vue';
</script>

<template>
    <div class="flex min-h-screen flex-col bg-background text-foreground">
        <Head title="Privacy Policy" />
        <PublicHeader />
        <main class="flex-1">
            <div class="container py-16">
                <h1 class="text-3xl font-semibold">Privacy Policy</h1>
                <p class="mt-4 text-muted-foreground">Coming soon.</p>
            </div>
        </main>
    </div>
</template>
```

**Step 7: Run tests to verify they pass**

Run: `cd goformx-laravel && ddev exec php artisan test --compact --filter=PrivacyPageTest`
Expected: PASS (2 tests)

**Step 8: Commit**

```bash
git add goformx-laravel/app/Http/Controllers/PrivacyController.php goformx-laravel/routes/web.php goformx-laravel/tests/Feature/PrivacyPageTest.php goformx-laravel/resources/js/pages/Privacy.vue
git commit -m "feat: add privacy page route, controller, and placeholder"
```

---

### Task 2: TermsController + route + test

**Files:**
- Create: `goformx-laravel/app/Http/Controllers/TermsController.php`
- Modify: `goformx-laravel/routes/web.php` (already has import from Task 1)
- Create: `goformx-laravel/tests/Feature/TermsPageTest.php`

**Step 1: Create the test file**

```bash
cd goformx-laravel && ddev exec php artisan make:test TermsPageTest --pest --no-interaction
```

**Step 2: Write the failing tests**

Replace `tests/Feature/TermsPageTest.php` with:

```php
<?php

use Inertia\Testing\AssertableInertia as Assert;

it('renders terms page for guests', function () {
    $this->get(route('terms'))
        ->assertOk()
        ->assertInertia(fn (Assert $page) => $page->component('Terms'));
});

it('renders terms page for authenticated users', function () {
    $user = \App\Models\User::factory()->create();

    $this->actingAs($user)
        ->get(route('terms'))
        ->assertOk()
        ->assertInertia(fn (Assert $page) => $page->component('Terms'));
});
```

**Step 3: Run tests to verify they fail**

Run: `cd goformx-laravel && ddev exec php artisan test --compact --filter=TermsPageTest`
Expected: FAIL (route not defined or component missing)

**Step 4: Create the controller**

```bash
cd goformx-laravel && ddev exec php artisan make:controller TermsController --invokable --no-interaction
```

Edit `app/Http/Controllers/TermsController.php`:

```php
<?php

namespace App\Http\Controllers;

use Illuminate\Http\Request;
use Inertia\Inertia;
use Inertia\Response;

class TermsController extends Controller
{
    public function __invoke(Request $request): Response
    {
        return Inertia::render('Terms');
    }
}
```

**Step 5: Create a minimal Terms.vue placeholder**

Create `resources/js/pages/Terms.vue`:

```vue
<script setup lang="ts">
import { Head } from '@inertiajs/vue3';
import PublicHeader from '@/components/PublicHeader.vue';
</script>

<template>
    <div class="flex min-h-screen flex-col bg-background text-foreground">
        <Head title="Terms of Service" />
        <PublicHeader />
        <main class="flex-1">
            <div class="container py-16">
                <h1 class="text-3xl font-semibold">Terms of Service</h1>
                <p class="mt-4 text-muted-foreground">Coming soon.</p>
            </div>
        </main>
    </div>
</template>
```

**Step 6: Run tests to verify they pass**

Run: `cd goformx-laravel && ddev exec php artisan test --compact --filter=TermsPageTest`
Expected: PASS (2 tests)

**Step 7: Commit**

```bash
git add goformx-laravel/app/Http/Controllers/TermsController.php goformx-laravel/tests/Feature/TermsPageTest.php goformx-laravel/resources/js/pages/Terms.vue
git commit -m "feat: add terms page route, controller, and placeholder"
```

---

### Task 3: Update sitemap + test

**Files:**
- Modify: `goformx-laravel/routes/web.php` (sitemap route)
- Modify: `goformx-laravel/tests/Feature/SitemapTest.php`

**Step 1: Write the failing test**

Add to `tests/Feature/SitemapTest.php`:

```php
test('sitemap contains privacy and terms URLs', function () {
    $appUrl = 'https://example.com';
    config(['app.url' => $appUrl]);

    $response = $this->get(route('sitemap'));

    $response->assertOk();
    $body = $response->getContent();
    expect($body)->toContain('<loc>'.$appUrl.'/privacy</loc>');
    expect($body)->toContain('<loc>'.$appUrl.'/terms</loc>');
});
```

**Step 2: Run test to verify it fails**

Run: `cd goformx-laravel && ddev exec php artisan test --compact --filter="sitemap contains privacy"`
Expected: FAIL

**Step 3: Update sitemap in web.php**

In the sitemap route, add to the `$urls` array:

```php
['loc' => $appUrl.'/privacy', 'lastmod' => $lastmod],
['loc' => $appUrl.'/terms', 'lastmod' => $lastmod],
```

**Step 4: Run sitemap tests to verify all pass**

Run: `cd goformx-laravel && ddev exec php artisan test --compact --filter=SitemapTest`
Expected: PASS (all sitemap tests)

**Step 5: Commit**

```bash
git add goformx-laravel/routes/web.php goformx-laravel/tests/Feature/SitemapTest.php
git commit -m "feat: add privacy and terms to sitemap"
```

---

### Task 4: PublicFooter component

**Files:**
- Create: `goformx-laravel/resources/js/components/PublicFooter.vue`

**Step 1: Create PublicFooter.vue**

Create `resources/js/components/PublicFooter.vue`:

```vue
<script setup lang="ts">
import { Link } from '@inertiajs/vue3';
</script>

<template>
    <footer class="border-t border-border/50 bg-background">
        <div
            class="container flex flex-col items-center justify-between gap-4 px-4 py-6 sm:flex-row sm:px-6"
        >
            <p class="text-sm text-muted-foreground">
                &copy; {{ new Date().getFullYear() }} GoFormX. All rights reserved.
            </p>
            <nav class="flex items-center gap-4">
                <Link
                    href="/privacy"
                    class="text-sm text-muted-foreground transition-colors hover:text-foreground"
                >
                    Privacy Policy
                </Link>
                <span class="text-muted-foreground/40">|</span>
                <Link
                    href="/terms"
                    class="text-sm text-muted-foreground transition-colors hover:text-foreground"
                >
                    Terms of Service
                </Link>
            </nav>
        </div>
    </footer>
</template>
```

**Step 2: Commit**

```bash
git add goformx-laravel/resources/js/components/PublicFooter.vue
git commit -m "feat: add PublicFooter component with legal links"
```

---

### Task 5: Add PublicFooter to all public pages

**Files:**
- Modify: `goformx-laravel/resources/js/pages/Home.vue`
- Modify: `goformx-laravel/resources/js/pages/Pricing.vue`
- Modify: `goformx-laravel/resources/js/pages/Demo.vue`
- Modify: `goformx-laravel/resources/js/pages/DemoUnconfigured.vue`
- Modify: `goformx-laravel/resources/js/pages/Forms/Fill.vue`
- Modify: `goformx-laravel/resources/js/pages/Privacy.vue`
- Modify: `goformx-laravel/resources/js/pages/Terms.vue`

**Step 1: Add to each page**

In each file's `<script setup>`, add the import:

```typescript
import PublicFooter from '@/components/PublicFooter.vue';
```

In each file's `<template>`, add `<PublicFooter />` just before the closing `</div>` (after `</main>`).

For example in Home.vue, the template ends:

```vue
        </main>
        <PublicFooter />
    </div>
</template>
```

Apply this same pattern to all 7 public pages listed above.

**Step 2: Run all tests to ensure nothing broke**

Run: `cd goformx-laravel && ddev exec php artisan test --compact`
Expected: All tests PASS

**Step 3: Commit**

```bash
git add goformx-laravel/resources/js/pages/Home.vue goformx-laravel/resources/js/pages/Pricing.vue goformx-laravel/resources/js/pages/Demo.vue goformx-laravel/resources/js/pages/DemoUnconfigured.vue goformx-laravel/resources/js/pages/Forms/Fill.vue goformx-laravel/resources/js/pages/Privacy.vue goformx-laravel/resources/js/pages/Terms.vue
git commit -m "feat: add PublicFooter to all public pages"
```

---

### Task 6: Privacy Policy content

**Files:**
- Modify: `goformx-laravel/resources/js/pages/Privacy.vue`

**Step 1: Replace placeholder with full content**

Replace `Privacy.vue` with the full privacy policy page. The page should:

- Use `PublicHeader` and `PublicFooter`
- Include `<Head>` with title "Privacy Policy", meta description, and canonical URL
- Have a sticky table of contents sidebar (hidden on mobile, visible on `lg:` breakpoint)
- Use Tailwind prose-style typography (`space-y-6`, `text-muted-foreground` for body, `text-foreground` for headings)
- Display "Last updated: February 26, 2026" prominently
- Cross-link to Terms page at the bottom
- Contact email: `privacy@goformx.com`

**Content sections** (all in the Vue template as structured HTML):

1. **Information We Collect** — account data (name, email), billing (Stripe token, no card numbers), form data (schemas, submissions created by users), usage data (IP, browser, device, pages visited), cookies (session, analytics)
2. **How We Use Your Information** — provide/maintain service, process payments, send transactional emails, improve product, ensure security, comply with law
3. **Legal Bases for Processing (GDPR)** — contract performance, legitimate interests, consent, legal obligation. Table format.
4. **Sharing and Third Parties** — Stripe (payments, US), hosting infrastructure, email service. We do not sell personal data. Each processor named with purpose and country.
5. **International Data Transfers** — primarily Canadian-hosted, Stripe and infrastructure may transfer to US. For EU users: standard contractual clauses. For Canadian users: PIPEDA cross-border transfer transparency.
6. **Data Retention** — account data while active + 30 days, billing records 7 years, form submissions deleted with account, server logs 90 days
7. **Your Rights** — subsections by jurisdiction:
   - All Users: access, correction, deletion, portability
   - Canada (PIPEDA): withdraw consent, complaint to OPC
   - Quebec (Law 25): de-indexation, incident notification rights
   - EU/EEA (GDPR): restrict processing, object, supervisory authority complaint
   - California (CCPA): right to know, delete, opt-out of sale (we don't sell — stated), non-discrimination
8. **Cookies and Tracking** — session cookies (essential), analytics cookies (if any), no third-party marketing trackers
9. **Children's Privacy** — not directed at under 16, no knowing collection
10. **Changes to This Policy** — material changes notified by email 30 days in advance
11. **Contact Us** — privacy@goformx.com, response timelines: 30 days (PIPEDA, GDPR), 45 days (CCPA)

**Step 2: Run Prettier and ESLint**

```bash
cd goformx-laravel && ddev exec npm run format && ddev exec npm run lint
```

**Step 3: Commit**

```bash
git add goformx-laravel/resources/js/pages/Privacy.vue
git commit -m "feat: add privacy policy content (PIPEDA, Law 25, GDPR, CCPA)"
```

---

### Task 7: Terms of Service content

**Files:**
- Modify: `goformx-laravel/resources/js/pages/Terms.vue`

**Step 1: Replace placeholder with full content**

Replace `Terms.vue` with the full terms of service page. Same layout pattern as Privacy:

- `PublicHeader` + `PublicFooter`
- `<Head>` with title "Terms of Service", meta description, canonical URL
- Sticky table of contents sidebar
- "Last updated: February 26, 2026"
- Cross-link to Privacy Policy

**Content sections:**

1. **Acceptance of Terms** — by creating account or using service, you agree. Must be 16+. If you don't agree, don't use the service.
2. **Description of Service** — GoFormX is a forms management platform for creating, hosting, and collecting form submissions. Includes free and paid tiers.
3. **Your Account** — accurate info required, one person per account, responsible for credentials and activity, 2FA recommended, notify us of unauthorized access
4. **Acceptable Use** — no illegal content, phishing, spam, malware distribution, collection of sensitive data (health, financial, children) without appropriate safeguards, no API abuse or rate limit circumvention, no reverse engineering, no impersonation
5. **Intellectual Property** — GoFormX owns the platform (code, design, trademarks). You own your form content and submission data. You grant us a limited license to host, process, and display your content solely to provide the service.
6. **Payment and Billing** — Stripe handles all billing. Subscription plans billed monthly or annually. Free tier available. Cancellation effective at end of billing period. No refunds for partial periods. We may change pricing with 30 days notice.
7. **Data and Privacy** — your use of the service is also governed by our Privacy Policy (link). Data processing details are described there.
8. **Service Availability** — best-effort availability, no SLA at this time. We may perform maintenance with reasonable notice. We are not liable for downtime.
9. **Limitation of Liability** — service provided "as is" without warranties. Our total liability limited to fees you paid in the 12 months prior to the claim. We are not liable for indirect, incidental, or consequential damages.
10. **Termination** — you may delete your account at any time via settings. We may suspend or terminate for violations with reasonable notice (except for egregious violations). Upon termination, data handled per our retention policy.
11. **Governing Law and Disputes** — governed by laws of Ontario, Canada. Disputes resolved in courts of Ontario. This does not limit statutory consumer rights under GDPR, CCPA, or other applicable law.
12. **Changes to These Terms** — 30 days email notice for material changes. Continued use after notice period constitutes acceptance.
13. **Contact Us** — support@goformx.com

**Step 2: Run Prettier and ESLint**

```bash
cd goformx-laravel && ddev exec npm run format && ddev exec npm run lint
```

**Step 3: Commit**

```bash
git add goformx-laravel/resources/js/pages/Terms.vue
git commit -m "feat: add terms of service content"
```

---

### Task 8: Run Pint + full test suite

**Files:** None (validation only)

**Step 1: Run PHP formatter**

```bash
cd goformx-laravel && ddev exec vendor/bin/pint --dirty --format agent
```

**Step 2: Run full test suite**

```bash
cd goformx-laravel && ddev exec php artisan test --compact
```

Expected: All tests PASS

**Step 3: Run frontend linting**

```bash
cd goformx-laravel && ddev exec npm run lint && ddev exec npm run format
```

**Step 4: Run Wayfinder generation** (new routes added)

```bash
cd goformx-laravel && ddev exec php artisan wayfinder:generate
```

**Step 5: Commit any formatting/generated changes**

```bash
git add -A goformx-laravel/
git commit -m "chore: format and regenerate wayfinder routes"
```

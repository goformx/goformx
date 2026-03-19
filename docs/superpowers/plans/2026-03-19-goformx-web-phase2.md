# GoFormX Web Phase 2 — Controllers, Templates & Routes

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add SSR controllers for auth and public pages, Inertia controller for dashboard, security middleware, Twig templates, and full route registration.

**Working directory:** `/home/fsd42/dev/goformx/goformx-web`

---

## File Structure (additions to existing scaffold)

```
goformx-web/
├── src/
│   ├── Controller/
│   │   ├── AuthController.php         # Login, register, logout, forgot/reset, 2FA, verify email
│   │   ├── PublicController.php       # Home, pricing, privacy, terms, demo
│   │   └── DashboardController.php    # Inertia dashboard (authenticated)
│   ├── Middleware/
│   │   └── SecurityHeadersMiddleware.php
│   └── AppServiceProvider.php         # Updated with all routes
├── templates/
│   ├── layout.html.twig               # Base layout
│   ├── home.html.twig                 # Updated
│   ├── pricing.html.twig
│   ├── privacy.html.twig
│   ├── terms.html.twig
│   └── auth/
│       ├── login.html.twig
│       ├── register.html.twig
│       ├── forgot-password.html.twig
│       ├── reset-password.html.twig
│       ├── verify-email.html.twig
│       └── two-factor-challenge.html.twig
└── tests/
    └── Unit/
        ├── SecurityHeadersMiddlewareTest.php
        ├── AuthControllerTest.php
        └── PublicControllerTest.php
```

---

### Task 1: Base Layout Template

**Files:**
- Create: `templates/layout.html.twig`
- Modify: `templates/home.html.twig`

- [ ] **Step 1: Create base layout**

```twig
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{% block title %}GoFormX{% endblock %}</title>
    <style>
        *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
        body { font-family: system-ui, -apple-system, sans-serif; color: #1a1a2e; background: #fafafa; line-height: 1.6; }
        .container { max-width: 1100px; margin: 0 auto; padding: 0 1.5rem; }
        nav { background: #fff; border-bottom: 1px solid #e5e7eb; padding: 1rem 0; }
        nav .container { display: flex; justify-content: space-between; align-items: center; }
        nav a { color: #4b5563; text-decoration: none; font-size: 0.875rem; }
        nav a:hover { color: #1a1a2e; }
        .nav-brand { font-weight: 700; font-size: 1.125rem; color: #1a1a2e !important; }
        .nav-links { display: flex; gap: 1.5rem; align-items: center; }
        .btn { display: inline-block; padding: 0.5rem 1rem; border-radius: 0.375rem; font-size: 0.875rem; font-weight: 500; text-decoration: none; cursor: pointer; border: none; }
        .btn-primary { background: #2563eb; color: #fff; }
        .btn-primary:hover { background: #1d4ed8; }
        .btn-outline { border: 1px solid #d1d5db; color: #374151; background: #fff; }
        .btn-outline:hover { background: #f9fafb; }
        main { padding: 3rem 0; }
        footer { border-top: 1px solid #e5e7eb; padding: 2rem 0; color: #9ca3af; font-size: 0.875rem; text-align: center; }
        .form-group { margin-bottom: 1rem; }
        .form-group label { display: block; margin-bottom: 0.25rem; font-size: 0.875rem; font-weight: 500; color: #374151; }
        .form-group input { width: 100%; padding: 0.5rem 0.75rem; border: 1px solid #d1d5db; border-radius: 0.375rem; font-size: 0.875rem; }
        .form-group input:focus { outline: none; border-color: #2563eb; box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.1); }
        .card { background: #fff; border: 1px solid #e5e7eb; border-radius: 0.5rem; padding: 2rem; }
        .auth-page { max-width: 28rem; margin: 4rem auto; }
        .error { color: #dc2626; font-size: 0.875rem; margin-top: 0.25rem; }
        .alert { padding: 0.75rem 1rem; border-radius: 0.375rem; margin-bottom: 1rem; font-size: 0.875rem; }
        .alert-success { background: #f0fdf4; color: #166534; border: 1px solid #bbf7d0; }
        .alert-error { background: #fef2f2; color: #991b1b; border: 1px solid #fecaca; }
    </style>
    {% block head %}{% endblock %}
</head>
<body>
    <nav>
        <div class="container">
            <a href="/" class="nav-brand">GoFormX</a>
            <div class="nav-links">
                <a href="/pricing">Pricing</a>
                <a href="/docs">Docs</a>
                {% block nav_auth %}
                <a href="/login" class="btn btn-outline">Sign in</a>
                <a href="/register" class="btn btn-primary">Get started</a>
                {% endblock %}
            </div>
        </div>
    </nav>

    <main>
        <div class="container">
            {% block content %}{% endblock %}
        </div>
    </main>

    <footer>
        <div class="container">
            &copy; {{ "now"|date("Y") }} GoFormX &middot;
            <a href="/privacy">Privacy</a> &middot;
            <a href="/terms">Terms</a>
        </div>
    </footer>
</body>
</html>
```

- [ ] **Step 2: Update home.html.twig to extend layout**

```twig
{% extends "layout.html.twig" %}

{% block title %}GoFormX — Forms Management Platform{% endblock %}

{% block content %}
<div style="text-align: center; padding: 4rem 0;">
    <h1 style="font-size: 2.5rem; font-weight: 800; margin-bottom: 1rem;">Build forms. Collect submissions.<br>Ship faster.</h1>
    <p style="font-size: 1.125rem; color: #6b7280; max-width: 600px; margin: 0 auto 2rem;">
        GoFormX makes it easy to create, embed, and manage forms for your website or application.
    </p>
    <a href="/register" class="btn btn-primary" style="padding: 0.75rem 2rem; font-size: 1rem;">Get started free</a>
</div>
{% endblock %}
```

- [ ] **Step 3: Commit**

```bash
git add templates/
git commit -m "feat(web): add base layout and update home template"
```

---

### Task 2: Auth Templates

**Files:**
- Create: `templates/auth/login.html.twig`
- Create: `templates/auth/register.html.twig`
- Create: `templates/auth/forgot-password.html.twig`
- Create: `templates/auth/reset-password.html.twig`
- Create: `templates/auth/verify-email.html.twig`
- Create: `templates/auth/two-factor-challenge.html.twig`

- [ ] **Step 1: Create all auth templates**

**login.html.twig:**
```twig
{% extends "layout.html.twig" %}
{% block title %}Sign in — GoFormX{% endblock %}
{% block content %}
<div class="auth-page">
    <div class="card">
        <h1 style="font-size: 1.5rem; margin-bottom: 1.5rem;">Sign in</h1>
        {% if error is defined and error %}
        <div class="alert alert-error">{{ error }}</div>
        {% endif %}
        <form method="POST" action="/login">
            <input type="hidden" name="_csrf_token" value="{{ csrf_token }}">
            <div class="form-group">
                <label for="email">Email</label>
                <input type="email" id="email" name="email" value="{{ email|default('') }}" required autofocus>
            </div>
            <div class="form-group">
                <label for="password">Password</label>
                <input type="password" id="password" name="password" required>
            </div>
            <button type="submit" class="btn btn-primary" style="width: 100%; margin-top: 0.5rem;">Sign in</button>
        </form>
        <p style="text-align: center; margin-top: 1rem; font-size: 0.875rem; color: #6b7280;">
            <a href="/forgot-password">Forgot password?</a> &middot;
            <a href="/register">Create account</a>
        </p>
    </div>
</div>
{% endblock %}
```

**register.html.twig:**
```twig
{% extends "layout.html.twig" %}
{% block title %}Create account — GoFormX{% endblock %}
{% block content %}
<div class="auth-page">
    <div class="card">
        <h1 style="font-size: 1.5rem; margin-bottom: 1.5rem;">Create account</h1>
        {% if errors is defined %}
        <div class="alert alert-error">
            {% for error in errors %}<p>{{ error }}</p>{% endfor %}
        </div>
        {% endif %}
        <form method="POST" action="/register">
            <input type="hidden" name="_csrf_token" value="{{ csrf_token }}">
            <div class="form-group">
                <label for="name">Name</label>
                <input type="text" id="name" name="name" value="{{ name|default('') }}" required autofocus>
            </div>
            <div class="form-group">
                <label for="email">Email</label>
                <input type="email" id="email" name="email" value="{{ email|default('') }}" required>
            </div>
            <div class="form-group">
                <label for="password">Password</label>
                <input type="password" id="password" name="password" required minlength="8">
            </div>
            <div class="form-group">
                <label for="password_confirmation">Confirm password</label>
                <input type="password" id="password_confirmation" name="password_confirmation" required>
            </div>
            <button type="submit" class="btn btn-primary" style="width: 100%; margin-top: 0.5rem;">Create account</button>
        </form>
        <p style="text-align: center; margin-top: 1rem; font-size: 0.875rem; color: #6b7280;">
            Already have an account? <a href="/login">Sign in</a>
        </p>
    </div>
</div>
{% endblock %}
```

**forgot-password.html.twig:**
```twig
{% extends "layout.html.twig" %}
{% block title %}Forgot password — GoFormX{% endblock %}
{% block content %}
<div class="auth-page">
    <div class="card">
        <h1 style="font-size: 1.5rem; margin-bottom: 0.5rem;">Forgot password</h1>
        <p style="color: #6b7280; font-size: 0.875rem; margin-bottom: 1.5rem;">Enter your email and we'll send a reset link.</p>
        {% if status is defined and status %}
        <div class="alert alert-success">{{ status }}</div>
        {% endif %}
        {% if error is defined and error %}
        <div class="alert alert-error">{{ error }}</div>
        {% endif %}
        <form method="POST" action="/forgot-password">
            <input type="hidden" name="_csrf_token" value="{{ csrf_token }}">
            <div class="form-group">
                <label for="email">Email</label>
                <input type="email" id="email" name="email" required autofocus>
            </div>
            <button type="submit" class="btn btn-primary" style="width: 100%;">Send reset link</button>
        </form>
        <p style="text-align: center; margin-top: 1rem; font-size: 0.875rem;">
            <a href="/login">Back to sign in</a>
        </p>
    </div>
</div>
{% endblock %}
```

**reset-password.html.twig:**
```twig
{% extends "layout.html.twig" %}
{% block title %}Reset password — GoFormX{% endblock %}
{% block content %}
<div class="auth-page">
    <div class="card">
        <h1 style="font-size: 1.5rem; margin-bottom: 1.5rem;">Reset password</h1>
        {% if error is defined and error %}
        <div class="alert alert-error">{{ error }}</div>
        {% endif %}
        <form method="POST" action="/reset-password">
            <input type="hidden" name="_csrf_token" value="{{ csrf_token }}">
            <input type="hidden" name="token" value="{{ token }}">
            <div class="form-group">
                <label for="email">Email</label>
                <input type="email" id="email" name="email" value="{{ email|default('') }}" required>
            </div>
            <div class="form-group">
                <label for="password">New password</label>
                <input type="password" id="password" name="password" required minlength="8">
            </div>
            <div class="form-group">
                <label for="password_confirmation">Confirm password</label>
                <input type="password" id="password_confirmation" name="password_confirmation" required>
            </div>
            <button type="submit" class="btn btn-primary" style="width: 100%;">Reset password</button>
        </form>
    </div>
</div>
{% endblock %}
```

**verify-email.html.twig:**
```twig
{% extends "layout.html.twig" %}
{% block title %}Verify email — GoFormX{% endblock %}
{% block content %}
<div class="auth-page">
    <div class="card" style="text-align: center;">
        <h1 style="font-size: 1.5rem; margin-bottom: 1rem;">Verify your email</h1>
        <p style="color: #6b7280; margin-bottom: 1.5rem;">
            We've sent a verification link to your email address. Please click the link to verify your account.
        </p>
        {% if status is defined and status %}
        <div class="alert alert-success">{{ status }}</div>
        {% endif %}
        <form method="POST" action="/email/verification-notification">
            <input type="hidden" name="_csrf_token" value="{{ csrf_token }}">
            <button type="submit" class="btn btn-outline">Resend verification email</button>
        </form>
    </div>
</div>
{% endblock %}
```

**two-factor-challenge.html.twig:**
```twig
{% extends "layout.html.twig" %}
{% block title %}Two-factor authentication — GoFormX{% endblock %}
{% block content %}
<div class="auth-page">
    <div class="card">
        <h1 style="font-size: 1.5rem; margin-bottom: 0.5rem;">Two-factor authentication</h1>
        <p style="color: #6b7280; font-size: 0.875rem; margin-bottom: 1.5rem;">Enter the code from your authenticator app.</p>
        {% if error is defined and error %}
        <div class="alert alert-error">{{ error }}</div>
        {% endif %}
        <form method="POST" action="/two-factor-challenge">
            <input type="hidden" name="_csrf_token" value="{{ csrf_token }}">
            <div class="form-group">
                <label for="code">Authentication code</label>
                <input type="text" id="code" name="code" inputmode="numeric" pattern="[0-9]{6}" maxlength="6" required autofocus autocomplete="one-time-code">
            </div>
            <button type="submit" class="btn btn-primary" style="width: 100%;">Verify</button>
        </form>
        <hr style="margin: 1.5rem 0; border: none; border-top: 1px solid #e5e7eb;">
        <form method="POST" action="/two-factor-challenge">
            <input type="hidden" name="_csrf_token" value="{{ csrf_token }}">
            <div class="form-group">
                <label for="recovery_code">Or use a recovery code</label>
                <input type="text" id="recovery_code" name="recovery_code">
            </div>
            <button type="submit" class="btn btn-outline" style="width: 100%;">Use recovery code</button>
        </form>
    </div>
</div>
{% endblock %}
```

- [ ] **Step 2: Commit**

```bash
git add templates/auth/
git commit -m "feat(web): add SSR auth templates (login, register, forgot/reset, 2FA, verify)"
```

---

### Task 3: Public Page Templates

**Files:**
- Create: `templates/pricing.html.twig`
- Create: `templates/privacy.html.twig`
- Create: `templates/terms.html.twig`

- [ ] **Step 1: Create public templates**

**pricing.html.twig:**
```twig
{% extends "layout.html.twig" %}
{% block title %}Pricing — GoFormX{% endblock %}
{% block content %}
<div style="text-align: center; padding: 2rem 0;">
    <h1 style="font-size: 2rem; font-weight: 700; margin-bottom: 0.5rem;">Simple, transparent pricing</h1>
    <p style="color: #6b7280; margin-bottom: 3rem;">Start free, upgrade when you need more.</p>

    <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 1.5rem; max-width: 900px; margin: 0 auto;">
        <div class="card">
            <h2 style="font-size: 1.25rem; font-weight: 600;">Free</h2>
            <p style="font-size: 2rem; font-weight: 700; margin: 1rem 0;">$0<span style="font-size: 0.875rem; color: #6b7280;">/mo</span></p>
            <ul style="list-style: none; text-align: left; font-size: 0.875rem; color: #4b5563;">
                <li style="padding: 0.25rem 0;">5 forms</li>
                <li style="padding: 0.25rem 0;">100 submissions/mo</li>
            </ul>
            <a href="/register" class="btn btn-outline" style="width: 100%; margin-top: 1rem;">Get started</a>
        </div>
        <div class="card" style="border-color: #2563eb;">
            <h2 style="font-size: 1.25rem; font-weight: 600;">Pro</h2>
            <p style="font-size: 2rem; font-weight: 700; margin: 1rem 0;">$19<span style="font-size: 0.875rem; color: #6b7280;">/mo</span></p>
            <ul style="list-style: none; text-align: left; font-size: 0.875rem; color: #4b5563;">
                <li style="padding: 0.25rem 0;">Unlimited forms</li>
                <li style="padding: 0.25rem 0;">10,000 submissions/mo</li>
                <li style="padding: 0.25rem 0;">Priority support</li>
            </ul>
            <a href="/register" class="btn btn-primary" style="width: 100%; margin-top: 1rem;">Start free trial</a>
        </div>
        <div class="card">
            <h2 style="font-size: 1.25rem; font-weight: 600;">Business</h2>
            <p style="font-size: 2rem; font-weight: 700; margin: 1rem 0;">$49<span style="font-size: 0.875rem; color: #6b7280;">/mo</span></p>
            <ul style="list-style: none; text-align: left; font-size: 0.875rem; color: #4b5563;">
                <li style="padding: 0.25rem 0;">Everything in Pro</li>
                <li style="padding: 0.25rem 0;">100,000 submissions/mo</li>
                <li style="padding: 0.25rem 0;">Custom branding</li>
            </ul>
            <a href="/register" class="btn btn-outline" style="width: 100%; margin-top: 1rem;">Start free trial</a>
        </div>
    </div>
</div>
{% endblock %}
```

**privacy.html.twig:**
```twig
{% extends "layout.html.twig" %}
{% block title %}Privacy Policy — GoFormX{% endblock %}
{% block content %}
<div style="max-width: 700px; margin: 0 auto;">
    <h1 style="font-size: 2rem; font-weight: 700; margin-bottom: 1.5rem;">Privacy Policy</h1>
    <p style="color: #6b7280; margin-bottom: 1rem;">Last updated: March 2026</p>
    <p>GoFormX collects only the data necessary to provide our forms management service. We do not sell your data to third parties.</p>
</div>
{% endblock %}
```

**terms.html.twig:**
```twig
{% extends "layout.html.twig" %}
{% block title %}Terms of Service — GoFormX{% endblock %}
{% block content %}
<div style="max-width: 700px; margin: 0 auto;">
    <h1 style="font-size: 2rem; font-weight: 700; margin-bottom: 1.5rem;">Terms of Service</h1>
    <p style="color: #6b7280; margin-bottom: 1rem;">Last updated: March 2026</p>
    <p>By using GoFormX, you agree to these terms of service.</p>
</div>
{% endblock %}
```

- [ ] **Step 2: Commit**

```bash
git add templates/
git commit -m "feat(web): add public page templates (pricing, privacy, terms)"
```

---

### Task 4: SecurityHeadersMiddleware

**Files:**
- Create: `src/Middleware/SecurityHeadersMiddleware.php`
- Create: `tests/Unit/SecurityHeadersMiddlewareTest.php`

- [ ] **Step 1: Write the failing test**

```php
<?php

declare(strict_types=1);

namespace GoFormX\Tests\Unit;

use GoFormX\Middleware\SecurityHeadersMiddleware;
use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;
use Symfony\Component\HttpFoundation\Request;
use Symfony\Component\HttpFoundation\Response;
use Waaseyaa\Foundation\Middleware\HttpHandlerInterface;

#[CoversClass(SecurityHeadersMiddleware::class)]
final class SecurityHeadersMiddlewareTest extends TestCase
{
    public function testAddsSecurityHeaders(): void
    {
        $middleware = new SecurityHeadersMiddleware(isProduction: false);
        $request = Request::create('/');
        $handler = $this->createHandler(new Response('ok'));

        $response = $middleware->process($request, $handler);

        $this->assertSame('DENY', $response->headers->get('X-Frame-Options'));
        $this->assertSame('nosniff', $response->headers->get('X-Content-Type-Options'));
        $this->assertSame('strict-origin-when-cross-origin', $response->headers->get('Referrer-Policy'));
        $this->assertStringContainsString('camera=()', $response->headers->get('Permissions-Policy'));
    }

    public function testAddsHstsInProduction(): void
    {
        $middleware = new SecurityHeadersMiddleware(isProduction: true);
        $request = Request::create('/');
        $handler = $this->createHandler(new Response('ok'));

        $response = $middleware->process($request, $handler);

        $this->assertSame('max-age=31536000; includeSubDomains', $response->headers->get('Strict-Transport-Security'));
    }

    public function testNoHstsInDevelopment(): void
    {
        $middleware = new SecurityHeadersMiddleware(isProduction: false);
        $request = Request::create('/');
        $handler = $this->createHandler(new Response('ok'));

        $response = $middleware->process($request, $handler);

        $this->assertNull($response->headers->get('Strict-Transport-Security'));
    }

    public function testPassesThroughResponse(): void
    {
        $middleware = new SecurityHeadersMiddleware(isProduction: false);
        $request = Request::create('/');
        $handler = $this->createHandler(new Response('content', 200));

        $response = $middleware->process($request, $handler);

        $this->assertSame(200, $response->getStatusCode());
        $this->assertSame('content', $response->getContent());
    }

    private function createHandler(Response $response): HttpHandlerInterface
    {
        return new class ($response) implements HttpHandlerInterface {
            public function __construct(private readonly Response $response) {}
            public function handle(Request $request): Response { return $this->response; }
        };
    }
}
```

- [ ] **Step 2: Write the implementation**

```php
<?php

declare(strict_types=1);

namespace GoFormX\Middleware;

use Symfony\Component\HttpFoundation\Request;
use Symfony\Component\HttpFoundation\Response;
use Waaseyaa\Foundation\Middleware\HttpHandlerInterface;
use Waaseyaa\Foundation\Middleware\HttpMiddlewareInterface;

final class SecurityHeadersMiddleware implements HttpMiddlewareInterface
{
    public function __construct(
        private readonly bool $isProduction = false,
    ) {
    }

    public function process(Request $request, HttpHandlerInterface $next): Response
    {
        $response = $next->handle($request);

        $response->headers->set('X-Frame-Options', 'DENY');
        $response->headers->set('X-Content-Type-Options', 'nosniff');
        $response->headers->set('Referrer-Policy', 'strict-origin-when-cross-origin');
        $response->headers->set('Permissions-Policy', 'camera=(), microphone=(), geolocation=()');

        if ($this->isProduction) {
            $response->headers->set('Strict-Transport-Security', 'max-age=31536000; includeSubDomains');
        }

        return $response;
    }
}
```

- [ ] **Step 3: Run tests**

Run: `cd /home/fsd42/dev/goformx/goformx-web && vendor/bin/phpunit tests/Unit/SecurityHeadersMiddlewareTest.php`
Expected: All 4 tests PASS.

- [ ] **Step 4: Commit**

```bash
git add src/Middleware/ tests/Unit/SecurityHeadersMiddlewareTest.php
git commit -m "feat(web): add SecurityHeadersMiddleware"
```

---

### Task 5: PublicController

**Files:**
- Create: `src/Controller/PublicController.php`
- Create: `tests/Unit/PublicControllerTest.php`

SSR controller for public pages. Returns rendered Twig templates.

- [ ] **Step 1: Write the failing test**

```php
<?php

declare(strict_types=1);

namespace GoFormX\Tests\Unit;

use GoFormX\Controller\PublicController;
use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;

#[CoversClass(PublicController::class)]
final class PublicControllerTest extends TestCase
{
    public function testAvailablePages(): void
    {
        $controller = new PublicController();

        $this->assertSame('home.html.twig', $controller->templateFor('home'));
        $this->assertSame('pricing.html.twig', $controller->templateFor('pricing'));
        $this->assertSame('privacy.html.twig', $controller->templateFor('privacy'));
        $this->assertSame('terms.html.twig', $controller->templateFor('terms'));
    }

    public function testUnknownPageReturnsNull(): void
    {
        $controller = new PublicController();

        $this->assertNull($controller->templateFor('nonexistent'));
    }
}
```

- [ ] **Step 2: Write the implementation**

```php
<?php

declare(strict_types=1);

namespace GoFormX\Controller;

final class PublicController
{
    /** @var array<string, string> */
    private const array PAGE_TEMPLATES = [
        'home' => 'home.html.twig',
        'pricing' => 'pricing.html.twig',
        'privacy' => 'privacy.html.twig',
        'terms' => 'terms.html.twig',
    ];

    public function templateFor(string $page): ?string
    {
        return self::PAGE_TEMPLATES[$page] ?? null;
    }
}
```

- [ ] **Step 3: Run tests**

Run: `cd /home/fsd42/dev/goformx/goformx-web && vendor/bin/phpunit tests/Unit/PublicControllerTest.php`
Expected: All 2 tests PASS.

- [ ] **Step 4: Commit**

```bash
git add src/Controller/PublicController.php tests/Unit/PublicControllerTest.php
git commit -m "feat(web): add PublicController for SSR pages"
```

---

### Task 6: AuthController

**Files:**
- Create: `src/Controller/AuthController.php`
- Create: `tests/Unit/AuthControllerTest.php`

Handles auth form validation logic. Template rendering and session management are handled by the framework — this controller validates input and returns structured results.

- [ ] **Step 1: Write the failing test**

```php
<?php

declare(strict_types=1);

namespace GoFormX\Tests\Unit;

use GoFormX\Controller\AuthController;
use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;

#[CoversClass(AuthController::class)]
final class AuthControllerTest extends TestCase
{
    private AuthController $controller;

    protected function setUp(): void
    {
        $this->controller = new AuthController();
    }

    public function testValidateLoginReturnsErrorsForMissingFields(): void
    {
        $errors = $this->controller->validateLogin('', '');

        $this->assertContains('Email is required.', $errors);
        $this->assertContains('Password is required.', $errors);
    }

    public function testValidateLoginReturnsErrorForInvalidEmail(): void
    {
        $errors = $this->controller->validateLogin('not-an-email', 'password');

        $this->assertContains('Please enter a valid email address.', $errors);
    }

    public function testValidateLoginReturnsEmptyForValidInput(): void
    {
        $errors = $this->controller->validateLogin('alice@test.com', 'password123');

        $this->assertSame([], $errors);
    }

    public function testValidateRegistrationReturnsErrorsForMissingFields(): void
    {
        $errors = $this->controller->validateRegistration('', '', '', '');

        $this->assertContains('Name is required.', $errors);
        $this->assertContains('Email is required.', $errors);
        $this->assertContains('Password is required.', $errors);
    }

    public function testValidateRegistrationReturnsErrorForShortPassword(): void
    {
        $errors = $this->controller->validateRegistration('Alice', 'alice@test.com', 'short', 'short');

        $this->assertContains('Password must be at least 8 characters.', $errors);
    }

    public function testValidateRegistrationReturnsErrorForMismatchedPasswords(): void
    {
        $errors = $this->controller->validateRegistration('Alice', 'alice@test.com', 'password123', 'different');

        $this->assertContains('Passwords do not match.', $errors);
    }

    public function testValidateRegistrationReturnsEmptyForValidInput(): void
    {
        $errors = $this->controller->validateRegistration('Alice', 'alice@test.com', 'password123', 'password123');

        $this->assertSame([], $errors);
    }

    public function testValidatePasswordResetReturnsErrors(): void
    {
        $errors = $this->controller->validatePasswordReset('', 'short', 'different');

        $this->assertContains('Email is required.', $errors);
        $this->assertContains('Password must be at least 8 characters.', $errors);
        $this->assertContains('Passwords do not match.', $errors);
    }

    public function testValidatePasswordResetReturnsEmptyForValidInput(): void
    {
        $errors = $this->controller->validatePasswordReset('alice@test.com', 'newpassword', 'newpassword');

        $this->assertSame([], $errors);
    }
}
```

- [ ] **Step 2: Write the implementation**

```php
<?php

declare(strict_types=1);

namespace GoFormX\Controller;

final class AuthController
{
    private const int MIN_PASSWORD_LENGTH = 8;

    /**
     * @return list<string>
     */
    public function validateLogin(string $email, string $password): array
    {
        $errors = [];

        if ($email === '') {
            $errors[] = 'Email is required.';
        } elseif (!filter_var($email, FILTER_VALIDATE_EMAIL)) {
            $errors[] = 'Please enter a valid email address.';
        }

        if ($password === '') {
            $errors[] = 'Password is required.';
        }

        return $errors;
    }

    /**
     * @return list<string>
     */
    public function validateRegistration(
        string $name,
        string $email,
        string $password,
        string $passwordConfirmation,
    ): array {
        $errors = [];

        if ($name === '') {
            $errors[] = 'Name is required.';
        }

        if ($email === '') {
            $errors[] = 'Email is required.';
        } elseif (!filter_var($email, FILTER_VALIDATE_EMAIL)) {
            $errors[] = 'Please enter a valid email address.';
        }

        if ($password === '') {
            $errors[] = 'Password is required.';
        } elseif (strlen($password) < self::MIN_PASSWORD_LENGTH) {
            $errors[] = 'Password must be at least 8 characters.';
        } elseif ($password !== $passwordConfirmation) {
            $errors[] = 'Passwords do not match.';
        }

        return $errors;
    }

    /**
     * @return list<string>
     */
    public function validatePasswordReset(
        string $email,
        string $password,
        string $passwordConfirmation,
    ): array {
        $errors = [];

        if ($email === '') {
            $errors[] = 'Email is required.';
        } elseif (!filter_var($email, FILTER_VALIDATE_EMAIL)) {
            $errors[] = 'Please enter a valid email address.';
        }

        if (strlen($password) < self::MIN_PASSWORD_LENGTH) {
            $errors[] = 'Password must be at least 8 characters.';
        } elseif ($password !== $passwordConfirmation) {
            $errors[] = 'Passwords do not match.';
        }

        return $errors;
    }
}
```

- [ ] **Step 3: Run tests**

Run: `cd /home/fsd42/dev/goformx/goformx-web && vendor/bin/phpunit tests/Unit/AuthControllerTest.php`
Expected: All 9 tests PASS.

- [ ] **Step 4: Commit**

```bash
git add src/Controller/AuthController.php tests/Unit/AuthControllerTest.php
git commit -m "feat(web): add AuthController with form validation"
```

---

### Task 7: DashboardController

**Files:**
- Create: `src/Controller/DashboardController.php`

Minimal Inertia controller for the authenticated dashboard.

- [ ] **Step 1: Write the controller**

```php
<?php

declare(strict_types=1);

namespace GoFormX\Controller;

use Symfony\Component\HttpFoundation\Request;
use Waaseyaa\Inertia\Inertia;
use Waaseyaa\Inertia\InertiaResponse;

final class DashboardController
{
    public function index(Request $request): InertiaResponse
    {
        return Inertia::render('Dashboard', []);
    }
}
```

- [ ] **Step 2: Commit**

```bash
git add src/Controller/DashboardController.php
git commit -m "feat(web): add DashboardController with Inertia rendering"
```

---

### Task 8: Update AppServiceProvider with All Routes

**Files:**
- Modify: `src/AppServiceProvider.php`

Register all routes from the migration spec.

- [ ] **Step 1: Update AppServiceProvider**

```php
<?php

declare(strict_types=1);

namespace GoFormX;

use GoFormX\Controller\DashboardController;
use GoFormX\Middleware\SecurityHeadersMiddleware;
use GoFormX\Service\GoFormsClient;
use Symfony\Component\Routing\Route;
use Waaseyaa\Entity\EntityTypeManager;
use Waaseyaa\Foundation\Middleware\HttpMiddlewareInterface;
use Waaseyaa\Foundation\ServiceProvider\ServiceProvider;
use Waaseyaa\Routing\WaaseyaaRouter;

final class AppServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->singleton(GoFormsClient::class, fn() => new GoFormsClient(
            baseUrl: $this->config['goforms_api_url'] ?? 'http://localhost:8090',
            sharedSecret: $this->config['goforms_shared_secret'] ?? '',
        ));
    }

    /**
     * @return list<HttpMiddlewareInterface>
     */
    public function middleware(EntityTypeManager $entityTypeManager): array
    {
        $isProduction = ($this->config['app_env'] ?? 'local') === 'production';

        return [
            new SecurityHeadersMiddleware($isProduction),
        ];
    }

    public function routes(WaaseyaaRouter $router, ?EntityTypeManager $entityTypeManager = null): void
    {
        // Public SSR pages
        $router->addRoute('home', new Route('/', defaults: ['_controller' => 'render.page']));
        $router->addRoute('pricing', new Route('/pricing', defaults: ['_controller' => 'render.page']));
        $router->addRoute('privacy', new Route('/privacy', defaults: ['_controller' => 'render.page']));
        $router->addRoute('terms', new Route('/terms', defaults: ['_controller' => 'render.page']));

        // Auth SSR pages (GET)
        $router->addRoute('login', new Route('/login', defaults: ['_controller' => 'render.page'], methods: ['GET']));
        $router->addRoute('login.post', new Route('/login', defaults: ['_controller' => 'render.page'], methods: ['POST']));
        $router->addRoute('register', new Route('/register', defaults: ['_controller' => 'render.page'], methods: ['GET']));
        $router->addRoute('register.post', new Route('/register', defaults: ['_controller' => 'render.page'], methods: ['POST']));
        $router->addRoute('forgot-password', new Route('/forgot-password', defaults: ['_controller' => 'render.page'], methods: ['GET']));
        $router->addRoute('forgot-password.post', new Route('/forgot-password', defaults: ['_controller' => 'render.page'], methods: ['POST']));
        $router->addRoute('reset-password', new Route('/reset-password/{token}', defaults: ['_controller' => 'render.page'], methods: ['GET']));
        $router->addRoute('reset-password.post', new Route('/reset-password', defaults: ['_controller' => 'render.page'], methods: ['POST']));
        $router->addRoute('verify-email', new Route('/verify-email', defaults: ['_controller' => 'render.page']));
        $router->addRoute('verify-email.verify', new Route('/verify-email/{id}/{hash}', defaults: ['_controller' => 'render.page']));
        $router->addRoute('two-factor-challenge', new Route('/two-factor-challenge', defaults: ['_controller' => 'render.page'], methods: ['GET']));
        $router->addRoute('two-factor-challenge.post', new Route('/two-factor-challenge', defaults: ['_controller' => 'render.page'], methods: ['POST']));
        $router->addRoute('logout', new Route('/logout', defaults: ['_controller' => 'render.page'], methods: ['POST']));

        // Authenticated Inertia routes
        $router->addRoute('dashboard', new Route('/dashboard', defaults: [
            '_controller' => fn(\Symfony\Component\HttpFoundation\Request $request) => (new DashboardController())->index($request),
        ]));

        // Stripe webhook
        $router->addRoute('stripe.webhook', new Route('/stripe/webhook', defaults: ['_controller' => 'render.page'], methods: ['POST']));
    }
}
```

- [ ] **Step 2: Run all tests**

Run: `cd /home/fsd42/dev/goformx/goformx-web && vendor/bin/phpunit tests/`
Expected: All tests PASS.

- [ ] **Step 3: Commit and push**

```bash
git add src/AppServiceProvider.php
git commit -m "feat(web): register all routes and SecurityHeadersMiddleware in AppServiceProvider"
git push
```

---

## Summary

| Task | What it builds | Tests |
|---|---|---|
| 1 | Base layout template + updated home | — |
| 2 | Auth templates (login, register, forgot/reset, 2FA, verify) | — |
| 3 | Public page templates (pricing, privacy, terms) | — |
| 4 | SecurityHeadersMiddleware | 4 |
| 5 | PublicController | 2 |
| 6 | AuthController (form validation) | 9 |
| 7 | DashboardController (Inertia) | — |
| 8 | AppServiceProvider with all routes | — |

**Total: 8 tasks, ~15 new tests, 4 controllers, 9 templates, 1 middleware**

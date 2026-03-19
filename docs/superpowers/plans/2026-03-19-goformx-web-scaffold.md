# GoFormX Web Application Scaffold — Phase 1

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Scaffold the `goformx-web/` Waaseyaa application that replaces `goformx-laravel/`. Phase 1 gets the app booting, serving a basic page, and wired to the Go API via GoFormsClient.

**Architecture:** Waaseyaa HttpKernel entry point, custom AppServiceProvider for routes and services, GoFormsClient for HMAC-signed requests to the Go API, Docker Compose for development. SSR templates via Twig for public/auth pages, Inertia v3 for authenticated app pages (frontend deferred to Phase 2).

**Tech Stack:** PHP 8.4+, Waaseyaa framework, Docker Compose (PHP/nginx, MariaDB, Go API, PostgreSQL), Taskfile

**Working directory:** `/home/fsd42/dev/goformx` (goformx monorepo)

---

## File Structure

```
goformx-web/
├── composer.json
├── public/
│   └── index.php                  # Entry point
├── config/
│   └── waaseyaa.php               # Framework config
├── src/
│   ├── AppServiceProvider.php     # Routes, services, middleware
│   └── Service/
│       └── GoFormsClient.php      # HMAC-signed HTTP client for Go API
├── templates/
│   └── home.html.twig             # Basic home page template
├── storage/
│   └── .gitkeep                   # SQLite DB, cache, logs
├── docker/
│   ├── php/
│   │   └── Dockerfile             # PHP 8.4 + nginx
│   └── node/
│       └── Dockerfile             # Node.js for Vite
├── docker-compose.yml
└── Taskfile.yml
```

---

### Task 1: Directory Structure & Composer

**Files:**
- Create: `goformx-web/composer.json`
- Create: various directories

- [ ] **Step 1: Create directory structure**

```bash
cd /home/fsd42/dev/goformx
mkdir -p goformx-web/{public,config,src/Service,templates,storage,docker/php,docker/node}
touch goformx-web/storage/.gitkeep
```

- [ ] **Step 2: Create composer.json**

```json
{
    "name": "goformx/web",
    "description": "GoFormX web application — built on Waaseyaa",
    "type": "project",
    "license": "proprietary",
    "require": {
        "php": ">=8.4",
        "waaseyaa/foundation": "0.1.x-dev",
        "waaseyaa/user": "0.1.x-dev",
        "waaseyaa/auth": "0.1.x-dev",
        "waaseyaa/billing": "0.1.x-dev",
        "waaseyaa/inertia": "0.1.x-dev",
        "waaseyaa/routing": "0.1.x-dev",
        "waaseyaa/entity": "0.1.x-dev",
        "waaseyaa/entity-storage": "0.1.x-dev",
        "waaseyaa/access": "0.1.x-dev",
        "waaseyaa/config": "0.1.x-dev",
        "waaseyaa/mail": "0.1.x-dev",
        "waaseyaa/ssr": "0.1.x-dev",
        "waaseyaa/cache": "0.1.x-dev",
        "waaseyaa/plugin": "0.1.x-dev",
        "waaseyaa/typed-data": "0.1.x-dev",
        "waaseyaa/database-legacy": "0.1.x-dev",
        "waaseyaa/validation": "0.1.x-dev",
        "waaseyaa/field": "0.1.x-dev",
        "guzzlehttp/guzzle": "^7.0"
    },
    "require-dev": {
        "phpunit/phpunit": "^10.5"
    },
    "autoload": {
        "psr-4": {
            "GoFormX\\": "src/"
        }
    },
    "autoload-dev": {
        "psr-4": {
            "GoFormX\\Tests\\": "tests/"
        }
    },
    "minimum-stability": "dev",
    "prefer-stable": true
}
```

- [ ] **Step 3: Install dependencies**

Run: `cd /home/fsd42/dev/goformx/goformx-web && composer install`
Expected: All packages install successfully.

- [ ] **Step 4: Commit**

```bash
git add goformx-web/
git commit -m "feat(web): scaffold goformx-web directory structure and composer.json"
```

---

### Task 2: Waaseyaa Config & Entry Point

**Files:**
- Create: `goformx-web/config/waaseyaa.php`
- Create: `goformx-web/public/index.php`

- [ ] **Step 1: Create config/waaseyaa.php**

```php
<?php

declare(strict_types=1);

return [
    'app_name' => 'GoFormX',
    'app_env' => $_ENV['APP_ENV'] ?? 'local',
    'app_url' => $_ENV['APP_URL'] ?? 'http://localhost:8080',
    'app_secret' => $_ENV['APP_SECRET'] ?? 'change-me-in-production',

    // Database (SQLite for entity storage)
    'database' => $_ENV['WAASEYAA_DB'] ?? dirname(__DIR__) . '/storage/goformx.sqlite',

    // GoForms API
    'goforms_api_url' => $_ENV['GOFORMS_API_URL'] ?? 'http://localhost:8090',
    'goforms_shared_secret' => $_ENV['GOFORMS_SHARED_SECRET'] ?? '',
    'goforms_public_url' => $_ENV['GOFORMS_PUBLIC_URL'] ?? 'https://api.goformx.com',

    // Auth (waaseyaa/auth)
    'auth_secret' => $_ENV['APP_SECRET'] ?? 'change-me-in-production',
    'password_reset_lifetime' => 3600,
    'email_verification_lifetime' => 3600,

    // Billing (waaseyaa/billing)
    'stripe_key' => $_ENV['STRIPE_KEY'] ?? '',
    'stripe_secret' => $_ENV['STRIPE_SECRET'] ?? '',
    'stripe_webhook_secret' => $_ENV['STRIPE_WEBHOOK_SECRET'] ?? '',
    'billing_success_url' => ($_ENV['APP_URL'] ?? 'http://localhost:8080') . '/billing?success=true',
    'billing_cancel_url' => ($_ENV['APP_URL'] ?? 'http://localhost:8080') . '/billing?canceled=true',
    'billing_portal_return_url' => ($_ENV['APP_URL'] ?? 'http://localhost:8080') . '/billing',
    'billing_founding_member_cap' => (int) ($_ENV['FOUNDING_MEMBER_CAP'] ?? 100),
    'billing_price_tier_map' => [
        ($_ENV['STRIPE_GROWTH_MONTHLY_PRICE'] ?? '') => 'growth',
        ($_ENV['STRIPE_GROWTH_YEARLY_PRICE'] ?? '') => 'growth',
        ($_ENV['STRIPE_BUSINESS_MONTHLY_PRICE'] ?? '') => 'business',
        ($_ENV['STRIPE_BUSINESS_YEARLY_PRICE'] ?? '') => 'business',
        ($_ENV['STRIPE_PRO_MONTHLY_PRICE'] ?? '') => 'pro',
        ($_ENV['STRIPE_PRO_YEARLY_PRICE'] ?? '') => 'pro',
    ],

    // Inertia
    'inertia_version' => '',

    // CORS
    'cors_allowed_origins' => [],

    // Providers (auto-discovered from package manifests + app providers)
    'providers' => [
        GoFormX\AppServiceProvider::class,
    ],
];
```

- [ ] **Step 2: Create public/index.php**

```php
<?php

declare(strict_types=1);

require_once dirname(__DIR__) . '/vendor/autoload.php';

use Waaseyaa\Foundation\Kernel\HttpKernel;

try {
    $kernel = new HttpKernel(dirname(__DIR__));
    $kernel->handle();
} catch (\Throwable $e) {
    http_response_code(500);
    header('Content-Type: application/json');
    if (($_ENV['APP_ENV'] ?? 'production') !== 'production') {
        echo json_encode([
            'error' => $e->getMessage(),
            'file' => $e->getFile(),
            'line' => $e->getLine(),
        ]);
    } else {
        echo json_encode(['error' => 'Internal Server Error']);
    }
    exit(1);
}
```

- [ ] **Step 3: Commit**

```bash
git add goformx-web/config/ goformx-web/public/
git commit -m "feat(web): add Waaseyaa config and HTTP entry point"
```

---

### Task 3: GoFormsClient Service

**Files:**
- Create: `goformx-web/src/Service/GoFormsClient.php`
- Create: `goformx-web/tests/Unit/GoFormsClientTest.php`

HMAC-signed HTTP client for the Go API. Replicates the Laravel GoFormsClient signature format exactly.

- [ ] **Step 1: Write the failing test**

```php
<?php

declare(strict_types=1);

namespace GoFormX\Tests\Unit;

use GoFormX\Service\GoFormsClient;
use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;

#[CoversClass(GoFormsClient::class)]
final class GoFormsClientTest extends TestCase
{
    public function testBuildSignatureMatchesExpectedFormat(): void
    {
        $client = new GoFormsClient(
            baseUrl: 'http://localhost:8090',
            sharedSecret: 'test-secret',
        );

        $signature = $client->buildSignature('GET', '/api/forms', 'user-123', '2026-03-19T12:00:00Z', 'pro');

        $expected = hash_hmac('sha256', 'GET:/api/forms:user-123:2026-03-19T12:00:00Z:pro', 'test-secret');
        $this->assertSame($expected, $signature);
    }

    public function testBuildHeadersContainsRequiredHeaders(): void
    {
        $client = new GoFormsClient(
            baseUrl: 'http://localhost:8090',
            sharedSecret: 'test-secret',
        );

        $headers = $client->buildHeaders('GET', '/api/forms', 'user-123', 'free');

        $this->assertSame('user-123', $headers['X-User-Id']);
        $this->assertSame('free', $headers['X-Plan-Tier']);
        $this->assertArrayHasKey('X-Timestamp', $headers);
        $this->assertArrayHasKey('X-Signature', $headers);
    }

    public function testBuildHeadersTimestampIsUtc(): void
    {
        $client = new GoFormsClient(
            baseUrl: 'http://localhost:8090',
            sharedSecret: 'test-secret',
        );

        $headers = $client->buildHeaders('GET', '/api/forms', 'user-123', 'free');

        $this->assertStringEndsWith('Z', $headers['X-Timestamp']);
    }

    public function testBuildUrlCombinesBaseAndPath(): void
    {
        $client = new GoFormsClient(
            baseUrl: 'http://localhost:8090',
            sharedSecret: 'test-secret',
        );

        $this->assertSame('http://localhost:8090/api/forms', $client->buildUrl('/api/forms'));
    }

    public function testBuildUrlHandlesTrailingSlash(): void
    {
        $client = new GoFormsClient(
            baseUrl: 'http://localhost:8090/',
            sharedSecret: 'test-secret',
        );

        $this->assertSame('http://localhost:8090/api/forms', $client->buildUrl('/api/forms'));
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/goformx/goformx-web && vendor/bin/phpunit tests/Unit/GoFormsClientTest.php`
Expected: FAIL — class not found.

- [ ] **Step 3: Write the implementation**

```php
<?php

declare(strict_types=1);

namespace GoFormX\Service;

final class GoFormsClient
{
    public function __construct(
        private readonly string $baseUrl,
        private readonly string $sharedSecret,
    ) {
    }

    /**
     * Build HMAC signature for a request.
     *
     * Payload: METHOD:PATH:USER_ID:TIMESTAMP:PLAN_TIER
     */
    public function buildSignature(
        string $method,
        string $path,
        string $userId,
        string $timestamp,
        string $planTier,
    ): string {
        $payload = implode(':', [$method, $path, $userId, $timestamp, $planTier]);

        return hash_hmac('sha256', $payload, $this->sharedSecret);
    }

    /**
     * Build auth headers for a request to the Go API.
     *
     * @return array<string, string>
     */
    public function buildHeaders(
        string $method,
        string $path,
        string $userId,
        string $planTier,
    ): array {
        $timestamp = gmdate('Y-m-d\TH:i:s\Z');
        $signature = $this->buildSignature($method, $path, $userId, $timestamp, $planTier);

        return [
            'X-User-Id' => $userId,
            'X-Timestamp' => $timestamp,
            'X-Signature' => $signature,
            'X-Plan-Tier' => $planTier,
        ];
    }

    /**
     * Build full URL from base + path.
     */
    public function buildUrl(string $path): string
    {
        return rtrim($this->baseUrl, '/') . $path;
    }

    /**
     * Make an authenticated GET request to the Go API.
     *
     * @return array<string, mixed>
     * @throws \RuntimeException on HTTP error
     */
    public function get(string $path, string $userId, string $planTier): array
    {
        return $this->request('GET', $path, $userId, $planTier);
    }

    /**
     * Make an authenticated POST request to the Go API.
     *
     * @param array<string, mixed> $body
     * @return array<string, mixed>
     * @throws \RuntimeException on HTTP error
     */
    public function post(string $path, string $userId, string $planTier, array $body = []): array
    {
        return $this->request('POST', $path, $userId, $planTier, $body);
    }

    /**
     * Make an authenticated PUT request to the Go API.
     *
     * @param array<string, mixed> $body
     * @return array<string, mixed>
     * @throws \RuntimeException on HTTP error
     */
    public function put(string $path, string $userId, string $planTier, array $body = []): array
    {
        return $this->request('PUT', $path, $userId, $planTier, $body);
    }

    /**
     * Make an authenticated DELETE request to the Go API.
     *
     * @return array<string, mixed>
     * @throws \RuntimeException on HTTP error
     */
    public function delete(string $path, string $userId, string $planTier): array
    {
        return $this->request('DELETE', $path, $userId, $planTier);
    }

    /**
     * @param array<string, mixed> $body
     * @return array<string, mixed>
     */
    private function request(
        string $method,
        string $path,
        string $userId,
        string $planTier,
        array $body = [],
    ): array {
        $url = $this->buildUrl($path);
        $headers = $this->buildHeaders($method, $path, $userId, $planTier);
        $headers['Content-Type'] = 'application/json';
        $headers['Accept'] = 'application/json';

        $context = stream_context_create([
            'http' => [
                'method' => $method,
                'header' => $this->formatHeaders($headers),
                'content' => $body !== [] ? json_encode($body, JSON_THROW_ON_ERROR) : '',
                'ignore_errors' => true,
                'timeout' => 10,
            ],
        ]);

        $response = file_get_contents($url, false, $context);
        if ($response === false) {
            throw new \RuntimeException("GoForms API request failed: {$method} {$path}");
        }

        $statusCode = $this->extractStatusCode($http_response_header ?? []);

        if ($statusCode >= 400) {
            throw new \RuntimeException(
                "GoForms API error {$statusCode}: {$method} {$path}",
                $statusCode,
            );
        }

        return json_decode($response, true, 512, JSON_THROW_ON_ERROR);
    }

    /**
     * @param array<string, string> $headers
     */
    private function formatHeaders(array $headers): string
    {
        $lines = [];
        foreach ($headers as $name => $value) {
            $lines[] = "{$name}: {$value}";
        }

        return implode("\r\n", $lines);
    }

    /**
     * @param list<string> $responseHeaders
     */
    private function extractStatusCode(array $responseHeaders): int
    {
        foreach ($responseHeaders as $header) {
            if (preg_match('/^HTTP\/\S+\s+(\d{3})/', $header, $matches)) {
                return (int) $matches[1];
            }
        }

        return 500;
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/fsd42/dev/goformx/goformx-web && vendor/bin/phpunit tests/Unit/GoFormsClientTest.php`
Expected: All 5 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add goformx-web/src/ goformx-web/tests/
git commit -m "feat(web): add GoFormsClient with HMAC assertion signing"
```

---

### Task 4: AppServiceProvider

**Files:**
- Create: `goformx-web/src/AppServiceProvider.php`

Registers GoFormsClient, routes, and app-level config.

- [ ] **Step 1: Create AppServiceProvider**

```php
<?php

declare(strict_types=1);

namespace GoFormX;

use GoFormX\Service\GoFormsClient;
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

    public function routes(WaaseyaaRouter $router, ?\Waaseyaa\Entity\EntityTypeManager $entityTypeManager = null): void
    {
        $router->addRoute('home', new \Symfony\Component\Routing\Route(
            path: '/',
            defaults: ['_controller' => 'render.page'],
        ));
    }
}
```

- [ ] **Step 2: Commit**

```bash
git add goformx-web/src/AppServiceProvider.php
git commit -m "feat(web): add AppServiceProvider with GoFormsClient and routes"
```

---

### Task 5: Home Page Template

**Files:**
- Create: `goformx-web/templates/home.html.twig`

A minimal SSR template that proves the rendering pipeline works.

- [ ] **Step 1: Create home template**

```twig
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>GoFormX</title>
    <style>
        body { font-family: system-ui, sans-serif; max-width: 800px; margin: 4rem auto; padding: 0 1rem; color: #1a1a1a; }
        h1 { font-size: 2rem; margin-bottom: 0.5rem; }
        p { color: #666; line-height: 1.6; }
        a { color: #2563eb; text-decoration: none; }
        a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <h1>GoFormX</h1>
    <p>Forms management platform — powered by <a href="https://github.com/waaseyaa">Waaseyaa</a>.</p>
    <p><a href="/login">Sign in</a> | <a href="/register">Create account</a></p>
</body>
</html>
```

- [ ] **Step 2: Commit**

```bash
git add goformx-web/templates/
git commit -m "feat(web): add home page template"
```

---

### Task 6: Docker Compose

**Files:**
- Create: `goformx-web/docker-compose.yml`
- Create: `goformx-web/docker/php/Dockerfile`
- Create: `goformx-web/docker/php/nginx.conf`

- [ ] **Step 1: Create PHP/nginx Dockerfile**

```dockerfile
FROM php:8.4-fpm-alpine

RUN apk add --no-cache nginx supervisor curl

# PHP extensions
RUN docker-php-ext-install pdo pdo_mysql opcache

# Composer
COPY --from=composer:2 /usr/bin/composer /usr/bin/composer

WORKDIR /app

# Nginx config
COPY nginx.conf /etc/nginx/http.d/default.conf

# Supervisor config
RUN printf "[supervisord]\nnodaemon=true\n\n[program:php-fpm]\ncommand=php-fpm\n\n[program:nginx]\ncommand=nginx -g 'daemon off;'\n" > /etc/supervisor/conf.d/supervisord.conf

EXPOSE 80

CMD ["supervisord", "-c", "/etc/supervisor/conf.d/supervisord.conf"]
```

- [ ] **Step 2: Create nginx config**

```nginx
server {
    listen 80;
    server_name _;
    root /app/public;
    index index.php;

    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }

    location ~ \.php$ {
        fastcgi_pass 127.0.0.1:9000;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        include fastcgi_params;
    }

    location ~ /\.(?!well-known) {
        deny all;
    }
}
```

- [ ] **Step 3: Create docker-compose.yml**

```yaml
services:
  web:
    build:
      context: ./docker/php
    volumes:
      - .:/app
    ports:
      - "8080:80"
    depends_on:
      - mariadb
    environment:
      - APP_ENV=local
      - APP_URL=http://localhost:8080
      - APP_SECRET=local-dev-secret-change-in-production
      - GOFORMS_API_URL=http://goforms:8090
      - GOFORMS_SHARED_SECRET=${GOFORMS_SHARED_SECRET:-}
      - GOFORMS_PUBLIC_URL=http://localhost:8091
      - STRIPE_KEY=${STRIPE_KEY:-}
      - STRIPE_SECRET=${STRIPE_SECRET:-}
      - STRIPE_WEBHOOK_SECRET=${STRIPE_WEBHOOK_SECRET:-}

  mariadb:
    image: mariadb:11.8
    volumes:
      - mariadb_data:/var/lib/mysql
    ports:
      - "3307:3306"
    environment:
      MARIADB_DATABASE: goformx
      MARIADB_USER: goformx
      MARIADB_PASSWORD: goformx
      MARIADB_ROOT_PASSWORD: root

  goforms:
    build:
      context: ../goforms
      dockerfile: Dockerfile
    ports:
      - "8091:8090"
    depends_on:
      - postgres
    environment:
      DATABASE_URL: postgres://goforms:goforms@postgres:5432/goforms?sslmode=disable

  postgres:
    image: postgres:17
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5433:5432"
    environment:
      POSTGRES_DB: goforms
      POSTGRES_USER: goforms
      POSTGRES_PASSWORD: goforms

  mailpit:
    image: axllent/mailpit
    ports:
      - "8025:8025"
      - "1025:1025"

volumes:
  mariadb_data:
  postgres_data:
```

- [ ] **Step 4: Commit**

```bash
git add goformx-web/docker/ goformx-web/docker-compose.yml
git commit -m "feat(web): add Docker Compose with PHP, MariaDB, Go API, and Mailpit"
```

---

### Task 7: Taskfile

**Files:**
- Create: `goformx-web/Taskfile.yml`

- [ ] **Step 1: Create Taskfile**

```yaml
version: '3'

tasks:
  dev:
    desc: Start development environment
    cmds:
      - docker compose up -d

  stop:
    desc: Stop development environment
    cmds:
      - docker compose down

  setup:
    desc: First-time setup — install deps, build containers
    cmds:
      - docker compose build
      - docker compose up -d
      - docker compose exec web composer install
      - echo "Setup complete. Visit http://localhost:8080"

  logs:
    desc: Tail all container logs
    cmds:
      - docker compose logs -f

  test:
    desc: Run PHP tests
    cmds:
      - docker compose exec web vendor/bin/phpunit tests/

  lint:
    desc: Run PHP CS Fixer
    cmds:
      - docker compose exec web vendor/bin/php-cs-fixer fix --dry-run --diff

  shell:
    desc: Open shell in PHP container
    cmds:
      - docker compose exec web sh

  composer:
    desc: Run composer command in container
    cmds:
      - docker compose exec web composer {{.CLI_ARGS}}
```

- [ ] **Step 2: Create .env.example**

```bash
APP_ENV=local
APP_URL=http://localhost:8080
APP_SECRET=change-me-in-production

GOFORMS_API_URL=http://goforms:8090
GOFORMS_SHARED_SECRET=
GOFORMS_PUBLIC_URL=http://localhost:8091

STRIPE_KEY=
STRIPE_SECRET=
STRIPE_WEBHOOK_SECRET=
FOUNDING_MEMBER_CAP=100

STRIPE_GROWTH_MONTHLY_PRICE=
STRIPE_GROWTH_YEARLY_PRICE=
STRIPE_BUSINESS_MONTHLY_PRICE=
STRIPE_BUSINESS_YEARLY_PRICE=
STRIPE_PRO_MONTHLY_PRICE=
STRIPE_PRO_YEARLY_PRICE=
```

- [ ] **Step 3: Create .gitignore**

```
/vendor/
/storage/*.sqlite
/node_modules/
.env
```

- [ ] **Step 4: Commit**

```bash
git add goformx-web/Taskfile.yml goformx-web/.env.example goformx-web/.gitignore
git commit -m "feat(web): add Taskfile, .env.example, and .gitignore"
```

---

## Summary

| Task | What it builds |
|---|---|
| 1 | Directory structure + composer.json with all Waaseyaa deps |
| 2 | Config (waaseyaa.php) + entry point (index.php) |
| 3 | GoFormsClient with HMAC signing (5 tests) |
| 4 | AppServiceProvider with routes and service registration |
| 5 | Home page Twig template |
| 6 | Docker Compose (PHP, MariaDB, Go API, PostgreSQL, Mailpit) |
| 7 | Taskfile, .env.example, .gitignore |

**Total: 7 tasks, 5 tests, Phase 1 scaffold**

Phase 2 (next) will add: controllers (auth, dashboard, forms, settings, billing), SSR auth templates, Inertia + Vue 3 frontend, and full route registration.

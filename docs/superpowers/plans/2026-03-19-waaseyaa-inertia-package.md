# `waaseyaa/inertia` Package Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a server-side Inertia v3 protocol adapter as a Waaseyaa Layer 6 (Interfaces) package.

**Architecture:** The package provides middleware that detects Inertia requests (`X-Inertia` header) and returns JSON page objects instead of HTML. For initial page loads, it renders a root HTML template with the page object embedded in a `<script type="application/json">` tag. Controllers use `Inertia::render('Component', $props)` to build responses. Shared props are registered via a service provider method.

**Tech Stack:** PHP 8.4+, Symfony HttpFoundation, Waaseyaa foundation/routing packages, PHPUnit 10.5

**Spec:** `docs/superpowers/specs/2026-03-19-laravel-to-waaseyaa-migration-design.md` (Section: `waaseyaa/inertia`)

**Working directory:** `/home/fsd42/dev/waaseyaa`

---

## File Structure

```
packages/inertia/
├── composer.json
├── src/
│   ├── InertiaServiceProvider.php       # Registers middleware, routes config
│   ├── Inertia.php                      # Static facade: render(), share(), version()
│   ├── InertiaResponse.php              # Value object: component + props + url + version + options
│   ├── InertiaMiddleware.php            # HttpMiddlewareInterface — protocol core
│   ├── RootTemplateRenderer.php         # Renders initial HTML page with embedded page object
│   └── PropResolver.php                 # Resolves optional/deferred/merge props
└── tests/
    └── Unit/
        ├── InertiaResponseTest.php
        ├── InertiaMiddlewareTest.php
        ├── RootTemplateRendererTest.php
        ├── PropResolverTest.php
        └── InertiaTest.php
```

---

### Task 1: Package Scaffold

**Files:**
- Create: `packages/inertia/composer.json`
- Create: `packages/inertia/src/InertiaServiceProvider.php`

- [ ] **Step 1: Create composer.json**

```json
{
    "name": "waaseyaa/inertia",
    "description": "Server-side Inertia.js v3 protocol adapter for Waaseyaa",
    "type": "library",
    "license": "GPL-2.0-or-later",
    "repositories": [
        {
            "type": "path",
            "url": "../foundation"
        }
    ],
    "require": {
        "php": ">=8.4",
        "waaseyaa/foundation": "@dev"
    },
    "require-dev": {
        "phpunit/phpunit": "^10.5"
    },
    "autoload": {
        "psr-4": {
            "Waaseyaa\\Inertia\\": "src/"
        }
    },
    "autoload-dev": {
        "psr-4": {
            "Waaseyaa\\Inertia\\Tests\\": "tests/"
        }
    },
    "extra": {
        "waaseyaa": {
            "providers": [
                "Waaseyaa\\Inertia\\InertiaServiceProvider"
            ]
        },
        "branch-alias": {
            "dev-main": "0.1.x-dev"
        }
    },
    "minimum-stability": "dev",
    "prefer-stable": true
}
```

- [ ] **Step 2: Create minimal service provider**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Inertia;

use Waaseyaa\Foundation\ServiceProvider\ServiceProvider;

final class InertiaServiceProvider extends ServiceProvider
{
    public function register(): void
    {
    }
}
```

- [ ] **Step 3: Register package in root composer.json**

Add to root `/home/fsd42/dev/waaseyaa/composer.json`:
- Add path repository: `{ "type": "path", "url": "packages/inertia" }`
- Add to require: `"waaseyaa/inertia": "@dev"`

- [ ] **Step 4: Run composer update to verify wiring**

Run: `cd /home/fsd42/dev/waaseyaa && composer update waaseyaa/inertia`
Expected: Package resolves and installs without errors.

- [ ] **Step 5: Commit**

```bash
git add packages/inertia/ composer.json composer.lock
git commit -m "feat(inertia): scaffold waaseyaa/inertia package"
```

---

### Task 2: InertiaResponse Value Object

**Files:**
- Create: `packages/inertia/src/InertiaResponse.php`
- Create: `packages/inertia/tests/Unit/InertiaResponseTest.php`

- [ ] **Step 1: Write the failing test**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Inertia\Tests\Unit;

use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;
use Waaseyaa\Inertia\InertiaResponse;

#[CoversClass(InertiaResponse::class)]
final class InertiaResponseTest extends TestCase
{
    public function testBasicPageObject(): void
    {
        $response = new InertiaResponse(
            component: 'Users/Index',
            props: ['users' => [['id' => 1, 'name' => 'Alice']]],
            url: '/users',
            version: 'abc123',
        );

        $page = $response->toPageObject();

        $this->assertSame('Users/Index', $page['component']);
        $this->assertSame('/users', $page['url']);
        $this->assertSame('abc123', $page['version']);
        $this->assertSame([['id' => 1, 'name' => 'Alice']], $page['props']['users']);
        $this->assertArrayNotHasKey('encryptHistory', $page);
        $this->assertArrayNotHasKey('clearHistory', $page);
    }

    public function testPageObjectWithEncryptHistory(): void
    {
        $response = new InertiaResponse(
            component: 'Dashboard',
            props: [],
            url: '/dashboard',
            version: 'v1',
            encryptHistory: true,
        );

        $page = $response->toPageObject();

        $this->assertTrue($page['encryptHistory']);
    }

    public function testPageObjectWithClearHistory(): void
    {
        $response = new InertiaResponse(
            component: 'Login',
            props: [],
            url: '/login',
            version: 'v1',
            clearHistory: true,
        );

        $page = $response->toPageObject();

        $this->assertTrue($page['clearHistory']);
    }

    public function testPageObjectWithDeferredProps(): void
    {
        $response = new InertiaResponse(
            component: 'Posts/Index',
            props: ['posts' => []],
            url: '/posts',
            version: 'v1',
            deferredProps: ['default' => ['comments', 'analytics']],
        );

        $page = $response->toPageObject();

        $this->assertSame(['default' => ['comments', 'analytics']], $page['deferredProps']);
    }

    public function testPageObjectWithMergeProps(): void
    {
        $response = new InertiaResponse(
            component: 'Feed/Index',
            props: ['posts' => []],
            url: '/feed',
            version: 'v1',
            mergeProps: ['posts'],
            prependProps: ['notifications'],
        );

        $page = $response->toPageObject();

        $this->assertSame(['posts'], $page['mergeProps']);
        $this->assertSame(['notifications'], $page['prependProps']);
    }

    public function testPageObjectOmitsEmptyOptionalFields(): void
    {
        $response = new InertiaResponse(
            component: 'Home',
            props: [],
            url: '/',
            version: 'v1',
        );

        $page = $response->toPageObject();

        $this->assertArrayNotHasKey('deferredProps', $page);
        $this->assertArrayNotHasKey('mergeProps', $page);
        $this->assertArrayNotHasKey('prependProps', $page);
        $this->assertArrayNotHasKey('deepMergeProps', $page);
        $this->assertArrayNotHasKey('onceProps', $page);
        $this->assertArrayNotHasKey('encryptHistory', $page);
        $this->assertArrayNotHasKey('clearHistory', $page);
    }

    public function testPropsIncludeErrorsDefault(): void
    {
        $response = new InertiaResponse(
            component: 'Home',
            props: ['title' => 'Welcome'],
            url: '/',
            version: 'v1',
        );

        $page = $response->toPageObject();

        $this->assertSame([], $page['props']['errors']);
        $this->assertSame('Welcome', $page['props']['title']);
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/inertia/tests/Unit/InertiaResponseTest.php`
Expected: FAIL — `InertiaResponse` class not found.

- [ ] **Step 3: Write the implementation**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Inertia;

final class InertiaResponse
{
    /**
     * @param array<string, mixed> $props
     * @param array<string, list<string>> $deferredProps
     * @param list<string> $mergeProps
     * @param list<string> $prependProps
     * @param list<string> $deepMergeProps
     * @param array<string, mixed> $onceProps
     */
    public function __construct(
        public readonly string $component,
        public readonly array $props,
        public readonly string $url,
        public readonly string $version,
        public readonly bool $encryptHistory = false,
        public readonly bool $clearHistory = false,
        public readonly bool $preserveFragment = false,
        public readonly array $deferredProps = [],
        public readonly array $mergeProps = [],
        public readonly array $prependProps = [],
        public readonly array $deepMergeProps = [],
        public readonly array $onceProps = [],
    ) {
    }

    /**
     * @return array<string, mixed>
     */
    public function toPageObject(): array
    {
        $props = $this->props;
        $props['errors'] ??= [];

        $page = [
            'component' => $this->component,
            'props' => $props,
            'url' => $this->url,
            'version' => $this->version,
        ];

        if ($this->encryptHistory) {
            $page['encryptHistory'] = true;
        }

        if ($this->clearHistory) {
            $page['clearHistory'] = true;
        }

        if ($this->preserveFragment) {
            $page['preserveFragment'] = true;
        }

        if ($this->deferredProps !== []) {
            $page['deferredProps'] = $this->deferredProps;
        }

        if ($this->mergeProps !== []) {
            $page['mergeProps'] = $this->mergeProps;
        }

        if ($this->prependProps !== []) {
            $page['prependProps'] = $this->prependProps;
        }

        if ($this->deepMergeProps !== []) {
            $page['deepMergeProps'] = $this->deepMergeProps;
        }

        if ($this->onceProps !== []) {
            $page['onceProps'] = $this->onceProps;
        }

        return $page;
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/inertia/tests/Unit/InertiaResponseTest.php`
Expected: All 7 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/inertia/src/InertiaResponse.php packages/inertia/tests/Unit/InertiaResponseTest.php
git commit -m "feat(inertia): add InertiaResponse value object with v3 page object support"
```

---

### Task 3: RootTemplateRenderer

**Files:**
- Create: `packages/inertia/src/RootTemplateRenderer.php`
- Create: `packages/inertia/tests/Unit/RootTemplateRendererTest.php`

- [ ] **Step 1: Write the failing test**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Inertia\Tests\Unit;

use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;
use Waaseyaa\Inertia\RootTemplateRenderer;

#[CoversClass(RootTemplateRenderer::class)]
final class RootTemplateRendererTest extends TestCase
{
    public function testRendersHtmlWithPageObject(): void
    {
        $renderer = new RootTemplateRenderer();
        $pageObject = [
            'component' => 'Home',
            'props' => ['errors' => []],
            'url' => '/',
            'version' => 'v1',
        ];

        $html = $renderer->render($pageObject);

        $this->assertStringContainsString('<!DOCTYPE html>', $html);
        $this->assertStringContainsString('<div id="app">', $html);
        $this->assertStringContainsString('<script type="application/json" data-page="true">', $html);
        $this->assertStringContainsString('"component":"Home"', $html);
    }

    public function testEscapesHtmlInPageObject(): void
    {
        $renderer = new RootTemplateRenderer();
        $pageObject = [
            'component' => 'Home',
            'props' => ['errors' => [], 'html' => '<script>alert("xss")</script>'],
            'url' => '/',
            'version' => 'v1',
        ];

        $html = $renderer->render($pageObject);

        $this->assertStringNotContainsString('<script>alert("xss")</script>', $html);
        $this->assertStringContainsString('\u003Cscript\u003E', $html);
    }

    public function testCustomTemplateCallback(): void
    {
        $renderer = new RootTemplateRenderer(
            template: fn (string $pageJson) => "<html><body><div id=\"app\"></div>{$pageJson}</body></html>",
        );
        $pageObject = [
            'component' => 'Test',
            'props' => ['errors' => []],
            'url' => '/test',
            'version' => 'v1',
        ];

        $html = $renderer->render($pageObject);

        $this->assertStringContainsString('<div id="app"></div>', $html);
        $this->assertStringContainsString('"component":"Test"', $html);
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/inertia/tests/Unit/RootTemplateRendererTest.php`
Expected: FAIL — class not found.

- [ ] **Step 3: Write the implementation**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Inertia;

final class RootTemplateRenderer
{
    /** @var (\Closure(string): string)|null */
    private ?\Closure $template;

    /**
     * @param (\Closure(string): string)|null $template Custom template callback.
     *   Receives the JSON-encoded page script tag, returns full HTML string.
     *   If null, uses a minimal default template.
     */
    public function __construct(?\Closure $template = null)
    {
        $this->template = $template;
    }

    /**
     * @param array<string, mixed> $pageObject
     */
    public function render(array $pageObject): string
    {
        $json = json_encode($pageObject, JSON_THROW_ON_ERROR | JSON_HEX_TAG | JSON_HEX_APOS | JSON_HEX_QUOT | JSON_HEX_AMP | JSON_UNESCAPED_UNICODE);
        $scriptTag = '<script type="application/json" data-page="true">' . $json . '</script>';

        if ($this->template !== null) {
            return ($this->template)($scriptTag);
        }

        return $this->defaultTemplate($scriptTag);
    }

    private function defaultTemplate(string $scriptTag): string
    {
        return <<<HTML
        <!DOCTYPE html>
        <html>
        <head>
            <meta charset="utf-8">
            <meta name="viewport" content="width=device-width, initial-scale=1">
        </head>
        <body>
            <div id="app"></div>
            {$scriptTag}
        </body>
        </html>
        HTML;
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/inertia/tests/Unit/RootTemplateRendererTest.php`
Expected: All 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/inertia/src/RootTemplateRenderer.php packages/inertia/tests/Unit/RootTemplateRendererTest.php
git commit -m "feat(inertia): add RootTemplateRenderer for initial page loads"
```

---

### Task 4: PropResolver

**Files:**
- Create: `packages/inertia/src/PropResolver.php`
- Create: `packages/inertia/tests/Unit/PropResolverTest.php`

- [ ] **Step 1: Write the failing test**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Inertia\Tests\Unit;

use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;
use Waaseyaa\Inertia\PropResolver;

#[CoversClass(PropResolver::class)]
final class PropResolverTest extends TestCase
{
    public function testResolvesClosureProps(): void
    {
        $resolver = new PropResolver();
        $props = [
            'static' => 'value',
            'lazy' => fn () => 'computed',
        ];

        $resolved = $resolver->resolve($props);

        $this->assertSame('value', $resolved['static']);
        $this->assertSame('computed', $resolved['lazy']);
    }

    public function testOptionalPropsExcludedByDefault(): void
    {
        $resolver = new PropResolver();
        $props = [
            'always' => 'here',
            'optional' => PropResolver::optional(fn () => 'expensive'),
        ];

        $resolved = $resolver->resolve($props);

        $this->assertSame('here', $resolved['always']);
        $this->assertArrayNotHasKey('optional', $resolved);
    }

    public function testOptionalPropsIncludedWhenRequested(): void
    {
        $resolver = new PropResolver();
        $props = [
            'always' => 'here',
            'optional' => PropResolver::optional(fn () => 'expensive'),
        ];

        $resolved = $resolver->resolve($props, only: ['always', 'optional']);

        $this->assertSame('here', $resolved['always']);
        $this->assertSame('expensive', $resolved['optional']);
    }

    public function testPartialReloadOnlyIncludesRequestedProps(): void
    {
        $resolver = new PropResolver();
        $props = [
            'users' => [1, 2, 3],
            'posts' => [4, 5, 6],
            'comments' => [7, 8, 9],
        ];

        $resolved = $resolver->resolve($props, only: ['users', 'posts']);

        $this->assertSame([1, 2, 3], $resolved['users']);
        $this->assertSame([4, 5, 6], $resolved['posts']);
        $this->assertArrayNotHasKey('comments', $resolved);
    }

    public function testPartialReloadExceptExcludesProps(): void
    {
        $resolver = new PropResolver();
        $props = [
            'users' => [1, 2, 3],
            'posts' => [4, 5, 6],
            'comments' => [7, 8, 9],
        ];

        $resolved = $resolver->resolve($props, except: ['comments']);

        $this->assertSame([1, 2, 3], $resolved['users']);
        $this->assertSame([4, 5, 6], $resolved['posts']);
        $this->assertArrayNotHasKey('comments', $resolved);
    }

    public function testExceptTakesPrecedenceOverOnly(): void
    {
        $resolver = new PropResolver();
        $props = [
            'users' => [1, 2, 3],
            'posts' => [4, 5, 6],
        ];

        $resolved = $resolver->resolve($props, only: ['users', 'posts'], except: ['posts']);

        $this->assertSame([1, 2, 3], $resolved['users']);
        $this->assertArrayNotHasKey('posts', $resolved);
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/inertia/tests/Unit/PropResolverTest.php`
Expected: FAIL — class not found.

- [ ] **Step 3: Write the implementation**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Inertia;

final class PropResolver
{
    /**
     * Mark a prop as optional — only resolved when explicitly requested via partial reload.
     */
    public static function optional(\Closure $callback): OptionalProp
    {
        return new OptionalProp($callback);
    }

    /**
     * Resolve props, evaluating closures and filtering for partial reloads.
     *
     * @param array<string, mixed> $props
     * @param list<string> $only   Include only these props (partial reload).
     * @param list<string> $except Exclude these props (partial reload). Takes precedence over $only.
     * @return array<string, mixed>
     */
    public function resolve(array $props, array $only = [], array $except = []): array
    {
        $resolved = [];

        foreach ($props as $key => $value) {
            if ($except !== [] && in_array($key, $except, true)) {
                continue;
            }

            if ($only !== [] && !in_array($key, $only, true)) {
                continue;
            }

            if ($value instanceof OptionalProp) {
                if ($only !== [] && in_array($key, $only, true)) {
                    $resolved[$key] = ($value->callback)();
                }
                continue;
            }

            $resolved[$key] = $value instanceof \Closure ? $value() : $value;
        }

        return $resolved;
    }
}
```

- [ ] **Step 4: Create the OptionalProp value object**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Inertia;

final readonly class OptionalProp
{
    public function __construct(
        public \Closure $callback,
    ) {
    }
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/inertia/tests/Unit/PropResolverTest.php`
Expected: All 6 tests PASS.

- [ ] **Step 6: Commit**

```bash
git add packages/inertia/src/PropResolver.php packages/inertia/src/OptionalProp.php packages/inertia/tests/Unit/PropResolverTest.php
git commit -m "feat(inertia): add PropResolver with optional props and partial reload support"
```

---

### Task 5: InertiaMiddleware

**Files:**
- Create: `packages/inertia/src/InertiaMiddleware.php`
- Create: `packages/inertia/tests/Unit/InertiaMiddlewareTest.php`

This is the core of the package — the middleware that implements the Inertia protocol.

- [ ] **Step 1: Write the failing test**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Inertia\Tests\Unit;

use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;
use Symfony\Component\HttpFoundation\Request;
use Symfony\Component\HttpFoundation\Response;
use Waaseyaa\Foundation\Middleware\HttpHandlerInterface;
use Waaseyaa\Inertia\InertiaMiddleware;
use Waaseyaa\Inertia\InertiaResponse;
use Waaseyaa\Inertia\RootTemplateRenderer;

#[CoversClass(InertiaMiddleware::class)]
final class InertiaMiddlewareTest extends TestCase
{
    private RootTemplateRenderer $renderer;

    protected function setUp(): void
    {
        $this->renderer = new RootTemplateRenderer();
    }

    public function testNonInertiaRequestPassesThrough(): void
    {
        $middleware = new InertiaMiddleware($this->renderer, 'v1');
        $request = Request::create('/users', 'GET');

        $handler = $this->createMockHandler(new Response('plain html', 200));
        $response = $middleware->process($request, $handler);

        $this->assertSame(200, $response->getStatusCode());
        $this->assertSame('plain html', $response->getContent());
    }

    public function testInitialInertiaVisitReturnsHtml(): void
    {
        $middleware = new InertiaMiddleware($this->renderer, 'v1');
        $request = Request::create('/users', 'GET');

        $inertiaResponse = new InertiaResponse(
            component: 'Users/Index',
            props: ['users' => []],
            url: '/users',
            version: 'v1',
        );
        $request->attributes->set('_inertia_response', $inertiaResponse);

        $handler = $this->createMockHandler(new Response());
        $response = $middleware->process($request, $handler);

        $this->assertSame(200, $response->getStatusCode());
        $this->assertStringContainsString('<!DOCTYPE html>', $response->getContent());
        $this->assertStringContainsString('"component":"Users\/Index"', $response->getContent());
    }

    public function testInertiaXhrReturnsJson(): void
    {
        $middleware = new InertiaMiddleware($this->renderer, 'v1');
        $request = Request::create('/users', 'GET');
        $request->headers->set('X-Inertia', 'true');

        $inertiaResponse = new InertiaResponse(
            component: 'Users/Index',
            props: ['users' => []],
            url: '/users',
            version: 'v1',
        );
        $request->attributes->set('_inertia_response', $inertiaResponse);

        $handler = $this->createMockHandler(new Response());
        $response = $middleware->process($request, $handler);

        $this->assertSame(200, $response->getStatusCode());
        $this->assertSame('application/json', $response->headers->get('Content-Type'));
        $this->assertSame('true', $response->headers->get('X-Inertia'));
        $this->assertSame('X-Inertia', $response->headers->get('Vary'));

        $page = json_decode($response->getContent(), true);
        $this->assertSame('Users/Index', $page['component']);
    }

    public function testVersionMismatchReturns409(): void
    {
        $middleware = new InertiaMiddleware($this->renderer, 'v2');
        $request = Request::create('/users', 'GET');
        $request->headers->set('X-Inertia', 'true');
        $request->headers->set('X-Inertia-Version', 'v1');

        $handler = $this->createMockHandler(new Response());
        $response = $middleware->process($request, $handler);

        $this->assertSame(409, $response->getStatusCode());
        $this->assertSame('/users', $response->headers->get('X-Inertia-Location'));
    }

    public function testPutRedirectUses303(): void
    {
        $middleware = new InertiaMiddleware($this->renderer, 'v1');
        $request = Request::create('/users/1', 'PUT');
        $request->headers->set('X-Inertia', 'true');

        $redirectResponse = new Response('', 302, ['Location' => '/users']);
        $handler = $this->createMockHandler($redirectResponse);
        $response = $middleware->process($request, $handler);

        $this->assertSame(303, $response->getStatusCode());
    }

    public function testGetRedirectKeeps302(): void
    {
        $middleware = new InertiaMiddleware($this->renderer, 'v1');
        $request = Request::create('/old', 'GET');
        $request->headers->set('X-Inertia', 'true');

        $redirectResponse = new Response('', 302, ['Location' => '/new']);
        $handler = $this->createMockHandler($redirectResponse);
        $response = $middleware->process($request, $handler);

        $this->assertSame(302, $response->getStatusCode());
    }

    public function testPartialReloadSetsRequestAttributes(): void
    {
        $middleware = new InertiaMiddleware($this->renderer, 'v1');
        $request = Request::create('/users', 'GET');
        $request->headers->set('X-Inertia', 'true');
        $request->headers->set('X-Inertia-Partial-Component', 'Users/Index');
        $request->headers->set('X-Inertia-Partial-Data', 'users,posts');

        $inertiaResponse = new InertiaResponse(
            component: 'Users/Index',
            props: ['users' => [], 'posts' => []],
            url: '/users',
            version: 'v1',
        );
        $request->attributes->set('_inertia_response', $inertiaResponse);

        $handler = $this->createMockHandler(new Response());
        $response = $middleware->process($request, $handler);

        $this->assertSame(200, $response->getStatusCode());
    }

    private function createMockHandler(Response $response): HttpHandlerInterface
    {
        return new class ($response) implements HttpHandlerInterface {
            public function __construct(private readonly Response $response)
            {
            }

            public function handle(Request $request): Response
            {
                return $this->response;
            }
        };
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/inertia/tests/Unit/InertiaMiddlewareTest.php`
Expected: FAIL — class not found.

- [ ] **Step 3: Write the implementation**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Inertia;

use Symfony\Component\HttpFoundation\JsonResponse;
use Symfony\Component\HttpFoundation\Request;
use Symfony\Component\HttpFoundation\Response;
use Waaseyaa\Foundation\Attribute\AsMiddleware;
use Waaseyaa\Foundation\Middleware\HttpHandlerInterface;
use Waaseyaa\Foundation\Middleware\HttpMiddlewareInterface;

#[AsMiddleware(pipeline: 'http', priority: 20)]
final class InertiaMiddleware implements HttpMiddlewareInterface
{
    private const array REDIRECT_METHODS = ['PUT', 'PATCH', 'DELETE'];

    public function __construct(
        private readonly RootTemplateRenderer $renderer,
        private readonly string $version,
    ) {
    }

    public function process(Request $request, HttpHandlerInterface $next): Response
    {
        $isInertia = $request->headers->get('X-Inertia') === 'true';

        if ($isInertia && $this->hasVersionMismatch($request)) {
            return new Response('', 409, [
                'X-Inertia-Location' => $request->getRequestUri(),
            ]);
        }

        $response = $next->handle($request);

        if ($isInertia && $this->isRedirect($response) && $this->shouldUse303($request)) {
            $response->setStatusCode(303);
        }

        $inertiaResponse = $request->attributes->get('_inertia_response');
        if (!$inertiaResponse instanceof InertiaResponse) {
            return $response;
        }

        $pageObject = $inertiaResponse->toPageObject();

        if ($isInertia) {
            return new JsonResponse($pageObject, 200, [
                'X-Inertia' => 'true',
                'Vary' => 'X-Inertia',
            ]);
        }

        $html = $this->renderer->render($pageObject);

        return new Response($html, 200, [
            'Content-Type' => 'text/html; charset=UTF-8',
        ]);
    }

    private function hasVersionMismatch(Request $request): bool
    {
        $clientVersion = $request->headers->get('X-Inertia-Version');

        return $clientVersion !== null && $clientVersion !== $this->version;
    }

    private function isRedirect(Response $response): bool
    {
        return in_array($response->getStatusCode(), [301, 302], true);
    }

    private function shouldUse303(Request $request): bool
    {
        return in_array($request->getMethod(), self::REDIRECT_METHODS, true);
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/inertia/tests/Unit/InertiaMiddlewareTest.php`
Expected: All 7 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/inertia/src/InertiaMiddleware.php packages/inertia/tests/Unit/InertiaMiddlewareTest.php
git commit -m "feat(inertia): add InertiaMiddleware implementing v3 protocol core"
```

---

### Task 6: Inertia Facade

**Files:**
- Create: `packages/inertia/src/Inertia.php`
- Create: `packages/inertia/tests/Unit/InertiaTest.php`

The static facade that controllers use: `Inertia::render('Component', $props)`.

- [ ] **Step 1: Write the failing test**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Inertia\Tests\Unit;

use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;
use Waaseyaa\Inertia\Inertia;
use Waaseyaa\Inertia\InertiaResponse;

#[CoversClass(Inertia::class)]
final class InertiaTest extends TestCase
{
    protected function setUp(): void
    {
        Inertia::reset();
    }

    public function testRenderCreatesInertiaResponse(): void
    {
        Inertia::setVersion('v1');
        $response = Inertia::render('Users/Index', ['users' => [1, 2, 3]]);

        $this->assertInstanceOf(InertiaResponse::class, $response);
        $page = $response->toPageObject();
        $this->assertSame('Users/Index', $page['component']);
        $this->assertSame([1, 2, 3], $page['props']['users']);
    }

    public function testSharedPropsAreMerged(): void
    {
        Inertia::setVersion('v1');
        Inertia::share('auth', ['user' => ['name' => 'Alice']]);
        Inertia::share('flash', ['success' => 'Saved!']);

        $response = Inertia::render('Dashboard', ['stats' => 42]);

        $page = $response->toPageObject();
        $this->assertSame(['name' => 'Alice'], $page['props']['auth']['user']);
        $this->assertSame(['success' => 'Saved!'], $page['props']['flash']);
        $this->assertSame(42, $page['props']['stats']);
    }

    public function testPagePropsOverrideSharedProps(): void
    {
        Inertia::setVersion('v1');
        Inertia::share('title', 'Default');

        $response = Inertia::render('Home', ['title' => 'Custom']);

        $page = $response->toPageObject();
        $this->assertSame('Custom', $page['props']['title']);
    }

    public function testSharedClosuresAreResolvedAtRenderTime(): void
    {
        Inertia::setVersion('v1');
        $counter = 0;
        Inertia::share('count', function () use (&$counter) {
            return ++$counter;
        });

        $response1 = Inertia::render('Page1', []);
        $response2 = Inertia::render('Page2', []);

        $this->assertSame(1, $response1->toPageObject()['props']['count']);
        $this->assertSame(2, $response2->toPageObject()['props']['count']);
    }

    public function testVersionIsIncludedInResponse(): void
    {
        Inertia::setVersion('abc123');
        $response = Inertia::render('Home', []);

        $this->assertSame('abc123', $response->toPageObject()['version']);
    }

    public function testRenderWithOptions(): void
    {
        Inertia::setVersion('v1');
        $response = Inertia::render('Settings', ['user' => 'Alice'], encryptHistory: true);

        $page = $response->toPageObject();
        $this->assertTrue($page['encryptHistory']);
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/inertia/tests/Unit/InertiaTest.php`
Expected: FAIL — class not found.

- [ ] **Step 3: Write the implementation**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Inertia;

final class Inertia
{
    /** @var array<string, mixed> */
    private static array $shared = [];
    private static string $version = '';

    public static function setVersion(string $version): void
    {
        self::$version = $version;
    }

    /**
     * Share a prop with all Inertia responses.
     */
    public static function share(string $key, mixed $value): void
    {
        self::$shared[$key] = $value;
    }

    /**
     * Render an Inertia response.
     *
     * @param array<string, mixed> $props
     */
    public static function render(
        string $component,
        array $props,
        bool $encryptHistory = false,
        bool $clearHistory = false,
    ): InertiaResponse {
        $mergedProps = self::resolveSharedProps();

        foreach ($props as $key => $value) {
            $mergedProps[$key] = $value;
        }

        return new InertiaResponse(
            component: $component,
            props: $mergedProps,
            url: '', // Set by middleware from the request
            version: self::$version,
            encryptHistory: $encryptHistory,
            clearHistory: $clearHistory,
        );
    }

    /**
     * @return array<string, mixed>
     */
    private static function resolveSharedProps(): array
    {
        $resolved = [];
        foreach (self::$shared as $key => $value) {
            $resolved[$key] = $value instanceof \Closure ? $value() : $value;
        }

        return $resolved;
    }

    /**
     * Reset state — for testing only.
     */
    public static function reset(): void
    {
        self::$shared = [];
        self::$version = '';
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/inertia/tests/Unit/InertiaTest.php`
Expected: All 6 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/inertia/src/Inertia.php packages/inertia/tests/Unit/InertiaTest.php
git commit -m "feat(inertia): add Inertia facade with render(), share(), and version support"
```

---

### Task 7: Wire Service Provider & Update Middleware URL

**Files:**
- Modify: `packages/inertia/src/InertiaServiceProvider.php`
- Modify: `packages/inertia/src/InertiaMiddleware.php`

The middleware needs to set the URL on the InertiaResponse from the request, and the service provider needs to register the middleware.

- [ ] **Step 1: Update InertiaMiddleware to set URL from request**

In `packages/inertia/src/InertiaMiddleware.php`, update the section that builds the page object:

Replace:
```php
        $pageObject = $inertiaResponse->toPageObject();
```

With:
```php
        $pageObject = $inertiaResponse->toPageObject();
        $pageObject['url'] = $request->getRequestUri();
```

- [ ] **Step 2: Update InertiaServiceProvider to register middleware**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Inertia;

use Waaseyaa\Entity\EntityTypeManager;
use Waaseyaa\Foundation\Middleware\HttpMiddlewareInterface;
use Waaseyaa\Foundation\ServiceProvider\ServiceProvider;

final class InertiaServiceProvider extends ServiceProvider
{
    public function register(): void
    {
    }

    /**
     * @return list<HttpMiddlewareInterface>
     */
    public function middleware(EntityTypeManager $entityTypeManager): array
    {
        $version = Inertia::getVersion();
        $renderer = new RootTemplateRenderer();

        return [
            new InertiaMiddleware($renderer, $version),
        ];
    }
}
```

- [ ] **Step 3: Add getVersion() to Inertia facade**

Add to `packages/inertia/src/Inertia.php`:

```php
    public static function getVersion(): string
    {
        return self::$version;
    }
```

- [ ] **Step 4: Run all tests**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/inertia/tests/`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/inertia/src/
git commit -m "feat(inertia): wire service provider with middleware registration"
```

---

### Task 8: Full Package Test Suite & Edge Cases

**Files:**
- Modify: `packages/inertia/tests/Unit/InertiaMiddlewareTest.php`

- [ ] **Step 1: Add edge case tests**

Add to `InertiaMiddlewareTest.php`:

```php
    public function testDeleteRedirectUses303(): void
    {
        $middleware = new InertiaMiddleware($this->renderer, 'v1');
        $request = Request::create('/users/1', 'DELETE');
        $request->headers->set('X-Inertia', 'true');

        $redirectResponse = new Response('', 302, ['Location' => '/users']);
        $handler = $this->createMockHandler($redirectResponse);
        $response = $middleware->process($request, $handler);

        $this->assertSame(303, $response->getStatusCode());
    }

    public function testPatchRedirectUses303(): void
    {
        $middleware = new InertiaMiddleware($this->renderer, 'v1');
        $request = Request::create('/users/1', 'PATCH');
        $request->headers->set('X-Inertia', 'true');

        $redirectResponse = new Response('', 302, ['Location' => '/users']);
        $handler = $this->createMockHandler($redirectResponse);
        $response = $middleware->process($request, $handler);

        $this->assertSame(303, $response->getStatusCode());
    }

    public function testNonInertiaRedirectKeepsStatusCode(): void
    {
        $middleware = new InertiaMiddleware($this->renderer, 'v1');
        $request = Request::create('/users/1', 'PUT');
        // No X-Inertia header

        $redirectResponse = new Response('', 302, ['Location' => '/users']);
        $handler = $this->createMockHandler($redirectResponse);
        $response = $middleware->process($request, $handler);

        $this->assertSame(302, $response->getStatusCode());
    }

    public function testVersionMatchDoesNotReturn409(): void
    {
        $middleware = new InertiaMiddleware($this->renderer, 'v1');
        $request = Request::create('/users', 'GET');
        $request->headers->set('X-Inertia', 'true');
        $request->headers->set('X-Inertia-Version', 'v1');

        $inertiaResponse = new InertiaResponse(
            component: 'Users/Index',
            props: [],
            url: '/users',
            version: 'v1',
        );
        $request->attributes->set('_inertia_response', $inertiaResponse);

        $handler = $this->createMockHandler(new Response());
        $response = $middleware->process($request, $handler);

        $this->assertSame(200, $response->getStatusCode());
    }

    public function testUrlIsSetFromRequest(): void
    {
        $middleware = new InertiaMiddleware($this->renderer, 'v1');
        $request = Request::create('/users?page=2', 'GET');
        $request->headers->set('X-Inertia', 'true');

        $inertiaResponse = new InertiaResponse(
            component: 'Users/Index',
            props: [],
            url: '',
            version: 'v1',
        );
        $request->attributes->set('_inertia_response', $inertiaResponse);

        $handler = $this->createMockHandler(new Response());
        $response = $middleware->process($request, $handler);

        $page = json_decode($response->getContent(), true);
        $this->assertSame('/users?page=2', $page['url']);
    }
```

- [ ] **Step 2: Run full test suite**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/inertia/tests/`
Expected: All tests PASS (should be ~28 tests total).

- [ ] **Step 3: Commit**

```bash
git add packages/inertia/tests/
git commit -m "test(inertia): add edge case tests for redirect handling and URL resolution"
```

---

### Task 9: ControllerDispatcher Integration

**Files:**
- Modify: `packages/foundation/src/Http/ControllerDispatcher.php`
- Create: `packages/inertia/tests/Unit/InertiaDispatcherTest.php`

**Architecture note:** Waaseyaa's middleware pipeline runs as a pre-flight check (auth, session, CSRF) and **completes before** the `ControllerDispatcher` runs. The dispatcher uses `ResponseSender` which calls `exit` (returns `never`). Therefore:

- **InertiaMiddleware** handles only version checking (409 response) — this correctly runs pre-controller.
- **InertiaResponse rendering** must happen in the `ControllerDispatcher`, alongside existing `SsrResponse` and array handling in the callable controller branch (lines 98-107).

When a callable controller returns an `InertiaResponse`, the dispatcher checks the request for the `X-Inertia` header:
- If present → `ResponseSender::json()` with the page object + Inertia headers
- If absent (initial page load) → `ResponseSender::html()` with rendered root template

- [ ] **Step 1: Write the failing test**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Inertia\Tests\Unit;

use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;
use Waaseyaa\Inertia\Inertia;
use Waaseyaa\Inertia\InertiaResponse;
use Waaseyaa\Inertia\RootTemplateRenderer;

#[CoversClass(InertiaResponse::class)]
final class InertiaDispatcherTest extends TestCase
{
    protected function setUp(): void
    {
        Inertia::reset();
        Inertia::setVersion('v1');
    }

    public function testInertiaResponsePageObjectForXhr(): void
    {
        $response = Inertia::render('Users/Index', ['users' => [1, 2, 3]]);
        $pageObject = $response->toPageObject();
        $pageObject['url'] = '/users';

        $this->assertSame('Users/Index', $pageObject['component']);
        $this->assertSame([1, 2, 3], $pageObject['props']['users']);
        $this->assertSame([], $pageObject['props']['errors']);
        $this->assertSame('/users', $pageObject['url']);
        $this->assertSame('v1', $pageObject['version']);
    }

    public function testInertiaResponseHtmlForInitialLoad(): void
    {
        $response = Inertia::render('Dashboard', ['stats' => 42]);
        $pageObject = $response->toPageObject();
        $pageObject['url'] = '/dashboard';

        $renderer = new RootTemplateRenderer();
        $html = $renderer->render($pageObject);

        $this->assertStringContainsString('<!DOCTYPE html>', $html);
        $this->assertStringContainsString('"component":"Dashboard"', $html);
        $this->assertStringContainsString('"stats":42', $html);
    }

    public function testControllerPatternWithSharedProps(): void
    {
        Inertia::share('auth', fn () => ['user' => ['name' => 'Alice']]);

        $response = Inertia::render('Dashboard', ['stats' => 42]);
        $page = $response->toPageObject();

        $this->assertSame('Dashboard', $page['component']);
        $this->assertSame(42, $page['props']['stats']);
        $this->assertSame(['name' => 'Alice'], $page['props']['auth']['user']);
    }

    public function testInertiaResponseIsDetectable(): void
    {
        $response = Inertia::render('Home', []);
        $this->assertInstanceOf(InertiaResponse::class, $response);
    }
}
```

- [ ] **Step 2: Run test to verify it passes**

These tests validate the InertiaResponse rendering patterns. They should pass immediately since they test existing classes.

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/inertia/tests/Unit/InertiaDispatcherTest.php`
Expected: All 4 tests PASS.

- [ ] **Step 3: Add InertiaResponse handling to ControllerDispatcher**

In `packages/foundation/src/Http/ControllerDispatcher.php`, modify the callable controller branch (lines 98-107).

Add after the `SsrResponse` check (line 101-102) and before the `is_array` check (line 103):

```php
if ($result instanceof \Waaseyaa\Inertia\InertiaResponse) {
    $pageObject = $result->toPageObject();
    $pageObject['url'] = $httpRequest->getRequestUri();

    if ($httpRequest->headers->get('X-Inertia') === 'true') {
        // XHR Inertia request → JSON page object
        ResponseSender::json(200, $pageObject, [
            'X-Inertia' => 'true',
            'Vary' => 'X-Inertia',
        ]);
    }

    // Initial page load → HTML with embedded page object
    $renderer = new \Waaseyaa\Inertia\RootTemplateRenderer();
    ResponseSender::html(200, $renderer->render($pageObject));
}
```

> **Note:** This is a soft dependency — if `waaseyaa/inertia` is not installed, `InertiaResponse` class won't exist and the `instanceof` check returns `false`. No error. Same pattern as existing `SsrResponse` handling.

- [ ] **Step 4: Also handle redirect 302→303 for PUT/PATCH/DELETE Inertia requests**

In the callable controller branch, after the `InertiaResponse` check, add redirect handling. When a callable controller returns a Symfony `RedirectResponse` and the request has `X-Inertia: true`:

```php
if ($result instanceof \Symfony\Component\HttpFoundation\RedirectResponse
    && $httpRequest->headers->get('X-Inertia') === 'true'
    && in_array($httpRequest->getMethod(), ['PUT', 'PATCH', 'DELETE'], true)
) {
    $result->setStatusCode(303);
    $result->send();
    exit;
}
if ($result instanceof \Symfony\Component\HttpFoundation\Response) {
    $result->send();
    exit;
}
```

- [ ] **Step 5: Run all inertia tests**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/inertia/tests/`
Expected: All tests PASS.

- [ ] **Step 6: Commit**

```bash
git add packages/foundation/src/Http/ControllerDispatcher.php packages/inertia/tests/Unit/InertiaDispatcherTest.php
git commit -m "feat(inertia): add InertiaResponse rendering to ControllerDispatcher"
```

- [ ] **Step 7: Simplify InertiaMiddleware scope**

Now that rendering is handled by the dispatcher, update `InertiaMiddleware` to only handle version checking. Remove the response rendering code and keep only the version mismatch → 409 logic.

In `packages/inertia/src/InertiaMiddleware.php`, simplify to:

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Inertia;

use Symfony\Component\HttpFoundation\Request;
use Symfony\Component\HttpFoundation\Response;
use Waaseyaa\Foundation\Attribute\AsMiddleware;
use Waaseyaa\Foundation\Middleware\HttpHandlerInterface;
use Waaseyaa\Foundation\Middleware\HttpMiddlewareInterface;

#[AsMiddleware(pipeline: 'http', priority: 20)]
final class InertiaMiddleware implements HttpMiddlewareInterface
{
    public function __construct(
        private readonly string $version,
    ) {
    }

    public function process(Request $request, HttpHandlerInterface $next): Response
    {
        if ($request->headers->get('X-Inertia') !== 'true') {
            return $next->handle($request);
        }

        $clientVersion = $request->headers->get('X-Inertia-Version');
        if ($clientVersion !== null && $clientVersion !== $this->version) {
            return new Response('', 409, [
                'X-Inertia-Location' => $request->getRequestUri(),
            ]);
        }

        return $next->handle($request);
    }
}
```

- [ ] **Step 8: Update InertiaMiddleware tests for reduced scope**

Remove rendering-related tests from `InertiaMiddlewareTest.php`. Keep only:
- `testNonInertiaRequestPassesThrough`
- `testVersionMismatchReturns409`
- `testVersionMatchPassesThrough`

- [ ] **Step 9: Update InertiaServiceProvider (no longer needs RootTemplateRenderer)**

```php
final class InertiaServiceProvider extends ServiceProvider
{
    public function register(): void
    {
    }

    public function middleware(EntityTypeManager $entityTypeManager): array
    {
        return [
            new InertiaMiddleware(Inertia::getVersion()),
        ];
    }
}
```

- [ ] **Step 10: Run all tests**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/inertia/tests/`
Expected: All tests PASS.

- [ ] **Step 11: Commit**

```bash
git add packages/inertia/src/ packages/inertia/tests/
git commit -m "refactor(inertia): simplify middleware to version checking only"
```

---

## Summary

| Task | What it builds | Tests |
|---|---|---|
| 1 | Package scaffold (composer.json, service provider) | — |
| 2 | InertiaResponse value object (v3 page object) | 7 |
| 3 | RootTemplateRenderer (initial HTML page) | 3 |
| 4 | PropResolver (optional/partial/closure props) | 6 |
| 5 | InertiaMiddleware (protocol core) | 7 |
| 6 | Inertia facade (render/share/version) | 6 |
| 7 | Service provider wiring + URL from request | — |
| 8 | Edge case tests | 5 |
| 9 | ControllerDispatcher integration + middleware simplification | 4 |

**Total: 9 tasks, ~32 tests, 7 source files + 1 modified foundation file**

After this plan is complete, `waaseyaa/inertia` will support:
- Full Inertia v3 protocol (HTML initial load, JSON XHR responses)
- Asset version checking (409 conflict via middleware)
- Redirect status code conversion (302 → 303 for PUT/PATCH/DELETE via dispatcher)
- Shared props with closure resolution
- Optional props (excluded unless explicitly requested)
- Partial reloads (only/except filtering)
- Deferred, merge, prepend, deep merge, and once props in page object
- Customizable root HTML template
- Seamless controller integration via ControllerDispatcher (soft dependency)

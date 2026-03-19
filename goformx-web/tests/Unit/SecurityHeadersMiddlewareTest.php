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

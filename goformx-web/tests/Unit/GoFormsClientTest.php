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

<?php

declare(strict_types=1);

namespace GoFormX\Service;

final class GoFormsClient implements GoFormsClientInterface
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

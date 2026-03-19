<?php

declare(strict_types=1);

namespace GoFormX\Service;

interface GoFormsClientInterface
{
    /**
     * @return array<string, mixed>
     * @throws \RuntimeException on HTTP error
     */
    public function get(string $path, string $userId, string $planTier): array;

    /**
     * @param array<string, mixed> $body
     * @return array<string, mixed>
     * @throws \RuntimeException on HTTP error
     */
    public function post(string $path, string $userId, string $planTier, array $body = []): array;

    /**
     * @param array<string, mixed> $body
     * @return array<string, mixed>
     * @throws \RuntimeException on HTTP error
     */
    public function put(string $path, string $userId, string $planTier, array $body = []): array;

    /**
     * @return array<string, mixed>
     * @throws \RuntimeException on HTTP error
     */
    public function delete(string $path, string $userId, string $planTier): array;
}

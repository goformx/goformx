<?php

namespace App\Services;

use App\Models\User;
use Illuminate\Http\Client\PendingRequest;
use Illuminate\Http\Client\RequestException;
use Illuminate\Http\Client\Response;
use Illuminate\Support\Facades\Http;

class GoFormsClient
{
    public function __construct(
        private readonly string $baseUrl,
        private readonly ?string $secret,
        private ?User $user = null,
    ) {}

    public function withUser(User $user): self
    {
        $instance = clone $this;
        $instance->user = $user;

        return $instance;
    }

    /**
     * @return array<int, mixed>
     */
    public function listForms(): array
    {
        $response = $this->get('/api/forms');

        return $response->json('data.forms', []);
    }

    /**
     * @return array<string, mixed>|null
     */
    public function getForm(string $id): ?array
    {
        try {
            $response = $this->get("/api/forms/{$id}");
        } catch (RequestException $e) {
            if ($e->response && $e->response->status() === 404) {
                return null;
            }

            throw $e;
        }

        return $response->json('data.form');
    }

    /**
     * @param  array<string, mixed>  $data
     * @return array<string, mixed>
     */
    public function createForm(array $data): array
    {
        $response = $this->post('/api/forms', $data);

        return $response->json('data.form', []);
    }

    /**
     * @param  array<string, mixed>  $data
     * @return array<string, mixed>
     */
    public function updateForm(string $id, array $data): array
    {
        $response = $this->put("/api/forms/{$id}", $data);

        return $response->json('data.form', []);
    }

    public function deleteForm(string $id): bool
    {
        $response = $this->delete("/api/forms/{$id}");

        return $response->successful();
    }

    /**
     * @return array<int, mixed>
     */
    public function listSubmissions(string $formId): array
    {
        $response = $this->get("/api/forms/{$formId}/submissions");

        return $response->json('data.submissions', []);
    }

    /**
     * @return array<string, mixed>|null
     */
    public function getSubmission(string $formId, string $submissionId): ?array
    {
        $response = $this->get("/api/forms/{$formId}/submissions/{$submissionId}");

        if ($response->notFound()) {
            return null;
        }

        return $response->json('data', $response->json());
    }

    public function getFormsCount(): int
    {
        $response = $this->get('/api/forms/usage/forms-count');

        return $response->json('data.count', 0);
    }

    public function getSubmissionsCount(string $month): int
    {
        $response = $this->request()->get('/api/forms/usage/submissions-count', ['month' => $month])->throw();

        return $response->json('data.count', 0);
    }

    private function get(string $url): Response
    {
        return $this->request()->get($url)->throw();
    }

    /**
     * @param  array<string, mixed>  $data
     */
    private function post(string $url, array $data = []): Response
    {
        return $this->request()->post($url, $data)->throw();
    }

    /**
     * @param  array<string, mixed>  $data
     */
    private function put(string $url, array $data = []): Response
    {
        return $this->request()->put($url, $data)->throw();
    }

    private function delete(string $url): Response
    {
        return $this->request()->delete($url)->throw();
    }

    private function request(): PendingRequest
    {
        if ($this->user === null) {
            throw new \RuntimeException('GoFormsClient requires an authenticated user. Call withUser() first.');
        }

        if ($this->secret === null || $this->secret === '') {
            throw new \RuntimeException('GoFormsClient requires GOFORMS_SHARED_SECRET to be set in .env (must match the Go service).');
        }

        return Http::baseUrl(rtrim($this->baseUrl, '/'))
            ->withHeaders($this->signRequest($this->user->getKey(), $this->secret));
    }

    /**
     * @return array<string, string>
     */
    private function signRequest(string $userId, string $secret): array
    {
        $timestamp = now()->utc()->format('Y-m-d\TH:i:s\Z');
        $planTier = $this->user->planTier();

        $payload = $userId.':'.$timestamp.':'.$planTier;
        $signature = hash_hmac('sha256', $payload, $secret, false);

        return [
            'X-User-Id' => $userId,
            'X-Timestamp' => $timestamp,
            'X-Signature' => $signature,
            'X-Plan-Tier' => $planTier,
        ];
    }

    public static function fromConfig(): self
    {
        return new self(
            config('services.goforms.url', 'http://localhost:8090'),
            config('services.goforms.secret'),
        );
    }
}

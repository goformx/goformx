<?php

declare(strict_types=1);

namespace GoFormX\Controller;

use GoFormX\Service\GoFormsClientInterface;
use Waaseyaa\Inertia\Inertia;
use Waaseyaa\Inertia\InertiaResponse;

final class FormController
{
    public function __construct(
        private readonly GoFormsClientInterface $client,
    ) {
    }

    public function index(string $userId, string $planTier): InertiaResponse
    {
        try {
            $response = $this->client->get('/api/forms', $userId, $planTier);
            $forms = $response['data']['forms'] ?? [];
        } catch (\RuntimeException $e) {
            $forms = [];
        }

        return Inertia::render('Forms/Index', ['forms' => $forms]);
    }

    public function show(string $formId, string $userId, string $planTier): InertiaResponse
    {
        $response = $this->client->get("/api/forms/{$formId}", $userId, $planTier);

        return Inertia::render('Forms/Show', ['form' => $response['data']['form'] ?? []]);
    }

    public function edit(string $formId, string $userId, string $planTier): InertiaResponse
    {
        $response = $this->client->get("/api/forms/{$formId}", $userId, $planTier);

        return Inertia::render('Forms/Edit', ['form' => $response['data']['form'] ?? []]);
    }

    /**
     * @param array<string, mixed> $data
     * @return array<string, mixed>
     */
    public function store(string $userId, string $planTier, array $data): array
    {
        return $this->client->post('/api/forms', $userId, $planTier, $data);
    }

    /**
     * @param array<string, mixed> $data
     * @return array<string, mixed>
     */
    public function update(string $formId, string $userId, string $planTier, array $data): array
    {
        return $this->client->put("/api/forms/{$formId}", $userId, $planTier, $data);
    }

    /**
     * @return array<string, mixed>
     */
    public function destroy(string $formId, string $userId, string $planTier): array
    {
        return $this->client->delete("/api/forms/{$formId}", $userId, $planTier);
    }

    public function submissions(string $formId, string $userId, string $planTier): InertiaResponse
    {
        $form = $this->client->get("/api/forms/{$formId}", $userId, $planTier);
        $submissions = $this->client->get("/api/forms/{$formId}/submissions", $userId, $planTier);

        return Inertia::render('Forms/Submissions', [
            'form' => $form['data']['form'] ?? [],
            'submissions' => $submissions['data']['submissions'] ?? [],
        ]);
    }

    public function embed(string $formId, string $userId, string $planTier): InertiaResponse
    {
        $response = $this->client->get("/api/forms/{$formId}", $userId, $planTier);

        return Inertia::render('Forms/Embed', ['form' => $response['data']['form'] ?? []]);
    }
}

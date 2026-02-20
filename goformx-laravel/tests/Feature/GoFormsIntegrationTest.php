<?php

use App\Models\User;

test('forms index reaches Go API with assertion auth when service is available', function () {
    $baseUrl = config('services.goforms.url');
    $secret = config('services.goforms.secret');

    if (! getenv('GOFORMS_LIVE_TEST')) {
        $this->markTestSkipped('Set GOFORMS_LIVE_TEST=1 to run against real Go API');
    }

    if (! $baseUrl || ! $secret) {
        $this->markTestSkipped('GOFORMS_API_URL and GOFORMS_SHARED_SECRET must be set');
    }

    $user = User::factory()->create();

    $response = $this
        ->actingAs($user)
        ->get(route('forms.index'));

    if ($response->status() === 500) {
        $content = $response->getContent();
        if (str_contains($content, 'Could not resolve host') || str_contains($content, 'Connection refused')) {
            $this->markTestSkipped('Go API not reachable (run with: ddev exec -- env GOFORMS_LIVE_TEST=1 php artisan test --filter="forms index reaches"');
        }
    }

    $response->assertOk();
    $response->assertInertia(fn ($page) => $page->component('Forms/Index')->has('forms'));
});

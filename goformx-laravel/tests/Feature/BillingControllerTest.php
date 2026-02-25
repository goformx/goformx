<?php

use App\Models\User;
use Illuminate\Support\Facades\Http;
use Inertia\Testing\AssertableInertia as Assert;

beforeEach(function () {
    config([
        'services.goforms.url' => 'http://localhost:8090',
        'services.goforms.secret' => 'test-secret',
    ]);
});

it('redirects unauthenticated users from billing page', function () {
    $this->get(route('billing.index'))
        ->assertRedirect(route('login'));
});

it('renders billing page for authenticated users', function () {
    Http::fake([
        '*/api/forms/usage/forms-count' => Http::response(['success' => true, 'data' => ['count' => 5]]),
        '*/api/forms/usage/submissions-count*' => Http::response(['success' => true, 'data' => ['count' => 42]]),
    ]);

    $user = User::factory()->create();

    $this->actingAs($user)
        ->get(route('billing.index'))
        ->assertOk()
        ->assertInertia(fn (Assert $page) => $page
            ->component('Billing/Index')
            ->has('currentTier')
            ->has('usage')
            ->where('usage.forms', 5)
            ->where('usage.submissions', 42)
        );
});

it('renders billing page with zero usage when API returns defaults', function () {
    Http::fake([
        '*/api/forms/usage/*' => Http::response(['success' => true, 'data' => ['count' => 0]]),
    ]);

    $user = User::factory()->create();

    $this->actingAs($user)
        ->get(route('billing.index'))
        ->assertOk()
        ->assertInertia(fn (Assert $page) => $page
            ->component('Billing/Index')
            ->where('usage.forms', 0)
            ->where('usage.submissions', 0)
        );
});

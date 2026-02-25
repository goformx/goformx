<?php

use App\Models\User;
use Inertia\Testing\AssertableInertia as Assert;

it('renders pricing page for guests', function () {
    $this->get(route('pricing'))
        ->assertOk()
        ->assertInertia(fn (Assert $page) => $page->component('Pricing'));
});

it('renders pricing page for authenticated users with plan tier', function () {
    $user = User::factory()->create();

    $this->actingAs($user)
        ->get(route('pricing'))
        ->assertOk()
        ->assertInertia(fn (Assert $page) => $page
            ->component('Pricing')
            ->has('currentTier')
        );
});

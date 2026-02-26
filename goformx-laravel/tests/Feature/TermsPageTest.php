<?php

use App\Models\User;
use Inertia\Testing\AssertableInertia as Assert;

it('renders terms page for guests', function () {
    $this->get(route('terms'))
        ->assertOk()
        ->assertInertia(fn (Assert $page) => $page->component('Terms'));
});

it('renders terms page for authenticated users', function () {
    $user = User::factory()->create();

    $this->actingAs($user)
        ->get(route('terms'))
        ->assertOk()
        ->assertInertia(fn (Assert $page) => $page->component('Terms'));
});

it('shares seo props on terms page', function () {
    config(['app.url' => 'https://example.com']);

    $this->get(route('terms'))
        ->assertOk()
        ->assertInertia(fn (Assert $page) => $page
            ->has('seo')
            ->where('seo.appUrl', 'https://example.com')
        );
});

<?php

use App\Models\User;
use Inertia\Testing\AssertableInertia as Assert;

it('renders privacy page for guests', function () {
    $this->get(route('privacy'))
        ->assertOk()
        ->assertInertia(fn (Assert $page) => $page->component('Privacy'));
});

it('renders privacy page for authenticated users', function () {
    $user = User::factory()->create();

    $this->actingAs($user)
        ->get(route('privacy'))
        ->assertOk()
        ->assertInertia(fn (Assert $page) => $page->component('Privacy'));
});

it('shares seo props on privacy page', function () {
    config(['app.url' => 'https://example.com']);

    $this->get(route('privacy'))
        ->assertOk()
        ->assertInertia(fn (Assert $page) => $page
            ->has('seo')
            ->where('seo.appUrl', 'https://example.com')
        );
});

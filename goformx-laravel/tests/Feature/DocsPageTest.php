<?php

use function Pest\Laravel\get;

it('renders the getting started docs page', function () {
    get('/docs')
        ->assertOk()
        ->assertInertia(fn ($page) => $page
            ->component('Docs/Show')
            ->has('title')
            ->has('content')
            ->has('navigation')
            ->where('slug', 'getting-started')
        );
});

it('renders a specific docs page by slug', function () {
    get('/docs/form-builder')
        ->assertOk()
        ->assertInertia(fn ($page) => $page
            ->component('Docs/Show')
            ->where('slug', 'form-builder')
        );
});

it('returns 404 for non-existent docs page', function () {
    get('/docs/does-not-exist')
        ->assertNotFound();
});

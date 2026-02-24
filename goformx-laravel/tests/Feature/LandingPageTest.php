<?php

use Inertia\Testing\AssertableInertia as Assert;

test('returns GoFormX home landing page', function () {
    $response = $this->get('/');

    $response->assertOk();
    $response->assertInertia(fn (Assert $page) => $page
        ->component('Home')
        ->has('canRegister')
    );
});

test('home page shares seo props', function () {
    config(['app.url' => 'https://example.com']);

    $response = $this->get('/');

    $response->assertOk();
    $response->assertInertia(fn (Assert $page) => $page
        ->has('seo')
        ->where('seo.appUrl', 'https://example.com')
        ->where('seo.defaultOgImage', null)
    );
});

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

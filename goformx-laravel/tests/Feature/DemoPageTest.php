<?php

use Illuminate\Support\Facades\Config;
use Inertia\Testing\AssertableInertia as Assert;

test('demo returns Demo page with formId when configured', function () {
    Config::set('services.goforms.demo_form_id', 'demo-form-123');

    $response = $this->get(route('demo'));

    $response->assertOk();
    $response->assertInertia(fn (Assert $page) => $page
        ->component('Demo')
        ->where('formId', 'demo-form-123')
    );
});

test('demo returns DemoUnconfigured page when demo form id not set', function () {
    Config::set('services.goforms.demo_form_id', null);

    $response = $this->get(route('demo'));

    $response->assertOk();
    $response->assertInertia(fn (Assert $page) => $page
        ->component('DemoUnconfigured')
    );
});

test('demo returns DemoUnconfigured page when demo form id is empty string', function () {
    Config::set('services.goforms.demo_form_id', '');

    $response = $this->get(route('demo'));

    $response->assertOk();
    $response->assertInertia(fn (Assert $page) => $page
        ->component('DemoUnconfigured')
    );
});

<?php

use App\Models\User;
use Illuminate\Support\Facades\Http;
use Inertia\Testing\AssertableInertia as Assert;

beforeEach(function () {
    config([
        'services.goforms.url' => 'http://goforms:8090',
        'services.goforms.secret' => 'test-secret',
        'services.stripe.prices.pro_monthly' => 'price_pro_monthly',
        'services.stripe.prices.pro_annual' => 'price_pro_annual',
        'services.stripe.prices.business_monthly' => 'price_business_monthly',
        'services.stripe.prices.business_annual' => 'price_business_annual',
    ]);
});

it('free user sees correct tier on pricing page', function () {
    $user = User::factory()->create();

    $this->actingAs($user)
        ->get(route('pricing'))
        ->assertOk()
        ->assertInertia(fn (Assert $page) => $page
            ->component('Pricing')
            ->where('currentTier', 'free')
        );
});

it('free user can initiate checkout for pro plan', function () {
    config(['services.stripe.prices.pro_monthly' => 'price_pro_test']);

    $user = User::factory()->create();

    $this->actingAs($user)
        ->post(route('billing.checkout'), ['price_id' => 'price_pro_test'])
        ->assertRedirect();
})->skip(! env('STRIPE_SECRET'), 'Stripe credentials required');

it('subscribed user has correct plan tier in assertion headers', function () {
    Http::fake([
        '*/api/forms' => Http::response(['data' => ['forms' => []]], 200),
    ]);

    $user = User::factory()->create();
    $user->subscriptions()->create([
        'type' => 'default',
        'stripe_id' => 'sub_test',
        'stripe_status' => 'active',
        'stripe_price' => 'price_pro_monthly',
    ]);

    $this->actingAs($user)
        ->get(route('forms.index'));

    Http::assertSent(function ($request) {
        return $request->hasHeader('X-Plan-Tier', 'pro');
    });
});

it('enterprise override takes priority over subscription', function () {
    Http::fake([
        '*/api/forms' => Http::response(['data' => ['forms' => []]], 200),
    ]);

    $user = User::factory()->create(['plan_override' => 'enterprise']);
    $user->subscriptions()->create([
        'type' => 'default',
        'stripe_id' => 'sub_test',
        'stripe_status' => 'active',
        'stripe_price' => 'price_pro_monthly',
    ]);

    $this->actingAs($user)
        ->get(route('forms.index'));

    Http::assertSent(function ($request) {
        return $request->hasHeader('X-Plan-Tier', 'enterprise');
    });
});

<?php

use App\Models\User;

beforeEach(function () {
    config([
        'services.stripe.prices.pro_monthly' => 'price_pro_monthly',
        'services.stripe.prices.pro_annual' => 'price_pro_annual',
        'services.stripe.prices.business_monthly' => 'price_business_monthly',
        'services.stripe.prices.business_annual' => 'price_business_annual',
    ]);
});

it('returns free for user with no subscription', function () {
    $user = User::factory()->create();

    expect($user->planTier())->toBe('free');
});

it('returns pro for user with pro monthly subscription', function () {
    $user = User::factory()->create();
    $user->subscriptions()->create([
        'type' => 'default',
        'stripe_id' => 'sub_test_pro',
        'stripe_status' => 'active',
        'stripe_price' => 'price_pro_monthly',
    ]);

    expect($user->planTier())->toBe('pro');
});

it('returns pro for user with pro annual subscription', function () {
    $user = User::factory()->create();
    $user->subscriptions()->create([
        'type' => 'default',
        'stripe_id' => 'sub_test_pro_annual',
        'stripe_status' => 'active',
        'stripe_price' => 'price_pro_annual',
    ]);

    expect($user->planTier())->toBe('pro');
});

it('returns business for user with business subscription', function () {
    $user = User::factory()->create();
    $user->subscriptions()->create([
        'type' => 'default',
        'stripe_id' => 'sub_test_biz',
        'stripe_status' => 'active',
        'stripe_price' => 'price_business_monthly',
    ]);

    expect($user->planTier())->toBe('business');
});

it('returns enterprise when plan_override is set', function () {
    $user = User::factory()->create(['plan_override' => 'enterprise']);

    expect($user->planTier())->toBe('enterprise');
});

it('returns plan_override over active subscription', function () {
    $user = User::factory()->create(['plan_override' => 'enterprise']);
    $user->subscriptions()->create([
        'type' => 'default',
        'stripe_id' => 'sub_test_pro',
        'stripe_status' => 'active',
        'stripe_price' => 'price_pro_monthly',
    ]);

    expect($user->planTier())->toBe('enterprise');
});

it('returns free for canceled subscription past grace period', function () {
    $user = User::factory()->create();
    $user->subscriptions()->create([
        'type' => 'default',
        'stripe_id' => 'sub_test_canceled',
        'stripe_status' => 'canceled',
        'stripe_price' => 'price_pro_monthly',
        'ends_at' => now()->subDay(),
    ]);

    expect($user->planTier())->toBe('free');
});

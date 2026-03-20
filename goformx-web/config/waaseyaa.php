<?php

declare(strict_types=1);

/** Read an environment variable with a fallback default. */
function env(string $key, string|int $default = ''): string|int
{
    $value = getenv($key);
    return $value !== false ? $value : $default;
}

return [
    'app_name' => 'GoFormX',
    'app_env' => env('APP_ENV', 'local'),
    'app_url' => env('APP_URL', 'http://localhost:8080'),
    'app_secret' => env('APP_SECRET', 'change-me-in-production'),

    // Database (SQLite for entity storage)
    'database' => env('WAASEYAA_DB') ?: dirname(__DIR__) . '/storage/goformx.sqlite',

    // GoForms API
    'goforms_api_url' => env('GOFORMS_API_URL', 'http://localhost:8090'),
    'goforms_shared_secret' => env('GOFORMS_SHARED_SECRET'),
    'goforms_public_url' => env('GOFORMS_PUBLIC_URL', 'https://api.goformx.com'),

    // Auth (waaseyaa/auth)
    'auth_secret' => env('APP_SECRET', 'change-me-in-production'),
    'password_reset_lifetime' => 3600,
    'email_verification_lifetime' => 3600,

    // Mail
    'mail' => [
        'transport' => env('MAIL_TRANSPORT', 'smtp'),
        'from_address' => env('MAIL_FROM_ADDRESS', 'noreply@goformx.com'),
        'from_name' => env('MAIL_FROM_NAME', 'GoFormX'),
        'host' => env('MAIL_HOST', 'mailpit'),
        'port' => (int) env('MAIL_PORT', 1025),
    ],

    // Billing (waaseyaa/billing)
    'stripe_key' => env('STRIPE_KEY'),
    'stripe_secret' => env('STRIPE_SECRET'),
    'stripe_webhook_secret' => env('STRIPE_WEBHOOK_SECRET'),
    'billing_success_url' => env('APP_URL', 'http://localhost:8080') . '/billing?success=true',
    'billing_cancel_url' => env('APP_URL', 'http://localhost:8080') . '/billing?canceled=true',
    'billing_portal_return_url' => env('APP_URL', 'http://localhost:8080') . '/billing',
    'billing_founding_member_cap' => (int) env('FOUNDING_MEMBER_CAP', 100),
    'billing_price_tier_map' => [
        env('STRIPE_GROWTH_MONTHLY_PRICE') => 'growth',
        env('STRIPE_GROWTH_YEARLY_PRICE') => 'growth',
        env('STRIPE_BUSINESS_MONTHLY_PRICE') => 'business',
        env('STRIPE_BUSINESS_YEARLY_PRICE') => 'business',
        env('STRIPE_PRO_MONTHLY_PRICE') => 'pro',
        env('STRIPE_PRO_YEARLY_PRICE') => 'pro',
    ],

    // Inertia
    'inertia_version' => '',

    // CORS
    'cors_allowed_origins' => [],

    // Providers (auto-discovered from package manifests + app providers)
    'providers' => [
        GoFormX\AppServiceProvider::class,
    ],
];

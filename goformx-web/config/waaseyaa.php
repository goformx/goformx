<?php

declare(strict_types=1);

return [
    'app_name' => 'GoFormX',
    'app_env' => $_ENV['APP_ENV'] ?? 'local',
    'app_url' => $_ENV['APP_URL'] ?? 'http://localhost:8080',
    'app_secret' => $_ENV['APP_SECRET'] ?? 'change-me-in-production',

    // Database (SQLite for entity storage)
    'database' => $_ENV['WAASEYAA_DB'] ?? dirname(__DIR__) . '/storage/goformx.sqlite',

    // GoForms API
    'goforms_api_url' => $_ENV['GOFORMS_API_URL'] ?? 'http://localhost:8090',
    'goforms_shared_secret' => $_ENV['GOFORMS_SHARED_SECRET'] ?? '',
    'goforms_public_url' => $_ENV['GOFORMS_PUBLIC_URL'] ?? 'https://api.goformx.com',

    // Auth (waaseyaa/auth)
    'auth_secret' => $_ENV['APP_SECRET'] ?? 'change-me-in-production',
    'password_reset_lifetime' => 3600,
    'email_verification_lifetime' => 3600,

    // Billing (waaseyaa/billing)
    'stripe_key' => $_ENV['STRIPE_KEY'] ?? '',
    'stripe_secret' => $_ENV['STRIPE_SECRET'] ?? '',
    'stripe_webhook_secret' => $_ENV['STRIPE_WEBHOOK_SECRET'] ?? '',
    'billing_success_url' => ($_ENV['APP_URL'] ?? 'http://localhost:8080') . '/billing?success=true',
    'billing_cancel_url' => ($_ENV['APP_URL'] ?? 'http://localhost:8080') . '/billing?canceled=true',
    'billing_portal_return_url' => ($_ENV['APP_URL'] ?? 'http://localhost:8080') . '/billing',
    'billing_founding_member_cap' => (int) ($_ENV['FOUNDING_MEMBER_CAP'] ?? 100),
    'billing_price_tier_map' => [
        ($_ENV['STRIPE_GROWTH_MONTHLY_PRICE'] ?? '') => 'growth',
        ($_ENV['STRIPE_GROWTH_YEARLY_PRICE'] ?? '') => 'growth',
        ($_ENV['STRIPE_BUSINESS_MONTHLY_PRICE'] ?? '') => 'business',
        ($_ENV['STRIPE_BUSINESS_YEARLY_PRICE'] ?? '') => 'business',
        ($_ENV['STRIPE_PRO_MONTHLY_PRICE'] ?? '') => 'pro',
        ($_ENV['STRIPE_PRO_YEARLY_PRICE'] ?? '') => 'pro',
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

<?php

return [

    /*
    |--------------------------------------------------------------------------
    | Third Party Services
    |--------------------------------------------------------------------------
    |
    | This file is for storing the credentials for third party services such
    | as Mailgun, Postmark, AWS and more. This file provides the de facto
    | location for this type of information, allowing packages to have
    | a conventional file to locate the various service credentials.
    |
    */

    'postmark' => [
        'key' => env('POSTMARK_API_KEY'),
    ],

    'resend' => [
        'key' => env('RESEND_API_KEY'),
    ],

    'ses' => [
        'key' => env('AWS_ACCESS_KEY_ID'),
        'secret' => env('AWS_SECRET_ACCESS_KEY'),
        'region' => env('AWS_DEFAULT_REGION', 'us-east-1'),
    ],

    'slack' => [
        'notifications' => [
            'bot_user_oauth_token' => env('SLACK_BOT_USER_OAUTH_TOKEN'),
            'channel' => env('SLACK_BOT_USER_DEFAULT_CHANNEL'),
        ],
    ],

    'goforms' => [
        'url' => env('GOFORMS_API_URL', 'http://localhost:8090'),
        'public_url' => env('GOFORMS_PUBLIC_URL', env('GOFORMS_API_URL', 'http://localhost:8090')),
        'secret' => env('GOFORMS_SHARED_SECRET'),
        // Empty or unset: use seeded demo form ID (run goforms migration 2026022301_seed_demo_form).
'demo_form_id' => env('GOFORMS_DEMO_FORM_ID') ?: '22222222-2222-4222-8222-222222222222',
    ],

    'stripe' => [
        'prices' => [
            'pro_monthly' => env('STRIPE_PRO_MONTHLY_PRICE_ID'),
            'pro_annual' => env('STRIPE_PRO_ANNUAL_PRICE_ID'),
            'business_monthly' => env('STRIPE_BUSINESS_MONTHLY_PRICE_ID'),
            'business_annual' => env('STRIPE_BUSINESS_ANNUAL_PRICE_ID'),
            'growth_monthly' => env('STRIPE_GROWTH_MONTHLY_PRICE_ID'),
            'growth_annual' => env('STRIPE_GROWTH_ANNUAL_PRICE_ID'),
        ],
    ],

    'founding_member_cap' => (int) env('FOUNDING_MEMBER_CAP', 100),

];

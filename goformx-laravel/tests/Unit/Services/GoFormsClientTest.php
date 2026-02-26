<?php

uses(Tests\TestCase::class, Illuminate\Foundation\Testing\RefreshDatabase::class);

use App\Models\User;
use App\Services\GoFormsClient;
use Illuminate\Support\Facades\Http;

beforeEach(function () {
    config([
        'services.goforms.url' => 'http://localhost:8090',
        'services.goforms.secret' => 'test-secret',
        'services.stripe.prices.pro_monthly' => 'price_pro_monthly',
        'services.stripe.prices.pro_annual' => 'price_pro_annual',
        'services.stripe.prices.business_monthly' => 'price_business_monthly',
        'services.stripe.prices.business_annual' => 'price_business_annual',
        'services.stripe.prices.growth_monthly' => 'price_growth_monthly',
        'services.stripe.prices.growth_annual' => 'price_growth_annual',
    ]);
});

it('adds signed assertion headers to requests', function () {
    $capturedRequest = null;
    Http::fake(function ($request) use (&$capturedRequest) {
        $capturedRequest = $request;

        return Http::response(['data' => []]);
    });

    $user = User::factory()->create();
    $client = GoFormsClient::fromConfig()->withUser($user);
    $client->listForms();

    expect($capturedRequest)->not->toBeNull()
        ->and($capturedRequest->hasHeader('X-User-Id'))->toBeTrue()
        ->and($capturedRequest->hasHeader('X-Timestamp'))->toBeTrue()
        ->and($capturedRequest->hasHeader('X-Signature'))->toBeTrue()
        ->and($capturedRequest->header('X-User-Id'))->toContain((string) $user->id);
});

it('uses ISO 8601 format for X-Timestamp', function () {
    $capturedRequest = null;
    Http::fake(function ($request) use (&$capturedRequest) {
        $capturedRequest = $request;

        return Http::response(['data' => []]);
    });

    $user = User::factory()->create();
    $client = GoFormsClient::fromConfig()->withUser($user);
    $client->listForms();

    $timestamp = $capturedRequest->header('X-Timestamp')[0] ?? '';
    expect($timestamp)->toMatch('/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/');
});

it('computes X-Signature as HMAC-SHA256 hex of method:path:user_id:timestamp:plan_tier', function () {
    $capturedRequest = null;
    Http::fake(function ($request) use (&$capturedRequest) {
        $capturedRequest = $request;

        return Http::response(['data' => []]);
    });

    $user = User::factory()->create(['id' => 42]);
    $client = GoFormsClient::fromConfig()->withUser($user);
    $client->listForms();

    $userId = $capturedRequest->header('X-User-Id')[0] ?? '';
    $timestamp = $capturedRequest->header('X-Timestamp')[0] ?? '';
    $signature = $capturedRequest->header('X-Signature')[0] ?? '';
    $planTier = $capturedRequest->header('X-Plan-Tier')[0] ?? '';

    $expectedPayload = "GET:/api/forms:{$userId}:{$timestamp}:{$planTier}";
    $expectedSignature = hash_hmac('sha256', $expectedPayload, 'test-secret', false);

    expect($signature)->toBe($expectedSignature)
        ->and(strlen($signature))->toBe(64);
});

it('throws when user is not set', function () {
    Http::fake(fn () => Http::response([]));

    $client = GoFormsClient::fromConfig();

    expect(fn () => $client->listForms())->toThrow(\RuntimeException::class, 'GoFormsClient requires an authenticated user');
});

it('throws when GOFORMS_SHARED_SECRET is not set', function () {
    config(['services.goforms.secret' => null]);
    Http::fake(fn () => Http::response([]));

    $user = User::factory()->create();
    $client = GoFormsClient::fromConfig()->withUser($user);

    expect(fn () => $client->listForms())->toThrow(\RuntimeException::class, 'GOFORMS_SHARED_SECRET');
});

// Response shape tests — each asserts the exact Go API envelope is unwrapped correctly.
// If Go changes its response structure, these tests will catch the drift before it reaches prod.

it('listForms unwraps data.forms and returns a flat array', function () {
    Http::fake([
        '*/api/forms' => Http::response([
            'success' => true,
            'data' => [
                'forms' => [
                    ['id' => 'form-1', 'title' => 'Alpha'],
                    ['id' => 'form-2', 'title' => 'Beta'],
                ],
                'count' => 2,
            ],
        ]),
    ]);

    $forms = GoFormsClient::fromConfig()->withUser(User::factory()->create())->listForms();

    expect($forms)->toBeArray()->toHaveCount(2)
        ->and($forms[0]['id'])->toBe('form-1')
        ->and($forms[1]['title'])->toBe('Beta');
});

it('listForms returns empty array when data.forms is absent', function () {
    Http::fake(['*/api/forms' => Http::response(['success' => true, 'data' => []])]);

    $forms = GoFormsClient::fromConfig()->withUser(User::factory()->create())->listForms();

    expect($forms)->toBe([]);
});

it('getForm unwraps data.form and returns a flat form array', function () {
    Http::fake([
        '*/api/forms/form-1' => Http::response([
            'success' => true,
            'data' => ['form' => ['id' => 'form-1', 'title' => 'My Form', 'status' => 'draft']],
        ]),
    ]);

    $form = GoFormsClient::fromConfig()->withUser(User::factory()->create())->getForm('form-1');

    expect($form)->toBeArray()
        ->and($form['id'])->toBe('form-1')
        ->and($form['title'])->toBe('My Form')
        ->and($form)->not->toHaveKey('form');
});

it('getForm returns null on 404', function () {
    Http::fake(['*/api/forms/missing' => Http::response([], 404)]);

    $form = GoFormsClient::fromConfig()->withUser(User::factory()->create())->getForm('missing');

    expect($form)->toBeNull();
});

it('createForm unwraps data.form and returns a flat form array', function () {
    Http::fake([
        '*/api/forms' => Http::response([
            'success' => true,
            'data' => ['form' => ['id' => 'new-form-id', 'title' => 'Untitled Form', 'status' => 'draft']],
        ], 201),
    ]);

    $form = GoFormsClient::fromConfig()->withUser(User::factory()->create())->createForm(['title' => 'Untitled Form']);

    expect($form)->toBeArray()
        ->and($form['id'])->toBe('new-form-id')
        ->and($form)->not->toHaveKey('form');
});

it('updateForm unwraps data.form and returns a flat form array', function () {
    Http::fake([
        '*/api/forms/form-1' => Http::response([
            'success' => true,
            'data' => ['form' => ['id' => 'form-1', 'title' => 'Updated Title', 'status' => 'draft']],
        ]),
    ]);

    $form = GoFormsClient::fromConfig()->withUser(User::factory()->create())->updateForm('form-1', ['title' => 'Updated Title']);

    expect($form)->toBeArray()
        ->and($form['id'])->toBe('form-1')
        ->and($form['title'])->toBe('Updated Title')
        ->and($form)->not->toHaveKey('form');
});

it('listSubmissions unwraps data.submissions and returns a flat array', function () {
    Http::fake([
        '*/api/forms/form-1/submissions' => Http::response([
            'success' => true,
            'data' => [
                'submissions' => [
                    ['id' => 'sub-1', 'status' => 'pending', 'submitted_at' => '2026-02-21T04:00:00Z'],
                ],
                'count' => 1,
            ],
        ]),
    ]);

    $submissions = GoFormsClient::fromConfig()->withUser(User::factory()->create())->listSubmissions('form-1');

    expect($submissions)->toBeArray()->toHaveCount(1)
        ->and($submissions[0]['id'])->toBe('sub-1')
        ->and($submissions)->not->toHaveKey('submissions');
});

it('getSubmission returns flat submission data from data key', function () {
    Http::fake([
        '*/api/forms/form-1/submissions/sub-1' => Http::response([
            'success' => true,
            'data' => [
                'id' => 'sub-1',
                'form_id' => 'form-1',
                'status' => 'pending',
                'submitted_at' => '2026-02-21T04:00:00Z',
                'data' => ['email' => 'test@example.com'],
            ],
        ]),
    ]);

    $submission = GoFormsClient::fromConfig()->withUser(User::factory()->create())->getSubmission('form-1', 'sub-1');

    expect($submission)->toBeArray()
        ->and($submission['id'])->toBe('sub-1')
        ->and($submission['form_id'])->toBe('form-1');
});

it('includes X-Plan-Tier header in signed requests', function () {
    Http::fake([
        '*/api/forms' => Http::response(['data' => ['forms' => []]], 200),
    ]);

    $user = User::factory()->create();

    $client = GoFormsClient::fromConfig()->withUser($user);
    $client->listForms();

    Http::assertSent(function ($request) {
        return $request->hasHeader('X-Plan-Tier', 'free')
            && $request->hasHeader('X-User-Id')
            && $request->hasHeader('X-Timestamp')
            && $request->hasHeader('X-Signature');
    });
});

it('signs plan tier into HMAC payload', function () {
    Http::fake([
        '*/api/forms' => Http::response(['data' => ['forms' => []]], 200),
    ]);

    $user = User::factory()->create();

    $client = GoFormsClient::fromConfig()->withUser($user);
    $client->listForms();

    Http::assertSent(function ($request) {
        $userId = $request->header('X-User-Id')[0];
        $timestamp = $request->header('X-Timestamp')[0];
        $signature = $request->header('X-Signature')[0];
        $planTier = $request->header('X-Plan-Tier')[0];

        $expectedPayload = 'GET:/api/forms:'.$userId.':'.$timestamp.':'.$planTier;
        $expectedSignature = hash_hmac('sha256', $expectedPayload, 'test-secret');

        return $signature === $expectedSignature;
    });
});

it('signs only the path portion when URL contains query parameters', function () {
    Http::fake([
        '*/api/forms/usage/submissions-count*' => Http::response(['data' => ['count' => 5]], 200),
    ]);

    $user = User::factory()->create();
    $client = GoFormsClient::fromConfig()->withUser($user);
    $client->getSubmissionsCount('2026-02');

    Http::assertSent(function ($request) {
        $userId = $request->header('X-User-Id')[0];
        $timestamp = $request->header('X-Timestamp')[0];
        $signature = $request->header('X-Signature')[0];
        $planTier = $request->header('X-Plan-Tier')[0];

        // HMAC payload must use path only (no query string) to match Go's c.Request().URL.Path
        $expectedPayload = 'GET:/api/forms/usage/submissions-count:'.$userId.':'.$timestamp.':'.$planTier;
        $expectedSignature = hash_hmac('sha256', $expectedPayload, 'test-secret');

        return $signature === $expectedSignature;
    });
});

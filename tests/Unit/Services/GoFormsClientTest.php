<?php

uses(Tests\TestCase::class, Illuminate\Foundation\Testing\RefreshDatabase::class);

use App\Models\User;
use App\Services\GoFormsClient;
use Illuminate\Support\Facades\Http;

beforeEach(function () {
    config([
        'services.goforms.url' => 'http://localhost:8090',
        'services.goforms.secret' => 'test-secret',
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

it('computes X-Signature as HMAC-SHA256 hex of user_id:timestamp', function () {
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

    $expectedPayload = "{$userId}:{$timestamp}";
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

<?php

use App\Models\User;
use Illuminate\Support\Facades\Http;
use Inertia\Testing\AssertableInertia as Assert;

beforeEach(function () {
    config([
        'services.goforms.url' => 'http://localhost:8090',
        'services.goforms.secret' => 'test-secret',
    ]);
});

test('index returns Inertia Forms/Index with forms', function () {
    Http::fake([
        '*/api/forms' => Http::response([
            'data' => [
                ['id' => '1', 'title' => 'Form One', 'description' => 'First form'],
                ['id' => '2', 'title' => 'Form Two'],
            ],
        ]),
    ]);

    $user = User::factory()->create();

    $response = $this
        ->actingAs($user)
        ->get(route('forms.index'));

    $response->assertOk();
    $response->assertInertia(fn (Assert $page) => $page
        ->component('Forms/Index')
        ->has('forms', 2)
        ->where('forms.0.title', 'Form One')
        ->where('forms.1.title', 'Form Two')
    );
});

test('index returns empty forms when API returns empty', function () {
    Http::fake(['*/api/forms' => Http::response(['data' => []])]);

    $user = User::factory()->create();

    $response = $this
        ->actingAs($user)
        ->get(route('forms.index'));

    $response->assertOk();
    $response->assertInertia(fn (Assert $page) => $page
        ->component('Forms/Index')
        ->has('forms', 0)
    );
});

test('edit returns Inertia Forms/Edit with form', function () {
    Http::fake([
        '*/api/forms/abc-123' => Http::response([
            'data' => ['id' => 'abc-123', 'title' => 'My Form', 'description' => 'Test'],
        ]),
    ]);

    $user = User::factory()->create();

    $response = $this
        ->actingAs($user)
        ->get(route('forms.edit', 'abc-123'));

    $response->assertOk();
    $response->assertInertia(fn (Assert $page) => $page
        ->component('Forms/Edit')
        ->where('form.title', 'My Form')
        ->where('form.id', 'abc-123')
    );
});

test('edit returns 404 when form does not exist', function () {
    Http::fake(['*/api/forms/unknown' => Http::response([], 404)]);

    $user = User::factory()->create();

    $response = $this
        ->actingAs($user)
        ->get(route('forms.edit', 'unknown'));

    $response->assertStatus(404);
});

test('store creates form and redirects to form edit', function () {
    Http::fake([
        '*/api/forms' => Http::response([
            'data' => ['id' => 'new-id', 'title' => 'New Form'],
        ]),
    ]);

    $user = User::factory()->create();

    $response = $this
        ->actingAs($user)
        ->post(route('forms.store'), [
            'title' => 'New Form',
            'description' => 'A new form',
        ]);

    $response->assertRedirect(route('forms.edit', 'new-id'));
    $response->assertSessionHas('success');
});

test('update redirects back with success', function () {
    Http::fake(['*/api/forms/form-1' => Http::response(['data' => []])]);

    $user = User::factory()->create();

    $response = $this
        ->actingAs($user)
        ->put(route('forms.update', 'form-1'), [
            'title' => 'Updated Title',
            'description' => 'Updated',
        ]);

    $response->assertRedirect();
    $response->assertSessionHas('success');
});

test('destroy deletes form and redirects to index', function () {
    Http::fake(['*/api/forms/form-1' => Http::response([], 200)]);

    $user = User::factory()->create();

    $response = $this
        ->actingAs($user)
        ->delete(route('forms.destroy', 'form-1'));

    $response->assertRedirect(route('forms.index'));
    $response->assertSessionHas('success');
});

test('index returns empty forms when Go API returns 401', function () {
    Http::fake(['*/api/forms' => Http::response(['error' => 'no session found'], 401)]);

    $user = User::factory()->create();

    $response = $this
        ->actingAs($user)
        ->get(route('forms.index'));

    $response->assertOk();
    $response->assertInertia(fn (Assert $page) => $page
        ->component('Forms/Index')
        ->has('forms', 0)
    );
});

test('index returns empty forms when Go API returns 404', function () {
    Http::fake(['*/api/forms' => Http::response([], 404)]);

    $user = User::factory()->create();

    $response = $this
        ->actingAs($user)
        ->get(route('forms.index'));

    $response->assertOk();
    $response->assertInertia(fn (Assert $page) => $page
        ->component('Forms/Index')
        ->has('forms', 0)
    );
});

test('index redirects with error when Go API returns 5xx', function () {
    Http::fake(['*/api/forms' => Http::response(['error' => 'Server error'], 500)]);

    $user = User::factory()->create();

    $response = $this
        ->actingAs($user)
        ->get(route('forms.index'));

    $response->assertRedirect();
    $response->assertSessionHas('error', 'Form service temporarily unavailable.');
});

test('index redirects with error when Go API returns 502', function () {
    Http::fake(['*/api/forms' => Http::response(['error' => 'Bad Gateway'], 502)]);

    $user = User::factory()->create();

    $response = $this
        ->actingAs($user)
        ->get(route('forms.index'));

    $response->assertRedirect();
    $response->assertSessionHas('error', 'Form service temporarily unavailable.');
});

test('edit redirects with error when Go API returns 502', function () {
    Http::fake(['*/api/forms/abc-123' => Http::response(['error' => 'Bad Gateway'], 502)]);

    $user = User::factory()->create();

    $response = $this
        ->actingAs($user)
        ->get(route('forms.edit', 'abc-123'));

    $response->assertRedirect();
    $response->assertSessionHas('error', 'Form service temporarily unavailable.');
});

test('store returns validation errors when Go API returns 422 with errors object', function () {
    Http::fake([
        '*/api/forms' => Http::response([
            'errors' => [
                'title' => ['The title field is required.'],
                'description' => ['Description is too long.'],
            ],
        ], 422),
    ]);

    $user = User::factory()->create();

    $response = $this
        ->actingAs($user)
        ->post(route('forms.store'), [
            'title' => 'Valid Title',
            'description' => 'Passes Laravel validation',
        ]);

    $response->assertRedirect();
    $response->assertSessionHasErrors(['title', 'description']);
});

test('store returns validation errors when Go API returns 422 with data.errors array', function () {
    Http::fake([
        '*/api/forms' => Http::response([
            'data' => [
                'errors' => [
                    ['field' => 'title', 'message' => 'Title is required'],
                    ['field' => 'title', 'message' => 'Title must be at least 3 characters'],
                ],
            ],
        ], 422),
    ]);

    $user = User::factory()->create();

    $response = $this
        ->actingAs($user)
        ->post(route('forms.store'), [
            'title' => 'Valid Title',
            'description' => null,
        ]);

    $response->assertRedirect();
    $response->assertSessionHasErrors(['title']);
});

test('forms routes require authentication', function () {
    $response = $this->get(route('forms.index'));
    $response->assertRedirect(route('login'));
});

test('preview returns Inertia Forms/Preview with form', function () {
    Http::fake([
        '*/api/forms/abc-123' => Http::response([
            'data' => [
                'id' => 'abc-123',
                'title' => 'My Form',
                'schema' => ['display' => 'form', 'components' => []],
            ],
        ]),
    ]);

    $user = User::factory()->create();

    $response = $this
        ->actingAs($user)
        ->get(route('forms.preview', 'abc-123'));

    $response->assertOk();
    $response->assertInertia(fn (Assert $page) => $page
        ->component('Forms/Preview')
        ->where('form.title', 'My Form')
        ->where('form.id', 'abc-123')
    );
});

test('preview returns 404 when form does not exist', function () {
    Http::fake(['*/api/forms/unknown' => Http::response([], 404)]);

    $user = User::factory()->create();

    $response = $this
        ->actingAs($user)
        ->get(route('forms.preview', 'unknown'));

    $response->assertStatus(404);
});

test('submissions returns Inertia Forms/Submissions with form and submissions', function () {
    Http::fake([
        '*/api/forms/abc-123' => Http::response([
            'data' => ['id' => 'abc-123', 'title' => 'My Form'],
        ]),
        '*/api/forms/abc-123/submissions' => Http::response([
            'data' => [
                'submissions' => [
                    ['id' => 'sub-1', 'submitted_at' => '2026-02-18T12:00:00Z', 'status' => 'pending'],
                ],
                'count' => 1,
            ],
        ]),
    ]);

    $user = User::factory()->create();

    $response = $this
        ->actingAs($user)
        ->get(route('forms.submissions', 'abc-123'));

    $response->assertOk();
    $response->assertInertia(fn (Assert $page) => $page
        ->component('Forms/Submissions')
        ->where('form.title', 'My Form')
        ->has('submissions')
    );
});

test('submissions returns 404 when form does not exist', function () {
    Http::fake(['*/api/forms/unknown' => Http::response([], 404)]);

    $user = User::factory()->create();

    $response = $this
        ->actingAs($user)
        ->get(route('forms.submissions', 'unknown'));

    $response->assertStatus(404);
});

test('submission show returns Inertia Forms/SubmissionShow with form and submission', function () {
    Http::fake([
        '*/api/forms/abc-123' => Http::response([
            'data' => ['id' => 'abc-123', 'title' => 'My Form'],
        ]),
        '*/api/forms/abc-123/submissions/sub-1' => Http::response([
            'data' => [
                'id' => 'sub-1',
                'form_id' => 'abc-123',
                'status' => 'pending',
                'submitted_at' => '2026-02-18T12:00:00Z',
                'data' => ['email' => 'test@example.com'],
            ],
        ]),
    ]);

    $user = User::factory()->create();

    $response = $this
        ->actingAs($user)
        ->get(route('forms.submissions.show', ['id' => 'abc-123', 'sid' => 'sub-1']));

    $response->assertOk();
    $response->assertInertia(fn (Assert $page) => $page
        ->component('Forms/SubmissionShow')
        ->where('form.title', 'My Form')
        ->where('submission.id', 'sub-1')
    );
});

test('submission show returns 404 when submission does not exist', function () {
    Http::fake([
        '*/api/forms/abc-123' => Http::response([
            'data' => ['id' => 'abc-123', 'title' => 'My Form'],
        ]),
        '*/api/forms/abc-123/submissions/unknown' => Http::response([], 404),
    ]);

    $user = User::factory()->create();

    $response = $this
        ->actingAs($user)
        ->get(route('forms.submissions.show', ['id' => 'abc-123', 'sid' => 'unknown']));

    $response->assertStatus(404);
});

test('embed returns Inertia Forms/Embed with form and embed_base_url', function () {
    Http::fake([
        '*/api/forms/abc-123' => Http::response([
            'data' => ['id' => 'abc-123', 'title' => 'My Form'],
        ]),
    ]);

    config(['services.goforms.public_url' => 'https://forms.example.com']);

    $user = User::factory()->create();

    $response = $this
        ->actingAs($user)
        ->get(route('forms.embed', 'abc-123'));

    $response->assertOk();
    $response->assertInertia(fn (Assert $page) => $page
        ->component('Forms/Embed')
        ->where('form.title', 'My Form')
        ->where('embed_base_url', 'https://forms.example.com')
    );
});

test('embed returns 404 when form does not exist', function () {
    Http::fake(['*/api/forms/unknown' => Http::response([], 404)]);

    $user = User::factory()->create();

    $response = $this
        ->actingAs($user)
        ->get(route('forms.embed', 'unknown'));

    $response->assertStatus(404);
});

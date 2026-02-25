<?php

namespace App\Http\Controllers;

use App\Http\Requests\StoreFormRequest;
use App\Http\Requests\UpdateFormRequest;
use App\Services\GoFormsClient;
use Illuminate\Http\Client\RequestException;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Log;
use Illuminate\Validation\ValidationException;
use Inertia\Inertia;
use Inertia\Response;
use Symfony\Component\HttpKernel\Exception\NotFoundHttpException;

class FormController extends Controller
{
    public function __construct(
        private readonly GoFormsClient $goFormsClient
    ) {}

    public function index(): Response|RedirectResponse
    {
        try {
            $client = $this->goFormsClient->withUser(auth()->user());
            $forms = $client->listForms();
        } catch (RequestException $e) {
            $response = $e->response;
            $status = $response?->status();
            if ($status !== null && in_array($status, [401, 404], true)) {
                Log::warning('GoForms API error on list forms; showing empty list', [
                    'status' => $status,
                    'body' => $response->body(),
                ]);

                return Inertia::render('Forms/Index', ['forms' => []]);
            }

            return $this->handleGoError($e, request());
        }

        return Inertia::render('Forms/Index', [
            'forms' => $forms,
        ]);
    }

    public function edit(Request $request, string $id): Response|RedirectResponse
    {
        try {
            $client = $this->goFormsClient->withUser(auth()->user());
            $form = $client->getForm($id);
        } catch (RequestException $e) {
            return $this->handleGoError($e, $request);
        }

        if ($form === null) {
            throw new NotFoundHttpException('Form not found.');
        }

        return Inertia::render('Forms/Edit', [
            'form' => $form,
        ]);
    }

    public function preview(Request $request, string $id): Response|RedirectResponse
    {
        try {
            $client = $this->goFormsClient->withUser(auth()->user());
            $form = $client->getForm($id);
        } catch (RequestException $e) {
            return $this->handleGoError($e, $request);
        }

        if ($form === null) {
            throw new NotFoundHttpException('Form not found.');
        }

        return Inertia::render('Forms/Preview', [
            'form' => $form,
        ]);
    }

    public function submissions(Request $request, string $id): Response|RedirectResponse
    {
        try {
            $client = $this->goFormsClient->withUser(auth()->user());
            $form = $client->getForm($id);
        } catch (RequestException $e) {
            return $this->handleGoError($e, $request);
        }

        if ($form === null) {
            throw new NotFoundHttpException('Form not found.');
        }

        try {
            $submissions = $client->listSubmissions($id);
        } catch (RequestException $e) {
            return $this->handleGoError($e, $request);
        }

        return Inertia::render('Forms/Submissions', [
            'form' => $form,
            'submissions' => $submissions,
        ]);
    }

    public function submission(Request $request, string $id, string $sid): Response|RedirectResponse
    {
        try {
            $client = $this->goFormsClient->withUser(auth()->user());
            $form = $client->getForm($id);
        } catch (RequestException $e) {
            return $this->handleGoError($e, $request);
        }

        if ($form === null) {
            throw new NotFoundHttpException('Form not found.');
        }

        try {
            $submission = $client->getSubmission($id, $sid);
        } catch (RequestException $e) {
            if ($e->response && $e->response->status() === 404) {
                throw new NotFoundHttpException('Submission not found.');
            }

            return $this->handleGoError($e, $request);
        }

        if ($submission === null) {
            throw new NotFoundHttpException('Submission not found.');
        }

        return Inertia::render('Forms/SubmissionShow', [
            'form' => $form,
            'submission' => $submission,
        ]);
    }

    public function embed(Request $request, string $id): Response|RedirectResponse
    {
        try {
            $client = $this->goFormsClient->withUser(auth()->user());
            $form = $client->getForm($id);
        } catch (RequestException $e) {
            return $this->handleGoError($e, $request);
        }

        if ($form === null) {
            throw new NotFoundHttpException('Form not found.');
        }

        $embedBaseUrl = rtrim(config('services.goforms.public_url', config('services.goforms.url')), '/');

        return Inertia::render('Forms/Embed', [
            'form' => $form,
            'embed_base_url' => $embedBaseUrl,
        ]);
    }

    public function store(StoreFormRequest $request): RedirectResponse
    {
        try {
            $client = $this->goFormsClient->withUser(auth()->user());
            $form = $client->createForm($request->validated());
        } catch (RequestException $e) {
            $status = $e->response?->status();
            if ($status !== null && in_array($status, [401, 404], true)) {
                Log::warning('GoForms API error on create form; redirecting back', [
                    'status' => $status,
                    'body' => $e->response->body(),
                ]);

                return redirect()->route('forms.index')
                    ->with('error', 'Form service could not create the form. Please try again.')
                    ->withInput();
            }

            return $this->handleGoError($e, $request);
        }

        $formId = $form['id'] ?? $form['ID'] ?? null;

        if ($formId === null) {
            Log::warning('GoForms API create returned no form id', ['response' => $form]);

            return redirect()->route('forms.index')
                ->with('error', 'Form service returned an invalid response. Please try again.')
                ->withInput();
        }

        return redirect()->route('forms.edit', $formId)
            ->with('success', 'Form created successfully.');
    }

    public function update(UpdateFormRequest $request, string $id): RedirectResponse
    {
        try {
            $client = $this->goFormsClient->withUser(auth()->user());
            $client->updateForm($id, $request->validated());
        } catch (RequestException $e) {
            return $this->handleGoError($e, $request);
        }

        return redirect()->back()->with('success', 'Form updated successfully.');
    }

    public function destroy(string $id): RedirectResponse
    {
        try {
            $client = $this->goFormsClient->withUser(auth()->user());
            $client->deleteForm($id);
        } catch (RequestException $e) {
            return $this->handleGoError($e, request());
        }

        return redirect()->route('forms.index')->with('success', 'Form deleted successfully.');
    }

    /**
     * Map Go API errors to user-facing responses.
     *
     * - null response (connection refused, timeout): "Form service temporarily unavailable"
     * - 404: NotFoundHttpException
     * - 403: Redirect back with upgrade prompt (plan limit / feature gating)
     * - 401: Log as misconfiguration, generic error message
     * - 400/422: Parse Go validation errors into Inertia validation format
     * - 5xx: Log, "Form service temporarily unavailable"
     * - Other: Generic "An error occurred" message
     */
    private function handleGoError(RequestException $e, Request $request): RedirectResponse
    {
        if ($e->response === null) {
            Log::error('GoForms API unreachable (connection refused, timeout)', ['error' => $e->getMessage()]);

            return redirect()->back()
                ->with('error', 'Form service temporarily unavailable.')
                ->withInput();
        }

        $status = $e->response->status();

        if ($status === 404) {
            throw new NotFoundHttpException('Resource not found.');
        }

        if ($status === 403) {
            $body = $e->response->json();
            $requiredTier = $body['data']['required_tier'] ?? null;

            return redirect()->back()
                ->with('error', $body['message'] ?? 'Plan limit reached. Please upgrade.')
                ->with('upgrade_tier', $requiredTier)
                ->withInput();
        }

        if ($status === 401) {
            Log::error('GoForms API returned 401 (auth misconfiguration)', [
                'path' => $request->path(),
                'body' => $e->response->body(),
            ]);

            return redirect()->back()
                ->with('error', 'An unexpected error occurred. Please try again.')
                ->withInput();
        }

        if (in_array($status, [400, 422], true)) {
            $messages = $this->parseGoValidationErrors($e->response);
            throw ValidationException::withMessages($messages);
        }

        if ($status >= 500) {
            Log::error('GoForms API server error', ['status' => $status, 'body' => $e->response->body()]);

            return redirect()->back()
                ->with('error', 'Form service temporarily unavailable.')
                ->withInput();
        }

        return redirect()->back()
            ->with('error', 'An error occurred. Please try again.')
            ->withInput();
    }

    /**
     * Parse Go validation JSON to Laravel/Inertia format.
     *
     * Supports:
     * - { errors: { field: [messages] } } (Laravel-style)
     * - { data: { errors: [{ field, message }] } } (Go BuildMultipleErrorResponse)
     * - { data: { field, message } } (Go BuildValidationErrorResponse)
     *
     * @return array<string, array<int, string>>
     */
    private function parseGoValidationErrors(\Illuminate\Http\Client\Response $response): array
    {
        $body = $response->json() ?? [];

        $errors = $body['errors'] ?? null;
        if (is_array($errors)) {
            $normalized = [];
            foreach ($errors as $field => $messages) {
                $normalized[$field] = is_array($messages) ? array_values(array_map('strval', $messages)) : [strval($messages)];
            }

            if ($normalized !== []) {
                return $normalized;
            }
        }

        $dataErrors = $body['data']['errors'] ?? null;
        if (is_array($dataErrors)) {
            $normalized = [];
            foreach ($dataErrors as $item) {
                $field = $item['field'] ?? 'form';
                $message = $item['message'] ?? 'Validation failed.';
                $normalized[$field] = array_merge($normalized[$field] ?? [], [$message]);
            }

            if ($normalized !== []) {
                return $normalized;
            }
        }

        $field = $body['data']['field'] ?? null;
        $message = $body['data']['message'] ?? $body['message'] ?? 'Validation failed.';
        if (is_string($field) && is_string($message)) {
            return [$field => [$message]];
        }

        return ['form' => [is_string($message) ? $message : 'Validation failed.']];
    }
}

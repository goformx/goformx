<?php

namespace App\Http\Controllers;

use App\Services\GoFormsClient;
use Illuminate\Http\Client\RequestException;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Log;
use Inertia\Inertia;
use Inertia\Response;

class BillingController extends Controller
{
    public function __construct(
        private readonly GoFormsClient $goFormsClient,
    ) {}

    public function index(Request $request): Response
    {
        $user = $request->user();
        $client = $this->goFormsClient->withUser($user);

        $formsCount = 0;
        $submissionsCount = 0;

        try {
            $formsCount = $client->getFormsCount();
            $submissionsCount = $client->getSubmissionsCount(now()->format('Y-m'));
        } catch (RequestException $e) {
            Log::warning('Failed to fetch usage data from GoForms API', [
                'status' => $e->response?->status(),
                'error' => $e->getMessage(),
            ]);
        }

        return Inertia::render('Billing/Index', [
            'currentTier' => $user->planTier(),
            'subscription' => $user->subscription()?->only(['stripe_status', 'ends_at', 'trial_ends_at']),
            'usage' => [
                'forms' => $formsCount,
                'submissions' => $submissionsCount,
            ],
            'prices' => config('services.stripe.prices'),
        ]);
    }

    public function checkout(Request $request): RedirectResponse
    {
        $request->validate([
            'price_id' => ['required', 'string'],
        ]);

        try {
            return $request->user()
                ->newSubscription('default', $request->input('price_id'))
                ->checkout([
                    'success_url' => route('billing.index').'?checkout=success',
                    'cancel_url' => route('billing.index').'?checkout=cancelled',
                ])
                ->redirect();
        } catch (\Stripe\Exception\InvalidRequestException $e) {
            Log::error('Stripe checkout failed: invalid request', ['error' => $e->getMessage()]);

            return redirect()->route('billing.index')
                ->with('error', 'Invalid subscription plan. Please try again.');
        } catch (\Stripe\Exception\ApiConnectionException $e) {
            Log::error('Stripe API unreachable', ['error' => $e->getMessage()]);

            return redirect()->route('billing.index')
                ->with('error', 'Payment service temporarily unavailable. Please try again.');
        } catch (\Exception $e) {
            Log::error('Stripe checkout unexpected error', ['error' => $e->getMessage()]);

            return redirect()->route('billing.index')
                ->with('error', 'Unable to start checkout. Please try again.');
        }
    }

    public function portal(Request $request): RedirectResponse
    {
        try {
            return $request->user()->redirectToBillingPortal(route('billing.index'));
        } catch (\Exception $e) {
            Log::error('Stripe billing portal error', ['error' => $e->getMessage()]);

            return redirect()->route('billing.index')
                ->with('error', 'Unable to open billing portal. Please try again.');
        }
    }
}

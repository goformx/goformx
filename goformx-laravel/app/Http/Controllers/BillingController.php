<?php

namespace App\Http\Controllers;

use App\Services\GoFormsClient;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;
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

        $formsCount = $client->getFormsCount();
        $submissionsCount = $client->getSubmissionsCount(now()->format('Y-m'));

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

        return $request->user()
            ->newSubscription('default', $request->input('price_id'))
            ->checkout([
                'success_url' => route('billing.index').'?checkout=success',
                'cancel_url' => route('billing.index').'?checkout=cancelled',
            ])
            ->redirect();
    }

    public function portal(Request $request): RedirectResponse
    {
        return $request->user()->redirectToBillingPortal(route('billing.index'));
    }
}

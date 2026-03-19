<?php

declare(strict_types=1);

namespace GoFormX\Controller;

use GoFormX\Service\GoFormsClientInterface;
use Waaseyaa\Billing\BillingManager;
use Waaseyaa\Billing\SubscriptionData;
use Waaseyaa\Inertia\Inertia;
use Waaseyaa\Inertia\InertiaResponse;

final class BillingController
{
    public function __construct(
        private readonly BillingManager $billing,
        private readonly GoFormsClientInterface $goForms,
    ) {
    }

    /**
     * @param list<SubscriptionData> $subscriptions
     */
    public function index(
        string $userId,
        string $planTier,
        ?string $planOverride,
        ?string $stripeId,
        array $subscriptions,
    ): InertiaResponse {
        $tier = $this->billing->resolveUserTier($planOverride, $subscriptions);

        $usage = [];
        try {
            $formsCount = $this->goForms->get('/api/forms/usage/forms-count', $userId, $planTier);
            $usage['forms_count'] = $formsCount['data']['count'] ?? 0;
        } catch (\RuntimeException) {
            $usage['forms_count'] = 0;
        }

        return Inertia::render('Billing/Index', [
            'tier' => $tier->value,
            'is_paid' => $tier->isPaid(),
            'stripe_id' => $stripeId,
            'usage' => $usage,
        ]);
    }

    public function checkout(string $stripeCustomerId, string $priceId): string
    {
        $session = $this->billing->createCheckoutSession($stripeCustomerId, $priceId);

        return $session->url;
    }

    public function portal(string $stripeCustomerId): string
    {
        return $this->billing->getPortalUrl($stripeCustomerId);
    }
}

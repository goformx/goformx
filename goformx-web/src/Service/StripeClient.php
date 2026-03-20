<?php

declare(strict_types=1);

namespace GoFormX\Service;

use Waaseyaa\Billing\CheckoutSession;
use Waaseyaa\Billing\StripeClientInterface;

final class StripeClient implements StripeClientInterface
{
    private \Stripe\StripeClient $client;
    private string $webhookSecret;

    public function __construct(string $secretKey, string $webhookSecret = '')
    {
        $this->client = new \Stripe\StripeClient($secretKey);
        $this->webhookSecret = $webhookSecret;
    }

    public function createCheckoutSession(array $params): CheckoutSession
    {
        $session = $this->client->checkout->sessions->create($params);

        return new CheckoutSession(
            id: $session->id,
            url: $session->url,
        );
    }

    public function createPortalSession(string $customerId, string $returnUrl): string
    {
        $session = $this->client->billingPortal->sessions->create([
            'customer' => $customerId,
            'return_url' => $returnUrl,
        ]);

        return $session->url;
    }

    public function constructWebhookEvent(string $payload, string $signature): array
    {
        $event = \Stripe\Webhook::constructEvent(
            $payload,
            $signature,
            $this->webhookSecret,
        );

        return $event->toArray();
    }
}

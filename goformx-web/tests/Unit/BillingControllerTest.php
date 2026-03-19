<?php

declare(strict_types=1);

namespace GoFormX\Tests\Unit;

use GoFormX\Controller\BillingController;
use GoFormX\Service\GoFormsClientInterface;
use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;
use Waaseyaa\Billing\BillingManager;
use Waaseyaa\Billing\CheckoutSession;
use Waaseyaa\Billing\FakeStripeClient;
use Waaseyaa\Inertia\Inertia;
use Waaseyaa\Inertia\InertiaResponse;

#[CoversClass(BillingController::class)]
final class BillingControllerTest extends TestCase
{
    private FakeStripeClient $stripe;
    private BillingManager $billing;

    protected function setUp(): void
    {
        Inertia::reset();
        Inertia::setVersion('v1');

        $this->stripe = new FakeStripeClient();
        $this->billing = new BillingManager(
            stripe: $this->stripe,
            priceTierMap: ['price_pro' => 'pro'],
            successUrl: 'http://test/success',
            cancelUrl: 'http://test/cancel',
            portalReturnUrl: 'http://test/billing',
        );
    }

    public function testIndexReturnsTierAndUsage(): void
    {
        $goForms = $this->createMock(GoFormsClientInterface::class);
        $goForms->method('get')->willReturn(['data' => ['count' => 5]]);

        $controller = new BillingController($this->billing, $goForms);
        $response = $controller->index('user-1', 'free', null, null, []);

        $this->assertInstanceOf(InertiaResponse::class, $response);
        $page = $response->toPageObject();
        $this->assertSame('Billing/Index', $page['component']);
        $this->assertSame('free', $page['props']['tier']);
        $this->assertFalse($page['props']['is_paid']);
        $this->assertSame(5, $page['props']['usage']['forms_count']);
    }

    public function testCheckoutReturnsUrl(): void
    {
        $this->stripe->setNextCheckoutSession(new CheckoutSession('cs_1', 'https://checkout.stripe.com/cs_1'));
        $goForms = $this->createMock(GoFormsClientInterface::class);

        $controller = new BillingController($this->billing, $goForms);
        $url = $controller->checkout('cus_abc', 'price_pro');

        $this->assertSame('https://checkout.stripe.com/cs_1', $url);
    }

    public function testPortalReturnsUrl(): void
    {
        $this->stripe->setNextPortalUrl('https://billing.stripe.com/portal');
        $goForms = $this->createMock(GoFormsClientInterface::class);

        $controller = new BillingController($this->billing, $goForms);
        $url = $controller->portal('cus_abc');

        $this->assertSame('https://billing.stripe.com/portal', $url);
    }
}

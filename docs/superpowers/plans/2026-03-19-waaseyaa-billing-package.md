# `waaseyaa/billing` Package Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Stripe billing package for Waaseyaa providing checkout sessions, customer portal, subscription syncing via webhooks, plan tier resolution, and founding member support.

**Architecture:** BillingManager is the primary service — wraps Stripe API for checkout/portal, resolves user plan tiers from subscriptions and overrides. WebhookHandler processes Stripe events and syncs subscription state. PlanTier is an enum. The package uses Stripe's PHP SDK directly (no Cashier). Subscription data is stored as Waaseyaa entities. Configuration is injected via service provider from app config.

**Tech Stack:** PHP 8.4+, stripe/stripe-php ^16.0, Waaseyaa foundation/user packages, PHPUnit 10.5

**Spec:** `docs/superpowers/specs/2026-03-19-laravel-to-waaseyaa-migration-design.md` (Section: `waaseyaa/billing`)

**Working directory:** `/home/fsd42/dev/waaseyaa`

---

## File Structure

```
packages/billing/
├── composer.json
├── src/
│   ├── BillingServiceProvider.php    # Registers services, entity types
│   ├── BillingManager.php            # createCheckoutSession(), getPortalUrl(), resolveUserTier()
│   ├── WebhookHandler.php            # Processes Stripe webhook events
│   ├── PlanTier.php                  # Enum: free, pro, business, growth, enterprise
│   ├── CheckoutSession.php           # Value object returned by createCheckoutSession()
│   ├── SubscriptionData.php          # Value object for subscription state
│   └── StripeClientInterface.php     # Interface for Stripe API (testable)
└── tests/
    └── Unit/
        ├── PlanTierTest.php
        ├── BillingManagerTest.php
        ├── WebhookHandlerTest.php
        ├── CheckoutSessionTest.php
        └── SubscriptionDataTest.php
```

## Design Decisions

- **StripeClientInterface** wraps Stripe SDK calls — allows unit testing with a fake implementation without hitting Stripe API
- **No entity types in v1** — subscription data is managed via StripeClientInterface and value objects. Entity type definitions will be added when the app layer needs persistent subscription storage. This keeps the package testable without database dependencies.
- **PlanTier resolution** is pure logic (no DB queries) — the caller provides the user's subscription data and plan_override, BillingManager returns the tier.

---

### Task 1: Package Scaffold

**Files:**
- Create: `packages/billing/composer.json`
- Create: `packages/billing/src/BillingServiceProvider.php`
- Modify: `/home/fsd42/dev/waaseyaa/composer.json`

- [ ] **Step 1: Create composer.json**

```json
{
    "name": "waaseyaa/billing",
    "description": "Stripe billing for Waaseyaa — subscriptions, checkout, portal, plan tiers",
    "type": "library",
    "license": "GPL-2.0-or-later",
    "repositories": [
        { "type": "path", "url": "../foundation" }
    ],
    "require": {
        "php": ">=8.4",
        "waaseyaa/foundation": "@dev",
        "stripe/stripe-php": "^16.0"
    },
    "require-dev": {
        "phpunit/phpunit": "^10.5"
    },
    "autoload": {
        "psr-4": { "Waaseyaa\\Billing\\": "src/" }
    },
    "autoload-dev": {
        "psr-4": { "Waaseyaa\\Billing\\Tests\\": "tests/" }
    },
    "extra": {
        "waaseyaa": {
            "providers": ["Waaseyaa\\Billing\\BillingServiceProvider"]
        },
        "branch-alias": { "dev-main": "0.1.x-dev" }
    },
    "minimum-stability": "dev",
    "prefer-stable": true
}
```

- [ ] **Step 2: Create minimal service provider**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Billing;

use Waaseyaa\Foundation\ServiceProvider\ServiceProvider;

final class BillingServiceProvider extends ServiceProvider
{
    public function register(): void
    {
    }
}
```

- [ ] **Step 3: Register package in root composer.json**

Add to root `/home/fsd42/dev/waaseyaa/composer.json`:
- Add path repository: `{ "type": "path", "url": "packages/billing" }`
- Add to require: `"waaseyaa/billing": "@dev"`
- Add to autoload-dev psr-4: `"Waaseyaa\\Billing\\Tests\\": "packages/billing/tests/"`

- [ ] **Step 4: Run composer update to verify wiring**

Run: `cd /home/fsd42/dev/waaseyaa && composer update waaseyaa/billing`
Expected: Package resolves and installs without errors. stripe/stripe-php is pulled in.

- [ ] **Step 5: Commit**

```bash
git add packages/billing/ composer.json composer.lock
git commit -m "feat(billing): scaffold waaseyaa/billing package with Stripe dependency"
```

---

### Task 2: PlanTier Enum

**Files:**
- Create: `packages/billing/src/PlanTier.php`
- Create: `packages/billing/tests/Unit/PlanTierTest.php`

- [ ] **Step 1: Write the failing test**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Billing\Tests\Unit;

use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;
use Waaseyaa\Billing\PlanTier;

#[CoversClass(PlanTier::class)]
final class PlanTierTest extends TestCase
{
    public function testAllTiersExist(): void
    {
        $this->assertSame('free', PlanTier::Free->value);
        $this->assertSame('pro', PlanTier::Pro->value);
        $this->assertSame('business', PlanTier::Business->value);
        $this->assertSame('growth', PlanTier::Growth->value);
        $this->assertSame('enterprise', PlanTier::Enterprise->value);
    }

    public function testFromValidString(): void
    {
        $this->assertSame(PlanTier::Pro, PlanTier::fromString('pro'));
        $this->assertSame(PlanTier::Business, PlanTier::fromString('business'));
    }

    public function testFromInvalidStringReturnsFree(): void
    {
        $this->assertSame(PlanTier::Free, PlanTier::fromString('invalid'));
        $this->assertSame(PlanTier::Free, PlanTier::fromString(''));
    }

    public function testFoundingMapsToBusiness(): void
    {
        $this->assertSame(PlanTier::Business, PlanTier::fromString('founding'));
    }

    public function testIsValidReturnsTrueForValidTiers(): void
    {
        $this->assertTrue(PlanTier::isValid('free'));
        $this->assertTrue(PlanTier::isValid('pro'));
        $this->assertTrue(PlanTier::isValid('business'));
        $this->assertTrue(PlanTier::isValid('growth'));
        $this->assertTrue(PlanTier::isValid('enterprise'));
    }

    public function testIsValidReturnsFalseForInvalidTiers(): void
    {
        $this->assertFalse(PlanTier::isValid('invalid'));
        $this->assertFalse(PlanTier::isValid('founding'));
        $this->assertFalse(PlanTier::isValid(''));
    }

    public function testIsPaidReturnsTrueForPaidTiers(): void
    {
        $this->assertTrue(PlanTier::Pro->isPaid());
        $this->assertTrue(PlanTier::Business->isPaid());
        $this->assertTrue(PlanTier::Growth->isPaid());
        $this->assertTrue(PlanTier::Enterprise->isPaid());
    }

    public function testIsPaidReturnsFalseForFree(): void
    {
        $this->assertFalse(PlanTier::Free->isPaid());
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/billing/tests/Unit/PlanTierTest.php`
Expected: FAIL — class not found.

- [ ] **Step 3: Write the implementation**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Billing;

enum PlanTier: string
{
    case Free = 'free';
    case Pro = 'pro';
    case Business = 'business';
    case Growth = 'growth';
    case Enterprise = 'enterprise';

    /**
     * Resolve a string to a PlanTier, with special handling for "founding" → Business.
     */
    public static function fromString(string $value): self
    {
        if ($value === 'founding') {
            return self::Business;
        }

        return self::tryFrom($value) ?? self::Free;
    }

    /**
     * Check if a string is a valid tier value (not including aliases like "founding").
     */
    public static function isValid(string $value): bool
    {
        return self::tryFrom($value) !== null;
    }

    public function isPaid(): bool
    {
        return $this !== self::Free;
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/billing/tests/Unit/PlanTierTest.php`
Expected: All 8 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/billing/src/PlanTier.php packages/billing/tests/Unit/PlanTierTest.php
git commit -m "feat(billing): add PlanTier enum with founding member mapping"
```

---

### Task 3: Value Objects — CheckoutSession & SubscriptionData

**Files:**
- Create: `packages/billing/src/CheckoutSession.php`
- Create: `packages/billing/src/SubscriptionData.php`
- Create: `packages/billing/tests/Unit/CheckoutSessionTest.php`
- Create: `packages/billing/tests/Unit/SubscriptionDataTest.php`

- [ ] **Step 1: Write the failing tests**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Billing\Tests\Unit;

use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;
use Waaseyaa\Billing\CheckoutSession;

#[CoversClass(CheckoutSession::class)]
final class CheckoutSessionTest extends TestCase
{
    public function testConstructAndAccessors(): void
    {
        $session = new CheckoutSession(
            id: 'cs_test_123',
            url: 'https://checkout.stripe.com/cs_test_123',
        );

        $this->assertSame('cs_test_123', $session->id);
        $this->assertSame('https://checkout.stripe.com/cs_test_123', $session->url);
    }
}
```

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Billing\Tests\Unit;

use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;
use Waaseyaa\Billing\SubscriptionData;

#[CoversClass(SubscriptionData::class)]
final class SubscriptionDataTest extends TestCase
{
    public function testConstructAndAccessors(): void
    {
        $sub = new SubscriptionData(
            stripeId: 'sub_123',
            stripeStatus: 'active',
            stripePrice: 'price_pro_monthly',
            quantity: 1,
            trialEndsAt: null,
            endsAt: null,
        );

        $this->assertSame('sub_123', $sub->stripeId);
        $this->assertSame('active', $sub->stripeStatus);
        $this->assertSame('price_pro_monthly', $sub->stripePrice);
        $this->assertSame(1, $sub->quantity);
        $this->assertNull($sub->trialEndsAt);
        $this->assertNull($sub->endsAt);
    }

    public function testIsActive(): void
    {
        $active = new SubscriptionData('sub_1', 'active', 'price_1', 1, null, null);
        $trialing = new SubscriptionData('sub_2', 'trialing', 'price_1', 1, null, null);
        $canceled = new SubscriptionData('sub_3', 'canceled', 'price_1', 1, null, null);
        $pastDue = new SubscriptionData('sub_4', 'past_due', 'price_1', 1, null, null);

        $this->assertTrue($active->isActive());
        $this->assertTrue($trialing->isActive());
        $this->assertFalse($canceled->isActive());
        $this->assertFalse($pastDue->isActive());
    }

    public function testHasPrice(): void
    {
        $sub = new SubscriptionData('sub_1', 'active', 'price_pro_monthly', 1, null, null);

        $this->assertTrue($sub->hasPrice('price_pro_monthly'));
        $this->assertFalse($sub->hasPrice('price_growth_monthly'));
    }

    public function testHasAnyPrice(): void
    {
        $sub = new SubscriptionData('sub_1', 'active', 'price_pro_monthly', 1, null, null);

        $this->assertTrue($sub->hasAnyPrice(['price_pro_monthly', 'price_pro_yearly']));
        $this->assertFalse($sub->hasAnyPrice(['price_growth_monthly', 'price_growth_yearly']));
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/billing/tests/Unit/CheckoutSessionTest.php packages/billing/tests/Unit/SubscriptionDataTest.php`
Expected: FAIL — classes not found.

- [ ] **Step 3: Write CheckoutSession**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Billing;

final readonly class CheckoutSession
{
    public function __construct(
        public string $id,
        public string $url,
    ) {
    }
}
```

- [ ] **Step 4: Write SubscriptionData**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Billing;

final readonly class SubscriptionData
{
    private const array ACTIVE_STATUSES = ['active', 'trialing'];

    public function __construct(
        public string $stripeId,
        public string $stripeStatus,
        public string $stripePrice,
        public int $quantity,
        public ?int $trialEndsAt,
        public ?int $endsAt,
    ) {
    }

    public function isActive(): bool
    {
        return in_array($this->stripeStatus, self::ACTIVE_STATUSES, true);
    }

    public function hasPrice(string $priceId): bool
    {
        return $this->stripePrice === $priceId;
    }

    /**
     * @param list<string> $priceIds
     */
    public function hasAnyPrice(array $priceIds): bool
    {
        return in_array($this->stripePrice, $priceIds, true);
    }
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/billing/tests/Unit/CheckoutSessionTest.php packages/billing/tests/Unit/SubscriptionDataTest.php`
Expected: All 5 tests PASS.

- [ ] **Step 6: Commit**

```bash
git add packages/billing/src/CheckoutSession.php packages/billing/src/SubscriptionData.php packages/billing/tests/Unit/CheckoutSessionTest.php packages/billing/tests/Unit/SubscriptionDataTest.php
git commit -m "feat(billing): add CheckoutSession and SubscriptionData value objects"
```

---

### Task 4: StripeClientInterface & FakeStripeClient

**Files:**
- Create: `packages/billing/src/StripeClientInterface.php`
- Create: `packages/billing/src/FakeStripeClient.php`

The interface wraps Stripe SDK calls for testability. FakeStripeClient is used in tests.

- [ ] **Step 1: Write the interface and fake**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Billing;

interface StripeClientInterface
{
    /**
     * Create a Stripe Checkout Session.
     *
     * @param array<string, mixed> $params
     */
    public function createCheckoutSession(array $params): CheckoutSession;

    /**
     * Create a Stripe Customer Portal session and return the URL.
     */
    public function createPortalSession(string $customerId, string $returnUrl): string;

    /**
     * Verify a webhook signature and return the parsed event payload.
     *
     * @return array<string, mixed>
     * @throws \RuntimeException If signature verification fails
     */
    public function constructWebhookEvent(string $payload, string $signature): array;
}
```

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Billing;

final class FakeStripeClient implements StripeClientInterface
{
    private CheckoutSession $nextCheckoutSession;
    private string $nextPortalUrl = 'https://billing.stripe.com/portal/fake';
    /** @var array<string, mixed> */
    private array $nextWebhookEvent = [];

    public function __construct()
    {
        $this->nextCheckoutSession = new CheckoutSession('cs_fake', 'https://checkout.stripe.com/fake');
    }

    public function setNextCheckoutSession(CheckoutSession $session): void
    {
        $this->nextCheckoutSession = $session;
    }

    public function setNextPortalUrl(string $url): void
    {
        $this->nextPortalUrl = $url;
    }

    /**
     * @param array<string, mixed> $event
     */
    public function setNextWebhookEvent(array $event): void
    {
        $this->nextWebhookEvent = $event;
    }

    public function createCheckoutSession(array $params): CheckoutSession
    {
        return $this->nextCheckoutSession;
    }

    public function createPortalSession(string $customerId, string $returnUrl): string
    {
        return $this->nextPortalUrl;
    }

    public function constructWebhookEvent(string $payload, string $signature): array
    {
        if ($this->nextWebhookEvent === []) {
            throw new \RuntimeException('No fake webhook event configured');
        }

        return $this->nextWebhookEvent;
    }
}
```

- [ ] **Step 2: Commit**

```bash
git add packages/billing/src/StripeClientInterface.php packages/billing/src/FakeStripeClient.php
git commit -m "feat(billing): add StripeClientInterface and FakeStripeClient for testing"
```

---

### Task 5: BillingManager

**Files:**
- Create: `packages/billing/src/BillingManager.php`
- Create: `packages/billing/tests/Unit/BillingManagerTest.php`

Core billing service. Handles checkout, portal, and plan tier resolution.

- [ ] **Step 1: Write the failing test**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Billing\Tests\Unit;

use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;
use Waaseyaa\Billing\BillingManager;
use Waaseyaa\Billing\CheckoutSession;
use Waaseyaa\Billing\FakeStripeClient;
use Waaseyaa\Billing\PlanTier;
use Waaseyaa\Billing\SubscriptionData;

#[CoversClass(BillingManager::class)]
final class BillingManagerTest extends TestCase
{
    private FakeStripeClient $stripe;
    private BillingManager $billing;

    protected function setUp(): void
    {
        $this->stripe = new FakeStripeClient();
        $this->billing = new BillingManager(
            stripe: $this->stripe,
            priceTierMap: [
                'price_growth_monthly' => 'growth',
                'price_growth_yearly' => 'growth',
                'price_business_monthly' => 'business',
                'price_business_yearly' => 'business',
                'price_pro_monthly' => 'pro',
                'price_pro_yearly' => 'pro',
            ],
            successUrl: 'https://app.test/billing?success=true',
            cancelUrl: 'https://app.test/billing?canceled=true',
            portalReturnUrl: 'https://app.test/billing',
            foundingMemberCap: 100,
        );
    }

    public function testCreateCheckoutSessionReturnsSession(): void
    {
        $expected = new CheckoutSession('cs_123', 'https://checkout.stripe.com/cs_123');
        $this->stripe->setNextCheckoutSession($expected);

        $session = $this->billing->createCheckoutSession('cus_abc', 'price_pro_monthly');

        $this->assertSame('cs_123', $session->id);
        $this->assertSame('https://checkout.stripe.com/cs_123', $session->url);
    }

    public function testGetPortalUrlReturnsUrl(): void
    {
        $this->stripe->setNextPortalUrl('https://billing.stripe.com/portal/real');

        $url = $this->billing->getPortalUrl('cus_abc');

        $this->assertSame('https://billing.stripe.com/portal/real', $url);
    }

    public function testResolveTierFromPlanOverride(): void
    {
        $tier = $this->billing->resolveUserTier(
            planOverride: 'enterprise',
            subscriptions: [],
        );

        $this->assertSame(PlanTier::Enterprise, $tier);
    }

    public function testResolveTierFoundingMapsToBusiness(): void
    {
        $tier = $this->billing->resolveUserTier(
            planOverride: 'founding',
            subscriptions: [],
        );

        $this->assertSame(PlanTier::Business, $tier);
    }

    public function testResolveTierFromActiveSubscription(): void
    {
        $sub = new SubscriptionData('sub_1', 'active', 'price_growth_monthly', 1, null, null);

        $tier = $this->billing->resolveUserTier(
            planOverride: null,
            subscriptions: [$sub],
        );

        $this->assertSame(PlanTier::Growth, $tier);
    }

    public function testResolveTierIgnoresCanceledSubscription(): void
    {
        $sub = new SubscriptionData('sub_1', 'canceled', 'price_growth_monthly', 1, null, null);

        $tier = $this->billing->resolveUserTier(
            planOverride: null,
            subscriptions: [$sub],
        );

        $this->assertSame(PlanTier::Free, $tier);
    }

    public function testResolveTierPlanOverrideTakesPrecedenceOverSubscription(): void
    {
        $sub = new SubscriptionData('sub_1', 'active', 'price_pro_monthly', 1, null, null);

        $tier = $this->billing->resolveUserTier(
            planOverride: 'enterprise',
            subscriptions: [$sub],
        );

        $this->assertSame(PlanTier::Enterprise, $tier);
    }

    public function testResolveTierDefaultsToFree(): void
    {
        $tier = $this->billing->resolveUserTier(
            planOverride: null,
            subscriptions: [],
        );

        $this->assertSame(PlanTier::Free, $tier);
    }

    public function testResolveTierInvalidOverrideIgnored(): void
    {
        $tier = $this->billing->resolveUserTier(
            planOverride: 'garbage',
            subscriptions: [],
        );

        $this->assertSame(PlanTier::Free, $tier);
    }

    public function testResolveTierHighestSubscriptionWins(): void
    {
        $proSub = new SubscriptionData('sub_1', 'active', 'price_pro_monthly', 1, null, null);
        $growthSub = new SubscriptionData('sub_2', 'active', 'price_growth_monthly', 1, null, null);

        $tier = $this->billing->resolveUserTier(
            planOverride: null,
            subscriptions: [$proSub, $growthSub],
        );

        $this->assertSame(PlanTier::Growth, $tier);
    }

    public function testFoundingMemberSlotsRemaining(): void
    {
        $remaining = $this->billing->foundingMemberSlotsRemaining(currentCount: 42);

        $this->assertSame(58, $remaining);
    }

    public function testFoundingMemberSlotsRemainingNeverNegative(): void
    {
        $remaining = $this->billing->foundingMemberSlotsRemaining(currentCount: 200);

        $this->assertSame(0, $remaining);
    }

    public function testCanGrantFoundingMembership(): void
    {
        $this->assertTrue($this->billing->canGrantFoundingMembership(currentCount: 99));
        $this->assertFalse($this->billing->canGrantFoundingMembership(currentCount: 100));
    }

    public function testResolveTierTrialingCountsAsActive(): void
    {
        $sub = new SubscriptionData('sub_1', 'trialing', 'price_pro_monthly', 1, null, null);

        $tier = $this->billing->resolveUserTier(
            planOverride: null,
            subscriptions: [$sub],
        );

        $this->assertSame(PlanTier::Pro, $tier);
    }

    public function testResolveTierUnknownPriceDefaultsToFree(): void
    {
        $sub = new SubscriptionData('sub_1', 'active', 'price_unknown', 1, null, null);

        $tier = $this->billing->resolveUserTier(
            planOverride: null,
            subscriptions: [$sub],
        );

        $this->assertSame(PlanTier::Free, $tier);
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/billing/tests/Unit/BillingManagerTest.php`
Expected: FAIL — class not found.

- [ ] **Step 3: Write the implementation**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Billing;

final class BillingManager
{
    /** @var array<string, int> tier priority (higher = better) */
    private const array TIER_PRIORITY = [
        'free' => 0,
        'pro' => 1,
        'business' => 2,
        'growth' => 3,
        'enterprise' => 4,
    ];

    /**
     * @param array<string, string> $priceTierMap Maps Stripe price IDs to tier names
     */
    public function __construct(
        private readonly StripeClientInterface $stripe,
        private readonly array $priceTierMap,
        private readonly string $successUrl,
        private readonly string $cancelUrl,
        private readonly string $portalReturnUrl,
        private readonly int $foundingMemberCap = 100,
    ) {
    }

    /**
     * Create a Stripe Checkout Session for a subscription.
     */
    public function createCheckoutSession(string $stripeCustomerId, string $priceId): CheckoutSession
    {
        return $this->stripe->createCheckoutSession([
            'customer' => $stripeCustomerId,
            'mode' => 'subscription',
            'line_items' => [['price' => $priceId, 'quantity' => 1]],
            'success_url' => $this->successUrl,
            'cancel_url' => $this->cancelUrl,
        ]);
    }

    /**
     * Get the Stripe Customer Portal URL.
     */
    public function getPortalUrl(string $stripeCustomerId): string
    {
        return $this->stripe->createPortalSession($stripeCustomerId, $this->portalReturnUrl);
    }

    /**
     * Resolve a user's plan tier from their override and subscriptions.
     *
     * Priority:
     * 1. plan_override (admin-set; "founding" → business)
     * 2. Highest active subscription tier
     * 3. Default: free
     *
     * @param list<SubscriptionData> $subscriptions
     */
    public function resolveUserTier(?string $planOverride, array $subscriptions): PlanTier
    {
        if ($planOverride !== null && $planOverride !== '') {
            $tier = PlanTier::fromString($planOverride);
            if ($tier !== PlanTier::Free || PlanTier::isValid($planOverride)) {
                return $tier;
            }
        }

        $highestPriority = -1;
        $highestTier = PlanTier::Free;

        foreach ($subscriptions as $sub) {
            if (!$sub->isActive()) {
                continue;
            }

            $tierName = $this->priceTierMap[$sub->stripePrice] ?? null;
            if ($tierName === null) {
                continue;
            }

            $priority = self::TIER_PRIORITY[$tierName] ?? 0;
            if ($priority > $highestPriority) {
                $highestPriority = $priority;
                $highestTier = PlanTier::fromString($tierName);
            }
        }

        return $highestTier;
    }

    /**
     * Calculate remaining founding member slots.
     */
    public function foundingMemberSlotsRemaining(int $currentCount): int
    {
        return max(0, $this->foundingMemberCap - $currentCount);
    }

    /**
     * Check if a founding membership can be granted.
     */
    public function canGrantFoundingMembership(int $currentCount): bool
    {
        return $this->foundingMemberSlotsRemaining($currentCount) > 0;
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/billing/tests/Unit/BillingManagerTest.php`
Expected: All 15 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/billing/src/BillingManager.php packages/billing/tests/Unit/BillingManagerTest.php
git commit -m "feat(billing): add BillingManager with checkout, portal, and tier resolution"
```

---

### Task 6: WebhookHandler

**Files:**
- Create: `packages/billing/src/WebhookHandler.php`
- Create: `packages/billing/tests/Unit/WebhookHandlerTest.php`

Processes Stripe webhook events. Returns structured data that the app layer can use to update entities.

- [ ] **Step 1: Write the failing test**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Billing\Tests\Unit;

use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;
use Waaseyaa\Billing\FakeStripeClient;
use Waaseyaa\Billing\WebhookHandler;

#[CoversClass(WebhookHandler::class)]
final class WebhookHandlerTest extends TestCase
{
    private FakeStripeClient $stripe;
    private WebhookHandler $handler;

    protected function setUp(): void
    {
        $this->stripe = new FakeStripeClient();
        $this->handler = new WebhookHandler($this->stripe);
    }

    public function testHandleCheckoutSessionCompleted(): void
    {
        $this->stripe->setNextWebhookEvent([
            'type' => 'checkout.session.completed',
            'data' => [
                'object' => [
                    'customer' => 'cus_abc',
                    'subscription' => 'sub_123',
                    'metadata' => ['user_id' => 'user-1'],
                ],
            ],
        ]);

        $result = $this->handler->handle('payload', 'sig');

        $this->assertSame('checkout.session.completed', $result['event']);
        $this->assertSame('cus_abc', $result['customer_id']);
        $this->assertSame('sub_123', $result['subscription_id']);
    }

    public function testHandleSubscriptionCreated(): void
    {
        $this->stripe->setNextWebhookEvent([
            'type' => 'customer.subscription.created',
            'data' => [
                'object' => [
                    'id' => 'sub_123',
                    'customer' => 'cus_abc',
                    'status' => 'active',
                    'items' => [
                        'data' => [
                            ['price' => ['id' => 'price_pro_monthly']],
                        ],
                    ],
                ],
            ],
        ]);

        $result = $this->handler->handle('payload', 'sig');

        $this->assertSame('customer.subscription.created', $result['event']);
        $this->assertSame('sub_123', $result['subscription_id']);
        $this->assertSame('active', $result['status']);
        $this->assertSame('price_pro_monthly', $result['price_id']);
    }

    public function testHandleSubscriptionUpdated(): void
    {
        $this->stripe->setNextWebhookEvent([
            'type' => 'customer.subscription.updated',
            'data' => [
                'object' => [
                    'id' => 'sub_123',
                    'customer' => 'cus_abc',
                    'status' => 'past_due',
                    'items' => [
                        'data' => [
                            ['price' => ['id' => 'price_pro_monthly']],
                        ],
                    ],
                ],
            ],
        ]);

        $result = $this->handler->handle('payload', 'sig');

        $this->assertSame('customer.subscription.updated', $result['event']);
        $this->assertSame('past_due', $result['status']);
    }

    public function testHandleSubscriptionDeleted(): void
    {
        $this->stripe->setNextWebhookEvent([
            'type' => 'customer.subscription.deleted',
            'data' => [
                'object' => [
                    'id' => 'sub_123',
                    'customer' => 'cus_abc',
                    'status' => 'canceled',
                    'items' => [
                        'data' => [
                            ['price' => ['id' => 'price_pro_monthly']],
                        ],
                    ],
                ],
            ],
        ]);

        $result = $this->handler->handle('payload', 'sig');

        $this->assertSame('customer.subscription.deleted', $result['event']);
        $this->assertSame('canceled', $result['status']);
    }

    public function testHandleInvoicePaymentSucceeded(): void
    {
        $this->stripe->setNextWebhookEvent([
            'type' => 'invoice.payment_succeeded',
            'data' => [
                'object' => [
                    'customer' => 'cus_abc',
                    'subscription' => 'sub_123',
                    'amount_paid' => 1999,
                ],
            ],
        ]);

        $result = $this->handler->handle('payload', 'sig');

        $this->assertSame('invoice.payment_succeeded', $result['event']);
        $this->assertSame(1999, $result['amount_paid']);
    }

    public function testHandleInvoicePaymentFailed(): void
    {
        $this->stripe->setNextWebhookEvent([
            'type' => 'invoice.payment_failed',
            'data' => [
                'object' => [
                    'customer' => 'cus_abc',
                    'subscription' => 'sub_123',
                    'amount_due' => 1999,
                ],
            ],
        ]);

        $result = $this->handler->handle('payload', 'sig');

        $this->assertSame('invoice.payment_failed', $result['event']);
        $this->assertSame(1999, $result['amount_due']);
    }

    public function testHandleUnknownEventReturnsNull(): void
    {
        $this->stripe->setNextWebhookEvent([
            'type' => 'charge.succeeded',
            'data' => ['object' => []],
        ]);

        $result = $this->handler->handle('payload', 'sig');

        $this->assertNull($result);
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/billing/tests/Unit/WebhookHandlerTest.php`
Expected: FAIL — class not found.

- [ ] **Step 3: Write the implementation**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Billing;

final class WebhookHandler
{
    public function __construct(
        private readonly StripeClientInterface $stripe,
    ) {
    }

    /**
     * Process a Stripe webhook event.
     *
     * @return array<string, mixed>|null Structured event data, or null for unhandled events
     */
    public function handle(string $payload, string $signature): ?array
    {
        $event = $this->stripe->constructWebhookEvent($payload, $signature);
        $type = $event['type'] ?? '';
        $object = $event['data']['object'] ?? [];

        return match ($type) {
            'checkout.session.completed' => $this->handleCheckoutCompleted($type, $object),
            'customer.subscription.created',
            'customer.subscription.updated',
            'customer.subscription.deleted' => $this->handleSubscriptionEvent($type, $object),
            'invoice.payment_succeeded' => $this->handleInvoiceSucceeded($type, $object),
            'invoice.payment_failed' => $this->handleInvoiceFailed($type, $object),
            default => null,
        };
    }

    /**
     * @param array<string, mixed> $object
     * @return array<string, mixed>
     */
    private function handleCheckoutCompleted(string $type, array $object): array
    {
        return [
            'event' => $type,
            'customer_id' => $object['customer'] ?? null,
            'subscription_id' => $object['subscription'] ?? null,
            'metadata' => $object['metadata'] ?? [],
        ];
    }

    /**
     * @param array<string, mixed> $object
     * @return array<string, mixed>
     */
    private function handleSubscriptionEvent(string $type, array $object): array
    {
        $priceId = $object['items']['data'][0]['price']['id'] ?? null;

        return [
            'event' => $type,
            'subscription_id' => $object['id'] ?? null,
            'customer_id' => $object['customer'] ?? null,
            'status' => $object['status'] ?? null,
            'price_id' => $priceId,
        ];
    }

    /**
     * @param array<string, mixed> $object
     * @return array<string, mixed>
     */
    private function handleInvoiceSucceeded(string $type, array $object): array
    {
        return [
            'event' => $type,
            'customer_id' => $object['customer'] ?? null,
            'subscription_id' => $object['subscription'] ?? null,
            'amount_paid' => $object['amount_paid'] ?? 0,
        ];
    }

    /**
     * @param array<string, mixed> $object
     * @return array<string, mixed>
     */
    private function handleInvoiceFailed(string $type, array $object): array
    {
        return [
            'event' => $type,
            'customer_id' => $object['customer'] ?? null,
            'subscription_id' => $object['subscription'] ?? null,
            'amount_due' => $object['amount_due'] ?? 0,
        ];
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/billing/tests/Unit/WebhookHandlerTest.php`
Expected: All 7 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/billing/src/WebhookHandler.php packages/billing/tests/Unit/WebhookHandlerTest.php
git commit -m "feat(billing): add WebhookHandler for Stripe event processing"
```

---

### Task 7: Wire Service Provider

**Files:**
- Modify: `packages/billing/src/BillingServiceProvider.php`

- [ ] **Step 1: Update BillingServiceProvider**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Billing;

use Waaseyaa\Foundation\ServiceProvider\ServiceProvider;

final class BillingServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->singleton(BillingManager::class, fn() => new BillingManager(
            stripe: $this->resolve(StripeClientInterface::class),
            priceTierMap: $this->config['billing_price_tier_map'] ?? [],
            successUrl: $this->config['billing_success_url'] ?? '/',
            cancelUrl: $this->config['billing_cancel_url'] ?? '/',
            portalReturnUrl: $this->config['billing_portal_return_url'] ?? '/',
            foundingMemberCap: (int) ($this->config['billing_founding_member_cap'] ?? 100),
        ));

        $this->singleton(WebhookHandler::class, fn() => new WebhookHandler(
            stripe: $this->resolve(StripeClientInterface::class),
        ));
    }
}
```

- [ ] **Step 2: Run all tests**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/billing/tests/`
Expected: All tests PASS.

- [ ] **Step 3: Run CS Fixer**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/php-cs-fixer fix packages/billing/`

- [ ] **Step 4: Run tests again after CS Fixer**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/billing/tests/`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/billing/src/BillingServiceProvider.php
git commit -m "feat(billing): wire BillingServiceProvider with singleton registrations"
```

---

### Task 8: Deploy

**Files:**
- Modify: `.github/workflows/split.yml`

- [ ] **Step 1: Add to split workflow**

In `.github/workflows/split.yml`, add to the matrix. Place it alongside `auth` under `# Layer 2: Services`:

```yaml
          - { local: 'packages/billing', remote: 'billing' }
```

- [ ] **Step 2: Create split target repo**

Create `waaseyaa/billing` repository on GitHub (public, no README init).

- [ ] **Step 3: Commit and push**

```bash
git add .github/workflows/split.yml
git commit -m "ci: add waaseyaa/billing to monorepo split workflow"
git push
```

- [ ] **Step 4: Tag and push**

```bash
git tag --sort=-v:refname | head -1
# Increment alpha
git tag v0.1.0-alpha.<next>
git push origin v0.1.0-alpha.<next>
```

- [ ] **Step 5: Submit to Packagist**

Submit `https://github.com/waaseyaa/billing` at `https://packagist.org/packages/submit` after split workflow completes.

---

## Summary

| Task | What it builds | Tests |
|---|---|---|
| 1 | Package scaffold (composer.json, service provider, Stripe dep) | — |
| 2 | PlanTier enum (tier resolution, founding mapping, validation) | 8 |
| 3 | CheckoutSession + SubscriptionData value objects | 5 |
| 4 | StripeClientInterface + FakeStripeClient (testability) | — |
| 5 | BillingManager (checkout, portal, tier resolution, founding) | 15 |
| 6 | WebhookHandler (Stripe event processing) | 7 |
| 7 | Service provider wiring | — |
| 8 | Deploy (split workflow, packagist) | — |

**Total: 8 tasks, ~35 tests, 8 source files**

# Stripe Subscription Flow Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Wire up Stripe subscriptions with four tiers (Free/Pro/Business/Enterprise), hosted checkout, and plan-based limit enforcement across Laravel and Go.

**Architecture:** Laravel owns billing state via Cashier. The existing HMAC assertion headers are extended to include plan tier (`X-Plan-Tier`). Go enforces form/submission limits and feature gates statelessly based on the tier from the header.

**Tech Stack:** Laravel Cashier (Stripe), Stripe Checkout (hosted), Stripe Customer Portal, Go Echo middleware, GORM count queries, Vue 3 + Inertia pricing/billing pages.

**Design doc:** `docs/plans/2026-02-24-stripe-subscription-design.md`

---

### Task 1: Install Laravel Cashier and Run Migrations

**Files:**
- Modify: `goformx-laravel/composer.json`
- Create: `goformx-laravel/database/migrations/XXXX_XX_XX_XXXXXX_add_plan_override_to_users_table.php`
- Modify: `goformx-laravel/.env.example`
- Modify: `goformx-laravel/config/services.php`

**Step 1: Install Cashier via Composer**

Run:
```bash
cd goformx-laravel && ddev composer require laravel/cashier
```

**Step 2: Publish and run Cashier migrations**

Run:
```bash
ddev artisan vendor:publish --tag=cashier-migrations
ddev artisan migrate
```

This adds `stripe_id`, `pm_type`, `pm_last_four`, `trial_ends_at` columns to users and creates `subscriptions` + `subscription_items` tables.

**Step 3: Create migration for plan_override column**

Run:
```bash
ddev artisan make:migration add_plan_override_to_users_table
```

Edit the generated migration:

```php
public function up(): void
{
    Schema::table('users', function (Blueprint $table) {
        $table->string('plan_override', 20)->nullable()->after('trial_ends_at');
    });
}

public function down(): void
{
    Schema::table('users', function (Blueprint $table) {
        $table->dropColumn('plan_override');
    });
}
```

Run:
```bash
ddev artisan migrate
```

**Step 4: Add Stripe env vars to .env.example**

Add after the existing `GOFORMS_*` vars in `goformx-laravel/.env.example`:

```env
STRIPE_KEY=
STRIPE_SECRET=
STRIPE_WEBHOOK_SECRET=
STRIPE_PRO_MONTHLY_PRICE_ID=
STRIPE_PRO_ANNUAL_PRICE_ID=
STRIPE_BUSINESS_MONTHLY_PRICE_ID=
STRIPE_BUSINESS_ANNUAL_PRICE_ID=
```

**Step 5: Add Stripe price config to services.php**

Add to `goformx-laravel/config/services.php` after the `goforms` key:

```php
'stripe' => [
    'prices' => [
        'pro_monthly' => env('STRIPE_PRO_MONTHLY_PRICE_ID'),
        'pro_annual' => env('STRIPE_PRO_ANNUAL_PRICE_ID'),
        'business_monthly' => env('STRIPE_BUSINESS_MONTHLY_PRICE_ID'),
        'business_annual' => env('STRIPE_BUSINESS_ANNUAL_PRICE_ID'),
    ],
],
```

**Step 6: Commit**

```bash
git add -A
git commit -m "feat(goformx-laravel): install Laravel Cashier and add Stripe config"
```

---

### Task 2: User Model — Billable Trait and planTier()

**Files:**
- Modify: `goformx-laravel/app/Models/User.php`
- Test: `goformx-laravel/tests/Unit/Models/UserPlanTierTest.php`

**Step 1: Write the failing test**

Create `goformx-laravel/tests/Unit/Models/UserPlanTierTest.php`:

```php
<?php

use App\Models\User;
use Laravel\Cashier\Subscription;

beforeEach(function () {
    config([
        'services.stripe.prices.pro_monthly' => 'price_pro_monthly',
        'services.stripe.prices.pro_annual' => 'price_pro_annual',
        'services.stripe.prices.business_monthly' => 'price_business_monthly',
        'services.stripe.prices.business_annual' => 'price_business_annual',
    ]);
});

it('returns free for user with no subscription', function () {
    $user = User::factory()->create();

    expect($user->planTier())->toBe('free');
});

it('returns pro for user with pro monthly subscription', function () {
    $user = User::factory()->create();
    $user->subscriptions()->create([
        'type' => 'default',
        'stripe_id' => 'sub_test_pro',
        'stripe_status' => 'active',
        'stripe_price' => 'price_pro_monthly',
    ]);

    expect($user->planTier())->toBe('pro');
});

it('returns pro for user with pro annual subscription', function () {
    $user = User::factory()->create();
    $user->subscriptions()->create([
        'type' => 'default',
        'stripe_id' => 'sub_test_pro_annual',
        'stripe_status' => 'active',
        'stripe_price' => 'price_pro_annual',
    ]);

    expect($user->planTier())->toBe('pro');
});

it('returns business for user with business subscription', function () {
    $user = User::factory()->create();
    $user->subscriptions()->create([
        'type' => 'default',
        'stripe_id' => 'sub_test_biz',
        'stripe_status' => 'active',
        'stripe_price' => 'price_business_monthly',
    ]);

    expect($user->planTier())->toBe('business');
});

it('returns enterprise when plan_override is set', function () {
    $user = User::factory()->create(['plan_override' => 'enterprise']);

    expect($user->planTier())->toBe('enterprise');
});

it('returns plan_override over active subscription', function () {
    $user = User::factory()->create(['plan_override' => 'enterprise']);
    $user->subscriptions()->create([
        'type' => 'default',
        'stripe_id' => 'sub_test_pro',
        'stripe_status' => 'active',
        'stripe_price' => 'price_pro_monthly',
    ]);

    expect($user->planTier())->toBe('enterprise');
});

it('returns free for canceled subscription past grace period', function () {
    $user = User::factory()->create();
    $user->subscriptions()->create([
        'type' => 'default',
        'stripe_id' => 'sub_test_canceled',
        'stripe_status' => 'canceled',
        'stripe_price' => 'price_pro_monthly',
        'ends_at' => now()->subDay(),
    ]);

    expect($user->planTier())->toBe('free');
});
```

**Step 2: Run test to verify it fails**

Run:
```bash
ddev artisan test --filter=UserPlanTier
```
Expected: FAIL — `planTier()` method does not exist.

**Step 3: Implement User model changes**

Modify `goformx-laravel/app/Models/User.php`:

Add `use Laravel\Cashier\Billable;` import and add `Billable` to the traits list.

Add `plan_override` to `$fillable` array.

Add the `planTier()` method:

```php
public function planTier(): string
{
    if ($this->plan_override) {
        return $this->plan_override;
    }

    $prices = config('services.stripe.prices');

    $businessPrices = array_filter([
        $prices['business_monthly'] ?? null,
        $prices['business_annual'] ?? null,
    ]);

    if ($businessPrices && $this->subscribedToPrice($businessPrices)) {
        return 'business';
    }

    $proPrices = array_filter([
        $prices['pro_monthly'] ?? null,
        $prices['pro_annual'] ?? null,
    ]);

    if ($proPrices && $this->subscribedToPrice($proPrices)) {
        return 'pro';
    }

    return 'free';
}
```

Add cast for `plan_override`:
```php
'plan_override' => 'string',
```

**Step 4: Run test to verify it passes**

Run:
```bash
ddev artisan test --filter=UserPlanTier
```
Expected: PASS

**Step 5: Commit**

```bash
git add app/Models/User.php tests/Unit/Models/UserPlanTierTest.php
git commit -m "feat(goformx-laravel): add Billable trait and planTier() to User model"
```

---

### Task 3: GoFormsClient — Extend HMAC Signing with Plan Tier

**Files:**
- Modify: `goformx-laravel/app/Services/GoFormsClient.php` (lines 134-163)
- Test: `goformx-laravel/tests/Unit/Services/GoFormsClientTest.php`

**Step 1: Write the failing test**

The existing `GoFormsClientTest.php` needs a new test. Add to `goformx-laravel/tests/Unit/Services/GoFormsClientTest.php`:

```php
it('includes X-Plan-Tier header in signed requests', function () {
    Http::fake([
        '*/api/forms' => Http::response(['data' => ['forms' => []]], 200),
    ]);

    config([
        'services.goforms.url' => 'http://goforms:8090',
        'services.goforms.secret' => 'test-secret',
        'services.stripe.prices.pro_monthly' => 'price_pro_monthly',
        'services.stripe.prices.pro_annual' => 'price_pro_annual',
        'services.stripe.prices.business_monthly' => 'price_business_monthly',
        'services.stripe.prices.business_annual' => 'price_business_annual',
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

    config([
        'services.goforms.url' => 'http://goforms:8090',
        'services.goforms.secret' => 'test-secret',
        'services.stripe.prices.pro_monthly' => 'price_pro_monthly',
        'services.stripe.prices.pro_annual' => 'price_pro_annual',
        'services.stripe.prices.business_monthly' => 'price_business_monthly',
        'services.stripe.prices.business_annual' => 'price_business_annual',
    ]);

    $user = User::factory()->create();

    $client = GoFormsClient::fromConfig()->withUser($user);
    $client->listForms();

    Http::assertSent(function ($request) use ($user) {
        $userId = $request->header('X-User-Id')[0];
        $timestamp = $request->header('X-Timestamp')[0];
        $signature = $request->header('X-Signature')[0];
        $planTier = $request->header('X-Plan-Tier')[0];

        $expectedPayload = $userId.':'.$timestamp.':'.$planTier;
        $expectedSignature = hash_hmac('sha256', $expectedPayload, 'test-secret');

        return $signature === $expectedSignature;
    });
});
```

**Step 2: Run test to verify it fails**

Run:
```bash
ddev artisan test --filter=GoFormsClientTest
```
Expected: FAIL — signature doesn't include plan tier.

**Step 3: Modify GoFormsClient signRequest()**

In `goformx-laravel/app/Services/GoFormsClient.php`, update `signRequest()` (lines 151-163):

```php
private function signRequest(string $userId, string $secret): array
{
    $timestamp = now()->utc()->format('Y-m-d\TH:i:s\Z');
    $planTier = $this->user->planTier();

    $payload = $userId.':'.$timestamp.':'.$planTier;
    $signature = hash_hmac('sha256', $payload, $secret, false);

    return [
        'X-User-Id' => $userId,
        'X-Timestamp' => $timestamp,
        'X-Signature' => $signature,
        'X-Plan-Tier' => $planTier,
    ];
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
ddev artisan test --filter=GoFormsClientTest
```
Expected: PASS

**Step 5: Run full test suite to check for regressions**

Run:
```bash
ddev artisan test --compact
```
Expected: All existing tests pass (they use Http::fake so signature changes don't affect them).

**Step 6: Commit**

```bash
git add app/Services/GoFormsClient.php tests/Unit/Services/GoFormsClientTest.php
git commit -m "feat(goformx-laravel): include plan tier in HMAC assertion headers"
```

---

### Task 4: Go Assertion Middleware — Verify Plan Tier Header

**Files:**
- Modify: `goforms/internal/application/middleware/assertion/assertion.go` (lines 20-98)
- Modify: `goforms/internal/application/middleware/context/context.go`
- Test: `goforms/internal/application/middleware/assertion/assertion_test.go`

**Step 1: Write the failing tests**

Add to `goforms/internal/application/middleware/assertion/assertion_test.go`:

```go
func TestVerify_ValidSignatureWithPlanTier_SetsTierInContext(t *testing.T) {
	secret := "test-secret"
	userID := "user-123"
	timestamp := time.Now().UTC().Format(time.RFC3339)
	planTier := "pro"

	payload := userID + ":" + timestamp + ":" + planTier
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	signature := hex.EncodeToString(mac.Sum(nil))

	cfg := &appconfig.Config{}
	cfg.Security.Assertion.Secret = secret
	cfg.Security.Assertion.TimestampSkewSeconds = 60

	logger := setupTestLogger()
	m := NewMiddleware(cfg, logger)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-Id", userID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signature)
	req.Header.Set("X-Plan-Tier", planTier)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var capturedTier string
	handler := m.Verify()(func(c echo.Context) error {
		capturedTier, _ = c.Get("plan_tier").(string)
		return c.String(http.StatusOK, "ok")
	})

	err := handler(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "pro", capturedTier)
}

func TestVerify_MissingPlanTier_DefaultsToFree(t *testing.T) {
	secret := "test-secret"
	userID := "user-123"
	timestamp := time.Now().UTC().Format(time.RFC3339)

	// Sign with "free" as the default tier
	payload := userID + ":" + timestamp + ":" + "free"
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	signature := hex.EncodeToString(mac.Sum(nil))

	cfg := &appconfig.Config{}
	cfg.Security.Assertion.Secret = secret
	cfg.Security.Assertion.TimestampSkewSeconds = 60

	logger := setupTestLogger()
	m := NewMiddleware(cfg, logger)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-Id", userID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signature)
	// No X-Plan-Tier header
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var capturedTier string
	handler := m.Verify()(func(c echo.Context) error {
		capturedTier, _ = c.Get("plan_tier").(string)
		return c.String(http.StatusOK, "ok")
	})

	err := handler(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "free", capturedTier)
}
```

**Step 2: Run tests to verify they fail**

Run:
```bash
cd goforms && go test -v -run "TestVerify_.*PlanTier" ./internal/application/middleware/assertion/...
```
Expected: FAIL — signature mismatch because payload format changed.

**Step 3: Add plan tier constant and context helpers**

Add to `goforms/internal/application/middleware/assertion/assertion.go` constants (after line 22):

```go
headerPlanTier = "X-Plan-Tier"
defaultPlanTier = "free"
```

Add to `goforms/internal/application/middleware/context/context.go`:

```go
PlanTierKey Key = "plan_tier"
```

And add getter/setter functions:

```go
func GetPlanTier(c echo.Context) string {
	tier, ok := c.Get(string(PlanTierKey)).(string)
	if !ok || tier == "" {
		return "free"
	}
	return tier
}

func SetPlanTier(c echo.Context, tier string) {
	c.Set(string(PlanTierKey), tier)
}
```

**Step 4: Update verifyAssertionHeaders to include plan tier**

Modify `goforms/internal/application/middleware/assertion/assertion.go`:

Change the function signature (line 62):

```go
func verifyAssertionHeaders(headers http.Header, cfg appconfig.AssertionConfig) (userID, planTier, failureReason string) {
```

Inside the function, after extracting existing headers (around line 65), add:

```go
planTier = strings.TrimSpace(headers.Get(headerPlanTier))
if planTier == "" {
    planTier = defaultPlanTier
}
```

Change the payload construction (around line 88) from:

```go
payload := userID + ":" + timestamp
```

to:

```go
payload := userID + ":" + timestamp + ":" + planTier
```

Update the return statements to include `planTier`.

**Step 5: Update Verify() to set plan tier in context**

In the `Verify()` method (line 42-59), update the call and context setting:

```go
userID, planTier, failReason := verifyAssertionHeaders(c.Request().Header, cfg)
if failReason != "" {
    m.logFailure(c, failReason)
    return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
}

context.SetUserID(c, userID)
context.SetPlanTier(c, planTier)
```

**Step 6: Update existing tests to use new 3-part payload**

All existing tests in `assertion_test.go` need their HMAC payloads updated from `userID + ":" + timestamp` to `userID + ":" + timestamp + ":free"` (since they don't set X-Plan-Tier, it defaults to "free").

**Step 7: Run all assertion tests**

Run:
```bash
cd goforms && go test -v ./internal/application/middleware/assertion/...
```
Expected: ALL PASS

**Step 8: Commit**

```bash
git add internal/application/middleware/assertion/ internal/application/middleware/context/
git commit -m "feat(goforms): extend assertion middleware to verify plan tier header"
```

---

### Task 5: Go Plan Limits Config and Domain Errors

**Files:**
- Create: `goforms/internal/domain/common/plans/plans.go`
- Create: `goforms/internal/domain/common/plans/plans_test.go`
- Modify: `goforms/internal/domain/common/errors/errors.go`

**Step 1: Write the failing test**

Create `goforms/internal/domain/common/plans/plans_test.go`:

```go
package plans_test

import (
	"testing"

	"github.com/goformx/goforms/internal/domain/common/plans"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetLimits_KnownTiers(t *testing.T) {
	tests := []struct {
		tier          string
		maxForms      int
		maxSubmissions int
	}{
		{"free", 3, 100},
		{"pro", 25, 2500},
		{"business", 100, 25000},
		{"enterprise", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			limits, err := plans.GetLimits(tt.tier)
			require.NoError(t, err)
			assert.Equal(t, tt.maxForms, limits.MaxForms)
			assert.Equal(t, tt.maxSubmissions, limits.MaxSubmissionsPerMonth)
		})
	}
}

func TestGetLimits_UnknownTier_ReturnsError(t *testing.T) {
	_, err := plans.GetLimits("unknown")
	require.Error(t, err)
}

func TestIsUnlimited(t *testing.T) {
	limits, _ := plans.GetLimits("enterprise")
	assert.True(t, limits.IsUnlimited())

	limits, _ = plans.GetLimits("free")
	assert.False(t, limits.IsUnlimited())
}

func TestIsValidTier(t *testing.T) {
	assert.True(t, plans.IsValidTier("free"))
	assert.True(t, plans.IsValidTier("pro"))
	assert.True(t, plans.IsValidTier("business"))
	assert.True(t, plans.IsValidTier("enterprise"))
	assert.False(t, plans.IsValidTier("unknown"))
	assert.False(t, plans.IsValidTier(""))
}

func TestNextTier(t *testing.T) {
	assert.Equal(t, "pro", plans.NextTier("free"))
	assert.Equal(t, "business", plans.NextTier("pro"))
	assert.Equal(t, "enterprise", plans.NextTier("business"))
	assert.Equal(t, "", plans.NextTier("enterprise"))
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd goforms && go test -v ./internal/domain/common/plans/...
```
Expected: FAIL — package does not exist.

**Step 3: Implement plans package**

Create `goforms/internal/domain/common/plans/plans.go`:

```go
package plans

import "fmt"

const (
	TierFree       = "free"
	TierPro        = "pro"
	TierBusiness   = "business"
	TierEnterprise = "enterprise"

	freeForms              = 3
	freeSubmissionsPerMonth = 100
	proForms               = 25
	proSubmissionsPerMonth  = 2500
	bizForms               = 100
	bizSubmissionsPerMonth  = 25000
)

type Limits struct {
	MaxForms               int
	MaxSubmissionsPerMonth int
}

func (l Limits) IsUnlimited() bool {
	return l.MaxForms == 0 && l.MaxSubmissionsPerMonth == 0
}

var tierLimits = map[string]Limits{
	TierFree:       {MaxForms: freeForms, MaxSubmissionsPerMonth: freeSubmissionsPerMonth},
	TierPro:        {MaxForms: proForms, MaxSubmissionsPerMonth: proSubmissionsPerMonth},
	TierBusiness:   {MaxForms: bizForms, MaxSubmissionsPerMonth: bizSubmissionsPerMonth},
	TierEnterprise: {MaxForms: 0, MaxSubmissionsPerMonth: 0},
}

var tierOrder = []string{TierFree, TierPro, TierBusiness, TierEnterprise}

func GetLimits(tier string) (Limits, error) {
	limits, ok := tierLimits[tier]
	if !ok {
		return Limits{}, fmt.Errorf("unknown plan tier: %s", tier)
	}
	return limits, nil
}

func IsValidTier(tier string) bool {
	_, ok := tierLimits[tier]
	return ok
}

func NextTier(current string) string {
	for i, t := range tierOrder {
		if t == current && i+1 < len(tierOrder) {
			return tierOrder[i+1]
		}
	}
	return ""
}
```

**Step 4: Run tests to verify they pass**

Run:
```bash
cd goforms && go test -v ./internal/domain/common/plans/...
```
Expected: PASS

**Step 5: Add domain error codes for limit/feature errors**

Add to `goforms/internal/domain/common/errors/errors.go` in the error code constants:

```go
ErrCodeLimitExceeded     ErrorCode = "LIMIT_EXCEEDED"
ErrCodeFeatureNotAvailable ErrorCode = "FEATURE_NOT_AVAILABLE"
```

Add to the `HTTPStatus()` method switch:

```go
case ErrCodeLimitExceeded, ErrCodeFeatureNotAvailable:
    return http.StatusForbidden
```

Add convenience constructors:

```go
func NewLimitExceeded(limitType string, current, limit int, requiredTier string) *DomainError {
	return &DomainError{
		Code:    ErrCodeLimitExceeded,
		Message: fmt.Sprintf("Plan limit reached for %s", limitType),
		Context: map[string]any{
			"limit_type":    limitType,
			"current":       current,
			"limit":         limit,
			"required_tier": requiredTier,
		},
	}
}

func NewFeatureNotAvailable(feature, requiredTier string) *DomainError {
	return &DomainError{
		Code:    ErrCodeFeatureNotAvailable,
		Message: fmt.Sprintf("Feature %s requires %s plan or higher", feature, requiredTier),
		Context: map[string]any{
			"feature":       feature,
			"required_tier": requiredTier,
		},
	}
}
```

**Step 6: Run all domain tests**

Run:
```bash
cd goforms && go test -v ./internal/domain/common/...
```
Expected: PASS

**Step 7: Commit**

```bash
git add internal/domain/common/plans/ internal/domain/common/errors/
git commit -m "feat(goforms): add plan limits config and limit/feature domain errors"
```

---

### Task 6: Go Repository — Count Methods

**Files:**
- Modify: `goforms/internal/domain/form/repository.go`
- Modify: `goforms/internal/infrastructure/persistence/form_repository.go` (GORM implementation)
- Regenerate mocks
- Test: `goforms/internal/infrastructure/persistence/form_repository_test.go` (if integration tests exist)

**Step 1: Add methods to repository interface**

Add to `goforms/internal/domain/form/repository.go` Repository interface:

```go
CountFormsByUser(ctx context.Context, userID string) (int, error)
CountSubmissionsByUserMonth(ctx context.Context, userID string, year int, month int) (int, error)
```

**Step 2: Regenerate mocks**

Run:
```bash
cd goforms && task generate
```

**Step 3: Implement GORM repository methods**

Find the GORM repository implementation file. Add:

```go
func (r *FormRepository) CountFormsByUser(ctx context.Context, userID string) (int, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.Form{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count forms by user: %w", err)
	}
	return int(count), nil
}

func (r *FormRepository) CountSubmissionsByUserMonth(ctx context.Context, userID string, year int, month int) (int, error) {
	startOfMonth := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	endOfMonth := startOfMonth.AddDate(0, 1, 0)

	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.FormSubmission{}).
		Joins("JOIN forms ON forms.uuid = form_submissions.form_id").
		Where("forms.user_id = ? AND form_submissions.created_at >= ? AND form_submissions.created_at < ?",
			userID, startOfMonth, endOfMonth).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count submissions by user month: %w", err)
	}
	return int(count), nil
}
```

**Step 4: Run tests**

Run:
```bash
cd goforms && task test:backend
```
Expected: PASS (new methods are additive, existing tests unaffected).

**Step 5: Commit**

```bash
git add internal/domain/form/repository.go internal/infrastructure/persistence/ test/mocks/
git commit -m "feat(goforms): add CountFormsByUser and CountSubmissionsByUserMonth to repository"
```

---

### Task 7: Go Form Service — Limit Enforcement

**Files:**
- Modify: `goforms/internal/domain/form/service.go` (lines 57-76 CreateForm, lines 136-161 SubmitForm)
- Modify: `goforms/internal/domain/form/model/form.go` (add PlanTier column)
- Test: `goforms/internal/domain/form/service_test.go`

**Step 1: Write the failing tests**

Create or extend `goforms/internal/domain/form/service_test.go`:

```go
func TestCreateForm_ExceedsFreeTierLimit_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockRepository(ctrl)
	mockEventBus := eventmocks.NewMockEventBus(ctrl)
	logger := setupTestLogger()

	svc := NewFormService(mockRepo, mockEventBus, logger)

	mockRepo.EXPECT().CountFormsByUser(gomock.Any(), "user-123").Return(3, nil)

	form := model.NewForm("user-123", "Test Form", "", model.JSON{})
	err := svc.CreateForm(context.Background(), form, "free")

	require.Error(t, err)
	var domainErr *errors.DomainError
	require.ErrorAs(t, err, &domainErr)
	assert.Equal(t, errors.ErrCodeLimitExceeded, domainErr.Code)
}

func TestCreateForm_UnderFreeTierLimit_Succeeds(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockRepository(ctrl)
	mockEventBus := eventmocks.NewMockEventBus(ctrl)
	logger := setupTestLogger()

	svc := NewFormService(mockRepo, mockEventBus, logger)

	mockRepo.EXPECT().CountFormsByUser(gomock.Any(), "user-123").Return(2, nil)
	mockRepo.EXPECT().CreateForm(gomock.Any(), gomock.Any()).Return(nil)
	mockEventBus.EXPECT().Publish(gomock.Any(), gomock.Any()).Return(nil)

	form := model.NewForm("user-123", "Test Form", "", model.JSON{})
	err := svc.CreateForm(context.Background(), form, "free")

	require.NoError(t, err)
}

func TestCreateForm_EnterpriseTier_NoLimit(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockRepository(ctrl)
	mockEventBus := eventmocks.NewMockEventBus(ctrl)
	logger := setupTestLogger()

	svc := NewFormService(mockRepo, mockEventBus, logger)

	// Enterprise has no limits — CountFormsByUser should not be called
	mockRepo.EXPECT().CreateForm(gomock.Any(), gomock.Any()).Return(nil)
	mockEventBus.EXPECT().Publish(gomock.Any(), gomock.Any()).Return(nil)

	form := model.NewForm("user-123", "Test Form", "", model.JSON{})
	err := svc.CreateForm(context.Background(), form, "enterprise")

	require.NoError(t, err)
}
```

**Step 2: Run tests to verify they fail**

Run:
```bash
cd goforms && go test -v -run "TestCreateForm_.*Tier" ./internal/domain/form/...
```
Expected: FAIL — CreateForm doesn't accept a planTier parameter.

**Step 3: Update Service interface and implementation**

Update `CreateForm` in the `Service` interface (`service.go:27-38`) to accept plan tier:

```go
CreateForm(ctx context.Context, form *model.Form, planTier string) error
```

Update the `formService.CreateForm` implementation to check limits before creating:

```go
func (s *formService) CreateForm(ctx context.Context, form *model.Form, planTier string) error {
	if err := form.Validate(); err != nil {
		return fmt.Errorf("form validation failed: %w", err)
	}

	limits, err := plans.GetLimits(planTier)
	if err != nil {
		return fmt.Errorf("get plan limits: %w", err)
	}

	if !limits.IsUnlimited() {
		count, err := s.repository.CountFormsByUser(ctx, form.UserID)
		if err != nil {
			return fmt.Errorf("count user forms: %w", err)
		}
		if count >= limits.MaxForms {
			return errors.NewLimitExceeded("max_forms", count, limits.MaxForms, plans.NextTier(planTier))
		}
	}

	if form.ID == "" {
		form.ID = uuid.New().String()
	}

	if err := s.repository.CreateForm(ctx, form); err != nil {
		return fmt.Errorf("failed to create form: %w", err)
	}

	if err := s.eventBus.Publish(ctx, formevents.NewFormCreatedEvent(form)); err != nil {
		s.logger.Error("failed to publish form created event", "error", err)
	}

	return nil
}
```

**Step 4: Update callers of CreateForm to pass plan tier**

Update `goforms/internal/application/handlers/web/form_service.go` `CreateForm()` method to accept and pass plan tier. The handler (`form_api.go:handleCreateForm`) reads plan tier from context and passes it through.

In `form_api.go`, update `handleCreateForm()`:

```go
planTier := context.GetPlanTier(c)
form, err := h.FormServiceHandler.CreateForm(c.Request().Context(), userID, req, planTier)
```

**Step 5: Add PlanTier column to Form model**

Add to `goforms/internal/domain/form/model/form.go` Form struct:

```go
PlanTier string `gorm:"size:20;not null;default:'free'" json:"plan_tier"`
```

Update `NewForm()` to accept and set plan tier. Update the handler to pass it.

**Step 6: Create Go migration for plan_tier column on forms**

Create migration file to add `plan_tier` column to `forms` table.

**Step 7: Run tests**

Run:
```bash
cd goforms && task test:backend
```
Expected: PASS

**Step 8: Commit**

```bash
git add internal/domain/form/ internal/application/handlers/web/
git commit -m "feat(goforms): enforce form creation limits based on plan tier"
```

---

### Task 8: Go Usage Endpoints

**Files:**
- Modify: `goforms/internal/application/handlers/web/form_api.go` (add routes)
- Test: add usage endpoint tests

**Step 1: Write the failing test**

```go
func TestHandleFormsCount_ReturnsCount(t *testing.T) {
	// Setup Echo, mock repository returning count of 5
	// Send GET /api/usage/forms-count with valid assertion headers
	// Assert response: {"success": true, "data": {"count": 5}}
}

func TestHandleSubmissionsCount_ReturnsMonthlyCount(t *testing.T) {
	// Setup Echo, mock repository returning count of 150
	// Send GET /api/usage/submissions-count?month=2026-02 with valid assertion headers
	// Assert response: {"success": true, "data": {"count": 150, "month": "2026-02"}}
}
```

**Step 2: Run tests to verify they fail**

Run:
```bash
cd goforms && go test -v -run "TestHandle.*Count" ./internal/application/handlers/web/...
```
Expected: FAIL — handlers don't exist.

**Step 3: Implement usage handlers**

Add to `form_api.go`:

```go
func (h *FormAPIHandler) handleFormsCount(c echo.Context) error {
	userID, ok := c.Get("user_id").(string)
	if !ok {
		return h.HandleForbidden(c, "User not authenticated")
	}

	count, err := h.FormService.CountFormsByUser(c.Request().Context(), userID)
	if err != nil {
		return h.HandleError(c, err, "Failed to count forms")
	}

	return c.JSON(http.StatusOK, response.APIResponse{
		Success: true,
		Data:    map[string]any{"count": count},
	})
}

func (h *FormAPIHandler) handleSubmissionsCount(c echo.Context) error {
	userID, ok := c.Get("user_id").(string)
	if !ok {
		return h.HandleForbidden(c, "User not authenticated")
	}

	monthStr := c.QueryParam("month")
	year, month, err := parseYearMonth(monthStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.APIResponse{
			Success: false,
			Message: "Invalid month format. Use YYYY-MM.",
		})
	}

	count, err := h.FormService.CountSubmissionsByUserMonth(c.Request().Context(), userID, year, month)
	if err != nil {
		return h.HandleError(c, err, "Failed to count submissions")
	}

	return c.JSON(http.StatusOK, response.APIResponse{
		Success: true,
		Data:    map[string]any{"count": count, "month": monthStr},
	})
}
```

**Step 4: Register routes**

In `RegisterLaravelRoutes()`, add:

```go
formsLaravel.GET("/usage/forms-count", h.handleFormsCount)
formsLaravel.GET("/usage/submissions-count", h.handleSubmissionsCount)
```

Note: These should be registered on a new `/api/usage` group, or under the existing Laravel group. Choose a path that doesn't conflict with form `:id` parameter matching.

**Step 5: Run tests**

Run:
```bash
cd goforms && task test:backend
```
Expected: PASS

**Step 6: Commit**

```bash
git add internal/application/handlers/web/
git commit -m "feat(goforms): add usage count endpoints for forms and submissions"
```

---

### Task 9: Go Feature Gating — Schema Validation

**Files:**
- Create: `goforms/internal/domain/common/plans/features.go`
- Create: `goforms/internal/domain/common/plans/features_test.go`
- Modify: `goforms/internal/domain/form/service.go` (CreateForm, UpdateForm)

**Step 1: Write the failing test**

Create `goforms/internal/domain/common/plans/features_test.go`:

```go
package plans_test

import (
	"testing"

	"github.com/goformx/goforms/internal/domain/common/plans"
	"github.com/stretchr/testify/assert"
)

func TestValidateSchemaFeatures_FileUploadOnFree_ReturnsError(t *testing.T) {
	schema := map[string]any{
		"components": []any{
			map[string]any{"type": "file", "key": "upload"},
		},
	}
	err := plans.ValidateSchemaFeatures(schema, "free")
	assert.Error(t, err)
}

func TestValidateSchemaFeatures_FileUploadOnPro_Succeeds(t *testing.T) {
	schema := map[string]any{
		"components": []any{
			map[string]any{"type": "file", "key": "upload"},
		},
	}
	err := plans.ValidateSchemaFeatures(schema, "pro")
	assert.NoError(t, err)
}

func TestValidateSchemaFeatures_BasicFieldsOnFree_Succeeds(t *testing.T) {
	schema := map[string]any{
		"components": []any{
			map[string]any{"type": "textfield", "key": "name"},
			map[string]any{"type": "email", "key": "email"},
			map[string]any{"type": "button", "key": "submit"},
		},
	}
	err := plans.ValidateSchemaFeatures(schema, "free")
	assert.NoError(t, err)
}

func TestValidateSchemaFeatures_Enterprise_AllowsEverything(t *testing.T) {
	schema := map[string]any{
		"components": []any{
			map[string]any{"type": "file", "key": "upload"},
		},
	}
	err := plans.ValidateSchemaFeatures(schema, "enterprise")
	assert.NoError(t, err)
}
```

**Step 2: Run tests to verify they fail**

Run:
```bash
cd goforms && go test -v -run "TestValidateSchemaFeatures" ./internal/domain/common/plans/...
```
Expected: FAIL — function doesn't exist.

**Step 3: Implement feature gating**

Create `goforms/internal/domain/common/plans/features.go`:

```go
package plans

import (
	"github.com/goformx/goforms/internal/domain/common/errors"
)

// featureRequirements maps Form.io component types to the minimum tier required.
var featureRequirements = map[string]string{
	"file":      TierPro,
	"signature": TierPro,
}

var tierRank = map[string]int{
	TierFree:       0,
	TierPro:        1,
	TierBusiness:   2,
	TierEnterprise: 3,
}

func hasTierAccess(userTier, requiredTier string) bool {
	return tierRank[userTier] >= tierRank[requiredTier]
}

// ValidateSchemaFeatures checks if a form schema uses any gated component types.
func ValidateSchemaFeatures(schema map[string]any, planTier string) error {
	components, ok := schema["components"].([]any)
	if !ok {
		return nil
	}

	return validateComponents(components, planTier)
}

func validateComponents(components []any, planTier string) error {
	for _, comp := range components {
		compMap, ok := comp.(map[string]any)
		if !ok {
			continue
		}

		compType, _ := compMap["type"].(string)
		if requiredTier, gated := featureRequirements[compType]; gated {
			if !hasTierAccess(planTier, requiredTier) {
				return errors.NewFeatureNotAvailable(compType, requiredTier)
			}
		}

		// Check nested components recursively
		if nested, ok := compMap["components"].([]any); ok {
			if err := validateComponents(nested, planTier); err != nil {
				return err
			}
		}
		if columns, ok := compMap["columns"].([]any); ok {
			for _, col := range columns {
				if colMap, ok := col.(map[string]any); ok {
					if nested, ok := colMap["components"].([]any); ok {
						if err := validateComponents(nested, planTier); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}
```

**Step 4: Run tests to verify they pass**

Run:
```bash
cd goforms && go test -v -run "TestValidateSchemaFeatures" ./internal/domain/common/plans/...
```
Expected: PASS

**Step 5: Wire feature validation into form service CreateForm/UpdateForm**

In `service.go` `CreateForm()`, after limit check and before repository call:

```go
if form.Schema != nil {
    if err := plans.ValidateSchemaFeatures(form.Schema, planTier); err != nil {
        return err
    }
}
```

Same for `UpdateForm()`.

**Step 6: Run full test suite**

Run:
```bash
cd goforms && task test:backend
```
Expected: PASS

**Step 7: Commit**

```bash
git add internal/domain/common/plans/ internal/domain/form/
git commit -m "feat(goforms): add feature gating for form schema components by plan tier"
```

---

### Task 10: Laravel Billing Controllers and Routes

**Files:**
- Create: `goformx-laravel/app/Http/Controllers/PricingController.php`
- Create: `goformx-laravel/app/Http/Controllers/BillingController.php`
- Modify: `goformx-laravel/routes/web.php`
- Test: `goformx-laravel/tests/Feature/PricingPageTest.php`
- Test: `goformx-laravel/tests/Feature/BillingControllerTest.php`

**Step 1: Write the failing tests**

Create `goformx-laravel/tests/Feature/PricingPageTest.php`:

```php
<?php

use App\Models\User;

it('renders pricing page for guests', function () {
    $this->get(route('pricing'))
        ->assertOk()
        ->assertInertia(fn ($page) => $page->component('Pricing'));
});

it('renders pricing page for authenticated users with plan tier', function () {
    $user = User::factory()->create();

    $this->actingAs($user)
        ->get(route('pricing'))
        ->assertOk()
        ->assertInertia(fn ($page) => $page
            ->component('Pricing')
            ->has('currentTier')
        );
});
```

Create `goformx-laravel/tests/Feature/BillingControllerTest.php`:

```php
<?php

use App\Models\User;

it('redirects unauthenticated users from billing page', function () {
    $this->get(route('billing.index'))
        ->assertRedirect(route('login'));
});

it('renders billing page for authenticated users', function () {
    Http::fake([
        '*/api/usage/*' => Http::response(['success' => true, 'data' => ['count' => 5]], 200),
    ]);

    $user = User::factory()->create();

    $this->actingAs($user)
        ->get(route('billing.index'))
        ->assertOk()
        ->assertInertia(fn ($page) => $page
            ->component('Billing/Index')
            ->has('currentTier')
            ->has('usage')
        );
});
```

**Step 2: Run tests to verify they fail**

Run:
```bash
ddev artisan test --filter=PricingPage --filter=BillingController
```
Expected: FAIL — routes and controllers don't exist.

**Step 3: Create PricingController**

Create `goformx-laravel/app/Http/Controllers/PricingController.php`:

```php
<?php

namespace App\Http\Controllers;

use Illuminate\Http\Request;
use Inertia\Inertia;
use Inertia\Response;

class PricingController extends Controller
{
    public function __invoke(Request $request): Response
    {
        return Inertia::render('Pricing', [
            'currentTier' => $request->user()?->planTier() ?? 'free',
            'prices' => config('services.stripe.prices'),
        ]);
    }
}
```

**Step 4: Create BillingController**

Create `goformx-laravel/app/Http/Controllers/BillingController.php`:

```php
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
```

**Step 5: Add GoFormsClient usage methods**

Add to `goformx-laravel/app/Services/GoFormsClient.php`:

```php
public function getFormsCount(): int
{
    $response = $this->request()->get('/api/usage/forms-count');
    return $response->json('data.count', 0);
}

public function getSubmissionsCount(string $month): int
{
    $response = $this->request()->get('/api/usage/submissions-count', ['month' => $month]);
    return $response->json('data.count', 0);
}
```

**Step 6: Add routes**

Add to `goformx-laravel/routes/web.php`:

```php
// Public
Route::get('/pricing', PricingController::class)->name('pricing');

// Authenticated
Route::middleware(['auth', 'verified'])->group(function () {
    Route::get('/billing', [BillingController::class, 'index'])->name('billing.index');
    Route::post('/billing/checkout', [BillingController::class, 'checkout'])->name('billing.checkout');
    Route::get('/billing/portal', [BillingController::class, 'portal'])->name('billing.portal');
});
```

Add Cashier webhook route. In `routes/web.php` or create `routes/cashier.php`:

```php
// In routes/web.php or bootstrap/app.php:
// Cashier registers its own /stripe/webhook route automatically when published.
```

Publish Cashier routes if needed, or register manually. Cashier's webhook controller handles the `POST /stripe/webhook` route. Ensure the route is excluded from CSRF verification in `bootstrap/app.php`.

**Step 7: Run tests**

Run:
```bash
ddev artisan test --filter=PricingPage --filter=BillingController
```
Expected: PASS

**Step 8: Commit**

```bash
git add app/Http/Controllers/PricingController.php app/Http/Controllers/BillingController.php \
  app/Services/GoFormsClient.php routes/web.php \
  tests/Feature/PricingPageTest.php tests/Feature/BillingControllerTest.php
git commit -m "feat(goformx-laravel): add pricing and billing controllers with routes"
```

---

### Task 11: Laravel Pricing Page (Vue)

**Files:**
- Create: `goformx-laravel/resources/js/pages/Pricing.vue`
- Create: `goformx-laravel/resources/js/types/billing.ts`

**Step 1: Create billing types**

Create `goformx-laravel/resources/js/types/billing.ts`:

```typescript
export interface PlanTier {
    name: string;
    tier: string;
    description: string;
    monthlyPrice: number | null;
    annualPrice: number | null;
    monthlyPriceId: string | null;
    annualPriceId: string | null;
    limits: {
        forms: number | string;
        submissions: number | string;
    };
    features: string[];
    highlighted?: boolean;
    cta: string;
    ctaVariant: 'default' | 'outline' | 'secondary';
}

export interface BillingUsage {
    forms: number;
    submissions: number;
}

export interface SubscriptionInfo {
    stripe_status: string | null;
    ends_at: string | null;
    trial_ends_at: string | null;
}
```

**Step 2: Create Pricing page component**

Create `goformx-laravel/resources/js/pages/Pricing.vue`. This is a public page with:

- Four tier cards (Free, Pro, Business, Enterprise) in a responsive grid
- Monthly/annual toggle switch
- Feature comparison for each tier
- CTA buttons: "Get Started" (free), "Subscribe" (Pro/Business), "Contact Us" (Enterprise)
- Subscribe buttons post to `/billing/checkout` with `price_id`
- If user is logged in and already on a tier, show "Current Plan" badge
- Use existing shadcn-vue Card, Button, Badge components

The component receives `currentTier` and `prices` as Inertia props.

Use the `AppHeaderLayout` for authenticated users, a minimal public layout for guests.

**Step 3: Generate Wayfinder routes**

Run:
```bash
ddev artisan wayfinder:generate
```

**Step 4: Verify page renders**

Run:
```bash
ddev exec npm run build
ddev artisan test --filter=PricingPage
```
Expected: PASS

**Step 5: Commit**

```bash
git add resources/js/pages/Pricing.vue resources/js/types/billing.ts
git commit -m "feat(goformx-laravel): add pricing page with tier comparison"
```

---

### Task 12: Laravel Billing Dashboard (Vue)

**Files:**
- Create: `goformx-laravel/resources/js/pages/Billing/Index.vue`

**Step 1: Create Billing dashboard page**

Create `goformx-laravel/resources/js/pages/Billing/Index.vue`. This authenticated page shows:

- Current plan name + status badge (active, trialing, past_due, canceled)
- Current period end date (if subscribed)
- Usage cards: "Forms: 12 / 25" and "Submissions: 1,847 / 2,500" with progress bars
- "Change Plan" button linking to pricing page
- "Manage Billing" button that navigates to `/billing/portal`
- Use `AppSidebarLayout` consistent with other authenticated pages (Dashboard, Forms)
- Use existing shadcn-vue Card, Button, Badge, Progress components

The component receives `currentTier`, `subscription`, `usage`, and `prices` as Inertia props.

**Step 2: Add billing link to navigation**

Modify `goformx-laravel/resources/js/components/NavMain.vue` (or wherever nav items are defined) to add a "Billing" link pointing to the billing route.

**Step 3: Verify page renders**

Run:
```bash
ddev exec npm run build
ddev artisan test --filter=BillingController
```
Expected: PASS

**Step 4: Commit**

```bash
git add resources/js/pages/Billing/ resources/js/components/
git commit -m "feat(goformx-laravel): add billing dashboard with usage display"
```

---

### Task 13: Laravel Error Handling for 403 Limit/Feature Errors

**Files:**
- Modify: `goformx-laravel/app/Http/Controllers/FormController.php` (lines 231-274 error handling)
- Test: `goformx-laravel/tests/Feature/FormControllerTest.php`

**Step 1: Write the failing test**

Add to `goformx-laravel/tests/Feature/FormControllerTest.php`:

```php
it('shows upgrade prompt when form creation hits plan limit', function () {
    Http::fake([
        '*/api/forms' => Http::response([
            'success' => false,
            'error' => 'limit_exceeded',
            'message' => 'Free plan allows 3 forms. Upgrade to Pro for up to 25.',
            'data' => [
                'limit_type' => 'max_forms',
                'current' => 3,
                'limit' => 3,
                'required_tier' => 'pro',
            ],
        ], 403),
    ]);

    $user = User::factory()->create();

    $this->actingAs($user)
        ->post(route('forms.store'), ['title' => 'New Form'])
        ->assertRedirect()
        ->assertSessionHas('error')
        ->assertSessionHas('upgrade_tier', 'pro');
});

it('shows upgrade prompt when feature is not available', function () {
    Http::fake([
        '*/api/forms/*' => Http::response([
            'success' => false,
            'error' => 'feature_not_available',
            'message' => 'Feature file requires pro plan or higher',
            'data' => [
                'feature' => 'file',
                'required_tier' => 'pro',
            ],
        ], 403),
    ]);

    $user = User::factory()->create();

    $this->actingAs($user)
        ->put(route('forms.update', 'some-id'), ['title' => 'Form', 'schema' => []])
        ->assertRedirect()
        ->assertSessionHas('error')
        ->assertSessionHas('upgrade_tier', 'pro');
});
```

**Step 2: Run test to verify it fails**

Run:
```bash
ddev artisan test --filter="upgrade prompt"
```
Expected: FAIL — 403 not handled specially.

**Step 3: Add 403 handling to FormController**

In `FormController.php`, update the `handleGoError` method to handle 403 responses:

```php
if ($status === 403) {
    $body = $e->response->json();
    $requiredTier = $body['data']['required_tier'] ?? null;

    return redirect()->back()
        ->with('error', $body['message'] ?? 'Plan limit reached. Please upgrade.')
        ->with('upgrade_tier', $requiredTier)
        ->withInput();
}
```

**Step 4: Run tests**

Run:
```bash
ddev artisan test --filter=FormController
```
Expected: PASS

**Step 5: Commit**

```bash
git add app/Http/Controllers/FormController.php tests/Feature/FormControllerTest.php
git commit -m "feat(goformx-laravel): handle 403 limit/feature errors with upgrade prompts"
```

---

### Task 14: Integration Test — End-to-End Subscription Flow

**Files:**
- Create: `goformx-laravel/tests/Feature/SubscriptionFlowTest.php`

**Step 1: Write the integration test**

Create `goformx-laravel/tests/Feature/SubscriptionFlowTest.php`:

```php
<?php

use App\Models\User;

it('free user sees correct tier on pricing page', function () {
    $user = User::factory()->create();

    $this->actingAs($user)
        ->get(route('pricing'))
        ->assertOk()
        ->assertInertia(fn ($page) => $page
            ->component('Pricing')
            ->where('currentTier', 'free')
        );
});

it('free user can initiate checkout for pro plan', function () {
    config(['services.stripe.prices.pro_monthly' => 'price_pro_test']);

    $user = User::factory()->create();

    // Cashier's checkout method calls Stripe API
    // In tests, we mock the Stripe client or skip this test for CI
    $this->actingAs($user)
        ->post(route('billing.checkout'), ['price_id' => 'price_pro_test'])
        ->assertRedirect(); // Redirects to Stripe checkout
})->skip(! env('STRIPE_SECRET'), 'Stripe credentials required');

it('subscribed user has correct plan tier in assertion headers', function () {
    Http::fake([
        '*/api/forms' => Http::response(['data' => ['forms' => []]], 200),
    ]);

    config([
        'services.goforms.url' => 'http://goforms:8090',
        'services.goforms.secret' => 'test-secret',
        'services.stripe.prices.pro_monthly' => 'price_pro_monthly',
        'services.stripe.prices.pro_annual' => 'price_pro_annual',
        'services.stripe.prices.business_monthly' => 'price_business_monthly',
        'services.stripe.prices.business_annual' => 'price_business_annual',
    ]);

    $user = User::factory()->create();
    $user->subscriptions()->create([
        'type' => 'default',
        'stripe_id' => 'sub_test',
        'stripe_status' => 'active',
        'stripe_price' => 'price_pro_monthly',
    ]);

    $this->actingAs($user)
        ->get(route('forms.index'));

    Http::assertSent(function ($request) {
        return $request->hasHeader('X-Plan-Tier', 'pro');
    });
});

it('enterprise override takes priority over subscription', function () {
    Http::fake([
        '*/api/forms' => Http::response(['data' => ['forms' => []]], 200),
    ]);

    config([
        'services.goforms.url' => 'http://goforms:8090',
        'services.goforms.secret' => 'test-secret',
        'services.stripe.prices.pro_monthly' => 'price_pro_monthly',
        'services.stripe.prices.pro_annual' => 'price_pro_annual',
        'services.stripe.prices.business_monthly' => 'price_business_monthly',
        'services.stripe.prices.business_annual' => 'price_business_annual',
    ]);

    $user = User::factory()->create(['plan_override' => 'enterprise']);
    $user->subscriptions()->create([
        'type' => 'default',
        'stripe_id' => 'sub_test',
        'stripe_status' => 'active',
        'stripe_price' => 'price_pro_monthly',
    ]);

    $this->actingAs($user)
        ->get(route('forms.index'));

    Http::assertSent(function ($request) {
        return $request->hasHeader('X-Plan-Tier', 'enterprise');
    });
});
```

**Step 2: Run tests**

Run:
```bash
ddev artisan test --filter=SubscriptionFlow
```
Expected: PASS

**Step 3: Run full test suites for both services**

Run:
```bash
cd /home/jones/dev/goformx && task test
```
Expected: All tests pass in both Go and Laravel.

**Step 4: Commit**

```bash
git add tests/Feature/SubscriptionFlowTest.php
git commit -m "test(goformx-laravel): add end-to-end subscription flow integration tests"
```

---

### Task 15: Final Lint, Format, and Verify

**Step 1: Lint and format Go code**

Run:
```bash
cd goforms && task lint
```
Fix any issues.

**Step 2: Lint and format Laravel code**

Run:
```bash
cd goformx-laravel && ddev exec vendor/bin/pint --dirty --format agent
ddev exec npm run lint
ddev exec npm run format
```
Fix any issues.

**Step 3: Build frontend**

Run:
```bash
cd goformx-laravel && ddev exec npm run build
```
Expected: Build succeeds with no errors.

**Step 4: Run full test suite one final time**

Run:
```bash
cd /home/jones/dev/goformx && task test
```
Expected: ALL PASS

**Step 5: Commit any lint/format fixes**

```bash
git add -A
git commit -m "chore: lint and format after Stripe subscription implementation"
```

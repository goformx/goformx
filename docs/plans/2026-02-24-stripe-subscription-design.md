# Stripe Subscription Flow Design

**Date**: 2026-02-24
**Status**: Approved

## Overview

Wire up Stripe subscriptions for GoFormX with four tiers (Free, Pro, Business, Enterprise), hosted checkout, and plan-based limit enforcement across the Laravel frontend and Go backend.

## Plan Tiers & Limits

| Tier | Forms | Submissions/mo | Billing |
|------|-------|----------------|---------|
| Free | 3 | 100 | None |
| Pro | 25 | 2,500 | Monthly + Annual (20% discount) |
| Business | 100 | 25,000 | Monthly + Annual (20% discount) |
| Enterprise | Unlimited | Unlimited | Custom (manual) |

## Feature Gating by Tier

| Feature | Free | Pro | Business | Enterprise |
|---------|------|-----|----------|------------|
| File uploads | - | Y | Y | Y |
| Conditional logic | - | Y | Y | Y |
| Multi-page forms | - | Y | Y | Y |
| Custom themes/CSS | - | - | Y | Y |
| Submission encryption | - | - | Y | Y |
| API keys | - | Y | Y | Y |
| Webhooks | - | - | Y | Y |
| Custom branding (remove badge) | - | - | Y | Y |
| Teams/collaboration | Deferred to future project |

## Architecture: Assertion Header Extension

Extends the existing HMAC assertion headers to include plan tier. Laravel owns all billing state via Cashier; Go enforces limits statelessly.

### Current HMAC Payload

```
payload = userID + ":" + timestamp
headers = X-User-Id, X-Timestamp, X-Signature
```

### Extended HMAC Payload

```
payload = userID + ":" + timestamp + ":" + planTier
headers = X-User-Id, X-Timestamp, X-Signature, X-Plan-Tier
```

Valid tier values: `free`, `pro`, `business`, `enterprise`.

Go defaults to `free` if `X-Plan-Tier` is missing (backward compatibility).

### Data Flow

```
Browser -> Laravel (Cashier/Stripe)
  |
  +- Stripe webhooks -> Laravel
  |   +- Updates subscription state in MariaDB
  |
  +- API calls -> Go (assertion headers now include plan tier)
      +- Go enforces form/submission limits based on tier from header
```

## Laravel: Cashier + Stripe Integration

### Package

`laravel/cashier` — adds subscriptions table, customer columns to users, Stripe webhook handling.

### User Model Changes

- Add `Billable` trait
- Cashier migration adds `stripe_id`, `pm_type`, `pm_last_four`, `trial_ends_at` to users
- Add nullable `plan_override` column for Enterprise users (set manually)

### Plan Tier Resolution

```php
public function planTier(): string
{
    if ($this->plan_override) {
        return $this->plan_override;
    }
    if ($this->subscribedToPrice([prices('business_monthly'), prices('business_annual')])) {
        return 'business';
    }
    if ($this->subscribedToPrice([prices('pro_monthly'), prices('pro_annual')])) {
        return 'pro';
    }
    return 'free';
}
```

### GoFormsClient Signing

`signRequest()` updated to include `$this->user->planTier()` in both the payload and as an `X-Plan-Tier` header.

### Stripe Configuration

Products/prices configured in Stripe Dashboard, referenced by price ID in `config/services.php`:
- `pro_monthly`, `pro_annual`
- `business_monthly`, `business_annual`

## Go: Limit Enforcement

Enforced in the **service layer** (not handler or repository).

### Plan Limits Config

```go
var PlanLimits = map[string]Limits{
    "free":       {MaxForms: 3,   MaxSubmissionsPerMonth: 100},
    "pro":        {MaxForms: 25,  MaxSubmissionsPerMonth: 2500},
    "business":   {MaxForms: 100, MaxSubmissionsPerMonth: 25000},
    "enterprise": {MaxForms: 0,   MaxSubmissionsPerMonth: 0}, // 0 = unlimited
}
```

### New Repository Methods

- `CountFormsByUser(ctx, userID) (int, error)`
- `CountSubmissionsByUserMonth(ctx, userID, month) (int, error)`

### Form Creation Check

1. Read plan tier from context
2. Count user's existing forms
3. Compare against `PlanLimits[tier].MaxForms`
4. Return `ErrLimitExceeded` if over

### Submission Check

1. For authenticated submissions: tier from context
2. For public submissions (`POST /forms/:id/submit`): look up form owner's tier from a `plan_tier` column on the forms table (denormalized, updated on each authenticated API call)
3. Count submissions for owner this month
4. Compare against limit, return `ErrLimitExceeded` if over

### Feature Gating

On `CreateForm` and `UpdateForm`, inspect form schema JSON for component types requiring higher tiers (e.g. `type: "file"` requires Pro+). Return `ErrFeatureNotAvailable` with required tier.

## Laravel: Routes & UI

### New Routes

```
GET  /pricing              PricingController@index      (public)
GET  /billing              BillingController@index      (auth)
POST /billing/checkout     BillingController@checkout   (auth)
GET  /billing/portal       BillingController@portal     (auth)
POST /stripe/webhook       Cashier webhook handler      (exempt from CSRF)
```

### Pricing Page (`/pricing`)

- Public page, 4-tier feature comparison table
- Monthly/annual toggle
- Free: "Get Started", Pro/Business: "Subscribe" (-> Stripe Checkout), Enterprise: "Contact Us"
- Unauthenticated users directed to register first

### Billing Dashboard (`/billing`)

- Current plan + status (active, trialing, past due, canceled)
- Current period end date
- Usage summary: forms used (e.g. "12 / 25"), submissions this month ("1,847 / 2,500")
- "Change Plan" -> Stripe Checkout (upgrade) or Stripe Portal (downgrade)
- "Manage Billing" -> Stripe Customer Portal

### Usage Endpoints (Go)

- `GET /api/usage/forms-count` — user's form count
- `GET /api/usage/submissions-count?month=2026-02` — user's submission count for month

### Checkout Flow

1. Pricing page sends `price_id` to `POST /billing/checkout`
2. Laravel creates Cashier Checkout Session with success/cancel URLs
3. Redirect to `checkout.stripe.com`
4. Stripe webhook confirms payment
5. Subscription activated

## Error Handling

### Limit Exceeded Response (Go, HTTP 403)

```json
{
  "success": false,
  "error": "limit_exceeded",
  "message": "Free plan allows 3 forms. Upgrade to Pro for up to 25.",
  "data": {
    "limit_type": "max_forms",
    "current": 3,
    "limit": 3,
    "required_tier": "pro"
  }
}
```

Laravel catches 403 and renders upgrade prompt with link to pricing page.

### Feature Not Available Response (Go, HTTP 403)

Same pattern with `error: "feature_not_available"`.

### Subscription Lifecycle

| Scenario | Behavior |
|----------|----------|
| Payment fails | `past_due` status, paid tier during retry window, then `free` |
| User cancels | Active until period end (grace period), then `free` |
| Downgrade over limit | Existing forms preserved, no new creates until under limit |
| Upgrade mid-cycle | Stripe prorates, tier upgrades immediately |
| Enterprise | `plan_override` column checked before Cashier state |

## Testing Strategy

### Laravel (Pest)

- `PlanTierTest` — `User::planTier()` for each subscription state
- `BillingControllerTest` — checkout redirect, portal redirect, pricing page
- `GoFormsClientTest` — `X-Plan-Tier` header included and signed
- `WebhookTest` — subscription state updates on Stripe events

### Go (testify)

- `assertion_test.go` — 3-part HMAC payload, backward compat default to `free`
- `plan_limits_test.go` — limit checks per tier
- `form_service_test.go` — creation rejected over limit, allowed under
- `submission_service_test.go` — monthly limits, owner tier lookup
- `feature_gate_test.go` — schema validation rejects gated features

### Integration

- End-to-end: Checkout Session -> mock webhook -> subscription active -> create form -> tier header reaches Go -> limits enforced

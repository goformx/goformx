<?php

namespace App\Models;

use Illuminate\Contracts\Auth\MustVerifyEmail;
use Illuminate\Database\Eloquent\Concerns\HasUuids;
use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Foundation\Auth\User as Authenticatable;
use Illuminate\Notifications\Notifiable;
use Illuminate\Support\Facades\Log;
use Laravel\Cashier\Billable;
use Laravel\Fortify\TwoFactorAuthenticatable;

class User extends Authenticatable implements MustVerifyEmail
{
    /** @use HasFactory<\Database\Factories\UserFactory> */
    use Billable, HasFactory, HasUuids, Notifiable, TwoFactorAuthenticatable;

    protected $keyType = 'string';

    public $incrementing = false;

    /**
     * The attributes that are mass assignable.
     *
     * @var list<string>
     */
    protected $fillable = [
        'name',
        'email',
        'password',
    ];

    /**
     * The attributes that should be hidden for serialization.
     *
     * @var list<string>
     */
    protected $hidden = [
        'password',
        'two_factor_secret',
        'two_factor_recovery_codes',
        'remember_token',
    ];

    /**
     * Get the attributes that should be cast.
     *
     * @return array<string, string>
     */
    protected function casts(): array
    {
        return [
            'email_verified_at' => 'datetime',
            'password' => 'hashed',
            'two_factor_confirmed_at' => 'datetime',
            'plan_override' => 'string',
        ];
    }

    private const VALID_TIERS = ['free', 'pro', 'business', 'growth', 'enterprise'];

    /**
     * Resolve the user's effective subscription tier.
     *
     * Priority: plan_override (admin-set) > active Stripe subscription > free default.
     * The returned tier is included in HMAC assertion headers sent to the Go API.
     *
     * Note: 'founding' override maps to 'business' — Go never sees the 'founding' string.
     */
    public function planTier(): string
    {
        if ($this->plan_override) {
            if ($this->plan_override === 'founding') {
                return 'business';
            }

            if (in_array($this->plan_override, self::VALID_TIERS, true)) {
                return $this->plan_override;
            }

            Log::warning('User has unrecognized plan_override, falling through to subscription check', [
                'user_id' => $this->getKey(),
                'plan_override' => $this->plan_override,
            ]);
        }

        $prices = config('services.stripe.prices');

        if (empty($prices) || ! is_array($prices)) {
            Log::error('Stripe price configuration missing - all users will resolve as free tier', [
                'user_id' => $this->getKey(),
            ]);

            return 'free';
        }

        $growthPrices = array_filter([
            $prices['growth_monthly'] ?? null,
            $prices['growth_annual'] ?? null,
        ]);

        if ($growthPrices && $this->subscribedToPrice($growthPrices)) {
            return 'growth';
        }

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

    public static function foundingMemberSlotsRemaining(): int
    {
        $cap = (int) config('services.founding_member_cap', 100);
        $used = static::query()->where('plan_override', 'founding')->count();

        return max(0, $cap - $used);
    }

    public static function canGrantFoundingMembership(): bool
    {
        return static::foundingMemberSlotsRemaining() > 0;
    }
}

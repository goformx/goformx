<?php

namespace App\Models;

// use Illuminate\Contracts\Auth\MustVerifyEmail;
use Illuminate\Database\Eloquent\Concerns\HasUuids;
use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Foundation\Auth\User as Authenticatable;
use Illuminate\Notifications\Notifiable;
use Laravel\Cashier\Billable;
use Laravel\Fortify\TwoFactorAuthenticatable;

class User extends Authenticatable
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
        'plan_override',
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
}

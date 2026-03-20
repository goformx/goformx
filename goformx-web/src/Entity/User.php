<?php

declare(strict_types=1);

namespace GoFormX\Entity;

use Waaseyaa\Entity\ContentEntityBase;

/**
 * User entity backed by MariaDB users table.
 */
final class User extends ContentEntityBase
{
    public function __construct(array $values = [])
    {
        parent::__construct($values, 'users', [
            'id' => 'id',
            'label' => 'name',
        ]);
    }

    public function name(): string
    {
        return (string) ($this->values['name'] ?? '');
    }

    public function email(): string
    {
        return (string) ($this->values['email'] ?? '');
    }

    public function password(): string
    {
        return (string) ($this->values['password'] ?? '');
    }

    public function emailVerifiedAt(): ?string
    {
        return $this->values['email_verified_at'] ?? null;
    }

    public function twoFactorSecret(): ?string
    {
        return $this->values['two_factor_secret'] ?? null;
    }

    public function twoFactorConfirmedAt(): ?string
    {
        return $this->values['two_factor_confirmed_at'] ?? null;
    }

    public function hasTwoFactorEnabled(): bool
    {
        return $this->twoFactorSecret() !== null && $this->twoFactorConfirmedAt() !== null;
    }

    /** @return list<string> */
    public function twoFactorRecoveryCodes(): array
    {
        $codes = $this->values['two_factor_recovery_codes'] ?? '[]';

        return is_string($codes) ? (json_decode($codes, true) ?: []) : (array) $codes;
    }

    public function stripeId(): ?string
    {
        return $this->values['stripe_id'] ?? null;
    }

    public function planOverride(): ?string
    {
        return $this->values['plan_override'] ?? null;
    }
}

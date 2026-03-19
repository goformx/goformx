<?php

declare(strict_types=1);

namespace GoFormX\Controller;

use Waaseyaa\Inertia\Inertia;
use Waaseyaa\Inertia\InertiaResponse;

final class SettingsController
{
    /**
     * @param array<string, mixed> $user
     */
    public function profile(array $user): InertiaResponse
    {
        return Inertia::render('Settings/Profile', ['user' => $user]);
    }

    /**
     * @return list<string>
     */
    public function validateProfileUpdate(string $name, string $email): array
    {
        $errors = [];

        if ($name === '') {
            $errors[] = 'Name is required.';
        }

        if ($email === '') {
            $errors[] = 'Email is required.';
        } elseif (!filter_var($email, FILTER_VALIDATE_EMAIL)) {
            $errors[] = 'Please enter a valid email address.';
        }

        return $errors;
    }

    /**
     * @return list<string>
     */
    public function validatePasswordChange(
        string $currentPassword,
        string $newPassword,
        string $newPasswordConfirmation,
    ): array {
        $errors = [];

        if ($currentPassword === '') {
            $errors[] = 'Current password is required.';
        }

        if (strlen($newPassword) < 8) {
            $errors[] = 'New password must be at least 8 characters.';
        }

        if ($newPassword !== $newPasswordConfirmation) {
            $errors[] = 'Passwords do not match.';
        }

        return $errors;
    }

    public function password(): InertiaResponse
    {
        return Inertia::render('Settings/Password', []);
    }

    public function appearance(): InertiaResponse
    {
        return Inertia::render('Settings/Appearance', []);
    }

    /**
     * @param array<string, mixed> $twoFactorData
     */
    public function twoFactor(array $twoFactorData): InertiaResponse
    {
        return Inertia::render('Settings/TwoFactor', ['twoFactor' => $twoFactorData]);
    }
}

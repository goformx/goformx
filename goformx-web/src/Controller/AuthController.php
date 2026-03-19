<?php

declare(strict_types=1);

namespace GoFormX\Controller;

final class AuthController
{
    private const int MIN_PASSWORD_LENGTH = 8;

    /**
     * @return list<string>
     */
    public function validateLogin(string $email, string $password): array
    {
        $errors = [];

        if ($email === '') {
            $errors[] = 'Email is required.';
        } elseif (!filter_var($email, FILTER_VALIDATE_EMAIL)) {
            $errors[] = 'Please enter a valid email address.';
        }

        if ($password === '') {
            $errors[] = 'Password is required.';
        }

        return $errors;
    }

    /**
     * @return list<string>
     */
    public function validateRegistration(
        string $name,
        string $email,
        string $password,
        string $passwordConfirmation,
    ): array {
        $errors = [];

        if ($name === '') {
            $errors[] = 'Name is required.';
        }

        if ($email === '') {
            $errors[] = 'Email is required.';
        } elseif (!filter_var($email, FILTER_VALIDATE_EMAIL)) {
            $errors[] = 'Please enter a valid email address.';
        }

        if ($password === '') {
            $errors[] = 'Password is required.';
        } elseif (strlen($password) < self::MIN_PASSWORD_LENGTH) {
            $errors[] = 'Password must be at least 8 characters.';
        } elseif ($password !== $passwordConfirmation) {
            $errors[] = 'Passwords do not match.';
        }

        return $errors;
    }

    /**
     * @return list<string>
     */
    public function validatePasswordReset(
        string $email,
        string $password,
        string $passwordConfirmation,
    ): array {
        $errors = [];

        if ($email === '') {
            $errors[] = 'Email is required.';
        } elseif (!filter_var($email, FILTER_VALIDATE_EMAIL)) {
            $errors[] = 'Please enter a valid email address.';
        }

        if (strlen($password) < self::MIN_PASSWORD_LENGTH) {
            $errors[] = 'Password must be at least 8 characters.';
        }

        if ($password !== $passwordConfirmation) {
            $errors[] = 'Passwords do not match.';
        }

        return $errors;
    }
}

<?php

declare(strict_types=1);

namespace GoFormX\Tests\Unit;

use GoFormX\Controller\AuthController;
use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;

#[CoversClass(AuthController::class)]
final class AuthControllerTest extends TestCase
{
    private AuthController $controller;

    protected function setUp(): void
    {
        $this->controller = new AuthController();
    }

    public function testValidateLoginReturnsErrorsForMissingFields(): void
    {
        $errors = $this->controller->validateLogin('', '');

        $this->assertContains('Email is required.', $errors);
        $this->assertContains('Password is required.', $errors);
    }

    public function testValidateLoginReturnsErrorForInvalidEmail(): void
    {
        $errors = $this->controller->validateLogin('not-an-email', 'password');

        $this->assertContains('Please enter a valid email address.', $errors);
    }

    public function testValidateLoginReturnsEmptyForValidInput(): void
    {
        $errors = $this->controller->validateLogin('alice@test.com', 'password123');

        $this->assertSame([], $errors);
    }

    public function testValidateRegistrationReturnsErrorsForMissingFields(): void
    {
        $errors = $this->controller->validateRegistration('', '', '', '');

        $this->assertContains('Name is required.', $errors);
        $this->assertContains('Email is required.', $errors);
        $this->assertContains('Password is required.', $errors);
    }

    public function testValidateRegistrationReturnsErrorForShortPassword(): void
    {
        $errors = $this->controller->validateRegistration('Alice', 'alice@test.com', 'short', 'short');

        $this->assertContains('Password must be at least 8 characters.', $errors);
    }

    public function testValidateRegistrationReturnsErrorForMismatchedPasswords(): void
    {
        $errors = $this->controller->validateRegistration('Alice', 'alice@test.com', 'password123', 'different');

        $this->assertContains('Passwords do not match.', $errors);
    }

    public function testValidateRegistrationReturnsEmptyForValidInput(): void
    {
        $errors = $this->controller->validateRegistration('Alice', 'alice@test.com', 'password123', 'password123');

        $this->assertSame([], $errors);
    }

    public function testValidatePasswordResetReturnsErrors(): void
    {
        $errors = $this->controller->validatePasswordReset('', 'short', 'different');

        $this->assertContains('Email is required.', $errors);
        $this->assertContains('Password must be at least 8 characters.', $errors);
        $this->assertContains('Passwords do not match.', $errors);
    }

    public function testValidatePasswordResetReturnsEmptyForValidInput(): void
    {
        $errors = $this->controller->validatePasswordReset('alice@test.com', 'newpassword', 'newpassword');

        $this->assertSame([], $errors);
    }
}

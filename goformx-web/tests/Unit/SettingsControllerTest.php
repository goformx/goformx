<?php

declare(strict_types=1);

namespace GoFormX\Tests\Unit;

use GoFormX\Controller\SettingsController;
use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;
use Waaseyaa\Inertia\Inertia;
use Waaseyaa\Inertia\InertiaResponse;

#[CoversClass(SettingsController::class)]
final class SettingsControllerTest extends TestCase
{
    private SettingsController $controller;

    protected function setUp(): void
    {
        Inertia::reset();
        Inertia::setVersion('v1');
        $this->controller = new SettingsController();
    }

    public function testProfileReturnsUserData(): void
    {
        $response = $this->controller->profile(['name' => 'Alice', 'email' => 'alice@test.com']);

        $this->assertInstanceOf(InertiaResponse::class, $response);
        $page = $response->toPageObject();
        $this->assertSame('Settings/Profile', $page['component']);
        $this->assertSame('Alice', $page['props']['user']['name']);
    }

    public function testValidateProfileUpdateErrors(): void
    {
        $errors = $this->controller->validateProfileUpdate('', '');
        $this->assertContains('Name is required.', $errors);
        $this->assertContains('Email is required.', $errors);
    }

    public function testValidateProfileUpdateValid(): void
    {
        $errors = $this->controller->validateProfileUpdate('Alice', 'alice@test.com');
        $this->assertSame([], $errors);
    }

    public function testValidatePasswordChangeErrors(): void
    {
        $errors = $this->controller->validatePasswordChange('', 'short', 'different');
        $this->assertContains('Current password is required.', $errors);
        $this->assertContains('New password must be at least 8 characters.', $errors);
    }

    public function testValidatePasswordChangeMismatch(): void
    {
        $errors = $this->controller->validatePasswordChange('current', 'longpassword', 'different');
        $this->assertContains('Passwords do not match.', $errors);
    }

    public function testValidatePasswordChangeValid(): void
    {
        $errors = $this->controller->validatePasswordChange('current', 'newpassword', 'newpassword');
        $this->assertSame([], $errors);
    }

    public function testPasswordReturnsInertia(): void
    {
        $response = $this->controller->password();
        $this->assertSame('Settings/Password', $response->toPageObject()['component']);
    }

    public function testAppearanceReturnsInertia(): void
    {
        $response = $this->controller->appearance();
        $this->assertSame('Settings/Appearance', $response->toPageObject()['component']);
    }

    public function testTwoFactorReturnsInertia(): void
    {
        $response = $this->controller->twoFactor(['enabled' => false]);
        $page = $response->toPageObject();
        $this->assertSame('Settings/TwoFactor', $page['component']);
        $this->assertFalse($page['props']['twoFactor']['enabled']);
    }
}

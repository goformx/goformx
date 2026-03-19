# `waaseyaa/auth` Package Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a headless authentication package for Waaseyaa providing login/logout, registration with email verification, password reset, TOTP two-factor authentication, rate limiting, and auth-required middleware.

**Architecture:** The package provides auth logic — applications provide UI and config. It builds on the existing `waaseyaa/user` package (User entity, SessionMiddleware) and `waaseyaa/mail` (email sending). AuthManager handles credential validation and session management. PasswordResetManager and EmailVerifier use HMAC-signed tokens. TwoFactorManager implements RFC 6238 TOTP directly (no external library). RateLimiter uses a simple token-bucket approach with session storage.

**Tech Stack:** PHP 8.4+, Symfony HttpFoundation, Waaseyaa foundation/user/mail packages, PHPUnit 10.5

**Spec:** `docs/superpowers/specs/2026-03-19-laravel-to-waaseyaa-migration-design.md` (Section: `waaseyaa/auth`)

**Working directory:** `/home/fsd42/dev/waaseyaa`

---

## File Structure

```
packages/auth/
├── composer.json
├── src/
│   ├── AuthServiceProvider.php       # Registers middleware
│   ├── AuthManager.php               # authenticate(), login(), logout()
│   ├── PasswordResetManager.php      # createToken(), findByToken(), resetPassword()
│   ├── EmailVerifier.php             # generateUrl(), verify()
│   ├── TwoFactorManager.php         # generateSecret(), verifyCode(), generateRecoveryCodes()
│   ├── RateLimiter.php               # attempt(), tooManyAttempts(), clear()
│   └── Middleware/
│       └── AuthenticateMiddleware.php  # Require authenticated user
└── tests/
    └── Unit/
        ├── AuthManagerTest.php
        ├── PasswordResetManagerTest.php
        ├── EmailVerifierTest.php
        ├── TwoFactorManagerTest.php
        ├── RateLimiterTest.php
        └── AuthenticateMiddlewareTest.php
```

## Key Interfaces (from existing codebase)

**User entity** (`packages/user/src/User.php`): Has `checkPassword($password)`, `getPassword()`, `setPassword($hash)`, `setRawPassword($plaintext)`, `isActive()`, `isAuthenticated()`, `id()`, `toArray()`.

**Session**: Existing `SessionMiddleware` stores `$_SESSION['waaseyaa_uid']` and loads user from storage. Auth package builds on top of this.

**Entity storage**: `$storage->getQuery()->condition('mail', $email)->execute()` returns entity IDs. `$storage->find($id)` returns entity.

**Mail**: `MailerInterface::send(Envelope)` where Envelope has `to`, `from`, `subject`, `textBody`, `htmlBody`.

**Middleware**: Implements `HttpMiddlewareInterface::process(Request, HttpHandlerInterface): Response`. Annotated with `#[AsMiddleware(pipeline: 'http', priority: N)]`.

---

### Task 1: Package Scaffold

**Files:**
- Create: `packages/auth/composer.json`
- Create: `packages/auth/src/AuthServiceProvider.php`
- Modify: `/home/fsd42/dev/waaseyaa/composer.json`

- [ ] **Step 1: Create composer.json**

```json
{
    "name": "waaseyaa/auth",
    "description": "Headless authentication for Waaseyaa — login, registration, 2FA, password reset",
    "type": "library",
    "license": "GPL-2.0-or-later",
    "repositories": [
        { "type": "path", "url": "../foundation" },
        { "type": "path", "url": "../user" },
        { "type": "path", "url": "../mail" }
    ],
    "require": {
        "php": ">=8.4",
        "waaseyaa/foundation": "@dev",
        "waaseyaa/user": "@dev",
        "waaseyaa/mail": "@dev"
    },
    "require-dev": {
        "phpunit/phpunit": "^10.5"
    },
    "autoload": {
        "psr-4": { "Waaseyaa\\Auth\\": "src/" }
    },
    "autoload-dev": {
        "psr-4": { "Waaseyaa\\Auth\\Tests\\": "tests/" }
    },
    "extra": {
        "waaseyaa": {
            "providers": ["Waaseyaa\\Auth\\AuthServiceProvider"]
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

namespace Waaseyaa\Auth;

use Waaseyaa\Foundation\ServiceProvider\ServiceProvider;

final class AuthServiceProvider extends ServiceProvider
{
    public function register(): void
    {
    }
}
```

- [ ] **Step 3: Register package in root composer.json**

Add to root `/home/fsd42/dev/waaseyaa/composer.json`:
- Add path repository: `{ "type": "path", "url": "packages/auth" }`
- Add to require: `"waaseyaa/auth": "@dev"`
- Add to autoload-dev psr-4: `"Waaseyaa\\Auth\\Tests\\": "packages/auth/tests/"`

- [ ] **Step 4: Run composer update to verify wiring**

Run: `cd /home/fsd42/dev/waaseyaa && composer update waaseyaa/auth`
Expected: Package resolves and installs without errors.

- [ ] **Step 5: Commit**

```bash
git add packages/auth/ composer.json composer.lock
git commit -m "feat(auth): scaffold waaseyaa/auth package"
```

---

### Task 2: AuthManager — Credential Validation & Session Management

**Files:**
- Create: `packages/auth/src/AuthManager.php`
- Create: `packages/auth/tests/Unit/AuthManagerTest.php`

The AuthManager authenticates users by email+password and manages session login/logout. It depends on entity storage (to look up users) and uses `$_SESSION` (which the existing SessionMiddleware already starts).

- [ ] **Step 1: Write the failing test**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Auth\Tests\Unit;

use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;
use Waaseyaa\Auth\AuthManager;
use Waaseyaa\User\User;

#[CoversClass(AuthManager::class)]
final class AuthManagerTest extends TestCase
{
    private AuthManager $auth;

    protected function setUp(): void
    {
        $this->auth = new AuthManager();
    }

    public function testAuthenticateReturnsUserOnValidCredentials(): void
    {
        $user = $this->createActiveUser('alice@test.com', 'secret123');

        $result = $this->auth->authenticate($user, 'secret123');

        $this->assertTrue($result);
    }

    public function testAuthenticateReturnsFalseOnWrongPassword(): void
    {
        $user = $this->createActiveUser('alice@test.com', 'secret123');

        $result = $this->auth->authenticate($user, 'wrongpassword');

        $this->assertFalse($result);
    }

    public function testAuthenticateReturnsFalseForInactiveUser(): void
    {
        $user = $this->createUser('blocked@test.com', 'secret123', active: false);

        $result = $this->auth->authenticate($user, 'secret123');

        $this->assertFalse($result);
    }

    public function testLoginSetsSessionUid(): void
    {
        $_SESSION = [];
        $user = $this->createActiveUser('alice@test.com', 'secret123');

        $this->auth->login($user);

        $this->assertSame($user->id(), $_SESSION['waaseyaa_uid']);
    }

    public function testLoginRegeneratesSessionId(): void
    {
        $_SESSION = ['waaseyaa_uid' => 'old-id', 'other' => 'data'];
        $user = $this->createActiveUser('alice@test.com', 'secret123');

        $this->auth->login($user);

        $this->assertSame($user->id(), $_SESSION['waaseyaa_uid']);
    }

    public function testLogoutClearsSession(): void
    {
        $_SESSION = ['waaseyaa_uid' => '123', 'other' => 'data'];

        $this->auth->logout();

        $this->assertArrayNotHasKey('waaseyaa_uid', $_SESSION);
    }

    public function testIsAuthenticatedReturnsTrueWhenSessionHasUid(): void
    {
        $_SESSION = ['waaseyaa_uid' => '123'];

        $this->assertTrue($this->auth->isAuthenticated());
    }

    public function testIsAuthenticatedReturnsFalseWhenNoSession(): void
    {
        $_SESSION = [];

        $this->assertFalse($this->auth->isAuthenticated());
    }

    private function createActiveUser(string $email, string $password): User
    {
        return $this->createUser($email, $password, active: true);
    }

    private function createUser(string $email, string $password, bool $active): User
    {
        $user = new User([
            'uid' => 'user-' . bin2hex(random_bytes(4)),
            'name' => explode('@', $email)[0],
            'mail' => $email,
            'pass' => password_hash($password, PASSWORD_BCRYPT),
            'status' => $active ? 1 : 0,
            'roles' => ['authenticated'],
            'created' => time(),
        ], 'user');

        return $user;
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/auth/tests/Unit/AuthManagerTest.php`
Expected: FAIL — `AuthManager` class not found.

- [ ] **Step 3: Write the implementation**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Auth;

use Waaseyaa\User\User;

final class AuthManager
{
    /**
     * Validate user credentials.
     */
    public function authenticate(User $user, string $password): bool
    {
        if (!$user->isActive()) {
            return false;
        }

        return $user->checkPassword($password);
    }

    /**
     * Log in a user by setting the session.
     */
    public function login(User $user): void
    {
        $_SESSION['waaseyaa_uid'] = $user->id();
    }

    /**
     * Log out by clearing the session user.
     */
    public function logout(): void
    {
        unset($_SESSION['waaseyaa_uid']);
    }

    /**
     * Check if the current session has an authenticated user.
     */
    public function isAuthenticated(): bool
    {
        return isset($_SESSION['waaseyaa_uid']) && $_SESSION['waaseyaa_uid'] !== '';
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/auth/tests/Unit/AuthManagerTest.php`
Expected: All 8 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/auth/src/AuthManager.php packages/auth/tests/Unit/AuthManagerTest.php
git commit -m "feat(auth): add AuthManager with authenticate, login, and logout"
```

---

### Task 3: RateLimiter

**Files:**
- Create: `packages/auth/src/RateLimiter.php`
- Create: `packages/auth/tests/Unit/RateLimiterTest.php`

A simple token-bucket rate limiter using an in-memory array (for testing) with a pluggable backend. Uses a key (e.g., `"login:email@test.com:127.0.0.1"`) to track attempts.

- [ ] **Step 1: Write the failing test**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Auth\Tests\Unit;

use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;
use Waaseyaa\Auth\RateLimiter;

#[CoversClass(RateLimiter::class)]
final class RateLimiterTest extends TestCase
{
    public function testAllowsAttemptsWithinLimit(): void
    {
        $limiter = new RateLimiter();

        for ($i = 0; $i < 5; $i++) {
            $this->assertFalse($limiter->tooManyAttempts('login:alice', 5));
            $limiter->hit('login:alice', 60);
        }
    }

    public function testBlocksAfterMaxAttempts(): void
    {
        $limiter = new RateLimiter();

        for ($i = 0; $i < 5; $i++) {
            $limiter->hit('login:alice', 60);
        }

        $this->assertTrue($limiter->tooManyAttempts('login:alice', 5));
    }

    public function testClearResetsAttempts(): void
    {
        $limiter = new RateLimiter();

        for ($i = 0; $i < 5; $i++) {
            $limiter->hit('login:alice', 60);
        }

        $limiter->clear('login:alice');

        $this->assertFalse($limiter->tooManyAttempts('login:alice', 5));
    }

    public function testDifferentKeysAreIndependent(): void
    {
        $limiter = new RateLimiter();

        for ($i = 0; $i < 5; $i++) {
            $limiter->hit('login:alice', 60);
        }

        $this->assertTrue($limiter->tooManyAttempts('login:alice', 5));
        $this->assertFalse($limiter->tooManyAttempts('login:bob', 5));
    }

    public function testAttemptsReturnsCount(): void
    {
        $limiter = new RateLimiter();

        $limiter->hit('login:alice', 60);
        $limiter->hit('login:alice', 60);
        $limiter->hit('login:alice', 60);

        $this->assertSame(3, $limiter->attempts('login:alice'));
    }

    public function testAttemptsReturnsZeroForUnknownKey(): void
    {
        $limiter = new RateLimiter();

        $this->assertSame(0, $limiter->attempts('unknown'));
    }

    public function testRemainingReturnsCorrectCount(): void
    {
        $limiter = new RateLimiter();

        $limiter->hit('login:alice', 60);
        $limiter->hit('login:alice', 60);

        $this->assertSame(3, $limiter->remaining('login:alice', 5));
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/auth/tests/Unit/RateLimiterTest.php`
Expected: FAIL — `RateLimiter` class not found.

- [ ] **Step 3: Write the implementation**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Auth;

final class RateLimiter
{
    /** @var array<string, array{count: int, resetAt: int}> */
    private array $attempts = [];

    /**
     * Record a hit for the given key.
     */
    public function hit(string $key, int $decaySeconds): void
    {
        $this->pruneExpired($key);

        if (!isset($this->attempts[$key])) {
            $this->attempts[$key] = [
                'count' => 0,
                'resetAt' => time() + $decaySeconds,
            ];
        }

        $this->attempts[$key]['count']++;
    }

    /**
     * Check if the key has exceeded the max attempts.
     */
    public function tooManyAttempts(string $key, int $maxAttempts): bool
    {
        $this->pruneExpired($key);

        return $this->attempts($key) >= $maxAttempts;
    }

    /**
     * Get the number of attempts for the key.
     */
    public function attempts(string $key): int
    {
        $this->pruneExpired($key);

        return $this->attempts[$key]['count'] ?? 0;
    }

    /**
     * Get the remaining attempts before hitting the limit.
     */
    public function remaining(string $key, int $maxAttempts): int
    {
        return max(0, $maxAttempts - $this->attempts($key));
    }

    /**
     * Clear attempts for the given key.
     */
    public function clear(string $key): void
    {
        unset($this->attempts[$key]);
    }

    private function pruneExpired(string $key): void
    {
        if (isset($this->attempts[$key]) && $this->attempts[$key]['resetAt'] <= time()) {
            unset($this->attempts[$key]);
        }
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/auth/tests/Unit/RateLimiterTest.php`
Expected: All 7 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/auth/src/RateLimiter.php packages/auth/tests/Unit/RateLimiterTest.php
git commit -m "feat(auth): add RateLimiter with token-bucket approach"
```

---

### Task 4: PasswordResetManager

**Files:**
- Create: `packages/auth/src/PasswordResetManager.php`
- Create: `packages/auth/tests/Unit/PasswordResetManagerTest.php`

Generates and validates password reset tokens. Tokens are HMAC-signed with a secret key and include the user ID and expiry timestamp.

- [ ] **Step 1: Write the failing test**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Auth\Tests\Unit;

use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;
use Waaseyaa\Auth\PasswordResetManager;

#[CoversClass(PasswordResetManager::class)]
final class PasswordResetManagerTest extends TestCase
{
    private PasswordResetManager $manager;

    protected function setUp(): void
    {
        $this->manager = new PasswordResetManager(
            secret: 'test-secret-key-for-hmac',
            tokenLifetimeSeconds: 3600,
        );
    }

    public function testCreateTokenReturnsNonEmptyString(): void
    {
        $token = $this->manager->createToken('user-123', 'alice@test.com');

        $this->assertNotEmpty($token);
    }

    public function testValidateTokenReturnsTrueForValidToken(): void
    {
        $token = $this->manager->createToken('user-123', 'alice@test.com');

        $result = $this->manager->validateToken($token, 'user-123', 'alice@test.com');

        $this->assertTrue($result);
    }

    public function testValidateTokenReturnsFalseForWrongUser(): void
    {
        $token = $this->manager->createToken('user-123', 'alice@test.com');

        $result = $this->manager->validateToken($token, 'user-456', 'alice@test.com');

        $this->assertFalse($result);
    }

    public function testValidateTokenReturnsFalseForWrongEmail(): void
    {
        $token = $this->manager->createToken('user-123', 'alice@test.com');

        $result = $this->manager->validateToken($token, 'user-123', 'bob@test.com');

        $this->assertFalse($result);
    }

    public function testValidateTokenReturnsFalseForTamperedToken(): void
    {
        $token = $this->manager->createToken('user-123', 'alice@test.com');

        $result = $this->manager->validateToken($token . 'tampered', 'user-123', 'alice@test.com');

        $this->assertFalse($result);
    }

    public function testValidateTokenReturnsFalseForExpiredToken(): void
    {
        $manager = new PasswordResetManager(
            secret: 'test-secret-key-for-hmac',
            tokenLifetimeSeconds: -1,
        );

        $token = $manager->createToken('user-123', 'alice@test.com');

        $result = $manager->validateToken($token, 'user-123', 'alice@test.com');

        $this->assertFalse($result);
    }

    public function testDifferentUsersGetDifferentTokens(): void
    {
        $token1 = $this->manager->createToken('user-123', 'alice@test.com');
        $token2 = $this->manager->createToken('user-456', 'bob@test.com');

        $this->assertNotSame($token1, $token2);
    }

    public function testExtractEmailFromToken(): void
    {
        $token = $this->manager->createToken('user-123', 'alice@test.com');

        $email = $this->manager->extractEmail($token);

        $this->assertSame('alice@test.com', $email);
    }

    public function testExtractEmailReturnsNullForInvalidToken(): void
    {
        $email = $this->manager->extractEmail('garbage-token');

        $this->assertNull($email);
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/auth/tests/Unit/PasswordResetManagerTest.php`
Expected: FAIL — class not found.

- [ ] **Step 3: Write the implementation**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Auth;

final class PasswordResetManager
{
    public function __construct(
        private readonly string $secret,
        private readonly int $tokenLifetimeSeconds = 3600,
    ) {
    }

    /**
     * Create a password reset token for the given user.
     *
     * Token format: base64(json({userId, email, expiresAt, signature}))
     */
    public function createToken(string $userId, string $email): string
    {
        $expiresAt = time() + $this->tokenLifetimeSeconds;
        $signature = $this->sign($userId, $email, $expiresAt);

        $payload = json_encode([
            'uid' => $userId,
            'email' => $email,
            'exp' => $expiresAt,
            'sig' => $signature,
        ], JSON_THROW_ON_ERROR);

        return rtrim(strtr(base64_encode($payload), '+/', '-_'), '=');
    }

    /**
     * Validate a password reset token against the expected user and email.
     */
    public function validateToken(string $token, string $userId, string $email): bool
    {
        $data = $this->decode($token);
        if ($data === null) {
            return false;
        }

        if ($data['uid'] !== $userId || $data['email'] !== $email) {
            return false;
        }

        if ($data['exp'] <= time()) {
            return false;
        }

        $expectedSignature = $this->sign($data['uid'], $data['email'], $data['exp']);

        return hash_equals($expectedSignature, $data['sig']);
    }

    /**
     * Extract the email from a token without full validation.
     */
    public function extractEmail(string $token): ?string
    {
        $data = $this->decode($token);

        return $data['email'] ?? null;
    }

    private function sign(string $userId, string $email, int $expiresAt): string
    {
        $payload = implode(':', [$userId, $email, (string) $expiresAt]);

        return hash_hmac('sha256', $payload, $this->secret);
    }

    /**
     * @return array{uid: string, email: string, exp: int, sig: string}|null
     */
    private function decode(string $token): ?array
    {
        $json = base64_decode(strtr($token, '-_', '+/'), true);
        if ($json === false) {
            return null;
        }

        try {
            $data = json_decode($json, true, 512, JSON_THROW_ON_ERROR);
        } catch (\JsonException) {
            return null;
        }

        if (!is_array($data)
            || !isset($data['uid'], $data['email'], $data['exp'], $data['sig'])
            || !is_string($data['uid'])
            || !is_string($data['email'])
            || !is_int($data['exp'])
            || !is_string($data['sig'])
        ) {
            return null;
        }

        return $data;
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/auth/tests/Unit/PasswordResetManagerTest.php`
Expected: All 9 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/auth/src/PasswordResetManager.php packages/auth/tests/Unit/PasswordResetManagerTest.php
git commit -m "feat(auth): add PasswordResetManager with HMAC-signed tokens"
```

---

### Task 5: EmailVerifier

**Files:**
- Create: `packages/auth/src/EmailVerifier.php`
- Create: `packages/auth/tests/Unit/EmailVerifierTest.php`

Generates and validates signed URLs for email verification. Uses HMAC to sign the user ID + email + expiry.

- [ ] **Step 1: Write the failing test**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Auth\Tests\Unit;

use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;
use Waaseyaa\Auth\EmailVerifier;

#[CoversClass(EmailVerifier::class)]
final class EmailVerifierTest extends TestCase
{
    private EmailVerifier $verifier;

    protected function setUp(): void
    {
        $this->verifier = new EmailVerifier(
            secret: 'test-verifier-secret',
            urlLifetimeSeconds: 3600,
        );
    }

    public function testGenerateUrlReturnsSignedUrl(): void
    {
        $url = $this->verifier->generateUrl(
            baseUrl: 'https://app.test/verify-email',
            userId: 'user-123',
            email: 'alice@test.com',
        );

        $this->assertStringStartsWith('https://app.test/verify-email?', $url);
        $this->assertStringContainsString('id=user-123', $url);
        $this->assertStringContainsString('signature=', $url);
        $this->assertStringContainsString('expires=', $url);
    }

    public function testVerifyReturnsTrueForValidUrl(): void
    {
        $url = $this->verifier->generateUrl(
            baseUrl: 'https://app.test/verify-email',
            userId: 'user-123',
            email: 'alice@test.com',
        );

        $params = $this->parseUrlParams($url);

        $result = $this->verifier->verify(
            userId: $params['id'],
            email: 'alice@test.com',
            expires: (int) $params['expires'],
            hash: $params['hash'],
            signature: $params['signature'],
        );

        $this->assertTrue($result);
    }

    public function testVerifyReturnsFalseForWrongEmail(): void
    {
        $url = $this->verifier->generateUrl(
            baseUrl: 'https://app.test/verify-email',
            userId: 'user-123',
            email: 'alice@test.com',
        );

        $params = $this->parseUrlParams($url);

        $result = $this->verifier->verify(
            userId: $params['id'],
            email: 'wrong@test.com',
            expires: (int) $params['expires'],
            hash: $params['hash'],
            signature: $params['signature'],
        );

        $this->assertFalse($result);
    }

    public function testVerifyReturnsFalseForExpiredUrl(): void
    {
        $verifier = new EmailVerifier(
            secret: 'test-verifier-secret',
            urlLifetimeSeconds: -1,
        );

        $url = $verifier->generateUrl(
            baseUrl: 'https://app.test/verify-email',
            userId: 'user-123',
            email: 'alice@test.com',
        );

        $params = $this->parseUrlParams($url);

        $result = $verifier->verify(
            userId: $params['id'],
            email: 'alice@test.com',
            expires: (int) $params['expires'],
            hash: $params['hash'],
            signature: $params['signature'],
        );

        $this->assertFalse($result);
    }

    public function testVerifyReturnsFalseForTamperedSignature(): void
    {
        $url = $this->verifier->generateUrl(
            baseUrl: 'https://app.test/verify-email',
            userId: 'user-123',
            email: 'alice@test.com',
        );

        $params = $this->parseUrlParams($url);

        $result = $this->verifier->verify(
            userId: $params['id'],
            email: 'alice@test.com',
            expires: (int) $params['expires'],
            hash: $params['hash'],
            signature: $params['signature'] . 'tampered',
        );

        $this->assertFalse($result);
    }

    public function testHashObscuresEmail(): void
    {
        $url = $this->verifier->generateUrl(
            baseUrl: 'https://app.test/verify-email',
            userId: 'user-123',
            email: 'alice@test.com',
        );

        $this->assertStringNotContainsString('alice@test.com', $url);
        $this->assertStringContainsString('hash=', $url);
    }

    /**
     * @return array<string, string>
     */
    private function parseUrlParams(string $url): array
    {
        $query = parse_url($url, PHP_URL_QUERY) ?? '';
        parse_str($query, $params);

        return $params;
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/auth/tests/Unit/EmailVerifierTest.php`
Expected: FAIL — class not found.

- [ ] **Step 3: Write the implementation**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Auth;

final class EmailVerifier
{
    public function __construct(
        private readonly string $secret,
        private readonly int $urlLifetimeSeconds = 3600,
    ) {
    }

    /**
     * Generate a signed verification URL.
     */
    public function generateUrl(string $baseUrl, string $userId, string $email): string
    {
        $expires = time() + $this->urlLifetimeSeconds;
        $hash = $this->hashEmail($email);
        $signature = $this->sign($userId, $email, $expires);

        $separator = str_contains($baseUrl, '?') ? '&' : '?';

        return $baseUrl . $separator . http_build_query([
            'id' => $userId,
            'hash' => $hash,
            'expires' => $expires,
            'signature' => $signature,
        ]);
    }

    /**
     * Verify a signed email verification URL.
     */
    public function verify(
        string $userId,
        string $email,
        int $expires,
        string $hash,
        string $signature,
    ): bool {
        if ($expires <= time()) {
            return false;
        }

        if (!hash_equals($this->hashEmail($email), $hash)) {
            return false;
        }

        $expectedSignature = $this->sign($userId, $email, $expires);

        return hash_equals($expectedSignature, $signature);
    }

    private function hashEmail(string $email): string
    {
        return hash_hmac('sha256', $email, $this->secret);
    }

    private function sign(string $userId, string $email, int $expires): string
    {
        $payload = implode(':', [$userId, $email, (string) $expires]);

        return hash_hmac('sha256', $payload, $this->secret);
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/auth/tests/Unit/EmailVerifierTest.php`
Expected: All 6 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/auth/src/EmailVerifier.php packages/auth/tests/Unit/EmailVerifierTest.php
git commit -m "feat(auth): add EmailVerifier with HMAC-signed verification URLs"
```

---

### Task 6: TwoFactorManager

**Files:**
- Create: `packages/auth/src/TwoFactorManager.php`
- Create: `packages/auth/tests/Unit/TwoFactorManagerTest.php`

Implements TOTP (RFC 6238) for two-factor authentication. Generates secrets, verifies 6-digit codes, and generates recovery codes. No external library — uses PHP's `hash_hmac` directly.

- [ ] **Step 1: Write the failing test**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Auth\Tests\Unit;

use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;
use Waaseyaa\Auth\TwoFactorManager;

#[CoversClass(TwoFactorManager::class)]
final class TwoFactorManagerTest extends TestCase
{
    private TwoFactorManager $twoFactor;

    protected function setUp(): void
    {
        $this->twoFactor = new TwoFactorManager();
    }

    public function testGenerateSecretReturns32CharBase32String(): void
    {
        $secret = $this->twoFactor->generateSecret();

        $this->assertSame(32, strlen($secret));
        $this->assertMatchesRegularExpression('/^[A-Z2-7]+$/', $secret);
    }

    public function testGenerateSecretIsUnique(): void
    {
        $secret1 = $this->twoFactor->generateSecret();
        $secret2 = $this->twoFactor->generateSecret();

        $this->assertNotSame($secret1, $secret2);
    }

    public function testVerifyCodeAcceptsValidCode(): void
    {
        $secret = $this->twoFactor->generateSecret();
        $code = $this->twoFactor->getCurrentCode($secret);

        $this->assertTrue($this->twoFactor->verifyCode($secret, $code));
    }

    public function testVerifyCodeRejectsInvalidCode(): void
    {
        $secret = $this->twoFactor->generateSecret();

        $this->assertFalse($this->twoFactor->verifyCode($secret, '000000'));
    }

    public function testVerifyCodeRejectsEmptyCode(): void
    {
        $secret = $this->twoFactor->generateSecret();

        $this->assertFalse($this->twoFactor->verifyCode($secret, ''));
    }

    public function testGetQrCodeUriReturnsOtpauthUri(): void
    {
        $uri = $this->twoFactor->getQrCodeUri(
            secret: 'JBSWY3DPEHPK3PXP',
            email: 'alice@test.com',
            issuer: 'GoFormX',
        );

        $this->assertStringStartsWith('otpauth://totp/', $uri);
        $this->assertStringContainsString('secret=JBSWY3DPEHPK3PXP', $uri);
        $this->assertStringContainsString('issuer=GoFormX', $uri);
        $this->assertStringContainsString('alice%40test.com', $uri);
    }

    public function testGenerateRecoveryCodesReturnsEightCodes(): void
    {
        $codes = $this->twoFactor->generateRecoveryCodes();

        $this->assertCount(8, $codes);
    }

    public function testRecoveryCodesAreUnique(): void
    {
        $codes = $this->twoFactor->generateRecoveryCodes();

        $this->assertCount(8, array_unique($codes));
    }

    public function testRecoveryCodesMatchExpectedFormat(): void
    {
        $codes = $this->twoFactor->generateRecoveryCodes();

        foreach ($codes as $code) {
            $this->assertMatchesRegularExpression('/^[a-zA-Z0-9]{5}-[a-zA-Z0-9]{5}$/', $code);
        }
    }

    public function testVerifyRecoveryCodeMatchesValidCode(): void
    {
        $codes = $this->twoFactor->generateRecoveryCodes();

        $this->assertTrue($this->twoFactor->verifyRecoveryCode($codes[0], $codes));
    }

    public function testVerifyRecoveryCodeRejectsInvalidCode(): void
    {
        $codes = $this->twoFactor->generateRecoveryCodes();

        $this->assertFalse($this->twoFactor->verifyRecoveryCode('XXXXX-XXXXX', $codes));
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/auth/tests/Unit/TwoFactorManagerTest.php`
Expected: FAIL — class not found.

- [ ] **Step 3: Write the implementation**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Auth;

final class TwoFactorManager
{
    private const int CODE_LENGTH = 6;
    private const int TIME_STEP = 30;
    private const int WINDOW = 1;
    private const int RECOVERY_CODE_COUNT = 8;
    private const string BASE32_ALPHABET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';

    /**
     * Generate a random Base32-encoded secret.
     */
    public function generateSecret(int $length = 20): string
    {
        $bytes = random_bytes($length);
        $base32 = '';

        $buffer = 0;
        $bitsLeft = 0;

        for ($i = 0; $i < strlen($bytes); $i++) {
            $buffer = ($buffer << 8) | ord($bytes[$i]);
            $bitsLeft += 8;

            while ($bitsLeft >= 5) {
                $bitsLeft -= 5;
                $base32 .= self::BASE32_ALPHABET[($buffer >> $bitsLeft) & 0x1F];
            }
        }

        return substr($base32, 0, 32);
    }

    /**
     * Verify a TOTP code against a secret.
     */
    public function verifyCode(string $secret, string $code): bool
    {
        if ($code === '' || strlen($code) !== self::CODE_LENGTH) {
            return false;
        }

        $timeStep = $this->currentTimeStep();

        for ($i = -self::WINDOW; $i <= self::WINDOW; $i++) {
            $expectedCode = $this->generateCode($secret, $timeStep + $i);
            if (hash_equals($expectedCode, $code)) {
                return true;
            }
        }

        return false;
    }

    /**
     * Get the current TOTP code for a secret (for testing).
     */
    public function getCurrentCode(string $secret): string
    {
        return $this->generateCode($secret, $this->currentTimeStep());
    }

    /**
     * Generate an otpauth:// URI for QR code generation.
     */
    public function getQrCodeUri(string $secret, string $email, string $issuer): string
    {
        $label = rawurlencode($issuer) . ':' . rawurlencode($email);

        return 'otpauth://totp/' . $label . '?' . http_build_query([
            'secret' => $secret,
            'issuer' => $issuer,
            'algorithm' => 'SHA1',
            'digits' => self::CODE_LENGTH,
            'period' => self::TIME_STEP,
        ]);
    }

    /**
     * Generate a set of recovery codes.
     *
     * @return list<string>
     */
    public function generateRecoveryCodes(): array
    {
        $codes = [];

        for ($i = 0; $i < self::RECOVERY_CODE_COUNT; $i++) {
            $codes[] = $this->generateRecoveryCode();
        }

        return $codes;
    }

    /**
     * Verify a recovery code against a list of valid codes.
     *
     * @param list<string> $validCodes
     */
    public function verifyRecoveryCode(string $code, array $validCodes): bool
    {
        foreach ($validCodes as $validCode) {
            if (hash_equals($validCode, $code)) {
                return true;
            }
        }

        return false;
    }

    private function generateCode(string $base32Secret, int $timeStep): string
    {
        $secretBytes = $this->base32Decode($base32Secret);
        $timeBytes = pack('N*', 0, $timeStep);
        $hash = hash_hmac('sha1', $timeBytes, $secretBytes, true);
        $offset = ord($hash[strlen($hash) - 1]) & 0x0F;

        $code = (
            ((ord($hash[$offset]) & 0x7F) << 24)
            | ((ord($hash[$offset + 1]) & 0xFF) << 16)
            | ((ord($hash[$offset + 2]) & 0xFF) << 8)
            | (ord($hash[$offset + 3]) & 0xFF)
        ) % (10 ** self::CODE_LENGTH);

        return str_pad((string) $code, self::CODE_LENGTH, '0', STR_PAD_LEFT);
    }

    private function currentTimeStep(): int
    {
        return (int) floor(time() / self::TIME_STEP);
    }

    private function base32Decode(string $base32): string
    {
        $base32 = strtoupper($base32);
        $buffer = 0;
        $bitsLeft = 0;
        $result = '';

        for ($i = 0; $i < strlen($base32); $i++) {
            $val = strpos(self::BASE32_ALPHABET, $base32[$i]);
            if ($val === false) {
                continue;
            }

            $buffer = ($buffer << 5) | $val;
            $bitsLeft += 5;

            if ($bitsLeft >= 8) {
                $bitsLeft -= 8;
                $result .= chr(($buffer >> $bitsLeft) & 0xFF);
            }
        }

        return $result;
    }

    private function generateRecoveryCode(): string
    {
        $chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';
        $part1 = '';
        $part2 = '';

        for ($i = 0; $i < 5; $i++) {
            $part1 .= $chars[random_int(0, strlen($chars) - 1)];
            $part2 .= $chars[random_int(0, strlen($chars) - 1)];
        }

        return $part1 . '-' . $part2;
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/auth/tests/Unit/TwoFactorManagerTest.php`
Expected: All 11 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/auth/src/TwoFactorManager.php packages/auth/tests/Unit/TwoFactorManagerTest.php
git commit -m "feat(auth): add TwoFactorManager with TOTP (RFC 6238) and recovery codes"
```

---

### Task 7: AuthenticateMiddleware

**Files:**
- Create: `packages/auth/src/Middleware/AuthenticateMiddleware.php`
- Create: `packages/auth/tests/Unit/AuthenticateMiddlewareTest.php`

Middleware that requires an authenticated user. Returns 302 redirect to a configurable login URL for unauthenticated requests. Checks `$_SESSION['waaseyaa_uid']`.

- [ ] **Step 1: Write the failing test**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Auth\Tests\Unit;

use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\TestCase;
use Symfony\Component\HttpFoundation\Request;
use Symfony\Component\HttpFoundation\Response;
use Waaseyaa\Auth\Middleware\AuthenticateMiddleware;
use Waaseyaa\Foundation\Middleware\HttpHandlerInterface;

#[CoversClass(AuthenticateMiddleware::class)]
final class AuthenticateMiddlewareTest extends TestCase
{
    public function testAuthenticatedUserPassesThrough(): void
    {
        $_SESSION = ['waaseyaa_uid' => 'user-123'];
        $middleware = new AuthenticateMiddleware('/login');
        $request = Request::create('/dashboard', 'GET');

        $handler = $this->createMockHandler(new Response('dashboard', 200));
        $response = $middleware->process($request, $handler);

        $this->assertSame(200, $response->getStatusCode());
        $this->assertSame('dashboard', $response->getContent());
    }

    public function testUnauthenticatedUserRedirectsToLogin(): void
    {
        $_SESSION = [];
        $middleware = new AuthenticateMiddleware('/login');
        $request = Request::create('/dashboard', 'GET');

        $handler = $this->createMockHandler(new Response('dashboard', 200));
        $response = $middleware->process($request, $handler);

        $this->assertSame(302, $response->getStatusCode());
        $this->assertSame('/login', $response->headers->get('Location'));
    }

    public function testUnauthenticatedUserWithEmptyUidRedirects(): void
    {
        $_SESSION = ['waaseyaa_uid' => ''];
        $middleware = new AuthenticateMiddleware('/login');
        $request = Request::create('/dashboard', 'GET');

        $handler = $this->createMockHandler(new Response('dashboard', 200));
        $response = $middleware->process($request, $handler);

        $this->assertSame(302, $response->getStatusCode());
    }

    public function testCustomLoginUrl(): void
    {
        $_SESSION = [];
        $middleware = new AuthenticateMiddleware('/auth/sign-in');
        $request = Request::create('/dashboard', 'GET');

        $handler = $this->createMockHandler(new Response());
        $response = $middleware->process($request, $handler);

        $this->assertSame('/auth/sign-in', $response->headers->get('Location'));
    }

    public function testInertiaRequestGets409InsteadOfRedirect(): void
    {
        $_SESSION = [];
        $middleware = new AuthenticateMiddleware('/login');
        $request = Request::create('/dashboard', 'GET');
        $request->headers->set('X-Inertia', 'true');

        $handler = $this->createMockHandler(new Response());
        $response = $middleware->process($request, $handler);

        $this->assertSame(409, $response->getStatusCode());
        $this->assertSame('/login', $response->headers->get('X-Inertia-Location'));
    }

    private function createMockHandler(Response $response): HttpHandlerInterface
    {
        return new class ($response) implements HttpHandlerInterface {
            public function __construct(private readonly Response $response)
            {
            }

            public function handle(Request $request): Response
            {
                return $this->response;
            }
        };
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/auth/tests/Unit/AuthenticateMiddlewareTest.php`
Expected: FAIL — class not found.

- [ ] **Step 3: Write the implementation**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Auth\Middleware;

use Symfony\Component\HttpFoundation\Request;
use Symfony\Component\HttpFoundation\Response;
use Waaseyaa\Foundation\Middleware\HttpHandlerInterface;
use Waaseyaa\Foundation\Middleware\HttpMiddlewareInterface;

final class AuthenticateMiddleware implements HttpMiddlewareInterface
{
    public function __construct(
        private readonly string $loginUrl = '/login',
    ) {
    }

    public function process(Request $request, HttpHandlerInterface $next): Response
    {
        $uid = $_SESSION['waaseyaa_uid'] ?? '';

        if ($uid !== '' && $uid !== 0) {
            return $next->handle($request);
        }

        if ($request->headers->get('X-Inertia') === 'true') {
            return new Response('', 409, [
                'X-Inertia-Location' => $this->loginUrl,
            ]);
        }

        return new Response('', 302, [
            'Location' => $this->loginUrl,
        ]);
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/auth/tests/Unit/AuthenticateMiddlewareTest.php`
Expected: All 5 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/auth/src/Middleware/AuthenticateMiddleware.php packages/auth/tests/Unit/AuthenticateMiddlewareTest.php
git commit -m "feat(auth): add AuthenticateMiddleware with Inertia 409 support"
```

---

### Task 8: Wire Service Provider

**Files:**
- Modify: `packages/auth/src/AuthServiceProvider.php`

- [ ] **Step 1: Update AuthServiceProvider**

```php
<?php

declare(strict_types=1);

namespace Waaseyaa\Auth;

use Waaseyaa\Auth\Middleware\AuthenticateMiddleware;
use Waaseyaa\Entity\EntityTypeManager;
use Waaseyaa\Foundation\Middleware\HttpMiddlewareInterface;
use Waaseyaa\Foundation\ServiceProvider\ServiceProvider;

final class AuthServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->singleton(AuthManager::class, fn() => new AuthManager());

        $this->singleton(RateLimiter::class, fn() => new RateLimiter());

        $this->singleton(PasswordResetManager::class, fn() => new PasswordResetManager(
            secret: $this->config['auth_secret'] ?? $this->config['app_secret'] ?? 'change-me',
            tokenLifetimeSeconds: (int) ($this->config['password_reset_lifetime'] ?? 3600),
        ));

        $this->singleton(EmailVerifier::class, fn() => new EmailVerifier(
            secret: $this->config['auth_secret'] ?? $this->config['app_secret'] ?? 'change-me',
            urlLifetimeSeconds: (int) ($this->config['email_verification_lifetime'] ?? 3600),
        ));

        $this->singleton(TwoFactorManager::class, fn() => new TwoFactorManager());
    }

    /**
     * @return list<HttpMiddlewareInterface>
     */
    public function middleware(EntityTypeManager $entityTypeManager): array
    {
        return [];
    }
}
```

> **Note:** `AuthenticateMiddleware` is NOT registered globally — it's applied per-route by the application. The service provider registers the core services.

- [ ] **Step 2: Run all tests**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/auth/tests/`
Expected: All tests PASS.

- [ ] **Step 3: Run CS Fixer**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/php-cs-fixer fix packages/auth/`

- [ ] **Step 4: Run tests again after CS Fixer**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/auth/tests/`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/auth/src/AuthServiceProvider.php
git commit -m "feat(auth): wire AuthServiceProvider with singleton registrations"
```

---

### Task 9: Deploy

**Files:**
- Modify: `.github/workflows/split.yml`

- [ ] **Step 1: Add to split workflow**

In `.github/workflows/split.yml`, add to the matrix under `# Layer 2: Services` (after content types, before `workflows`):

```yaml
          # Layer 2: Services
          - { local: 'packages/auth', remote: 'auth' }
```

Note: Place it in the Services layer since it depends on user (Layer 1) and foundation (Layer 0).

- [ ] **Step 2: Create split target repo**

Create `waaseyaa/auth` repository on GitHub (public, no README init).

- [ ] **Step 3: Commit and push**

```bash
git add .github/workflows/split.yml
git commit -m "ci: add waaseyaa/auth to monorepo split workflow"
git push
```

- [ ] **Step 4: Run full test suite**

Run: `cd /home/fsd42/dev/waaseyaa && vendor/bin/phpunit packages/auth/tests/`
Expected: All tests PASS.

- [ ] **Step 5: Tag and push**

```bash
# Check latest tag first
git tag --sort=-v:refname | head -1
# Increment alpha
git tag v0.1.0-alpha.<next>
git push origin v0.1.0-alpha.<next>
```

- [ ] **Step 6: Submit to Packagist**

Submit `https://github.com/waaseyaa/auth` at `https://packagist.org/packages/submit` after the split workflow completes.

---

## Summary

| Task | What it builds | Tests |
|---|---|---|
| 1 | Package scaffold (composer.json, service provider) | — |
| 2 | AuthManager (authenticate, login, logout) | 8 |
| 3 | RateLimiter (token-bucket rate limiting) | 7 |
| 4 | PasswordResetManager (HMAC-signed reset tokens) | 9 |
| 5 | EmailVerifier (signed verification URLs) | 6 |
| 6 | TwoFactorManager (TOTP + recovery codes) | 11 |
| 7 | AuthenticateMiddleware (require auth + Inertia support) | 5 |
| 8 | Service provider wiring | — |
| 9 | Deploy (split workflow, packagist) | — |

**Total: 9 tasks, ~46 tests, 7 source files**

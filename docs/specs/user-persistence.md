# User Persistence Spec

**Owns:** User entity storage, authentication data, and the migration path from raw PDO to Waaseyaa entity storage

---

## Current State (Migration Target)

**`UserRepository`** (`goformx-web/src/Service/UserRepository.php`) uses raw PDO to query a MariaDB `users` table. This violates the Waaseyaa persistence invariant (see `.claude/rules/waaseyaa-invariants.md`).

### Current Schema (MariaDB)

```sql
CREATE TABLE users (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    email_verified_at TIMESTAMP NULL,
    two_factor_secret VARCHAR(255) NULL,
    two_factor_confirmed_at TIMESTAMP NULL,
    two_factor_recovery_codes TEXT NULL,
    stripe_id VARCHAR(255) NULL,
    plan_override VARCHAR(50) NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE subscriptions (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    stripe_price VARCHAR(255),
    stripe_status VARCHAR(50),
    FOREIGN KEY (user_id) REFERENCES users(id)
);
```

### Current Methods

| Method | Query | Returns |
|--------|-------|---------|
| `findByEmail(string)` | SELECT * WHERE email = ? | `?array` |
| `findById(string)` | SELECT * WHERE id = ? | `?array` |
| `create(array)` | INSERT INTO users | `string` (id) |
| `updatePassword(string, string)` | UPDATE password | `void` |
| `verifyEmail(string)` | UPDATE email_verified_at | `void` |
| `updateProfile(string, string, string)` | UPDATE name, email | `void` |
| `delete(string)` | DELETE WHERE id = ? | `void` |
| `updateRecoveryCodes(string, array)` | UPDATE two_factor_recovery_codes | `void` |
| `getPlanTier(string)` | SELECT + subscription lookup | `string` |

---

## Target State

### User Entity

```php
use Waaseyaa\Entity\EntityBase;

class User extends EntityBase
{
    public function __construct(array $values)
    {
        parent::__construct($values, 'user', [
            'id' => 'id',
            'uuid' => 'uuid',
            'label' => 'name',
        ]);
    }

    public function name(): string { return $this->values['name'] ?? ''; }
    public function email(): string { return $this->values['email'] ?? ''; }
    public function password(): string { return $this->values['password'] ?? ''; }
    public function emailVerifiedAt(): ?string { return $this->values['email_verified_at'] ?? null; }
    public function twoFactorSecret(): ?string { return $this->values['two_factor_secret'] ?? null; }
    public function twoFactorConfirmedAt(): ?string { return $this->values['two_factor_confirmed_at'] ?? null; }
    public function twoFactorRecoveryCodes(): array {
        $codes = $this->values['two_factor_recovery_codes'] ?? '[]';
        return is_string($codes) ? json_decode($codes, true) ?: [] : $codes;
    }
    public function stripeId(): ?string { return $this->values['stripe_id'] ?? null; }
    public function planOverride(): ?string { return $this->values['plan_override'] ?? null; }
    public function hasTwoFactorEnabled(): bool {
        return $this->twoFactorSecret() !== null && $this->twoFactorConfirmedAt() !== null;
    }
}
```

### Entity Registration

```php
// In AppServiceProvider::register()
$entityTypeManager->addEntityType(new EntityType(
    id: 'user',
    label: 'User',
    class: User::class,
    keys: ['id' => 'id', 'uuid' => 'uuid', 'label' => 'name'],
));
```

### Repository (Post-Migration)

Replace `UserRepository` with `EntityRepository`-based queries:

```php
// Find by email
$users = $entityRepository->findBy(['email' => $email], limit: 1);
$user = $users[0] ?? null; // Returns User entity, not array

// Find by ID
$user = $entityRepository->find($id); // Returns ?User

// Create
$user = new User(['name' => $name, 'email' => $email, 'password' => password_hash($pw, PASSWORD_BCRYPT)]);
$storage->save($user);

// Update
$user->set('name', $newName);
$storage->save($user);

// Delete
$storage->delete($user);
```

---

## Migration Considerations

### Password Hashing

Password hashing (`password_hash`/`password_verify`) must remain in the application layer, NOT in the entity or storage layer. The entity stores the hash; the auth controller verifies it.

### Subscription / Plan Tier

`getPlanTier()` currently queries a separate `subscriptions` table. Options:
1. **Keep as separate query** — subscription is not part of the User entity, query via `DatabaseInterface`
2. **Create Subscription entity** — separate entity type with its own storage

Recommended: Option 1 for now. Subscriptions are a billing concern, not a user identity concern.

### MariaDB vs SQLite

Current state: Users in MariaDB, Waaseyaa entities in SQLite.

The User entity migration must target **MariaDB** (the existing users table), not SQLite. Waaseyaa's `DatabaseInterface` supports both — configure the User entity storage to use the MariaDB connection.

### Migration Steps

1. Define `User` entity class extending `EntityBase`
2. Register entity type in `AppServiceProvider::register()`
3. Create `UserStorageDriver` implementing `EntityStorageDriverInterface` (backed by MariaDB `DatabaseInterface`)
4. Replace `UserRepository` usages with `EntityRepository` calls
5. Update `AuthController` and route handlers to work with `User` entity objects instead of arrays
6. Remove `UserRepository` class
7. Update tests

---

## Callers to Update

All callers currently receive `?array` from `UserRepository`. After migration, they receive `?User`:

| Caller | Current Pattern | Target Pattern |
|--------|----------------|----------------|
| Login route | `$user['password']` | `$user->password()` |
| Register route | `$users->create([...])` | `$storage->save(new User([...]))` |
| Profile update route | `$users->updateProfile(...)` | `$user->set('name', ...); $storage->save($user)` |
| Password update route | `$users->updatePassword(...)` | `$user->set('password', ...); $storage->save($user)` |
| 2FA routes | `$user['two_factor_secret']` | `$user->twoFactorSecret()` |
| Billing routes | `$users->getPlanTier(...)` | Separate subscription query |
| Settings routes | `$user['name']`, `$user['email']` | `$user->name()`, `$user->email()` |

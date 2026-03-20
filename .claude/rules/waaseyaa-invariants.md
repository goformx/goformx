# Waaseyaa Framework Invariants

This rule is always active. Follow it silently. These are non-negotiable constraints for all code in goformx-web.

---

## Persistence Pipeline

All entity persistence MUST follow this pipeline:

```
Entity (extends ContentEntityBase or EntityBase)
  → registered via EntityTypeManager
  → stored via SqlEntityStorage (implements EntityStorageInterface)
  → queried via EntityRepository (wraps EntityStorageDriverInterface)
  → uses DatabaseInterface (Doctrine DBAL, NOT raw PDO)
```

### Forbidden Patterns

| Pattern | Why | Use Instead |
|---------|-----|-------------|
| `new \PDO(...)` | Bypasses framework DB layer | `DatabaseInterface` via DI |
| `$pdo->prepare(...)` | Raw PDO queries | `EntityRepository::findBy()` or `DatabaseInterface::select()` |
| `\PDO::FETCH_ASSOC` returning arrays | Untyped, no entity lifecycle | Proper entity objects extending `EntityBase` |
| Direct SQL strings | No migration tracking, no events | `SqlEntityStorage` CRUD methods |

### Required Pattern

```php
// 1. Define entity type
$entityTypeManager->addEntityType(new EntityType(
    id: 'user',
    label: 'User',
    class: User::class,
    keys: ['id' => 'id', 'uuid' => 'uuid', 'label' => 'name'],
));

// 2. Use EntityRepository for queries
$user = $entityRepository->find($id);
$users = $entityRepository->findBy(['email' => $email]);

// 3. Use SqlEntityStorage for writes
$storage->save($user);
$storage->delete($user);
```

---

## No Illuminate / No Laravel

GoFormX-web is a **Waaseyaa application**. The following are forbidden:

| Forbidden | Why | Waaseyaa Equivalent |
|-----------|-----|-------------------|
| `use Illuminate\*` | Not a dependency | Waaseyaa or Symfony equivalents |
| `use Laravel\*` | Not a dependency | Waaseyaa packages |
| Laravel facades (`DB::`, `Cache::`, `Log::`) | Magic globals, not in framework | DI-injected services |
| Eloquent models (`extends Model`) | ORM not available | `EntityBase` / `ContentEntityBase` |
| Laravel middleware signatures | Different interface | `HttpMiddlewareInterface` |
| `env()` outside config files | Config should be centralized | `$this->config[...]` in providers |

---

## Dependency Direction

```
Infrastructure (Symfony, Doctrine DBAL, external SDKs)
  → Foundation (Waaseyaa kernel, DI, middleware pipeline)
    → Domain (entities, services, repositories)
      → Application (controllers, route handlers)
```

**Dependencies point inward.** Controllers depend on services. Services depend on entity interfaces. Infrastructure implements interfaces. Never import from a higher layer.

---

## Service Provider Pattern

All DI registration, route definition, and entity type registration happens in `ServiceProvider` subclasses:

- `register()` — bind interfaces to implementations, register singletons
- `routes()` — define HTTP routes via `WaaseyaaRouter`
- `middleware()` — return middleware instances
- Entity types registered in `register()` or `boot()` via `EntityTypeManager`

---

## Auth Pattern

- Session-based auth via `Waaseyaa\Auth\AuthManager`
- User ID stored in `$_SESSION['waaseyaa_uid']`
- Cross-service auth to Go API via HMAC assertion headers (see `docs/specs/cross-service-auth.md`)
- Never store passwords in entity storage — use dedicated auth tables with `password_hash()`/`password_verify()`

---

## Environment Access

- **In config files:** `getenv()` or `env()` helper (wraps `getenv()`)
- **In app code:** `$this->config[...]` from service provider
- **NEVER:** `$_ENV` (Waaseyaa's EnvLoader only populates `putenv()`/`getenv()`)

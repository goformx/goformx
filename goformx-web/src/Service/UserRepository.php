<?php

declare(strict_types=1);

namespace GoFormX\Service;

final class UserRepository
{
    private \PDO $pdo;

    public function __construct(string $host, string $database, string $username, string $password)
    {
        $this->pdo = new \PDO(
            sprintf('mysql:host=%s;dbname=%s;charset=utf8mb4', $host, $database),
            $username,
            $password,
            [\PDO::ATTR_ERRMODE => \PDO::ERRMODE_EXCEPTION],
        );
    }

    /**
     * @return array<string, mixed>|null
     */
    public function findByEmail(string $email): ?array
    {
        $stmt = $this->pdo->prepare('SELECT * FROM users WHERE email = :email LIMIT 1');
        $stmt->execute(['email' => $email]);
        $user = $stmt->fetch(\PDO::FETCH_ASSOC);

        return $user !== false ? $user : null;
    }

    /**
     * @return array<string, mixed>|null
     */
    public function findById(string $id): ?array
    {
        $stmt = $this->pdo->prepare('SELECT * FROM users WHERE id = :id LIMIT 1');
        $stmt->execute(['id' => $id]);
        $user = $stmt->fetch(\PDO::FETCH_ASSOC);

        return $user !== false ? $user : null;
    }

    /**
     * @param array<string, mixed> $data
     */
    public function create(array $data): string
    {
        $id = $data['id'] ?? bin2hex(random_bytes(18));
        $stmt = $this->pdo->prepare(
            'INSERT INTO users (id, name, email, password, created_at, updated_at)
             VALUES (:id, :name, :email, :password, NOW(), NOW())',
        );
        $stmt->execute([
            'id' => $id,
            'name' => $data['name'],
            'email' => $data['email'],
            'password' => password_hash($data['password'], PASSWORD_BCRYPT),
        ]);

        return $id;
    }

    public function updatePassword(string $id, string $newPassword): void
    {
        $stmt = $this->pdo->prepare('UPDATE users SET password = :password, updated_at = NOW() WHERE id = :id');
        $stmt->execute([
            'password' => password_hash($newPassword, PASSWORD_BCRYPT),
            'id' => $id,
        ]);
    }

    public function verifyEmail(string $id): void
    {
        $stmt = $this->pdo->prepare('UPDATE users SET email_verified_at = NOW(), updated_at = NOW() WHERE id = :id');
        $stmt->execute(['id' => $id]);
    }

    public function updateProfile(string $id, string $name, string $email): void
    {
        $stmt = $this->pdo->prepare('UPDATE users SET name = :name, email = :email, updated_at = NOW() WHERE id = :id');
        $stmt->execute([
            'name' => $name,
            'email' => $email,
            'id' => $id,
        ]);
    }

    public function delete(string $id): void
    {
        $stmt = $this->pdo->prepare('DELETE FROM users WHERE id = :id');
        $stmt->execute(['id' => $id]);
    }

    /**
     * @param list<string> $codes
     */
    public function updateRecoveryCodes(string $id, array $codes): void
    {
        $stmt = $this->pdo->prepare(
            'UPDATE users SET two_factor_recovery_codes = :codes, updated_at = NOW() WHERE id = :id',
        );
        $stmt->execute([
            'codes' => json_encode($codes),
            'id' => $id,
        ]);
    }

    public function getPlanTier(string $id): string
    {
        $user = $this->findById($id);
        if ($user === null) {
            return 'free';
        }

        $override = $user['plan_override'] ?? null;
        if ($override !== null && $override !== '') {
            if ($override === 'founding') {
                return 'business';
            }
            if (in_array($override, ['free', 'pro', 'business', 'growth', 'enterprise'], true)) {
                return $override;
            }
        }

        // Check active subscriptions
        $stmt = $this->pdo->prepare(
            "SELECT stripe_price FROM subscriptions
             WHERE user_id = :id AND stripe_status IN ('active', 'trialing')
             ORDER BY id DESC LIMIT 1",
        );
        $stmt->execute(['id' => $id]);
        $sub = $stmt->fetch(\PDO::FETCH_ASSOC);

        if ($sub === false) {
            return 'free';
        }

        // Price-to-tier mapping would come from config; return 'pro' as default paid
        return 'pro';
    }
}

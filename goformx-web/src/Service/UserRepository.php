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
        $stmt = $this->pdo->prepare('SELECT * FROM users WHERE mail = :email LIMIT 1');
        $stmt->execute(['email' => $email]);
        $user = $stmt->fetch(\PDO::FETCH_ASSOC);

        return $user !== false ? $user : null;
    }

    /**
     * @return array<string, mixed>|null
     */
    public function findById(string $uid): ?array
    {
        $stmt = $this->pdo->prepare('SELECT * FROM users WHERE uid = :uid LIMIT 1');
        $stmt->execute(['uid' => $uid]);
        $user = $stmt->fetch(\PDO::FETCH_ASSOC);

        return $user !== false ? $user : null;
    }

    /**
     * @param array<string, mixed> $data
     */
    public function create(array $data): string
    {
        $uid = $data['uid'] ?? bin2hex(random_bytes(18));
        $stmt = $this->pdo->prepare(
            'INSERT INTO users (uid, uuid, name, mail, pass, status, roles, created_at, updated_at)
             VALUES (:uid, :uuid, :name, :mail, :pass, :status, :roles, NOW(), NOW())',
        );
        $stmt->execute([
            'uid' => $uid,
            'uuid' => $uid,
            'name' => $data['name'],
            'mail' => $data['email'],
            'pass' => password_hash($data['password'], PASSWORD_BCRYPT),
            'status' => 1,
            'roles' => json_encode(['authenticated']),
        ]);

        return $uid;
    }

    public function updatePassword(string $uid, string $newPassword): void
    {
        $stmt = $this->pdo->prepare('UPDATE users SET pass = :pass, updated_at = NOW() WHERE uid = :uid');
        $stmt->execute([
            'pass' => password_hash($newPassword, PASSWORD_BCRYPT),
            'uid' => $uid,
        ]);
    }

    public function verifyEmail(string $uid): void
    {
        $stmt = $this->pdo->prepare('UPDATE users SET email_verified_at = NOW(), updated_at = NOW() WHERE uid = :uid');
        $stmt->execute(['uid' => $uid]);
    }

    public function getPlanTier(string $uid): string
    {
        $user = $this->findById($uid);
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
             WHERE user_id = :uid AND stripe_status IN ('active', 'trialing')
             ORDER BY id DESC LIMIT 1",
        );
        $stmt->execute(['uid' => $uid]);
        $sub = $stmt->fetch(\PDO::FETCH_ASSOC);

        if ($sub === false) {
            return 'free';
        }

        // Price-to-tier mapping would come from config; return 'pro' as default paid
        return 'pro';
    }
}

<?php

declare(strict_types=1);

namespace GoFormX\Service;

use GoFormX\Entity\User;
use Waaseyaa\Database\DatabaseInterface;
use Waaseyaa\EntityStorage\EntityRepository;

/**
 * User persistence layer backed by Waaseyaa EntityRepository + DatabaseInterface.
 *
 * Standard entity CRUD goes through EntityRepository (which dispatches domain events).
 * Subscription queries use DatabaseInterface directly (subscriptions are not an entity type).
 */
final class UserRepository
{
    public function __construct(
        private readonly EntityRepository $entityRepository,
        private readonly DatabaseInterface $database,
    ) {}

    public function findByEmail(string $email): ?User
    {
        $users = $this->entityRepository->findBy(['email' => $email], limit: 1);

        return $users[0] ?? null;
    }

    public function findById(string $id): ?User
    {
        return $this->entityRepository->find($id);
    }

    /**
     * @param array{name: string, email: string, password: string, id?: string} $data
     */
    public function create(array $data): string
    {
        $id = $data['id'] ?? bin2hex(random_bytes(18));
        $user = new User([
            'id' => $id,
            'name' => $data['name'],
            'email' => $data['email'],
            'password' => password_hash($data['password'], PASSWORD_BCRYPT),
            'created_at' => date('Y-m-d H:i:s'),
            'updated_at' => date('Y-m-d H:i:s'),
        ]);
        $user->enforceIsNew();
        $this->entityRepository->save($user);

        return $id;
    }

    public function updatePassword(string $id, string $newPassword): void
    {
        $user = $this->findById($id);
        if ($user === null) {
            return;
        }

        $user->set('password', password_hash($newPassword, PASSWORD_BCRYPT));
        $user->set('updated_at', date('Y-m-d H:i:s'));
        $this->entityRepository->save($user);
    }

    public function verifyEmail(string $id): void
    {
        $user = $this->findById($id);
        if ($user === null) {
            return;
        }

        $user->set('email_verified_at', date('Y-m-d H:i:s'));
        $user->set('updated_at', date('Y-m-d H:i:s'));
        $this->entityRepository->save($user);
    }

    public function updateProfile(string $id, string $name, string $email): void
    {
        $user = $this->findById($id);
        if ($user === null) {
            return;
        }

        $user->set('name', $name);
        $user->set('email', $email);
        $user->set('updated_at', date('Y-m-d H:i:s'));
        $this->entityRepository->save($user);
    }

    public function delete(string $id): void
    {
        $user = $this->findById($id);
        if ($user === null) {
            return;
        }

        $this->entityRepository->delete($user);
    }

    /**
     * @param list<string> $codes
     */
    public function updateRecoveryCodes(string $id, array $codes): void
    {
        $user = $this->findById($id);
        if ($user === null) {
            return;
        }

        $user->set('two_factor_recovery_codes', json_encode($codes));
        $user->set('updated_at', date('Y-m-d H:i:s'));
        $this->entityRepository->save($user);
    }

    /**
     * Determine the user's billing plan tier.
     *
     * Subscriptions are a billing concern — queried via DatabaseInterface,
     * not part of the User entity.
     */
    public function getPlanTier(string $id): string
    {
        $user = $this->findById($id);
        if ($user === null) {
            return 'free';
        }

        $override = $user->planOverride();
        if ($override !== null && $override !== '') {
            if ($override === 'founding') {
                return 'business';
            }
            if (in_array($override, ['free', 'pro', 'business', 'growth', 'enterprise'], true)) {
                return $override;
            }
        }

        // Check active subscriptions via DatabaseInterface
        $result = $this->database->select('subscriptions')
            ->fields('subscriptions', ['stripe_price'])
            ->condition('user_id', $id)
            ->condition('stripe_status', ['active', 'trialing'], 'IN')
            ->execute();

        $sub = null;
        foreach ($result as $row) {
            $sub = $row;
            break;
        }

        if ($sub === null) {
            return 'free';
        }

        return 'pro';
    }
}

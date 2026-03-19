-- Stripe subscriptions (replaces Laravel Cashier subscriptions table)

CREATE TABLE IF NOT EXISTS subscriptions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    type VARCHAR(255) NOT NULL DEFAULT 'default',
    stripe_id VARCHAR(255) NOT NULL UNIQUE,
    stripe_status VARCHAR(255) NOT NULL,
    stripe_price VARCHAR(255) NULL,
    quantity INT NULL,
    trial_ends_at TIMESTAMP NULL,
    ends_at TIMESTAMP NULL,
    _data JSON NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_subscriptions_user_id (user_id),
    INDEX idx_subscriptions_stripe_status (stripe_status),
    CONSTRAINT fk_subscriptions_user FOREIGN KEY (user_id) REFERENCES users(uid) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

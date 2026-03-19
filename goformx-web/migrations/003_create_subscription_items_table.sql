-- Stripe subscription items

CREATE TABLE IF NOT EXISTS subscription_items (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    subscription_id BIGINT UNSIGNED NOT NULL,
    stripe_id VARCHAR(255) NOT NULL UNIQUE,
    stripe_product VARCHAR(255) NOT NULL,
    stripe_price VARCHAR(255) NOT NULL,
    quantity INT NULL,
    _data JSON NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_subscription_items_subscription_id (subscription_id),
    CONSTRAINT fk_subscription_items_subscription FOREIGN KEY (subscription_id) REFERENCES subscriptions(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

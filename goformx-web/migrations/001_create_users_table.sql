-- GoFormX Users table (Waaseyaa entity storage)
-- Preserves existing Laravel user UUIDs, passwords, and Stripe fields

CREATE TABLE IF NOT EXISTS users (
    uid VARCHAR(36) NOT NULL PRIMARY KEY,
    uuid VARCHAR(36) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    mail VARCHAR(255) NOT NULL UNIQUE,
    pass VARCHAR(255) NOT NULL,
    status TINYINT(1) NOT NULL DEFAULT 1,
    roles JSON NOT NULL DEFAULT ('["authenticated"]'),
    permissions JSON NOT NULL DEFAULT ('[]'),
    email_verified_at TIMESTAMP NULL,
    two_factor_secret TEXT NULL,
    two_factor_recovery_codes TEXT NULL,
    two_factor_confirmed_at TIMESTAMP NULL,
    stripe_id VARCHAR(255) NULL,
    pm_type VARCHAR(255) NULL,
    pm_last_four VARCHAR(4) NULL,
    trial_ends_at TIMESTAMP NULL,
    plan_override VARCHAR(50) NULL,
    _data JSON NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_users_mail (mail),
    INDEX idx_users_stripe_id (stripe_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

<?php

declare(strict_types=1);

/**
 * Run database migrations.
 * Usage: php bin/migrate.php
 */

require_once dirname(__DIR__) . '/vendor/autoload.php';

// Load .env file (PHP CLI doesn't read it automatically)
$envFile = dirname(__DIR__) . '/.env';
if (is_file($envFile)) {
    foreach (file($envFile, FILE_IGNORE_NEW_LINES | FILE_SKIP_EMPTY_LINES) as $line) {
        if (str_starts_with(trim($line), '#')) {
            continue;
        }
        if (str_contains($line, '=')) {
            putenv(trim($line));
        }
    }
}

$dsn = sprintf(
    'mysql:host=%s;dbname=%s;charset=utf8mb4',
    getenv('DB_HOST') ?: '127.0.0.1',
    getenv('DB_DATABASE') ?: 'goformx',
);

$pdo = new PDO(
    $dsn,
    getenv('DB_USERNAME') ?: 'goformx',
    getenv('DB_PASSWORD') ?: 'goformx',
    [PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION],
);

// Track applied migrations
$pdo->exec('CREATE TABLE IF NOT EXISTS migrations (
    id INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    migration VARCHAR(255) NOT NULL,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4');

$applied = $pdo->query('SELECT migration FROM migrations')->fetchAll(PDO::FETCH_COLUMN);

$migrationsDir = dirname(__DIR__) . '/migrations';
$files = glob($migrationsDir . '/*.sql');
sort($files);

$count = 0;
foreach ($files as $file) {
    $name = basename($file);
    if (in_array($name, $applied, true)) {
        continue;
    }

    echo "Applying: {$name}\n";
    $sql = file_get_contents($file);
    $pdo->exec($sql);
    $pdo->prepare('INSERT INTO migrations (migration) VALUES (?)')->execute([$name]);
    $count++;
}

echo $count > 0 ? "Applied {$count} migration(s).\n" : "No new migrations.\n";

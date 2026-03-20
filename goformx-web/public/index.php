<?php

declare(strict_types=1);

require_once dirname(__DIR__) . '/vendor/autoload.php';

use Waaseyaa\Foundation\Kernel\HttpKernel;

try {
    $kernel = new HttpKernel(dirname(__DIR__));
    $kernel->handle();
} catch (\Throwable $e) {
    error_log('[GoFormX] Unhandled exception: ' . $e->getMessage() . ' in ' . $e->getFile() . ':' . $e->getLine());
    error_log('[GoFormX] Stack trace: ' . $e->getTraceAsString());
    http_response_code(500);
    header('Content-Type: application/json');
    if (($_ENV['APP_ENV'] ?? 'production') !== 'production') {
        echo json_encode([
            'error' => $e->getMessage(),
            'file' => $e->getFile(),
            'line' => $e->getLine(),
        ]);
    } else {
        echo json_encode(['error' => 'Internal Server Error']);
    }
    exit(1);
}

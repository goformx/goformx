<?php

namespace Deployer;

require 'recipe/laravel.php';
require 'contrib/rsync.php';

// ── Host ─────────────────────────────────────────────────────────────────────
host('production')
    ->set('hostname', getenv('DEPLOY_HOST') ?: 'coforge.xyz')
    ->set('remote_user', 'deployer')
    ->set('deploy_path', '/home/deployer/goformx')
    ->set('keep_releases', 5);

// ── Project ───────────────────────────────────────────────────────────────────
set('application', 'goformx-laravel');

// ── rsync: push from CI runner instead of server pulling from git ─────────────
// The workflow runs from goformx-laravel/ working directory.
// public/build/ is pre-built by `npm run build` in the CI step.
set('rsync_src', __DIR__);
set('rsync', [
    'flags' => 'rzE',
    'exclude-file' => false,
    'include-file' => false,
    'filter-file' => false,
    'filter-perdir' => false,
    'exclude' => [
        '.git',
        '.ddev',
        'node_modules',
        'tests',
        '.env',
        '.env.*',
        'storage',
        'database/database.sqlite',
    ],
    'include' => ['public/build/'],
    'filter' => [],
    'options' => ['delete'],
    'timeout' => 120,
]);

// Override update_code to use rsync instead of git clone
Deployer::get()->tasks->remove('deploy:update_code');
task('deploy:update_code', ['rsync']);

// ── Shared (persisted across releases) ───────────────────────────────────────
set('shared_files', ['.env']);
set('shared_dirs', ['storage']);
set('writable_dirs', [
    'bootstrap/cache',
    'storage',
    'storage/app/public',
    'storage/framework/cache',
    'storage/framework/sessions',
    'storage/framework/views',
    'storage/logs',
]);
set('writable_mode', 'chmod');
set('writable_chmod_mode', '0775');

// ── Composer ──────────────────────────────────────────────────────────────────
set('composer_options', '--prefer-dist --no-progress --no-interaction --optimize-autoloader --no-dev');

// ── Artisan tasks ─────────────────────────────────────────────────────────────
task('artisan:storage:link', artisan('storage:link --force'));
task('artisan:migrate', artisan('migrate --force'));
task('artisan:optimize', artisan('optimize'));

// Reload PHP-FPM after symlink so opcache picks up the new release
task('php-fpm:reload', function () {
    run('sudo /bin/systemctl reload php8.4-fpm');
});

// ── Task order ────────────────────────────────────────────────────────────────
after('deploy:vendors', 'artisan:storage:link');
after('artisan:storage:link', 'artisan:migrate');
after('artisan:migrate', 'artisan:optimize');
after('deploy:symlink', 'php-fpm:reload');

after('rollback', 'artisan:optimize');
after('rollback', 'php-fpm:reload');
after('deploy:failed', 'deploy:unlock');

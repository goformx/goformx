#!/usr/bin/env bash
set -euo pipefail

# GoFormX Web Deployment Script
# Usage: ./deploy/deploy.sh [target-host]

TARGET="${1:-deployer@goformx.com}"
DEPLOY_PATH="/home/deployer/goformx-web"
RELEASES_PATH="${DEPLOY_PATH}/releases"
SHARED_PATH="${DEPLOY_PATH}/shared"
RELEASE_NAME="$(date +%Y%m%d%H%M%S)"
RELEASE_PATH="${RELEASES_PATH}/${RELEASE_NAME}"
KEEP_RELEASES=5

echo "==> Deploying GoFormX Web to ${TARGET}:${DEPLOY_PATH}"
echo "==> Release: ${RELEASE_NAME}"

# Create release directory and sync files
ssh "${TARGET}" "mkdir -p ${RELEASE_PATH} ${SHARED_PATH}"
rsync -azP --exclude='.git' --exclude='node_modules' --exclude='frontend/node_modules' \
    --exclude='storage/*.sqlite' --exclude='.env' \
    ./ "${TARGET}:${RELEASE_PATH}/"

# Link shared files
ssh "${TARGET}" "ln -nfs ${SHARED_PATH}/.env ${RELEASE_PATH}/.env"
ssh "${TARGET}" "mkdir -p ${RELEASE_PATH}/storage && ln -nfs ${SHARED_PATH}/storage/goformx.sqlite ${RELEASE_PATH}/storage/goformx.sqlite 2>/dev/null || true"

# Install PHP dependencies
ssh "${TARGET}" "cd ${RELEASE_PATH} && composer install --no-dev --optimize-autoloader --no-interaction"

# Build frontend
ssh "${TARGET}" "cd ${RELEASE_PATH}/frontend && npm ci && npm run build"

# Run migrations
ssh "${TARGET}" "cd ${RELEASE_PATH} && php bin/migrate.php"

# Activate release
ssh "${TARGET}" "ln -nfs ${RELEASE_PATH} ${DEPLOY_PATH}/current"

# Reload PHP-FPM
ssh "${TARGET}" "sudo systemctl reload php8.4-fpm" || echo "Warning: Could not reload PHP-FPM"

# Cleanup old releases
ssh "${TARGET}" "cd ${RELEASES_PATH} && ls -1dt */ | tail -n +$((KEEP_RELEASES + 1)) | xargs -r rm -rf"

echo "==> Deployed successfully: ${RELEASE_NAME}"

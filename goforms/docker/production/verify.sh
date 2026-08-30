#!/bin/sh
# Canonical image-content gate. Build only; never publish or use real credentials.
set -eu

cd "$(dirname "$0")/../.."
prefix="goformx-packaging-$(date +%s)-$$"
api="$prefix:api"
maintenance="$prefix:maintenance"
default="$prefix:default"
cleanup() {
    docker image rm "$api" "$maintenance" "$default" >/dev/null 2>&1 || true
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

docker build --target api -f docker/production/Dockerfile -t "$api" .
docker build --target maintenance -f docker/production/Dockerfile -t "$maintenance" .
docker build -f docker/production/Dockerfile -t "$default" .

for serving in "$api" "$default"; do
    test "$(docker image inspect --format '{{json .Config.Cmd}}' "$serving")" = '["./bin/goforms"]'
    test "$(docker image inspect --format '{{json .Config.Healthcheck.Test}}' "$serving")" = '["CMD-SHELL","wget --no-verbose --tries=1 --spider http://localhost:8090/health || exit 1"]'
    docker run --rm --network none --read-only --cap-drop ALL --security-opt no-new-privileges \
        --entrypoint /bin/sh "$serving" -ec '
        test "$(id -u)" = 1001
        test -x /app/bin/goforms
        test "$(find /app -type f | sort)" = /app/bin/goforms
        ! command -v goformx-token
        ! command -v goformx-webhook-keys
    '
done

test "$(docker image inspect --format '{{json .Config.Healthcheck}}' "$maintenance")" = null
test "$(docker image inspect --format '{{json .Config.Cmd}}' "$maintenance")" = '["./bin/goformx-token"]'
docker run --rm --network none --read-only --cap-drop ALL --security-opt no-new-privileges \
    --entrypoint /bin/sh "$maintenance" -ec '
    test "$(id -u)" = 1001
    test "$(find /app -type f | sort)" = "$(printf "%s\n" /app/bin/goformx-token /app/bin/goformx-webhook-keys)"
    for tool in goformx-token goformx-webhook-keys; do
        test -x "/app/bin/$tool"
        # No arguments must reach the real CLI usage guard, not a missing loader.
        status=0
        output=$(/app/bin/"$tool" 2>&1) || status=$?
        test "$status" = 1
        case "$output" in
            "$tool: usage: $tool "*) ;;
            *) printf "%s\n" "unexpected CLI usage result: $tool" >&2; exit 1 ;;
        esac
    done
'
printf '%s\n' 'Packaging verified: API/default omit maintenance tools; maintenance CLIs execute as non-root.'

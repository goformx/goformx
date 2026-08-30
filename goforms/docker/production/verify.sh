#!/bin/sh
# Canonical image-content gate. Build only; never publish or use real credentials.
set -eu

cd "$(dirname "$0")/../.."
if ! docker buildx version >/dev/null 2>&1; then
    printf '%s\n' 'Packaging requires a working Docker Buildx plugin; repair plugin discovery before retrying.' >&2
    exit 1
fi
# Do not silently fall back to a different, deprecated builder.
export DOCKER_BUILDKIT=1

assert_config() {
    actual=$(docker image inspect --format "$2" "$1")
    if [ "$actual" != "$3" ]; then
        printf '%s\n' "$1: $2 expected $3; got $actual" >&2
        exit 1
    fi
}

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
    assert_config "$serving" '{{json .Config.Cmd}}' '["./bin/goforms"]'
    assert_config "$serving" '{{json .Config.WorkingDir}}' '"/app"'
    assert_config "$serving" '{{json .Config.Entrypoint}}' 'null'
    assert_config "$serving" '{{json .Config.ExposedPorts}}' '{"8090/tcp":{}}'
    assert_config "$serving" '{{json .Config.Healthcheck.Test}}' '["CMD-SHELL","wget --no-verbose --tries=1 --spider http://localhost:8090/health || exit 1"]'
    docker run --rm --network none --read-only --cap-drop ALL --security-opt no-new-privileges \
        --entrypoint /bin/sh "$serving" -ec '
        check() { "$@" || { printf "%s\n" "API packaging assertion failed: $*" >&2; exit 1; }; }
        check test "$(id -u)" = 1001
        check test -x /app/bin/goforms
        check test "$(find /app -type f | sort)" = /app/bin/goforms
    '
done

assert_config "$maintenance" '{{json .Config.Healthcheck}}' 'null'
assert_config "$maintenance" '{{json .Config.Cmd}}' '["./bin/goformx-token"]'
assert_config "$maintenance" '{{json .Config.WorkingDir}}' '"/app"'
assert_config "$maintenance" '{{json .Config.Entrypoint}}' 'null'
assert_config "$maintenance" '{{json .Config.ExposedPorts}}' 'null'
docker run --rm --network none --read-only --cap-drop ALL --security-opt no-new-privileges \
    --entrypoint /bin/sh "$maintenance" -ec '
    check() { "$@" || { printf "%s\n" "Maintenance packaging assertion failed: $*" >&2; exit 1; }; }
    check test "$(id -u)" = 1001
    check test "$(find /app -type f | sort)" = "$(printf "%s\n" /app/bin/goformx-token /app/bin/goformx-webhook-keys)"
    for tool in goformx-token goformx-webhook-keys; do
        check test -x "/app/bin/$tool"
        # No arguments must reach the real CLI usage guard, not a missing loader.
        status=0
        output=$(/app/bin/"$tool" 2>&1) || status=$?
        check test "$status" = 1
        case "$output" in
            "$tool: usage: $tool "*) ;;
            *) printf "%s\n" "unexpected CLI usage result: $tool" >&2; exit 1 ;;
        esac
    done
'
printf '%s\n' 'Packaging verified: API/default omit maintenance tools; maintenance CLIs execute as non-root.'

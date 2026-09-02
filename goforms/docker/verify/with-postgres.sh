#!/bin/sh
# Disposable PostgreSQL lifecycle for `task verify`.
#
# Usage: sh docker/verify/with-postgres.sh -- command [args...]
#
# Starts docker-compose.verify.yml under a Compose project name that is unique
# to this run, exports GOFORMX_TEST_DATABASE_URL and GOFORMX_VERIFY_PROJECT to
# the command, and always removes exactly that project (containers, volumes,
# orphans) after the command finishes, fails, or is interrupted.
#
# Exit status:
#   - the command's status when it is non-zero, even if cleanup also fails;
#     the cleanup failure is reported on stderr and never masks it;
#   - otherwise the cleanup status, so a leaked project fails the run;
#   - 130/143 after SIGINT/SIGTERM, once the command was stopped and cleanup ran;
#   - 2 for usage or project-name validation errors, before Docker is touched.
#
# Task's `defer:` is deliberately not used: Task ignores the exit status of a
# deferred command and only reports it under --verbose. See
# docs/testing-strategy.md for the operational contract and recovery command.
set -u

prefix=goformx-verify-
service=postgres

fail() {
    status=$1
    shift
    printf 'verify-postgres: %s\n' "$*" >&2
    exit "$status"
}

if [ "${1-}" != '--' ] || [ "$#" -lt 2 ]; then
    fail 2 'usage: with-postgres.sh -- command [args...]'
fi
shift

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd) || fail 2 'cannot resolve the wrapper directory'
compose_dir=$(CDPATH='' cd -- "$script_dir/../.." && pwd) || fail 2 'cannot resolve the service directory'
compose_file="$compose_dir/docker-compose.verify.yml"
[ -f "$compose_file" ] || fail 2 "missing Compose file $compose_file"

if [ -n "${GOFORMX_VERIFY_PROJECT:-}" ]; then
    project=$GOFORMX_VERIFY_PROJECT
else
    pid=$$
    entropy=$(od -An -N4 -tx1 /dev/urandom 2>/dev/null | tr -d ' \n')
    project="$prefix$(date +%s)-$pid${entropy:+-$entropy}"
fi
case "$project" in
    "$prefix"?*) ;;
    *) fail 2 "GOFORMX_VERIFY_PROJECT must start with '$prefix' followed by a run identifier so cleanup can only touch this verification run; got '$project'" ;;
esac
case "$project" in
    *[!a-z0-9_-]*) fail 2 "GOFORMX_VERIFY_PROJECT may contain only lowercase letters, digits, '-' and '_'; got '$project'" ;;
esac

if ! docker compose version >/dev/null 2>&1; then
    fail 1 'Verification requires the Docker CLI with the Compose v2 plugin (docker compose version must succeed); install or repair plugin discovery before retrying.'
fi

compose() {
    docker compose -p "$project" -f "$compose_file" "$@"
}

recovery="docker compose -p $project -f $compose_file down --volumes --remove-orphans"

existing=$(compose ps -a -q)
if [ -n "$existing" ]; then
    fail 1 "Compose project $project already has containers; verification never reuses state. Remove only that project with: $recovery"
fi

cleaned=0
cleanup_status=0
cleanup() {
    if [ "$cleaned" -eq 1 ]; then
        return 0
    fi
    cleaned=1
    printf 'verify-postgres: removing Compose project %s (containers, volumes, orphans)\n' "$project" >&2
    compose down --volumes --remove-orphans
    cleanup_status=$?
    if [ "$cleanup_status" -ne 0 ]; then
        printf 'verify-postgres: CLEANUP FAILED with status %s; Compose project %s may still exist\n' "$cleanup_status" "$project" >&2
        printf 'verify-postgres: recover with: %s\n' "$recovery" >&2
    fi
}

child=
# shellcheck disable=SC2329 # invoked indirectly by the signal traps below
on_signal() {
    trap '' INT TERM
    printf 'verify-postgres: received signal; stopping verification and removing Compose project %s\n' "$project" >&2
    if [ -n "$child" ]; then
        kill -s TERM "$child" 2>/dev/null
        wait "$child" 2>/dev/null
    fi
    cleanup
    exit "$1"
}
trap 'on_signal 130' INT
trap 'on_signal 143' TERM
trap cleanup EXIT

printf 'verify-postgres: Compose project %s (recovery if this run leaks: %s)\n' "$project" "$recovery" >&2
export GOFORMX_VERIFY_PROJECT="$project"

compose up -d --wait
status=$?
if [ "$status" -ne 0 ]; then
    printf 'verify-postgres: failed to start disposable PostgreSQL (status %s)\n' "$status" >&2
    cleanup
    exit "$status"
fi

mapping=$(compose port "$service" 5432)
status=$?
if [ "$status" -ne 0 ]; then
    printf 'verify-postgres: cannot read the published PostgreSQL port (status %s)\n' "$status" >&2
    cleanup
    exit "$status"
fi
port=${mapping##*:}
case "$port" in
    '' | *[!0-9]*)
        printf 'verify-postgres: unexpected published port mapping %s\n' "'$mapping'" >&2
        cleanup
        exit 1
        ;;
esac
export GOFORMX_TEST_DATABASE_URL="postgres://goformx:testpass@127.0.0.1:$port/goformx?sslmode=disable"
printf 'verify-postgres: disposable PostgreSQL listening on 127.0.0.1:%s\n' "$port" >&2

# Run asynchronously so trapped signals interrupt `wait` immediately and the
# handler can stop the command before cleanup.
"$@" &
child=$!
wait "$child"
status=$?
child=

cleanup
if [ "$status" -ne 0 ]; then
    if [ "$cleanup_status" -ne 0 ]; then
        printf 'verify-postgres: verification failed with status %s; returning it instead of the cleanup status reported above\n' "$status" >&2
    fi
    exit "$status"
fi
exit "$cleanup_status"

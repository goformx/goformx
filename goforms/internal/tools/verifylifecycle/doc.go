// Package verifylifecycle holds the contract tests for the disposable PostgreSQL
// lifecycle wrapper that task verify runs (docker/verify/with-postgres.sh).
//
// The tests drive the real POSIX shell script against a recording Docker CLI
// double, so they prove exit-status precedence, cleanup command construction,
// and per-run project scoping without touching a Docker daemon.
package verifylifecycle

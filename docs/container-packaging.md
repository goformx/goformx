# API and maintenance images

The canonical `goforms/docker/production/Dockerfile` has two supported targets:

- `api` contains only `/app/bin/goforms`, runs as UID 1001, and retains the API command, port 8090 and health check. It is also the default final target. Release CI explicitly selects `api` for the existing registry image.
- `maintenance` contains `/app/bin/goformx-token` and `/app/bin/goformx-webhook-keys`, runs as UID 1001, and has no API health check. Its default command prints token CLI usage and exits; operators must explicitly select a maintenance operation.

From the repository root at the reviewed release commit:

```sh
docker build --target api -f goforms/docker/production/Dockerfile -t goformx-api:reviewed goforms
docker build --target maintenance -f goforms/docker/production/Dockerfile -t goformx-maintenance:reviewed goforms
```

Build both from the same exact source revision and record the resulting image IDs/digests. These local example tags are not immutable release identifiers. This change does not publish a new maintenance registry tag; build the maintenance target from the reviewed checkout and pin its resulting image for operational use. An existing API-image digest no longer supplies maintenance tools in new builds. Select the maintenance image explicitly and supply its operation as a command, for example `/app/bin/goformx-webhook-keys verify`. Follow the vault, writer-shutdown and backup prerequisites in [webhooks](webhooks.md) and [management audit](management-audit.md); do not pass secrets in command arguments or reuse serving credentials merely for convenience.

This is packaging hygiene, not a database privilege boundary. An API process with token-table INSERT authority can still mint tokens without a CLI. Separate runtime/migration/operator database credentials and grants remain required follow-up work; no such separation or post-compromise tenant isolation is claimed here.

The Pi infrastructure currently owns a separate build recipe and source pin. This PR does not change that repository, promote a pin, or deploy either image. Do not infer live image contents from this source checkout.

## Verification

`task packaging` requires working Docker Buildx and checks plugin availability before building; it does not silently fall back to the legacy builder. It builds explicit API, explicit maintenance and default targets using this same Dockerfile. It checks image commands, working directories, entrypoints, exposed ports and health checks, UID 1001, exact application executable inventory, absence of maintenance tools from API/default images, and execution of both maintenance CLI usage guards. Containers have no network, credentials or writable root filesystem. Temporary image tags are removed on exit; ordinary Docker build cache is retained.

This gate runs at the end of `task verify`, including pull requests, before release publication. Root Taskfile changes trigger the workflow too. It tests locally native artifacts; release CI retains the existing amd64/arm64 build. It does not connect to PostgreSQL or execute a real maintenance operation; those behaviors remain covered by the Go integration suite.

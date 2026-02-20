# Design: Remove Dead Asset Infrastructure from goforms

**Date:** 2026-02-20
**Status:** Approved

## Context

When the frontend moved to `goformx-laravel`, the goforms Go backend became API-only. The `internal/infrastructure/web/` package was a Vite-powered asset infrastructure (embed.FS, manifest loading, dev/prod asset servers, resolvers) that was never wired into the FX module and has no `//go:embed` directive. The embed endpoint loads Form.io from CDN and needs no local assets.

## Decision

Delete the entire dead asset infrastructure (Option A).

## Scope

### Delete entirely

| Path | Reason |
|------|--------|
| `internal/infrastructure/web/server.go` | DevelopmentAssetServer, EmbeddedAssetServer, factories |
| `internal/infrastructure/web/resolver.go` | All resolvers, loadManifestFromFS, factories |
| `internal/infrastructure/web/assets.go` | AssetManager, NewModule, factories |
| `internal/infrastructure/web/types.go` | All interfaces, types, errors, constants |
| `internal/infrastructure/web/resolver_test.go` | Tests for deleted code |
| `test/mocks/web/mock_web.go` | Mocks for deleted interfaces |
| `public/fonts/bootstrap-icons.woff` | Served by nothing |
| `public/fonts/bootstrap-icons.woff2` | Served by nothing |

### Modify

| File | Change |
|------|--------|
| `internal/infrastructure/module.go` | Remove `embed` + `infraweb` imports; remove `AssetServerParams`, `AssetManagerParams`, `ProvideAssetServer`, `NewAssetManager` |
| `internal/infrastructure/config/app.go` | Remove `ViteDevHost`, `ViteDevPort` fields |
| `internal/infrastructure/config/viper.go` | Remove assignments for those two fields |

## Verification

- `task build` passes
- `task test:backend` passes

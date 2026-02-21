# Remove Dead Asset Infrastructure Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Delete the entire `internal/infrastructure/web/` package and all associated dead code (mocks, config fields, unused fonts) left over from when goforms had its own Vite-powered frontend.

**Architecture:** Pure deletion — nothing in the `infraweb` package is wired into the FX module, no `//go:embed` directive exists, and the embed endpoint loads Form.io from CDN. Remove the package, fix the two import sites, clean up config.

**Tech Stack:** Go, Uber FX, golangci-lint, go-task

---

### Task 1: Establish baseline

**Files:** none (read-only verification)

**Step 1: Confirm build and tests pass before touching anything**

```bash
cd goforms && task build && task test:backend
```

Expected: both pass clean. If not, note which tests/errors exist before proceeding.

**Step 2: Confirm the mock package is only self-referencing (not imported by tests)**

```bash
grep -r "mocks/web" goforms --include="*.go"
```

Expected output: only the two lines in `mock_web.go` and `types.go` itself (the `//go:generate` comment). No test file imports it.

---

### Task 2: Delete the `internal/infrastructure/web/` package

**Files:**
- Delete: `internal/infrastructure/web/server.go`
- Delete: `internal/infrastructure/web/resolver.go`
- Delete: `internal/infrastructure/web/assets.go`
- Delete: `internal/infrastructure/web/types.go`
- Delete: `internal/infrastructure/web/resolver_test.go`

**Step 1: Delete all five files**

```bash
rm goforms/internal/infrastructure/web/server.go \
   goforms/internal/infrastructure/web/resolver.go \
   goforms/internal/infrastructure/web/assets.go \
   goforms/internal/infrastructure/web/types.go \
   goforms/internal/infrastructure/web/resolver_test.go
```

**Step 2: Verify the directory is now empty**

```bash
ls goforms/internal/infrastructure/web/
```

Expected: `ls: cannot access ...` or empty listing — directory gone or empty.

---

### Task 3: Fix `internal/infrastructure/module.go`

**Files:**
- Modify: `goforms/internal/infrastructure/module.go`

The file imports `"embed"` (line 14) and `infraweb "github.com/goformx/goforms/internal/infrastructure/web"` (line 27), and defines four now-dead items: `AssetServerParams` (lines 117–124), `AssetManagerParams` (lines 126–132), `ProvideAssetServer` (lines 228–249), `NewAssetManager` (lines 251–268).

**Step 1: Remove the `embed` import**

In the import block, delete:
```go
	"embed"
```

**Step 2: Remove the `infraweb` import**

In the import block, delete:
```go
	infraweb "github.com/goformx/goforms/internal/infrastructure/web"
```

**Step 3: Remove `AssetServerParams` and `AssetManagerParams`**

Delete these two structs entirely:

```go
// AssetServerParams groups the dependencies for creating an asset server.
// The asset server handles static file serving with environment-specific optimizations.
type AssetServerParams struct {
	fx.In
	Config *config.Config `validate:"required"`
	Logger logging.Logger `validate:"required"`
	DistFS embed.FS
}

// AssetManagerParams contains dependencies for creating an asset manager
type AssetManagerParams struct {
	fx.In
	DistFS embed.FS
	Logger logging.Logger `validate:"required"`
	Config *config.Config `validate:"required"`
}
```

**Step 4: Remove `ProvideAssetServer` and `NewAssetManager`**

Delete these two functions entirely:

```go
// ProvideAssetServer creates an appropriate asset server based on the environment.
// In development, it serves static files from public directory while Vite handles JS/CSS.
// In production, it serves from embedded filesystem for optimal performance.
func ProvideAssetServer(p AssetServerParams) (infraweb.AssetServer, error) {
	...
}

// NewAssetManager creates a new asset manager with proper dependency validation.
// Returns the interface type for better dependency injection.
func NewAssetManager(p AssetManagerParams) (infraweb.AssetManagerInterface, error) {
	...
}
```

**Step 5: Verify the file compiles**

```bash
cd goforms && go build ./internal/infrastructure/...
```

Expected: no errors.

---

### Task 4: Remove `ViteDevHost` and `ViteDevPort` from config

**Files:**
- Modify: `goforms/internal/infrastructure/config/app.go` (lines 29–30)
- Modify: `goforms/internal/infrastructure/config/viper.go` (lines 165–166)

**Step 1: Remove fields from `app.go`**

Delete these two lines from the `AppConfig` struct:
```go
	ViteDevHost string `json:"vite_dev_host"`
	ViteDevPort string `json:"vite_dev_port"`
```

**Step 2: Remove assignments from `viper.go`**

Delete these two lines from the `AppConfig` literal:
```go
		ViteDevHost:    vc.viper.GetString("app.vite_dev_host"),
		ViteDevPort:    vc.viper.GetString("app.vite_dev_port"),
```

**Step 3: Verify config package compiles**

```bash
cd goforms && go build ./internal/infrastructure/config/...
```

Expected: no errors.

---

### Task 5: Delete mocks and unused fonts

**Files:**
- Delete: `goforms/test/mocks/web/mock_web.go`
- Delete: `goforms/public/fonts/bootstrap-icons.woff`
- Delete: `goforms/public/fonts/bootstrap-icons.woff2`

**Step 1: Delete the mock file and font files**

```bash
rm goforms/test/mocks/web/mock_web.go \
   goforms/public/fonts/bootstrap-icons.woff \
   goforms/public/fonts/bootstrap-icons.woff2
```

**Step 2: Remove the now-empty directories if empty**

```bash
rmdir goforms/test/mocks/web 2>/dev/null || true
rmdir goforms/public/fonts 2>/dev/null || true
```

---

### Task 6: Full verification

**Step 1: Build the binary**

```bash
cd goforms && task build
```

Expected: `Build successful` (or similar), exit code 0.

**Step 2: Run the linter**

```bash
cd goforms && task lint:backend
```

Expected: no errors. If unused import warnings appear, re-check Tasks 3–4.

**Step 3: Run all tests**

```bash
cd goforms && task test:backend
```

Expected: all tests pass, same count as baseline from Task 1.

---

### Task 7: Commit

**Step 1: Stage the deletions and modifications**

```bash
git -C /path/to/repo add \
  goforms/internal/infrastructure/web/ \
  goforms/internal/infrastructure/module.go \
  goforms/internal/infrastructure/config/app.go \
  goforms/internal/infrastructure/config/viper.go \
  goforms/test/mocks/web/ \
  goforms/public/fonts/
```

**Step 2: Commit**

```bash
git commit -m "chore: delete dead asset infrastructure from goforms

The infraweb package (asset servers, resolvers, embed.FS wiring,
Vite manifest loading) was never wired into the FX module and had
no go:embed directive. The embed endpoint uses CDN. goforms is
API-only with no frontend.

- Delete internal/infrastructure/web/ package (5 files)
- Remove ProvideAssetServer, NewAssetManager, AssetServerParams,
  AssetManagerParams from infrastructure/module.go
- Remove ViteDevHost, ViteDevPort from config
- Delete test/mocks/web/ (mocks for deleted interfaces)
- Delete public/fonts/ (bootstrap-icons served by nothing)

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

**Step 3: Push**

```bash
git push
```

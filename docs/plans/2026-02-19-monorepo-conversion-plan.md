# Monorepo Conversion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Merge `goformx/goforms` and `goformx/goformx-laravel` into a single monorepo at `goformx/goformx` with shared tooling, preserving full git history.

**Architecture:** Use `git subtree add` to import each repo into subdirectories of a new empty repo. Add root-level Taskfile, CI workflows, gitignore, and env reference. Remove per-service `.github/` directories.

**Tech Stack:** Git subtree, GitHub Actions (path-filtered), Taskfile v3, DDEV, Docker

**Design doc:** `docs/plans/2026-02-19-monorepo-conversion-design.md`

---

### Task 1: Create GitHub Repo and Perform Subtree Merge

**Prereq:** User must create the empty repo `goformx/goformx` on GitHub first (no README, no .gitignore, no license). This is a manual step.

**Files:**
- Create: new local clone of `goformx/goformx`

**Step 1: Ask user to create the empty repo on GitHub**

The user needs to create `goformx/goformx` on GitHub with no initialization files. Wait for confirmation.

**Step 2: Clone the empty repo and add remotes**

```bash
cd /home/fsd42/dev
git clone git@github.com:goformx/goformx.git goformx-monorepo
cd goformx-monorepo
git remote add goforms-origin git@github.com:goformx/goforms.git
git remote add laravel-origin git@github.com:goformx/goformx-laravel.git
```

**Step 3: Fetch both repos**

```bash
git fetch goforms-origin
git fetch laravel-origin
```

**Step 4: Subtree-add goforms**

```bash
git subtree add --prefix=goforms goforms-origin main
```

Expected: Creates a merge commit that adds all goforms files under `goforms/` with full history.

**Step 5: Verify goforms history**

```bash
git log --oneline goforms/main.go | head -5
```

Expected: Shows commits from the original goforms repo.

**Step 6: Subtree-add goformx-laravel**

```bash
git subtree add --prefix=goformx-laravel laravel-origin main
```

Expected: Creates a merge commit that adds all Laravel files under `goformx-laravel/` with full history.

**Step 7: Verify laravel history**

```bash
git log --oneline goformx-laravel/artisan | head -5
```

Expected: Shows commits from the original goformx-laravel repo.

**Step 8: Verify directory structure**

```bash
ls goforms/main.go goformx-laravel/artisan
```

Expected: Both files exist.

**Step 9: Commit checkpoint — do NOT push yet**

No commit needed (subtree add creates its own merge commits). Verify:

```bash
git log --oneline -5
```

Expected: Shows the two subtree merge commits at the top.

---

### Task 2: Create Root .gitignore

**Files:**
- Create: `.gitignore`

**Step 1: Create the combined .gitignore**

```gitignore
# Environment
.env
.env.local
.env.*.local

# IDE
.idea/
.vscode/
.fleet/
.nova/
.zed/
*.swp
*.swo
*~

# OS
.DS_Store
.DS_Store?
._*
Thumbs.db

# Go (goforms/)
goforms/bin/
goforms/dist/
goforms/coverage/
goforms/coverage.html
goforms/tmp/
goforms/.task/
goforms/air.log
goforms/storage/

# Laravel (goformx-laravel/)
goformx-laravel/vendor/
goformx-laravel/node_modules/
goformx-laravel/public/build/
goformx-laravel/public/hot
goformx-laravel/public/storage
goformx-laravel/storage/*.key
goformx-laravel/storage/pail
goformx-laravel/bootstrap/ssr
goformx-laravel/.phpunit.cache
goformx-laravel/.phpunit.result.cache
goformx-laravel/resources/js/actions
goformx-laravel/resources/js/routes
goformx-laravel/resources/js/wayfinder
```

Note: Each service also has its own `.gitignore` inside its subdirectory which git respects. The root `.gitignore` catches anything that leaks to the root level and provides IDE/OS coverage.

**Step 2: Stage and commit**

```bash
git add .gitignore
git commit -m "chore: add root .gitignore for monorepo"
```

---

### Task 3: Create Root .env.example

**Files:**
- Create: `.env.example`

**Step 1: Create the shared secrets reference**

```
# GoFormX Monorepo - Shared Configuration Reference
#
# Each service has its own .env file:
#   - goforms/.env         (copy from goforms/.env.example)
#   - goformx-laravel/.env (copy from goformx-laravel/.env.example)
#
# The following values MUST match across both .env files:

GOFORMS_SHARED_SECRET=your-secret-here

# Service URLs (for reference):
# Go API:     http://localhost:8090 (or http://goforms:8090 in DDEV)
# Laravel:    http://localhost:8000 (or https://goformx-laravel.ddev.site in DDEV)
```

**Step 2: Stage and commit**

```bash
git add .env.example
git commit -m "chore: add root .env.example with shared secret reference"
```

---

### Task 4: Create Root Taskfile.yml

**Files:**
- Create: `Taskfile.yml`

**Step 1: Create the root Taskfile**

```yaml
# yaml-language-server: $schema=https://taskfile.dev/schema.json
version: '3'
output: 'prefixed'

includes:
  go:
    taskfile: ./goforms/Taskfile.yml
    dir: ./goforms

tasks:
  dev:
    desc: Start both services concurrently
    deps: [dev:go, dev:laravel]

  dev:go:
    desc: Start Go backend with hot reload
    dir: ./goforms
    cmd: task dev:backend

  dev:laravel:
    desc: Start Laravel with Vite
    dir: ./goformx-laravel
    cmd: composer run dev

  install:
    desc: Install dependencies for both services
    cmds:
      - task: install:go
      - task: install:laravel

  install:go:
    desc: Install Go dependencies and tools
    dir: ./goforms
    cmd: task install

  install:laravel:
    desc: Install PHP and Node dependencies
    dir: ./goformx-laravel
    cmds:
      - composer install
      - npm install

  test:
    desc: Run all test suites
    deps: [test:go, test:laravel]

  test:go:
    desc: Run Go unit tests
    dir: ./goforms
    cmd: task test:backend

  test:laravel:
    desc: Run Laravel Pest tests
    dir: ./goformx-laravel
    cmd: php artisan test --compact

  lint:
    desc: Run all linters
    deps: [lint:go, lint:laravel]

  lint:go:
    desc: Run Go linters (fmt, vet, golangci-lint)
    dir: ./goforms
    cmd: task lint:backend

  lint:laravel:
    desc: Run PHP and frontend linters
    dir: ./goformx-laravel
    cmds:
      - vendor/bin/pint --dirty --format agent
      - npm run lint

  setup:
    desc: Full first-time setup (install, generate, migrate)
    cmds:
      - task: install
      - task: go:generate
      - task: go:migrate:up
```

**Step 2: Verify Taskfile syntax**

```bash
task --list
```

Expected: Shows all root tasks plus `go:*` namespace tasks from goforms.

**Step 3: Stage and commit**

```bash
git add Taskfile.yml
git commit -m "chore: add root Taskfile.yml orchestrating both services"
```

---

### Task 5: Create CI Workflow — goforms-ci.yml

**Files:**
- Create: `.github/workflows/goforms-ci.yml`

**Step 1: Create the Go CI workflow**

Adapted from `goforms/.github/workflows/ci-cd.yml`. Key changes:
- Add `paths: ['goforms/**']` to triggers
- Add `working-directory: ./goforms` to all run steps
- Update `go-version-file` to `goforms/go.mod`
- Update `cache-dependency-path` paths to `goforms/` prefix
- Update `dorny/paths-filter` glob patterns to `goforms/` prefix
- Update Docker build context to `./goforms`
- Update artifact paths to `goforms/` prefix

```yaml
# goforms-ci.yml - Go backend CI/CD
name: "GoForms CI/CD"

on:
  push:
    branches: [main]
    tags: ['v*']
    paths:
      - 'goforms/**'
      - '.github/workflows/goforms-ci.yml'
  pull_request:
    branches: [main]
    paths:
      - 'goforms/**'
      - '.github/workflows/goforms-ci.yml'

env:
  RUNNING_IN_ACT: ${{ github.actor == 'nektos/act' }}
  GO_VERSION_FILE: 'goforms/go.mod'
  TASK_VERSION: '3.x'
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

permissions:
  contents: write
  packages: write
  security-events: write
  attestations: write
  id-token: write

jobs:
  test:
    name: Test & Lint
    runs-on: ubuntu-latest
    timeout-minutes: 15
    defaults:
      run:
        working-directory: ./goforms
    outputs:
      should-build: ${{ steps.changes.outputs.code == 'true' }}
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4
        with:
          fetch-depth: 1

      - name: Check for code changes
        uses: dorny/paths-filter@v3
        id: changes
        with:
          filters: |
            code:
              - 'goforms/**.go'
              - 'goforms/go.mod'
              - 'goforms/go.sum'
              - 'goforms/Dockerfile'
              - 'goforms/docker/**'

      - name: Set up Go
        if: steps.changes.outputs.code == 'true'
        uses: actions/setup-go@v5
        with:
          go-version-file: ${{ env.GO_VERSION_FILE }}
          cache: true
          cache-dependency-path: goforms/go.sum

      - name: Install Task
        if: steps.changes.outputs.code == 'true'
        uses: arduino/setup-task@v2
        with:
          version: ${{ env.TASK_VERSION }}
          repo-token: ${{ secrets.GITHUB_TOKEN }}

      - name: Cache dependencies
        if: steps.changes.outputs.code == 'true'
        uses: actions/cache@v4
        with:
          path: |
            ~/go/bin
            ~/.cache/go-build
            ~/go/pkg/mod
          key: ${{ runner.os }}-go-deps-${{ hashFiles('goforms/go.mod', 'goforms/go.sum') }}
          restore-keys: |
            ${{ runner.os }}-go-deps-

      - name: Install dependencies
        if: steps.changes.outputs.code == 'true'
        run: task install

      - name: Generate code
        if: steps.changes.outputs.code == 'true'
        run: task generate

      - name: Lint
        if: steps.changes.outputs.code == 'true'
        uses: golangci/golangci-lint-action@v8
        with:
          version: latest
          github-token: ${{ secrets.GITHUB_TOKEN }}
          only-new-issues: ${{ github.event_name == 'pull_request' }}
          working-directory: ./goforms

      - name: Run tests
        if: steps.changes.outputs.code == 'true'
        run: task test

  build:
    name: Build Application
    needs: test
    runs-on: ubuntu-latest
    timeout-minutes: 15
    defaults:
      run:
        working-directory: ./goforms
    if: |
      needs.test.outputs.should-build == 'true' &&
      (github.ref == 'refs/heads/main' || startsWith(github.ref, 'refs/tags/v'))
    outputs:
      binary-artifact: goformx-binary
      version: ${{ steps.version.outputs.version }}
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4
        with:
          fetch-depth: 1

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: ${{ env.GO_VERSION_FILE }}
          cache: true
          cache-dependency-path: goforms/go.sum

      - name: Install Task
        uses: arduino/setup-task@v2
        with:
          version: ${{ env.TASK_VERSION }}
          repo-token: ${{ secrets.GITHUB_TOKEN }}

      - name: Restore dependencies cache
        uses: actions/cache@v4
        with:
          path: |
            ~/go/bin
            ~/.cache/go-build
            ~/go/pkg/mod
          key: ${{ runner.os }}-go-deps-${{ hashFiles('goforms/go.mod', 'goforms/go.sum') }}
          restore-keys: |
            ${{ runner.os }}-go-deps-

      - name: Get version
        id: version
        run: |
          if [[ $GITHUB_REF == refs/tags/* ]]; then
            VERSION=${GITHUB_REF#refs/tags/}
          else
            VERSION=main-${GITHUB_SHA::8}
          fi
          echo "version=$VERSION" >> $GITHUB_OUTPUT
          echo "Building version: $VERSION"

      - name: Build application
        run: |
          task install
          task generate
          mkdir -p bin
          task build:backend

      - name: Upload binary artifact
        uses: actions/upload-artifact@v4
        with:
          name: goformx-binary
          path: |
            goforms/bin/
            goforms/migrations/
          retention-days: 7

  docker:
    name: Build Docker Image
    needs: [test, build]
    runs-on: ubuntu-latest
    timeout-minutes: 20
    if: |
      needs.test.outputs.should-build == 'true' &&
      (github.ref == 'refs/heads/main' || startsWith(github.ref, 'refs/tags/v'))
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4
        with:
          fetch-depth: 1

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to Container Registry
        if: github.event_name != 'pull_request'
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Extract metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
          tags: |
            type=ref,event=branch
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=sha,prefix={{branch}}-,format=short
            type=raw,value=latest,enable={{is_default_branch}}

      - name: Get build info
        id: build-info
        run: |
          if [[ $GITHUB_REF == refs/tags/* ]]; then
            VERSION=${GITHUB_REF#refs/tags/}
          else
            VERSION=main-${GITHUB_SHA::8}
          fi
          BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
          echo "version=$VERSION" >> $GITHUB_OUTPUT
          echo "build-time=$BUILD_TIME" >> $GITHUB_OUTPUT
          echo "git-commit=$GITHUB_SHA" >> $GITHUB_OUTPUT

      - name: Build and push Docker image
        id: push
        uses: docker/build-push-action@v6
        with:
          context: ./goforms
          file: ./goforms/docker/production/Dockerfile
          push: ${{ github.event_name != 'pull_request' }}
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          build-args: |
            VERSION=${{ steps.build-info.outputs.version }}
            BUILD_TIME=${{ steps.build-info.outputs.build-time }}
            GIT_COMMIT=${{ steps.build-info.outputs.git-commit }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
          platforms: linux/amd64,linux/arm64

      - name: Generate attestation
        if: github.event_name != 'pull_request'
        uses: actions/attest-build-provenance@v2
        with:
          subject-name: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
          subject-digest: ${{ steps.push.outputs.digest }}
          push-to-registry: true

  release:
    name: Create Release
    needs: [build, docker]
    runs-on: ubuntu-latest
    timeout-minutes: 10
    if: startsWith(github.ref, 'refs/tags/v')
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4
        with:
          fetch-depth: 1

      - name: Download build artifacts
        uses: actions/download-artifact@v4
        with:
          name: goformx-binary
          path: release/

      - name: Create release archive
        run: |
          cd release
          tar -czf ../goformx-${{ needs.build.outputs.version }}.tar.gz .

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          files: goformx-${{ needs.build.outputs.version }}.tar.gz
          draft: false
          prerelease: ${{ contains(needs.build.outputs.version, '-') }}
          generate_release_notes: true
          make_latest: ${{ !contains(needs.build.outputs.version, '-') }}
```

**Step 2: Stage and commit**

```bash
git add .github/workflows/goforms-ci.yml
git commit -m "ci: add path-filtered Go CI/CD workflow for monorepo"
```

---

### Task 6: Create CI Workflow — laravel-ci.yml

**Files:**
- Create: `.github/workflows/laravel-ci.yml`

**Step 1: Create the Laravel test workflow**

```yaml
name: "Laravel Tests"

on:
  push:
    branches: [main]
    paths:
      - 'goformx-laravel/**'
      - '.github/workflows/laravel-ci.yml'
  pull_request:
    branches: [main]
    paths:
      - 'goformx-laravel/**'
      - '.github/workflows/laravel-ci.yml'

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

jobs:
  ci:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: ./goformx-laravel
    strategy:
      matrix:
        php-version: ['8.4', '8.5']

    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Setup PHP
        uses: shivammathur/setup-php@v2
        with:
          php-version: ${{ matrix.php-version }}
          tools: composer:v2
          coverage: xdebug

      - name: Setup Node
        uses: actions/setup-node@v4
        with:
          node-version: '22'

      - name: Install Node Dependencies
        run: npm i

      - name: Install Dependencies
        run: composer install --no-interaction --prefer-dist --optimize-autoloader

      - name: Copy Environment File
        run: cp .env.example .env

      - name: Generate Application Key
        run: php artisan key:generate

      - name: Build Assets
        run: npm run build

      - name: Tests
        run: ./vendor/bin/pest
```

**Step 2: Stage and commit**

```bash
git add .github/workflows/laravel-ci.yml
git commit -m "ci: add path-filtered Laravel test workflow for monorepo"
```

---

### Task 7: Create CI Workflow — laravel-lint.yml

**Files:**
- Create: `.github/workflows/laravel-lint.yml`

**Step 1: Create the Laravel lint workflow**

```yaml
name: "Laravel Linter"

on:
  push:
    branches: [main]
    paths:
      - 'goformx-laravel/**'
      - '.github/workflows/laravel-lint.yml'
  pull_request:
    branches: [main]
    paths:
      - 'goformx-laravel/**'
      - '.github/workflows/laravel-lint.yml'

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

permissions:
  contents: write

jobs:
  quality:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: ./goformx-laravel
    steps:
      - uses: actions/checkout@v4

      - name: Setup PHP
        uses: shivammathur/setup-php@v2
        with:
          php-version: '8.4'

      - name: Install Dependencies
        run: |
          composer install -q --no-ansi --no-interaction --no-scripts --no-progress --prefer-dist
          npm install

      - name: Run Pint
        run: composer lint

      - name: Format Frontend
        run: npm run format

      - name: Lint Frontend
        run: npm run lint
```

**Step 2: Stage and commit**

```bash
git add .github/workflows/laravel-lint.yml
git commit -m "ci: add path-filtered Laravel lint workflow for monorepo"
```

---

### Task 8: Create CI Workflow — security.yml

**Files:**
- Create: `.github/workflows/security.yml`
- Create: `.github/codeql-config.yml`

**Step 1: Create the CodeQL config**

```yaml
# .github/codeql-config.yml
name: "Custom CodeQL Config"

disable-default-queries: false

queries:
- uses: security-and-quality

paths-ignore:
- "**/*.md"
- "**/docs/**"
- "**/*.txt"
- "**/testdata/**"
- "**/*_test.go"
- "**/test/**"
- "**/tests/**"
- "**/node_modules/**"
- "**/dist/**"
- "**/build/**"
- "**/vendor/**"

query-filters:
- exclude:
    id: go/path-injection
- exclude:
    id: js/incomplete-sanitization
```

**Step 2: Create the security workflow**

```yaml
# security.yml - Weekly security scans
name: Security Scan

on:
  schedule:
    - cron: '30 1 * * 0'
  workflow_dispatch:
  push:
    branches: [main]
    paths:
      - '.github/workflows/security.yml'
      - '.github/codeql-config.yml'

permissions:
  actions: read
  contents: read
  security-events: write

jobs:
  codeql:
    name: CodeQL Analysis
    runs-on: ubuntu-latest
    timeout-minutes: 30
    strategy:
      fail-fast: false
      matrix:
        include:
          - language: go
            working-directory: ./goforms
          - language: javascript
            working-directory: ./goformx-laravel

    steps:
      - name: Checkout repository
        uses: actions/checkout@v4
        with:
          fetch-depth: 1

      - name: Set up Go
        if: matrix.language == 'go'
        uses: actions/setup-go@v5
        with:
          go-version-file: 'goforms/go.mod'
          cache: true

      - name: Set up Node.js
        if: matrix.language == 'javascript'
        uses: actions/setup-node@v4
        with:
          node-version: '22'

      - name: Install Task
        if: matrix.language == 'go'
        uses: arduino/setup-task@v2
        with:
          version: '3.x'
          repo-token: ${{ secrets.GITHUB_TOKEN }}

      - name: Prepare Go environment
        if: matrix.language == 'go'
        working-directory: ./goforms
        run: |
          task install:go-tools
          task generate

      - name: Prepare JavaScript environment
        if: matrix.language == 'javascript'
        working-directory: ./goformx-laravel
        run: |
          npm ci
          npm run build

      - name: Initialize CodeQL
        uses: github/codeql-action/init@v3
        with:
          languages: ${{ matrix.language }}
          queries: security-extended
          config-file: ./.github/codeql-config.yml

      - name: Perform CodeQL Analysis
        uses: github/codeql-action/analyze@v3
        with:
          category: "/language:${{ matrix.language }}"

  dependency-review:
    name: Dependency Review
    runs-on: ubuntu-latest
    if: github.event_name == 'workflow_dispatch'
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Dependency Review
        uses: actions/dependency-review-action@v4
        with:
          fail-on-severity: moderate
          allow-licenses: MIT, Apache-2.0, BSD-2-Clause, BSD-3-Clause, ISC
```

**Step 3: Stage and commit**

```bash
git add .github/codeql-config.yml .github/workflows/security.yml
git commit -m "ci: add security scanning workflow for monorepo"
```

---

### Task 9: Create CI Workflow — Claude workflows

**Files:**
- Create: `.github/workflows/claude.yml`
- Create: `.github/workflows/claude-code-review.yml`

**Step 1: Create claude.yml**

```yaml
name: Claude Code

on:
  issue_comment:
    types: [created]
  pull_request_review_comment:
    types: [created]
  issues:
    types: [opened, assigned]
  pull_request_review:
    types: [submitted]

jobs:
  claude:
    if: |
      (github.event_name == 'issue_comment' && contains(github.event.comment.body, '@claude')) ||
      (github.event_name == 'pull_request_review_comment' && contains(github.event.comment.body, '@claude')) ||
      (github.event_name == 'pull_request_review' && contains(github.event.review.body, '@claude')) ||
      (github.event_name == 'issues' && (contains(github.event.issue.body, '@claude') || contains(github.event.issue.title, '@claude')))
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: read
      issues: read
      id-token: write
      actions: read
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4
        with:
          fetch-depth: 1

      - name: Run Claude Code
        id: claude
        uses: anthropics/claude-code-action@v1
        with:
          claude_code_oauth_token: ${{ secrets.CLAUDE_CODE_OAUTH_TOKEN }}
          additional_permissions: |
            actions: read
```

**Step 2: Create claude-code-review.yml**

```yaml
name: Claude Code Review

on:
  pull_request:
    types: [opened, synchronize, ready_for_review, reopened]

jobs:
  claude-review:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: read
      issues: read
      id-token: write

    steps:
      - name: Checkout repository
        uses: actions/checkout@v4
        with:
          fetch-depth: 1

      - name: Run Claude Code Review
        id: claude-review
        uses: anthropics/claude-code-action@v1
        with:
          claude_code_oauth_token: ${{ secrets.CLAUDE_CODE_OAUTH_TOKEN }}
          plugin_marketplaces: 'https://github.com/anthropics/claude-code.git'
          plugins: 'code-review@claude-code-plugins'
          prompt: '/code-review:code-review ${{ github.repository }}/pull/${{ github.event.pull_request.number }}'
```

**Step 3: Stage and commit**

```bash
git add .github/workflows/claude.yml .github/workflows/claude-code-review.yml
git commit -m "ci: add Claude Code workflows for monorepo"
```

---

### Task 10: Create Dependabot Config

**Files:**
- Create: `.github/dependabot.yml`

**Step 1: Create dependabot.yml**

```yaml
version: 2
updates:
  - package-ecosystem: "gomod"
    directory: "/goforms"
    schedule:
      interval: weekly

  - package-ecosystem: "composer"
    directory: "/goformx-laravel"
    schedule:
      interval: weekly

  - package-ecosystem: "npm"
    directory: "/goformx-laravel"
    schedule:
      interval: weekly

  - package-ecosystem: "github-actions"
    directory: "/"
    schedule:
      interval: weekly
```

**Step 2: Stage and commit**

```bash
git add .github/dependabot.yml
git commit -m "ci: add dependabot config for all package ecosystems"
```

---

### Task 11: Remove Per-Service .github/ Directories

**Files:**
- Remove: `goforms/.github/` (entire directory)
- Remove: `goformx-laravel/.github/` (entire directory)

**Step 1: Remove goforms .github/**

```bash
git rm -r goforms/.github/
```

**Step 2: Remove goformx-laravel .github/**

```bash
git rm -r goformx-laravel/.github/
```

**Step 3: Commit**

```bash
git commit -m "chore: remove per-service .github/ dirs (workflows now at monorepo root)"
```

---

### Task 12: Update Root CLAUDE.md

**Files:**
- Modify: `CLAUDE.md` (copy from current `/home/fsd42/dev/goformx/CLAUDE.md` into the monorepo, updating wording)

**Step 1: Copy and update CLAUDE.md**

Copy the existing `/home/fsd42/dev/goformx/CLAUDE.md` into the monorepo root. Update the opening paragraph to say "monorepo" instead of implying separate directories. The rest of the content is already accurate.

Change the Project Overview section from:

> GoFormX is a forms management platform split into two independent services in this workspace:

To:

> GoFormX is a forms management platform organized as a monorepo with two services:

**Step 2: Stage and commit**

```bash
git add CLAUDE.md
git commit -m "docs: add root CLAUDE.md for monorepo"
```

---

### Task 13: Copy Design Docs

**Files:**
- Create: `docs/plans/2026-02-19-monorepo-conversion-design.md`

**Step 1: Copy the design doc into the monorepo**

Copy from `/home/fsd42/dev/goformx/docs/plans/2026-02-19-monorepo-conversion-design.md` into the monorepo at the same path.

**Step 2: Stage and commit**

```bash
git add docs/
git commit -m "docs: add monorepo conversion design doc"
```

---

### Task 14: Verify Everything Works

**Step 1: Verify git history for goforms**

```bash
git log --oneline --follow goforms/main.go | wc -l
```

Expected: A number greater than 0 (commits touching main.go from original repo).

**Step 2: Verify git history for laravel**

```bash
git log --oneline --follow goformx-laravel/artisan | wc -l
```

Expected: A number greater than 0.

**Step 3: Verify directory structure**

```bash
ls -la goforms/main.go goforms/Taskfile.yml goforms/go.mod
ls -la goformx-laravel/artisan goformx-laravel/composer.json
ls -la .github/workflows/goforms-ci.yml .github/workflows/laravel-ci.yml
ls -la Taskfile.yml .gitignore .env.example CLAUDE.md
```

Expected: All files exist.

**Step 4: Verify no per-service .github/ dirs remain**

```bash
ls goforms/.github/ 2>&1
ls goformx-laravel/.github/ 2>&1
```

Expected: Both return "No such file or directory".

**Step 5: Verify root Taskfile works**

```bash
task --list
```

Expected: Shows root tasks (dev, install, test, lint, setup) plus `go:*` namespace tasks.

**Step 6: Verify commit log is clean**

```bash
git log --oneline -15
```

Expected: Shows the monorepo setup commits at the top, followed by the subtree merge commits.

---

### Task 15: Push to GitHub

**Step 1: Confirm with user before pushing**

Ask user to confirm they are ready to push.

**Step 2: Push to origin**

```bash
git push -u origin main
```

Expected: Pushes all commits (including full history from both repos) to `goformx/goformx`.

**Step 3: Verify on GitHub**

Open `https://github.com/goformx/goformx` and verify:
- Both subdirectories are visible
- Commit history is present
- CI workflows appear under Actions tab

---

### Task 16: Archive Old Repos

**Prereq:** User confirms everything works in the monorepo.

**Step 1: Archive goformx/goforms**

```bash
gh repo archive goformx/goforms --yes
```

**Step 2: Archive goformx/goformx-laravel**

```bash
gh repo archive goformx/goformx-laravel --yes
```

**Step 3: Verify archives**

```bash
gh repo view goformx/goforms --json isArchived
gh repo view goformx/goformx-laravel --json isArchived
```

Expected: Both show `"isArchived": true`.

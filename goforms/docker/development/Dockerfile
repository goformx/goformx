# ------------------------------------------------------------
# Base stage — shared system dependencies
# ------------------------------------------------------------
FROM golang:1.25-alpine AS base

RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    wget \
    go-task-task \
    curl \
    dumb-init

WORKDIR /app
ENV PATH="/go/bin:/root/go/bin:/usr/local/bin:${PATH}"


# ------------------------------------------------------------
# Dev stage — full tooling + Air + npm run dev
# ------------------------------------------------------------
FROM base AS dev

# Install Go tooling
RUN go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest \
    && go install go.uber.org/mock/mockgen@latest \
    && go install github.com/air-verse/air@latest \
    && go install github.com/bombsimon/wsl/v4/cmd/wsl@latest \
    && go install golang.org/x/tools/gopls@latest \
    && go install github.com/go-delve/delve/cmd/dlv@latest \
    && go install golang.org/x/tools/cmd/goimports@latest \
    && go install github.com/securego/gosec/v2/cmd/gosec@latest \
    && go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Create runtime dirs
RUN mkdir -p logs

# Note: Source code is mounted via bind mount in docker-compose.yml
# No need to COPY source here - bind mount provides live code for development

EXPOSE 8090 5173

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8090/health || exit 1

# Run Air for Go hot-reload (frontend lives in goformx-laravel)
# Configure git safe directory for bind-mounted repos, then start
CMD ["dumb-init", "--", "sh", "-c", "git config --global --add safe.directory /app && air -c .air.toml"]

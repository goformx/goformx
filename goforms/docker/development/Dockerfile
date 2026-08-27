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
RUN go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 \
    && go install github.com/air-verse/air@v1.67.4 \
    && go install github.com/bombsimon/wsl/v4/cmd/wsl@v4.7.0 \
    && go install golang.org/x/tools/gopls@v0.23.0 \
    && go install github.com/go-delve/delve/cmd/dlv@v1.27.1 \
    && go install golang.org/x/tools/cmd/goimports@v0.49.0 \
    && go install github.com/securego/gosec/v2/cmd/gosec@v2.29.0 \
    && go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1

# Create runtime dirs
RUN mkdir -p logs

# Note: Source code is mounted via bind mount in docker-compose.yml
# No need to COPY source here - bind mount provides live code for development

EXPOSE 8090 5173

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8090/health || exit 1

# Configure git safe directory for bind-mounted repos, then start Air.
CMD ["dumb-init", "--", "sh", "-c", "git config --global --add safe.directory /app && air -c .air.toml"]

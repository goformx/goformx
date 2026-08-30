# Go build stage
FROM golang:1.26.7-alpine@sha256:28d89ee9cc0ff9fec75c82ca201e6bf7fdf9a679d4b7b24dfa04f2bb766bb468 AS go-builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Define build arguments
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT=unknown
ARG GO_VERSION

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Generate code artifacts (mocks)
RUN go generate ./...

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X github.com/goformx/goforms/internal/infrastructure/version.Version=${VERSION} -X github.com/goformx/goforms/internal/infrastructure/version.BuildTime=${BUILD_TIME} -X github.com/goformx/goforms/internal/infrastructure/version.GitCommit=${GIT_COMMIT} -X github.com/goformx/goforms/internal/infrastructure/version.GoVersion=${GO_VERSION}" \
    -o bin/goforms ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o bin/goformx-webhook-keys ./cmd/goformx-webhook-keys
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o bin/goformx-token ./cmd/goformx-token

# Shared non-root runtime; no application or maintenance executables.
FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b AS runtime

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata wget

# Create non-root user
RUN addgroup -g 1001 -S goforms && \
    adduser -u 1001 -S goforms -G goforms

WORKDIR /app

# Maintenance is opt-in and never inherits the API command or health check.
FROM runtime AS maintenance
COPY --from=go-builder /app/bin/goformx-webhook-keys ./bin/goformx-webhook-keys
COPY --from=go-builder /app/bin/goformx-token ./bin/goformx-token
USER goforms
CMD ["./bin/goformx-token"]

# Keep API last so builds without --target also produce the serving image.
FROM runtime AS api
COPY --from=go-builder /app/bin/goforms ./bin/goforms

# Switch to non-root user
USER goforms

# Expose port
EXPOSE 8090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8090/health || exit 1

# Run the application
CMD ["./bin/goforms"]

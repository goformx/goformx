# Go build stage
FROM golang:1.25-alpine AS go-builder

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

# Install build tools
RUN go install go.uber.org/mock/mockgen@latest

# Copy source code
COPY . .

# Generate code artifacts (mocks)
RUN go generate ./...

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X github.com/goformx/goforms/internal/infrastructure/version.Version=${VERSION} -X github.com/goformx/goforms/internal/infrastructure/version.BuildTime=${BUILD_TIME} -X github.com/goformx/goforms/internal/infrastructure/version.GitCommit=${GIT_COMMIT} -X github.com/goformx/goforms/internal/infrastructure/version.GoVersion=${GO_VERSION}" \
    -o bin/goforms main.go

# Production stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata wget

# Create non-root user
RUN addgroup -g 1001 -S goforms && \
    adduser -u 1001 -S goforms -G goforms

WORKDIR /app

# Copy binary from builder
COPY --from=go-builder /app/bin/goforms ./bin/goforms

# Create necessary directories
RUN mkdir -p logs && \
    chown -R goforms:goforms /app

# Switch to non-root user
USER goforms

# Expose port
EXPOSE 8090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8090/health || exit 1

# Run the application
CMD ["./bin/goforms"]

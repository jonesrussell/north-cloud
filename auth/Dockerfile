FROM golang:1.25-alpine AS builder

# Copy infrastructure module to parent directory (required for go.mod replace directive)
# The replace directive uses ../infrastructure, so we need infrastructure at /build/infrastructure
WORKDIR /build
COPY infrastructure/ ./infrastructure/

# Now work in auth subdirectory to match the replace directive path
WORKDIR /build/auth

# Copy go mod files
COPY auth/go.mod auth/go.sum ./
RUN go mod download

# Copy auth source code
COPY auth/ .
RUN go build -o auth main.go

FROM alpine:latest

# Install runtime dependencies and create non-root user
RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy binary and config from builder
COPY --from=builder /build/auth/auth .
COPY --from=builder /build/auth/config.yml .

# Change ownership to non-root user
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

EXPOSE 8040

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8040/health || exit 1

CMD ["./auth", "-config", "config.yml"]


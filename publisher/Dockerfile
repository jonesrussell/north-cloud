FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Copy infrastructure module to parent directory (required for go.mod replace directive)
# The replace directive uses ../infrastructure, so we need infrastructure at /build/infrastructure
WORKDIR /build
COPY infrastructure/ ./infrastructure/

# Now work in publisher subdirectory to match the replace directive path
WORKDIR /build/publisher

# Copy go mod files
COPY publisher/go.mod publisher/go.sum ./
RUN go mod download

# Copy publisher source code
COPY publisher/ .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o integration ./main.go

# Final stage
FROM alpine:latest

# Install runtime dependencies and create non-root user
RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/publisher/integration .

# Copy config file (if exists)
COPY --from=builder /build/publisher/config.yml* ./

# Change ownership to non-root user
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

EXPOSE 8080

# Health check (adjust endpoint based on your service)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./integration", "-config", "config.yml"]

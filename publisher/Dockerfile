FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy infrastructure module to parent directory (required for go.mod replace directive)
WORKDIR /build
COPY infrastructure/ ./infrastructure/

# Now work in publisher subdirectory to match the replace directive path
WORKDIR /build/publisher

# Copy go mod files
COPY publisher/go.mod publisher/go.sum ./
RUN go mod download

# Copy publisher source code
COPY publisher/ .

# Build the application (single binary with api and router commands)
# Use -mod=mod to ignore vendor directory (vendor may be out of sync)
RUN CGO_ENABLED=0 GOOS=linux go build -mod=mod -a -installsuffix cgo -o publisher .

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata curl && \
    addgroup -g 1000 publisher && \
    adduser -D -u 1000 -G publisher publisher

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/publisher/publisher .

# Change ownership to non-root user
RUN chown -R publisher:publisher /app

# Switch to non-root user
USER publisher

# Default to API server (can be overridden in docker-compose)
CMD ["/app/publisher", "api"]

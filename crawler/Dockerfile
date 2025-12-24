FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Copy infrastructure module to parent directory (required for go.mod replace directive)
# The replace directive uses ../infrastructure, so we need infrastructure at /build/infrastructure
WORKDIR /build
COPY infrastructure/ ./infrastructure/

# Now work in crawler subdirectory to match the replace directive path
WORKDIR /build/crawler

# Copy go mod files
COPY crawler/go.mod crawler/go.sum ./
RUN go mod download

# Copy crawler source code
COPY crawler/ .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o crawler ./main.go

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/crawler/crawler .

# Copy config file (if exists)
COPY --from=builder /build/crawler/config.yaml* ./

# Change ownership to non-root user
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Default command - can be overridden in docker-compose
# When volume is mounted, it will use bin/crawler if available
CMD ["./crawler"]


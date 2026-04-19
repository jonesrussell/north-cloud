# Build stage
FROM golang:1.26.2-alpine AS builder

RUN apk add --no-cache ca-certificates

# Prepare workspace for replace ../infrastructure
WORKDIR /build
COPY infrastructure ./infrastructure

# Move into source-manager module
WORKDIR /build/source-manager

# Copy go mod files first for caching
COPY source-manager/go.mod source-manager/go.sum ./
RUN go mod download

# Copy source
COPY source-manager/ .

# Build static binaries
RUN CGO_ENABLED=0 go build \
    -ldflags="-w -s" \
    -trimpath \
    -o source-manager \
    .

RUN CGO_ENABLED=0 go build \
    -ldflags="-w -s" \
    -trimpath \
    -o seed-communities \
    ./cmd/seed-communities

RUN CGO_ENABLED=0 go build \
    -ldflags="-w -s" \
    -trimpath \
    -o seed-municipalities \
    ./cmd/seed-municipalities

RUN CGO_ENABLED=0 go build \
    -ldflags="-w -s" \
    -trimpath \
    -o refresh-communities \
    ./cmd/refresh-communities

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata wget && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy binaries and config
COPY --from=builder /build/source-manager/source-manager .
COPY --from=builder /build/source-manager/seed-communities .
COPY --from=builder /build/source-manager/seed-municipalities .
COPY --from=builder /build/source-manager/refresh-communities .
COPY --from=builder /build/source-manager/config.yml.example ./config.yml

# Switch to non-root user
USER appuser

EXPOSE 8050

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8050/health || exit 1

CMD ["./source-manager", "-config", "config.yml"]

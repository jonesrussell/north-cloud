# Build stage
FROM golang:1.26.0-alpine AS builder

RUN apk add --no-cache ca-certificates

# Prepare workspace for replace directive (../infrastructure)
WORKDIR /build
COPY infrastructure ./infrastructure

# Move into publisher module
WORKDIR /build/publisher

# Copy go mod files first for caching
COPY publisher/go.mod publisher/go.sum ./
RUN go mod download

# Copy source
COPY publisher/ .

# Build static binary (single binary with commands)
RUN CGO_ENABLED=0 go build \
    -mod=mod \
    -ldflags="-w -s" \
    -trimpath \
    -o publisher \
    .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata curl && \
    addgroup -g 1000 publisher && \
    adduser -D -u 1000 -G publisher publisher

WORKDIR /app

# Copy binary and config
COPY --from=builder /build/publisher/publisher .
COPY --from=builder /build/publisher/config.yml.example ./config.yml

# Switch to non-root user
USER publisher

EXPOSE 8070

# Default to API server
CMD ["/app/publisher", "api"]

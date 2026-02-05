# Build stage
FROM golang:1.25.7-alpine AS builder

RUN apk add --no-cache ca-certificates

# Prepare workspace for replace ../infrastructure
WORKDIR /build
COPY infrastructure ./infrastructure

# Move into search module
WORKDIR /build/search

# Copy go mod files first for caching
COPY search/go.mod search/go.sum ./
RUN go mod download

# Copy source
COPY search/ .

# Build static binary
RUN CGO_ENABLED=0 go build \
    -ldflags="-w -s" \
    -trimpath \
    -o search \
    main.go

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates wget

WORKDIR /root

# Copy binary
COPY --from=builder /build/search/search .

# Copy config
COPY --from=builder /build/search/config.yml.example ./config.yml

EXPOSE 8090

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8090/health || exit 1

CMD ["./search"]

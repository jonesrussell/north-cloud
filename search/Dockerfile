# Production Dockerfile for Search Service
# Multi-stage build for smaller image size

# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy infrastructure module to parent directory
WORKDIR /build
COPY infrastructure/ ./infrastructure/

# Work in search subdirectory
WORKDIR /build/search

# Copy go mod files
COPY search/go.mod search/go.sum ./
RUN go mod download

# Copy search source code
COPY search/ .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o search cmd/httpd/main.go

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /build/search/search .

# Copy config template
COPY --from=builder /build/search/config.yml.example ./config.yml

EXPOSE 8090

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8090/health || exit 1

# Start service
CMD ["./search"]

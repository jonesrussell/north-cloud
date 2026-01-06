# Build stage
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache ca-certificates

# Prepare workspace for replace ../infrastructure
WORKDIR /build
COPY infrastructure ./infrastructure

# Move into index-manager module
WORKDIR /build/index-manager

# Copy go mod files first for caching
COPY index-manager/go.mod index-manager/go.sum ./
RUN go mod download

# Copy source
COPY index-manager/ .

# Build static binary
RUN CGO_ENABLED=0 go build \
    -ldflags="-w -s" \
    -trimpath \
    -o /index-manager \
    cmd/httpd/main.go

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root

# Copy binary
COPY --from=builder /index-manager .

# Copy migrations + config
COPY --from=builder /build/index-manager/migrations ./migrations
COPY --from=builder /build/index-manager/config.yml.example ./config.yml

EXPOSE 8090

CMD ["./index-manager"]

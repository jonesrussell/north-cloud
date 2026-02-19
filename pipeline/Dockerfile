# Build stage
FROM golang:1.26.0-alpine AS builder

RUN apk add --no-cache ca-certificates

# Prepare workspace for replace directive (../infrastructure)
WORKDIR /build
COPY infrastructure ./infrastructure

# Move into pipeline module
WORKDIR /build/pipeline

# Copy go mod files first for caching
COPY pipeline/go.mod pipeline/go.sum ./
RUN go mod download

# Copy source
COPY pipeline/ .

# Build static binary
RUN CGO_ENABLED=0 go build \
    -mod=mod \
    -ldflags="-w -s" \
    -trimpath \
    -o pipeline \
    .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata curl wget && \
    addgroup -g 1000 pipeline && \
    adduser -D -u 1000 -G pipeline pipeline

WORKDIR /app

# Copy binary and config
COPY --from=builder /build/pipeline/pipeline .
COPY --from=builder /build/pipeline/config.yml.example ./config.yml

# Switch to non-root user
USER pipeline

EXPOSE 8075

CMD ["/app/pipeline"]

FROM golang:1.25-alpine AS builder

# Only install git if you pull private modules
RUN apk add --no-cache ca-certificates

# Prepare workspace for replace ../infrastructure
WORKDIR /build
COPY infrastructure ./infrastructure

# Move into crawler module
WORKDIR /build/crawler

# Copy go mod files first for caching
COPY crawler/go.mod crawler/go.sum ./
RUN go mod download

# Copy source
COPY crawler/ .

# Build static binary
RUN CGO_ENABLED=0 go build \
    -ldflags="-w -s" \
    -trimpath \
    -o crawler \
    main.go

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy binary
COPY --from=builder /build/crawler/crawler .

# Copy config
COPY --from=builder /build/crawler/config.example.yaml ./config.yaml

# Switch to non-root user
USER appuser

EXPOSE 8060

CMD ["./crawler"]

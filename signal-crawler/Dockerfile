# Build stage
FROM golang:1.26.1-alpine AS builder

RUN apk add --no-cache ca-certificates gcc musl-dev

# Prepare workspace for replace directive (../infrastructure)
WORKDIR /build
COPY infrastructure ./infrastructure

# Move into signal-crawler module
WORKDIR /build/signal-crawler

# Copy go mod files first for caching
COPY signal-crawler/go.mod signal-crawler/go.sum ./
RUN go mod download

# Copy source
COPY signal-crawler/ .

# Build binary (CGO_ENABLED=1 for SQLite via mattn/go-sqlite3)
RUN CGO_ENABLED=1 go build \
    -ldflags="-w -s" \
    -trimpath \
    -o signal-crawler \
    main.go

# Runtime stage
FROM alpine:3.21

RUN apk --no-cache add ca-certificates sqlite-libs

WORKDIR /app

COPY --from=builder /build/signal-crawler/signal-crawler .
COPY --from=builder /build/signal-crawler/config.yml .

RUN addgroup -g 1000 signal-crawler && \
    adduser -D -u 1000 -G signal-crawler signal-crawler
RUN chown -R signal-crawler:signal-crawler /app
USER signal-crawler

ENTRYPOINT ["./signal-crawler"]

# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o nc-http-proxy .

# Runtime stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/nc-http-proxy .

# Create directories
RUN mkdir -p /app/fixtures /app/cache /app/certs

EXPOSE 8055

ENV PROXY_PORT=8055
ENV PROXY_MODE=replay
ENV PROXY_FIXTURES_DIR=/app/fixtures
ENV PROXY_CACHE_DIR=/app/cache
ENV PROXY_CERT_FILE=/app/certs/proxy.crt
ENV PROXY_KEY_FILE=/app/certs/proxy.key

HEALTHCHECK --interval=10s --timeout=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8055/health || exit 1

ENTRYPOINT ["./nc-http-proxy"]

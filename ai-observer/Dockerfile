# Build stage
FROM golang:1.26.0-alpine AS builder

RUN apk add --no-cache ca-certificates

# Prepare workspace for replace directive (../infrastructure)
WORKDIR /build
COPY infrastructure ./infrastructure

# Move into ai-observer module
WORKDIR /build/ai-observer

# Copy go mod files first for caching
COPY ai-observer/go.mod ai-observer/go.sum ./
RUN go mod download

# Copy source
COPY ai-observer/ .

# Build static binary
RUN CGO_ENABLED=0 go build \
    -mod=mod \
    -ldflags="-w -s" \
    -trimpath \
    -o ai-observer \
    .

# Runtime stage
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -g 1000 ai-observer && \
    adduser -D -u 1000 -G ai-observer ai-observer

WORKDIR /app

COPY --from=builder /build/ai-observer/ai-observer .

USER ai-observer

CMD ["/app/ai-observer"]

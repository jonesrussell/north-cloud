# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install only what is needed
RUN apk add --no-cache ca-certificates

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

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
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/config.yml.example ./config.yml

EXPOSE 8090

CMD ["./index-manager"]

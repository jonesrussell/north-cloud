FROM golang:1.25.7-alpine AS builder

# Prepare workspace for replace ../infrastructure
WORKDIR /build
COPY infrastructure ./infrastructure

# Move into auth module
WORKDIR /build/auth

# Copy go mod files first for caching
COPY auth/go.mod auth/go.sum ./
RUN go mod download

# Copy source
COPY auth/ .

# Build static binary
RUN CGO_ENABLED=0 go build \
    -ldflags="-w -s" \
    -trimpath \
    -o /auth \
    main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /auth .
COPY --from=builder /build/auth/config.yml.example ./config.yml

EXPOSE 8040

CMD ["./auth"]

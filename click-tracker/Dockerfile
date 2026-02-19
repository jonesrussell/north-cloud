FROM golang:1.25.7-alpine AS builder

# Prepare workspace for replace ../infrastructure
WORKDIR /build
COPY infrastructure ./infrastructure

# Move into click-tracker module
WORKDIR /build/click-tracker

# Copy go mod files first for caching
COPY click-tracker/go.mod click-tracker/go.sum ./
RUN go mod download

# Copy source
COPY click-tracker/ .

# Build static binary
RUN CGO_ENABLED=0 go build \
    -ldflags="-w -s" \
    -trimpath \
    -o /click-tracker \
    main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /click-tracker .
COPY --from=builder /build/click-tracker/config.yml.example ./config.yml

EXPOSE 8093

CMD ["./click-tracker"]

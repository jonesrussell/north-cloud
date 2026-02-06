# Build stage
FROM golang:1.25.7-alpine AS builder

RUN apk add --no-cache ca-certificates

# Prepare workspace for replace directives (../infrastructure, ../index-manager)
WORKDIR /build
COPY infrastructure ./infrastructure
COPY index-manager ./index-manager

# Move into classifier module
WORKDIR /build/classifier

# Copy go mod files first for caching
COPY classifier/go.mod classifier/go.sum ./
RUN go mod download

# Copy source
COPY classifier/ .

# Build static binary
RUN CGO_ENABLED=0 go build \
    -ldflags="-w -s" \
    -trimpath \
    -o classifier \
    main.go

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root

COPY --from=builder /build/classifier/classifier .

COPY --from=builder /build/classifier/config.yml.example ./config.yml
COPY --from=builder /build/classifier/migrations ./migrations

EXPOSE 8070

CMD ["./classifier"]
FROM golang:1.25.7-alpine AS builder

RUN apk add --no-cache ca-certificates

# Prepare workspace for replace directive (../infrastructure)
WORKDIR /build
COPY infrastructure ./infrastructure

# Move into mcp-north-cloud module
WORKDIR /build/mcp-north-cloud

# Copy go mod files first for caching
COPY mcp-north-cloud/go.mod mcp-north-cloud/go.sum ./
RUN go mod download

# Copy source
COPY mcp-north-cloud/ .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -trimpath \
    -o /build/mcp-north-cloud/mcp-north-cloud \
    main.go

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /build/mcp-north-cloud/mcp-north-cloud /app/mcp-north-cloud

CMD ["/app/mcp-north-cloud"]

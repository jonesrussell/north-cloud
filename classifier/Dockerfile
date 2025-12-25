# Production Dockerfile for Classifier Service
# Multi-stage build for smaller image size

# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy infrastructure module to parent directory (required for go.mod replace directive)
# The replace directive uses ../infrastructure, so we need infrastructure at /build/infrastructure
WORKDIR /build
COPY infrastructure/ ./infrastructure/

# Now work in classifier subdirectory to match the replace directive path
WORKDIR /build/classifier

# Copy go mod files
COPY classifier/go.mod classifier/go.sum ./
RUN go mod download

# Copy classifier source code
COPY classifier/ .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o classifier main.go

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /build/classifier/classifier .

# Copy migrations
COPY --from=builder /build/classifier/migrations ./migrations

# Copy config template
COPY --from=builder /build/classifier/config.yml.example ./config.yml

EXPOSE 8070

# Default command - starts both HTTP server and processor concurrently
# Can be overridden to run only one service:
#   - ["./classifier", "httpd"]     # HTTP server only
#   - ["./classifier", "processor"] # Processor only
CMD ["./classifier"]

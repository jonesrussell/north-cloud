# Production Dockerfile for Classifier Service
# Multi-stage build for smaller image size

# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o /classifier main.go

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /classifier .

# Copy migrations
COPY --from=builder /app/migrations ./migrations

# Copy config template
COPY --from=builder /app/config.yml.example ./config.yml

EXPOSE 8070

CMD ["./classifier"]

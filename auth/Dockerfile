FROM golang:1.25-alpine AS builder

# Copy infrastructure module to parent directory (required for go.mod replace directive if needed)
WORKDIR /build
COPY infrastructure/ ./infrastructure/

# Now work in auth subdirectory
WORKDIR /build/auth

# Copy go mod files
COPY auth/go.mod auth/go.sum* ./
RUN go mod download

# Copy source code
COPY auth/ .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o bin/auth main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /build/auth/bin/auth .

EXPOSE 8040

CMD ["./auth"]

FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o gosources main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /build/gosources .
COPY --from=builder /build/config.yml .

EXPOSE 8050

CMD ["./gosources", "-config", "config.yml"]


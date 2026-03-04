FROM golang:1.26.0-alpine AS builder

RUN apk add --no-cache ca-certificates

WORKDIR /build
COPY infrastructure ./infrastructure

WORKDIR /build/rfp-ingestor
COPY rfp-ingestor/go.mod rfp-ingestor/go.sum ./
RUN go mod download

COPY rfp-ingestor/ .

RUN CGO_ENABLED=0 go build \
    -ldflags="-w -s" \
    -trimpath \
    -o /rfp-ingestor \
    main.go

FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /rfp-ingestor .
COPY --from=builder /build/rfp-ingestor/config.yml.example ./config.yml

USER appuser

EXPOSE 8095

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8095/health || exit 1

CMD ["./rfp-ingestor"]

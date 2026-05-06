# Build stage
# CGO_ENABLED=1 required for SQLite (catalogue) via mattn/go-sqlite3
FROM public.ecr.aws/docker/library/golang:1.26.2-alpine AS builder

RUN apk add --no-cache ca-certificates gcc musl-dev

# Prepare workspace for replace directives:
#   ../infrastructure  => /build/infrastructure
#   ../../indigenous-taxonomy => /indigenous-taxonomy (WP16 will add real imports;
#     until then the replace directive exists in go.mod but go.sum is empty and
#     no COPY is needed. WP19 replaces the replace directive with a real v1.1.0 pin,
#     at which point the COPY below can be removed entirely.)
WORKDIR /build
COPY infrastructure ./infrastructure
# COPY indigenous-taxonomy /indigenous-taxonomy  # uncomment when WP16 adds imports

# Move into alert-crawler module
WORKDIR /build/alert-crawler

# Copy go mod files first for layer caching
COPY alert-crawler/go.mod alert-crawler/go.sum ./
RUN go mod download

# Copy source
COPY alert-crawler/ .

# Build binary
RUN CGO_ENABLED=1 go build \
    -ldflags="-w -s" \
    -trimpath \
    -o alert-crawler \
    main.go

# Runtime stage
FROM public.ecr.aws/docker/library/alpine:3.21

RUN apk --no-cache add ca-certificates sqlite-libs

WORKDIR /app

COPY --from=builder /build/alert-crawler/alert-crawler .
COPY --from=builder /build/alert-crawler/config.yml .

# Non-root user uid/gid 1000 — matches PR-004 mitigation and Ansible volume-ownership convention.
# Host-mounted volumes (e.g. data/) must be owned by uid 1000 on the host.
# In Ansible, use: file: owner: "1000" (not deploy_user, which is uid 1001).
RUN addgroup -g 1000 alert-crawler && \
    adduser -D -u 1000 -G alert-crawler alert-crawler
RUN chown -R alert-crawler:alert-crawler /app
USER alert-crawler

ENTRYPOINT ["./alert-crawler"]

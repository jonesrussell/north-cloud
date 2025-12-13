ARG VARIANT=1.25-bookworm
FROM mcr.microsoft.com/devcontainers/go:${VARIANT}

# Install PostgreSQL client tools for database migrations and management
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y install --no-install-recommends postgresql-client \
    && apt-get clean -y && rm -rf /var/lib/apt/lists/*

# Fix permissions for Go module cache and sumdb so vscode user can write to them
# Create all necessary directories with proper ownership
RUN mkdir -p /go/pkg/mod/cache/download /go/pkg/sumdb/sum.golang.org && \
    chown -R vscode:vscode /go && \
    chmod -R u+w /go/pkg

# Install air for hot reloading in development
# Install as vscode user to ensure it's in the correct GOPATH/bin
USER vscode
# Create user's Go cache directory and set it as default
RUN mkdir -p /home/vscode/go/pkg/mod && \
    echo 'export GOMODCACHE=/home/vscode/go/pkg/mod' >> /home/vscode/.bashrc && \
    echo 'export GOMODCACHE=/home/vscode/go/pkg/mod' >> /home/vscode/.zshrc
RUN go install github.com/air-verse/air@latest
USER root

#!/bin/sh
# Wrapper script to ensure the MCP server binary exists before running
# IMPORTANT: All output except JSON must go to stderr for MCP stdio protocol

BINARY_PATH="/app/tmp/mcp-north-cloud"
SOURCE_PATH="/app/main.go"

# Build binary if it doesn't exist (silently, output to stderr only)
if [ ! -f "$BINARY_PATH" ]; then
    mkdir -p /app/tmp 2>/dev/null
    cd /app 2>/dev/null
    # Use /app/tmp for build cache to avoid permission issues
    # Redirect all output to stderr to avoid breaking JSON-RPC
    GOCACHE=/app/tmp/go-build-cache GOMODCACHE=/tmp/go-mod-cache go build -o "$BINARY_PATH" "$SOURCE_PATH" >&2
    if [ $? -ne 0 ]; then
        echo "Failed to build MCP server binary" >&2
        exit 1
    fi
fi

# Run the binary - this will handle stdin/stdout for JSON-RPC
exec "$BINARY_PATH"

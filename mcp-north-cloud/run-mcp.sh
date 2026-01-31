#!/usr/bin/env bash
set -e

# Resolve project root (parent of mcp-north-cloud/)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

# Load .env so AUTH_JWT_SECRET is available to the MCP
if [ -f .env ]; then
  set -a
  . .env
  set +a
fi

exec "$SCRIPT_DIR/bin/mcp-north-cloud" "$@"

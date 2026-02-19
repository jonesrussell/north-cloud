#!/usr/bin/env bash
set -e

# Resolve project root (parent of mcp-north-cloud/)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

# Load .env so AUTH_JWT_SECRET etc. are available to the MCP.
# Export each KEY=VALUE line; skip UID/GID (read-only in bash, only needed by Docker Compose).
if [ -f .env ]; then
  while IFS= read -r line || [ -n "$line" ]; do
    [[ "$line" =~ ^(UID|GID)= ]] && continue
    [[ "$line" =~ ^# ]] && continue
    [[ "$line" =~ ^[[:space:]]*$ ]] && continue
    [[ "$line" =~ ^[A-Za-z_][A-Za-z0-9_]*= ]] && export "$line"
  done < .env
fi

exec "$SCRIPT_DIR/bin/mcp-north-cloud" "$@"

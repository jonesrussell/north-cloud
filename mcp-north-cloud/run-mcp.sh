#!/usr/bin/env bash
set -e

# Resolve project root (parent of mcp-north-cloud/)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

# Load .env so AUTH_JWT_SECRET etc. are available to the MCP.
# Skip UID/GID: they are read-only in bash and only needed by Docker Compose.
if [ -f .env ]; then
  set -a
  tmp_loaded=""
  if tmp_loaded=$(mktemp 2>/dev/null); then
    grep -v '^UID=' .env | grep -v '^GID=' > "$tmp_loaded"
    . "$tmp_loaded"
    rm -f "$tmp_loaded"
  else
    while IFS= read -r line || [ -n "$line" ]; do
      [[ "$line" =~ ^(UID|GID)= ]] && continue
      [[ "$line" =~ ^#.*$ ]] && continue
      [[ "$line" =~ ^[[:space:]]*$ ]] && continue
      export "$line" 2>/dev/null || true
    done < .env
  fi
  set +a
fi

exec "$SCRIPT_DIR/bin/mcp-north-cloud" "$@"

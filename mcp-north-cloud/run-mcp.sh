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
    if [[ "$line" =~ ^([A-Za-z_][A-Za-z0-9_]*)= ]]; then
      var_name="${BASH_REMATCH[1]}"
      # Only export if not already set by the caller (e.g., .mcp.json env vars)
      [[ -z "${!var_name+x}" ]] && export "$line"
    fi
  done < .env
fi

exec "$SCRIPT_DIR/bin/mcp-north-cloud" "$@"

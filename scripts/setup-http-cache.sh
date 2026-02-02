#!/bin/bash
# Setup HTTP response cache directory for nc-http-proxy
# This creates the user-local cache directory used in record mode

set -e

CACHE_DIR="${NORTHCLOUD_HTTP_CACHE:-$HOME/.northcloud/http-cache}"

echo "Setting up HTTP response cache..."
echo "Cache directory: $CACHE_DIR"

# Create directory if it doesn't exist
mkdir -p "$CACHE_DIR"

# Create a README in the cache directory
cat > "$CACHE_DIR/README.txt" << 'EOF'
North Cloud HTTP Response Cache
================================

This directory contains cached HTTP responses from nc-http-proxy.

Structure:
  {domain}/
    {METHOD}_{hash}.json    # Cached response

Files here are:
- Created when proxy runs in "record" mode
- Read when proxy runs in "replay" mode (after fixtures)
- NOT version controlled (user-specific)

To clear the cache:
  task proxy:clear
  # or
  rm -rf ~/.northcloud/http-cache/*

To promote a cached response to a fixture:
  1. Copy the file to crawler/fixtures/{domain}/
  2. Commit the fixture

EOF

echo "Cache directory ready at: $CACHE_DIR"
echo ""
echo "Usage:"
echo "  1. Start proxy in record mode: task proxy:mode:record"
echo "  2. Run your crawler"
echo "  3. Responses are cached in $CACHE_DIR"
echo "  4. Switch to replay mode: task proxy:mode:replay"

#!/bin/bash
# Regression test: validate deploy.sh health check ports match expected service ports.
# Run: bash scripts/deploy_test.sh
# This prevents port drift like #458 (classifier checked on publisher's port).

set -euo pipefail

DEPLOY_SCRIPT="$(dirname "$0")/deploy.sh"
PASS=0
FAIL=0

# Expected service→port mapping (source of truth: CLAUDE.md / docker-compose)
declare -A EXPECTED_PORTS=(
  [auth]=8040
  [crawler]=8080
  [source-manager]=8050
  [classifier]=8071
  [publisher]=8070
  [index-manager]=8090
  [pipeline]=8075
  [search-service]=8090
)

check_port() {
  local service="$1"
  local expected="$2"

  # Find all check_health lines for this service and extract ports
  local ports
  ports=$(grep -oP "check_health\s+\"${service}\"\s+\"/health\"\s+\"\K[0-9]+" "$DEPLOY_SCRIPT" || true)

  if [ -z "$ports" ]; then
    echo "SKIP: $service — no health check found in deploy.sh"
    return
  fi

  while IFS= read -r port; do
    if [ "$port" = "$expected" ]; then
      echo "PASS: $service → port $port"
      PASS=$((PASS + 1))
    else
      echo "FAIL: $service → port $port (expected $expected)"
      FAIL=$((FAIL + 1))
    fi
  done <<< "$ports"
}

echo "=== deploy.sh health check port validation ==="
echo ""

for service in "${!EXPECTED_PORTS[@]}"; do
  check_port "$service" "${EXPECTED_PORTS[$service]}"
done

echo ""
echo "Results: $PASS passed, $FAIL failed"

if [ "$FAIL" -gt 0 ]; then
  echo "ERROR: Port mismatches detected — update deploy.sh or EXPECTED_PORTS"
  exit 1
fi

echo "All health check ports match expected values."

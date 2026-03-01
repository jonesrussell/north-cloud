#!/usr/bin/env bash
#
# docker-dev-doctor.sh — Diagnose common dev stack issues
#
# Usage: ./scripts/docker-dev-doctor.sh
#        task docker:dev:doctor
#

set -euo pipefail

COMPOSE="docker compose -f docker-compose.base.yml -f docker-compose.dev.yml"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

pass() { echo -e "  ${GREEN}[PASS]${NC} $1"; }
fail() { echo -e "  ${RED}[FAIL]${NC} $1"; }
warn() { echo -e "  ${YELLOW}[WARN]${NC} $1"; }
info() { echo -e "  ${CYAN}[INFO]${NC} $1"; }

ISSUES=0
WARNINGS=0

# Go services and their Postgres hostname + health port
GO_SERVICES="source-manager crawler classifier publisher index-manager pipeline click-tracker auth"

declare -A PG_HOST=(
  [source-manager]=postgres-source-manager
  [crawler]=postgres-crawler
  [classifier]=postgres-classifier
  [publisher]=postgres-publisher
  [index-manager]=postgres-index-manager
  [pipeline]=postgres-pipeline
  [click-tracker]=postgres-click-tracker
)

declare -A HEALTH_PORT=(
  [source-manager]=8050
  [crawler]=8080
  [classifier]=8070
  [publisher]=8070
  [index-manager]=8090
  [pipeline]=8075
  [click-tracker]=8093
  [auth]=8040
)

# ── 1. Container Status ──────────────────────────────────────
echo -e "\n${BOLD}1. Container Status${NC}"

EXITED=""
RESTARTING=""

while IFS= read -r line; do
  name=$(echo "$line" | awk '{print $1}')
  state=$(echo "$line" | awk '{print $2}')

  case "$state" in
    running)  pass "$name" ;;
    exited)   fail "$name (exited)"; EXITED="$EXITED $name"; ((ISSUES++)) ;;
    restarting) fail "$name (restarting)"; RESTARTING="$RESTARTING $name"; ((ISSUES++)) ;;
    *)        warn "$name ($state)"; ((WARNINGS++)) ;;
  esac
done < <($COMPOSE ps --format '{{.Name}} {{.State}}' 2>/dev/null)

if [ -z "$($COMPOSE ps --format '{{.Name}}' 2>/dev/null)" ]; then
  fail "No containers found. Run: task docker:dev:up"
  echo -e "\n${RED}No containers running — remaining checks skipped.${NC}"
  exit 1
fi

# ── 2. Go Version Check ──────────────────────────────────────
echo -e "\n${BOLD}2. Go Version (stale image detection)${NC}"

STALE_IMAGES=""
for svc in $GO_SERVICES; do
  container=$($COMPOSE ps -q "$svc" 2>/dev/null || true)
  if [ -z "$container" ]; then
    info "$svc — not running, skipped"
    continue
  fi

  # Get the Go version inside the running container
  go_ver=$(docker exec "$container" go version 2>/dev/null | grep -oP 'go\K[0-9]+\.[0-9]+(\.[0-9]+)?' || true)
  if [ -z "$go_ver" ]; then
    warn "$svc — could not determine Go version"
    ((WARNINGS++))
    continue
  fi

  # Get the required version from go.mod (mounted at /app/go.mod)
  required=$(docker exec "$container" head -5 /app/go.mod 2>/dev/null | grep -oP '^go \K[0-9]+\.[0-9]+' || true)
  if [ -z "$required" ]; then
    warn "$svc — could not read go.mod"
    ((WARNINGS++))
    continue
  fi

  # Compare major.minor
  running_minor=$(echo "$go_ver" | cut -d. -f1-2)
  if [ "$running_minor" != "$required" ]; then
    fail "$svc — running Go $go_ver, go.mod requires >= $required (STALE IMAGE)"
    STALE_IMAGES="$STALE_IMAGES $svc"
    ((ISSUES++))
  else
    pass "$svc — Go $go_ver (matches go.mod $required)"
  fi
done

# ── 3. DNS Resolution ────────────────────────────────────────
echo -e "\n${BOLD}3. DNS Resolution${NC}"

SHARED_HOSTS="elasticsearch redis"

for svc in $GO_SERVICES; do
  container=$($COMPOSE ps -q "$svc" 2>/dev/null || true)
  [ -z "$container" ] && continue

  # Check service-specific Postgres hostname
  pg=${PG_HOST[$svc]:-}
  if [ -n "$pg" ]; then
    if docker exec "$container" nslookup "$pg" >/dev/null 2>&1 || \
       docker exec "$container" getent hosts "$pg" >/dev/null 2>&1; then
      pass "$svc -> $pg"
    else
      fail "$svc cannot resolve $pg"
      ((ISSUES++))
    fi
  fi

  # Check shared infra (only for first service to avoid noise)
  if [ "$svc" = "source-manager" ]; then
    for host in $SHARED_HOSTS; do
      if docker exec "$container" nslookup "$host" >/dev/null 2>&1 || \
         docker exec "$container" getent hosts "$host" >/dev/null 2>&1; then
        pass "shared -> $host"
      else
        fail "Cannot resolve $host from network"
        ((ISSUES++))
      fi
    done
  fi
done

# ── 4. Postgres Connectivity ─────────────────────────────────
echo -e "\n${BOLD}4. Postgres Connectivity${NC}"

PG_CONTAINERS="postgres-source-manager postgres-crawler postgres-classifier postgres-publisher postgres-index-manager postgres-pipeline postgres-click-tracker"

for pg in $PG_CONTAINERS; do
  container=$($COMPOSE ps -q "$pg" 2>/dev/null || true)
  if [ -z "$container" ]; then
    info "$pg — not running, skipped"
    continue
  fi

  if docker exec "$container" pg_isready -U postgres >/dev/null 2>&1; then
    pass "$pg accepting connections"
  else
    fail "$pg not accepting connections"
    ((ISSUES++))
  fi
done

# ── 5. Service Health Endpoints ───────────────────────────────
echo -e "\n${BOLD}5. Service Health Endpoints${NC}"

for svc in $GO_SERVICES; do
  container=$($COMPOSE ps -q "$svc" 2>/dev/null || true)
  [ -z "$container" ] && continue

  port=${HEALTH_PORT[$svc]}
  if docker exec "$container" wget -q -O /dev/null "http://localhost:${port}/health" 2>/dev/null; then
    pass "$svc :${port}/health"
  else
    fail "$svc :${port}/health not responding"
    ((ISSUES++))
  fi
done

# ── 6. Summary + Fix Suggestions ─────────────────────────────
echo -e "\n${BOLD}6. Summary${NC}"

if [ "$ISSUES" -eq 0 ] && [ "$WARNINGS" -eq 0 ]; then
  echo -e "\n  ${GREEN}All checks passed.${NC}\n"
  exit 0
fi

[ "$WARNINGS" -gt 0 ] && echo -e "  ${YELLOW}Warnings: $WARNINGS${NC}"
[ "$ISSUES" -gt 0 ] && echo -e "  ${RED}Issues: $ISSUES${NC}"

echo -e "\n${BOLD}Suggested Fixes:${NC}"

if [ -n "$STALE_IMAGES" ]; then
  echo -e "  ${CYAN}Stale Go images detected. Rebuild from scratch:${NC}"
  echo -e "    task docker:dev:rebuild"
fi

if [ -n "$RESTARTING" ]; then
  echo -e "  ${CYAN}Containers restarting — check logs:${NC}"
  for c in $RESTARTING; do
    echo -e "    $COMPOSE logs --tail=50 $c"
  done
fi

if [ -n "$EXITED" ]; then
  echo -e "  ${CYAN}Containers exited — check logs:${NC}"
  for c in $EXITED; do
    echo -e "    $COMPOSE logs --tail=50 $c"
  done
fi

echo ""
exit 1

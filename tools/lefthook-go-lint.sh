#!/bin/bash
# Run golangci-lint only for Go package directories touched by staged files.
# Usage:
#   tools/lefthook-go-lint.sh [--print] FILE...

set -euo pipefail

MODE="run"
if [[ "${1:-}" == "--print" ]]; then
  MODE="print"
  shift
fi

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

declare -a SERVICES=()
declare -A SEEN_SERVICES=()
declare -A SERVICE_PKGS=()
declare -A SEEN_PKGS=()

add_service() {
  local svc="$1"
  if [[ -z "${SEEN_SERVICES[$svc]+x}" ]]; then
    SERVICES+=("$svc")
    SEEN_SERVICES["$svc"]=1
  fi
}

add_package() {
  local svc="$1"
  local pkg="$2"
  local key="$svc|$pkg"

  if [[ -n "${SEEN_PKGS[$key]+x}" ]]; then
    return
  fi

  add_service "$svc"
  SEEN_PKGS["$key"]=1
  SERVICE_PKGS["$svc"]="${SERVICE_PKGS[$svc]:-} $pkg"
}

for raw_file in "$@"; do
  file="${raw_file//\\//}"
  [[ "$file" == *.go ]] || continue
  [[ "$file" == */vendor/* ]] && continue
  [[ "$file" == */* ]] || continue

  svc="${file%%/*}"
  [[ -f "$REPO_ROOT/$svc/go.mod" ]] || continue

  rel="${file#"$svc"/}"
  dir="${rel%/*}"
  if [[ "$dir" == "$rel" ]]; then
    dir="."
  fi

  if [[ "$dir" == "." ]]; then
    pkg="."
    pkg_dir="$REPO_ROOT/$svc"
  else
    pkg="./$dir"
    pkg_dir="$REPO_ROOT/$svc/$dir"
  fi

  [[ -d "$pkg_dir" ]] || continue
  add_package "$svc" "$pkg"
done

if [[ ${#SERVICES[@]} -eq 0 ]]; then
  echo "No changed Go package directories to lint."
  exit 0
fi

for svc in "${SERVICES[@]}"; do
  pkgs="${SERVICE_PKGS[$svc]# }"
  if [[ "$MODE" == "print" ]]; then
    echo "$svc: $pkgs"
    continue
  fi

  echo "Linting $svc packages: $pkgs"
  (cd "$REPO_ROOT/$svc" && GOWORK=off golangci-lint run --config ../.golangci.yml $pkgs)
done

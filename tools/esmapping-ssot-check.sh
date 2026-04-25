#!/usr/bin/env bash
# Fail if service packages redefine raw_content / classified_content field maps.
# Canonical definitions live in infrastructure/esmapping only.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
violations=0

check_dir() {
	local dir="$1"
	for f in "$dir"/*.go; do
		[[ -e "$f" ]] || continue
		local base
		base=$(basename "$f")
		case "$base" in
		*_test.go | community.go | factory.go | versions.go | mappings.go)
			continue
			;;
		esac
		if grep -E '"type"[[:space:]]*:' "$f" >/dev/null 2>&1; then
			echo "ERROR: Elasticsearch field map literals found in $f" >&2
			echo "       Move definitions to infrastructure/esmapping (SSoT)." >&2
			violations=$((violations + 1))
		fi
	done
}

check_dir "$ROOT/classifier/internal/elasticsearch/mappings"
check_dir "$ROOT/index-manager/internal/elasticsearch/mappings"

if [[ "$violations" -ne 0 ]]; then
	echo "$violations file(s) violate ES mapping SSoT boundaries." >&2
	exit 1
fi

echo "OK: no raw_content/classified_content field maps outside infrastructure/esmapping."

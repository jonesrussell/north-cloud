#!/usr/bin/env bash
# Generate favicon.ico from favicon.svg.
# Uses sharp-cli and png-to-ico (npm). Run from dashboard/: ./scripts/generate-favicon.sh
# Alternative: ImageMagick - convert -background none -resize 32x32 public/favicon.svg public/favicon.ico

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DASHBOARD_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
PUBLIC_DIR="$DASHBOARD_DIR/public"
SVG="$PUBLIC_DIR/favicon.svg"
OUT="$PUBLIC_DIR/favicon.ico"

if [[ ! -f "$SVG" ]]; then
  echo "Missing $SVG"
  exit 1
fi

cd "$DASHBOARD_DIR"
npx --yes sharp-cli resize 32 32 -i "$SVG" -o "$PUBLIC_DIR/favicon-32.png"
npx --yes png-to-ico "$PUBLIC_DIR/favicon-32.png" > "$OUT"
rm -f "$PUBLIC_DIR/favicon-32.png"
echo "Generated $OUT"

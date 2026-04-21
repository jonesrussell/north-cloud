#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT"
VENV="${SCRIPT_DIR}/.venv-ports-gen"
if [ ! -x "${VENV}/bin/python" ]; then
  python3 -m venv "$VENV"
  "${VENV}/bin/pip" install -q -r "${SCRIPT_DIR}/ports-gen-requirements.txt"
fi
exec "${VENV}/bin/python" "${SCRIPT_DIR}/generate-ports-env-table.py" "$@"

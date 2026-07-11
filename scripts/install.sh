#!/usr/bin/env bash
# Install a reviewed wheel artifact for the current user.
set -euo pipefail

RELEASE_ARTIFACT="${ACMG_RELEASE_ARTIFACT:-}"

if ! command -v python3 >/dev/null 2>&1; then
    echo "Error: Python 3.12 or 3.13 is required." >&2
    exit 1
fi

python3 - <<'PY'
import sys
if not (3, 12) <= sys.version_info[:2] < (3, 14):
    raise SystemExit("Error: Python 3.12 or 3.13 is required.")
PY

if [[ ! "$RELEASE_ARTIFACT" =~ ^https://[^[:space:]#]+\.whl#sha256=[[:xdigit:]]{64}$ ]]; then
    echo "Error: no verified immutable release wheel is configured; refusing source installation." >&2
    exit 1
fi

python3 -m pip install --user --require-hashes --upgrade "$RELEASE_ARTIFACT"


BIN_DIRECTORY="$(python3 -m site --user-base)/bin"
printf 'Installed acmg-mcp. Ensure %s is on PATH, then configure your MCP client to run acmg-mcp.\n' "$BIN_DIRECTORY"

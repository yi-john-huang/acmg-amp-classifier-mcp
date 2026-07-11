#!/usr/bin/env bash
# Install the Python stdio MCP server for the current user.
set -euo pipefail

REPOSITORY="git+https://github.com/yi-john-huang/acmg-amp-classifier-mcp.git"

if ! command -v python3 >/dev/null 2>&1; then
    echo "Error: Python 3.12 or newer is required." >&2
    exit 1
fi

python3 - <<'PY'
import sys
if sys.version_info < (3, 12):
    raise SystemExit("Error: Python 3.12 or newer is required.")
PY

python3 -m pip install --user --upgrade "$REPOSITORY"

BIN_DIRECTORY="$(python3 -m site --user-base)/bin"
printf 'Installed acmg-mcp. Ensure %s is on PATH, then configure your MCP client to run acmg-mcp.\n' "$BIN_DIRECTORY"

#!/usr/bin/env bash
# Watch source files and auto-run scripts/deploy.sh on change (debounced).
# Requires fswatch:  brew install fswatch
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

if ! command -v fswatch >/dev/null 2>&1; then
  echo "fswatch not found. Install it with: brew install fswatch" >&2
  exit 1
fi

echo "Initial deploy..."
"$SCRIPT_DIR/deploy.sh"

echo "Watching for changes (Ctrl-C to stop)..."
# -o batches events into a single count per interval; -l 2 adds 2s latency so a
# burst of saves coalesces into one deploy.
fswatch -o -l 2 \
  -e '/\.git/' -e '/node_modules/' -e '/\.next/' -e '/out/' -e '/bin/' -e '\.db$' \
  "$ROOT_DIR/backend" \
  "$ROOT_DIR/frontend/app" \
  "$ROOT_DIR/frontend/lib" \
  "$ROOT_DIR/frontend/components" \
  "$ROOT_DIR/frontend/public" \
  "$ROOT_DIR/Dockerfile" \
  "$ROOT_DIR/docker-compose.yml" \
| while read -r _; do
    echo "==> change detected, deploying..."
    "$SCRIPT_DIR/deploy.sh" || echo "deploy failed; will retry on next change"
  done

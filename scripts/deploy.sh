#!/usr/bin/env bash
# Sync the working tree to the VPS over rsync/ssh and rebuild the container.
# Config comes from .deploy.env at the repo root (gitignored).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="$ROOT_DIR/.deploy.env"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Missing $ENV_FILE — copy .deploy.env.example to .deploy.env and fill it in." >&2
  exit 1
fi
# shellcheck disable=SC1090
source "$ENV_FILE"

: "${DEPLOY_HOST:?set DEPLOY_HOST in .deploy.env (e.g. root@1.2.3.4)}"
: "${DEPLOY_PATH:?set DEPLOY_PATH in .deploy.env (e.g. /opt/sing-box-admin)}"
SSH_PORT="${DEPLOY_PORT:-22}"

echo "==> rsync -> $DEPLOY_HOST:$DEPLOY_PATH"
# --delete keeps the remote tree in sync, but excluded paths are never deleted,
# so the VPS-side .env and the docker data volume are safe.
rsync -az --delete \
  -e "ssh -p $SSH_PORT" \
  --exclude='.git' \
  --exclude='**/node_modules' \
  --exclude='frontend/.next' \
  --exclude='frontend/out' \
  --exclude='backend/bin' \
  --exclude='backend/internal/web/dist' \
  --exclude='*.db' \
  --exclude='.env' \
  --exclude='.deploy.env' \
  "$ROOT_DIR/" "$DEPLOY_HOST:$DEPLOY_PATH/"

echo "==> remote: docker compose up -d --build"
ssh -p "$SSH_PORT" "$DEPLOY_HOST" "cd '$DEPLOY_PATH' && docker compose up -d --build"

echo "==> done. Panel: http://${DEPLOY_HOST#*@}:8080"

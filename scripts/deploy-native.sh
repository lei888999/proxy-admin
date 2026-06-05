#!/usr/bin/env bash
# Native (non-Docker) test deploy: build the embedded single binary locally,
# ship just the binary to the VPS, and run it under systemd.
# VPS needs nothing but this binary (and sing-box itself for status to read).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="$ROOT_DIR/.deploy.env"

[[ -f "$ENV_FILE" ]] || { echo "Missing $ENV_FILE — copy .deploy.env.example to .deploy.env." >&2; exit 1; }
# shellcheck disable=SC1090
source "$ENV_FILE"
: "${DEPLOY_HOST:?set DEPLOY_HOST in .deploy.env (e.g. root@1.2.3.4)}"
: "${DEPLOY_PATH:?set DEPLOY_PATH in .deploy.env (e.g. /opt/sing-box-admin)}"
SSH_PORT="${DEPLOY_PORT:-22}"
SINGBOX_VERSION="${SINGBOX_VERSION:-1.14.0}"

echo "==> detecting VPS architecture"
RAW_ARCH="$(ssh -p "$SSH_PORT" "$DEPLOY_HOST" 'uname -m')"
case "$RAW_ARCH" in
  x86_64|amd64)  GOARCH=amd64 ;;
  aarch64|arm64) GOARCH=arm64 ;;
  *) echo "unsupported VPS arch: $RAW_ARCH" >&2; exit 1 ;;
esac
echo "    $RAW_ARCH -> GOARCH=$GOARCH"

echo "==> building frontend (static export)"
( cd "$ROOT_DIR/frontend" && npm run build )

echo "==> embedding frontend into backend/internal/web/dist"
rm -rf "$ROOT_DIR/backend/internal/web/dist"
mkdir -p "$ROOT_DIR/backend/internal/web/dist"
cp -r "$ROOT_DIR/frontend/out/." "$ROOT_DIR/backend/internal/web/dist/"

echo "==> cross-compiling linux/$GOARCH static binary"
BIN="$(mktemp -t sing-box-admin.XXXXXX)"
( cd "$ROOT_DIR/backend" && CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" \
    go build -trimpath -ldflags="-s -w" -o "$BIN" ./cmd/server )

echo "==> fetching sing-box ${SINGBOX_VERSION} (linux/${GOARCH})"
SB_TMP="$(mktemp -d)"
curl -fsSL "https://github.com/SagerNet/sing-box/releases/download/v${SINGBOX_VERSION}/sing-box-${SINGBOX_VERSION}-linux-${GOARCH}.tar.gz" \
  | tar -xz -C "$SB_TMP"

echo "==> uploading binary + sing-box to $DEPLOY_HOST:$DEPLOY_PATH"
ssh -p "$SSH_PORT" "$DEPLOY_HOST" "mkdir -p '$DEPLOY_PATH' '$DEPLOY_PATH/data' '$DEPLOY_PATH/data/singbox/bin'"
scp -P "$SSH_PORT" "$BIN" "$DEPLOY_HOST:$DEPLOY_PATH/sing-box-admin.new"
scp -P "$SSH_PORT" "$SB_TMP/sing-box-${SINGBOX_VERSION}-linux-${GOARCH}/sing-box" \
  "$DEPLOY_HOST:$DEPLOY_PATH/data/singbox/bin/sing-box"
ssh -p "$SSH_PORT" "$DEPLOY_HOST" "chmod +x '$DEPLOY_PATH/data/singbox/bin/sing-box'"
rm -f "$BIN"
rm -rf "$SB_TMP"

echo "==> installing/refreshing systemd unit and restarting (needs root on VPS)"
ssh -p "$SSH_PORT" "$DEPLOY_HOST" "bash -se" <<EOF
set -euo pipefail
cd '$DEPLOY_PATH'
mv -f sing-box-admin.new sing-box-admin
chmod +x sing-box-admin
if [[ ! -f .env ]]; then
  echo "ERROR: $DEPLOY_PATH/.env missing — create it with JWT_SECRET (openssl rand -hex 32)." >&2
  exit 1
fi
cat > /etc/systemd/system/sing-box-admin.service <<UNIT
[Unit]
Description=sing-box-admin (native test deploy)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=$DEPLOY_PATH
EnvironmentFile=$DEPLOY_PATH/.env
Environment=DB_PATH=$DEPLOY_PATH/data/sing-box-admin.db
Environment=SINGBOX_DIR=$DEPLOY_PATH/data/singbox
ExecStart=$DEPLOY_PATH/sing-box-admin
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable sing-box-admin >/dev/null 2>&1 || true
systemctl restart sing-box-admin
sleep 1
systemctl --no-pager --lines=10 status sing-box-admin || true
EOF

echo "==> done. Panel: http://${DEPLOY_HOST#*@}:8080"
echo "    Logs: ssh $DEPLOY_HOST 'journalctl -u sing-box-admin -f'"

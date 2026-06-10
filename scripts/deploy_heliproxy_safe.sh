#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DATA_DIR="${HELIPROXY_DATA_DIR:-/home/wiee/.openclaw/workspace/heliproxy-data}"
CONTAINER_NAME="${HELIPROXY_CONTAINER_NAME:-heliproxy}"
IMAGE_NAME="${HELIPROXY_IMAGE_NAME:-heliproxy:latest}"
HTTP_PORT="${HELIPROXY_HTTP_PORT:-18081}"
WS_PORT="${HELIPROXY_WS_PORT:-18082}"
USER_SPEC="$(id -u):$(id -g)"

log() { printf '[heliproxy-deploy] %s\n' "$*"; }
fail() { printf '[heliproxy-deploy] ERROR: %s\n' "$*" >&2; exit 1; }

[ -d "$DATA_DIR" ] || fail "data dir not found: $DATA_DIR"
[ -f "$DATA_DIR/config.yaml" ] || fail "config not found: $DATA_DIR/config.yaml"

log "building $IMAGE_NAME from $ROOT_DIR"
docker build -t "$IMAGE_NAME" "$ROOT_DIR"

log "recreating $CONTAINER_NAME with HTTP:$HTTP_PORT and WS:$WS_PORT published"
docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
docker run -d \
  --name "$CONTAINER_NAME" \
  --user "$USER_SPEC" \
  --restart unless-stopped \
  -p "$HTTP_PORT:18081" \
  -p "$WS_PORT:18082" \
  -v "$DATA_DIR:/app/data" \
  -e DATA_DIR=/app/data \
  "$IMAGE_NAME" >/dev/null

log "checking HTTP health"
for _ in {1..20}; do
  if curl -fsS "http://127.0.0.1:$HTTP_PORT/healthz" >/tmp/heliproxy-health.json; then
    cat /tmp/heliproxy-health.json
    printf '\n'
    break
  fi
  sleep 1
done
curl -fsS "http://127.0.0.1:$HTTP_PORT/healthz" >/dev/null || fail "HTTP health failed on :$HTTP_PORT"

log "checking host listeners"
ss -ltn | grep -q ":$HTTP_PORT " || fail "host port $HTTP_PORT is not listening"
ss -ltn | grep -q ":$WS_PORT " || fail "host port $WS_PORT is not listening"

log "checking WebSocket slotSubscribe"
node - <<'NODE'
const fs = require('fs');
const path = require('path');
let WebSocket;
for (const candidate of [
  path.join(process.cwd(), 'node_modules', 'ws'),
  '/home/wiee/meridian/node_modules/ws',
]) {
  try { WebSocket = require(candidate); break; } catch {}
}
if (!WebSocket) throw new Error('node ws module not found; install npm dependency or set a known ws module path');
const cfgPath = process.env.HELIPROXY_CONFIG_PATH || '/home/wiee/.openclaw/workspace/heliproxy-data/config.yaml';
const cfg = fs.readFileSync(cfgPath, 'utf8');
const match = cfg.match(/client_keys:\s*\n\s*-\s*([^\s]+)/);
if (!match) throw new Error('client key not found in config');
const key = match[1];
const port = process.env.HELIPROXY_WS_PORT || '18082';
const ws = new WebSocket(`ws://127.0.0.1:${port}/?api-key=${encodeURIComponent(key)}`);
const timer = setTimeout(() => { console.error('websocket timeout'); ws.terminate(); process.exit(2); }, 10000);
ws.on('open', () => ws.send(JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'slotSubscribe' })));
ws.on('message', msg => {
  const text = String(msg);
  if (!text.includes('"result"')) {
    console.error('unexpected websocket response:', text.slice(0, 200));
    process.exit(3);
  }
  console.log('websocket ok:', text.slice(0, 120));
  clearTimeout(timer);
  ws.close();
});
ws.on('error', err => { clearTimeout(timer); console.error(err.message); process.exit(1); });
NODE

log "container ports"
docker ps --filter "name=$CONTAINER_NAME" --format 'table {{.Names}}\t{{.Ports}}\t{{.Status}}'
log "done"

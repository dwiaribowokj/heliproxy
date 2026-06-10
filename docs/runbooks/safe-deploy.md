# Heliproxy Safe Deploy Runbook

Use this runbook whenever rebuilding or recreating the local Heliproxy container.

## Why this exists

`@solana/web3.js` derives the WebSocket endpoint from the RPC URL. If Meridian uses:

```env
RPC_URL=http://localhost:18081/?api-key=<heliproxy-client-key>
```

then web3.js connects subscriptions to:

```text
ws://localhost:18082/?api-key=<heliproxy-client-key>
```

If the container is recreated without `-p 18082:18082`, HTTP RPC still works but WebSocket subscriptions fail with:

```text
ws error: connect ECONNREFUSED 127.0.0.1:18082
```

## Recommended deploy

From the repo root:

```bash
./scripts/deploy_heliproxy_safe.sh
```

The script:

1. Builds `heliproxy:latest`.
2. Recreates the `heliproxy` container.
3. Publishes both required ports:
   - `18081:18081` HTTP JSON-RPC / REST / dashboard
   - `18082:18082` Solana WebSocket proxy
4. Mounts the runtime config directory:
   - `/home/wiee/.openclaw/workspace/heliproxy-data:/app/data`
5. Runs as the host user so it can read `config.yaml` when the file is mode `600`.
6. Verifies `/healthz`.
7. Verifies host listeners for both ports.
8. Verifies WebSocket with `slotSubscribe`.

## Manual deploy checklist

If you must use `docker run` manually, include **both** ports:

```bash
docker rm -f heliproxy
docker run -d \
  --name heliproxy \
  --user "$(id -u):$(id -g)" \
  --restart unless-stopped \
  -p 18081:18081 \
  -p 18082:18082 \
  -v /home/wiee/.openclaw/workspace/heliproxy-data:/app/data \
  -e DATA_DIR=/app/data \
  heliproxy:latest
```

Then verify:

```bash
curl -fsS http://127.0.0.1:18081/healthz
ss -ltn | grep -E ':1808[12] '
docker ps --filter name=heliproxy --format 'table {{.Names}}\t{{.Ports}}\t{{.Status}}'
```

Expected docker port output includes:

```text
0.0.0.0:18081-18082->18081-18082/tcp
```

## Troubleshooting

### `ECONNREFUSED 127.0.0.1:18082`

Host port `18082` is not published/listening. Re-run:

```bash
./scripts/deploy_heliproxy_safe.sh
```

### HTTP works but live position subscriptions fail

Check WebSocket port first:

```bash
ss -ltn | grep ':18082 '
docker logs --tail=40 heliproxy
```

### Container exits with `permission denied` on `/app/data/config.yaml`

Run the container as the host user:

```bash
--user "$(id -u):$(id -g)"
```

# Heliproxy

Heliproxy is a Helius-compatible proxy for Solana JSON-RPC and selected Helius REST endpoints. It stores multiple upstream Helius API keys, rotates them with sticky round-robin, and lets clients keep the same request style by changing only the base URL and API key.

## Features

- Helius-style client auth with `?api-key=`.
- JSON-RPC proxy on `POST /` and `POST /rpc`.
- Helius REST proxy for `/v1/...`, including wallet balance calls.
- Sticky round-robin key rotation with failover.
- Failover on network timeout, `401`, `403`, `408`, `429`, and `5xx` upstream responses.
- Built-in admin dashboard.
- Optional Helius Admin API usage refresh when `project_id` is configured per key.
- File-backed config stored outside the image in a Docker volume.

## Request Shape

RPC clients can use the same body they send to Helius:

```bash
curl -sS -X POST 'http://localhost:18081/?api-key=<heliproxy-client-key>' \
  -H 'content-type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"getHealth"}'
```

REST calls keep the Helius path shape:

```bash
curl -sS 'http://localhost:18081/v1/wallet/<wallet-address>/balances?api-key=<heliproxy-client-key>'
```

## Repository Safety

Do not commit runtime secrets. This repo ignores:

- `data/`
- `config.yaml`
- `config.*.yaml`
- `.env` and `.env.*`
- logs and build artifacts

Use `config.example.yaml` and `docker-compose.example.yml` as templates only. Real client keys, admin keys, and Helius API keys belong in the mounted runtime config or environment variables on first run.

## Configuration

Default config path:

```text
./data/config.yaml
```

Inside Docker the default path is:

```text
/app/data/config.yaml
```

Override config path or data dir:

```bash
HELIPROXY_CONFIG=/path/to/config.yaml
HELIPROXY_DATA_DIR=/path/to/data
DATA_DIR=/path/to/data
```

### First-Run Environment Variables

These values are read only when the config file does not exist yet. After `config.yaml` is created, edit the dashboard or the mounted config file.

| Variable | Purpose |
| --- | --- |
| `HELIPROXY_CLIENT_KEY` / `HELIPROXY_CLIENT_KEYS` | One or more client-facing heliproxy keys |
| `HELIPROXY_ADMIN_KEY` / `HELIPROXY_ADMIN_KEYS` | One or more admin dashboard/API keys |
| `HELIUS_API_KEY` / `HELIUS_API_KEYS` | One or more upstream Helius API keys |
| `HELIUS_PROJECT_ID` / `HELIUS_PROJECT_IDS` | Optional Helius project IDs for usage refresh |
| `HELIUS_KEY_NAME` / `HELIUS_KEY_NAMES` | Optional names for keys in dashboard/status |
| `HELIPROXY_HOST` | Bind host, default `0.0.0.0` |
| `HELIPROXY_PORT` / `PORT` | Listen port, default `18081` |
| `HELIUS_RPC_URL` | Upstream RPC base URL, default `https://mainnet.helius-rpc.com/` |
| `HELIUS_REST_URL` | Upstream REST base URL, default `https://api.helius.xyz` |
| `HELIUS_ADMIN_URL` | Admin API base URL, default `https://admin-api.helius.xyz/v0` |
| `HELIPROXY_STICKY_LIMIT` | Requests per key before rotation, default `3` |
| `HELIPROXY_COOLDOWN_SECONDS` | Cooldown after failover-worthy failure, default `60` |
| `HELIPROXY_REQUEST_TIMEOUT_SECONDS` | Upstream request timeout, default `30` |
| `HELIPROXY_MAX_BODY_BYTES` | Request body limit, default `33554432` |

Comma-separated env values are index-aligned. Example:

```bash
HELIUS_API_KEYS=key1,key2,key3
HELIUS_PROJECT_IDS=project1,project2,project3
HELIUS_KEY_NAMES=helius-1,helius-2,helius-3
```

## Local Docker Deploy

1. Clone and enter the repo:

```bash
git clone git@github.com:dwiaribowokj/heliproxy.git
cd heliproxy
```

2. Build the image:

```bash
docker build -t heliproxy:latest .
```

3. Create a runtime data directory:

```bash
mkdir -p ./data
chmod 700 ./data
```

4. Start the container. Replace every `change-me-*` value before running:

```bash
docker run -d \
  --name heliproxy \
  --restart unless-stopped \
  -p 18081:18081 \
  -v "$PWD/data:/app/data" \
  -e HELIPROXY_CLIENT_KEY='change-me-client-key' \
  -e HELIPROXY_ADMIN_KEY='change-me-admin-key' \
  -e HELIUS_API_KEYS='change-me-helius-key-1,change-me-helius-key-2' \
  -e HELIUS_PROJECT_IDS='optional-project-id-1,optional-project-id-2' \
  heliproxy:latest
```

If the mounted `./data` directory is owned by your user and the container cannot write `config.yaml`, run with your host user:

```bash
docker rm -f heliproxy
docker run -d \
  --name heliproxy \
  --user "$(id -u):$(id -g)" \
  --restart unless-stopped \
  -p 18081:18081 \
  -v "$PWD/data:/app/data" \
  -e HELIPROXY_CLIENT_KEY='change-me-client-key' \
  -e HELIPROXY_ADMIN_KEY='change-me-admin-key' \
  -e HELIUS_API_KEYS='change-me-helius-key-1,change-me-helius-key-2' \
  heliproxy:latest
```

5. Check health:

```bash
curl -sS http://localhost:18081/healthz
```

6. Open the dashboard:

```text
http://localhost:18081/dashboard?api-key=<heliproxy-admin-key>
```

7. Test RPC:

```bash
curl -sS -X POST 'http://localhost:18081/?api-key=<heliproxy-client-key>' \
  -H 'content-type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"getHealth"}'
```

Expected result:

```json
{"id":1,"jsonrpc":"2.0","result":"ok"}
```

8. Test REST:

```bash
curl -sS 'http://localhost:18081/v1/wallet/11111111111111111111111111111111/balances?api-key=<heliproxy-client-key>'
```

## Docker Compose Deploy

1. Copy the example:

```bash
cp docker-compose.example.yml docker-compose.yml
```

2. Edit `docker-compose.yml` and replace all placeholder keys.

3. Start:

```bash
docker compose up -d --build
```

4. Watch logs:

```bash
docker logs -f heliproxy
```

5. Stop:

```bash
docker compose down
```

## Admin Endpoints

All admin endpoints require an admin key via `?api-key=`.

```bash
curl -sS 'http://localhost:18081/api/admin/status?api-key=<admin-key>'
curl -sS 'http://localhost:18081/api/admin/config?api-key=<admin-key>'
curl -sS 'http://localhost:18081/api/admin/usage?api-key=<admin-key>'
```

Update config with `PUT /api/admin/config` or use the dashboard. API keys returned by config/status are masked.

## Usage And Billing Cycle

Usage refresh calls Helius Admin API:

```text
GET https://admin-api.helius.xyz/v0/admin/projects/<project_id>/usage
X-Api-Key: <helius-api-key>
```

Set `project_id` for each Helius key to enable usage and billing-cycle display. RPC and REST proxying still work without `project_id`; usage refresh returns `missing_project_id` for that key.

If Helius returns an upstream error such as `Found project without billing period start`, heliproxy reports it as-is. That means the key/project works for proxying, but Helius Admin API cannot provide usage for that project yet.

## Meridian Integration

Set Meridian to use heliproxy as the central Helius endpoint.

`.env` example:

```env
# Direct Helius rollback values:
# RPC_URL=https://mainnet.helius-rpc.com/?api-key=your_helius_api_key_here
# HELIUS_BASE_URL=https://api.helius.xyz
# HELIUS_API_KEY=your_helius_api_key_here

RPC_URL=http://localhost:18081/?api-key=<heliproxy-client-key>
HELIUS_BASE_URL=http://localhost:18081
HELIUS_API_KEY=<heliproxy-client-key>
```

`user-config.json` example:

```json
{
  "rpcUrl": "http://localhost:18081/?api-key=<heliproxy-client-key>",
  "heliusBaseUrl": "http://localhost:18081",
  "heliusApiKey": "<heliproxy-client-key>"
}
```

Restart Meridian after changing config so its process reads the new values.

## Key Rotation Behavior

`sticky_round_robin_limit` controls how many successful requests a key handles before rotating. With the default `3`, key order behaves like:

```text
key1, key1, key1, key2, key2, key2, key3, key3, key3, ...
```

Failed keys are temporarily cooled down. Failover is attempted for network errors, timeout, `401`, `403`, `408`, `429`, and `5xx`. JSON-RPC errors returned with HTTP `200` are passed back to the client and are not treated as failover conditions.

## Development

Run tests with local Go:

```bash
go test ./...
```

Run tests without local Go installed:

```bash
docker run --rm -v "$PWD:/src" -w /src golang:1.23-alpine \
  sh -lc '/usr/local/go/bin/gofmt -w . && /usr/local/go/bin/go test ./...'
```

Build:

```bash
docker build -t heliproxy:latest .
```

## Troubleshooting

- `invalid_api_key`: the client/admin key in `?api-key=` does not match `config.yaml`.
- `no_available_helius_keys`: no enabled upstream Helius key is configured or every key is in cooldown.
- Dashboard changes disappear: verify the mounted data directory is writable by the container and check `docker logs heliproxy`.
- Usage refresh fails but RPC works: verify `project_id` and Helius Admin API availability for that project.
- `429` from Helius: add more upstream keys, increase cooldown, or reduce client request volume.

## License

MIT. See [LICENSE](LICENSE).

# Node / Agent API

The agent exposes an HTTP API on its configured port (default `2054`, configurable via `LUI_WEB_PORT`). All endpoints are protected by HMAC-SHA256 request signing.

## Authentication

Every request must include:

| Header | Value |
|--------|-------|
| `Authorization` | `Bearer <api_token>` |
| `X-LUI-Timestamp` | Unix epoch seconds |
| `X-LUI-Nonce` | 24-character random string |
| `X-LUI-Signature` | HMAC-SHA256 of canonical request |
| `X-LUI-Body-SHA256` | SHA-256 hex digest of request body |

The canonical string signed by HMAC is:

```
METHOD\nPATH\nBODY_SHA256\nTIMESTAMP\nNONCE
```

The hub verifies each response to prevent replay attacks (5-minute timestamp skew window, nonce dedup for 10 minutes).

## Response Format

All endpoints return a standard envelope:

```json
{
  "success": true,
  "msg": "optional message",
  "obj": { ... }
}
```

## Endpoints

### Status

**`GET /api/v1/status`** — System metrics and xray version.

Response `obj`:
```json
{
  "cpu": 12.5,
  "mem": { "current": 524288000, "total": 1073741824 },
  "disk": { "current": 8589934592, "total": 34359738368 },
  "netIO": { "up": 123456, "down": 789012 },
  "xray": { "version": "v26.4.25" },
  "panelVersion": "0.0.1",
  "uptime": 3600
}
```

### Metrics

**`GET /api/v1/metrics`** — Detailed system metrics.

### Sysinfo

**`GET /api/v1/sysinfo`** — OS, kernel, CPU info, network interfaces.

### Config

**`GET /api/v1/config`** — Get the currently stored config version.

**`POST /api/v1/config/push`** — Push a new config to the agent. Stores in SQLite AND writes to disk + restarts Xray.

```
Body: { "hub_node_id": "...", "hub_endpoint": "...", "xray_config": {...}, "client_list": [...] }
```

**`POST /api/v1/config/apply`** — Write xray config to disk and restart Xray. Atomic: writes `config.json` to the agent's binary folder (and legacy path), then runs `systemctl restart xray`.

```
Body: { "xray_config": {...} }
```

### Xray Management

**`GET /api/v1/xray/version`** — Detect the installed Xray version.

**`GET /api/v1/xray/status`** — Check if Xray is running and return its version.

Response:
```json
{ "success": true, "obj": { "version": "v26.4.25", "running": true } }
```

**`POST /api/v1/xray/install`** — Download and install a specific Xray version from GitHub.

```
Body: { "version": "v26.4.25" }
```

The agent downloads `Xray-{os}-{arch}.zip` from the official Xray-core releases, extracts the binary, and places it in the configured binary folder.

**`POST /api/v1/xray/restart`** — Restart the Xray service via `systemctl restart xray`.

### Logs

**`GET /api/v1/logs?lines=N`** — Fetch recent agent log lines (default 50).

### Firewall

**`GET /api/v1/firewall/status`** — UFW status and rule list.

Response:
```json
{ "success": true, "obj": { "active": true, "installed": true, "rules": [...] } }
```

**`POST /api/v1/firewall/rules`** — Add a firewall rule.

```
Body: { "port": "2053", "protocol": "tcp", "action": "allow", "comment": "web panel" }
```

Actions: `allow`, `deny`, `reject`, `limit`. Protocol: `tcp`, `udp` (or empty for both).

**`DELETE /api/v1/firewall/rules`** — Delete a firewall rule by number.

```
Body: { "rule_number": "1" }
```

**`POST /api/v1/firewall/enable`** — Enable UFW (with `--force`).

**`POST /api/v1/firewall/disable`** — Disable UFW (with `--force`).

### Lifecycle

**`POST /api/v1/restart`** — Restart the agent service itself.

**`POST /api/v1/certs`** — Push TLS certificate and key to the agent.

```
Body: { "certPEM": "...", "keyPEM": "..." }
```

**`GET /api/v1/certs/status`** — Get the current TLS certificate metadata.

**`GET /api/v1/server/getWebCertFiles`** — Get node's web TLS certificate file paths.

### Health

**`GET /healthz`** — Returns `{"status":"ok"}` (no auth required).

**`GET /readyz`** — Returns `{"status":"ok"}` when the agent is fully ready (no auth required).

### Agent-to-Hub Registration

During registration, the agent calls the hub's endpoint:

**`POST /panel/api/nodes/register`** — Register a new node with a one-time token.

```
Body: { "token": "...", "hostname": "...", "address": "...", "port": 2054, "version": "0.0.1" }
```

## Endpoint Summary

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/healthz` | No | Liveness check |
| GET | `/readyz` | No | Readiness check |
| GET | `/api/v1/status` | Yes | System status + xray version |
| GET | `/api/v1/metrics` | Yes | System metrics |
| GET | `/api/v1/sysinfo` | Yes | System info |
| GET | `/api/v1/config` | Yes | Current config version |
| POST | `/api/v1/config/push` | Yes | Push and apply config |
| POST | `/api/v1/config/apply` | Yes | Write config to disk + restart xray |
| GET | `/api/v1/xray/version` | Yes | Xray version |
| GET | `/api/v1/xray/status` | Yes | Xray running + version |
| POST | `/api/v1/xray/install` | Yes | Download and install Xray |
| POST | `/api/v1/xray/restart` | Yes | Restart Xray service |
| GET | `/api/v1/logs` | Yes | Agent log lines |
| GET | `/api/v1/firewall/status` | Yes | UFW status + rules |
| POST | `/api/v1/firewall/rules` | Yes | Add firewall rule |
| DELETE | `/api/v1/firewall/rules` | Yes | Delete firewall rule |
| POST | `/api/v1/firewall/enable` | Yes | Enable UFW |
| POST | `/api/v1/firewall/disable` | Yes | Disable UFW |
| POST | `/api/v1/restart` | Yes | Restart agent |
| POST | `/api/v1/certs` | Yes | Push TLS cert |
| GET | `/api/v1/certs/status` | Yes | Cert metadata |
| GET | `/api/v1/server/getWebCertFiles` | Yes | Web cert file paths |

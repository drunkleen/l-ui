# Node (Agent) API

The node API is the hub's control plane for remote VPS nodes. All endpoints are prefixed with `/api/v1`.

## Authentication

Hub requests carry:
- `Authorization: Bearer <token>` — the node's API token
- `X-LUI-Timestamp` — Unix timestamp
- `X-LUI-Nonce` — random string (24 chars)
- `X-LUI-Signature` — HMAC-SHA256 of the request
- `X-LUI-Body-SHA256` — SHA-256 digest of the request body (empty for GET)

The node verifies the timestamp, nonce, signature, and bearer token. Requests are rejected when the signature or timestamp is invalid.

## Node Status

### `GET /api/v1/server/status`
Returns system metrics, Xray version, and panel version.

### `GET /api/v1/server/health`
Quick liveness check — returns a lightweight health status.

## Configuration

### `GET /api/v1/server/config`
Returns the current node configuration.

### `GET /api/v1/server/configVersion`
Returns the current config version number (used for drift detection).

### `POST /api/v1/server/pushConfig`
Push Xray config and client list to the node. Body includes `hubNodeID`, `hubEndpoint`, `xrayConfig`, and `clientList`. Returns the new config version.

## Metrics

### `GET /api/v1/server/metrics`
Returns CPU, memory, disk, and network traffic metrics.

### `GET /api/v1/server/sysinfo`
Returns detailed system information (OS, kernel, CPU, etc.).

## Logs

### `GET /api/v1/server/logs?lines=50&filter=&showDirect=true&showBlocked=true&showProxy=true`
Returns recent agent log lines. Supports filtering and toggling traffic categories.

## Firewall

### `GET /api/v1/firewall/status`
Returns UFW status and current rules.

### `POST /api/v1/firewall/rules`
Add a firewall rule. Body: `{"port": 443, "protocol": "tcp", "action": "allow", "comment": "HTTPS"}`.

### `DELETE /api/v1/firewall/rules`
Delete a firewall rule by number. Body: `{"rule_number": "1"}`.

## Lifecycle

### `POST /api/v1/restart`
Restart the agent process.

### `POST /api/v1/server/restartXrayService`
Restart the local Xray service.

### `POST /api/v1/server/updatePanel`
Trigger agent self-update to the latest release.

### `POST /api/v1/server/reinstallBundle`
Reinstall the agent bundle from the hub (used for reconciliation).

### `POST /api/v1/server/cleanup`
Remove all Xray config, clients, and log data from the node.

### `POST /api/v1/server/rotateToken`
Rotate the API token. Body: `{"token": "new-token"}`.

## Inbounds

### `GET /api/v1/inbounds/list`
List all remote inbounds with their IDs (for cache synchronization).

### `POST /api/v1/inbounds/add`
Add a new inbound. Body contains the full inbound config.

### `POST /api/v1/inbounds/del/:id`
Delete an inbound by remote ID.

### `POST /api/v1/inbounds/update/:id`
Update an inbound by remote ID.

## Clients

### `POST /api/v1/clients/add`
Add a new client to an inbound.

### `POST /api/v1/clients/del/:email`
Delete a client by email.

### `POST /api/v1/clients/update/:email`
Update a client by old email. Body contains new client data.

### `POST /api/v1/clients/resetTraffic/:email`
Reset traffic counters for a specific client.

### `POST /api/v1/server/resetAllTraffics`
Reset all traffic counters on the node.

## Certificates

### `POST /api/v1/certs`
Push a TLS certificate and key to the node. Body: `{"cert": "...", "key": "..."}`.

### `GET /api/v1/certs/status`
Get the node's current TLS certificate info (subject, issuer, serial, validity, fingerprint).

## Web Cert Files

### `GET /api/v1/server/getWebCertFiles`
Fetch certificate file paths used by the node's web listener (for panel inbound TLS assignment).

## Response Format

All endpoints return a standard envelope:

```json
{
  "success": true,
  "msg": "optional message",
  "obj": {}
}
```

Errors set `success: false` and include a message in `msg`.

## Operational Notes

- Status and heartbeat data are used by the hub dashboard
- Stale node data is shown explicitly instead of being hidden
- The hub treats the node as a remote agent, not as a second UI instance
- Transient HTTP failures are automatically retried by the hub's retry layer (3 attempts, exponential backoff)

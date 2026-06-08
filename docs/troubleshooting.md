# Troubleshooting

## Local Dev Does Not Start

- Check whether ports `2053` (backend) or `5173` (Vite) are already in use.
- Stop any leftover local processes before retrying.
- `make dev` prints backend compile progress and should wait for `/csrf-token` before starting Vite.

## Backend Compiles But Never Starts

- Read the backend log output in the terminal.
- Check whether the database or log directory is writable.
- Verify that `LUI_DB_FOLDER` and `LUI_LOG_FOLDER` point to valid paths.

## Bootstrap Fails

- Check SSH credentials and sudo access.
- Verify systemd availability on the VPS.
- Check the node API health endpoint after bootstrap.
- Look at the bootstrap job details in the panel — each step shows success/failure and output.
- If SSH dial fails, verify the port is open and the host key is accepted.
- If bundle upload fails, check disk space on the VPS (`df -h`).

## Probe / Heartbeat Fails

- Verify the node agent is running (`systemctl status l-ui` on the node).
- Check network connectivity between the hub and the agent (firewall rules, routing).
- If the probe shows "HTTP 503" or "connection refused", the agent may be starting up — the hub retries automatically (3 attempts, 200ms–2s backoff).
- If the probe shows "HTTP 400" or "HTTP 401", the API token or signature may be mismatched — try rotating the node's credentials.

## Config Push Fails

- Verify the node is online and reachable.
- Check the node's config version in the panel — if it's `0`, no config has been pushed yet.
- The push includes both Xray config and client list; check both in the payload.
- If the node has inbounds with TLS certificates that don't exist on the node, use "Get Web Cert Files" or "Set Cert from Panel" to update the inbound's certificate paths.

## TLS / HTTPS Fails

### Hub Web TLS
- If the hub panel shows certificate errors, check that the cert files exist and paths are set via `l-ui cert`.
- For Let's Encrypt IP certificates: validity is ~6 days; auto-renewal is handled by acme.sh.

### Agent TLS (hub↔node)
- Generate a certificate from the hub panel (Nodes → Cert → Generate).
- Verify the cert status endpoint (`GET /api/v1/certs/status`) on the node.
- The `NodeCertRenewalJob` runs daily and renews certs expiring within 30 days.
- If the certificate is rejected, try the "Fetch and Pin" action to update the pinned SHA-256.

## Node Is Offline

- Heartbeats may be stale — wait for the next heartbeat cycle (every 5 seconds).
- Verify the node agent process is running (`systemctl status l-ui`).
- Use the reconcile action: it probes, restarts Xray, and reinstalls the bundle if needed.
- If the node is permanently unreachable, re-bootstrap it.

## Node Alerts Not Firing

- Check Telegram bot settings are configured (token, chat ID).
- Verify alert thresholds in the Telegram tab of Settings:
  - `nodeCpuThreshold` — CPU % threshold (default 90)
  - `nodeMemThreshold` — Memory % threshold (default 90)
  - `nodeDiskThreshold` — Disk % threshold (default 90)
  - `nodeDownThreshold` — Consecutive failed heartbeats before alert (default 3, ~15 seconds)
- The `NodeAlertJob` runs every 10 seconds.

## Release Build Fails

- Run `make build` locally first.
- Make sure `frontend/public/openapi.json` and `hub/web/dist/` are up to date.
- If the release workflow fails, check the Go and Node versions used by GitHub Actions.

## Retry / Transient Errors

- The hub retries transient HTTP failures automatically (3 attempts).
- If you see repeated retries in the logs, the issue is likely persistent (network partition, down node).
- Non-idempotent operations (AddInbound, PushConfig, InstallXray) are NOT retried to avoid duplicate side effects.

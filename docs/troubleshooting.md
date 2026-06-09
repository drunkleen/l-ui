# Troubleshooting

## Local Dev Startup Issues

**Backend compiles but server won't start** — Check port 2053 availability. Another process may be listening. Use `lsof -i :2053` or `ss -tlnp | grep 2053`.

**"failed to create log folder" errors in tests** — The test runner tries to create `/var/log/l-ui/` which requires root. Tests that don't need logging should use `config.IsDebug()` guards. Run `sudo mkdir -p /var/log/l-ui && sudo chown $USER /var/log/l-ui` for your user.

**Database migration errors** — Delete `./tmp/l-ui.db` and `./tmp/l-ui-agent.db` and restart `make dev`.

## Bootstrap Failures

**"missing l-ui-agent executable in bundle"** — The bootstrap downloaded an old-format release tarball (before the hub/agent split). The agent's extraction step auto-renames `l-ui/` → `l-ui-agent/` on new code. If you're running an old hub version, upgrade.

**"Process exited with status 1" at verify-agent** — The agent service started but the hub can't reach its API. Common causes:

- UFW is blocking the agent port. Check `ufw status` on the node and add a rule for the agent port.
- The agent is listening on a different port than expected. Check the env file at `/etc/default/l-ui-agent` for `LUI_WEB_PORT`.
- The agent's systemd service has `EnvironmentFile=/etc/default/l-ui` (old path) instead of `/etc/default/l-ui-agent`. Regenerate the service file with the correct path.

**SSH connection refused** — Verify the VPS IP, port, and credentials. Check that the SSH service is running on the remote host.

**"sudo not found"** — The bootstrap requires sudo. Install sudo on the remote host or add `NOPASSWD` for the SSH user.

## Probe / Heartbeat Failures

**Node shows offline** — The hub probes `GET /api/v1/status` every 5 seconds. If the agent is unreachable:

1. SSH into the node and check `systemctl status l-ui-agent`
2. Check `journalctl -u l-ui-agent -n 50` for errors
3. Verify the node's address/port/scheme in the hub panel
4. Test the API manually: `curl -skH "Authorization: Bearer <token>" https://<node>:<port>/api/v1/status`
5. Check UFW is not blocking the agent port

**"unauthorized" probe responses** — The API token on the hub doesn't match the agent's token. Regenerate the token on both sides.

## Config Push Failures

**Config push succeeds but xray doesn't restart** — The agent stores the config and attempts `systemctl restart xray`. If the xray service doesn't exist or has a wrong config path:

1. SSH into the node and check `systemctl status xray` (exit code 23 = config file not found)
2. Check the xray service file for the correct `-config` path: `grep ExecStart /etc/systemd/system/xray.service`
3. Verify the config file exists: `ls -la /usr/local/l-ui-agent/bin/config.json`
4. Run `systemctl restart xray` manually and check for errors

**HTTP 400 on firewall rules** — The hub sends port as a string (`"port": "2020"`). If you're using a custom script, make sure the port value is a string, not a number.

## TLS / HTTPS Failures

**Hub web panel shows certificate warning** — The hub uses a self-signed certificate by default. Configure proper certificates in the panel settings or use a reverse proxy (Caddy, Nginx) with Let's Encrypt.

**Agent TLS certificate expired** — The hub auto-renews agent certificates daily. If a certificate expired, run `systemctl restart l-ui-agent` on the node to trigger a fresh cert push.

## Node Offline Handling

When a node is unreachable for 15+ seconds, the hub marks it offline. All operations (config push, xray restart, firewall changes) are blocked until the node comes back online.

To recover an offline node:
1. Check the node's network connectivity
2. SSH into the node and restart the agent: `systemctl restart l-ui-agent`
3. If the agent won't start, reinstall the agent bundle from the hub panel

## Node Alerts Not Firing

- Verify the Telegram bot is enabled in settings
- Check that the bot token is valid
- Ensure the CPU/memory/disk thresholds are configured
- Alerts fire every 10 seconds — they are not instant

## Release Build Failures

**"Error: missing l-ui-agent.service in bundle" during release** — The bundle was built from an old workflow that uses the `l-ui/` prefix. Update `.github/workflows/release.yml` to use the `l-ui-agent/` directory structure.

**Windows builds fail with "missing xray dat files"** — The Windows release bundle doesn't include Xray dat files. They are downloaded separately on first run.

## Retry / Transient Errors

The hub retries transient errors with exponential backoff (3 attempts, 200ms initial, 5s max, 0.2 jitter). Transient errors include:

- Connection refused
- DNS resolution failure
- I/O timeout
- TLS handshake failure
- Connection reset by peer
- Broken pipe
- HTTP 429 (too many requests)
- HTTP 5xx (server error)

Persistent errors (4xx client errors, auth failures) are NOT retried.

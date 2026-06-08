# Bootstrap

Bootstrap is the process that turns a fresh VPS into a managed node. It runs as an async job tracked in the in-memory job store.

## Inputs

- SSH host or IP
- SSH user
- SSH password or SSH key (with optional passphrase)
- SSH port (default 22)
- Agent port (auto-selected if not specified)
- Optional domain, ACME email, and DNS provider for TLS

## Validation

The hub validates:

- Node name is non-empty
- Address is a valid IP or hostname
- SSH credentials are present (password, key, or both)
- Remote host has `systemd` and `sudo` (or is root)

## What the Hub Checks on the Remote Host

- CPU architecture (`uname -m`)
- `sudo` availability and passwordless (or password-enabled) sudo access
- `systemd` availability

## Flow

1. **Build node** — create a `model.Node` with generated API token, auto-selected port, and defaults.
2. **Prepare identity** — if a node with the same name already exists, reuse its API token and ID.
3. **SSH connect** — dial the remote host with retry (3 attempts, 2–10s exponential backoff + jitter).
4. **Detect arch** — run `uname -m` and map it to a bundle architecture.
5. **Build bundle** — create a tarball with the agent binary, service files, and metadata.
6. **Upload bundle** — SCP the bundle to `/tmp/l-ui-node-bundle.tar.gz` (with retry on failure, 3 attempts, 1–3s backoff).
7. **Install bundle** — extract the tarball to `/usr/local/l-ui`, with rollback support.
8. **Write env** — create `/etc/default/l-ui` with API token and port.
9. **Install service** — copy the correct systemd unit file based on the OS distribution.
10. **Verify bundle** — check the binary exists and is executable.
11. **Start service** — `systemctl daemon-reload && systemctl enable l-ui && systemctl restart l-ui`.
12. **Optional TLS** — install Caddy with a Let's Encrypt certificate (domain + DNS provider).
13. **Verify agent** — poll the agent's status endpoint until it responds (up to 30 iterations, 2–4s randomized jitter).
14. **Persist** — save the node in the database with the bundle SHA256.

## Error Handling

Every network-dependent step has retry logic:
- SSH dial: 3 attempts, 2–10s backoff
- Bundle upload: 3 attempts, 1–3s backoff
- Agent verification: polls with 2–4s jitter

If the service fails to start, the hub attempts a rollback:
1. Stop the failed agent.
2. Remove the new install.
3. Restore the previous version (if any).
4. Restart the old service.

## Cancellation

- If the caller's context is canceled before the goroutine starts, the job is marked as `failed` immediately.
- The bootstrap goroutine uses a 30-minute timeout, independent of the HTTP request context, so long-running bootstraps survive client disconnects.

## After Bootstrap

- The node should appear in the hub node list.
- Heartbeats should start updating status and metrics.
- If a domain is configured, HTTPS should become available after DNS resolves correctly.
- The config version starts at `0`, indicating no config has been pushed yet.

## Common Failure Points

- SSH credentials are wrong or the host key has changed
- `sudo` is unavailable or requires a TTY
- The host does not run `systemd` (e.g., OpenRC, SysV init)
- DNS is not pointing at the node when HTTPS/TLS is enabled
- Port 80 is required for Let's Encrypt HTTP-01 challenges (Caddy mode)
- The bundle download from GitHub fails due to network restrictions

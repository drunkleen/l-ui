# Node Bootstrap

The bootstrap flow provisions a new VPS node via SSH, installs the agent, and registers it with the hub.

## Inputs

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Node display name |
| `address` | Yes | VPS IP address or hostname |
| `sshUser` | Yes | SSH username (typically `root`) |
| `sshPassword` | Yes | SSH password (or key-based auth via the `authorized_keys` flow) |
| `sshPort` | No | SSH port (default `22`) |
| `agentPort` | No | Agent HTTP port (default `2054`) |
| `useTLS` | No | Enable TLS for agent communication |
| `domain` | Required if TLS | Domain for Let's Encrypt certificate |
| `acmeEmail` | No | Email for ACME registration |

## Validation

Before starting the bootstrap, the hub validates:

- SSH connection is reachable (TCP dial)
- `sudo` is available on the remote host
- `systemd` is available on the remote host
- OS architecture is supported (amd64, arm64, armv7, armv6, etc.)
- Required tools exist on the remote host (`curl`, `tar`, `systemctl`)

## Bootstrap Steps

The bootstrap runs as an async job with real-time progress tracking. Each step reports its output and status to the hub's job store.

| Step | Description |
|------|-------------|
| **detect-arch** | Runs `uname -m` on the remote host |
| **detect-arch-retry** | Retry with fallback detection if the first attempt fails |
| **map-arch** | Maps the detected architecture to a Go release arch name |
| **prepare-dirs** | Creates `/usr/local/l-ui-agent/`, `/etc/l-ui/`, `/var/log/l-ui/` |
| **build-bundle** | Resolves the agent release version from the bundle cache |
| **upload-bundle** | Downloads the agent tarball from GitHub to the remote host |
| **install-bundle** | Extracts the tarball and renames old-format `l-ui/` to `l-ui-agent/` |
| **write-env** | Writes the environment file to `/etc/default/l-ui-agent` |
| **install-service** | Installs the agent systemd unit from the bundle (or falls back to local files) |
| **verify-bundle** | Checks that the agent binary exists and is executable |
| **daemon-reload** | Runs `systemctl daemon-reload` |
| **enable-service** | Enables `systemctl enable l-ui-agent` |
| **restart-service** | Starts `systemctl restart l-ui-agent` and checks journalctl output |
| **service-diag** | Runs diagnostic checks if the service fails to start |
| **verify-agent** | Polls `http://127.0.0.1:{port}/api/v1/status` up to 30 times with HMAC auth |
| **rollback** | Cleans up on failure (removes install dir, disables service) |

### Backward Compatibility

When downloading release tarballs from old releases:

- Bundles with `l-ui/` prefix are automatically renamed to `l-ui-agent/`
- Bundles with `l-ui` binary get a symlink `l-ui-agent` → `l-ui`
- Old service files (`l-ui.service*`) are patched to remove the `run` subcommand and fix the binary path

## Error Handling

- Each step stores its output and success status
- On failure, the bootstrap stops and runs the cleanup step
- The hub retries the entire bootstrap on transient SSH errors (3 attempts with backoff)
- The frontend polls the job status every 1.2 seconds and shows the progress timeline

## Cleanup

When a bootstrap fails, the cleanup step:

1. Removes the agent install directory
2. Disables and stops the systemd service
3. Removes the service unit file
4. Removes the environment file
5. Removes the downloaded bundle

## TLS Bootstrap

When `useTLS=true`, after the agent is verified:

1. Installs Caddy as a reverse proxy
2. Configures Caddy with the agent domain
3. Caddy obtains a Let's Encrypt certificate automatically
4. The hub updates the node record to use HTTPS

## Post-Bootstrap

After a successful bootstrap:

1. The hub persists the node record to the database
2. The frontend refetches the node list (new card appears automatically)
3. A config push is triggered to write the xray config and restart xray
4. The heartbeat job starts collecting metrics from the new node

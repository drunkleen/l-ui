# Install

## Deployment Modes

L-UI supports two deployment styles:

- **Hub only**: run the panel centrally on one machine; add remote VPS nodes later
- **Hub + agent nodes**: deploy the lightweight agent binary to remote VPSes over SSH or via registration tokens

## Database

- Default: SQLite
- Production or multi-host: Postgres
- MySQL/MariaDB: supported through the shared storage layer

### Recommended Layout

- Local/dev: keep database and logs under the repo `./tmp/` directory
- Production hub: use system paths such as `/etc/l-ui` and `/var/log/l-ui`
- Agent nodes: local SQLite database under `/etc/l-ui/l-ui.db` (separate from the hub database)

## Hub Installation

1. Install the Go binary or use the release bundle via `install.sh`.
2. Point the panel at the chosen database (`LUI_DB_FOLDER`, `LUI_DB_TYPE`, `LUI_DB_DSN`).
3. Ensure the runtime folders exist and are writable.
4. Start the hub and open the panel in a browser.

## Agent Installation

### SSH Bootstrap (from the hub panel)

1. Create a node from the Nodes page.
2. Enter SSH credentials (host, user, password or key, SSH port).
3. Optionally configure TLS/domain for the agent endpoint.
4. The hub uploads the node bundle, installs the agent, and starts it.

### Registration Token (curl | sh)

1. Generate a registration token from the hub panel.
2. Run the provided curl | sh command on the VPS:
   ```bash
   bash <(curl -Ls https://hub.example.com/install.sh) LUI_REGISTRATION_TOKEN=xxx LUI_HUB_ENDPOINT=https://hub.example.com
   ```
3. The agent registers itself with the hub and appears in the node list.

## Docker Deployment

A `docker-compose.yml` is provided for production deployment. It includes health checks,
resource limits, log rotation, and an isolated bridge network for the `lui` and `postgres`
services. Postgres data is stored in a named volume so it survives container restarts.

```bash
# SQLite (default) — lui service only
docker compose up -d

# With Postgres — run the postgres profile
docker compose --profile postgres up -d
```

Set `POSTGRES_PASSWORD` in a `.env` file or environment to avoid the default:

```bash
echo "POSTGRES_PASSWORD=your-secure-password" > .env
docker compose --profile postgres up -d
```

The compose file mounts `$PWD/db` for persistent SQLite data and `$PWD/cert` for
certificate files. Fail2ban requires `NET_ADMIN` and `NET_RAW` capabilities or bans
are only logged.

## Docker vs Bare-Metal

| Aspect | Docker Compose | Bare-Metal |
|---|---|---|
| Database | SQLite on bind mount, or Postgres container | SQLite on filesystem, or external Postgres |
| Fail2ban | Requires `NET_ADMIN`/`NET_RAW` caps in compose | Works natively with iptables |
| Updates | Rebuild the image, `docker compose pull && up -d` | Re-run `install.sh` or replace binary |
| Log rotation | Handled by compose `logging` driver (json-file, 3 files × 10 MB) | Configure `logrotate` or journald |
| Resource limits | Enforced via `deploy.resources.limits` | Handled by systemd or host cgroups |
| Certificate storage | Bind mount `$PWD/cert/` | Directory of your choosing |
| Health checks | Built-in `healthcheck` on both services | Use systemd unit `ExecStartPre` probes |

## Environment Variables

| Variable | Description | Default |
|---|---|---|
| `LUI_DB_FOLDER` | Directory for SQLite database | `/etc/l-ui` |
| `LUI_DB_TYPE` | Database type (`sqlite` or `postgres`) | `sqlite` |
| `LUI_DB_DSN` | Postgres connection string | — |
| `LUI_LOG_FOLDER` | Log output directory | `/var/log/l-ui` |
| `LUI_WEB_PORT` | Hub/agent HTTP port | `2053` (hub) / `2054` (agent) |
| `LUI_DEBUG` | Enable debug log output | `false` |
| `LUI_BOOTSTRAP_API_TOKEN` | Agent bootstrap API token | — |
| `LUI_REGISTRATION_TOKEN` | One-time agent registration token | — |
| `LUI_HUB_ENDPOINT` | Hub endpoint URL for registration | — |
| `LUI_CERT_DIR` | Agent TLS certificate directory | `{DB_FOLDER}/certs` |

## Local Development

Use the Makefile from the repo root:

```bash
make dev
```

That target starts the backend first, waits for readiness, then launches Vite.

Local development login: `admin` / `admin`.

## Production Releases

- `make build` builds the frontend and both Go binaries (hub + agent) locally
- Tagged pushes build release tarballs (`l-ui-hub-linux-<arch>.tar.gz` + `l-ui-agent-linux-<arch>.tar.gz`) in GitHub Actions
- Tagged pushes also publish the production Docker image

## Installer And CLI

- `install.sh` is the production installer
- It downloads the latest release or a pinned tag, installs the service, and finishes with a command reference
- The `l-ui` wrapper exposes service operations: `start`, `stop`, `restart`, `status`, `settings`, `enable`, `disable`, `log`, `banlog`, `update`, `legacy`, `install`, `uninstall`
- The Go binary supports administrative subcommands:
  - `run` — start the hub server
  - `agent` — start the agent server
  - `migrate` / `migrate-db` — run database migrations
  - `setting` — read/write panel settings
  - `cert` — manage hub web TLS certificate paths

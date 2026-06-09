# Installation

## Deployment Modes

L-UI supports two deployment modes:

| Mode | Description |
|------|-------------|
| **Hub only** | Install the hub on a server. Add VPS nodes later via SSH bootstrap or registration tokens. |
| **Hub + Agent (same host)** | Run the hub and the agent on the same machine (e.g., a VPS that acts as both control plane and a node). |

## Database Options

| Database | Default | When to Use |
|----------|---------|-------------|
| **SQLite** | Yes | Single-server deployment. Zero configuration. |
| **Postgres** | No | Multi-node hub, hosted environments, or when you need replication/backup tooling. |
| **MySQL/MariaDB** | No | Existing MySQL infrastructure. GORM-compatible. |

Set via environment variables:

```bash
export LUI_DB_TYPE=postgres
export LUI_DB_DSN="host=localhost user=l-ui dbname=l-ui password=secret sslmode=disable"
```

## Hub Installation

### Quick Install (Linux)

```bash
bash <(curl -Ls https://raw.githubusercontent.com/drunkleen/l-ui/master/install.sh)
```

### Manual Install

1. Download the latest hub release tarball from GitHub:
   ```bash
   curl -fL -o l-ui-hub.tar.gz https://github.com/drunkleen/l-ui/releases/latest/download/l-ui-hub-linux-amd64.tar.gz
   ```
2. Extract to `/usr/local`:
   ```bash
   tar -xzf l-ui-hub.tar.gz -C /usr/local
   # Creates /usr/local/l-ui-hub/
   ```
3. Install the systemd service:
   ```bash
   cp /usr/local/l-ui-hub/l-ui.service /etc/systemd/system/l-ui.service
   systemctl daemon-reload
   systemctl enable --now l-ui
   ```
4. Verify:
   ```bash
   systemctl status l-ui
   journalctl -u l-ui -n 20
   ```

The hub listens on port `2053` by default (configurable via `LUI_WEB_PORT`).

## Agent Installation

Agents are installed via the hub panel in two ways:

### Method 1: SSH Bootstrap (Recommended)

1. In the hub panel, go to **Nodes → Add Node**
2. Enter the VPS IP, SSH credentials, and desired agent port
3. Click **Bootstrap** — the hub provisions the agent over SSH
4. The bootstrap timeline shows progress in real-time

The agent is installed to `/usr/local/l-ui-agent/` with systemd service `l-ui-agent.service`.

### Method 2: Registration Token

1. In the hub panel, go to **Nodes → Add Node -> Generate Token**
2. Copy the displayed one-time `curl | sh` command
3. Run it on the target VPS
4. The agent registers itself with the hub and appears in the node list

## Docker Deployment

```yaml
version: "3.8"
services:
  l-ui:
    image: ghcr.io/drunkleen/l-ui:latest
    ports:
      - "2053:2053"
    volumes:
      - l-ui-data:/etc/l-ui
      - l-ui-logs:/var/log/l-ui
      - l-ui-cert:/var/lib/l-ui/cert
    environment:
      - LUI_WEB_PORT=2053
      - LUI_DB_TYPE=postgres
      - LUI_DB_DSN=host=db ...
    depends_on:
      db:
        condition: service_healthy

  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: l-ui
      POSTGRES_PASSWORD: secret
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]

volumes:
  l-ui-data:
  l-ui-logs:
  l-ui-cert:
  pgdata:
```

**Note:** Docker deployments are hub-only. Agents are installed on separate VPS nodes via SSH bootstrap.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LUI_WEB_PORT` | `2053` | HTTP listen port |
| `LUI_DB_FOLDER` | `/etc/l-ui` | Database file location (SQLite) |
| `LUI_DB_TYPE` | `sqlite` | Database type: `sqlite`, `postgres`, `mysql` |
| `LUI_DB_DSN` | — | Database connection string (for postgres/mysql) |
| `LUI_LOG_FOLDER` | `/var/log/l-ui` | Log file directory |
| `LUI_DEBUG` | `false` | Enable debug mode (verbose logging, Vite dev mode) |
| `LUI_MAIN_FOLDER` | `<binary_dir>` | Install directory for binaries and config |
| `LUI_BIN_FOLDER` | `<main_dir>/bin` | Directory for xray binary and config files |
| `LUI_BOOTSTRAP_API_TOKEN` | — | Pre-shared token for new node registrations |
| `LUI_REGISTRATION_TOKEN` | — | One-time token for registration flow |
| `LUI_HUB_ENDPOINT` | — | Hub API endpoint (for agent registration) |
| `LUI_CERT_DIR` | — | TLS certificate directory |
| `LUI_SKIP_HSTS` | `false` | Skip HSTS header in web panel responses |

## Local Development

```bash
make dev
```

This builds the Go backend, starts the server on port 2053, waits for readiness, and starts Vite on port 5173 with HMR. Runtime data is stored in `./tmp/`.

Local development login: `admin` / `admin`.

## Installer And CLI

The `install.sh` script:
1. Detects the OS and architecture
2. Downloads the latest hub release tarball
3. Installs the binary and systemd service
4. Prints available CLI commands

The hub CLI supports: `start`, `stop`, `restart`, `status`, `settings`, `enable`, `disable`, `log`, `banlog`, `update`, `legacy`, `install`, `uninstall`.

The agent has no CLI — it starts automatically via systemd.

## Production Releases

- GitHub Actions builds release tarballs for every version tag (`v1.2.3`)
- Hub tarball: `l-ui-hub-linux-{arch}.tar.gz` — extracts to `l-ui-hub/`
- Agent tarball: `l-ui-agent-linux-{arch}.tar.gz` — extracts to `l-ui-agent/`
- Docker images: `ghcr.io/drunkleen/l-ui:{version}`
- Windows builds are available as `.zip` archives (hub only, no service support)

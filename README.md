[English](/README.md)

<h1 align="center">L-UI</h1>
<h3 align="center">One Dashboard. Multiple Xray Panels.</h1>

<p align="center">
  <a href="https://github.com/drunkleen/l-ui/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/drunkleen/l-ui/ci.yml.svg?label=CI" alt="CI"></a>
  <a href="https://github.com/drunkleen/l-ui/releases"><img src="https://img.shields.io/github/v/release/drunkleen/l-ui" alt="Release"></a>
  <a href="https://pkg.go.dev/github.com/drunkleen/l-ui"><img src="https://pkg.go.dev/badge/github.com/drunkleen/l-ui.svg" alt="Go Reference"></a>
  <a href="https://goreportcard.com/report/github.com/drunkleen/l-ui"><img src="https://goreportcard.com/badge/github.com/drunkleen/l-ui" alt="Go Report Card"></a>
</p>

L-UI is a hub for managing remote VPS nodes and their Xray instances. The hub does not run Xray itself; it provisions nodes over SSH/SCP, installs a lightweight agent, and controls nodes through signed APIs.

<p align="center">
  <img alt="l-ui" src="./media/l-ui.png" style="width: 90%;">
</p>

## What This Repo Contains

- `hub/` — hub server: Gin controllers, services, cron jobs, and the embedded frontend
- `agent/` — standalone lightweight agent binary deployed to remote VPS nodes
- `internal/` — shared packages: database models, config, certgen, retry, bundle, SSH utils, auth
- `frontend/` — React + Ant Design admin panel (multi-page Vite app)
- `web/dist/` — generated frontend build, embedded into the hub binary
- `docs/` — architecture, install, bootstrap, API, and troubleshooting notes
- `Makefile` — the main developer and release command surface

## Quick Start

```bash
bash <(curl -Ls https://raw.githubusercontent.com/drunkleen/l-ui/master/install.sh)
```

## Development

Use the Makefile from the repo root:

| Command | Purpose |
|---|---|
| `make dev` | Start the Go backend first, then Vite, with local `./tmp` runtime files |
| `make build` | Build the frontend bundle and both Go binaries (hub + agent) |
| `make build-hub` | Build the hub binary only |
| `make build-agent` | Build the agent binary only |
| `make test` | Run backend + frontend tests |
| `make test-back` | Run repo-wide Go tests |
| `make test-front` | Run the frontend Vitest suite |
| `make lint` | Run Go vet and frontend lint |
| `make typecheck` | Run the frontend TypeScript typecheck |
| `make gen-api` | Regenerate frontend OpenAPI artifacts |
| `make gen-zod` | Regenerate frontend Zod/types artifacts |
| `make clean` | Remove local build artifacts |

`make dev` is the preferred local development path. It prints backend compile progress, waits for readiness, and then starts Vite.

Local development login: `admin` / `admin`.

## Installer And CLI

- `install.sh` is the production installer
- It downloads a release bundle, installs the service, and prints a command reference at the end
- The installed `l-ui` wrapper is the service entrypoint for `start`, `stop`, `restart`, `status`, `settings`, `enable`, `disable`, `log`, `banlog`, `update`, `legacy`, `install`, and `uninstall`
- The Go binary supports `run`, `agent`, `migrate`, `migrate-db`, `setting`, and `cert` subcommands

## Build and Release

- `make build` produces the frontend assets under `hub/web/dist/` and both Go binaries under `bin/`
- GitHub Actions runs CI on pushes to `main` and pull requests
- Tagged pushes like `v1.2.3` trigger release packaging and published artifacts
- Docker images are published on version tags too

### CI/CD Flow

- `ci.yml` validates Go tests, frontend lint/typecheck/tests, and a frontend build on PRs and `main`
- `release.yml` builds the release tarballs and Windows zip assets
- `docker.yml` publishes production container images for version tags

## Database

- SQLite is the default
- Postgres is supported for hosted or multi-node deployments
- MySQL/MariaDB is supported through the shared GORM storage layer

See [`docs/install.md`](./docs/install.md) for the deployment model and storage layout.

## Hub / Agent Model

- The hub owns nodes, inbounds, clients, routing, and assignments
- The agent is a lightweight binary deployed to each VPS; it runs Xray and exposes a management API
- The hub provisions agents over SSH/SCP and communicates via signed HTTP after bootstrap
- The hub never runs Xray itself — it is a pure control plane

## Features

- **Node bootstrap** — SSH-based provisioning with retry logic, Caddy TLS support, and async job tracking
- **Node monitoring** — heartbeat polling (with retry), CPU/MEM/network/disk sparkline charts, configurable alert thresholds with Telegram notifications
- **Config push** — push Xray config and client lists to nodes with version tracking and drift detection
- **TLS certificates** — built-in CA for agent certificate generation, push, and daily auto-renewal
- **UFW firewall management** — view, add, delete rules on nodes; auto-open/close ports for Xray inbounds; port group management
- **Node registration tokens** — one-time curl\|sh registration flow for nodes without SSH
- **Subscription endpoints** — standard Xray subscription URL generation
- **Telegram bot** — notifications for node down, resource thresholds, login events, and database backup

## Docs

- [`docs/architecture.md`](./docs/architecture.md)
- [`docs/bootstrap.md`](./docs/bootstrap.md)
- [`docs/install.md`](./docs/install.md)
- [`docs/node-api.md`](./docs/node-api.md)
- [`docs/troubleshooting.md`](./docs/troubleshooting.md)
- [`CONTRIBUTING.md`](./CONTRIBUTING.md)

## Support

<p align="center">
  <a href="https://www.patreon.com/DrunkLeen" target="_blank">
    <img src="./media/patreon.png" alt="Patreon" style="height: 70px !important;width: 250px !important;" />
  </a>
  <a href="https://www.buymeacoffee.com/drunkleen" target="_blank">
    <img src="./media/buy-me-a-coffee.png" alt="Buy Me A Coffee" style="height: 70px !important;width: 250px !important;" />
  </a>
</p>

### Crypto

- BTC: `bc1qsmvxpn79g6wkel3w67k37r9nvzm5jnggeltxl6`
- ETH (ERC20): `0x8613aD01910d17Bc922D95cf16Dc233B92cd32d6`
- USDT (TRC20): `0x8613aD01910d17Bc922D95cf16Dc233B92cd32d6`
- DOGE: `D8U25FjxdxdQ7pEH37cMSw8HXBdY1qZ7n3`
- TRX: `TGNru3vuDfPh5zBJ31DKzcVVvFgfMK9J48`

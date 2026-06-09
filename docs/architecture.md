# Architecture

## Hub / Agent Split

L-UI follows a strict control-plane / data-plane split:

- **Hub** is the central panel. It manages nodes, inbounds, clients, routing, and assignments. It never runs Xray itself.
- **Agent** is a lightweight binary deployed to each VPS node. It exposes a signed HTTP API and manages local Xray via systemd.

Communication between hub and agent uses HMAC-SHA256 request signing:

- Every HTTP request includes: `Authorization: Bearer <token>`, `X-LUI-Timestamp`, `X-LUI-Nonce`, `X-LUI-Signature`, `X-LUI-Body-SHA256`
- The hub verifies the agent's token before accepting commands
- The agent verifies incoming requests with constant-time HMAC comparison

## Data Flow

```
User (Browser)
    │
    ├── HTTPS ──> Hub Web API (Fiber v3, port 2053)
    │                │
    │                ├── JWT auth ──> Session ──> Controllers ──> Services ──> SQLite/Postgres
    │                │
    │                ├── HMAC-signed HTTP ──> Agent API (Fiber v3, port 2054+)
    │                │                           │
    │                │                           ├── HMAC auth middleware
    │                │                           ├── Xray management (install, config, restart)
    │                │                           ├── UFW firewall control
    │                │                           ├── System metrics / status
    │                │                           └── Bundle updates
    │                │
    │                └── WebSocket ──> Real-time updates (traffic, node health)
    │
    └── SSH/SCP ──> Node bootstrap (provisioning new agents)
```

## Frontend Architecture

The frontend is a React 19 + Ant Design 6 + TypeScript 6 + Vite 8 application with multiple HTML entry points:

- `index.html` — admin panel (routes under `/panel/*`)
- `login.html` — login and 2FA
- `subpage.html` — public subscription viewer

The frontend is built into `hub/web/dist/` and embedded into the Go binary via `//go:embed`.

### Key patterns

- **TanStack React Query** for server state (queries + mutations)
- **Ant Design** for UI components (tables, forms, modals, steps)
- **Zod** for schema validation (shared between forms and API responses)
- **i18n** via `react-i18next` with JSON translation files
- **WebSocket** for real-time traffic and online status updates
- **Dark theme** via Ant Design ConfigProvider tokens

## Startup Order

1. Parse config file and CLI flags
2. Initialize database (SQLite/Postgres/MySQL)
3. Run migrations
4. Start cron scheduler (heartbeat, traffic collection, alerts, cert renewal)
5. Start Fiber HTTP server
6. Start Telegram bot (if enabled)
7. Start WebSocket hub for real-time updates

## Node Lifecycle

```
Registered ──> Online ──> Config Pushed ──> Drift ──> Offline ──> Reconciled ──> Deleted
     │            │             │                │           │            │
     │            │             │                │           │            │
  Bootstrap   Heartbeat      pushConfig       version     5s no      reinstall
  completes   received       applied          mismatch    heartbeat   or cleanup
```

## Bootstrap Process

1. User submits SSH credentials and node config via the frontend
2. Hub opens an SSH connection to the target VPS
3. Downloads the agent release tarball from GitHub
4. Extracts and installs the agent binary
5. Creates systemd service and env file
6. Starts the agent and writes xray config
7. Verifies the agent responds to signed API calls
8. Persists the node record in the database

See [`bootstrap.md`](./bootstrap.md) for the full step list and error handling.

## Monitoring

- **Heartbeat** — every 5 seconds, hub probes `GET /api/v1/status` on each node
- **Traffic sync** — every 5 seconds, hub fetches traffic snapshots from nodes
- **Alerts** — every 10 seconds, checks CPU, memory, disk thresholds; sends Telegram notifications
- **Xray crash detection** — periodic check, auto-restart on consecutive crash detection

## TLS Certificate Flow

1. Hub generates a CA on startup (or loads existing)
2. When enrolling a node, hub generates a node-specific certificate signed by the CA
3. Hub pushes the certificate pair to the agent via `POST /api/v1/certs`
4. Agent stores the cert/key and uses them for its TLS listener
5. A daily cron job checks certificate expiry and auto-renews

## Retry Architecture

- **Transient errors** (connection refused, timeout, TLS handshake, 5xx, 429) are retried with exponential backoff
- `retry.DefaultConfig`: 3 attempts, 200ms initial backoff, 5s max backoff, 0.2 jitter
- **Idempotent operations** (GET, DELETE, config push, restart) use retry
- **Non-idempotent operations** (create inbound, add rule) do not retry

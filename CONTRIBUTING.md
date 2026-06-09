# Contributing

Thanks for taking the time to contribute to L-UI.

## Before You Start

You will need:

- Go 1.26+
- Node.js 22+ and npm 10+
- Git
- A C compiler for `github.com/mattn/go-sqlite3`

## Local Setup

The recommended local workflow:

```bash
make dev
```

That target:

- builds the Go backend
- starts the backend first
- waits for `http://127.0.0.1:2053`
- starts Vite on `http://localhost:5173`
- stores local runtime data in `./tmp/`

Local development login: `admin` / `admin`.

Useful commands:

| Command | Purpose |
|---|---|
| `make dev` | Local backend-first dev loop |
| `make build` | Build frontend assets and Go binary |
| `make test` | Run backend + frontend tests |
| `make test-back` | Run Go tests |
| `make test-front` | Run frontend Vitest |
| `make lint` | Run Go vet and frontend lint |
| `make typecheck` | Run frontend TypeScript checks |
| `make clean` | Remove build output |

## Recommended Checks

Run the checks that match what you changed:

- Go/backend changes: `make test-back` and `make lint`
- Frontend changes: `cd frontend && npm run typecheck && npm run lint && npm test && npm run build`
- Docs-only changes: at minimum skim the rendered markdown and verify the command names are correct

## Windows Notes

`go build` on Windows requires a working GCC toolchain because of the SQLite CGO dependency. MinGW-w64 or MSYS2 both work.

If you hit `cgo: C compiler "gcc" not found`, install a compiler and confirm `gcc --version` in a new terminal.

## Frontend Workflow

The frontend is a multi-entry Vite app under `frontend/`.

- `npm run dev` starts the UI in HMR mode
- `npm run build` generates `hub/web/dist/`
- `npm run test` runs Vitest
- `npm run lint` runs ESLint
- `npm run typecheck` runs TypeScript

See [`frontend/README.md`](./frontend/README.md) for the source layout and script details.

## Backend Workflow

The backend is a Go module at the repo root with three main entry points:

- `hub/main.go` — the hub server
- `agent/main.go` — the agent binary
- `hub/cmd/` — cobra CLI commands

Key packages:

| Package | Purpose |
|---------|---------|
| `hub/web/service/` | Business logic (xray, node, inbound, config, firewall) |
| `hub/web/controller/` | Fiber HTTP handlers |
| `hub/web/runtime/` | Hub-to-agent HTTP client (HMAC-signed) |
| `agent/service/` | Agent-side services (xray install, config apply, firewall) |
| `agent/controller/` | Agent HTTP handlers |
| `internal/` | Shared packages (config, auth, SSH, bundle, UFW, retry) |
| `xray/` | Xray core primitives (config structs, process, gRPC API) |

## Framework Conventions

- Backend uses **Fiber v3** (`github.com/gofiber/fiber/v3`)
- All handler functions return `fiber.Handler` as `func(c fiber.Ctx) error`
- JSON responses use `c.Status(code).JSON(obj)`
- Request body binding uses `c.Bind().Body(&dst)` or `c.Bind().JSON(&dst)`
- Session management uses Fiber's session middleware
- Frontend uses **React 19 + Ant Design 6**
- Query state is managed with **TanStack React Query**
- Validation uses **Zod** (frontend) and `go-playground/validator` (backend)

## Docs Workflow

Keep the top-level README, the docs in `docs/`, and the frontend README in sync when you change workflow, commands, or architecture.

- `README.md` is the entry point for contributors
- `CONTRIBUTING.md` explains how to work safely in the repo
- `docs/*.md` explain runtime behavior and operational details

## Pull Requests

1. Branch from `main`.
2. Keep the diff focused.
3. Run the relevant checks.
4. Use commit messages in the `area: summary` style.
5. Describe how to verify the change in the PR body.

## CI/CD And Releases

- PRs and pushes to `main` run the CI workflow
- Tag pushes like `v1.2.3` publish release artifacts and Docker images
- If you touch release or installer behavior, update the docs and the release notes together

## Runtime Paths

- Local dev runtime data should stay under `./tmp/`
- Hub production defaults: `/etc/l-ui` for the database, `/var/log/l-ui` for logs
- Agent production install: `/usr/local/l-ui-agent/`
- Hub production install: `/usr/local/l-ui-hub/`
- Do not commit runtime files, generated bundles, or local databases

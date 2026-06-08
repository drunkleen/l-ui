# Contributing

Thanks for taking the time to contribute to L-UI.

## Before You Start

You will need:

- Go 1.26+
- Node.js 22+ and npm 10+
- Git
- A C compiler for `github.com/mattn/go-sqlite3`

## Local Setup

The recommended local workflow is the Makefile.

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

The frontend is a multi-page Vite app under `frontend/`.

- `npm run dev` starts the UI in HMR mode
- `npm run build` generates `web/dist/`
- `npm run test` runs Vitest
- `npm run lint` runs ESLint
- `npm run typecheck` runs TypeScript

See [`frontend/README.md`](./frontend/README.md) for the source layout and script details.

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
- Production defaults are `/etc/l-ui` for the database and `/var/log/l-ui` for logs
- Do not commit runtime files, generated bundles, or local databases

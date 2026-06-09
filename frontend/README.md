# l-ui frontend

The frontend is a React 19 + Ant Design 6 + TypeScript 6 + Vite 8 app.
It builds into `../hub/web/dist/` and is embedded into the Go binary.

## What Lives Here

- `index.html` — admin panel routes under `/panel/*`
- `login.html` — login and 2FA
- `subpage.html` — public subscription viewer

## Development

Use the repo-level Makefile for the full stack:

- `make dev` starts the backend first, waits for readiness, then starts Vite

For frontend-only work:

```sh
cd frontend
npm install
npm run dev
```

Vite serves on `http://localhost:5173/` and proxies API calls to the Go panel on `http://localhost:2053/`.

## Scripts

| Command | What |
|---|---|
| `npm run dev` | Vite dev server with API + WS proxy to Go |
| `npm run build` | Regenerates OpenAPI + Zod, then builds into `../hub/web/dist/` |
| `npm run preview` | Serve the built bundle locally |
| `npm run typecheck` | `tsc --noEmit` |
| `npm run lint` | ESLint flat config |
| `npm run test` | Vitest single run |
| `npm run test:watch` | Vitest watch mode |
| `npm run gen:api` | Build `public/openapi.json` from the API docs source |
| `npm run gen:zod` | Regenerate `src/generated/{zod,types}.ts` from Go |

## Build Output

`npm run build` outputs to `../hub/web/dist/` with hashed assets under `assets/`.
The Go binary embeds this directory for release builds.

## Layout

```
frontend/
├── index.html            # Admin panel entry
├── login.html            # Login page entry
├── subpage.html          # Subscription viewer entry
├── tsconfig.json
├── eslint.config.js
├── vite.config.js
└── src/
    ├── entries/           # Entry point bootstraps
    ├── pages/             # Page components
    │   ├── nodes/         # Node management (list, form, bootstrap timeline)
    │   ├── inbounds/      # Inbound management
    │   ├── clients/       # Client management
    │   ├── settings/      # Panel settings
    │   ├── index/         # Dashboard overview
    │   └── api-docs/      # OpenAPI viewer
    ├── components/        # Shared components (form, JSON editor, charts)
    ├── hooks/             # React hooks (theme, settings, i18n)
    ├── api/               # API layer (axios, queries, mutations, websocket)
    ├── i18n/              # Internationalization (react-i18next)
    ├── schemas/           # Zod validation schemas (source of truth)
    ├── generated/         # Auto-generated Zod + TypeScript from Go
    ├── styles/            # Global CSS (utils, page-shell, page-cards)
    ├── test/              # Test setup and fixtures
    └── utils/             # Utilities (HTTP, format, Zod helpers)
```

## Key Rules

- Prefer TypeScript strict-mode friendly code
- Keep the UI on Ant Design 6
- Use function components and hooks
- Keep generated files out of hand edits
- Treat `src/schemas/` as the source of truth for API and form validation
- Dark theme via Ant Design ConfigProvider tokens + CSS `.is-dark` class overrides

## UI Patterns

- **Forms**: compact layout (8px item margin, 12px labels, 13px inputs)
- **Modals**: minimal padding (16px/20px), inline error display
- **Steps/timeline**: Ant Design Steps, horizontal with wrapping, custom colored icons (green check / red cross)
- **Tables**: Ant Design Table with search/sort/filter, compact size
- **Charts**: Recharts sparklines for CPU, memory, network, disk
- **Dark theme**: Catppuccin-inspired palette, CSS variable overrides

## Testing

Vitest covers schema fixtures, parser behavior, and page-level utilities.
Regenerate snapshots after intentional changes: `npx vitest run -u`

## Adding a Page

1. Add the page component under `src/pages/<page>/`.
2. Register it in `src/routes.tsx` if it lives under `/panel/...`.
3. For a new standalone entry, add the HTML file, bootstrap entry, and Vite input.
4. Wire the matching Go controller to serve the HTML in production.

# l-ui frontend

The frontend is a React 19 + Ant Design 6 + TypeScript + Vite 8 app.
It builds into `../web/dist/` and is embedded into the Go binary.

## What Lives Here

- `index.html`: admin panel routes under `/panel/*`
- `login.html`: login and 2FA
- `subpage.html`: public subscription viewer

## Development

Use the repo-level Makefile for the full stack:

- `make dev` starts the backend first, waits for readiness, then starts Vite

For frontend-only work:

```sh
npm install
npm run dev
```

Vite serves on `http://localhost:5173/` and proxies API calls to the Go panel on `http://localhost:2053/`.

## Scripts

| Command | What |
|---|---|
| `npm run dev` | Vite dev server with API + WS proxy to Go |
| `npm run build` | Regenerates OpenAPI + Zod, then builds into `../web/dist/` |
| `npm run preview` | Serve the built bundle locally |
| `npm run typecheck` | `tsc --noEmit` |
| `npm run lint` | ESLint flat config |
| `npm run test` | Vitest single run |
| `npm run test:watch` | Vitest watch mode |
| `npm run gen:api` | Build `public/openapi.json` from the API docs source |
| `npm run gen:zod` | Regenerate `src/generated/{zod,types}.ts` from Go |

## Build Output

`npm run build` outputs to `../web/dist/` with hashed assets under `assets/`.
The Go binary embeds this directory for release builds.

## Layout

```
frontend/
├── *.html
├── tsconfig.json
├── eslint.config.js
├── vite.config.js
└── src/
    ├── entries/
    ├── pages/
    ├── components/
    ├── hooks/
    ├── api/
    ├── i18n/
    ├── models/
    ├── schemas/
    ├── generated/
    ├── styles/
    ├── test/
    └── utils/
```

## Key Rules

- Prefer TypeScript strict-mode friendly code
- Keep the UI on Ant Design 6
- Use function components and hooks
- Keep generated files out of hand edits
- Treat `src/schemas/` as the source of truth for API and form validation

## Testing

Vitest covers schema fixtures, parser behavior, and page-level utilities.

```sh
npm run test
npm run typecheck
npm run lint
```

Regenerate snapshots after intentional changes:

```sh
npx vitest run -u
```

## Adding a Page

1. Add the page component under `src/pages/<page>/`.
2. Register it in `src/routes.tsx` if it lives under `/panel/...`.
3. For a new standalone entry, add the HTML file, bootstrap entry, and Vite input.
4. Wire the matching Go controller to serve the HTML in production.

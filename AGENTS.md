# AGENTS.md

## Quick Reference
- Go entrypoint: `main.go`; Frontend: `frontend/` (Vite app → `web/dist/` → embedded by Go)
- Frontend commands in `frontend/`: `npm ci`, `npm run dev`, `npm run typecheck`, `npm test`, `npm run build`
- Go tests: `mkdir -p web/dist && touch web/dist/.gitkeep && go test $(go list ./... | grep -v '/frontend/node_modules/')`
- `make dev`, `make test`, `make test-back`, `make test-front`, `make gen-api`, `make gen-zod`, `make build`, `make clean`, `make release`
- Go 1.26.4, Node 22, npm 10+
- Default DB: SQLite at `/etc/l-ui/l-ui.db`; Postgres via `LUI_DB_TYPE=postgres` + `LUI_DB_DSN`

## Architecture
- **Hub-first control plane**: hub manages VPS nodes over SSH/SCP + node APIs; nodes are lightweight agents
- **Three Vite entry pages**: `index.html` (`/panel/*`), `login.html`, `subpage.html`
- **Release bundles**: hub/agent split tarballs per arch, versioned+arch cached in hub DB, downloaded from GitHub release assets
- **Supported arches**: amd64, arm64, 386, armv7, armv6, armv5, s390x
- **CLI menus**: Bubble Tea TUI (`hub/cmd/menu.go`); `l-ui install`, `l-ui update`, `l-ui uninstall` subcommands in Go
- **Shell scripts are thin wrappers**: `install.sh` (50 lines, arch→download→exec), `l-ui.sh` (delegates to Go binary)

## Frontend Theme (Catppuccin)
- **🌙 Mocha** → Dark mode (default), **🌌 Frappé** → Soft dark mode, **☀️ Latte** → Light mode
- 3-theme cycle: Mocha → Frappé → Latte → Mocha (via single `toggleTheme()` call)
- Theme based on `useTheme.tsx` hook with Ant Design `ConfigProvider` + CSS classes (`body.dark/.light`, `[data-theme="frappe"]`)
- Key Catppuccin colors: Mocha Base `#1e1e2e`, Surface0 `#313244`, Text `#cdd6f4`, Blue `#89b4fa`, Green `#a6e3a1`, Red `#f38ba8`
- Frappé: Base `#303446`, Text `#c6d0f5`, Blue `#8caaee`, Green `#a6d189`
- Latte: Base `#eff1f5`, Text `#4c4f69`, Blue `#1e66f5`, Green `#40a02b`
- CSS files with theme overrides: `AppSidebar.css`, `page-shell.css`, `LogModal.css`, `XrayLogModal.css`, `NodeFormModal.css`
- Component-level themes: `DateTimePicker.tsx` (3 themes), `JsonEditor.tsx` (CodeMirror dark themes)

## Project State
- All Go packages pass with 0 failures (35+ packages)
- Single `release.yml` workflow (Docker multi-arch build commented out)
- Bootstrap flow: `StartBootstrap` uses `context.Background()`, agent API at `/api/v1/status`
- Service files patched at install time with `run` subcommand in `ExecStart`
- `.github/workflows/release.yml`: 7-arch binary release; Docker job commented out

## Workflow (ALL agents MUST follow)

### 1. Full Problem Analysis ("fix it")
When user says "fix it" after describing a problem:
**keep the steps (2. Consult First, 3. Plan Before Code, 4. Part-by-Part TDD + Audit and Test Loop, 5. Final Sweep) always in mind.**
1. **Identify the bug/issue** — find the exact root cause
2. **Map all affected code paths** — every function/file the bug touches, every caller, every integration point
3. **Identify side effects** — what else this change might break, what depends on the current behavior
4. **List all symptoms** — not just the reported symptom, but related misbehavior that shares the same root
5. Present findings to user before proceeding (if scope is unclear)

### 2. Consult First
When asked to do/change/fix anything, **stop and consult the user**:
- Ask clarifying questions about what they want
- Present options with trade-offs
- Mark the best/recommended approach
- Wait for user decision before proceeding

### 3. Plan Before Code
Write the plan in `PLAN.md` before touching any code:
- Outline every step with files to change
- Document **what each change affects** and what might break
- List dependencies between steps
- Add test strategy for each step
- If audit during step N uncovers a bug, add it to the plan before fixing

### 4. Part-by-Part TDD + Audit and Test Loop
Execute the plan **one part at a time**. For each part:

1. **Write tests first** for the code you're about to create/change
2. **Run tests** — they should fail (new tests) or pass baseline
3. **Implement the change** — make the tests pass
4. **Run tests again** to verify
5. **Audit** the code you just wrote/changed for misbehavior or bugs:
   - Run full test suite (`go test ./...` / `npm test`)
   - Run type checker (`npm run typecheck`)
   - Search for edge cases, nil panics, race conditions, error handling gaps
   - Check that pre-existing behavior still works
6. **If audit finds a bug or misbehavior** → add it to PLAN.md → plan its fix → fix → audit loop
7. **Mark part complete** in PLAN.md, move to next part

### 5. Final Sweep
When all parts are done:
1. Run full test suite and type checker
2. **Audit across ALL completed parts** for misbehavior, regressions, or missed edge cases
3. **Fix → audit → fix → audit loop** until everything is perfect
4. Report results to user with a short summary

### Hard Rules
- Never edit `frontend/public/openapi.json` or `frontend/src/generated/{zod,types}.ts` by hand; use `npm run gen:api` / `npm run gen:zod`
- Verify tags/asset naming before editing release workflows
- Keep PLAN.md limited to open work only; durable guidance here
- After each milestone: update PLAN.md, run tests, prompt user with 2-5 line summary

### Git Rules
- When user says "commit", commit immediately. If changes span multiple logical concerns (frontend theme ≠ backend bootstrap logic), split into separate commits per concern.
- Before committing: inspect `git status`, `git diff`, `git log --oneline -10`; stage only intended files.
- Write a concise commit message matching repo style (imperative, prefix like `feat:` / `fix:` / `refactor:`).
- Never force-push, amend, or create empty commits unless explicitly told.

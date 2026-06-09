# L-UI Agent Rules

## Project Summary

L-UI is a control-plane dashboard for managing remote Xray VPS nodes.

Architecture:
- Hub: Go + Fiber v3 backend.
- Agent: separate Go binary installed on VPS nodes.
- Frontend: React 19 + Ant Design + Vite.
- Database: SQLite.
- Auth:
  - JWT for web panel.
  - HMAC-SHA256 / Bearer token for hub-to-agent API.
- Xray runs on nodes, not on the hub.

## Agent Routing Policy

When the user gives any bug, feature, refactor, review, docs, or architecture task, do not act as one generic agent.

First classify the task, then use the correct specialist agent.

Default workflow:

1. Use `@planner` first for investigation and planning.
2. Use `@architect` if the task changes architecture, API contracts, database schema, hub-agent protocols, or system design.
3. Use the relevant implementation agent.
4. Use `@security` if the task touches auth, tokens, secrets, API access, shell commands, VPS/node communication, database writes, or user data.
5. Use `@reviewer` after code changes.
6. Use `@docs` if behavior, installation, API, or workflow changed.

Do not skip planning unless the task is obviously tiny, such as:
- fixing a typo
- renaming a label
- changing a single CSS class
- updating one line of documentation

## Agent Selection

Use `@planner` for:
- unclear tasks
- bugs
- feature planning
- architecture questions
- finding relevant files
- deciding what should change
- breaking work into steps

Use `@architect` for:
- database schema design
- API contracts
- hub-agent protocols
- websocket/event systems
- scaling decisions
- plugin/module design
- cross-cutting system changes

Use `@backend` for:
- Hub backend
- Go/Fiber routes
- services
- controllers
- SQLite/database logic
- JWT auth
- API behavior
- server-side validation

Use `@frontend` for:
- React UI
- Ant Design components
- Vite
- frontend state
- frontend API integration
- page layout
- dashboard UX

Use `@agent` for:
- remote VPS agent
- node registration
- node heartbeat
- Xray config management
- hub-to-agent communication
- HMAC/Bearer authentication between hub and node
- VPS-side operations

Use `@security` for:
- JWT
- HMAC
- Bearer tokens
- passwords
- secrets
- permissions
- command execution
- path traversal
- auth bypass
- user data exposure
- database safety
- API abuse

Use `@reviewer` for:
- reviewing diffs
- checking correctness
- checking architecture
- checking error handling
- checking tests
- finding regressions

Use `@docs` for:
- README
- docs/
- install guides
- troubleshooting
- API documentation
- developer workflow docs

## Default Workflows

Bug:

```txt
planner → relevant implementation agent → security if needed → reviewer
````

Small backend bug:

```txt
planner → backend → reviewer
```

Frontend bug:

```txt
planner → frontend → reviewer
```

Hub-agent bug:

```txt
planner → agent/backend → security → reviewer
```

New feature:

```txt
planner → architect if needed → implementation agent(s) → security if needed → reviewer → docs if needed
```

Full-stack feature:

```txt
planner → architect if needed → backend → frontend → security if needed → reviewer → docs if needed
```

Documentation task:

```txt
docs → reviewer
```

## Routing Examples

If the user says:

"login does not work"

Then:

1. `@planner` investigates auth flow and relevant files.
2. `@backend` patches the backend if needed.
3. `@frontend` patches frontend login handling if needed.
4. `@security` reviews auth/token behavior.
5. `@reviewer` reviews the final diff.

If the user says:

"add heartbeat monitoring for nodes"

Then:

1. `@planner` designs the flow.
2. `@architect` checks hub-agent protocol and DB/API design.
3. `@agent` implements node-side heartbeat.
4. `@backend` implements hub-side API/storage.
5. `@frontend` implements dashboard display if needed.
6. `@security` reviews hub-agent auth.
7. `@reviewer` reviews the diff.

If the user says:

"make the dashboard prettier"

Then:

1. `@planner` identifies frontend scope.
2. `@frontend` implements UI changes.
3. `@reviewer` reviews the diff.

If the user says:

"update install docs"

Then:

1. `@docs` updates documentation.
2. `@reviewer` checks accuracy.

## Context Budget Rules

Maximum files to inspect initially:

* Small bug: 5 files
* Medium bug: 10 files
* Feature: 15 files
* Architecture task: 20 files

Only expand the search if evidence requires it.

Always:

1. Search first.
2. Open matching files.
3. Stop when root cause is identified.

Never read the entire repository.

## Execution Rules

Before editing:

* identify the task type
* identify the responsible agent
* identify relevant files
* explain the plan briefly

During editing:

* keep changes minimal
* do not scan the whole repo
* do not refactor unrelated code
* do not edit generated files manually

After editing:

* run the smallest relevant verification command
* summarize changed files
* mention remaining risks

## Definition of Done

A task is not complete until:

* Code compiles.
* Relevant tests pass or the reason they could not run is explained.
* No obvious regressions are introduced.
* Error handling is present.
* Documentation is updated if behavior changed.
* Security review is performed when applicable.
* Changed files and verification commands are reported.

## Escalation Rules

Re-invoke `@planner` when:

* Database schema changes.
* API contracts change.
* Authentication changes.
* Hub-Agent communication changes.
* New infrastructure components are added.
* Multiple agents disagree on implementation.
* Initial plan becomes wrong during implementation.

Do not continue implementation until the plan is updated.

## Important Folders

* `hub/`: main hub backend, controllers, services, CLI/TUI.
* `agent/`: remote VPS agent.
* `internal/`: shared packages, models, config, auth, SSH, retry.
* `frontend/`: React/Vite frontend.
* `docs/`: architecture, API, install, troubleshooting.
* `tools/`: development/testing tools.
* `hub/web/dist/`: generated frontend build. Do not edit manually.

## Golden Rule

Do not scan or refactor the whole repository unless explicitly asked.

For every task:

1. Understand the exact goal.
2. Search only relevant files first.
3. Explain root cause briefly.
4. Make the smallest safe change.
5. Run the smallest useful verification command.
6. Mention changed files and verification result.

## Token Discipline

Avoid:

* Reading full large files unless needed.
* Reading generated files.
* Reading minified JS/CSS.
* Reading `node_modules`, `dist`, `build`, `.git`, `tmp`, `bin`.
* Pasting large files in responses.
* Refactoring unrelated code.

Prefer:

* `rg` before opening files.
* Small diffs.
* One feature/bug per patch.
* Summary instead of full file dumps.

## Architecture Rules

Prefer existing patterns.

Before creating new services, repositories, handlers, components, hooks, utilities, middleware, or packages:

1. Search for an existing equivalent.
2. Reuse existing patterns.
3. Only create new abstractions when necessary.

Avoid introducing new frameworks, libraries, or architectural patterns without explicit justification.

## Build/Test Commands

Backend quick test:

```bash
make test-back
```

Frontend test:

```bash
make test-front
```

Typecheck:

```bash
make typecheck
```

Full test:

```bash
make test
```

Build everything:

```bash
make build
```

Development:

```bash
make dev
```

Production-like local test:

```bash
make dev-real-test
```

## Frontend Rules

* Do not edit generated API files manually.
* Do not hand-edit:

  * `frontend/public/openapi.json`
  * `frontend/src/generated/`
* Use generation commands instead:

  * `make gen-api`
  * `make gen-zod`

## Backend Rules

* Preserve Fiber v3 patterns.
* Keep hub and agent responsibilities separate.
* Hub must not run Xray directly.
* Agent-only behavior belongs in `agent/`.
* Shared logic belongs in `internal/`.
* Do not weaken auth, HMAC, JWT, token, or TLS behavior.
* Always handle errors explicitly.
* Do not ignore failed config push, DB writes, or auth errors unless existing behavior clearly does so.

## Database Rules

* Be careful with migrations and model changes.
* Any DB schema change needs:

  * migration/update path
  * backward compatibility check
  * test or manual verification command

## Security Rules

Never:

* Log secrets, tokens, private keys, JWTs, HMAC secrets, passwords.
* Commit real `.env` values.
* Disable auth for convenience.
* Trust user input without validation.
* Shell out with unsanitized values.

## Git Rules

Only commit when explicitly asked.

Before committing:

```bash
git status
git diff
git log --oneline -10
```

Commit style:

* `feat: ...`
* `fix: ...`
* `refactor: ...`
* `docs: ...`
* `test: ...`

Never force-push, amend, or rewrite history unless explicitly told.

## How to Answer

Keep answers short:

* What was wrong
* What changed
* Files changed
* How to verify

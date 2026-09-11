# AGENTS.md

This file provides guidance to Codex (Codex.ai/code) when working with code in this repository.

## What this is

A server-side admin panel for [sing-box](https://github.com/SagerNet/sing-box), intended to run on an overseas VPS that acts as the proxy server. The Go backend runs and manages the local sing-box process and its config; the Next.js frontend is the admin UI. In production it ships as **one CGO-free binary** that serves both the API and the embedded frontend.

Current state is milestone **M1 (skeleton)**: admin login + sing-box status only. Process control, server-config generation, user/inbound management, traffic stats, etc. are future milestones — see `docs/superpowers/specs/` and `docs/superpowers/plans/`.

## Repository layout

Monorepo with two halves:
- `backend/` — Go module `singbox-admin` (Gin + GORM + SQLite). Entry point `cmd/server/main.go`; everything else under `internal/`.
- `frontend/` — Next.js 16 (App Router, TypeScript, Tailwind v4, shadcn/ui). Built as a static export and embedded into the backend binary.

## Commands

All driven from the top-level `Makefile`:

- `make dev` — run backend on :8080 and `next dev` on :3000 together (development).
- `make build` — `next build` (static export to `frontend/out/`) → copy into `backend/internal/web/dist/` → `go build` the single binary to `backend/bin/sing-box-admin`.
- `make run` — `make build` then run the binary.
- `make test` — backend `go test ./...` + frontend `vitest run`.
- `make test-backend` / `make test-frontend` — one side only.

Running a single test:
- Backend: `cd backend && go test ./internal/handlers/ -run TestLoginSuccessSetsCookie -v`
- Frontend: `cd frontend && npm test -- app/login`

The binary reads configuration from env vars: `PORT` (8080), `DB_PATH` (sing-box-admin.db), `JWT_SECRET`, `DEFAULT_ADMIN_USER` (admin), `DEFAULT_ADMIN_PASS` (mnice7082).

### Docker (recommended for the VPS)

`docker compose up -d --build` runs the whole thing as **one container**. The multi-stage `Dockerfile` builds the frontend, compiles the embedded single binary (`CGO_ENABLED=0`), and copies the `sing-box` binary from the official `ghcr.io/sagernet/sing-box` image so the panel can manage it in-container. Makefile shortcuts: `make docker-up` / `make docker-down` / `make docker-logs`.

Key points:
- Copy `.env.example` to `.env` and set `JWT_SECRET` (compose requires it; generate with `openssl rand -hex 32`). Without a persisted secret, sessions reset on every restart.
- The service uses `network_mode: host` so panel-managed sing-box can bind proxy ports directly on the VPS — **Linux hosts only** (host networking is a no-op on Docker Desktop for macOS/Windows).
- The SQLite DB lives on the named volume `singbox_admin_data` at `/data`, so data survives `docker compose down`.
- Pin the bundled sing-box version by setting `SING_BOX_IMAGE` (e.g. `ghcr.io/sagernet/sing-box:v1.11.4`) in `.env`; default is `:latest`.

### Verifying changes — pick the cheapest layer that proves what you need

1. **Logic** (handlers, auth, api client, components): `make test` — local, seconds, no Docker/VPS.
2. **Real UI + end-to-end flow**: `make build && ./backend/bin/sing-box-admin`, open http://localhost:8080 — local single binary, no Docker.
3. **VPS-only behavior** (panel managing/launching sing-box, host networking, real proxying): deploy to the VPS and run under Docker. Only this layer needs the deploy loop.

### Deploy loop to the VPS

Two modes share `.deploy.env` (gitignored — copy from `.deploy.env.example`, set `DEPLOY_HOST=user@ip`, `DEPLOY_PATH`, optional `DEPLOY_PORT`; key-based SSH assumed). Both have a watch variant that redeploys on save (`brew install fswatch`).

**Native — recommended for the test/debug loop, no Docker** (`make deploy-native` / `make watch-deploy-native`, `scripts/deploy-native.sh`):
- Builds the frontend and cross-compiles the embedded **single static binary locally** (arch auto-detected from the VPS via `uname -m`), ships just the binary, and runs it under **systemd** (`sing-box-admin.service`, installed automatically; reference copy in `deploy/`).
- VPS needs nothing but the binary — no Go/Node/Docker. It does need `sing-box` on `PATH` for status to read, a `$DEPLOY_PATH/.env` with `JWT_SECRET`, and root (to write the unit). DB lives at `$DEPLOY_PATH/data/` (the unit overrides `DB_PATH`). Fast: seconds per deploy.
- Logs: `journalctl -u sing-box-admin -f`.

**Docker — matches production** (`make deploy` / `make watch-deploy`, `scripts/deploy.sh`):
- rsyncs the tree (excluding `node_modules`/`.git`/`.env`/data) and runs `docker compose up -d --build` over SSH. VPS needs Docker + compose and a `.env` with `JWT_SECRET`. Data persists in the `singbox_admin_data` volume. Slower (full image rebuild) but identical to prod; layer caching skips `npm ci`/`go mod download` unless lockfiles change.

Both modes deliberately never overwrite the VPS-side `.env` or the database.

## Architecture (the parts that span files)

**Single-binary serving.** `internal/web/embed.go` uses `//go:embed all:dist` to embed the frontend export, and registers a Gin `NoRoute` handler that serves static files and falls back to `index.html` for unknown non-`/api/` paths (client-side routing). `internal/web/dist/` is a build artifact — gitignored except the tracked placeholder `index.html`/`.gitkeep` so a fresh checkout compiles before `make build` runs. During development there is no embedding: `frontend/next.config.ts` `rewrites` proxies `/api/*` to `http://localhost:8080`, so the browser sees same-origin (cookies work, no CORS).

**Auth flow (cookie, not bearer header).** Login (`internal/handlers/auth.go`) verifies bcrypt and sets a JWT in an **httpOnly, SameSite=Lax cookie named `token`** (7-day expiry). `internal/middleware/auth.go` reads that cookie on protected routes. Because the cookie is httpOnly, the frontend cannot read the token — instead `frontend/lib/api.ts` sends every request with `credentials: "include"` and treats a 401 as "not logged in" (throws `UnauthorizedError`, pages redirect to `/login`). JWT generate/parse lives in `internal/auth/jwt.go`.

**sing-box status is abstracted for testing.** `internal/service/singbox.go` defines a `Runner` interface (`Version()`, `IsRunning()`) so tests inject a fake; the real `execRunner` shells out to `sing-box version` and `pgrep -x sing-box`. Status returns `{installed, version, running}` and reports `installed:false` rather than erroring when sing-box is absent (the VPS may not have it yet).

**Wiring.** `cmd/server/main.go` is the only place that assembles config → DB → JWT manager → handlers → routes → embedded web. Each `internal/` package has one responsibility and is unit-tested in isolation.

## Conventions

- **CGO-free on purpose.** The SQLite driver is `github.com/glebarez/sqlite` (pure Go), not `mattn/go-sqlite3`, so the binary cross-compiles for the VPS without a C toolchain. Don't swap it for a CGO driver.
- **TDD.** Every backend package and the frontend logic/pages were built test-first; keep adding tests alongside code.
- Default admin (`admin`/`mnice7082`) is seeded only when the `admins` table is empty (`internal/database/database.go`). If `JWT_SECRET` is unset a random one is generated and logged — sessions then reset on restart, so set it in production.
- Frontend components come from shadcn/ui's `base-nova` style, which is built on `@base-ui/react` (not Radix); the `cn` helper is in `frontend/lib/utils.ts`.
- **shadcn-only UI.** All UI is built from shadcn/ui primitives (`base-nova`). Don't hand-roll raw `<button>`/`<input>`/form controls or bespoke interactive containers where a shadcn component exists — add the component via the shadcn workflow and style through the theme tokens in `frontend/app/globals.css` instead.

## Working in this codebase

Guidelines to reduce common mistakes. These bias toward caution over speed; for trivial tasks, use judgment.

**Think before coding.** Don't assume — state assumptions explicitly and ask when uncertain. If multiple interpretations exist, present them rather than silently picking one. If a simpler approach exists, say so. If something is unclear, stop and name what's confusing.

**Simplicity first.** Write the minimum code that solves the problem — nothing speculative. No features beyond what was asked, no abstractions for single-use code, no unrequested "flexibility," no error handling for impossible cases. If 200 lines could be 50, rewrite it.

**Surgical changes.** Touch only what you must. Don't "improve" adjacent code, refactor what isn't broken, or reformat to taste — match existing style. Remove imports/vars/functions that *your* change orphaned; leave pre-existing dead code alone (mention it, don't delete it). Every changed line should trace to the request.

**Goal-driven execution.** Turn tasks into verifiable goals ("add validation" → "write tests for invalid inputs, then make them pass"). For multi-step work, state a brief plan with a verify step each, then loop until verified. This repo already leans on this: `make test` (logic), local `make build` + run (UI/flow), VPS deploy (environment-specific) — pick the cheapest layer that proves the change.

## UI / visual design

Frontend styling follows `docs/DESIGN.md` — an xAI-inspired **single dark canvas** (no light mode): near-black `#0a0a0a` background, `#191919` cards with `#212327` hairline borders and **no shadows**, white pill buttons, ink/`#7d8187`-mute text, weight-400 type, and uppercase tracked **Geist Mono** eyebrow labels. The palette is wired as shadcn theme tokens in `frontend/app/globals.css` (forced dark via the `dark` class on `<html>`), so styling flows through the shadcn primitives — change tokens there rather than hardcoding colors per component.

## Design docs

Specs and implementation plans live under `docs/superpowers/`. Read the relevant spec before extending a feature area — the M1 boundary (what's intentionally deferred) is documented there. `docs/DESIGN.md` holds the visual identity spec.

# M6 — Traffic Stats, Clash Subscription & Fixed Layout

**Date:** 2026-06-05
**Status:** Approved design (pending user spec review)

## Goal

Add four capabilities to the sing-box admin panel:

1. A repo rule that all UI is built from shadcn/ui primitives, plus migrating the remaining hand-rolled controls.
2. A per-user **Clash subscription** link the user can hand to a client.
3. Per-user **traffic statistics** (cumulative usage) plus a live throughput widget.
4. A **fixed layout**: left sidebar and top header stay put; only the content area scrolls.

Theme stays the existing dark xAI identity (`docs/DESIGN.md`) — we adopt only the *structure* of the reference screenshot, never its light palette.

## Scope decisions (locked)

- Traffic data source: **both** V2Ray StatsService (cumulative) and Clash API (live throughput).
- Traffic persistence: **persisted cumulative totals** in SQLite via a background poller.
- Subscription auth: **per-user random token** at `GET /sub/{token}` (token is the credential).
- Layout: keep dark theme, only make sidebar + header fixed and the content area scrollable.
- Navigation: keep the minimal four destinations, **add group headings**, no search box, no header icons.
- Subscription server address: **new `SERVER_HOST` env var**, falling back to the request `Host` header.
- Live throughput dashboard widget: **in scope** this milestone.

---

## Section A — Layout, shadcn rule, nav groups

### Fixed layout

`frontend/components/app-shell.tsx` is restructured so the page no longer scrolls as one block:

- Root: `flex h-screen overflow-hidden bg-background text-foreground`.
- Sidebar `<aside>`: fixed width, `shrink-0`, its own column, does **not** scroll with content (its nav list may scroll internally only if it overflows).
- Right side: a flex column holding a fixed `<header>` (`shrink-0`) and `<main className="flex-1 overflow-y-auto">`. Only `<main>` scrolls.

The inner content wrapper (`mx-auto max-w-4xl`) stays.

### Navigation with group headings

Nav items are grouped with Geist Mono uppercase eyebrow labels (per `docs/DESIGN.md`):

- `概览` group: 概览 (`/dashboard`), 入站 (`/inbounds`), 用户 (`/users`)
- `系统` group: 配置 (`/config`)

No global search box, no header action icons.

### shadcn migration

- Replace raw `<button>` elements in `app-shell.tsx` (logout) and `app/login/page.tsx` with the shadcn `Button`.
- Build the sidebar/header from shadcn primitives where one exists (`Button`, `Card`, `Separator`). Add a shadcn component (e.g. `separator`) only if it earns its place; otherwise use token-styled `div`s consistent with existing components. Do **not** introduce the heavyweight shadcn `sidebar` block unless it is a clean fit — a styled `<aside>` using theme tokens is acceptable and matches the current minimal design.
- Sweep `app/login`, `app/config`, `app/dashboard` for any remaining raw `<button>`/`<input>` and switch them to shadcn primitives.

### CLAUDE.md rule

Add under **Conventions**:

> **shadcn-only UI.** All UI is built from shadcn/ui primitives (`base-nova`). Don't hand-roll raw `<button>`/`<input>`/form controls or bespoke containers where a shadcn component exists — add the component via the shadcn workflow and style through theme tokens instead.

---

## Section B — Clash subscription

### Data model

`models.User` gains:

- `SubToken string` — unique, 32 hex chars, random. Unique index. Backfilled for existing rows on migration (generate where empty).

### Config

- New env var `SERVER_HOST` in `internal/config` (the VPS public domain or IP). Used as the proxy `server` address in generated subscriptions.
- When `SERVER_HOST` is empty, the subscription handler falls back to the request `Host` header (strip any port if needed for the proxy `server` field, keep the inbound's own port).

### Driver interface

`inbound.Driver` gains:

```go
ClashProxy(serverHost string, port uint16, settings string, cred Cred) (map[string]any, error)
```

Each driver emits a single Clash proxy node:

- **vless-reality**: `{ name, type: "vless", server, port, uuid, network: "tcp", tls: true, flow, servername, reality-opts: { public-key, short-id }, client-fingerprint: "chrome" }`. `name` defaults to the inbound tag.
- **hysteria2**: `{ name, type: "hysteria2", server, port, password, sni, skip-cert-verify: true, alpn: ["h3"] }`.

(Secrets like the reality *private* key are never used here — only the public key/short-id and the user credential.)

### Endpoint

`GET /sub/{token}` — registered **outside** the `/api` group and **before** the SPA `NoRoute` fallback, with **no auth middleware** (the token is the credential).

- Look up the user by `SubToken`; unknown/empty token → `404`.
- For each inbound the user belongs to, call the matching driver's `ClashProxy` with the user's credential (uuid or password per `CredentialKind`).
- Assemble a Clash YAML document: `proxies: [...]`, one `proxy-groups` entry (`{ name: "节点", type: select, proxies: [<all node names>] }`), and a minimal `rules: ["MATCH,节点"]`. Serve as `text/yaml; charset=utf-8`.
- Proxy node names are unique per inbound (`<tag>`); if a user has multiple inbounds the group lists them all.

### UI

`frontend/app/users/page.tsx` row gains a **订阅** action that copies the absolute `/sub/{token}` URL to the clipboard (using `window.location.origin + "/sub/" + token`). The token is exposed in the `User` API DTO as `subToken`.

Token reset: folded into the existing **重置凭证** action (resetting creds also rotates `SubToken`), so no separate button. This is documented so the user knows resetting credentials invalidates the old subscription link.

---

## Section C — Traffic statistics

### Config generation

`inbound.Generate` additionally emits an `experimental` block:

```json
"experimental": {
  "clash_api": { "external_controller": "127.0.0.1:<clashPort>", "secret": "<secret>" },
  "v2ray_api": {
    "listen": "127.0.0.1:<v2rayPort>",
    "stats": { "enabled": true, "inbounds": [<all tags>], "users": ["u<id>", ...] }
  }
}
```

- Inbound user objects use a **stable stats key as their `name`**: `u<userID>` (e.g. `u7`). The friendly display name is irrelevant to sing-box and only used in the panel UI, so this is safe and lets v2ray stats correlate to a DB user even across renames and across multiple inbounds (stats aggregate per user name).
- The Clash/V2Ray API listen addresses and the generated secret are persisted in a panel-owned **`meta`** table (key/value), generated once and reused, so the poller and the emitted config always agree. If the meta values are missing they are generated and stored at config-generation time.

### Data model

`models.User` gains:

- `UpBytes int64` — cumulative upload bytes.
- `DownBytes int64` — cumulative download bytes.

`models.Meta` (new): `Key string` (PK), `Value string`. Stores `clash_api_addr`, `clash_api_secret`, `v2ray_api_addr`.

### Poller

A new `internal/traffic` service:

- Started from `cmd/server/main.go` as a goroutine on a ticker (default 10s; constant).
- Each tick: issue a V2Ray `StatsService.QueryStats` gRPC call with `pattern: "user>>>"` and `reset: true`.
- Parse stat names `user>>>u<id>>>>traffic>>>uplink` / `...>>>downlink`, extract `<id>`, and **add the returned delta** to that user's `UpBytes` / `DownBytes`. Using `reset:true` + delta-accumulation means cumulative totals survive sing-box restarts (each restart simply starts the counter from zero again and we keep adding deltas).
- gRPC client: a **minimal hand-written client** for the V2Ray `StatsService` proto (`QueryStats`), no import of the sing-box library. Uses `google.golang.org/grpc` (pure Go, CGO-free).
- Errors (sing-box not running, API unreachable) are logged and skipped — the poller keeps ticking.
- The stats source is abstracted behind an interface so the accumulation logic is unit-tested against a fake returning canned stat entries.

### Live throughput

- `GET /api/traffic/live` (auth-protected): reads one tick of the Clash API `/traffic` stream (`http://<clash_api_addr>/traffic` with the secret as bearer) server-side and returns `{ up: <bytes/s>, down: <bytes/s> }`. If sing-box is down / API unreachable, returns `{ up: 0, down: 0 }` (not an error) so the widget degrades gracefully.

### UI

- `frontend/app/users/page.tsx`: add **上行** and **下行总量** columns, human-formatted (B/KB/MB/GB), from `user.upBytes` / `user.downBytes`. Add a **重置流量** row action behind a 二次确认 modal (calls a new `POST /api/users/{id}/reset-traffic`, which zeroes the user's counters).
- `frontend/app/dashboard/page.tsx`: add a live throughput widget (current up/down speed) polling `/api/traffic/live` on an interval while the page is mounted.

### API DTOs

`UserView` gains `subToken`, `upBytes`, `downBytes`. (`subToken` is fine to expose — it is the subscription credential the admin needs to share; the page is already admin-only.)

---

## Section D — Sequencing, testing, verification

Task order keeps the tree green at each commit:

1. **A** — frontend layout + nav groups + shadcn migration + CLAUDE.md rule. Pure UI/CSS/docs; no backend.
2. **B** — subscription: `SubToken` model field + migration backfill, `SERVER_HOST` config, driver `ClashProxy` method (both drivers), `/sub/{token}` handler, `subToken` in DTO, users-page 订阅 copy action.
3. **C** — traffic: `Meta` model, `UpBytes`/`DownBytes` fields, `experimental` block + `u<id>` naming in `Generate`, gRPC stats client + poller service wired in `main.go`, `/api/traffic/live` + `/api/users/{id}/reset-traffic` handlers, users-page columns + 重置流量, dashboard live widget.

### Testing (TDD, per existing conventions)

- **Backend**
  - `driver_test.go`: `ClashProxy` for vless-reality and hysteria2 produces the expected node maps; no private key leaks.
  - `generate_test.go`: emitted config contains `experimental.clash_api` + `experimental.v2ray_api.stats` with the right inbound tags and `u<id>` user names; meta values are generated/reused.
  - subscription handler test: valid token → YAML with the user's nodes + group + rules; unknown token → 404; `SERVER_HOST` honored, Host fallback when unset.
  - poller test: fake stats client returns deltas across two ticks → user counters accumulate correctly (including the restart-to-zero case).
  - users handler test: `reset-traffic` zeroes counters; `reset` rotates `SubToken`.
- **Frontend**
  - app-shell renders fixed structure (sidebar + header present, main is the scroll container) and grouped nav labels.
  - users page renders up/down columns, 订阅 copy action, 重置流量 二次确认.
  - dashboard renders the live widget and polls `/api/traffic/live`.

### Verification layers

- `make test` for all logic (cheapest).
- `make build` + run the single binary at `http://localhost:8080` for the real UI/flow (fixed layout, subscription copy, pages render).
- VPS deploy only to validate actual traffic accounting and a real client consuming the subscription (environment-specific).

## Out of scope (YAGNI)

- Per-inbound (vs per-user) traffic breakdown.
- Historical time-series charts / graphs of traffic over time (only cumulative totals + instantaneous live speed).
- Subscription formats other than Clash (no sing-box/v2ray/shadowrocket variants this milestone).
- Quotas / auto-disable on traffic limit.

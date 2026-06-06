# M8 — Outbound Management & Per-User Routing

**Date:** 2026-06-06
**Status:** Approved design (pending user spec review)

## Goal

Let the admin define one or more upstream proxy **outbounds** (http or socks5, with optional auth) and route a given user's traffic through a chosen outbound — so egress hides the VPS's IP (and the client's real IP) behind an upstream proxy. Proxied users' **DNS is resolved through their outbound** (not the VPS's local resolver) so it doesn't leak — automatically, with no extra configuration.

## Scope decisions (locked)

- Routing granularity: **per-user single outbound**. Each user may be assigned one outbound; unassigned (`null`) means `direct`. The assignment applies to all of the user's traffic regardless of which inbound they connected through.
- Outbound types: **http** and **socks5**, each with **optional** username/password. No http-over-TLS option this round.
- Multiple outbounds supported.
- Deleting an outbound that users reference: **delete it and null those users' assignment** (they fall back to direct), then regenerate.
- DNS leak prevention: **automatic, zero-config**. The generator adds a `dns` block tying each proxied user's DNS to their outbound (via `detour`); no new UI or settings. Encrypted upstream resolver hardcoded to `tls://1.1.1.1`.

## Data model

### New: `models.Outbound`
```go
type Outbound struct {
    ID        uint   `gorm:"primaryKey" json:"id"`
    Tag       string `gorm:"uniqueIndex;not null" json:"tag"`
    Type      string `gorm:"not null" json:"type"`     // "http" | "socks5"
    Server    string `gorm:"not null" json:"server"`
    Port      uint16 `gorm:"not null" json:"port"`
    Username  string `json:"username"`                 // optional
    Password  string `json:"password"`                 // optional
    CreatedAt time.Time `json:"createdAt"`
    UpdatedAt time.Time `json:"updatedAt"`
}
```
These are upstream-proxy credentials the admin enters (not generated). Like the users page already shows plaintext password, the outbound fields are returned in an admin-only view (the panel is admin-only).

### `models.User` gains
```go
OutboundID *uint `json:"outboundId"`   // nullable; null => direct
```
A nullable foreign key to `Outbound`. On outbound deletion, referencing users have this set back to `NULL`.

### Migration
`database.Init` AutoMigrates `&models.Outbound{}`. The new nullable `OutboundID` column on `User` is added by AutoMigrate; existing users default to `NULL` (direct) — no backfill needed.

## Config generation (`internal/inbound/generate.go`)

Signature becomes:
```go
func Generate(inbounds []models.Inbound, outbounds []models.Outbound, exp ExperimentalConfig) (string, error)
```

(Its only caller is `Service.Regenerate`, updated to pass outbounds.)

### `outbounds` array
Always includes `{type:"direct", tag:"direct"}`. For each configured outbound, append:
- **http**: `{"type":"http","tag":<tag>,"server":<server>,"server_port":<port>}` plus `"username"`/`"password"` when non-empty.
- **socks5**: same but `"type":"socks"` (sing-box's name for SOCKS5).

### `route` block
- Build `userOutbound map[uint]string` (user ID → outbound tag) from the inbounds' preloaded `Users` and their `OutboundID`, resolved against the outbounds list (`id → tag`). Users whose `OutboundID` is null or points to a missing outbound are omitted (→ direct).
- Group users by outbound tag and emit one rule each, sorted by tag for deterministic output:
  ```json
  { "auth_user": ["u<id>", "u<id>"], "outbound": "<tag>" }
  ```
- `"final": "direct"`.
- If no user has an assignment, still emit a `route` with empty `rules` and `final:"direct"` (harmless and keeps output shape stable), or omit `rules` — either is valid; the implementation emits `route` with the rules slice (possibly empty) and `final`.

**sing-box field note:** the route rule matches the authenticated inbound user via `auth_user` (array of usernames), which equals our stable `u<id>` key — the same key used for traffic stats. The exact field name will be verified against the bundled sing-box version during implementation (`auth_user` is the current sing-box field; older builds used `user`).

### `dns` block (DNS-leak prevention)

Derived from the same `userOutbound` grouping (no extra input). Goal: a proxied user's domain resolution travels through their outbound, so neither the VPS's resolver nor a plaintext query leaks it.

- **servers**:
  - `{"tag":"local","address":"local"}` — the VPS's own resolver, used by direct users and as the fallback.
  - For each outbound that has ≥1 assigned user: `{"tag":"dns-<tag>","address":"tls://1.1.1.1","detour":"<tag>"}` — an encrypted (DoT) resolver whose queries egress through that outbound.
- **rules**: for each such outbound, `{"auth_user":["u<id>",…],"server":"dns-<tag>"}` (same grouping/sorting as the route rules).
- **`"final":"local"`**, **`"strategy":"prefer_ipv4"`**.

Because the proxied user's DNS is sent through the outbound's `detour`, resolution happens at/through the upstream proxy — no leak. Direct users resolve via `local`, which is correct (their egress is the VPS anyway). The resolver address `tls://1.1.1.1` is a hardcoded constant. If there are no outbounds with assigned users, the `dns` block contains only the `local` server with `final:"local"` (a harmless, sensible default).

## Service & API (`internal/inbound`)

Outbound logic lives in a new `internal/inbound/outbound.go` on the existing `Service` (it already owns the DB, the user/inbound model, and `Regenerate`).

- `OutboundView{ID, Tag, Type, Server, Port, Username, Password}` and `ListOutboundViews()`.
- `CreateOutbound(typ, tag, server string, port uint16, username, password string) (models.Outbound, error)` — validates: type ∈ {http, socks5}; tag non-empty + unique; port > 0; server non-empty. Calls `Regenerate`.
- `UpdateOutbound(id, …)` — same validation, tag-unique excluding self. Regenerate.
- `DeleteOutbound(id)` — sets `OutboundID = NULL` on users referencing it, deletes the row, Regenerate.
- User assignment: extend `CreateUser`/`UpdateUser` to accept an `outboundID *uint` and persist it (validated to reference an existing outbound or be null). `UserView` gains `OutboundID *uint` (json `outboundId`).
- `Regenerate` loads outbounds and passes them to `Generate`.

### Errors
Reuse existing sentinels where they fit (`ErrInvalidTag`, `ErrTagExists`, `ErrNotFound`); add `ErrInvalidType` / `ErrInvalidOutbound` as needed for bad type or a non-existent assigned outbound. Handlers map them to 400/404/409 like the inbound handlers.

### Handlers & routes (`internal/handlers/outbound.go`)
- `GET /api/outbounds` → `ListOutboundViews`
- `POST /api/outbounds` → create
- `PUT /api/outbounds/:id` → update
- `DELETE /api/outbounds/:id` → delete

All inside the authed group. User create/update bodies gain `outboundId`.

## Frontend

- **Nav**: add **出站** to the `概览` group in `app-shell.tsx` (概览 / 入站 / 出站 / 用户).
- **`app/outbounds/page.tsx`**: a shadcn `Table` (columns 标签 / 类型 / 服务器 / 端口 / 用户名) with ghost icon actions (编辑 / 删除, delete behind a 二次确认 Modal), and a create/edit `Modal` with: type shadcn `Select` (http / socks5), 服务器, 端口, 用户名 (optional), 密码 (optional). Mirrors the users-page table style.
- **User modal** (`app/users/page.tsx`): add an **出站** shadcn `Select` — first option 「直连」 (value = none) then each outbound by tag. Persists as `outboundId` on create/update.
- **`lib/api.ts`**: `Outbound` interface + `listOutbounds/createOutbound/updateOutbound/deleteOutbound`; `User.outboundId?: number | null`; `createUser/updateUser` signatures carry `outboundId`.

## Testing (TDD, per repo conventions)

**Backend**
- `generate_test.go`: given one http outbound + one socks5 outbound and a user assigned to one of them, assert: both outbound objects present (socks5 → type `socks`), a `route.rules` entry `{auth_user:["u<id>"], outbound:<tag>}`, `route.final == "direct"`, and a user with no assignment produces no rule. Auth omitted when username/password empty. **DNS**: a `dns.servers` entry `{tag:"dns-<tag>", detour:"<tag>"}` exists for the assigned outbound, a `dns.rules` entry maps `auth_user:["u<id>"] → dns-<tag>`, and `dns.final == "local"`.
- `outbound_test.go` (service): create/update/delete outbound; tag uniqueness; assigning `outboundId` to a user round-trips through `UserView`; deleting an outbound nulls referencing users and regenerates.
- handler tests: outbound CRUD happy paths + validation (bad type → 400, duplicate tag → 409, unknown id → 404).

**Frontend**
- outbounds page: renders the table, create submits the right payload, delete needs 二次确认.
- user modal: the 出站 select offers 直连 + outbounds and submits the chosen `outboundId`.

## Verification layers
1. `make test` (logic).
2. `make build` + run the binary locally; eyeball the 出站 page and the user-form select.
3. VPS: define a real upstream http/socks5 outbound, assign a user, 应用并重启, and confirm that user's egress IP is the upstream's (e.g. `curl ifconfig.me` through that user) while an unassigned user still egresses via the VPS. Confirm no DNS leak for the proxied user (e.g. a DNS-leak test site shows the upstream's resolver, not the VPS's).

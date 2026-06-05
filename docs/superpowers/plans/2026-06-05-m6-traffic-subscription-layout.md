# M6 — Traffic Stats, Clash Subscription & Fixed Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add per-user Clash subscriptions, per-user cumulative traffic stats + a live throughput widget, a fixed (non-scrolling) sidebar/header layout, and a shadcn-only UI rule.

**Architecture:** Backend stays Gin + GORM + the driver registry. Subscriptions and traffic both extend the existing `inbound.Service` (it already owns the db + drivers + config regeneration). Traffic uses sing-box's `experimental.v2ray_api` (gRPC StatsService, cumulative) and `experimental.clash_api` (HTTP, live), queried by a small `internal/traffic` package. Frontend keeps the dark xAI theme; only the layout structure changes.

**Tech Stack:** Go (Gin, GORM, glebarez/sqlite, **new:** `google.golang.org/grpc` + `github.com/goccy/go-yaml`), Next.js 16 / Tailwind v4 / shadcn (base-nova), Vitest.

**Reference spec:** `docs/superpowers/specs/2026-06-05-traffic-subscription-layout-design.md`

---

## File structure

**Backend (`backend/`)**
- `internal/models/inbound.go` — add `SubToken`, `UpBytes`, `DownBytes` to `User`.
- `internal/models/meta.go` *(new)* — `Meta{Key,Value}` key/value table.
- `internal/database/database.go` — AutoMigrate `Meta`.
- `internal/config/config.go` — add `ServerHost`.
- `internal/inbound/driver.go` — add `ClashProxy` to the `Driver` interface; add `ExperimentalConfig` struct.
- `internal/inbound/driver_vless.go` / `driver_hysteria2.go` — implement `ClashProxy`.
- `internal/inbound/generate.go` — `Generate` takes `ExperimentalConfig`, emits the `experimental` block, names users `u<id>`.
- `internal/inbound/service.go` — `genToken`, token on create/reset, `BackfillUserTokens`, `ResetUserTraffic`, `AddTraffic`, `APIConfig`, `UserView` gains the new fields, `Regenerate` passes `ExperimentalConfig`.
- `internal/inbound/subscription.go` *(new)* — `UserSubscription(token, serverHost)` builds Clash YAML.
- `internal/traffic/stats.go` *(new)* — V2Ray StatsService gRPC client + hand-rolled protobuf codec + `StatsClient` interface.
- `internal/traffic/poller.go` *(new)* — ticker that accumulates deltas via a `TrafficStore`.
- `internal/traffic/live.go` *(new)* — one-tick Clash `/traffic` read.
- `internal/handlers/subscription.go` *(new)* — `GET /sub/:token`.
- `internal/handlers/traffic.go` *(new)* — `GET /api/traffic/live`.
- `internal/handlers/user.go` — add `ResetTraffic` handler + interface method.
- `cmd/server/main.go` — wire backfill, subscription route, traffic routes, poller.

**Frontend (`frontend/`)**
- `components/app-shell.tsx` — fixed layout + grouped nav + shadcn `Button`.
- `components/app-shell.test.tsx` *(new)* — structure + nav tests.
- `lib/utils.ts` — add `formatBytes`.
- `lib/api.ts` — `User.subToken/upBytes/downBytes`, `getLiveTraffic`, `resetUserTraffic`.
- `app/users/page.tsx` — 订阅 copy, traffic columns, 重置流量.
- `app/dashboard/page.tsx` — live throughput widget.
- `CLAUDE.md` — shadcn-only rule.

---

## Section A — Layout, shadcn rule, nav groups

### Task 1: Add the shadcn-only rule to CLAUDE.md

**Files:**
- Modify: `CLAUDE.md` (the `## Conventions` section)

- [ ] **Step 1: Add the rule**

In `CLAUDE.md`, under `## Conventions`, add this bullet after the existing `Frontend components come from shadcn/ui's base-nova style…` bullet:

```markdown
- **shadcn-only UI.** All UI is built from shadcn/ui primitives (`base-nova`). Don't hand-roll raw `<button>`/`<input>`/form controls or bespoke interactive containers where a shadcn component exists — add the component via the shadcn workflow and style through the theme tokens in `frontend/app/globals.css` instead.
```

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: require shadcn primitives for all UI"
```

---

### Task 2: Fixed sidebar/header layout + grouped nav

**Files:**
- Modify: `frontend/components/app-shell.tsx`
- Test: `frontend/components/app-shell.test.tsx` *(new)*

- [ ] **Step 1: Write the failing test**

Create `frontend/components/app-shell.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { AppShell } from "./app-shell";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/dashboard",
}));

vi.mock("@/lib/api", () => ({
  getStatus: vi.fn().mockResolvedValue({ installed: true, version: "1.13.13", running: false, hasConfig: true }),
  logout: vi.fn(),
  UnauthorizedError: class extends Error {},
}));

describe("AppShell", () => {
  it("renders grouped nav labels and a scrollable main", () => {
    render(<AppShell><div>content</div></AppShell>);
    // group heading
    expect(screen.getByText("概览", { selector: "p" })).toBeInTheDocument();
    // nav destinations
    expect(screen.getByRole("link", { name: "入站" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "用户" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "配置" })).toBeInTheDocument();
    // main is the scroll container
    const main = screen.getByRole("main");
    expect(main.className).toContain("overflow-y-auto");
    // content rendered
    expect(screen.getByText("content")).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd frontend && npm test -- app-shell`
Expected: FAIL (no group heading `<p>概览</p>`, main lacks `overflow-y-auto`).

- [ ] **Step 3: Rewrite app-shell with fixed layout + grouped nav**

Replace the entire body of `frontend/components/app-shell.tsx` with:

```tsx
"use client";

import { useEffect } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { getStatus, logout, UnauthorizedError } from "@/lib/api";
import { Button } from "@/components/ui/button";

const NAV_GROUPS: { heading: string; items: { href: string; label: string }[] }[] = [
  {
    heading: "概览",
    items: [
      { href: "/dashboard", label: "概览" },
      { href: "/inbounds", label: "入站" },
      { href: "/users", label: "用户" },
    ],
  },
  {
    heading: "系统",
    items: [{ href: "/config", label: "配置" }],
  },
];

export function AppShell({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();

  useEffect(() => {
    getStatus().catch((err) => {
      if (err instanceof UnauthorizedError) router.push("/login");
    });
  }, [router]);

  async function onLogout() {
    await logout();
    router.push("/login");
  }

  return (
    <div className="flex h-screen overflow-hidden bg-background text-foreground">
      <aside className="flex w-56 shrink-0 flex-col border-r border-border bg-card">
        <div className="shrink-0 px-5 py-5">
          <span className="font-mono text-sm tracking-[0.18em] text-foreground uppercase">
            sing-box admin
          </span>
        </div>
        <nav className="flex-1 overflow-y-auto px-3 pb-4">
          {NAV_GROUPS.map((group) => (
            <div key={group.heading} className="mb-4">
              <p className="px-3 pb-1 font-mono text-[10px] tracking-[0.18em] text-muted-foreground uppercase">
                {group.heading}
              </p>
              <div className="flex flex-col gap-1">
                {group.items.map((item) => {
                  const active = pathname === item.href;
                  return (
                    <Link
                      key={item.href}
                      href={item.href}
                      className={`rounded-lg px-3 py-2 text-sm transition-colors ${
                        active ? "bg-secondary text-foreground" : "text-muted-foreground hover:text-foreground"
                      }`}
                    >
                      {item.label}
                    </Link>
                  );
                })}
              </div>
            </div>
          ))}
        </nav>
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-14 shrink-0 items-center justify-between border-b border-border px-6">
          <span className="text-sm text-muted-foreground">sing-box 管理面板</span>
          <Button variant="outline" className="rounded-full" onClick={onLogout}>
            退出登录
          </Button>
        </header>
        <main className="min-w-0 flex-1 overflow-y-auto px-6 py-8">
          <div className="mx-auto max-w-4xl">{children}</div>
        </main>
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd frontend && npm test -- app-shell`
Expected: PASS.

- [ ] **Step 5: Run the full frontend suite (no regressions)**

Run: `cd frontend && npm test`
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add frontend/components/app-shell.tsx frontend/components/app-shell.test.tsx
git commit -m "feat: fixed sidebar/header layout with grouped nav"
```

---

## Section B — Clash subscription

### Task 3: Add `User.SubToken` model field and `config.ServerHost`

**Files:**
- Modify: `backend/internal/models/inbound.go`
- Modify: `backend/internal/config/config.go`

- [ ] **Step 1: Add the model field**

In `backend/internal/models/inbound.go`, change the `User` struct to add `SubToken`:

```go
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"not null" json:"name"`
	UUID      string    `gorm:"not null" json:"uuid"`
	Password  string    `gorm:"not null" json:"password"`
	SubToken  string    `gorm:"uniqueIndex" json:"subToken"`
	Inbounds  []Inbound `gorm:"many2many:user_inbounds;" json:"-"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
```

- [ ] **Step 2: Add `ServerHost` to config**

In `backend/internal/config/config.go`, add the field to the `Config` struct (after `SingboxBin`):

```go
	SingboxBin       string
	ServerHost       string
```

And in `Load()`, after the `SingboxBin:` line inside the struct literal:

```go
		SingboxBin:       os.Getenv("SINGBOX_BIN"),
		ServerHost:       os.Getenv("SERVER_HOST"),
```

- [ ] **Step 3: Verify it compiles**

Run: `cd backend && go build ./...`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/models/inbound.go backend/internal/config/config.go
git commit -m "feat: add User.SubToken and SERVER_HOST config"
```

---

### Task 4: `Driver.ClashProxy` for both protocols

**Files:**
- Modify: `backend/internal/inbound/driver.go`
- Modify: `backend/internal/inbound/driver_vless.go`
- Modify: `backend/internal/inbound/driver_hysteria2.go`
- Test: `backend/internal/inbound/driver_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `backend/internal/inbound/driver_test.go`:

```go
func TestVlessClashProxy(t *testing.T) {
	d, _ := Get("vless-reality")
	s, _ := d.BuildSettings(map[string]any{"handshake": "www.example.com"})
	p, err := d.ClashProxy("v1", "vps.example.com", 8443, s, Cred{Name: "u1", Credential: "uuid-1"})
	if err != nil {
		t.Fatalf("ClashProxy: %v", err)
	}
	if p["name"] != "v1" || p["type"] != "vless" || p["server"] != "vps.example.com" {
		t.Fatalf("proxy=%v", p)
	}
	if p["uuid"] != "uuid-1" || p["port"] != uint16(8443) {
		t.Fatalf("proxy=%v", p)
	}
	ro := p["reality-opts"].(map[string]any)
	if ro["public-key"] == "" || ro["public-key"] == nil {
		t.Fatal("missing reality public-key")
	}
	if strings.Contains(toJSON(p), "private") {
		t.Fatal("clash proxy leaked a private key")
	}
}

func TestHy2ClashProxy(t *testing.T) {
	d, _ := Get("hysteria2")
	s, _ := d.BuildSettings(map[string]any{"serverName": "bing.com"})
	p, err := d.ClashProxy("h1", "vps.example.com", 443, s, Cred{Name: "u1", Credential: "pw-1"})
	if err != nil {
		t.Fatalf("ClashProxy: %v", err)
	}
	if p["type"] != "hysteria2" || p["password"] != "pw-1" || p["sni"] != "bing.com" {
		t.Fatalf("proxy=%v", p)
	}
	if p["skip-cert-verify"] != true {
		t.Fatalf("expected skip-cert-verify true: %v", p)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/inbound/ -run ClashProxy -v`
Expected: FAIL — compile error, `ClashProxy` not in the interface.

- [ ] **Step 3: Add `ClashProxy` to the interface**

In `backend/internal/inbound/driver.go`, add to the `Driver` interface (after `ResetSecrets`):

```go
	ResetSecrets(existing string) (string, error)                          // new secrets, keep params
	ClashProxy(name, serverHost string, port uint16, settings string, cred Cred) (map[string]any, error)
```

- [ ] **Step 4: Implement for vless**

In `backend/internal/inbound/driver_vless.go`, add:

```go
func (vlessReality) ClashProxy(name, serverHost string, port uint16, settings string, cred Cred) (map[string]any, error) {
	var s vlessSettings
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		return nil, err
	}
	return map[string]any{
		"name": name, "type": "vless", "server": serverHost, "port": port,
		"uuid": cred.Credential, "network": "tcp", "udp": true, "tls": true,
		"flow": s.Flow, "servername": s.ServerName,
		"reality-opts":       map[string]any{"public-key": s.RealityPublicKey, "short-id": s.ShortID},
		"client-fingerprint": "chrome",
	}, nil
}
```

- [ ] **Step 5: Implement for hysteria2**

In `backend/internal/inbound/driver_hysteria2.go`, add:

```go
func (hysteria2) ClashProxy(name, serverHost string, port uint16, settings string, cred Cred) (map[string]any, error) {
	var s hy2Settings
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		return nil, err
	}
	return map[string]any{
		"name": name, "type": "hysteria2", "server": serverHost, "port": port,
		"password": cred.Credential, "sni": s.ServerName,
		"skip-cert-verify": true, "alpn": []string{"h3"},
	}, nil
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `cd backend && go test ./internal/inbound/ -run ClashProxy -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/inbound/driver.go backend/internal/inbound/driver_vless.go backend/internal/inbound/driver_hysteria2.go backend/internal/inbound/driver_test.go
git commit -m "feat: drivers emit Clash proxy nodes"
```

---

### Task 5: Service-side token handling

**Files:**
- Modify: `backend/internal/inbound/service.go`
- Test: `backend/internal/inbound/service_test.go`

- [ ] **Step 1: Write the failing test**

Append to `backend/internal/inbound/service_test.go` (it already has helpers like `newTestService`/`fakeWriter`; if the helper name differs, reuse whatever the file uses to build a `*Service` with an in-memory db):

```go
func TestCreateUserAssignsSubToken(t *testing.T) {
	s := newTestService(t)
	u, err := s.CreateUser("alice", nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if len(u.SubToken) != 32 {
		t.Fatalf("sub token len=%d, want 32", len(u.SubToken))
	}
	old := u.SubToken
	u2, err := s.ResetUserCreds(u.ID)
	if err != nil {
		t.Fatalf("ResetUserCreds: %v", err)
	}
	if u2.SubToken == old || len(u2.SubToken) != 32 {
		t.Fatalf("reset must rotate sub token; old=%s new=%s", old, u2.SubToken)
	}
}

func TestBackfillUserTokens(t *testing.T) {
	s := newTestService(t)
	// Create a user with an empty token directly via the db.
	u := models.User{Name: "old", UUID: "x", Password: "y"}
	if err := s.db.Create(&u).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := s.BackfillUserTokens(); err != nil {
		t.Fatalf("BackfillUserTokens: %v", err)
	}
	var got models.User
	s.db.First(&got, u.ID)
	if len(got.SubToken) != 32 {
		t.Fatalf("token not backfilled: %q", got.SubToken)
	}
}
```

> If `newTestService(t)` does not exist in `service_test.go`, add this helper near the top of the file:
> ```go
> func newTestService(t *testing.T) *Service {
> 	t.Helper()
> 	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
> 	if err != nil {
> 		t.Fatalf("open db: %v", err)
> 	}
> 	if err := db.AutoMigrate(&models.Inbound{}, &models.User{}); err != nil {
> 		t.Fatalf("migrate: %v", err)
> 	}
> 	return NewService(db, &fakeWriter{})
> }
> ```
> (Imports `"github.com/glebarez/sqlite"`, `"gorm.io/gorm"`, `"singbox-admin/internal/models"`.)

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/inbound/ -run 'SubToken|Backfill' -v`
Expected: FAIL — `genToken`/`BackfillUserTokens` undefined, `SubToken` not set.

- [ ] **Step 3: Add `genToken`, set token on create/reset, add backfill**

In `backend/internal/inbound/service.go`:

Add after `genPassword`:

```go
func genToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
```

Add `"encoding/hex"` to the imports.

In `CreateUser`, change the user construction:

```go
	u := models.User{Name: name, UUID: genUUID(), Password: genPassword(), SubToken: genToken()}
```

In `ResetUserCreds`, change the secret rotation line:

```go
	u.UUID, u.Password, u.SubToken = genUUID(), genPassword(), genToken()
```

Add a backfill method (anywhere in the file):

```go
// BackfillUserTokens gives a SubToken to any pre-existing user that lacks one.
func (s *Service) BackfillUserTokens() error {
	var us []models.User
	if err := s.db.Where("sub_token = '' OR sub_token IS NULL").Find(&us).Error; err != nil {
		return err
	}
	for i := range us {
		us[i].SubToken = genToken()
		if err := s.db.Model(&us[i]).Update("sub_token", us[i].SubToken).Error; err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Expose `SubToken` in `UserView`**

Change the `UserView` struct to add `SubToken`:

```go
type UserView struct {
	ID          uint     `json:"id"`
	Name        string   `json:"name"`
	UUID        string   `json:"uuid"`
	Password    string   `json:"password"`
	SubToken    string   `json:"subToken"`
	InboundIDs  []uint   `json:"inboundIds"`
	InboundTags []string `json:"inboundTags"`
}
```

And in `ListUserViews`, set it in the per-user `v`:

```go
		v := UserView{ID: u.ID, Name: u.Name, UUID: u.UUID, Password: u.Password, SubToken: u.SubToken, InboundIDs: []uint{}, InboundTags: []string{}}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd backend && go test ./internal/inbound/ -run 'SubToken|Backfill' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/inbound/service.go backend/internal/inbound/service_test.go
git commit -m "feat: assign and rotate per-user subscription tokens"
```

---

### Task 6: Subscription builder + `/sub/:token` handler + routing

**Files:**
- Create: `backend/internal/inbound/subscription.go`
- Create: `backend/internal/handlers/subscription.go`
- Modify: `backend/cmd/server/main.go`
- Test: `backend/internal/inbound/subscription_test.go` *(new)*
- Test: `backend/internal/handlers/subscription_test.go` *(new)*

- [ ] **Step 1: Add the YAML dependency**

Run: `cd backend && go get github.com/goccy/go-yaml@latest`
Expected: `go.mod` now lists it as a direct require.

- [ ] **Step 2: Write the failing builder test**

Create `backend/internal/inbound/subscription_test.go`:

```go
package inbound

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"singbox-admin/internal/models"
)

func TestUserSubscription(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.AutoMigrate(&models.Inbound{}, &models.User{})
	s := NewService(db, &fakeWriter{})
	if _, err := s.CreateInbound("vless-reality", "v1", 8443, nil); err != nil {
		t.Fatalf("inbound: %v", err)
	}
	var in models.Inbound
	db.First(&in, "tag = ?", "v1")
	u, _ := s.CreateUser("alice", []uint{in.ID})

	yaml, err := s.UserSubscription(u.SubToken, "vps.example.com")
	if err != nil {
		t.Fatalf("UserSubscription: %v", err)
	}
	if !strings.Contains(yaml, "proxies:") || !strings.Contains(yaml, "vps.example.com") {
		t.Fatalf("subscription missing proxies/server:\n%s", yaml)
	}
	if !strings.Contains(yaml, u.UUID) {
		t.Fatal("subscription should contain the user's uuid")
	}
	if !strings.Contains(yaml, "节点") {
		t.Fatal("subscription should contain the proxy group")
	}

	if _, err := s.UserSubscription("nope", "vps.example.com"); err != ErrNotFound {
		t.Fatalf("unknown token should be ErrNotFound, got %v", err)
	}
}
```

- [ ] **Step 3: Run to verify it fails**

Run: `cd backend && go test ./internal/inbound/ -run UserSubscription -v`
Expected: FAIL — `UserSubscription` undefined.

- [ ] **Step 4: Implement the builder**

Create `backend/internal/inbound/subscription.go`:

```go
package inbound

import (
	"github.com/goccy/go-yaml"

	"singbox-admin/internal/models"
)

// UserSubscription returns a Clash-format YAML config for the user identified
// by subToken. serverHost is the public address clients dial.
func (s *Service) UserSubscription(subToken, serverHost string) (string, error) {
	if subToken == "" {
		return "", ErrNotFound
	}
	var u models.User
	if err := s.db.Preload("Inbounds").Where("sub_token = ?", subToken).First(&u).Error; err != nil {
		return "", ErrNotFound
	}

	proxies := []map[string]any{}
	names := []string{}
	for _, in := range u.Inbounds {
		d, ok := Get(in.Type)
		if !ok {
			continue
		}
		cred := Cred{Name: u.Name, Credential: u.UUID}
		if d.CredentialKind() == "password" {
			cred.Credential = u.Password
		}
		p, err := d.ClashProxy(in.Tag, serverHost, in.Port, in.Settings, cred)
		if err != nil {
			return "", err
		}
		proxies = append(proxies, p)
		names = append(names, in.Tag)
	}

	cfg := map[string]any{
		"proxies": proxies,
		"proxy-groups": []map[string]any{
			{"name": "节点", "type": "select", "proxies": names},
		},
		"rules": []string{"MATCH,节点"},
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
```

- [ ] **Step 5: Run the builder test to verify it passes**

Run: `cd backend && go test ./internal/inbound/ -run UserSubscription -v`
Expected: PASS.

- [ ] **Step 6: Write the failing handler test**

Create `backend/internal/handlers/subscription_test.go`:

```go
package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeSub struct{ yaml string; err error }

func (f fakeSub) UserSubscription(token, host string) (string, error) { return f.yaml, f.err }

func TestSubscriptionServesYAML(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewSubscriptionHandler(fakeSub{yaml: "proxies: []\n"}, "vps.example.com")
	r.GET("/sub/:token", h.Get)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/sub/abc", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/yaml; charset=utf-8" {
		t.Fatalf("content-type=%q", ct)
	}
	if w.Body.String() != "proxies: []\n" {
		t.Fatalf("body=%q", w.Body.String())
	}
}

func TestSubscriptionUnknownToken404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewSubscriptionHandler(fakeSub{err: errNotFoundForTest}, "vps.example.com")
	r.GET("/sub/:token", h.Get)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/sub/nope", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("code=%d, want 404", w.Code)
	}
}
```

Add this helper at the bottom of the same test file:

```go
var errNotFoundForTest = inbound.ErrNotFound
```

…and add the import `"singbox-admin/internal/inbound"` to the test file.

- [ ] **Step 7: Run to verify it fails**

Run: `cd backend && go test ./internal/handlers/ -run Subscription -v`
Expected: FAIL — `NewSubscriptionHandler` undefined.

- [ ] **Step 8: Implement the handler**

Create `backend/internal/handlers/subscription.go`:

```go
package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"singbox-admin/internal/inbound"
)

type SubscriptionProvider interface {
	UserSubscription(token, serverHost string) (string, error)
}

type SubscriptionHandler struct {
	provider   SubscriptionProvider
	serverHost string
}

func NewSubscriptionHandler(p SubscriptionProvider, serverHost string) *SubscriptionHandler {
	return &SubscriptionHandler{provider: p, serverHost: serverHost}
}

func (h *SubscriptionHandler) Get(c *gin.Context) {
	host := h.serverHost
	if host == "" {
		host = c.Request.Host // falls back to the request authority (may include :port)
	}
	body, err := h.provider.UserSubscription(c.Param("token"), host)
	if err != nil {
		if errors.Is(err, inbound.ErrNotFound) {
			c.String(http.StatusNotFound, "not found")
			return
		}
		c.String(http.StatusInternalServerError, "error")
		return
	}
	c.Header("Content-Type", "text/yaml; charset=utf-8")
	c.String(http.StatusOK, body)
}
```

- [ ] **Step 9: Wire the route in main.go**

In `backend/cmd/server/main.go`:

- After `userHandler := handlers.NewUserHandler(inbSvc)` add:
  ```go
  subHandler := handlers.NewSubscriptionHandler(inbSvc, cfg.ServerHost)
  ```
- After `inbound.SeedDefaults(inbSvc)` (and its error check) add the backfill:
  ```go
  if err := inbSvc.BackfillUserTokens(); err != nil {
      log.Fatalf("backfill tokens: %v", err)
  }
  ```
- After the `api := r.Group("/api")` block closes (the `}`), and **before** `web.Register(r)`, add:
  ```go
  r.GET("/sub/:token", subHandler.Get)
  ```

- [ ] **Step 10: Run the handler test + build**

Run: `cd backend && go test ./internal/handlers/ -run Subscription -v && go build ./...`
Expected: PASS, build OK.

- [ ] **Step 11: Commit**

```bash
git add backend/internal/inbound/subscription.go backend/internal/inbound/subscription_test.go backend/internal/handlers/subscription.go backend/internal/handlers/subscription_test.go backend/cmd/server/main.go backend/go.mod backend/go.sum
git commit -m "feat: per-user Clash subscription endpoint"
```

---

### Task 7: Frontend subscription copy action

**Files:**
- Modify: `frontend/lib/api.ts`
- Modify: `frontend/app/users/page.tsx`
- Test: `frontend/app/users/page.test.tsx`

- [ ] **Step 1: Add `subToken` to the `User` type**

In `frontend/lib/api.ts`, change the `User` interface:

```ts
export interface User {
  id: number;
  name: string;
  uuid: string;
  password: string;
  subToken: string;
  upBytes: number;
  downBytes: number;
  inboundIds: number[];
  inboundTags: string[];
}
```

*(The `upBytes`/`downBytes` fields are added now so Task 13 doesn't re-touch the type; the backend already returns them after Task 8.)*

- [ ] **Step 2: Write the failing test**

In `frontend/app/users/page.test.tsx`, add inside `describe("UsersPage", …)`:

```tsx
it("订阅按钮复制订阅链接", async () => {
  const writeText = vi.fn().mockResolvedValue(undefined);
  Object.assign(navigator, { clipboard: { writeText } });
  listUsersMock.mockResolvedValue([
    { id: 1, name: "alice", uuid: "u", password: "p", subToken: "tok123", upBytes: 0, downBytes: 0, inboundIds: [], inboundTags: [] },
  ]);
  render(<UsersPage />);
  await waitFor(() => expect(screen.getByText("alice")).toBeInTheDocument());
  await userEvent.click(screen.getByRole("button", { name: /订阅/ }));
  expect(writeText).toHaveBeenCalledWith(expect.stringContaining("/sub/tok123"));
});
```

Also update the two existing user objects in this test file (the list-display test and any other) to include `subToken: "t", upBytes: 0, downBytes: 0` so they satisfy the new type at runtime (they are plain objects, so this is only needed where the test asserts on them — add the fields to avoid `undefined` in the row).

- [ ] **Step 3: Run to verify it fails**

Run: `cd frontend && npm test -- app/users`
Expected: FAIL — no 订阅 button.

- [ ] **Step 4: Add the copy action**

In `frontend/app/users/page.tsx`, add a handler near `doDelete`:

```tsx
function copySub(token: string) {
  navigator.clipboard.writeText(`${window.location.origin}/sub/${token}`);
}
```

And add the button in the row actions, before 编辑:

```tsx
<Button variant="outline" className="rounded-full" onClick={() => copySub(u.subToken)}>订阅</Button>
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `cd frontend && npm test -- app/users`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/lib/api.ts frontend/app/users/page.tsx frontend/app/users/page.test.tsx
git commit -m "feat: copy Clash subscription link from users page"
```

---

## Section C — Traffic statistics

### Task 8: `Meta` model + user traffic fields + migration

**Files:**
- Create: `backend/internal/models/meta.go`
- Modify: `backend/internal/models/inbound.go`
- Modify: `backend/internal/database/database.go`

- [ ] **Step 1: Create the Meta model**

Create `backend/internal/models/meta.go`:

```go
package models

// Meta is a small panel-owned key/value store (e.g. generated API addresses
// and secrets that must stay stable across restarts).
type Meta struct {
	Key   string `gorm:"primaryKey"`
	Value string `gorm:"not null"`
}
```

- [ ] **Step 2: Add traffic counters to User**

In `backend/internal/models/inbound.go`, add to the `User` struct (after `SubToken`):

```go
	SubToken  string    `gorm:"uniqueIndex" json:"subToken"`
	UpBytes   int64     `gorm:"not null;default:0" json:"upBytes"`
	DownBytes int64     `gorm:"not null;default:0" json:"downBytes"`
```

- [ ] **Step 3: AutoMigrate Meta**

In `backend/internal/database/database.go`, change the AutoMigrate call:

```go
	if err := db.AutoMigrate(&models.Admin{}, &models.Inbound{}, &models.User{}, &models.Meta{}); err != nil {
```

- [ ] **Step 4: Verify build**

Run: `cd backend && go build ./... && go test ./internal/database/ ./internal/inbound/`
Expected: success (existing tests still pass).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/models/meta.go backend/internal/models/inbound.go backend/internal/database/database.go
git commit -m "feat: Meta table and per-user traffic counters"
```

---

### Task 9: Emit `experimental` block; name users `u<id>`; meta-backed API config

**Files:**
- Modify: `backend/internal/inbound/driver.go` (add `ExperimentalConfig`)
- Modify: `backend/internal/inbound/generate.go`
- Modify: `backend/internal/inbound/service.go` (APIConfig + Regenerate)
- Test: `backend/internal/inbound/generate_test.go`

- [ ] **Step 1: Add the config struct**

In `backend/internal/inbound/driver.go`, add near `TypeInfo`:

```go
// ExperimentalConfig holds the sing-box experimental API endpoints the panel
// uses for traffic stats (clash_api = live, v2ray_api = cumulative).
type ExperimentalConfig struct {
	ClashAddr   string
	ClashSecret string
	V2RayAddr   string
}
```

- [ ] **Step 2: Update the failing generate test**

Replace the existing `TestGeneratePicksCredByProtocol` body's `Generate(ins)` call and add assertions. The full updated test:

```go
func TestGeneratePicksCredByProtocol(t *testing.T) {
	vd, _ := Get("vless-reality")
	vs, _ := vd.BuildSettings(nil)
	hd, _ := Get("hysteria2")
	hs, _ := hd.BuildSettings(nil)
	ins := []models.Inbound{
		{ID: 3, Tag: "v1", Type: "vless-reality", Network: "tcp", Port: 8443, Settings: vs,
			Users: []models.User{{ID: 7, Name: "a", UUID: "uuid-x", Password: "pw-x"}}},
		{ID: 4, Tag: "h1", Type: "hysteria2", Network: "udp", Port: 443, Settings: hs,
			Users: []models.User{{ID: 7, Name: "a", UUID: "uuid-x", Password: "pw-x"}}},
	}
	exp := ExperimentalConfig{ClashAddr: "127.0.0.1:9090", ClashSecret: "sec", V2RayAddr: "127.0.0.1:9091"}
	out, err := Generate(ins, exp)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(out, `"uuid": "uuid-x"`) {
		t.Fatal("vless should use UUID")
	}
	if !strings.Contains(out, `"password": "pw-x"`) {
		t.Fatal("hysteria2 should use Password")
	}
	if !strings.Contains(out, `"name": "u7"`) {
		t.Fatal("inbound user name should be the stable stats key u7")
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	expBlock, ok := cfg["experimental"].(map[string]any)
	if !ok {
		t.Fatal("missing experimental block")
	}
	if _, ok := expBlock["clash_api"].(map[string]any); !ok {
		t.Fatal("missing clash_api")
	}
	v2 := expBlock["v2ray_api"].(map[string]any)
	stats := v2["stats"].(map[string]any)
	if stats["enabled"] != true {
		t.Fatal("v2ray stats not enabled")
	}
	users := stats["users"].([]any)
	if len(users) != 1 || users[0] != "u7" {
		t.Fatalf("stats.users=%v, want [u7]", users)
	}
}
```

- [ ] **Step 3: Run to verify it fails**

Run: `cd backend && go test ./internal/inbound/ -run GeneratePicks -v`
Expected: FAIL — `Generate` signature mismatch / no experimental block.

- [ ] **Step 4: Rewrite `Generate`**

Replace `backend/internal/inbound/generate.go` with:

```go
package inbound

import (
	"encoding/json"
	"fmt"
	"sort"

	"singbox-admin/internal/models"
)

func Generate(inbounds []models.Inbound, exp ExperimentalConfig) (string, error) {
	ins := []map[string]any{}
	tags := []string{}
	userSet := map[uint]struct{}{}
	for _, in := range inbounds {
		d, ok := Get(in.Type)
		if !ok {
			return "", fmt.Errorf("%w: %s", ErrUnknownType, in.Type)
		}
		kind := d.CredentialKind()
		creds := make([]Cred, 0, len(in.Users))
		for _, u := range in.Users {
			c := u.UUID
			if kind == "password" {
				c = u.Password
			}
			// Stable per-user stats key (independent of display name).
			creds = append(creds, Cred{Name: fmt.Sprintf("u%d", u.ID), Credential: c})
			userSet[u.ID] = struct{}{}
		}
		piece, err := d.BuildInbound(in.Tag, in.Port, in.Settings, creds)
		if err != nil {
			return "", err
		}
		ins = append(ins, piece)
		tags = append(tags, in.Tag)
	}

	userKeys := make([]string, 0, len(userSet))
	for id := range userSet {
		userKeys = append(userKeys, fmt.Sprintf("u%d", id))
	}
	sort.Strings(userKeys)

	cfg := map[string]any{
		"log":       map[string]any{"level": "info"},
		"inbounds":  ins,
		"outbounds": []map[string]any{{"type": "direct", "tag": "direct"}},
		"experimental": map[string]any{
			"clash_api": map[string]any{
				"external_controller": exp.ClashAddr,
				"secret":              exp.ClashSecret,
			},
			"v2ray_api": map[string]any{
				"listen": exp.V2RayAddr,
				"stats": map[string]any{
					"enabled":  true,
					"inbounds": tags,
					"users":    userKeys,
				},
			},
		},
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
```

- [ ] **Step 5: Add `APIConfig` to the service and update `Regenerate`**

In `backend/internal/inbound/service.go`:

Add constants + the meta-backed accessor (e.g. at the end of the file):

```go
const (
	metaClashAddr   = "clash_api_addr"
	metaClashSecret = "clash_api_secret"
	metaV2RayAddr   = "v2ray_api_addr"

	defaultClashAddr = "127.0.0.1:9090"
	defaultV2RayAddr = "127.0.0.1:9091"
)

// APIConfig returns the experimental API endpoints, generating + persisting
// them in the meta table on first use so config and pollers stay in sync.
func (s *Service) APIConfig() (ExperimentalConfig, error) {
	get := func(key, def string) (string, error) {
		var m models.Meta
		err := s.db.First(&m, "key = ?", key).Error
		if err == nil {
			return m.Value, nil
		}
		val := def
		if key == metaClashSecret {
			val = genToken()
		}
		if err := s.db.Create(&models.Meta{Key: key, Value: val}).Error; err != nil {
			return "", err
		}
		return val, nil
	}
	clashAddr, err := get(metaClashAddr, defaultClashAddr)
	if err != nil {
		return ExperimentalConfig{}, err
	}
	secret, err := get(metaClashSecret, "")
	if err != nil {
		return ExperimentalConfig{}, err
	}
	v2Addr, err := get(metaV2RayAddr, defaultV2RayAddr)
	if err != nil {
		return ExperimentalConfig{}, err
	}
	return ExperimentalConfig{ClashAddr: clashAddr, ClashSecret: secret, V2RayAddr: v2Addr}, nil
}
```

Change `Regenerate` to pass the config:

```go
func (s *Service) Regenerate() error {
	ins, err := s.listInbounds()
	if err != nil {
		return err
	}
	exp, err := s.APIConfig()
	if err != nil {
		return err
	}
	content, err := Generate(ins, exp)
	if err != nil {
		return err
	}
	return s.writer.SaveConfig(content)
}
```

- [ ] **Step 6: Run inbound tests**

Run: `cd backend && go test ./internal/inbound/`
Expected: PASS (generate test + existing suite; `APIConfig` needs the `Meta` table, which the in-memory test dbs migrate via `AutoMigrate(&models.Inbound{}, &models.User{})` — add `&models.Meta{}` to any in-test `AutoMigrate` that exercises `Regenerate`, i.e. in `seed_test.go` and the `newTestService` helper and `subscription_test.go`).

> Concretely: in `seed_test.go`, `subscription_test.go`, and the `newTestService` helper, change `db.AutoMigrate(&models.Inbound{}, &models.User{})` to `db.AutoMigrate(&models.Inbound{}, &models.User{}, &models.Meta{})`.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/inbound/driver.go backend/internal/inbound/generate.go backend/internal/inbound/generate_test.go backend/internal/inbound/service.go backend/internal/inbound/seed_test.go backend/internal/inbound/subscription_test.go backend/internal/inbound/service_test.go
git commit -m "feat: emit experimental stats config with stable user keys"
```

---

### Task 10: V2Ray StatsService gRPC client

**Files:**
- Create: `backend/internal/traffic/stats.go`
- Test: `backend/internal/traffic/stats_test.go` *(new)*

- [ ] **Step 1: Add the gRPC dependency**

Run: `cd backend && go get google.golang.org/grpc@latest`
Expected: `go.mod` lists `google.golang.org/grpc` as a direct require. (Pure Go — stays CGO-free.)

- [ ] **Step 2: Write the failing codec test**

Create `backend/internal/traffic/stats_test.go`:

```go
package traffic

import "testing"

func TestQueryStatsRequestEncode(t *testing.T) {
	b := encodeQueryStatsRequest("user>>>", true)
	// field 1 (pattern) tag = 0x0a, len 7, then "user>>>"; field 2 (reset) tag 0x10, value 1
	want := append([]byte{0x0a, 0x07}, []byte("user>>>")...)
	want = append(want, 0x10, 0x01)
	if string(b) != string(want) {
		t.Fatalf("encode=% x, want % x", b, want)
	}
}

func TestQueryStatsResponseDecode(t *testing.T) {
	// one Stat{name:"user>>>u7>>>traffic>>>uplink", value:1024}
	name := "user>>>u7>>>traffic>>>uplink"
	inner := append([]byte{0x0a, byte(len(name))}, []byte(name)...)
	inner = append(inner, 0x10, 0x80, 0x08) // value 1024 as varint
	msg := append([]byte{0x0a, byte(len(inner))}, inner...)

	stats, err := decodeQueryStatsResponse(msg)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(stats) != 1 || stats[0].Name != name || stats[0].Value != 1024 {
		t.Fatalf("stats=%+v", stats)
	}
}
```

- [ ] **Step 3: Run to verify it fails**

Run: `cd backend && go test ./internal/traffic/ -v`
Expected: FAIL — package/functions don't exist.

- [ ] **Step 4: Implement the client + codec**

Create `backend/internal/traffic/stats.go`:

```go
package traffic

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Stat is one V2Ray stats counter.
type Stat struct {
	Name  string
	Value int64
}

// StatsClient queries cumulative-since-last-reset counters. Abstracted so the
// poller can be tested against a fake.
type StatsClient interface {
	QueryStats(ctx context.Context) ([]Stat, error)
	Close() error
}

// --- minimal protobuf wire encode/decode for the two tiny messages ---

func encodeQueryStatsRequest(pattern string, reset bool) []byte {
	out := []byte{0x0a, byte(len(pattern))}
	out = append(out, []byte(pattern)...)
	if reset {
		out = append(out, 0x10, 0x01)
	}
	return out
}

func decodeVarint(b []byte, i int) (uint64, int) {
	var x uint64
	var s uint
	for ; i < len(b); i++ {
		c := b[i]
		x |= uint64(c&0x7f) << s
		if c < 0x80 {
			return x, i + 1
		}
		s += 7
	}
	return x, i
}

// decodeQueryStatsResponse parses repeated Stat stat = 1 (each an embedded msg).
func decodeQueryStatsResponse(b []byte) ([]Stat, error) {
	stats := []Stat{}
	i := 0
	for i < len(b) {
		tag := b[i]
		i++
		if tag != 0x0a { // field 1, length-delimited
			return nil, fmt.Errorf("unexpected tag 0x%x", tag)
		}
		l, ni := decodeVarint(b, i)
		i = ni
		inner := b[i : i+int(l)]
		i += int(l)
		stats = append(stats, decodeStat(inner))
	}
	return stats, nil
}

func decodeStat(b []byte) Stat {
	var s Stat
	i := 0
	for i < len(b) {
		tag := b[i]
		i++
		switch tag {
		case 0x0a: // name, length-delimited
			l, ni := decodeVarint(b, i)
			i = ni
			s.Name = string(b[i : i+int(l)])
			i += int(l)
		case 0x10: // value, varint
			v, ni := decodeVarint(b, i)
			i = ni
			s.Value = int64(v)
		default:
			return s
		}
	}
	return s
}

// statsCodec marshals our two hand-rolled message types for grpc.ForceCodec.
type statsCodec struct{}

func (statsCodec) Name() string { return "v2ray-stats" }

func (statsCodec) Marshal(v any) ([]byte, error) {
	r, ok := v.(*queryStatsRequest)
	if !ok {
		return nil, fmt.Errorf("unexpected request type %T", v)
	}
	return encodeQueryStatsRequest(r.pattern, r.reset), nil
}

func (statsCodec) Unmarshal(data []byte, v any) error {
	resp, ok := v.(*queryStatsResponse)
	if !ok {
		return fmt.Errorf("unexpected response type %T", v)
	}
	stats, err := decodeQueryStatsResponse(data)
	if err != nil {
		return err
	}
	resp.stats = stats
	return nil
}

type queryStatsRequest struct {
	pattern string
	reset   bool
}
type queryStatsResponse struct {
	stats []Stat
}

// grpcStatsClient is the real client talking to sing-box's v2ray_api.
type grpcStatsClient struct{ cc *grpc.ClientConn }

// NewStatsClient dials the v2ray_api listen address (insecure, localhost).
func NewStatsClient(addr string) (StatsClient, error) {
	cc, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &grpcStatsClient{cc: cc}, nil
}

const statsMethod = "/v2ray.core.app.stats.command.StatsService/QueryStats"

func (c *grpcStatsClient) QueryStats(ctx context.Context) ([]Stat, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req := &queryStatsRequest{pattern: "user>>>", reset: true}
	resp := &queryStatsResponse{}
	if err := c.cc.Invoke(ctx, statsMethod, req, resp, grpc.ForceCodec(statsCodec{})); err != nil {
		return nil, err
	}
	return resp.stats, nil
}

func (c *grpcStatsClient) Close() error { return c.cc.Close() }

var _ = binary.LittleEndian // keep binary import if unused elsewhere
```

> Note: remove the `binary` import + the trailing `var _ =` line if `go vet`/build flags it as unused — it is only there as a guard and can be dropped.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd backend && go test ./internal/traffic/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/traffic/stats.go backend/internal/traffic/stats_test.go backend/go.mod backend/go.sum
git commit -m "feat: minimal V2Ray StatsService gRPC client"
```

---

### Task 11: Traffic poller + `AddTraffic` store

**Files:**
- Create: `backend/internal/traffic/poller.go`
- Test: `backend/internal/traffic/poller_test.go` *(new)*
- Modify: `backend/internal/inbound/service.go` (add `AddTraffic`)
- Modify: `backend/cmd/server/main.go` (start the poller)

- [ ] **Step 1: Write the failing poller test**

Create `backend/internal/traffic/poller_test.go`:

```go
package traffic

import (
	"context"
	"testing"
)

type fakeStats struct{ batches [][]Stat; i int }

func (f *fakeStats) QueryStats(ctx context.Context) ([]Stat, error) {
	if f.i >= len(f.batches) {
		return nil, nil
	}
	b := f.batches[f.i]
	f.i++
	return b, nil
}
func (f *fakeStats) Close() error { return nil }

type memStore struct{ up, down map[uint]int64 }

func (m *memStore) AddTraffic(userID uint, up, down int64) error {
	m.up[userID] += up
	m.down[userID] += down
	return nil
}

func TestPollOnceAccumulates(t *testing.T) {
	fs := &fakeStats{batches: [][]Stat{
		{{Name: "user>>>u7>>>traffic>>>uplink", Value: 100}, {Name: "user>>>u7>>>traffic>>>downlink", Value: 200}},
		{{Name: "user>>>u7>>>traffic>>>uplink", Value: 50}}, // second reset-delta
	}}
	st := &memStore{up: map[uint]int64{}, down: map[uint]int64{}}
	p := NewPoller(fs, st, 0)

	if err := p.pollOnce(context.Background()); err != nil {
		t.Fatalf("pollOnce: %v", err)
	}
	if st.up[7] != 100 || st.down[7] != 200 {
		t.Fatalf("after tick1 up=%d down=%d", st.up[7], st.down[7])
	}
	if err := p.pollOnce(context.Background()); err != nil {
		t.Fatalf("pollOnce: %v", err)
	}
	if st.up[7] != 150 || st.down[7] != 200 {
		t.Fatalf("after tick2 up=%d down=%d (deltas should accumulate)", st.up[7], st.down[7])
	}
}

func TestParseStatName(t *testing.T) {
	id, dir, ok := parseUserStat("user>>>u42>>>traffic>>>downlink")
	if !ok || id != 42 || dir != "downlink" {
		t.Fatalf("parse=%d %s %v", id, dir, ok)
	}
	if _, _, ok := parseUserStat("inbound>>>x>>>traffic>>>uplink"); ok {
		t.Fatal("non-user stat should not parse")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/traffic/ -run 'Poll|ParseStat' -v`
Expected: FAIL — `NewPoller`/`parseUserStat` undefined.

- [ ] **Step 3: Implement the poller**

Create `backend/internal/traffic/poller.go`:

```go
package traffic

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"
)

// TrafficStore persists accumulated per-user traffic.
type TrafficStore interface {
	AddTraffic(userID uint, up, down int64) error
}

type Poller struct {
	client   StatsClient
	store    TrafficStore
	interval time.Duration
}

func NewPoller(client StatsClient, store TrafficStore, interval time.Duration) *Poller {
	return &Poller{client: client, store: store, interval: interval}
}

// parseUserStat parses "user>>>u<id>>>>traffic>>><uplink|downlink>".
func parseUserStat(name string) (uint, string, bool) {
	parts := strings.Split(name, ">>>")
	if len(parts) != 4 || parts[0] != "user" || parts[2] != "traffic" {
		return 0, "", false
	}
	if !strings.HasPrefix(parts[1], "u") {
		return 0, "", false
	}
	id, err := strconv.ParseUint(parts[1][1:], 10, 64)
	if err != nil {
		return 0, "", false
	}
	return uint(id), parts[3], true
}

func (p *Poller) pollOnce(ctx context.Context) error {
	stats, err := p.client.QueryStats(ctx)
	if err != nil {
		return err
	}
	type delta struct{ up, down int64 }
	acc := map[uint]*delta{}
	for _, s := range stats {
		id, dir, ok := parseUserStat(s.Name)
		if !ok {
			continue
		}
		d := acc[id]
		if d == nil {
			d = &delta{}
			acc[id] = d
		}
		if dir == "uplink" {
			d.up += s.Value
		} else if dir == "downlink" {
			d.down += s.Value
		}
	}
	for id, d := range acc {
		if err := p.store.AddTraffic(id, d.up, d.down); err != nil {
			return err
		}
	}
	return nil
}

// Run polls until ctx is cancelled. Errors are logged and skipped so a stopped
// sing-box doesn't kill the loop.
func (p *Poller) Run(ctx context.Context) {
	t := time.NewTicker(p.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := p.pollOnce(ctx); err != nil {
				log.Printf("traffic poll: %v", err)
			}
		}
	}
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd backend && go test ./internal/traffic/ -run 'Poll|ParseStat' -v`
Expected: PASS.

- [ ] **Step 5: Add `AddTraffic` + `ResetUserTraffic` to the service**

In `backend/internal/inbound/service.go`, add:

```go
// AddTraffic accumulates a delta onto a user's cumulative counters.
func (s *Service) AddTraffic(userID uint, up, down int64) error {
	return s.db.Model(&models.User{}).Where("id = ?", userID).
		UpdateColumns(map[string]any{
			"up_bytes":   gorm.Expr("up_bytes + ?", up),
			"down_bytes": gorm.Expr("down_bytes + ?", down),
		}).Error
}

// ResetUserTraffic zeroes a user's cumulative counters.
func (s *Service) ResetUserTraffic(id uint) error {
	var u models.User
	if err := s.db.First(&u, id).Error; err != nil {
		return ErrNotFound
	}
	return s.db.Model(&u).UpdateColumns(map[string]any{"up_bytes": 0, "down_bytes": 0}).Error
}
```

(`gorm` is already imported in `service.go`.)

Also set the new fields in `ListUserViews`’ per-user `v`:

```go
		v := UserView{ID: u.ID, Name: u.Name, UUID: u.UUID, Password: u.Password, SubToken: u.SubToken,
			UpBytes: u.UpBytes, DownBytes: u.DownBytes, InboundIDs: []uint{}, InboundTags: []string{}}
```

…and add the fields to the `UserView` struct:

```go
	SubToken    string   `json:"subToken"`
	UpBytes     int64    `json:"upBytes"`
	DownBytes   int64    `json:"downBytes"`
	InboundIDs  []uint   `json:"inboundIds"`
```

- [ ] **Step 6: Start the poller in main.go**

In `backend/cmd/server/main.go`:

Add imports: `"context"`, `"singbox-admin/internal/traffic"`.

After `inbSvc.BackfillUserTokens()` wiring and before building handlers, add:

```go
	exp, err := inbSvc.APIConfig()
	if err != nil {
		log.Fatalf("api config: %v", err)
	}
	if statsClient, err := traffic.NewStatsClient(exp.V2RayAddr); err != nil {
		log.Printf("traffic stats disabled: %v", err)
	} else {
		poller := traffic.NewPoller(statsClient, inbSvc, 10*time.Second)
		go poller.Run(context.Background())
	}
```

- [ ] **Step 7: Build + test**

Run: `cd backend && go build ./... && go test ./internal/inbound/ ./internal/traffic/`
Expected: success.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/traffic/poller.go backend/internal/traffic/poller_test.go backend/internal/inbound/service.go backend/cmd/server/main.go
git commit -m "feat: traffic poller accumulating per-user usage"
```

---

### Task 12: Live throughput read + handlers

**Files:**
- Create: `backend/internal/traffic/live.go`
- Modify: `backend/internal/handlers/user.go` (ResetTraffic + interface)
- Create: `backend/internal/handlers/traffic.go`
- Modify: `backend/cmd/server/main.go` (routes)
- Test: `backend/internal/handlers/traffic_test.go` *(new)*

- [ ] **Step 1: Implement the live read**

Create `backend/internal/traffic/live.go`:

```go
package traffic

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Live is a one-tick throughput snapshot in bytes/second.
type Live struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

// ReadLive reads a single tick from the Clash API /traffic stream. On any
// failure it returns a zero snapshot (the widget degrades gracefully).
func ReadLive(addr, secret string) Live {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/traffic", nil)
	if err != nil {
		return Live{}
	}
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Live{}
	}
	defer resp.Body.Close()
	sc := bufio.NewScanner(resp.Body)
	if sc.Scan() {
		var l Live
		if json.Unmarshal(sc.Bytes(), &l) == nil {
			return l
		}
	}
	return Live{}
}
```

- [ ] **Step 2: Write the failing handler test**

Create `backend/internal/handlers/traffic_test.go`:

```go
package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTrafficLiveReturnsSnapshot(t *testing.T) {
	// fake clash api emitting one traffic line
	clash := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"up":1000,"down":2000}` + "\n"))
	}))
	defer clash.Close()
	addr := clash.Listener.Addr().String()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewTrafficHandler(addr, "")
	r.GET("/api/traffic/live", h.Live)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/traffic/live", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d", w.Code)
	}
	var body struct{ Up, Down int64 }
	json.Unmarshal(w.Body.Bytes(), &body)
	if body.Up != 1000 || body.Down != 2000 {
		t.Fatalf("body=%+v", body)
	}
}
```

- [ ] **Step 3: Run to verify it fails**

Run: `cd backend && go test ./internal/handlers/ -run TrafficLive -v`
Expected: FAIL — `NewTrafficHandler` undefined.

- [ ] **Step 4: Implement the traffic handler**

Create `backend/internal/handlers/traffic.go`:

```go
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"singbox-admin/internal/traffic"
)

type TrafficHandler struct {
	clashAddr   string
	clashSecret string
}

func NewTrafficHandler(clashAddr, clashSecret string) *TrafficHandler {
	return &TrafficHandler{clashAddr: clashAddr, clashSecret: clashSecret}
}

func (h *TrafficHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, traffic.ReadLive(h.clashAddr, h.clashSecret))
}
```

- [ ] **Step 5: Add `ResetTraffic` to the user handler**

In `backend/internal/handlers/user.go`:

Add to the `UserController` interface:

```go
	DeleteUser(id uint) error
	ResetUserTraffic(id uint) error
```

Add the handler method:

```go
func (h *UserHandler) ResetTraffic(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	if err := h.ctrl.ResetUserTraffic(id); err != nil {
		writeUserErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
```

- [ ] **Step 6: Wire routes in main.go**

In `backend/cmd/server/main.go`:

Build the traffic handler after `exp, err := inbSvc.APIConfig()` (reuse `exp`):

```go
	trafficHandler := handlers.NewTrafficHandler(exp.ClashAddr, exp.ClashSecret)
```

Inside the `authed` group, add:

```go
		authed.GET("/traffic/live", trafficHandler.Live)
		authed.POST("/users/:id/reset-traffic", userHandler.ResetTraffic)
```

- [ ] **Step 7: Build + test**

Run: `cd backend && go test ./internal/handlers/ ./internal/traffic/ && go build ./...`
Expected: PASS, build OK.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/traffic/live.go backend/internal/handlers/traffic.go backend/internal/handlers/traffic_test.go backend/internal/handlers/user.go backend/cmd/server/main.go
git commit -m "feat: live throughput endpoint and reset-traffic"
```

---

### Task 13: Frontend traffic UI

**Files:**
- Modify: `frontend/lib/utils.ts` (add `formatBytes`)
- Modify: `frontend/lib/api.ts` (`getLiveTraffic`, `resetUserTraffic`)
- Modify: `frontend/app/users/page.tsx` (columns + 重置流量)
- Modify: `frontend/app/dashboard/page.tsx` (live widget)
- Test: `frontend/app/users/page.test.tsx`, `frontend/app/dashboard/page.test.tsx`

- [ ] **Step 1: Add `formatBytes`**

In `frontend/lib/utils.ts`, append:

```ts
export function formatBytes(n: number): string {
  if (!n || n < 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}
```

- [ ] **Step 2: Add API functions**

In `frontend/lib/api.ts`, append:

```ts
export interface LiveTraffic {
  up: number;
  down: number;
}

export async function getLiveTraffic(): Promise<LiveTraffic> {
  const res = await request("/api/traffic/live");
  if (!res.ok) throw new Error("加载实时流量失败");
  return res.json();
}

export async function resetUserTraffic(id: number): Promise<void> {
  const res = await request(`/api/users/${id}/reset-traffic`, { method: "POST" });
  if (!res.ok) throw new Error("重置流量失败");
}
```

- [ ] **Step 3: Write the failing users-page test**

In `frontend/app/users/page.test.tsx`:

Add `resetUserTraffic` and `getLiveTraffic` to the `@/lib/api` mock object:

```ts
  resetUserTraffic: vi.fn(),
  getLiveTraffic: vi.fn(),
```

Add a test:

```tsx
it("显示用户上下行总量", async () => {
  listUsersMock.mockResolvedValue([
    { id: 1, name: "alice", uuid: "u", password: "p", subToken: "t", upBytes: 1024, downBytes: 1048576, inboundIds: [], inboundTags: [] },
  ]);
  render(<UsersPage />);
  await waitFor(() => expect(screen.getByText("alice")).toBeInTheDocument());
  expect(screen.getByText("1.0 KB")).toBeInTheDocument();
  expect(screen.getByText("1.0 MB")).toBeInTheDocument();
});
```

- [ ] **Step 4: Run to verify it fails**

Run: `cd frontend && npm test -- app/users`
Expected: FAIL — no formatted byte cells.

- [ ] **Step 5: Add columns + 重置流量 to users page**

In `frontend/app/users/page.tsx`:

- Import the helper + api: add `formatBytes` to the `@/lib/utils` import (create the import if absent) and add `resetUserTraffic` to the `@/lib/api` import.
- Add state + handler near `confirmDelete`:
  ```tsx
  const [confirmResetTraffic, setConfirmResetTraffic] = useState<User | null>(null);
  async function doResetTraffic() {
    if (!confirmResetTraffic) return;
    await resetUserTraffic(confirmResetTraffic.id);
    setConfirmResetTraffic(null);
    refresh();
  }
  ```
- Update the grid template to add two columns. Change both the header row and the data row `grid-cols-[1fr_2fr_2fr_2fr_auto]` to `grid-cols-[1fr_1.6fr_1.6fr_1.6fr_1fr_auto]` and add header cells:
  ```tsx
  <span>名称</span><span>UUID</span><span>密码</span><span>入站</span><span>流量</span><span></span>
  ```
- In the data row, add a traffic cell after the 入站 cell:
  ```tsx
  <span className="text-xs text-muted-foreground">↑{formatBytes(u.upBytes)} ↓{formatBytes(u.downBytes)}</span>
  ```
- Add a 重置流量 button in the row actions:
  ```tsx
  <Button variant="outline" className="rounded-full" onClick={() => setConfirmResetTraffic(u)}>重置流量</Button>
  ```
- Add the confirm modal next to the existing delete modal:
  ```tsx
  <Modal open={confirmResetTraffic !== null} onClose={() => setConfirmResetTraffic(null)} title="重置流量">
    <p className="mb-4 text-sm text-muted-foreground">
      确定清零用户 <span className="font-mono">{confirmResetTraffic?.name}</span> 的累计流量？
    </p>
    <div className="flex justify-end gap-3">
      <Button variant="outline" className="rounded-full" onClick={() => setConfirmResetTraffic(null)}>取消</Button>
      <Button className="rounded-full" onClick={doResetTraffic}>确认重置</Button>
    </div>
  </Modal>
  ```

- [ ] **Step 6: Run the users test to verify it passes**

Run: `cd frontend && npm test -- app/users`
Expected: PASS.

- [ ] **Step 7: Write the failing dashboard test**

In `frontend/app/dashboard/page.test.tsx` (create if missing — mirror the mock style of other page tests), add `getLiveTraffic` to the `@/lib/api` mock returning `{ up: 1024, down: 2048 }`, then:

```tsx
it("显示实时吞吐", async () => {
  render(<DashboardPage />);
  await waitFor(() => expect(screen.getByText(/实时吞吐|实时流量/)).toBeInTheDocument());
  await waitFor(() => expect(screen.getByText(/1\.0 KB\/s/)).toBeInTheDocument());
});
```

> If `dashboard/page.test.tsx` does not exist, create it with the standard header: mock `next/navigation` (`useRouter`/`usePathname`) and `@/lib/api` (`getStatus`, `startSingbox`, `stopSingbox`, `applySingbox`, `getLiveTraffic`, `logout`, `UnauthorizedError`).

- [ ] **Step 8: Run to verify it fails**

Run: `cd frontend && npm test -- app/dashboard`
Expected: FAIL — no live widget.

- [ ] **Step 9: Add the live widget to the dashboard**

In `frontend/app/dashboard/page.tsx`:

- Add imports: `getLiveTraffic, LiveTraffic` to the `@/lib/api` import and `formatBytes` from `@/lib/utils`.
- Add state + polling effect:
  ```tsx
  const [live, setLive] = useState<LiveTraffic>({ up: 0, down: 0 });
  useEffect(() => {
    let active = true;
    const tick = () => getLiveTraffic().then((l) => active && setLive(l)).catch(() => {});
    tick();
    const id = setInterval(tick, 3000);
    return () => {
      active = false;
      clearInterval(id);
    };
  }, []);
  ```
- Add a card after the status card (inside the `AppShell`, below the existing `</Card>`):
  ```tsx
  <Card className="mt-6 rounded-lg">
    <CardHeader>
      <CardTitle className="font-mono text-xs tracking-wider text-muted-foreground uppercase">
        实时吞吐
      </CardTitle>
    </CardHeader>
    <CardContent>
      <div className="flex gap-10">
        <div>
          <p className="font-mono text-xs tracking-wider text-muted-foreground uppercase">上行</p>
          <p className="text-2xl font-normal">{formatBytes(live.up)}/s</p>
        </div>
        <div>
          <p className="font-mono text-xs tracking-wider text-muted-foreground uppercase">下行</p>
          <p className="text-2xl font-normal">{formatBytes(live.down)}/s</p>
        </div>
      </div>
    </CardContent>
  </Card>
  ```

- [ ] **Step 10: Run dashboard test + full suite**

Run: `cd frontend && npm test -- app/dashboard && npm test`
Expected: PASS.

- [ ] **Step 11: Commit**

```bash
git add frontend/lib/utils.ts frontend/lib/api.ts frontend/app/users/page.tsx frontend/app/users/page.test.tsx frontend/app/dashboard/page.tsx frontend/app/dashboard/page.test.tsx
git commit -m "feat: per-user traffic columns and live throughput widget"
```

---

### Task 14: Full build + manual verification

**Files:** none (verification only)

- [ ] **Step 1: Run all backend tests**

Run: `cd backend && go vet ./... && go test ./...`
Expected: all pass, vet clean.

- [ ] **Step 2: Run all frontend tests**

Run: `cd frontend && npm test`
Expected: all pass.

- [ ] **Step 3: Build the single binary**

Run: `cd /Users/silence/Documents/sing-box-admin && make build`
Expected: builds `backend/bin/sing-box-admin`.

- [ ] **Step 4: Restore the dist artifacts dirtied by the build**

Run: `git checkout -- backend/internal/web/dist`
Expected: working tree clean except intended changes.

- [ ] **Step 5: Run the binary and eyeball the UI**

Run: `./backend/bin/sing-box-admin` then open http://localhost:8080
Verify: login → sidebar/header stay fixed while content scrolls; nav shows group headings; users page shows traffic columns + 订阅 + 重置流量 (二次确认); dashboard shows the live throughput card; `GET /sub/<token>` returns Clash YAML (copy a token's link from the users page). Stop the binary when done.

> sing-box itself is absent locally, so live numbers stay at 0 and the poller logs a connection error each tick — that is expected; real traffic accounting is validated on the VPS.

- [ ] **Step 6: Final commit (if any verification fixes were needed)**

```bash
git add -A
git commit -m "chore: M6 verification fixes"
```

---

## Self-review notes

- **Spec coverage:** A (layout/nav/shadcn rule) → Tasks 1–2; B (subscription) → Tasks 3–7; C (traffic stats + live) → Tasks 8–13; sequencing/verification → Task 14. `SERVER_HOST` (Task 3), per-user token (Task 5), `u<id>` stat key (Task 9), both stats mechanisms (Tasks 10–12), persisted cumulative + reset (Tasks 8/11/12). All spec sections map to tasks.
- **Type consistency:** `ClashProxy(name, serverHost string, port uint16, settings string, cred Cred)` is identical across the interface (Task 4) and both drivers; `ExperimentalConfig{ClashAddr,ClashSecret,V2RayAddr}` is identical in Tasks 9/11/12; `UserView` accretes `SubToken/UpBytes/DownBytes` once (Tasks 5/11) matching the frontend `User` type (Task 7); `Stat{Name,Value}` and `parseUserStat` agree between Tasks 10 and 11.
- **Migration note:** any in-memory test db that exercises `Regenerate`/`APIConfig` must migrate `&models.Meta{}` (called out in Task 9 Step 6).
- **Out of scope (unchanged):** per-inbound breakdown, historical charts, non-Clash subscription formats, quotas.

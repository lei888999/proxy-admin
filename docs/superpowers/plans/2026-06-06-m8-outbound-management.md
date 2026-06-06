# M8 — Outbound Management & Per-User Routing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the admin define http/socks5 upstream outbounds and route a chosen user's traffic (and DNS) through one of them, hiding the VPS egress IP and preventing DNS leaks.

**Architecture:** A new `Outbound` model + a nullable `User.OutboundID`. `inbound.Generate` grows to emit the `outbounds`, `route`, and `dns` blocks derived from each user's outbound assignment. Outbound CRUD + user assignment live on the existing `inbound.Service`. New 出站 page + an outbound selector in the user modal.

**Tech Stack:** Go (Gin, GORM, glebarez/sqlite), sing-box config JSON, Next.js 16 / Tailwind v4 / shadcn (base-nova), Vitest.

**Reference spec:** `docs/superpowers/specs/2026-06-06-outbound-management-design.md`

---

## File structure

**Backend (`backend/`)**
- `internal/models/outbound.go` *(new)* — `Outbound` model.
- `internal/models/inbound.go` — add `OutboundID *uint` to `User`.
- `internal/database/database.go` — AutoMigrate `Outbound`.
- `internal/inbound/generate.go` — `Generate(inbounds, outbounds, exp)`; emit `outbounds`/`route`/`dns`.
- `internal/inbound/outbound.go` *(new)* — `OutboundView`, outbound CRUD, `listOutbounds`, `validateOutbound`.
- `internal/inbound/service.go` — add outbound errors; `Regenerate` loads outbounds; `CreateUser`/`UpdateUser` accept `outboundID`; `UserView.OutboundID`.
- `internal/handlers/outbound.go` *(new)* — `OutboundController` + `OutboundHandler`.
- `internal/handlers/user.go` — `userBody.OutboundID`; pass through; interface signatures; error mapping.
- `cmd/server/main.go` — outbound handler + routes.

**Frontend (`frontend/`)**
- `lib/api.ts` — `Outbound` type + CRUD; `User.outboundId`; `createUser`/`updateUser` carry `outboundId`.
- `components/app-shell.tsx` — add 出站 nav entry.
- `app/outbounds/page.tsx` *(new)* — outbound table + modal.
- `app/users/page.tsx` — outbound `Select` in the user modal.
- test files alongside.

---

## Task 1: Outbound model + `User.OutboundID` + migration

**Files:**
- Create: `backend/internal/models/outbound.go`
- Modify: `backend/internal/models/inbound.go`, `backend/internal/database/database.go`

- [ ] **Step 1: Create the Outbound model**

Create `backend/internal/models/outbound.go`:

```go
package models

import "time"

// Outbound is an upstream proxy (http or socks5) that user traffic can egress
// through. Username/Password are the upstream credentials (optional).
type Outbound struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Tag       string    `gorm:"uniqueIndex;not null" json:"tag"`
	Type      string    `gorm:"not null" json:"type"` // "http" | "socks5"
	Server    string    `gorm:"not null" json:"server"`
	Port      uint16    `gorm:"not null" json:"port"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
```

- [ ] **Step 2: Add `OutboundID` to User**

In `backend/internal/models/inbound.go`, add to the `User` struct (after `DownBytes`):

```go
	DownBytes int64     `gorm:"not null;default:0" json:"downBytes"`
	OutboundID *uint    `json:"outboundId"`
```

- [ ] **Step 3: AutoMigrate Outbound**

In `backend/internal/database/database.go`, add `&models.Outbound{}` to the `AutoMigrate(...)` call:

```go
	if err := db.AutoMigrate(&models.Admin{}, &models.Inbound{}, &models.User{}, &models.Meta{}, &models.Outbound{}); err != nil {
```

- [ ] **Step 4: Verify build**

Run: `cd backend && go build ./...`
Expected: success.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/models/outbound.go backend/internal/models/inbound.go backend/internal/database/database.go
git commit -m "feat: Outbound model and nullable User.OutboundID"
```

---

## Task 2: Generate `outbounds` / `route` / `dns`

**Files:**
- Modify: `backend/internal/inbound/generate.go`
- Test: `backend/internal/inbound/generate_test.go`

- [ ] **Step 1: Update the failing test**

Replace the body of `TestGeneratePicksCredByProtocol` in `backend/internal/inbound/generate_test.go` with this (changes the call to 3 args, assigns a user to an outbound, asserts outbound/route/dns):

```go
func TestGeneratePicksCredByProtocol(t *testing.T) {
	vd, _ := Get("vless-reality")
	vs, _ := vd.BuildSettings(nil)
	hd, _ := Get("hysteria2")
	hs, _ := hd.BuildSettings(nil)
	ob := uint(9)
	ins := []models.Inbound{
		{ID: 3, Tag: "v1", Type: "vless-reality", Network: "tcp", Port: 8443, Settings: vs,
			Users: []models.User{{ID: 7, Name: "a", UUID: "uuid-x", Password: "pw-x", OutboundID: &ob}}},
		{ID: 4, Tag: "h1", Type: "hysteria2", Network: "udp", Port: 443, Settings: hs,
			Users: []models.User{{ID: 8, Name: "b", UUID: "uuid-y", Password: "pw-y"}}}, // no outbound -> direct
	}
	outs := []models.Outbound{
		{ID: 9, Tag: "proxyA", Type: "socks5", Server: "1.2.3.4", Port: 1080, Username: "u", Password: "p"},
	}
	exp := ExperimentalConfig{ClashAddr: "127.0.0.1:9090", ClashSecret: "sec"}
	out, err := Generate(ins, outs, exp)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(out, `"uuid": "uuid-x"`) || !strings.Contains(out, `"password": "pw-y"`) {
		t.Fatal("creds not emitted per protocol")
	}

	var cfg map[string]any
	if err := json.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	// outbounds: direct + the socks5 (sing-box type "socks")
	obs := cfg["outbounds"].([]any)
	var foundSocks bool
	for _, o := range obs {
		m := o.(map[string]any)
		if m["tag"] == "proxyA" {
			foundSocks = true
			if m["type"] != "socks" || m["server"] != "1.2.3.4" || m["username"] != "u" {
				t.Fatalf("socks outbound wrong: %v", m)
			}
		}
	}
	if !foundSocks {
		t.Fatal("proxyA outbound missing")
	}

	// route: rule maps u7 -> proxyA, final direct, u8 has no rule
	route := cfg["route"].(map[string]any)
	if route["final"] != "direct" {
		t.Fatalf("route.final=%v", route["final"])
	}
	rules := route["rules"].([]any)
	if len(rules) != 1 {
		t.Fatalf("want 1 route rule, got %d", len(rules))
	}
	r0 := rules[0].(map[string]any)
	if r0["outbound"] != "proxyA" || r0["auth_user"].([]any)[0] != "u7" {
		t.Fatalf("route rule wrong: %v", r0)
	}

	// dns: detour server + rule for proxyA, final local
	dns := cfg["dns"].(map[string]any)
	if dns["final"] != "local" {
		t.Fatalf("dns.final=%v", dns["final"])
	}
	var foundDNS bool
	for _, s := range dns["servers"].([]any) {
		m := s.(map[string]any)
		if m["tag"] == "dns-proxyA" && m["detour"] == "proxyA" {
			foundDNS = true
		}
	}
	if !foundDNS {
		t.Fatal("dns detour server for proxyA missing")
	}
	dnsRules := dns["rules"].([]any)
	if len(dnsRules) != 1 || dnsRules[0].(map[string]any)["server"] != "dns-proxyA" {
		t.Fatalf("dns rule wrong: %v", dnsRules)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/inbound/ -run GeneratePicks -v`
Expected: FAIL (compile: `Generate` takes 2 args / signature mismatch).

- [ ] **Step 3: Rewrite `generate.go`**

Replace `backend/internal/inbound/generate.go` with:

```go
package inbound

import (
	"encoding/json"
	"fmt"
	"sort"

	"singbox-admin/internal/models"
)

// dnsResolver is the encrypted (DoT) upstream resolver used by per-outbound DNS
// servers so a proxied user's queries egress through their outbound.
const dnsResolver = "tls://1.1.1.1"

func Generate(inbounds []models.Inbound, outbounds []models.Outbound, exp ExperimentalConfig) (string, error) {
	tagByID := map[uint]string{}
	for _, o := range outbounds {
		tagByID[o.ID] = o.Tag
	}

	ins := []map[string]any{}
	userOutbound := map[uint]string{} // user id -> outbound tag
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
			// Stable per-user key; surfaces as the Clash-API connection's
			// metadata.user (traffic stats) and as auth_user in route/dns rules.
			creds = append(creds, Cred{Name: fmt.Sprintf("u%d", u.ID), Credential: c})
			if u.OutboundID != nil {
				if tag, ok := tagByID[*u.OutboundID]; ok {
					userOutbound[u.ID] = tag
				}
			}
		}
		piece, err := d.BuildInbound(in.Tag, in.Port, in.Settings, creds)
		if err != nil {
			return "", err
		}
		ins = append(ins, piece)
	}

	// outbounds array: direct + each configured outbound.
	obs := []map[string]any{{"type": "direct", "tag": "direct"}}
	for _, o := range outbounds {
		ob := map[string]any{"tag": o.Tag, "server": o.Server, "server_port": o.Port}
		if o.Type == "socks5" {
			ob["type"] = "socks" // sing-box's name for SOCKS5
		} else {
			ob["type"] = "http"
		}
		if o.Username != "" {
			ob["username"] = o.Username
		}
		if o.Password != "" {
			ob["password"] = o.Password
		}
		obs = append(obs, ob)
	}

	// Group assigned users by outbound tag (sorted for deterministic output).
	usersByTag := map[string][]string{}
	for uid, tag := range userOutbound {
		usersByTag[tag] = append(usersByTag[tag], fmt.Sprintf("u%d", uid))
	}
	tags := make([]string, 0, len(usersByTag))
	for tag := range usersByTag {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	routeRules := []map[string]any{}
	dnsServers := []map[string]any{{"tag": "local", "address": "local"}}
	dnsRules := []map[string]any{}
	for _, tag := range tags {
		users := usersByTag[tag]
		sort.Strings(users)
		routeRules = append(routeRules, map[string]any{"auth_user": users, "outbound": tag})
		dnsServers = append(dnsServers, map[string]any{"tag": "dns-" + tag, "address": dnsResolver, "detour": tag})
		dnsRules = append(dnsRules, map[string]any{"auth_user": users, "server": "dns-" + tag})
	}

	cfg := map[string]any{
		"log": map[string]any{"level": "info"},
		"dns": map[string]any{
			"servers":  dnsServers,
			"rules":    dnsRules,
			"final":    "local",
			"strategy": "prefer_ipv4",
		},
		"inbounds":  ins,
		"outbounds": obs,
		"route": map[string]any{
			"rules": routeRules,
			"final": "direct",
		},
		"experimental": map[string]any{
			"clash_api": map[string]any{
				"external_controller": exp.ClashAddr,
				"secret":              exp.ClashSecret,
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

- [ ] **Step 4: Run to verify it passes**

Run: `cd backend && go test ./internal/inbound/ -run GeneratePicks -v`
Expected: PASS. (Other callers of `Generate` — only `Regenerate` — break until Task 3; that's fine, the package won't fully compile yet. If you need the package green now, do Task 3's `Regenerate` edit before running the whole package.)

- [ ] **Step 5: Commit**

```bash
git add backend/internal/inbound/generate.go backend/internal/inbound/generate_test.go
git commit -m "feat: generate outbounds, per-user route, and DNS-leak-safe dns block"
```

---

## Task 3: Outbound CRUD on the service

**Files:**
- Create: `backend/internal/inbound/outbound.go`
- Modify: `backend/internal/inbound/service.go` (errors + `Regenerate`)
- Test: `backend/internal/inbound/outbound_test.go` *(new)*

- [ ] **Step 1: Add outbound errors + load outbounds in Regenerate**

In `backend/internal/inbound/service.go`, extend the error block:

```go
var (
	ErrTagExists       = errors.New("tag exists")
	ErrNotFound        = errors.New("not found")
	ErrInvalidTag      = errors.New("invalid tag")
	ErrInvalidName     = errors.New("invalid name")
	ErrPortInUse       = errors.New("port in use")
	ErrInvalidType     = errors.New("invalid type")
	ErrInvalidOutbound = errors.New("invalid outbound")
)
```

And change `Regenerate` to load and pass outbounds:

```go
func (s *Service) Regenerate() error {
	ins, err := s.listInbounds()
	if err != nil {
		return err
	}
	obs, err := s.listOutbounds()
	if err != nil {
		return err
	}
	exp, err := s.APIConfig()
	if err != nil {
		return err
	}
	content, err := Generate(ins, obs, exp)
	if err != nil {
		return err
	}
	return s.writer.SaveConfig(content)
}
```

- [ ] **Step 2: Write the failing test**

Create `backend/internal/inbound/outbound_test.go`:

```go
package inbound

import (
	"testing"

	"singbox-admin/internal/models"
)

func TestOutboundCRUD(t *testing.T) {
	s, _ := newTestService(t)
	o, err := s.CreateOutbound("socks5", "proxyA", "1.2.3.4", 1080, "u", "p")
	if err != nil {
		t.Fatalf("CreateOutbound: %v", err)
	}
	if o.ID == 0 {
		t.Fatal("no id")
	}
	if _, err := s.CreateOutbound("socks5", "proxyA", "5.6.7.8", 1080, "", ""); err != ErrTagExists {
		t.Fatalf("dup tag err=%v, want ErrTagExists", err)
	}
	if _, err := s.CreateOutbound("ftp", "x", "1.2.3.4", 1, "", ""); err != ErrInvalidType {
		t.Fatalf("bad type err=%v, want ErrInvalidType", err)
	}
	views, _ := s.ListOutboundViews()
	if len(views) != 1 || views[0].Tag != "proxyA" || views[0].Type != "socks5" {
		t.Fatalf("views=%v", views)
	}
	if _, err := s.UpdateOutbound(o.ID, "http", "proxyB", "9.9.9.9", 8080, "", ""); err != nil {
		t.Fatalf("UpdateOutbound: %v", err)
	}
}

func TestDeleteOutboundNullsUsers(t *testing.T) {
	s, _ := newTestService(t)
	o, _ := s.CreateOutbound("http", "proxyA", "1.2.3.4", 8080, "", "")
	u, err := s.CreateUser("alice", nil, &o.ID)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := s.DeleteOutbound(o.ID); err != nil {
		t.Fatalf("DeleteOutbound: %v", err)
	}
	var got models.User
	s.db.First(&got, u.ID)
	if got.OutboundID != nil {
		t.Fatalf("user outbound not nulled: %v", got.OutboundID)
	}
}
```

> Note: `CreateUser` now takes a third arg `outboundID *uint` — that signature change is implemented in Task 4. If you implement Task 3 before Task 4, this test won't compile yet; implement Task 4's `CreateUser`/`UpdateUser`/`validateOutbound` changes together with this task, or temporarily call `s.CreateUser("alice", nil, &o.ID)` after Task 4. Recommended: do Task 3 and Task 4 back-to-back, committing each.

- [ ] **Step 3: Run to verify it fails**

Run: `cd backend && go test ./internal/inbound/ -run Outbound -v`
Expected: FAIL (`CreateOutbound`/`ListOutboundViews`/`DeleteOutbound` undefined).

- [ ] **Step 4: Implement outbound CRUD**

Create `backend/internal/inbound/outbound.go`:

```go
package inbound

import (
	"strings"

	"singbox-admin/internal/models"
)

type OutboundView struct {
	ID       uint   `json:"id"`
	Tag      string `json:"tag"`
	Type     string `json:"type"`
	Server   string `json:"server"`
	Port     uint16 `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

var outboundTypes = map[string]bool{"http": true, "socks5": true}

func (s *Service) listOutbounds() ([]models.Outbound, error) {
	var obs []models.Outbound
	err := s.db.Order("id").Find(&obs).Error
	return obs, err
}

func (s *Service) ListOutboundViews() ([]OutboundView, error) {
	obs, err := s.listOutbounds()
	if err != nil {
		return nil, err
	}
	views := make([]OutboundView, 0, len(obs))
	for _, o := range obs {
		views = append(views, OutboundView{
			ID: o.ID, Tag: o.Tag, Type: o.Type, Server: o.Server,
			Port: o.Port, Username: o.Username, Password: o.Password,
		})
	}
	return views, nil
}

func validateOutboundFields(typ, tag, server string, port uint16) (string, string, string, error) {
	typ = strings.TrimSpace(typ)
	if !outboundTypes[typ] {
		return "", "", "", ErrInvalidType
	}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "", "", "", ErrInvalidTag
	}
	server = strings.TrimSpace(server)
	if server == "" || port == 0 {
		return "", "", "", ErrInvalidOutbound
	}
	return typ, tag, server, nil
}

func (s *Service) CreateOutbound(typ, tag, server string, port uint16, username, password string) (models.Outbound, error) {
	typ, tag, server, err := validateOutboundFields(typ, tag, server, port)
	if err != nil {
		return models.Outbound{}, err
	}
	var count int64
	s.db.Model(&models.Outbound{}).Where("tag = ?", tag).Count(&count)
	if count > 0 {
		return models.Outbound{}, ErrTagExists
	}
	o := models.Outbound{Tag: tag, Type: typ, Server: server, Port: port, Username: username, Password: password}
	if err := s.db.Create(&o).Error; err != nil {
		return models.Outbound{}, err
	}
	return o, s.Regenerate()
}

func (s *Service) UpdateOutbound(id uint, typ, tag, server string, port uint16, username, password string) (models.Outbound, error) {
	var o models.Outbound
	if err := s.db.First(&o, id).Error; err != nil {
		return models.Outbound{}, ErrNotFound
	}
	typ, tag, server, err := validateOutboundFields(typ, tag, server, port)
	if err != nil {
		return models.Outbound{}, err
	}
	var count int64
	s.db.Model(&models.Outbound{}).Where("tag = ? AND id <> ?", tag, id).Count(&count)
	if count > 0 {
		return models.Outbound{}, ErrTagExists
	}
	o.Type, o.Tag, o.Server, o.Port, o.Username, o.Password = typ, tag, server, port, username, password
	if err := s.db.Save(&o).Error; err != nil {
		return models.Outbound{}, err
	}
	return o, s.Regenerate()
}

func (s *Service) DeleteOutbound(id uint) error {
	var o models.Outbound
	if err := s.db.First(&o, id).Error; err != nil {
		return ErrNotFound
	}
	// Referencing users fall back to direct.
	if err := s.db.Model(&models.User{}).Where("outbound_id = ?", id).Update("outbound_id", nil).Error; err != nil {
		return err
	}
	if err := s.db.Delete(&o).Error; err != nil {
		return err
	}
	return s.Regenerate()
}

// validateOutbound checks that an optional assigned outbound exists.
func (s *Service) validateOutbound(id *uint) error {
	if id == nil {
		return nil
	}
	var count int64
	s.db.Model(&models.Outbound{}).Where("id = ?", *id).Count(&count)
	if count == 0 {
		return ErrInvalidOutbound
	}
	return nil
}
```

- [ ] **Step 5: Run (after Task 4) to verify it passes**

Run: `cd backend && go test ./internal/inbound/ -run Outbound -v`
Expected: PASS (requires Task 4's `CreateUser` signature). Build the package after Task 4.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/inbound/outbound.go backend/internal/inbound/outbound_test.go backend/internal/inbound/service.go
git commit -m "feat: outbound CRUD on the service"
```

---

## Task 4: User outbound assignment

**Files:**
- Modify: `backend/internal/inbound/service.go` (CreateUser/UpdateUser/UserView/ListUserViews)
- Test: `backend/internal/inbound/service_test.go`

- [ ] **Step 1: Write the failing test**

Append to `backend/internal/inbound/service_test.go`:

```go
func TestUserOutboundAssignment(t *testing.T) {
	s, _ := newTestService(t)
	o, _ := s.CreateOutbound("http", "proxyA", "1.2.3.4", 8080, "", "")
	u, err := s.CreateUser("alice", nil, &o.ID)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	views, _ := s.ListUserViews()
	if len(views) != 1 || views[0].OutboundID == nil || *views[0].OutboundID != o.ID {
		t.Fatalf("view outbound not set: %+v", views)
	}
	// reassign to none (direct)
	if _, err := s.UpdateUser(u.ID, "alice", nil, nil); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	views, _ = s.ListUserViews()
	if views[0].OutboundID != nil {
		t.Fatalf("outbound should be cleared, got %v", views[0].OutboundID)
	}
	// assigning a non-existent outbound errors
	bad := uint(9999)
	if _, err := s.UpdateUser(u.ID, "alice", nil, &bad); err != ErrInvalidOutbound {
		t.Fatalf("bad outbound err=%v, want ErrInvalidOutbound", err)
	}
}
```

> The existing `service_test.go` calls `CreateUser`/`UpdateUser` with the old arity in other tests (e.g. `TestCreateUserWithInbounds`, `TestUpdateUserReplacesInbounds`, `TestResetUserCreds`, `TestCreateUserAssignsSubToken`, `TestBackfillUserTokens`, `TestAddAndResetTraffic`, `TestListUserViewsExposesTraffic`). Update each call to pass a trailing `nil` (no outbound). Do the same in `seed.go`/`subscription_test.go` if they call `CreateUser` (they call `CreateInbound`, not `CreateUser`, so likely no change). Likewise the handler test fake in Task 5.

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/inbound/ -run UserOutbound -v`
Expected: FAIL (signature mismatch / `OutboundID` not on view).

- [ ] **Step 3: Implement**

In `backend/internal/inbound/service.go`:

Add `OutboundID` to `UserView`:

```go
type UserView struct {
	ID          uint     `json:"id"`
	Name        string   `json:"name"`
	UUID        string   `json:"uuid"`
	Password    string   `json:"password"`
	SubToken    string   `json:"subToken"`
	UpBytes     int64    `json:"upBytes"`
	DownBytes   int64    `json:"downBytes"`
	OutboundID  *uint    `json:"outboundId"`
	InboundIDs  []uint   `json:"inboundIds"`
	InboundTags []string `json:"inboundTags"`
}
```

Set it in `ListUserViews`' per-user `v`:

```go
		v := UserView{ID: u.ID, Name: u.Name, UUID: u.UUID, Password: u.Password, SubToken: u.SubToken,
			UpBytes: u.UpBytes, DownBytes: u.DownBytes, OutboundID: u.OutboundID, InboundIDs: []uint{}, InboundTags: []string{}}
```

Change `CreateUser`:

```go
func (s *Service) CreateUser(name string, inboundIDs []uint, outboundID *uint) (models.User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return models.User{}, ErrInvalidName
	}
	if err := s.validateOutbound(outboundID); err != nil {
		return models.User{}, err
	}
	u := models.User{Name: name, UUID: genUUID(), Password: genPassword(), SubToken: genToken(), OutboundID: outboundID}
	if err := s.db.Create(&u).Error; err != nil {
		return models.User{}, err
	}
	if err := s.setUserInbounds(&u, inboundIDs); err != nil {
		return models.User{}, err
	}
	return u, s.Regenerate()
}
```

Change `UpdateUser`:

```go
func (s *Service) UpdateUser(id uint, name string, inboundIDs []uint, outboundID *uint) (models.User, error) {
	var u models.User
	if err := s.db.First(&u, id).Error; err != nil {
		return models.User{}, ErrNotFound
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return models.User{}, ErrInvalidName
	}
	if err := s.validateOutbound(outboundID); err != nil {
		return models.User{}, err
	}
	u.Name = name
	// Assign with Select so a nil outbound clears the column.
	if err := s.db.Model(&u).Select("name", "outbound_id").Updates(map[string]any{"name": name, "outbound_id": outboundID}).Error; err != nil {
		return models.User{}, err
	}
	if err := s.setUserInbounds(&u, inboundIDs); err != nil {
		return models.User{}, err
	}
	return u, s.Regenerate()
}
```

> Why `Updates(map…)` instead of `Save`: GORM's `Save`/`Updates(struct)` skips nil-pointer/zero fields, so setting `OutboundID` back to `nil` would be ignored. A `map` update writes `NULL` explicitly.

- [ ] **Step 4: Fix the other CreateUser/UpdateUser call sites in tests**

Update every existing `s.CreateUser(name, ids)` → `s.CreateUser(name, ids, nil)` and `s.UpdateUser(id, name, ids)` → `s.UpdateUser(id, name, ids, nil)` in `service_test.go`.

- [ ] **Step 5: Run to verify it passes**

Run: `cd backend && go test ./internal/inbound/`
Expected: PASS (whole package, including Task 3's outbound tests).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/inbound/service.go backend/internal/inbound/service_test.go
git commit -m "feat: per-user outbound assignment"
```

---

## Task 5: Outbound handlers + user body + routes

**Files:**
- Create: `backend/internal/handlers/outbound.go`
- Modify: `backend/internal/handlers/user.go`, `backend/cmd/server/main.go`
- Test: `backend/internal/handlers/outbound_test.go` *(new)*, `backend/internal/handlers/user_test.go` (fake)

- [ ] **Step 1: Extend the user body + interface + error mapping**

In `backend/internal/handlers/user.go`:

`userBody`:

```go
type userBody struct {
	Name       string `json:"name" binding:"required"`
	InboundIDs []uint `json:"inboundIds"`
	OutboundID *uint  `json:"outboundId"`
}
```

`UserController` (update the two signatures):

```go
	CreateUser(name string, inboundIDs []uint, outboundID *uint) (models.User, error)
	UpdateUser(id uint, name string, inboundIDs []uint, outboundID *uint) (models.User, error)
```

`Create` / `Update` calls:

```go
	u, err := h.ctrl.CreateUser(b.Name, b.InboundIDs, b.OutboundID)
```
```go
	u, err := h.ctrl.UpdateUser(id, b.Name, b.InboundIDs, b.OutboundID)
```

`writeUserErr` — add the invalid-outbound case:

```go
func writeUserErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, inbound.ErrInvalidName), errors.Is(err, inbound.ErrInvalidOutbound):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, inbound.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
```

- [ ] **Step 2: Fix the user_test.go fake**

In `backend/internal/handlers/user_test.go`, update the fake `UserController` implementation's `CreateUser`/`UpdateUser` method signatures to match (add `outboundID *uint`). Keep their existing behavior; just add the param.

- [ ] **Step 3: Write the failing outbound handler test**

Create `backend/internal/handlers/outbound_test.go`:

```go
package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"singbox-admin/internal/inbound"
	"singbox-admin/internal/models"
)

type fakeOutboundCtrl struct {
	created models.Outbound
	err     error
}

func (f *fakeOutboundCtrl) ListOutboundViews() ([]inbound.OutboundView, error) {
	return []inbound.OutboundView{{ID: 1, Tag: "proxyA", Type: "socks5"}}, nil
}
func (f *fakeOutboundCtrl) CreateOutbound(typ, tag, server string, port uint16, username, password string) (models.Outbound, error) {
	if f.err != nil {
		return models.Outbound{}, f.err
	}
	return models.Outbound{ID: 1, Tag: tag, Type: typ, Server: server, Port: port}, nil
}
func (f *fakeOutboundCtrl) UpdateOutbound(id uint, typ, tag, server string, port uint16, username, password string) (models.Outbound, error) {
	return models.Outbound{ID: id, Tag: tag}, f.err
}
func (f *fakeOutboundCtrl) DeleteOutbound(id uint) error { return f.err }

func TestOutboundListAndCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewOutboundHandler(&fakeOutboundCtrl{})
	r.GET("/api/outbounds", h.List)
	r.POST("/api/outbounds", h.Create)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/outbounds", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("list code=%d", w.Code)
	}

	body := `{"type":"socks5","tag":"proxyA","server":"1.2.3.4","port":1080}`
	w = httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/outbounds", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("create code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestOutboundCreateBadType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewOutboundHandler(&fakeOutboundCtrl{err: inbound.ErrInvalidType})
	r.POST("/api/outbounds", h.Create)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/outbounds", bytes.NewBufferString(`{"type":"ftp","tag":"x","server":"1.2.3.4","port":1}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code=%d, want 400", w.Code)
	}
}
```

- [ ] **Step 4: Run to verify it fails**

Run: `cd backend && go test ./internal/handlers/ -run Outbound -v`
Expected: FAIL (`NewOutboundHandler` undefined).

- [ ] **Step 5: Implement the outbound handler**

Create `backend/internal/handlers/outbound.go`:

```go
package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"singbox-admin/internal/inbound"
	"singbox-admin/internal/models"
)

type OutboundController interface {
	ListOutboundViews() ([]inbound.OutboundView, error)
	CreateOutbound(typ, tag, server string, port uint16, username, password string) (models.Outbound, error)
	UpdateOutbound(id uint, typ, tag, server string, port uint16, username, password string) (models.Outbound, error)
	DeleteOutbound(id uint) error
}

type OutboundHandler struct{ ctrl OutboundController }

func NewOutboundHandler(ctrl OutboundController) *OutboundHandler { return &OutboundHandler{ctrl: ctrl} }

type outboundBody struct {
	Type     string `json:"type"`
	Tag      string `json:"tag"`
	Server   string `json:"server"`
	Port     uint16 `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func writeOutboundErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, inbound.ErrInvalidType), errors.Is(err, inbound.ErrInvalidTag), errors.Is(err, inbound.ErrInvalidOutbound):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, inbound.ErrTagExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, inbound.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func (h *OutboundHandler) List(c *gin.Context) {
	views, err := h.ctrl.ListOutboundViews()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, views)
}

func (h *OutboundHandler) Create(c *gin.Context) {
	var b outboundBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	o, err := h.ctrl.CreateOutbound(b.Type, b.Tag, b.Server, b.Port, b.Username, b.Password)
	if err != nil {
		writeOutboundErr(c, err)
		return
	}
	c.JSON(http.StatusOK, o)
}

func (h *OutboundHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	var b outboundBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	o, err := h.ctrl.UpdateOutbound(id, b.Type, b.Tag, b.Server, b.Port, b.Username, b.Password)
	if err != nil {
		writeOutboundErr(c, err)
		return
	}
	c.JSON(http.StatusOK, o)
}

func (h *OutboundHandler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	if err := h.ctrl.DeleteOutbound(id); err != nil {
		writeOutboundErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
```

- [ ] **Step 6: Wire routes in main.go**

In `backend/cmd/server/main.go`, after `userHandler := handlers.NewUserHandler(inbSvc)`:

```go
	outboundHandler := handlers.NewOutboundHandler(inbSvc)
```

Inside the `authed` group (near the users routes):

```go
		authed.GET("/outbounds", outboundHandler.List)
		authed.POST("/outbounds", outboundHandler.Create)
		authed.PUT("/outbounds/:id", outboundHandler.Update)
		authed.DELETE("/outbounds/:id", outboundHandler.Delete)
```

- [ ] **Step 7: Run + build + vet**

Run: `cd backend && go test ./... && go vet ./... && go build ./...`
Expected: all pass.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/handlers/outbound.go backend/internal/handlers/outbound_test.go backend/internal/handlers/user.go backend/internal/handlers/user_test.go backend/cmd/server/main.go
git commit -m "feat: outbound HTTP endpoints and user outboundId body"
```

---

## Task 6: Frontend API client

**Files:**
- Modify: `frontend/lib/api.ts`

- [ ] **Step 1: Add the Outbound type, CRUD, and user outboundId**

In `frontend/lib/api.ts`:

Add `outboundId` to the `User` interface (after `downBytes`):

```ts
  downBytes: number;
  outboundId: number | null;
```

Add near the other types:

```ts
export interface Outbound {
  id: number;
  tag: string;
  type: string;
  server: string;
  port: number;
  username: string;
  password: string;
}

export async function listOutbounds(): Promise<Outbound[]> {
  const res = await request("/api/outbounds");
  if (!res.ok) throw new Error("加载出站失败");
  return res.json();
}

export async function createOutbound(o: Omit<Outbound, "id">): Promise<Outbound> {
  const res = await request("/api/outbounds", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(o),
  });
  if (!res.ok) {
    const b = await res.json().catch(() => ({}));
    throw new Error(b.error || "创建出站失败");
  }
  return res.json();
}

export async function updateOutbound(id: number, o: Omit<Outbound, "id">): Promise<Outbound> {
  const res = await request(`/api/outbounds/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(o),
  });
  if (!res.ok) {
    const b = await res.json().catch(() => ({}));
    throw new Error(b.error || "更新出站失败");
  }
  return res.json();
}

export async function deleteOutbound(id: number): Promise<void> {
  const res = await request(`/api/outbounds/${id}`, { method: "DELETE" });
  if (!res.ok) throw new Error("删除出站失败");
}
```

Update `createUser`/`updateUser` to carry `outboundId`:

```ts
export async function createUser(name: string, inboundIds: number[], outboundId: number | null): Promise<User> {
  const res = await request("/api/users", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, inboundIds, outboundId }),
  });
  if (!res.ok) {
    const b = await res.json().catch(() => ({}));
    throw new Error(b.error || "创建用户失败");
  }
  return res.json();
}

export async function updateUser(id: number, name: string, inboundIds: number[], outboundId: number | null): Promise<User> {
  const res = await request(`/api/users/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, inboundIds, outboundId }),
  });
  if (!res.ok) throw new Error("更新用户失败");
  return res.json();
}
```

- [ ] **Step 2: Typecheck**

Run: `cd frontend && npx tsc --noEmit`
Expected: errors in `app/users/page.tsx` (callers of `createUser`/`updateUser` need the new arg) — fixed in Task 8. The api.ts file itself should be type-clean.

- [ ] **Step 3: Commit**

```bash
git add frontend/lib/api.ts
git commit -m "feat: outbound API client and user outboundId"
```

---

## Task 7: Outbounds page + nav

**Files:**
- Create: `frontend/app/outbounds/page.tsx`
- Modify: `frontend/components/app-shell.tsx`
- Test: `frontend/app/outbounds/page.test.tsx` *(new)*

- [ ] **Step 1: Add the 出站 nav entry**

In `frontend/components/app-shell.tsx`, add to the `概览` group items (after 入站):

```tsx
      { href: "/inbounds", label: "入站" },
      { href: "/outbounds", label: "出站" },
      { href: "/users", label: "用户" },
```

- [ ] **Step 2: Write the failing test**

Create `frontend/app/outbounds/page.test.tsx`:

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import OutboundsPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/outbounds",
}));

const listMock = vi.fn();
const createMock = vi.fn();
const deleteMock = vi.fn();
vi.mock("@/lib/api", () => ({
  listOutbounds: () => listMock(),
  createOutbound: (...a: unknown[]) => createMock(...a),
  updateOutbound: vi.fn(),
  deleteOutbound: (...a: unknown[]) => deleteMock(...a),
  getStatus: vi.fn().mockResolvedValue({ installed: true, version: "1", running: false, hasConfig: true }),
  logout: vi.fn(),
  UnauthorizedError: class extends Error {},
}));

beforeEach(() => {
  listMock.mockReset().mockResolvedValue([]);
  createMock.mockReset().mockResolvedValue({ id: 1 });
  deleteMock.mockReset().mockResolvedValue(undefined);
});

describe("OutboundsPage", () => {
  it("新建出站提交字段", async () => {
    render(<OutboundsPage />);
    await userEvent.click(await screen.findByRole("button", { name: /新建出站/ }));
    await userEvent.type(screen.getByLabelText(/标签/), "proxyA");
    await userEvent.type(screen.getByLabelText(/服务器/), "1.2.3.4");
    await userEvent.clear(screen.getByLabelText(/端口/));
    await userEvent.type(screen.getByLabelText(/端口/), "1080");
    await userEvent.click(screen.getByRole("button", { name: /创建/ }));
    await waitFor(() => expect(createMock).toHaveBeenCalled());
    const arg = createMock.mock.calls[0][0];
    expect(arg.tag).toBe("proxyA");
    expect(arg.server).toBe("1.2.3.4");
    expect(arg.port).toBe(1080);
  });

  it("删除出站需二次确认", async () => {
    listMock.mockResolvedValue([{ id: 5, tag: "proxyA", type: "socks5", server: "1.2.3.4", port: 1080, username: "", password: "" }]);
    render(<OutboundsPage />);
    await waitFor(() => expect(screen.getByText("proxyA")).toBeInTheDocument());
    await userEvent.click(screen.getByRole("button", { name: /^删除$/ }));
    expect(deleteMock).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: /确认删除/ }));
    await waitFor(() => expect(deleteMock).toHaveBeenCalledWith(5));
  });
});
```

- [ ] **Step 3: Run to verify it fails**

Run: `cd frontend && npm test -- app/outbounds`
Expected: FAIL (page doesn't exist).

- [ ] **Step 4: Create the outbounds page**

Create `frontend/app/outbounds/page.tsx`:

```tsx
"use client";

import { useCallback, useEffect, useState } from "react";
import { listOutbounds, createOutbound, updateOutbound, deleteOutbound, Outbound } from "@/lib/api";
import { AppShell } from "@/components/app-shell";
import { Modal } from "@/components/modal";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card } from "@/components/ui/card";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from "@/components/ui/select";
import { Pencil, Trash2 } from "lucide-react";

type Form = { id?: number; type: string; tag: string; server: string; port: string; username: string; password: string };
const empty: Form = { type: "socks5", tag: "", server: "", port: "1080", username: "", password: "" };

export default function OutboundsPage() {
  const [outbounds, setOutbounds] = useState<Outbound[]>([]);
  const [form, setForm] = useState<Form | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<Outbound | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(() => {
    listOutbounds().then(setOutbounds).catch(() => setError("加载出站失败"));
  }, []);
  useEffect(() => { refresh(); }, [refresh]);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!form) return;
    setBusy(true);
    setError("");
    const payload = {
      type: form.type, tag: form.tag, server: form.server,
      port: Number(form.port), username: form.username, password: form.password,
    };
    try {
      if (form.id) await updateOutbound(form.id, payload);
      else await createOutbound(payload);
      setForm(null);
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存失败");
    } finally {
      setBusy(false);
    }
  }

  async function doDelete() {
    if (!confirmDelete) return;
    await deleteOutbound(confirmDelete.id);
    setConfirmDelete(null);
    refresh();
  }

  return (
    <AppShell>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-normal tracking-tight">出站</h1>
        <Button className="rounded-full" onClick={() => { setError(""); setForm({ ...empty }); }}>新建出站</Button>
      </div>

      <Card className="overflow-hidden rounded-lg p-0">
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <TableHead>标签</TableHead>
              <TableHead>类型</TableHead>
              <TableHead>服务器</TableHead>
              <TableHead>端口</TableHead>
              <TableHead>用户名</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {outbounds.map((o) => (
              <TableRow key={o.id}>
                <TableCell className="font-medium">{o.tag}</TableCell>
                <TableCell className="text-xs text-muted-foreground">{o.type}</TableCell>
                <TableCell className="font-mono text-xs text-muted-foreground">{o.server}</TableCell>
                <TableCell className="font-mono text-xs text-muted-foreground">{o.port}</TableCell>
                <TableCell className="text-xs text-muted-foreground">{o.username || "—"}</TableCell>
                <TableCell>
                  <div className="flex items-center justify-end gap-0.5">
                    <Button variant="ghost" size="icon" aria-label="编辑" onClick={() => { setError(""); setForm({ id: o.id, type: o.type, tag: o.tag, server: o.server, port: String(o.port), username: o.username, password: o.password }); }}>
                      <Pencil />
                    </Button>
                    <Button variant="ghost" size="icon" aria-label="删除" className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive" onClick={() => setConfirmDelete(o)}>
                      <Trash2 />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
            {outbounds.length === 0 && (
              <TableRow className="hover:bg-transparent">
                <TableCell colSpan={6} className="py-8 text-center text-sm text-muted-foreground">暂无出站。</TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>

      <Modal open={form !== null} onClose={() => setForm(null)} title={form?.id ? "编辑出站" : "新建出站"}>
        {form && (
          <form onSubmit={submit} className="space-y-4">
            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground">类型</Label>
              <Select value={form.type} onValueChange={(v) => v && setForm({ ...form, type: v })}>
                <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="socks5">SOCKS5</SelectItem>
                  <SelectItem value="http">HTTP</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex gap-4">
              <div className="flex-1 space-y-2">
                <Label htmlFor="tag" className="text-xs text-muted-foreground">标签</Label>
                <Input id="tag" value={form.tag} onChange={(e) => setForm({ ...form, tag: e.target.value })} />
              </div>
              <div className="w-28 space-y-2">
                <Label htmlFor="port" className="text-xs text-muted-foreground">端口</Label>
                <Input id="port" value={form.port} onChange={(e) => setForm({ ...form, port: e.target.value })} />
              </div>
            </div>
            <div className="space-y-2">
              <Label htmlFor="server" className="text-xs text-muted-foreground">服务器</Label>
              <Input id="server" value={form.server} onChange={(e) => setForm({ ...form, server: e.target.value })} />
            </div>
            <div className="flex gap-4">
              <div className="flex-1 space-y-2">
                <Label htmlFor="username" className="text-xs text-muted-foreground">用户名（可选）</Label>
                <Input id="username" value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} />
              </div>
              <div className="flex-1 space-y-2">
                <Label htmlFor="password" className="text-xs text-muted-foreground">密码（可选）</Label>
                <Input id="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} />
              </div>
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
            <div className="flex justify-end gap-3 pt-2">
              <Button type="button" variant="outline" className="rounded-full" onClick={() => setForm(null)}>取消</Button>
              <Button type="submit" className="rounded-full" disabled={busy}>{form.id ? "保存" : "创建"}</Button>
            </div>
          </form>
        )}
      </Modal>

      <Modal open={confirmDelete !== null} onClose={() => setConfirmDelete(null)} title="删除出站">
        <p className="mb-4 text-sm text-muted-foreground">
          确定删除出站 <span className="font-mono">{confirmDelete?.tag}</span>？引用它的用户将回退为直连。
        </p>
        <div className="flex justify-end gap-3">
          <Button variant="outline" className="rounded-full" onClick={() => setConfirmDelete(null)}>取消</Button>
          <Button className="rounded-full" onClick={doDelete}>确认删除</Button>
        </div>
      </Modal>
    </AppShell>
  );
}
```

- [ ] **Step 5: Run to verify it passes**

Run: `cd frontend && npm test -- app/outbounds`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/app/outbounds/page.tsx frontend/app/outbounds/page.test.tsx frontend/components/app-shell.tsx
git commit -m "feat: outbounds management page and nav entry"
```

---

## Task 8: Outbound selector in the user modal

**Files:**
- Modify: `frontend/app/users/page.tsx`
- Test: `frontend/app/users/page.test.tsx`

- [ ] **Step 1: Update the failing test**

In `frontend/app/users/page.test.tsx`:

Add to the `@/lib/api` mock:

```ts
  listOutbounds: vi.fn().mockResolvedValue([{ id: 5, tag: "proxyA", type: "socks5", server: "1.2.3.4", port: 1080, username: "", password: "" }]),
```

Update the "新建用户" test's create assertion to expect the third arg (outboundId, default null when not chosen):

```tsx
    expect(createUserMock.mock.calls[0][1]).toEqual([1]);
    expect(createUserMock.mock.calls[0][2]).toBeNull();
```

(Existing user objects in the mock data already include the required fields; add `outboundId: null` to each so they satisfy the type — e.g. the list-display, delete, 订阅, and traffic test user literals.)

- [ ] **Step 2: Run to verify it fails**

Run: `cd frontend && npm test -- app/users`
Expected: FAIL (createUser called with 2 args / outboundId undefined).

- [ ] **Step 3: Wire the outbound select into the user modal**

In `frontend/app/users/page.tsx`:

- Imports: add `listOutbounds, Outbound` to the `@/lib/api` import and the Select primitives:
  ```tsx
  import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from "@/components/ui/select";
  ```
- Form type + state: extend `Form` and load outbounds:
  ```tsx
  type Form = { id?: number; name: string; inboundIds: number[]; outboundId: number | null };
  ```
  ```tsx
  const [outbounds, setOutbounds] = useState<Outbound[]>([]);
  ```
  In the existing `useEffect`, also load them:
  ```tsx
    listOutbounds().then(setOutbounds).catch(() => {});
  ```
- Update the "新建用户" button and edit button to set `outboundId`:
  - create: `setForm({ name: "", inboundIds: [], outboundId: null })`
  - edit: `setForm({ id: u.id, name: u.name, inboundIds: u.inboundIds, outboundId: u.outboundId })`
- `submit`: pass `form.outboundId`:
  ```tsx
      if (form.id) await updateUser(form.id, form.name, form.inboundIds, form.outboundId);
      else await createUser(form.name, form.inboundIds, form.outboundId);
  ```
- Add the select inside the form (after the 所属入站 block). The Select value is a string; `""` means 直连:
  ```tsx
  <div className="space-y-2">
    <Label className="text-xs text-muted-foreground">出站</Label>
    <Select
      value={form.outboundId == null ? "" : String(form.outboundId)}
      onValueChange={(v) => setForm({ ...form, outboundId: v ? Number(v) : null })}
    >
      <SelectTrigger className="w-full"><SelectValue placeholder="直连" /></SelectTrigger>
      <SelectContent>
        <SelectItem value="">直连</SelectItem>
        {outbounds.map((o) => (
          <SelectItem key={o.id} value={String(o.id)}>{o.tag} · {o.type}</SelectItem>
        ))}
      </SelectContent>
    </Select>
  </div>
  ```

> base-ui `Select` `onValueChange` provides `string | null`; the empty-string item maps to `null` (直连). If base-ui rejects an empty-string `value`, use the sentinel `"direct"` instead and map `v === "direct" ? null : Number(v)` plus `value={form.outboundId == null ? "direct" : String(form.outboundId)}`.

- [ ] **Step 4: Run to verify it passes**

Run: `cd frontend && npm test -- app/users`
Expected: PASS.

- [ ] **Step 5: Full frontend suite**

Run: `cd frontend && npm test`
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add frontend/app/users/page.tsx frontend/app/users/page.test.tsx
git commit -m "feat: assign an outbound when creating/editing a user"
```

---

## Task 9: Full build + verification

**Files:** none (verification only)

- [ ] **Step 1: Backend**

Run: `cd backend && go vet ./... && go test ./...`
Expected: all pass, vet clean.

- [ ] **Step 2: Frontend**

Run: `cd frontend && npm test`
Expected: all pass.

- [ ] **Step 3: Build the single binary**

Run: `cd /Users/silence/Documents/sing-box-admin && make build`
Expected: builds `backend/bin/sing-box-admin`.

- [ ] **Step 4: Restore dist artifacts dirtied by the build**

Run: `git checkout -- backend/internal/web/dist`
Expected: working tree clean apart from intended changes.

- [ ] **Step 5: Run + eyeball, and validate the generated config**

Run: `JWT_SECRET=dev DB_PATH=/tmp/m8.db ./backend/bin/sing-box-admin` then open http://localhost:8080.
Verify: 出站 page CRUD works; user modal has an 出站 select (直连 + outbounds). Create an outbound, assign it to a user, then read the generated config:
```bash
curl -s -c /tmp/m8c.txt -X POST localhost:8080/api/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"mnice7082"}' -o /dev/null
curl -s -b /tmp/m8c.txt localhost:8080/api/singbox/config | python3 -m json.tool | grep -A12 '"route"\|"dns"\|"outbounds"'
```
Confirm the generated JSON has: the outbound (socks5→`socks`), a `route.rules` entry `{auth_user:["u<id>"], outbound:<tag>}` with `final:"direct"`, and a `dns` block with `dns-<tag>` (detour) + rule + `final:"local"`. Stop the binary and `rm /tmp/m8.db /tmp/m8c.txt` when done.

> **sing-box field check:** on the VPS, after 应用并重启, run `sing-box check -c <config.json>` (or check the service logs) to confirm `auth_user` is accepted by the bundled sing-box version. If that version rejects `auth_user` in favor of `user`, change the two `"auth_user"` keys in `generate.go` to `"user"` and re-run `TestGeneratePicksCredByProtocol` (update the assertion key too).

- [ ] **Step 6: Commit any verification fixes**

```bash
git add -A
git commit -m "chore: M8 verification fixes"
```

---

## Self-review notes

- **Spec coverage:** Outbound model + `User.OutboundID` → Task 1; outbounds/route/dns generation → Task 2; outbound CRUD + delete-nulls-users → Task 3; per-user assignment → Task 4; HTTP endpoints + user body → Task 5; API client → Task 6; 出站 page + nav → Task 7; user-modal select → Task 8; verification incl. DNS + `auth_user` check → Task 9.
- **Signature consistency:** `Generate(inbounds, outbounds, exp)` is used identically in Task 2 (def) and Task 3 (`Regenerate`). `CreateOutbound/UpdateOutbound(typ, tag, server string, port uint16, username, password string)` matches across service (Task 3), the controller interface (Task 5), and the handler test fake (Task 5). `CreateUser/UpdateUser(..., outboundID *uint)` matches across service (Task 4), interface + body (Task 5), and the api client (Task 6). `OutboundView{ID,Tag,Type,Server,Port,Username,Password}` matches the TS `Outbound` (Task 6).
- **Cross-task ordering caveat:** Tasks 3 and 4 are interdependent (the outbound test uses the new `CreateUser` arity; `Regenerate` calls the new `Generate`). Implement Task 2 → Task 3 → Task 4 in sequence and only run the full `internal/inbound` package test after Task 4. Flagged inline.
- **Out of scope (unchanged):** domain/geo split-routing, http-over-TLS, outbound health-checks, outbound chaining.
```

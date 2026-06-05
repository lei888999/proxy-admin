# M3: 入站/用户管理（VLESS+Reality）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 面板可视化管理 VLESS+Reality 入站与用户，从 DB 自动生成 sing-box 配置并可应用重启；前端加「入站」页，配置页改只读。

**Architecture:** 新增 `internal/inbound` 包（KeyGen 纯 Go 生成 reality 密钥/uuid/short_id；Generate 纯函数构建配置；Service 做 CRUD + 自动重新生成写入 M2 的 ConfigStore）。新增 `internal/models` 的 Inbound/User。singbox.Service 加 Restart()。前端在深色 B 端 shell 下加入站页，全中文。

**Tech Stack:** Go 1.26 + Gin + GORM + glebarez/sqlite + golang.org/x/crypto/curve25519；Next.js 16 + TS + Tailwind + Vitest。

参考规范：`docs/superpowers/specs/2026-06-05-inbound-user-management-design.md`

---

## File Structure

后端：
- `backend/internal/models/inbound.go`（新）— Inbound, User
- `backend/internal/database/database.go`（改）— AutoMigrate 加 Inbound/User
- `backend/internal/inbound/keygen.go`（新）— KeyGen + goKeyGen
- `backend/internal/inbound/generate.go`（新）— Generate 纯函数
- `backend/internal/inbound/service.go`（新）— Service（CRUD + Regenerate）
- `backend/internal/singbox/service.go`（改）— 加 Restart()
- `backend/internal/handlers/inbound.go`（新）— handlers + 接口
- `backend/cmd/server/main.go`（改）— 接线路由

前端：
- `frontend/lib/api.ts`（改）— inbound/user/apply 方法 + 类型
- `frontend/components/app-shell.tsx`（改）+ `app-shell.test.tsx`（改）— 导航加「入站」
- `frontend/app/config/page.tsx`（改）+ `config/page.test.tsx`（改）— 只读
- `frontend/app/dashboard/page.tsx`（改）+ `dashboard/page.test.tsx`（改）— 应用按钮
- `frontend/app/inbounds/page.tsx`（新）+ `inbounds/page.test.tsx`（新）

---

## Task 1: 数据模型 Inbound/User + AutoMigrate

**Files:**
- Create: `backend/internal/models/inbound.go`
- Modify: `backend/internal/database/database.go`
- Test: `backend/internal/database/inbound_migrate_test.go`

- [ ] **Step 1: 写模型**

Create `backend/internal/models/inbound.go`:
```go
package models

import "time"

type Inbound struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	Tag               string    `gorm:"uniqueIndex;not null" json:"tag"`
	Port              uint16    `gorm:"not null" json:"port"`
	Flow              string    `gorm:"not null" json:"flow"`
	RealityPrivateKey string    `gorm:"not null" json:"-"` // never exposed to the frontend
	RealityPublicKey  string    `gorm:"not null" json:"realityPublicKey"`
	RealityShortID    string    `gorm:"not null" json:"realityShortId"`
	Handshake         string    `gorm:"not null" json:"handshake"`
	HandshakePort     uint16    `gorm:"not null" json:"handshakePort"`
	ServerName        string    `gorm:"not null" json:"serverName"`
	Users             []User    `gorm:"constraint:OnDelete:CASCADE" json:"users"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	InboundID uint      `gorm:"index;not null" json:"inboundId"`
	Name      string    `gorm:"not null" json:"name"`
	UUID      string    `gorm:"not null" json:"uuid"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
```
> JSON tag 用 camelCase（与现有 `Status` 一致）；`RealityPrivateKey` 标 `json:"-"`，私钥永不下发前端。`Generate`/`Service` 读的是 Go 字段名（不受 tag 影响）。

- [ ] **Step 2: 写失败测试**

Create `backend/internal/database/inbound_migrate_test.go`:
```go
package database

import (
	"testing"

	"singbox-admin/internal/models"
)

func TestInitMigratesInboundAndUser(t *testing.T) {
	db, err := Init(":memory:", "admin", "mnice7082")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if !db.Migrator().HasTable(&models.Inbound{}) {
		t.Fatal("inbounds table missing")
	}
	if !db.Migrator().HasTable(&models.User{}) {
		t.Fatal("users table missing")
	}
}
```

- [ ] **Step 3: 运行确认失败**

Run: `cd backend && go test ./internal/database/ -run TestInitMigratesInboundAndUser`
Expected: FAIL（users/inbounds 表不存在）

- [ ] **Step 4: 改 AutoMigrate**

In `backend/internal/database/database.go`, change the AutoMigrate line:
```go
	if err := db.AutoMigrate(&models.Admin{}, &models.Inbound{}, &models.User{}); err != nil {
		return nil, err
	}
```

- [ ] **Step 5: 运行确认通过**

Run: `cd backend && go test ./internal/database/`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add backend/internal/models/inbound.go backend/internal/database
git commit -m "feat(models): Inbound/User models + AutoMigrate"
```

---

## Task 2: KeyGen（reality 密钥 / uuid / short_id）

**Files:**
- Create: `backend/internal/inbound/keygen.go`
- Test: `backend/internal/inbound/keygen_test.go`

- [ ] **Step 1: 写失败测试**

Create `backend/internal/inbound/keygen_test.go`:
```go
package inbound

import (
	"encoding/base64"
	"encoding/hex"
	"regexp"
	"testing"

	"golang.org/x/crypto/curve25519"
)

func TestRealityKeypairDerivesPublicFromPrivate(t *testing.T) {
	priv, pub, err := NewKeyGen().RealityKeypair()
	if err != nil {
		t.Fatalf("RealityKeypair: %v", err)
	}
	pb, err := base64.RawURLEncoding.DecodeString(priv)
	if err != nil || len(pb) != 32 {
		t.Fatalf("priv decode err=%v len=%d", err, len(pb))
	}
	pubBytes, err := base64.RawURLEncoding.DecodeString(pub)
	if err != nil || len(pubBytes) != 32 {
		t.Fatalf("pub decode err=%v len=%d", err, len(pubBytes))
	}
	want, _ := curve25519.X25519(pb, curve25519.Basepoint)
	if base64.RawURLEncoding.EncodeToString(want) != pub {
		t.Fatal("public key is not X25519(private, basepoint)")
	}
}

func TestUUIDv4Format(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if got := NewKeyGen().UUID(); !re.MatchString(got) {
		t.Fatalf("uuid %q not v4", got)
	}
}

func TestShortIDIs8Hex(t *testing.T) {
	s := NewKeyGen().ShortID()
	if len(s) != 8 {
		t.Fatalf("len %d, want 8", len(s))
	}
	if _, err := hex.DecodeString(s); err != nil {
		t.Fatalf("not hex: %v", err)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/inbound/`
Expected: FAIL（NewKeyGen 未定义）

- [ ] **Step 3: 实现**

Create `backend/internal/inbound/keygen.go`:
```go
package inbound

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

// KeyGen produces the secrets needed for a VLESS+Reality inbound.
type KeyGen interface {
	RealityKeypair() (privateKey, publicKey string, err error)
	UUID() string
	ShortID() string
}

type goKeyGen struct{}

func NewKeyGen() KeyGen { return goKeyGen{} }

func (goKeyGen) RealityKeypair() (string, string, error) {
	priv := make([]byte, 32)
	if _, err := rand.Read(priv); err != nil {
		return "", "", err
	}
	// X25519 clamping (matches `sing-box generate reality-keypair`).
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64
	pub, err := curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		return "", "", err
	}
	enc := base64.RawURLEncoding
	return enc.EncodeToString(priv), enc.EncodeToString(pub), nil
}

func (goKeyGen) UUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func (goKeyGen) ShortID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd backend && go test ./internal/inbound/`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/inbound/keygen.go backend/internal/inbound/keygen_test.go
git commit -m "feat(inbound): Go reality keypair/uuid/short_id generation"
```

---

## Task 3: 配置生成器 Generate

**Files:**
- Create: `backend/internal/inbound/generate.go`
- Test: `backend/internal/inbound/generate_test.go`

- [ ] **Step 1: 写失败测试**

Create `backend/internal/inbound/generate_test.go`:
```go
package inbound

import (
	"encoding/json"
	"testing"

	"singbox-admin/internal/models"
)

func TestGenerateVlessReality(t *testing.T) {
	in := models.Inbound{
		Tag: "vless-in", Port: 443, Flow: "xtls-rprx-vision",
		RealityPrivateKey: "PRIV", RealityShortID: "deadbeef",
		Handshake: "www.microsoft.com", HandshakePort: 443, ServerName: "www.microsoft.com",
		Users: []models.User{{Name: "alice", UUID: "uuid-1"}},
	}
	out, err := Generate([]models.Inbound{in})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out)
	}
	ins := cfg["inbounds"].([]any)
	if len(ins) != 1 {
		t.Fatalf("inbounds len %d", len(ins))
	}
	ib := ins[0].(map[string]any)
	if ib["type"] != "vless" || ib["listen_port"].(float64) != 443 {
		t.Fatalf("inbound = %v", ib)
	}
	users := ib["users"].([]any)
	u := users[0].(map[string]any)
	if u["uuid"] != "uuid-1" || u["flow"] != "xtls-rprx-vision" {
		t.Fatalf("user = %v", u)
	}
	reality := ib["tls"].(map[string]any)["reality"].(map[string]any)
	if reality["private_key"] != "PRIV" {
		t.Fatalf("reality = %v", reality)
	}
	if reality["short_id"].([]any)[0] != "deadbeef" {
		t.Fatalf("short_id = %v", reality["short_id"])
	}
}

func TestGenerateEmptyInboundsIsValid(t *testing.T) {
	out, err := Generate(nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if _, ok := cfg["outbounds"]; !ok {
		t.Fatal("missing outbounds scaffold")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/inbound/ -run TestGenerate`
Expected: FAIL（Generate 未定义）

- [ ] **Step 3: 实现**

Create `backend/internal/inbound/generate.go`:
```go
package inbound

import (
	"encoding/json"

	"singbox-admin/internal/models"
)

// Generate builds a complete sing-box config from the given inbounds.
func Generate(inbounds []models.Inbound) (string, error) {
	type rUser struct {
		Name string `json:"name"`
		UUID string `json:"uuid"`
		Flow string `json:"flow"`
	}
	type handshake struct {
		Server     string `json:"server"`
		ServerPort uint16 `json:"server_port"`
	}
	type reality struct {
		Enabled    bool      `json:"enabled"`
		Handshake  handshake `json:"handshake"`
		PrivateKey string    `json:"private_key"`
		ShortID    []string  `json:"short_id"`
	}
	type tlsCfg struct {
		Enabled    bool    `json:"enabled"`
		ServerName string  `json:"server_name"`
		Reality    reality `json:"reality"`
	}
	type vlessIn struct {
		Type       string  `json:"type"`
		Tag        string  `json:"tag"`
		Listen     string  `json:"listen"`
		ListenPort uint16  `json:"listen_port"`
		Users      []rUser `json:"users"`
		TLS        tlsCfg  `json:"tls"`
	}
	type outbound struct {
		Type string `json:"type"`
		Tag  string `json:"tag"`
	}
	type logCfg struct {
		Level string `json:"level"`
	}
	type cfg struct {
		Log       logCfg     `json:"log"`
		Inbounds  []vlessIn  `json:"inbounds"`
		Outbounds []outbound `json:"outbounds"`
	}

	c := cfg{
		Log:       logCfg{Level: "info"},
		Inbounds:  []vlessIn{},
		Outbounds: []outbound{{Type: "direct", Tag: "direct"}},
	}
	for _, in := range inbounds {
		users := []rUser{}
		for _, u := range in.Users {
			users = append(users, rUser{Name: u.Name, UUID: u.UUID, Flow: in.Flow})
		}
		c.Inbounds = append(c.Inbounds, vlessIn{
			Type: "vless", Tag: in.Tag, Listen: "::", ListenPort: in.Port,
			Users: users,
			TLS: tlsCfg{
				Enabled:    true,
				ServerName: in.ServerName,
				Reality: reality{
					Enabled:    true,
					Handshake:  handshake{Server: in.Handshake, ServerPort: in.HandshakePort},
					PrivateKey: in.RealityPrivateKey,
					ShortID:    []string{in.RealityShortID},
				},
			},
		})
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd backend && go test ./internal/inbound/`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/inbound/generate.go backend/internal/inbound/generate_test.go
git commit -m "feat(inbound): config generator for VLESS+Reality"
```

---

## Task 4: inbound.Service（CRUD + Regenerate）

**Files:**
- Create: `backend/internal/inbound/service.go`
- Test: `backend/internal/inbound/service_test.go`

- [ ] **Step 1: 写失败测试**

Create `backend/internal/inbound/service_test.go`:
```go
package inbound

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"singbox-admin/internal/models"
)

type fakeWriter struct {
	last  string
	calls int
}

func (f *fakeWriter) SaveConfig(c string) error { f.last = c; f.calls++; return nil }

type fakeKeyGen struct{}

func (fakeKeyGen) RealityKeypair() (string, string, error) { return "PRIV", "PUB", nil }
func (fakeKeyGen) UUID() string                            { return "uuid-fixed" }
func (fakeKeyGen) ShortID() string                         { return "deadbeef" }

func newTestService(t *testing.T) (*Service, *fakeWriter) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := db.AutoMigrate(&models.Inbound{}, &models.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	w := &fakeWriter{}
	return NewService(db, w, fakeKeyGen{}), w
}

func TestCreateInboundGeneratesKeysAndWritesConfig(t *testing.T) {
	s, w := newTestService(t)
	in, err := s.CreateInbound("vless-in", 443, "www.microsoft.com")
	if err != nil {
		t.Fatalf("CreateInbound: %v", err)
	}
	if in.RealityPublicKey != "PUB" || in.RealityShortID != "deadbeef" || in.ServerName != "www.microsoft.com" {
		t.Fatalf("inbound = %+v", in)
	}
	if w.calls == 0 || !strings.Contains(w.last, "vless-in") {
		t.Fatalf("config not regenerated: %q", w.last)
	}
}

func TestCreateInboundDuplicateTag(t *testing.T) {
	s, _ := newTestService(t)
	if _, err := s.CreateInbound("t", 1, "h"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateInbound("t", 2, "h"); err != ErrTagExists {
		t.Fatalf("err = %v, want ErrTagExists", err)
	}
}

func TestCreateUserAndCascadeDelete(t *testing.T) {
	s, w := newTestService(t)
	in, _ := s.CreateInbound("t", 1, "h")
	u, err := s.CreateUser(in.ID, "alice")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.UUID != "uuid-fixed" || !strings.Contains(w.last, "uuid-fixed") {
		t.Fatalf("user/config wrong: %+v / %q", u, w.last)
	}
	if err := s.DeleteInbound(in.ID); err != nil {
		t.Fatalf("DeleteInbound: %v", err)
	}
	us, _ := s.ListUsers(in.ID)
	if len(us) != 0 {
		t.Fatalf("users not cascade-deleted: %d", len(us))
	}
}

func TestCreateUserInboundNotFound(t *testing.T) {
	s, _ := newTestService(t)
	if _, err := s.CreateUser(999, "x"); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestDeleteInboundNotFound(t *testing.T) {
	s, _ := newTestService(t)
	if err := s.DeleteInbound(123); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/inbound/ -run 'TestCreate|TestDelete'`
Expected: FAIL（NewService 未定义）

- [ ] **Step 3: 实现**

Create `backend/internal/inbound/service.go`:
```go
package inbound

import (
	"errors"

	"gorm.io/gorm"

	"singbox-admin/internal/models"
)

var (
	ErrTagExists = errors.New("tag exists")
	ErrNotFound  = errors.New("not found")
)

// ConfigWriter is satisfied by *singbox.Service (SaveConfig).
type ConfigWriter interface {
	SaveConfig(content string) error
}

type Service struct {
	db     *gorm.DB
	writer ConfigWriter
	keygen KeyGen
}

func NewService(db *gorm.DB, writer ConfigWriter, keygen KeyGen) *Service {
	return &Service{db: db, writer: writer, keygen: keygen}
}

func (s *Service) ListInbounds() ([]models.Inbound, error) {
	var ins []models.Inbound
	err := s.db.Preload("Users").Order("id").Find(&ins).Error
	return ins, err
}

func (s *Service) CreateInbound(tag string, port uint16, handshake string) (models.Inbound, error) {
	var count int64
	s.db.Model(&models.Inbound{}).Where("tag = ?", tag).Count(&count)
	if count > 0 {
		return models.Inbound{}, ErrTagExists
	}
	priv, pub, err := s.keygen.RealityKeypair()
	if err != nil {
		return models.Inbound{}, err
	}
	in := models.Inbound{
		Tag: tag, Port: port, Flow: "xtls-rprx-vision",
		RealityPrivateKey: priv, RealityPublicKey: pub, RealityShortID: s.keygen.ShortID(),
		Handshake: handshake, HandshakePort: 443, ServerName: handshake,
	}
	if err := s.db.Create(&in).Error; err != nil {
		return models.Inbound{}, err
	}
	return in, s.Regenerate()
}

func (s *Service) DeleteInbound(id uint) error {
	res := s.db.Select("Users").Delete(&models.Inbound{ID: id})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return s.Regenerate()
}

func (s *Service) ListUsers(inboundID uint) ([]models.User, error) {
	var us []models.User
	err := s.db.Where("inbound_id = ?", inboundID).Order("id").Find(&us).Error
	return us, err
}

func (s *Service) CreateUser(inboundID uint, name string) (models.User, error) {
	var in models.Inbound
	if err := s.db.First(&in, inboundID).Error; err != nil {
		return models.User{}, ErrNotFound
	}
	u := models.User{InboundID: inboundID, Name: name, UUID: s.keygen.UUID()}
	if err := s.db.Create(&u).Error; err != nil {
		return models.User{}, err
	}
	return u, s.Regenerate()
}

func (s *Service) DeleteUser(id uint) error {
	res := s.db.Delete(&models.User{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return s.Regenerate()
}

// Regenerate rebuilds config.json from the current inbounds/users.
func (s *Service) Regenerate() error {
	ins, err := s.ListInbounds()
	if err != nil {
		return err
	}
	content, err := Generate(ins)
	if err != nil {
		return err
	}
	return s.writer.SaveConfig(content)
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd backend && go test ./internal/inbound/`
Expected: PASS（keygen + generate + service 全部）

- [ ] **Step 5: 提交**

```bash
git add backend/internal/inbound/service.go backend/internal/inbound/service_test.go
git commit -m "feat(inbound): Service CRUD with auto config regeneration"
```

---

## Task 5: singbox.Service.Restart()

**Files:**
- Modify: `backend/internal/singbox/service.go`
- Test: `backend/internal/singbox/service_test.go`

- [ ] **Step 1: 追加失败测试**（在 `service_test.go` 末尾）

```go
func TestRestartStartsWhenStopped(t *testing.T) {
	env := newFakeEnv()
	env.binPath = "/usr/bin/sing-box"
	env.spawnPid = 11
	env.alive[11] = true
	svc := newService(t, env)
	_ = svc.SaveConfig(`{"log":{}}`)
	st, err := svc.Restart()
	if err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if !st.Running {
		t.Fatalf("not running after restart: %+v", st)
	}
}

func TestRestartNotInstalled(t *testing.T) {
	svc := newService(t, newFakeEnv())
	if _, err := svc.Restart(); err != ErrNotInstalled {
		t.Fatalf("err = %v, want ErrNotInstalled", err)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/singbox/ -run TestRestart`
Expected: FAIL（Restart 未定义）

- [ ] **Step 3: 实现** — 在 `service.go` 的 `Stop` 方法之后加：

```go
// Restart stops sing-box (ignoring "not running") and starts it again so a
// regenerated config takes effect.
func (s *Service) Restart() (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.pm.Stop(); err != nil && err != ErrNotRunning {
		return s.statusLocked(), err
	}
	bin := s.resolveBin()
	if bin == "" {
		return s.statusLocked(), ErrNotInstalled
	}
	if !s.store.Exists() {
		return s.statusLocked(), ErrNoConfig
	}
	if err := s.pm.Start(bin, s.store.Path()); err != nil {
		return s.statusLocked(), err
	}
	return s.statusLocked(), nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd backend && go test ./internal/singbox/`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/singbox/service.go backend/internal/singbox/service_test.go
git commit -m "feat(singbox): Service.Restart for applying regenerated config"
```

---

## Task 6: HTTP handlers（inbound/user/apply）

**Files:**
- Create: `backend/internal/handlers/inbound.go`
- Test: `backend/internal/handlers/inbound_test.go`

> 错误映射复用同包 `singbox.go` 的 `writeStartError`（apply 重启错误）。

- [ ] **Step 1: 写失败测试**

Create `backend/internal/handlers/inbound_test.go`:
```go
package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"singbox-admin/internal/inbound"
	"singbox-admin/internal/models"
	"singbox-admin/internal/singbox"
)

type fakeInbCtrl struct {
	inbounds    []models.Inbound
	createErr   error
	deleteErr   error
	users       []models.User
	createUErr  error
	deleteUErr  error
	regenErr    error
	lastTag     string
	lastUName   string
}

func (f *fakeInbCtrl) ListInbounds() ([]models.Inbound, error) { return f.inbounds, nil }
func (f *fakeInbCtrl) CreateInbound(tag string, port uint16, hs string) (models.Inbound, error) {
	f.lastTag = tag
	return models.Inbound{ID: 1, Tag: tag, Port: port}, f.createErr
}
func (f *fakeInbCtrl) DeleteInbound(id uint) error                 { return f.deleteErr }
func (f *fakeInbCtrl) ListUsers(id uint) ([]models.User, error)    { return f.users, nil }
func (f *fakeInbCtrl) CreateUser(id uint, name string) (models.User, error) {
	f.lastUName = name
	return models.User{ID: 1, Name: name, UUID: "u"}, f.createUErr
}
func (f *fakeInbCtrl) DeleteUser(id uint) error { return f.deleteUErr }
func (f *fakeInbCtrl) Regenerate() error        { return f.regenErr }

type fakeRestarter struct {
	status singbox.Status
	err    error
}

func (f *fakeRestarter) Restart() (singbox.Status, error) { return f.status, f.err }

func inbRouter(ctrl InboundController, r Restarter) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewInboundHandler(ctrl, r)
	e := gin.New()
	e.GET("/api/inbounds", h.ListInbounds)
	e.POST("/api/inbounds", h.CreateInbound)
	e.DELETE("/api/inbounds/:id", h.DeleteInbound)
	e.GET("/api/inbounds/:id/users", h.ListUsers)
	e.POST("/api/inbounds/:id/users", h.CreateUser)
	e.DELETE("/api/users/:id", h.DeleteUser)
	e.POST("/api/singbox/apply", h.Apply)
	return e
}

func req(e *gin.Engine, m, p, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(m, p, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	e.ServeHTTP(w, r)
	return w
}

func TestCreateInboundOK(t *testing.T) {
	c := &fakeInbCtrl{}
	w := req(inbRouter(c, &fakeRestarter{}), http.MethodPost, "/api/inbounds", `{"tag":"v1","port":443,"handshake":"www.microsoft.com"}`)
	if w.Code != 200 || c.lastTag != "v1" {
		t.Fatalf("code=%d tag=%q body=%s", w.Code, c.lastTag, w.Body.String())
	}
}

func TestCreateInboundDuplicate(t *testing.T) {
	w := req(inbRouter(&fakeInbCtrl{createErr: inbound.ErrTagExists}, &fakeRestarter{}), http.MethodPost, "/api/inbounds", `{"tag":"v1","port":443,"handshake":"h"}`)
	if w.Code != 409 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestCreateInboundBadBody(t *testing.T) {
	w := req(inbRouter(&fakeInbCtrl{}, &fakeRestarter{}), http.MethodPost, "/api/inbounds", `{"tag":"","port":0}`)
	if w.Code != 400 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestDeleteInboundNotFound(t *testing.T) {
	w := req(inbRouter(&fakeInbCtrl{deleteErr: inbound.ErrNotFound}, &fakeRestarter{}), http.MethodDelete, "/api/inbounds/9", "")
	if w.Code != 404 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestCreateUserOK(t *testing.T) {
	c := &fakeInbCtrl{}
	w := req(inbRouter(c, &fakeRestarter{}), http.MethodPost, "/api/inbounds/1/users", `{"name":"alice"}`)
	if w.Code != 200 || c.lastUName != "alice" {
		t.Fatalf("code=%d name=%q", w.Code, c.lastUName)
	}
}

func TestApplyReturnsStatus(t *testing.T) {
	r := &fakeRestarter{status: singbox.Status{Running: true, Installed: true}}
	w := req(inbRouter(&fakeInbCtrl{}, r), http.MethodPost, "/api/singbox/apply", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"running":true`) {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestApplyMapsSingboxError(t *testing.T) {
	r := &fakeRestarter{err: singbox.ErrNotInstalled}
	w := req(inbRouter(&fakeInbCtrl{}, r), http.MethodPost, "/api/singbox/apply", "")
	if w.Code != 400 {
		t.Fatalf("code=%d", w.Code)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/handlers/ -run 'TestCreateInbound|TestDeleteInbound|TestCreateUser|TestApply'`
Expected: FAIL（NewInboundHandler 未定义）

- [ ] **Step 3: 实现**

Create `backend/internal/handlers/inbound.go`:
```go
package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"singbox-admin/internal/inbound"
	"singbox-admin/internal/models"
	"singbox-admin/internal/singbox"
)

type InboundController interface {
	ListInbounds() ([]models.Inbound, error)
	CreateInbound(tag string, port uint16, handshake string) (models.Inbound, error)
	DeleteInbound(id uint) error
	ListUsers(inboundID uint) ([]models.User, error)
	CreateUser(inboundID uint, name string) (models.User, error)
	DeleteUser(id uint) error
	Regenerate() error
}

type Restarter interface {
	Restart() (singbox.Status, error)
}

type InboundHandler struct {
	ctrl      InboundController
	restarter Restarter
}

func NewInboundHandler(ctrl InboundController, r Restarter) *InboundHandler {
	return &InboundHandler{ctrl: ctrl, restarter: r}
}

func parseID(c *gin.Context) (uint, bool) {
	v, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return 0, false
	}
	return uint(v), true
}

func (h *InboundHandler) ListInbounds(c *gin.Context) {
	ins, err := h.ctrl.ListInbounds()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ins)
}

type createInboundBody struct {
	Tag       string `json:"tag" binding:"required"`
	Port      uint16 `json:"port" binding:"required"`
	Handshake string `json:"handshake" binding:"required"`
}

func (h *InboundHandler) CreateInbound(c *gin.Context) {
	var b createInboundBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag/port/handshake required"})
		return
	}
	in, err := h.ctrl.CreateInbound(b.Tag, b.Port, b.Handshake)
	if err != nil {
		if errors.Is(err, inbound.ErrTagExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "tag exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, in)
}

func (h *InboundHandler) DeleteInbound(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	if err := h.ctrl.DeleteInbound(id); err != nil {
		writeInboundError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *InboundHandler) ListUsers(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	us, err := h.ctrl.ListUsers(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, us)
}

type createUserBody struct {
	Name string `json:"name" binding:"required"`
}

func (h *InboundHandler) CreateUser(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	var b createUserBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}
	u, err := h.ctrl.CreateUser(id, b.Name)
	if err != nil {
		writeInboundError(c, err)
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *InboundHandler) DeleteUser(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	if err := h.ctrl.DeleteUser(id); err != nil {
		writeInboundError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *InboundHandler) Apply(c *gin.Context) {
	if err := h.ctrl.Regenerate(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	st, err := h.restarter.Restart()
	if err != nil {
		writeStartError(c, err) // from singbox.go (same package)
		return
	}
	c.JSON(http.StatusOK, st)
}

func writeInboundError(c *gin.Context, err error) {
	if errors.Is(err, inbound.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd backend && go test ./internal/handlers/`
Expected: PASS（含已有 auth/singbox handler 测试）

- [ ] **Step 5: 提交**

```bash
git add backend/internal/handlers/inbound.go backend/internal/handlers/inbound_test.go
git commit -m "feat(handlers): inbound/user CRUD + apply endpoints"
```

---

## Task 7: 接线 main.go

**Files:**
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: 改 main.go** — 完整替换为：

```go
package main

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"

	"singbox-admin/internal/auth"
	"singbox-admin/internal/config"
	"singbox-admin/internal/database"
	"singbox-admin/internal/handlers"
	"singbox-admin/internal/inbound"
	"singbox-admin/internal/middleware"
	"singbox-admin/internal/singbox"
	"singbox-admin/internal/web"
)

func main() {
	cfg := config.Load()

	db, err := database.Init(cfg.DBPath, cfg.DefaultAdminUser, cfg.DefaultAdminPass)
	if err != nil {
		log.Fatalf("database init: %v", err)
	}

	jm := auth.NewJWTManager(cfg.JWTSecret, 7*24*time.Hour)
	authHandler := handlers.NewAuthHandler(db, jm)

	sbSvc := singbox.NewDefault(cfg.SingboxDir, cfg.SingboxBin)
	sbHandler := handlers.NewSingboxHandler(sbSvc)

	inbSvc := inbound.NewService(db, sbSvc, inbound.NewKeyGen())
	inbHandler := handlers.NewInboundHandler(inbSvc, sbSvc)

	r := gin.Default()

	api := r.Group("/api")
	{
		api.POST("/auth/login", authHandler.Login)
		api.POST("/auth/logout", authHandler.Logout)

		authed := api.Group("", middleware.RequireAuth(jm))
		authed.GET("/status", sbHandler.Status)
		authed.GET("/singbox/config", sbHandler.GetConfig)
		authed.PUT("/singbox/config", sbHandler.PutConfig)
		authed.POST("/singbox/start", sbHandler.Start)
		authed.POST("/singbox/stop", sbHandler.Stop)
		authed.POST("/singbox/apply", inbHandler.Apply)

		authed.GET("/inbounds", inbHandler.ListInbounds)
		authed.POST("/inbounds", inbHandler.CreateInbound)
		authed.DELETE("/inbounds/:id", inbHandler.DeleteInbound)
		authed.GET("/inbounds/:id/users", inbHandler.ListUsers)
		authed.POST("/inbounds/:id/users", inbHandler.CreateUser)
		authed.DELETE("/users/:id", inbHandler.DeleteUser)
	}

	web.Register(r)

	log.Printf("listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 2: 编译 + vet + 全后端测试**

Run: `cd backend && go build ./... && go vet ./... && go test ./...`
Expected: 编译通过，全部 PASS。

- [ ] **Step 3: 端到端冒烟（建入站→应用→状态）**

Run:
```bash
cd backend && JWT_SECRET=t DB_PATH=/tmp/m3.db SINGBOX_DIR=/tmp/m3sb go build -o /tmp/m3srv ./cmd/server && (/tmp/m3srv >/tmp/m3.log 2>&1 &)
for i in $(seq 1 40); do curl -s -o /dev/null localhost:8080/api/nope 2>/dev/null && break; sleep 0.25; done
curl -s -c /tmp/m3.cookie -X POST localhost:8080/api/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"mnice7082"}' >/dev/null
echo -n "create inbound: "; curl -s -o /dev/null -w "%{http_code}\n" -b /tmp/m3.cookie -X POST localhost:8080/api/inbounds -H 'Content-Type: application/json' -d '{"tag":"v1","port":443,"handshake":"www.microsoft.com"}'
echo -n "list inbounds: "; curl -s -b /tmp/m3.cookie localhost:8080/api/inbounds | head -c 200; echo
echo -n "status hasConfig: "; curl -s -b /tmp/m3.cookie localhost:8080/api/status; echo
pkill -f /tmp/m3srv; rm -f /tmp/m3.db /tmp/m3.cookie /tmp/m3srv; rm -rf /tmp/m3sb
```
Expected: create inbound → 200；list 返回含 `"tag":"v1"` 和 reality 公钥；status `"hasConfig":true`（建入站后自动生成了配置）。

- [ ] **Step 4: 提交**

```bash
git add backend/cmd/server/main.go
git commit -m "feat(backend): wire inbound/user/apply routes"
```

---

## Task 8: 前端 API 客户端

**Files:**
- Modify: `frontend/lib/api.ts`
- Modify: `frontend/lib/api.test.ts`

- [ ] **Step 1: 追加失败测试**（在 `api.test.ts` 的 describe 内）

```ts
  it("listInbounds returns array", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify([{ id: 1, tag: "v1" }]), { status: 200 })));
    const ins = await listInbounds();
    expect(ins[0].tag).toBe("v1"); // camelCase JSON tags (see models)
  });

  it("createInbound posts body", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: 1, tag: "v1" }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    await createInbound("v1", 443, "www.microsoft.com");
    const [, init] = fetchMock.mock.calls[0];
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body).port).toBe(443);
  });

  it("applySingbox throws backend error on failure", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: "sing-box not installed" }), { status: 400 })));
    await expect(applySingbox()).rejects.toThrow(/not installed/);
  });
```

更新 import 行加入新符号：
```ts
import { login, getStatus, UnauthorizedError, startSingbox, getConfig, saveConfig, listInbounds, createInbound, applySingbox } from "./api";
```

- [ ] **Step 2: 运行确认失败**

Run: `cd frontend && npm test -- lib/api`
Expected: FAIL（listInbounds 未导出）

- [ ] **Step 3: 实现** — 在 `frontend/lib/api.ts` 末尾追加：

```ts
export interface Inbound {
  id: number;
  tag: string;
  port: number;
  flow: string;
  realityPublicKey: string;
  realityShortId: string;
  serverName: string;
  users?: SingboxUser[];
}

export interface SingboxUser {
  id: number;
  inboundId: number;
  name: string;
  uuid: string;
}

export async function listInbounds(): Promise<Inbound[]> {
  const res = await request("/api/inbounds");
  if (!res.ok) throw new Error("加载入站失败");
  return res.json();
}

export async function createInbound(tag: string, port: number, handshake: string): Promise<Inbound> {
  const res = await request("/api/inbounds", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ tag, port, handshake }),
  });
  if (!res.ok) {
    const b = await res.json().catch(() => ({}));
    throw new Error(b.error || "创建入站失败");
  }
  return res.json();
}

export async function deleteInbound(id: number): Promise<void> {
  const res = await request(`/api/inbounds/${id}`, { method: "DELETE" });
  if (!res.ok) throw new Error("删除入站失败");
}

export async function listUsers(inboundID: number): Promise<SingboxUser[]> {
  const res = await request(`/api/inbounds/${inboundID}/users`);
  if (!res.ok) throw new Error("加载用户失败");
  return res.json();
}

export async function createUser(inboundID: number, name: string): Promise<SingboxUser> {
  const res = await request(`/api/inbounds/${inboundID}/users`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  });
  if (!res.ok) {
    const b = await res.json().catch(() => ({}));
    throw new Error(b.error || "创建用户失败");
  }
  return res.json();
}

export async function deleteUser(id: number): Promise<void> {
  const res = await request(`/api/users/${id}`, { method: "DELETE" });
  if (!res.ok) throw new Error("删除用户失败");
}

export async function applySingbox(): Promise<SingboxStatus> {
  const res = await request("/api/singbox/apply", { method: "POST" });
  if (!res.ok) {
    const b = await res.json().catch(() => ({}));
    throw new Error(b.detail || b.error || "应用失败");
  }
  return res.json();
}
```

> 后端 models 已加 camelCase json tag（Task 1），故前端用 `id/tag/port/flow/realityPublicKey/realityShortId/serverName/users` 与 `id/inboundId/name/uuid`，与 `Status` 风格一致。私钥 `json:"-"` 不下发。

- [ ] **Step 4: 运行确认通过**

Run: `cd frontend && npm test -- lib/api`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add frontend/lib/api.ts frontend/lib/api.test.ts
git commit -m "feat(frontend): api client for inbounds/users/apply"
```

---

## Task 9: 侧边栏加「入站」

**Files:**
- Modify: `frontend/components/app-shell.tsx`
- Modify: `frontend/components/app-shell.test.tsx`

- [ ] **Step 1: 改测试**（在 app-shell.test.tsx 第一个 it 内追加断言）

把 `renders nav items 概览 and 配置` 测试体改为：
```tsx
  it("renders nav items 概览 入站 配置", () => {
    render(<AppShell><div>内容</div></AppShell>);
    expect(screen.getByRole("link", { name: /概览/ })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /入站/ })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /配置/ })).toBeInTheDocument();
  });
```

- [ ] **Step 2: 运行确认失败**

Run: `cd frontend && npm test -- components/app-shell`
Expected: FAIL（没有「入站」链接）

- [ ] **Step 3: 实现** — 在 `app-shell.tsx` 的 `NAV` 数组加一项（在 概览 与 配置 之间）：

```tsx
const NAV = [
  { href: "/dashboard", label: "概览" },
  { href: "/inbounds", label: "入站" },
  { href: "/config", label: "配置" },
];
```

- [ ] **Step 4: 运行确认通过**

Run: `cd frontend && npm test -- components/app-shell`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add frontend/components/app-shell.tsx frontend/components/app-shell.test.tsx
git commit -m "feat(frontend): add 入站 nav item"
```

---

## Task 10: 配置页改只读

**Files:**
- Modify: `frontend/app/config/page.tsx`
- Modify: `frontend/app/config/page.test.tsx`

- [ ] **Step 1: 改测试**（整体替换 config/page.test.tsx）

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import ConfigPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/config",
}));
const getConfigMock = vi.fn();
vi.mock("@/lib/api", () => ({
  getConfig: () => getConfigMock(),
  getStatus: vi.fn().mockResolvedValue({ installed: true, version: "1.13.13", running: false, hasConfig: true }),
  logout: vi.fn(),
  UnauthorizedError: class extends Error {},
}));

beforeEach(() => {
  getConfigMock.mockReset();
});

describe("ConfigPage", () => {
  it("载入配置且文本框只读", async () => {
    getConfigMock.mockResolvedValue('{"log":{}}');
    render(<ConfigPage />);
    await waitFor(() => expect(screen.getByRole("textbox")).toHaveValue('{"log":{}}'));
    expect(screen.getByRole("textbox")).toHaveAttribute("readonly");
    expect(screen.queryByRole("button", { name: /保存/ })).toBeNull();
  });
});
```

- [ ] **Step 2: 运行确认失败**

Run: `cd frontend && npm test -- app/config`
Expected: FAIL（仍可编辑 + 有保存按钮）

- [ ] **Step 3: 实现**（整体替换 config/page.tsx）

```tsx
"use client";

import { useEffect, useState } from "react";
import { getConfig } from "@/lib/api";
import { AppShell } from "@/components/app-shell";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function ConfigPage() {
  const [content, setContent] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    getConfig().then(setContent).catch(() => setError("加载配置失败"));
  }, []);

  return (
    <AppShell>
      <h1 className="mb-6 text-2xl font-normal tracking-tight">配置</h1>
      <Card className="rounded-lg">
        <CardHeader>
          <CardTitle className="font-mono text-xs tracking-wider text-muted-foreground uppercase">
            config.json（由入站/用户自动生成，只读）
          </CardTitle>
        </CardHeader>
        <CardContent>
          <textarea
            value={content}
            readOnly
            spellCheck={false}
            className="h-96 w-full rounded-lg border border-input bg-secondary p-3 font-mono text-sm text-foreground outline-none"
          />
          {error && <p className="mt-3 text-sm text-destructive">{error}</p>}
        </CardContent>
      </Card>
    </AppShell>
  );
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd frontend && npm test -- app/config`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add frontend/app/config
git commit -m "feat(frontend): config page read-only preview"
```

---

## Task 11: 概览页加「应用并重启」

**Files:**
- Modify: `frontend/app/dashboard/page.tsx`
- Modify: `frontend/app/dashboard/page.test.tsx`

- [ ] **Step 1: 追加失败测试**（在 dashboard/page.test.tsx 的 vi.mock("@/lib/api") 加 applySingbox，并加一个 it）

把 `vi.mock("@/lib/api", ...)` 改为包含 apply：
```tsx
const applyMock = vi.fn();
vi.mock("@/lib/api", () => ({
  getStatus: () => getStatusMock(),
  startSingbox: () => startMock(),
  stopSingbox: () => stopMock(),
  applySingbox: () => applyMock(),
  logout: vi.fn(),
  UnauthorizedError: class extends Error {},
}));
```
在 beforeEach 加 `applyMock.mockReset();`，并追加：
```tsx
  it("点击应用并重启调用 applySingbox", async () => {
    getStatusMock.mockResolvedValue({ installed: true, version: "1.13.13", running: false, hasConfig: true });
    applyMock.mockResolvedValue({ installed: true, version: "1.13.13", running: true, hasConfig: true });
    render(<DashboardPage />);
    const btn = await screen.findByRole("button", { name: /应用并重启/ });
    await userEvent.click(btn);
    expect(applyMock).toHaveBeenCalled();
  });
```

- [ ] **Step 2: 运行确认失败**

Run: `cd frontend && npm test -- app/dashboard`
Expected: FAIL（无「应用并重启」按钮）

- [ ] **Step 3: 实现** — 在 `dashboard/page.tsx`：
(a) import 加 `applySingbox`：
```tsx
import { getStatus, startSingbox, stopSingbox, applySingbox, UnauthorizedError, SingboxStatus } from "@/lib/api";
```
(b) 在启动/停止按钮那一行的 `<div className="mt-6 flex gap-3">` 内，停止按钮之后加：
```tsx
                <Button
                  variant="outline"
                  className="rounded-full"
                  disabled={busy || !status.installed}
                  onClick={() => run(applySingbox)}
                >
                  应用并重启
                </Button>
```

- [ ] **Step 4: 运行确认通过**

Run: `cd frontend && npm test -- app/dashboard`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add frontend/app/dashboard
git commit -m "feat(frontend): apply-and-restart button on dashboard"
```

---

## Task 12: 入站页

**Files:**
- Create: `frontend/app/inbounds/page.tsx`
- Test: `frontend/app/inbounds/page.test.tsx`

- [ ] **Step 1: 写失败测试**

Create `frontend/app/inbounds/page.test.tsx`:
```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import InboundsPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/inbounds",
}));

const listInboundsMock = vi.fn();
const createInboundMock = vi.fn();
const deleteInboundMock = vi.fn();
vi.mock("@/lib/api", () => ({
  listInbounds: () => listInboundsMock(),
  createInbound: (...a: unknown[]) => createInboundMock(...a),
  deleteInbound: (...a: unknown[]) => deleteInboundMock(...a),
  listUsers: vi.fn().mockResolvedValue([]),
  createUser: vi.fn(),
  deleteUser: vi.fn(),
  getStatus: vi.fn().mockResolvedValue({ installed: true, version: "1.13.13", running: false, hasConfig: true }),
  logout: vi.fn(),
  UnauthorizedError: class extends Error {},
}));

beforeEach(() => {
  listInboundsMock.mockReset();
  createInboundMock.mockReset();
  deleteInboundMock.mockReset();
});

describe("InboundsPage", () => {
  it("渲染入站列表", async () => {
    listInboundsMock.mockResolvedValue([
      { id: 1, tag: "v1", port: 443, flow: "xtls-rprx-vision", realityPublicKey: "PUB", realityShortId: "deadbeef", serverName: "www.microsoft.com", users: [] },
    ]);
    render(<InboundsPage />);
    await waitFor(() => expect(screen.getByText("v1")).toBeInTheDocument());
    expect(screen.getByText(/443/)).toBeInTheDocument();
  });

  it("新建入站调用 createInbound", async () => {
    listInboundsMock.mockResolvedValue([]);
    createInboundMock.mockResolvedValue({ id: 1, tag: "v2", port: 8443, users: [] });
    render(<InboundsPage />);
    await screen.findByRole("button", { name: /新建入站/ });
    await userEvent.type(screen.getByLabelText(/标签/), "v2");
    await userEvent.type(screen.getByLabelText(/端口/), "8443");
    await userEvent.click(screen.getByRole("button", { name: /新建入站/ }));
    await waitFor(() => expect(createInboundMock).toHaveBeenCalled());
  });
});
```

- [ ] **Step 2: 运行确认失败**

Run: `cd frontend && npm test -- app/inbounds`
Expected: FAIL（页面不存在）

- [ ] **Step 3: 实现**

Create `frontend/app/inbounds/page.tsx`:
```tsx
"use client";

import { useCallback, useEffect, useState } from "react";
import {
  listInbounds, createInbound, deleteInbound,
  listUsers, createUser, deleteUser,
  Inbound, SingboxUser,
} from "@/lib/api";
import { AppShell } from "@/components/app-shell";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function InboundsPage() {
  const [inbounds, setInbounds] = useState<Inbound[]>([]);
  const [tag, setTag] = useState("");
  const [port, setPort] = useState("");
  const [handshake, setHandshake] = useState("www.microsoft.com");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(() => {
    listInbounds().then(setInbounds).catch(() => setError("加载入站失败"));
  }, []);
  useEffect(refresh, [refresh]);

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      await createInbound(tag, Number(port), handshake);
      setTag(""); setPort("");
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "创建失败");
    } finally {
      setBusy(false);
    }
  }

  async function onDelete(id: number) {
    await deleteInbound(id);
    refresh();
  }

  return (
    <AppShell>
      <h1 className="mb-6 text-2xl font-normal tracking-tight">入站</h1>

      <Card className="mb-6 rounded-lg">
        <CardHeader>
          <CardTitle className="font-mono text-xs tracking-wider text-muted-foreground uppercase">
            新建入站（VLESS + Reality）
          </CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={onCreate} className="flex flex-wrap items-end gap-4">
            <div className="space-y-2">
              <Label htmlFor="tag" className="text-xs text-muted-foreground">标签</Label>
              <Input id="tag" className="h-10 w-40" value={tag} onChange={(e) => setTag(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="port" className="text-xs text-muted-foreground">端口</Label>
              <Input id="port" className="h-10 w-28" value={port} onChange={(e) => setPort(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="hs" className="text-xs text-muted-foreground">握手域名</Label>
              <Input id="hs" className="h-10 w-56" value={handshake} onChange={(e) => setHandshake(e.target.value)} />
            </div>
            <Button type="submit" className="h-10 rounded-full" disabled={busy}>新建入站</Button>
          </form>
          {error && <p className="mt-3 text-sm text-destructive">{error}</p>}
        </CardContent>
      </Card>

      <div className="space-y-4">
        {inbounds.map((ib) => (
          <InboundCard key={ib.id} inbound={ib} onDelete={() => onDelete(ib.id)} />
        ))}
        {inbounds.length === 0 && <p className="text-sm text-muted-foreground">暂无入站，先新建一个。</p>}
      </div>
    </AppShell>
  );
}

function InboundCard({ inbound, onDelete }: { inbound: Inbound; onDelete: () => void }) {
  const [users, setUsers] = useState<SingboxUser[]>([]);
  const [name, setName] = useState("");

  const refresh = useCallback(() => {
    listUsers(inbound.id).then(setUsers).catch(() => {});
  }, [inbound.id]);
  useEffect(refresh, [refresh]);

  async function onAdd(e: React.FormEvent) {
    e.preventDefault();
    if (!name) return;
    await createUser(inbound.id, name);
    setName("");
    refresh();
  }
  async function onRemove(id: number) {
    await deleteUser(id);
    refresh();
  }

  const field = (k: string, v: string) => (
    <div className="flex justify-between gap-4 py-1">
      <span className="font-mono text-xs text-muted-foreground uppercase">{k}</span>
      <span className="truncate font-mono text-xs">{v}</span>
    </div>
  );

  return (
    <Card className="rounded-lg">
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="text-base font-normal">
            {inbound.tag} <span className="text-muted-foreground">:{inbound.port}</span>
          </CardTitle>
          <Button variant="outline" className="rounded-full" onClick={onDelete}>删除</Button>
        </div>
      </CardHeader>
      <CardContent>
        <div className="mb-4 border-b border-border pb-3">
          {field("公钥", inbound.realityPublicKey)}
          {field("short id", inbound.realityShortId)}
          {field("sni", inbound.serverName)}
          {field("flow", inbound.flow)}
        </div>
        <p className="mb-2 font-mono text-xs text-muted-foreground uppercase">用户</p>
        <div className="divide-y divide-border">
          {users.map((u) => (
            <div key={u.id} className="flex items-center justify-between gap-4 py-2">
              <span className="text-sm">{u.name}</span>
              <span className="truncate font-mono text-xs text-muted-foreground">{u.uuid}</span>
              <Button variant="outline" className="rounded-full" onClick={() => onRemove(u.id)}>删除</Button>
            </div>
          ))}
          {users.length === 0 && <p className="py-2 text-sm text-muted-foreground">暂无用户</p>}
        </div>
        <form onSubmit={onAdd} className="mt-3 flex items-center gap-3">
          <Input className="h-9 w-40" placeholder="用户名称" value={name} onChange={(e) => setName(e.target.value)} />
          <Button type="submit" className="h-9 rounded-full">添加用户</Button>
        </form>
      </CardContent>
    </Card>
  );
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd frontend && npm test -- app/inbounds`
Expected: PASS。然后 `cd frontend && npm test`（全量）与 `npm run build`（路由含 /inbounds）。

- [ ] **Step 5: 提交**

```bash
git add frontend/app/inbounds
git commit -m "feat(frontend): inbounds page with user management"
```

---

## Task 13: 全量验证 + 截图

**Files:** 无（验证）

- [ ] **Step 1: 全后端测试**

Run: `cd backend && go test ./...`
Expected: 全 PASS。

- [ ] **Step 2: 全前端测试 + 构建**

Run: `cd frontend && npm test && npm run build`
Expected: 全 PASS；构建成功，路由含 `/login /dashboard /inbounds /config`。

- [ ] **Step 3: 实机截图**

`make build` 出单二进制，用临时 DB/SINGBOX_DIR 起在 :8080，浏览器登录后：
- 截图 `/inbounds`：新建一个入站，确认列表显示标签/端口/公钥、可加用户。
- 截图 `/dashboard`：确认有「应用并重启」按钮。
- 截图 `/config`：确认只读预览展示生成的 JSON。
完成后清理进程与临时文件。

- [ ] **Step 4: 提交（如有样式微调）**

```bash
git add -A && git commit -m "chore(frontend): M3 visual polish" || echo "no changes"
```

---

## Self-Review Notes

- **Spec 覆盖**：模型(Task1)、AutoMigrate(Task1)、KeyGen(Task2)、Generate(Task3)、Service CRUD+Regenerate(Task4)、Restart(Task5)、端点+错误码(Task6)、接线(Task7)、前端 api(Task8)、导航(Task9)、配置只读(Task10)、应用按钮(Task11)、入站页+用户管理+只读连接参数(Task12)、验证(Task13)。仅 VLESS+Reality、入站/用户为真相源——均符合。
- **类型一致性**：`KeyGen{RealityKeypair,UUID,ShortID}`、`ConfigWriter{SaveConfig}`(由 *singbox.Service 满足)、`InboundController`/`Restarter` 与 *inbound.Service / *singbox.Service 方法签名一致。`models.Inbound`/`User` 带 camelCase json tag（与 `Status` 一致），故 API 输出 `id/tag/port/flow/realityPublicKey/realityShortId/serverName/users`、`id/inboundId/name/uuid`，前端 TS 类型与页面（Task8/12）一致；`RealityPrivateKey` 标 `json:"-"` 不下发。
- **占位符**：无 TBD/TODO；每步含完整代码。
- **执行注意**：Task6 依赖同包 `writeStartError`（M2 singbox.go 已有）；Task8 前端字段名大写需与后端一致（已在任务内强调）；Task11 修改 dashboard 既有 mock，注意保留原有 start/stop 测试。
```

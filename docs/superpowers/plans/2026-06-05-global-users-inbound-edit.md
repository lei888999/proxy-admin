# M5: 全局用户 + 入站编辑 + Modal UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用户改为全局多对多实体（独立用户管理页）、入站支持编辑+重置密钥、入站/用户新建编辑走 modal、协议选择用 shadcn Select。

**Architecture:** driver 增 `CredentialKind/UpdateSettings/ResetSecrets`（model-free，先独立加好）；随后一次性把 User 模型改成全局 m2m、generate 按协议取凭证、service 加全局用户 CRUD + 入站 update/reset、handlers 拆出用户 handler。前端加轻量 Modal + shadcn Select。

**Tech Stack:** Go 1.26 + Gin + GORM(m2m)；Next.js 16 + TS + Tailwind + shadcn/ui + Vitest。

参考规范：`docs/superpowers/specs/2026-06-05-global-users-inbound-edit-design.md`

---

## File Structure

后端：
- `internal/inbound/driver.go`（改接口）、`driver_vless.go`、`driver_hysteria2.go`、`driver_test.go`（加 3 方法）
- `internal/models/inbound.go`（重写：User 全局 m2m）
- `internal/inbound/generate.go`（改：按 CredentialKind 取 uuid/password）
- `internal/inbound/service.go`（改：全局 User CRUD、UpdateInbound、ResetInboundKeys、UserView、InboundView 去 users）+ test
- `internal/handlers/inbound.go`（改：UpdateInbound/ResetKeys，去内联用户路由）+ test
- `internal/handlers/user.go`（新：UserController + 用户 handlers）+ test
- `cmd/server/main.go`（改：用户路由 + 入站 PUT/reset-keys，去旧用户路由）

前端：
- `frontend/components/ui/select.tsx`（shadcn add）
- `frontend/components/modal.tsx`（新）
- `frontend/lib/api.ts`（改）
- `frontend/components/app-shell.tsx`+test（加「用户」）
- `frontend/app/inbounds/page.tsx`+test（modal/select/edit/reset）
- `frontend/app/users/page.tsx`+test（新）

---

## Task 1: driver 增 CredentialKind / UpdateSettings / ResetSecrets

**Files:** Modify `backend/internal/inbound/driver.go`, `driver_vless.go`, `driver_hysteria2.go`, `driver_test.go`

- [ ] **Step 1: 追加失败测试**（在 `driver_test.go` 末尾）

```go
func TestVlessUpdateAndReset(t *testing.T) {
	d, _ := Get("vless-reality")
	s, _ := d.BuildSettings(map[string]any{"handshake": "a.com"})
	// update keeps private key, changes handshake
	u, err := d.UpdateSettings(s, map[string]any{"handshake": "b.com", "handshakePort": float64(8443)})
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	pi, _ := d.PublicInfo(u)
	if pi["serverName"] != "b.com" {
		t.Fatalf("serverName=%v", pi["serverName"])
	}
	in1, _ := d.BuildInbound("t", 1, s, nil)
	in2, _ := d.BuildInbound("t", 1, u, nil)
	pk1 := in1["tls"].(map[string]any)["reality"].(map[string]any)["private_key"]
	pk2 := in2["tls"].(map[string]any)["reality"].(map[string]any)["private_key"]
	if pk1 != pk2 {
		t.Fatal("UpdateSettings must keep the private key")
	}
	// reset changes the key, keeps handshake
	r, err := d.ResetSecrets(u)
	if err != nil {
		t.Fatalf("ResetSecrets: %v", err)
	}
	in3, _ := d.BuildInbound("t", 1, r, nil)
	pk3 := in3["tls"].(map[string]any)["reality"].(map[string]any)["private_key"]
	if pk3 == pk2 {
		t.Fatal("ResetSecrets must change the key")
	}
	rpi, _ := d.PublicInfo(r)
	if rpi["serverName"] != "b.com" {
		t.Fatal("ResetSecrets must keep handshake")
	}
	if d.CredentialKind() != "uuid" {
		t.Fatalf("CredentialKind=%s", d.CredentialKind())
	}
}

func TestHy2UpdateAndReset(t *testing.T) {
	d, _ := Get("hysteria2")
	s, _ := d.BuildSettings(nil)
	u, err := d.UpdateSettings(s, map[string]any{"serverName": "x.com", "upMbps": float64(10), "downMbps": float64(20)})
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	in, _ := d.BuildInbound("t", 1, u, nil)
	if in["up_mbps"] != 10 || in["down_mbps"] != 20 {
		t.Fatalf("up/down=%v/%v", in["up_mbps"], in["down_mbps"])
	}
	cert1 := in["tls"].(map[string]any)["certificate"]
	in0, _ := d.BuildInbound("t", 1, s, nil)
	cert0 := in0["tls"].(map[string]any)["certificate"]
	_ = cert0
	r, _ := d.ResetSecrets(u)
	in2, _ := d.BuildInbound("t", 1, r, nil)
	cert2 := in2["tls"].(map[string]any)["certificate"]
	if toJSON(cert1) == toJSON(cert2) {
		t.Fatal("ResetSecrets must change the cert")
	}
	rpi, _ := d.PublicInfo(r)
	if rpi["serverName"] != "x.com" || rpi["upMbps"] != 10 {
		t.Fatalf("reset publicInfo=%v", rpi)
	}
	if d.CredentialKind() != "password" {
		t.Fatalf("CredentialKind=%s", d.CredentialKind())
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/inbound/ -run 'TestVlessUpdateAndReset|TestHy2UpdateAndReset'`
Expected: FAIL（CredentialKind/UpdateSettings/ResetSecrets 未定义）

- [ ] **Step 3: 加接口方法** — 在 `driver.go` 的 `Driver` 接口里追加（在 `PublicInfo` 之后）：
```go
	CredentialKind() string                                            // "uuid" | "password"
	UpdateSettings(existing string, params map[string]any) (string, error) // keep secrets, apply params
	ResetSecrets(existing string) (string, error)                      // new secrets, keep params
```

- [ ] **Step 4: 实现 vless 三方法** — 在 `driver_vless.go` 末尾追加：
```go
func (vlessReality) CredentialKind() string { return "uuid" }

func (vlessReality) UpdateSettings(existing string, params map[string]any) (string, error) {
	var s vlessSettings
	if err := json.Unmarshal([]byte(existing), &s); err != nil {
		return "", err
	}
	if v, ok := params["handshake"].(string); ok && v != "" {
		s.Handshake = v
		s.ServerName = v
	}
	if v, ok := toInt(params["handshakePort"]); ok && v > 0 {
		s.HandshakePort = uint16(v)
	}
	b, err := json.Marshal(s)
	return string(b), err
}

func (d vlessReality) ResetSecrets(existing string) (string, error) {
	var s vlessSettings
	if err := json.Unmarshal([]byte(existing), &s); err != nil {
		return "", err
	}
	priv, pub, err := d.kg.RealityKeypair()
	if err != nil {
		return "", err
	}
	s.RealityPrivateKey, s.RealityPublicKey, s.ShortID = priv, pub, d.kg.ShortID()
	b, err := json.Marshal(s)
	return string(b), err
}
```

- [ ] **Step 5: 实现 hy2 三方法** — 在 `driver_hysteria2.go` 末尾追加：
```go
func (hysteria2) CredentialKind() string { return "password" }

func (hysteria2) UpdateSettings(existing string, params map[string]any) (string, error) {
	var s hy2Settings
	if err := json.Unmarshal([]byte(existing), &s); err != nil {
		return "", err
	}
	if v, ok := params["serverName"].(string); ok && v != "" {
		s.ServerName = v
	}
	if v, ok := toInt(params["upMbps"]); ok {
		s.UpMbps = v
	}
	if v, ok := toInt(params["downMbps"]); ok {
		s.DownMbps = v
	}
	b, err := json.Marshal(s)
	return string(b), err
}

func (hysteria2) ResetSecrets(existing string) (string, error) {
	var s hy2Settings
	if err := json.Unmarshal([]byte(existing), &s); err != nil {
		return "", err
	}
	cert, key, err := selfSignedCert(s.ServerName)
	if err != nil {
		return "", err
	}
	s.CertPEM, s.KeyPEM = cert, key
	b, err := json.Marshal(s)
	return string(b), err
}
```

- [ ] **Step 6: 运行确认通过 + vet**

Run: `cd backend && go test ./internal/inbound/ && go vet ./internal/inbound/`
Expected: 全 PASS（旧 model/generate/service 未动，仍编译通过）。

- [ ] **Step 7: 提交**

```bash
git add backend/internal/inbound/driver.go backend/internal/inbound/driver_vless.go backend/internal/inbound/driver_hysteria2.go backend/internal/inbound/driver_test.go
git commit -m "feat(inbound): driver CredentialKind/UpdateSettings/ResetSecrets"
```

---

## Task 2: 核心切换（全局用户 m2m + generate + service + handlers）

> 耦合核心：改 User 模型为全局 m2m → generate 按协议取凭证 → service 全局用户 CRUD + 入站 update/reset → handlers 拆用户。一个任务内完成，结尾 `go test ./internal/...` 全绿（main.go 预期编译失败，Task 3 修）。

**Files:** Rewrite `models/inbound.go`, `inbound/generate.go`+test, `inbound/service.go`+test, `handlers/inbound.go`+test; Create `handlers/user.go`+test.

- [ ] **Step 1: 重写 `backend/internal/models/inbound.go`**
```go
package models

import "time"

type Inbound struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Tag       string    `gorm:"uniqueIndex;not null" json:"tag"`
	Type      string    `gorm:"not null" json:"type"`
	Network   string    `gorm:"not null" json:"network"`
	Port      uint16    `gorm:"not null" json:"port"`
	Settings  string    `gorm:"not null" json:"-"`
	Users     []User    `gorm:"many2many:user_inbounds;" json:"-"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"not null" json:"name"`
	UUID      string    `gorm:"not null" json:"uuid"`
	Password  string    `gorm:"not null" json:"password"`
	Inbounds  []Inbound `gorm:"many2many:user_inbounds;" json:"-"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
```

- [ ] **Step 2: 重写 `backend/internal/inbound/generate.go`**
```go
package inbound

import (
	"encoding/json"
	"fmt"

	"singbox-admin/internal/models"
)

func Generate(inbounds []models.Inbound) (string, error) {
	ins := []map[string]any{}
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
			creds = append(creds, Cred{Name: u.Name, Credential: c})
		}
		piece, err := d.BuildInbound(in.Tag, in.Port, in.Settings, creds)
		if err != nil {
			return "", err
		}
		ins = append(ins, piece)
	}
	cfg := map[string]any{
		"log":       map[string]any{"level": "info"},
		"inbounds":  ins,
		"outbounds": []map[string]any{{"type": "direct", "tag": "direct"}},
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
```

- [ ] **Step 3: 重写 `backend/internal/inbound/generate_test.go`**
```go
package inbound

import (
	"encoding/json"
	"strings"
	"testing"

	"singbox-admin/internal/models"
)

func TestGeneratePicksCredByProtocol(t *testing.T) {
	vd, _ := Get("vless-reality")
	vs, _ := vd.BuildSettings(nil)
	hd, _ := Get("hysteria2")
	hs, _ := hd.BuildSettings(nil)
	ins := []models.Inbound{
		{Tag: "v1", Type: "vless-reality", Network: "tcp", Port: 8443, Settings: vs,
			Users: []models.User{{Name: "a", UUID: "uuid-x", Password: "pw-x"}}},
		{Tag: "h1", Type: "hysteria2", Network: "udp", Port: 443, Settings: hs,
			Users: []models.User{{Name: "a", UUID: "uuid-x", Password: "pw-x"}}},
	}
	out, err := Generate(ins)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(out, `"uuid": "uuid-x"`) {
		t.Fatal("vless should use UUID")
	}
	if !strings.Contains(out, `"password": "pw-x"`) {
		t.Fatal("hysteria2 should use Password")
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
}
```

- [ ] **Step 4: 重写 `backend/internal/inbound/service.go`**
```go
package inbound

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"

	"gorm.io/gorm"

	"singbox-admin/internal/models"
)

var (
	ErrTagExists   = errors.New("tag exists")
	ErrNotFound    = errors.New("not found")
	ErrInvalidTag  = errors.New("invalid tag")
	ErrInvalidName = errors.New("invalid name")
	ErrPortInUse   = errors.New("port in use")
)

type ConfigWriter interface {
	SaveConfig(content string) error
}

type Service struct {
	db     *gorm.DB
	writer ConfigWriter
}

func NewService(db *gorm.DB, writer ConfigWriter) *Service {
	return &Service{db: db, writer: writer}
}

func genUUID() string { return NewKeyGen().UUID() }

func genPassword() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// ---------- Inbounds ----------

type InboundView struct {
	ID         uint           `json:"id"`
	Type       string         `json:"type"`
	Tag        string         `json:"tag"`
	Port       uint16         `json:"port"`
	Network    string         `json:"network"`
	PublicInfo map[string]any `json:"publicInfo"`
}

func (s *Service) listInbounds() ([]models.Inbound, error) {
	var ins []models.Inbound
	err := s.db.Preload("Users").Order("id").Find(&ins).Error
	return ins, err
}

func (s *Service) ListInboundViews() ([]InboundView, error) {
	var ins []models.Inbound
	if err := s.db.Order("id").Find(&ins).Error; err != nil {
		return nil, err
	}
	views := make([]InboundView, 0, len(ins))
	for _, in := range ins {
		v := InboundView{ID: in.ID, Type: in.Type, Tag: in.Tag, Port: in.Port, Network: in.Network}
		if d, ok := Get(in.Type); ok {
			if pi, err := d.PublicInfo(in.Settings); err == nil {
				v.PublicInfo = pi
			}
		}
		views = append(views, v)
	}
	return views, nil
}

func (s *Service) Types() []TypeInfo { return Types() }

func (s *Service) CreateInbound(typ, tag string, port uint16, params map[string]any) (models.Inbound, error) {
	d, ok := Get(typ)
	if !ok {
		return models.Inbound{}, ErrUnknownType
	}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return models.Inbound{}, ErrInvalidTag
	}
	var count int64
	s.db.Model(&models.Inbound{}).Where("tag = ?", tag).Count(&count)
	if count > 0 {
		return models.Inbound{}, ErrTagExists
	}
	s.db.Model(&models.Inbound{}).Where("port = ? AND network = ?", port, d.Network()).Count(&count)
	if count > 0 {
		return models.Inbound{}, ErrPortInUse
	}
	settings, err := d.BuildSettings(params)
	if err != nil {
		return models.Inbound{}, err
	}
	in := models.Inbound{Tag: tag, Type: typ, Network: d.Network(), Port: port, Settings: settings}
	if err := s.db.Create(&in).Error; err != nil {
		return models.Inbound{}, err
	}
	return in, s.Regenerate()
}

func (s *Service) UpdateInbound(id uint, tag string, port uint16, params map[string]any) (models.Inbound, error) {
	var in models.Inbound
	if err := s.db.First(&in, id).Error; err != nil {
		return models.Inbound{}, ErrNotFound
	}
	d, ok := Get(in.Type)
	if !ok {
		return models.Inbound{}, ErrUnknownType
	}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return models.Inbound{}, ErrInvalidTag
	}
	var count int64
	s.db.Model(&models.Inbound{}).Where("tag = ? AND id <> ?", tag, id).Count(&count)
	if count > 0 {
		return models.Inbound{}, ErrTagExists
	}
	s.db.Model(&models.Inbound{}).Where("port = ? AND network = ? AND id <> ?", port, in.Network, id).Count(&count)
	if count > 0 {
		return models.Inbound{}, ErrPortInUse
	}
	settings, err := d.UpdateSettings(in.Settings, params)
	if err != nil {
		return models.Inbound{}, err
	}
	in.Tag, in.Port, in.Settings = tag, port, settings
	if err := s.db.Save(&in).Error; err != nil {
		return models.Inbound{}, err
	}
	return in, s.Regenerate()
}

func (s *Service) ResetInboundKeys(id uint) (models.Inbound, error) {
	var in models.Inbound
	if err := s.db.First(&in, id).Error; err != nil {
		return models.Inbound{}, ErrNotFound
	}
	d, ok := Get(in.Type)
	if !ok {
		return models.Inbound{}, ErrUnknownType
	}
	settings, err := d.ResetSecrets(in.Settings)
	if err != nil {
		return models.Inbound{}, err
	}
	in.Settings = settings
	if err := s.db.Save(&in).Error; err != nil {
		return models.Inbound{}, err
	}
	return in, s.Regenerate()
}

func (s *Service) DeleteInbound(id uint) error {
	var in models.Inbound
	if err := s.db.First(&in, id).Error; err != nil {
		return ErrNotFound
	}
	if err := s.db.Model(&in).Association("Users").Clear(); err != nil {
		return err
	}
	if err := s.db.Delete(&in).Error; err != nil {
		return err
	}
	return s.Regenerate()
}

// ---------- Users (global, many-to-many) ----------

type UserView struct {
	ID          uint     `json:"id"`
	Name        string   `json:"name"`
	UUID        string   `json:"uuid"`
	Password    string   `json:"password"`
	InboundIDs  []uint   `json:"inboundIds"`
	InboundTags []string `json:"inboundTags"`
}

func (s *Service) ListUserViews() ([]UserView, error) {
	var us []models.User
	if err := s.db.Preload("Inbounds").Order("id").Find(&us).Error; err != nil {
		return nil, err
	}
	views := make([]UserView, 0, len(us))
	for _, u := range us {
		v := UserView{ID: u.ID, Name: u.Name, UUID: u.UUID, Password: u.Password, InboundIDs: []uint{}, InboundTags: []string{}}
		for _, in := range u.Inbounds {
			v.InboundIDs = append(v.InboundIDs, in.ID)
			v.InboundTags = append(v.InboundTags, in.Tag)
		}
		views = append(views, v)
	}
	return views, nil
}

func (s *Service) setUserInbounds(u *models.User, inboundIDs []uint) error {
	var ins []models.Inbound
	if len(inboundIDs) > 0 {
		if err := s.db.Find(&ins, inboundIDs).Error; err != nil {
			return err
		}
	}
	return s.db.Model(u).Association("Inbounds").Replace(ins)
}

func (s *Service) CreateUser(name string, inboundIDs []uint) (models.User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return models.User{}, ErrInvalidName
	}
	u := models.User{Name: name, UUID: genUUID(), Password: genPassword()}
	if err := s.db.Create(&u).Error; err != nil {
		return models.User{}, err
	}
	if err := s.setUserInbounds(&u, inboundIDs); err != nil {
		return models.User{}, err
	}
	return u, s.Regenerate()
}

func (s *Service) UpdateUser(id uint, name string, inboundIDs []uint) (models.User, error) {
	var u models.User
	if err := s.db.First(&u, id).Error; err != nil {
		return models.User{}, ErrNotFound
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return models.User{}, ErrInvalidName
	}
	u.Name = name
	if err := s.db.Save(&u).Error; err != nil {
		return models.User{}, err
	}
	if err := s.setUserInbounds(&u, inboundIDs); err != nil {
		return models.User{}, err
	}
	return u, s.Regenerate()
}

func (s *Service) ResetUserCreds(id uint) (models.User, error) {
	var u models.User
	if err := s.db.First(&u, id).Error; err != nil {
		return models.User{}, ErrNotFound
	}
	u.UUID, u.Password = genUUID(), genPassword()
	if err := s.db.Save(&u).Error; err != nil {
		return models.User{}, err
	}
	return u, s.Regenerate()
}

func (s *Service) DeleteUser(id uint) error {
	var u models.User
	if err := s.db.First(&u, id).Error; err != nil {
		return ErrNotFound
	}
	if err := s.db.Model(&u).Association("Inbounds").Clear(); err != nil {
		return err
	}
	if err := s.db.Delete(&u).Error; err != nil {
		return err
	}
	return s.Regenerate()
}

func (s *Service) Regenerate() error {
	ins, err := s.listInbounds()
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

- [ ] **Step 5: 重写 `backend/internal/inbound/service_test.go`**
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

func newTestService(t *testing.T) (*Service, *fakeWriter) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := db.AutoMigrate(&models.Inbound{}, &models.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	w := &fakeWriter{}
	return NewService(db, w), w
}

func TestCreateUserWithInbounds(t *testing.T) {
	s, w := newTestService(t)
	v, _ := s.CreateInbound("vless-reality", "v1", 8443, nil)
	h, _ := s.CreateInbound("hysteria2", "h1", 443, nil)
	u, err := s.CreateUser("alice", []uint{v.ID, h.ID})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.UUID == "" || u.Password == "" {
		t.Fatalf("creds empty: %+v", u)
	}
	// config now has alice's uuid (vless) and password (hy2)
	if !strings.Contains(w.last, u.UUID) || !strings.Contains(w.last, u.Password) {
		t.Fatalf("config missing creds: %q", w.last)
	}
	views, _ := s.ListUserViews()
	if len(views) != 1 || len(views[0].InboundIDs) != 2 {
		t.Fatalf("view=%+v", views)
	}
}

func TestUpdateUserReplacesInbounds(t *testing.T) {
	s, _ := newTestService(t)
	v, _ := s.CreateInbound("vless-reality", "v1", 8443, nil)
	h, _ := s.CreateInbound("hysteria2", "h1", 443, nil)
	u, _ := s.CreateUser("a", []uint{v.ID})
	if _, err := s.UpdateUser(u.ID, "a2", []uint{h.ID}); err != nil {
		t.Fatal(err)
	}
	views, _ := s.ListUserViews()
	if views[0].Name != "a2" || len(views[0].InboundIDs) != 1 || views[0].InboundIDs[0] != h.ID {
		t.Fatalf("view=%+v", views[0])
	}
}

func TestDeleteUserKeepsInbound(t *testing.T) {
	s, _ := newTestService(t)
	v, _ := s.CreateInbound("vless-reality", "v1", 8443, nil)
	u, _ := s.CreateUser("a", []uint{v.ID})
	if err := s.DeleteUser(u.ID); err != nil {
		t.Fatal(err)
	}
	var nUsers, nInbounds int64
	s.db.Model(&models.User{}).Count(&nUsers)
	s.db.Model(&models.Inbound{}).Count(&nInbounds)
	if nUsers != 0 || nInbounds != 1 {
		t.Fatalf("users=%d inbounds=%d", nUsers, nInbounds)
	}
}

func TestResetUserCreds(t *testing.T) {
	s, _ := newTestService(t)
	u, _ := s.CreateUser("a", nil)
	old := u.UUID
	r, _ := s.ResetUserCreds(u.ID)
	if r.UUID == old {
		t.Fatal("uuid not reset")
	}
}

func TestUpdateInboundKeepsKeyChangesTag(t *testing.T) {
	s, _ := newTestService(t)
	in, _ := s.CreateInbound("vless-reality", "v1", 8443, map[string]any{"handshake": "a.com"})
	d, _ := Get("vless-reality")
	before, _ := d.PublicInfo(in.Settings)
	up, err := s.UpdateInbound(in.ID, "v1b", 9443, map[string]any{"handshake": "b.com"})
	if err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}
	after, _ := d.PublicInfo(up.Settings)
	if up.Tag != "v1b" || up.Port != 9443 {
		t.Fatalf("not updated: %+v", up)
	}
	if after["realityPublicKey"] != before["realityPublicKey"] {
		t.Fatal("UpdateInbound must keep the key")
	}
	if after["serverName"] != "b.com" {
		t.Fatalf("serverName=%v", after["serverName"])
	}
}

func TestUpdateInboundDuplicateTag(t *testing.T) {
	s, _ := newTestService(t)
	_, _ = s.CreateInbound("vless-reality", "v1", 8443, nil)
	in2, _ := s.CreateInbound("vless-reality", "v2", 8444, nil)
	if _, err := s.UpdateInbound(in2.ID, "v1", 8444, nil); err != ErrTagExists {
		t.Fatalf("err=%v, want ErrTagExists", err)
	}
	// updating to its own tag is fine
	if _, err := s.UpdateInbound(in2.ID, "v2", 8444, nil); err != nil {
		t.Fatalf("same-tag update should pass: %v", err)
	}
}

func TestResetInboundKeysChangesKey(t *testing.T) {
	s, _ := newTestService(t)
	in, _ := s.CreateInbound("vless-reality", "v1", 8443, nil)
	d, _ := Get("vless-reality")
	before, _ := d.PublicInfo(in.Settings)
	r, _ := s.ResetInboundKeys(in.ID)
	after, _ := d.PublicInfo(r.Settings)
	if after["realityPublicKey"] == before["realityPublicKey"] {
		t.Fatal("reset must change the key")
	}
}
```

- [ ] **Step 6: 重写 `backend/internal/handlers/inbound.go`**

完整替换（去内联用户路由；加 UpdateInbound/ResetKeys）：
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
	ListInboundViews() ([]inbound.InboundView, error)
	Types() []inbound.TypeInfo
	CreateInbound(typ, tag string, port uint16, params map[string]any) (models.Inbound, error)
	UpdateInbound(id uint, tag string, port uint16, params map[string]any) (models.Inbound, error)
	ResetInboundKeys(id uint) (models.Inbound, error)
	DeleteInbound(id uint) error
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

func (h *InboundHandler) ListTypes(c *gin.Context) { c.JSON(http.StatusOK, h.ctrl.Types()) }

func (h *InboundHandler) ListInbounds(c *gin.Context) {
	views, err := h.ctrl.ListInboundViews()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, views)
}

type inboundBody struct {
	Type   string         `json:"type"`
	Tag    string         `json:"tag" binding:"required"`
	Port   uint16         `json:"port" binding:"required"`
	Params map[string]any `json:"params"`
}

func writeInboundCreateErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, inbound.ErrUnknownType), errors.Is(err, inbound.ErrInvalidTag):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, inbound.ErrTagExists), errors.Is(err, inbound.ErrPortInUse):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, inbound.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func (h *InboundHandler) CreateInbound(c *gin.Context) {
	var b inboundBody
	if err := c.ShouldBindJSON(&b); err != nil || b.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type/tag/port required"})
		return
	}
	in, err := h.ctrl.CreateInbound(b.Type, b.Tag, b.Port, b.Params)
	if err != nil {
		writeInboundCreateErr(c, err)
		return
	}
	c.JSON(http.StatusOK, in)
}

func (h *InboundHandler) UpdateInbound(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	var b inboundBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag/port required"})
		return
	}
	in, err := h.ctrl.UpdateInbound(id, b.Tag, b.Port, b.Params)
	if err != nil {
		writeInboundCreateErr(c, err)
		return
	}
	c.JSON(http.StatusOK, in)
}

func (h *InboundHandler) ResetKeys(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	in, err := h.ctrl.ResetInboundKeys(id)
	if err != nil {
		writeInboundCreateErr(c, err)
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
		if errors.Is(err, inbound.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		writeStartError(c, err)
		return
	}
	c.JSON(http.StatusOK, st)
}
```

- [ ] **Step 7: 重写 `backend/internal/handlers/inbound_test.go`**
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
	views     []inbound.InboundView
	types     []inbound.TypeInfo
	createErr error
	updateErr error
	resetErr  error
	deleteErr error
	regenErr  error
	lastType  string
	lastTag   string
	lastID    uint
}

func (f *fakeInbCtrl) ListInboundViews() ([]inbound.InboundView, error) { return f.views, nil }
func (f *fakeInbCtrl) Types() []inbound.TypeInfo                        { return f.types }
func (f *fakeInbCtrl) CreateInbound(typ, tag string, port uint16, p map[string]any) (models.Inbound, error) {
	f.lastType, f.lastTag = typ, tag
	return models.Inbound{ID: 1, Tag: tag, Type: typ, Port: port}, f.createErr
}
func (f *fakeInbCtrl) UpdateInbound(id uint, tag string, port uint16, p map[string]any) (models.Inbound, error) {
	f.lastID, f.lastTag = id, tag
	return models.Inbound{ID: id, Tag: tag, Port: port}, f.updateErr
}
func (f *fakeInbCtrl) ResetInboundKeys(id uint) (models.Inbound, error) {
	f.lastID = id
	return models.Inbound{ID: id}, f.resetErr
}
func (f *fakeInbCtrl) DeleteInbound(id uint) error { return f.deleteErr }
func (f *fakeInbCtrl) Regenerate() error           { return f.regenErr }

type fakeRestarter struct {
	status singbox.Status
	err    error
}

func (f *fakeRestarter) Restart() (singbox.Status, error) { return f.status, f.err }

func inbRouter(ctrl InboundController, r Restarter) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewInboundHandler(ctrl, r)
	e := gin.New()
	e.GET("/api/inbound-types", h.ListTypes)
	e.GET("/api/inbounds", h.ListInbounds)
	e.POST("/api/inbounds", h.CreateInbound)
	e.PUT("/api/inbounds/:id", h.UpdateInbound)
	e.POST("/api/inbounds/:id/reset-keys", h.ResetKeys)
	e.DELETE("/api/inbounds/:id", h.DeleteInbound)
	e.POST("/api/singbox/apply", h.Apply)
	return e
}

func inbReq(e *gin.Engine, m, p, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(m, p, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	e.ServeHTTP(w, r)
	return w
}

func TestListTypes(t *testing.T) {
	c := &fakeInbCtrl{types: []inbound.TypeInfo{{Type: "hysteria2", Label: "Hysteria2", Network: "udp", DefaultPort: 443}}}
	w := inbReq(inbRouter(c, &fakeRestarter{}), http.MethodGet, "/api/inbound-types", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "hysteria2") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestCreateInboundOK(t *testing.T) {
	c := &fakeInbCtrl{}
	w := inbReq(inbRouter(c, &fakeRestarter{}), http.MethodPost, "/api/inbounds", `{"type":"hysteria2","tag":"h1","port":443,"params":{"upMbps":50}}`)
	if w.Code != 200 || c.lastType != "hysteria2" {
		t.Fatalf("code=%d type=%q", w.Code, c.lastType)
	}
}

func TestUpdateInboundOK(t *testing.T) {
	c := &fakeInbCtrl{}
	w := inbReq(inbRouter(c, &fakeRestarter{}), http.MethodPut, "/api/inbounds/5", `{"tag":"v2","port":9443,"params":{"handshake":"x.com"}}`)
	if w.Code != 200 || c.lastID != 5 || c.lastTag != "v2" {
		t.Fatalf("code=%d id=%d tag=%q", w.Code, c.lastID, c.lastTag)
	}
}

func TestUpdateInboundConflict(t *testing.T) {
	w := inbReq(inbRouter(&fakeInbCtrl{updateErr: inbound.ErrTagExists}, &fakeRestarter{}), http.MethodPut, "/api/inbounds/5", `{"tag":"v2","port":1}`)
	if w.Code != 409 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestResetKeysOK(t *testing.T) {
	c := &fakeInbCtrl{}
	w := inbReq(inbRouter(c, &fakeRestarter{}), http.MethodPost, "/api/inbounds/7/reset-keys", "")
	if w.Code != 200 || c.lastID != 7 {
		t.Fatalf("code=%d id=%d", w.Code, c.lastID)
	}
}

func TestApplyReturnsStatus(t *testing.T) {
	r := &fakeRestarter{status: singbox.Status{Running: true, Installed: true}}
	w := inbReq(inbRouter(&fakeInbCtrl{}, r), http.MethodPost, "/api/singbox/apply", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"running":true`) {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestApplyMapsSingboxError(t *testing.T) {
	r := &fakeRestarter{err: singbox.ErrNotInstalled}
	w := inbReq(inbRouter(&fakeInbCtrl{}, r), http.MethodPost, "/api/singbox/apply", "")
	if w.Code != 400 {
		t.Fatalf("code=%d", w.Code)
	}
}
```

- [ ] **Step 8: 创建 `backend/internal/handlers/user.go`**
```go
package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"singbox-admin/internal/inbound"
	"singbox-admin/internal/models"
)

type UserController interface {
	ListUserViews() ([]inbound.UserView, error)
	CreateUser(name string, inboundIDs []uint) (models.User, error)
	UpdateUser(id uint, name string, inboundIDs []uint) (models.User, error)
	ResetUserCreds(id uint) (models.User, error)
	DeleteUser(id uint) error
}

type UserHandler struct{ ctrl UserController }

func NewUserHandler(ctrl UserController) *UserHandler { return &UserHandler{ctrl: ctrl} }

func (h *UserHandler) List(c *gin.Context) {
	views, err := h.ctrl.ListUserViews()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, views)
}

type userBody struct {
	Name       string `json:"name" binding:"required"`
	InboundIDs []uint `json:"inboundIds"`
}

func writeUserErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, inbound.ErrInvalidName):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid name"})
	case errors.Is(err, inbound.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func (h *UserHandler) Create(c *gin.Context) {
	var b userBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}
	u, err := h.ctrl.CreateUser(b.Name, b.InboundIDs)
	if err != nil {
		writeUserErr(c, err)
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *UserHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	var b userBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}
	u, err := h.ctrl.UpdateUser(id, b.Name, b.InboundIDs)
	if err != nil {
		writeUserErr(c, err)
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *UserHandler) Reset(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	u, err := h.ctrl.ResetUserCreds(id)
	if err != nil {
		writeUserErr(c, err)
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *UserHandler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	if err := h.ctrl.DeleteUser(id); err != nil {
		writeUserErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
```

- [ ] **Step 9: 创建 `backend/internal/handlers/user_test.go`**
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
)

type fakeUserCtrl struct {
	views     []inbound.UserView
	createErr error
	lastName  string
	lastIDs   []uint
}

func (f *fakeUserCtrl) ListUserViews() ([]inbound.UserView, error) { return f.views, nil }
func (f *fakeUserCtrl) CreateUser(name string, ids []uint) (models.User, error) {
	f.lastName, f.lastIDs = name, ids
	return models.User{ID: 1, Name: name, UUID: "u", Password: "p"}, f.createErr
}
func (f *fakeUserCtrl) UpdateUser(id uint, name string, ids []uint) (models.User, error) {
	return models.User{ID: id, Name: name}, nil
}
func (f *fakeUserCtrl) ResetUserCreds(id uint) (models.User, error) {
	return models.User{ID: id, UUID: "new"}, nil
}
func (f *fakeUserCtrl) DeleteUser(id uint) error { return nil }

func userRouter(ctrl UserController) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewUserHandler(ctrl)
	e := gin.New()
	e.GET("/api/users", h.List)
	e.POST("/api/users", h.Create)
	e.PUT("/api/users/:id", h.Update)
	e.POST("/api/users/:id/reset", h.Reset)
	e.DELETE("/api/users/:id", h.Delete)
	return e
}

func ureq(e *gin.Engine, m, p, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(m, p, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	e.ServeHTTP(w, r)
	return w
}

func TestUserCreate(t *testing.T) {
	c := &fakeUserCtrl{}
	w := ureq(userRouter(c), http.MethodPost, "/api/users", `{"name":"alice","inboundIds":[1,2]}`)
	if w.Code != 200 || c.lastName != "alice" || len(c.lastIDs) != 2 {
		t.Fatalf("code=%d name=%q ids=%v", w.Code, c.lastName, c.lastIDs)
	}
}

func TestUserCreateInvalidName(t *testing.T) {
	w := ureq(userRouter(&fakeUserCtrl{createErr: inbound.ErrInvalidName}), http.MethodPost, "/api/users", `{"name":"x"}`)
	if w.Code != 400 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestUserList(t *testing.T) {
	c := &fakeUserCtrl{views: []inbound.UserView{{ID: 1, Name: "a", UUID: "u", Password: "p", InboundTags: []string{"v1"}}}}
	w := ureq(userRouter(c), http.MethodGet, "/api/users", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"password":"p"`) {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestUserReset(t *testing.T) {
	w := ureq(userRouter(&fakeUserCtrl{}), http.MethodPost, "/api/users/3/reset", "")
	if w.Code != 200 {
		t.Fatalf("code=%d", w.Code)
	}
}
```

- [ ] **Step 10: 验证 internal 包**

Run: `cd backend && go test ./internal/... && go vet ./internal/...`
Expected: 全 PASS。（`go build ./...` / `go test ./...` 会因 main.go 旧路由编译失败——预期，Task 3 修。）

- [ ] **Step 11: 提交**

```bash
git add backend/internal/models/inbound.go backend/internal/inbound/generate.go backend/internal/inbound/generate_test.go backend/internal/inbound/service.go backend/internal/inbound/service_test.go backend/internal/handlers/inbound.go backend/internal/handlers/inbound_test.go backend/internal/handlers/user.go backend/internal/handlers/user_test.go
git commit -m "feat: global users (m2m) + inbound update/reset; split user handlers"
```

---

## Task 3: 接线 main.go

**Files:** Modify `backend/cmd/server/main.go`

- [ ] **Step 1: 改 main.go** — 替换 inbound/user 接线与路由组。`inbHandler` 之后加 `userHandler`，并重写 `authed` 中入站/用户路由：

把构造段改为：
```go
	inbSvc := inbound.NewService(db, sbSvc)
	if err := inbound.SeedDefaults(inbSvc); err != nil {
		log.Fatalf("seed defaults: %v", err)
	}
	inbHandler := handlers.NewInboundHandler(inbSvc, sbSvc)
	userHandler := handlers.NewUserHandler(inbSvc)
```
把 `authed` 组里入站/用户相关路由替换为：
```go
		authed.GET("/status", sbHandler.Status)
		authed.GET("/singbox/config", sbHandler.GetConfig)
		authed.PUT("/singbox/config", sbHandler.PutConfig)
		authed.POST("/singbox/start", sbHandler.Start)
		authed.POST("/singbox/stop", sbHandler.Stop)
		authed.POST("/singbox/apply", inbHandler.Apply)

		authed.GET("/inbound-types", inbHandler.ListTypes)
		authed.GET("/inbounds", inbHandler.ListInbounds)
		authed.POST("/inbounds", inbHandler.CreateInbound)
		authed.PUT("/inbounds/:id", inbHandler.UpdateInbound)
		authed.POST("/inbounds/:id/reset-keys", inbHandler.ResetKeys)
		authed.DELETE("/inbounds/:id", inbHandler.DeleteInbound)

		authed.GET("/users", userHandler.List)
		authed.POST("/users", userHandler.Create)
		authed.PUT("/users/:id", userHandler.Update)
		authed.POST("/users/:id/reset", userHandler.Reset)
		authed.DELETE("/users/:id", userHandler.Delete)
```
（删除旧的 `authed.GET/POST "/inbounds/:id/users"`、旧 `DELETE "/users/:id"` 等 M4 行。）

- [ ] **Step 2: 全量构建 + vet + 测试**

Run: `cd backend && go build ./... && go vet ./... && go test ./...`
Expected: 全 PASS。

- [ ] **Step 3: 端到端冒烟（用户挂入站 → 配置含 uuid+password）**

```bash
cd backend && JWT_SECRET=t DB_PATH=/tmp/m5.db SINGBOX_DIR=/tmp/m5sb go build -o /tmp/m5srv ./cmd/server && (/tmp/m5srv >/tmp/m5.log 2>&1 &)
for i in $(seq 1 40); do curl -s -o /dev/null localhost:8080/api/nope 2>/dev/null && break; sleep 0.25; done
C=/tmp/m5.cookie
curl -s -c $C -X POST localhost:8080/api/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"mnice7082"}' >/dev/null
echo "inbound ids:"; curl -s -b $C localhost:8080/api/inbounds | python3 -c "import sys,json;print([i['id'] for i in json.load(sys.stdin)])"
echo -n "create user: "; curl -s -o /dev/null -w "%{http_code}\n" -b $C -X POST localhost:8080/api/users -H 'Content-Type: application/json' -d '{"name":"alice","inboundIds":[1,2]}'
echo "users:"; curl -s -b $C localhost:8080/api/users | head -c 300; echo
echo -n "update inbound 1 (tag): "; curl -s -o /dev/null -w "%{http_code}\n" -b $C -X PUT localhost:8080/api/inbounds/1 -H 'Content-Type: application/json' -d '{"tag":"vless-x","port":8443,"params":{"handshake":"a.com"}}'
echo -n "reset keys inbound 1: "; curl -s -o /dev/null -w "%{http_code}\n" -b $C -X POST localhost:8080/api/inbounds/1/reset-keys
pkill -f /tmp/m5srv; rm -f /tmp/m5.db /tmp/m5.cookie /tmp/m5srv; rm -rf /tmp/m5sb
```
Expected: inbound ids [1,2]; create user 200; users 返回 alice（含 uuid+password+inboundTags 两个）；update 200；reset-keys 200。确保杀进程清临时文件，`git add` 只加 main.go。

- [ ] **Step 4: 提交**

```bash
git add backend/cmd/server/main.go
git commit -m "feat(backend): wire global user + inbound update/reset routes"
```

---

## Task 4: 前端 — shadcn Select + Modal 组件 + api 客户端

**Files:** add shadcn select; Create `frontend/components/modal.tsx`; Modify `frontend/lib/api.ts`, `frontend/lib/api.test.ts`

- [ ] **Step 1: 加 shadcn select（前台运行）**

Run: `cd frontend && npx --yes shadcn@latest add select --yes`
确认生成 `frontend/components/ui/select.tsx`（导出 `Select, SelectTrigger, SelectValue, SelectContent, SelectItem`）。`git status` 确认无 node_modules 待提交。

- [ ] **Step 2: 创建 `frontend/components/modal.tsx`**
```tsx
"use client";

import { useEffect } from "react";

export function Modal({
  open,
  onClose,
  title,
  children,
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  children: React.ReactNode;
}) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
    >
      <div
        className="w-full max-w-lg rounded-lg border border-border bg-card p-6"
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="mb-4 text-lg font-normal tracking-tight">{title}</h2>
        {children}
      </div>
    </div>
  );
}
```

- [ ] **Step 3: 追加 api 失败测试**（在 `api.test.ts` describe 内）
```ts
  it("listUsers + createUser", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response(JSON.stringify([{ id: 1, name: "a", uuid: "u", password: "p", inboundIds: [], inboundTags: [] }]), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ id: 2 }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    const us = await listUsers();
    expect(us[0].password).toBe("p");
    await createUser("bob", [1, 2]);
    const [, init] = fetchMock.mock.calls[1];
    expect(JSON.parse(init.body).inboundIds).toEqual([1, 2]);
  });

  it("updateInbound + resetInboundKeys", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: 1 }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    await updateInbound(1, "t", 443, { handshake: "x" });
    expect(fetchMock.mock.calls[0][1].method).toBe("PUT");
    await resetInboundKeys(1);
    expect(fetchMock.mock.calls[1][0]).toContain("/reset-keys");
  });
```
import 行增补：
```ts
import { login, getStatus, UnauthorizedError, listUsers, createUser, updateInbound, resetInboundKeys } from "./api";
```

- [ ] **Step 4: 运行确认失败**

Run: `cd frontend && npm test -- lib/api`
Expected: FAIL（未导出）

- [ ] **Step 5: 改 `frontend/lib/api.ts`**
(a) 把 `Inbound` 接口的 `users?` 去掉（卡片不再展示用户），并新增 `User` 类型：
```ts
export interface Inbound {
  id: number;
  type: string;
  tag: string;
  port: number;
  network: string;
  publicInfo: Record<string, unknown>;
}

export interface User {
  id: number;
  name: string;
  uuid: string;
  password: string;
  inboundIds: number[];
  inboundTags: string[];
}
```
(b) 删除旧的 `listUsers(inboundID)`、`createUser(inboundID,name)`、`deleteUser(id)`（M4 内联用户）签名，替换为全局版 + 入站 update/reset：
```ts
export async function listUsers(): Promise<User[]> {
  const res = await request("/api/users");
  if (!res.ok) throw new Error("加载用户失败");
  return res.json();
}

export async function createUser(name: string, inboundIds: number[]): Promise<User> {
  const res = await request("/api/users", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, inboundIds }),
  });
  if (!res.ok) {
    const b = await res.json().catch(() => ({}));
    throw new Error(b.error || "创建用户失败");
  }
  return res.json();
}

export async function updateUser(id: number, name: string, inboundIds: number[]): Promise<User> {
  const res = await request(`/api/users/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, inboundIds }),
  });
  if (!res.ok) throw new Error("更新用户失败");
  return res.json();
}

export async function resetUserCreds(id: number): Promise<User> {
  const res = await request(`/api/users/${id}/reset`, { method: "POST" });
  if (!res.ok) throw new Error("重置凭证失败");
  return res.json();
}

export async function deleteUser(id: number): Promise<void> {
  const res = await request(`/api/users/${id}`, { method: "DELETE" });
  if (!res.ok) throw new Error("删除用户失败");
}

export async function updateInbound(id: number, tag: string, port: number, params: Record<string, unknown>): Promise<Inbound> {
  const res = await request(`/api/inbounds/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ tag, port, params }),
  });
  if (!res.ok) {
    const b = await res.json().catch(() => ({}));
    throw new Error(b.error || "更新入站失败");
  }
  return res.json();
}

export async function resetInboundKeys(id: number): Promise<Inbound> {
  const res = await request(`/api/inbounds/${id}/reset-keys`, { method: "POST" });
  if (!res.ok) throw new Error("重置密钥失败");
  return res.json();
}
```
保留 `listInbounds`、`createInbound`、`deleteInbound`、`listInboundTypes`、`applySingbox`、`getStatus` 等。

- [ ] **Step 6: 运行确认通过**

Run: `cd frontend && npm test -- lib/api`
Expected: PASS。（`app/inbounds/page.test.tsx`、`app/users` 在后续任务更新；若 `npm test` 全量此时仅这些页面失败属预期——本任务只需 lib/api 绿。先不跑全量或忽略页面失败。）

- [ ] **Step 7: 提交**

```bash
git add frontend/components/ui/select.tsx frontend/components/modal.tsx frontend/lib/api.ts frontend/lib/api.test.ts frontend/components.json frontend/package.json frontend/package-lock.json
git commit -m "feat(frontend): shadcn select, modal component, global-user + inbound-edit api"
```

---

## Task 5: 入站页 — modal 新建/编辑 + Select + 重置密钥确认

**Files:** Rewrite `frontend/app/inbounds/page.tsx`, `frontend/app/inbounds/page.test.tsx`

- [ ] **Step 1: 整体替换 `page.test.tsx`**
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
const listTypesMock = vi.fn();
const createInboundMock = vi.fn();
const resetKeysMock = vi.fn();
const deleteInboundMock = vi.fn();
vi.mock("@/lib/api", () => ({
  listInbounds: () => listInboundsMock(),
  listInboundTypes: () => listTypesMock(),
  createInbound: (...a: unknown[]) => createInboundMock(...a),
  updateInbound: vi.fn(),
  resetInboundKeys: (...a: unknown[]) => resetKeysMock(...a),
  deleteInbound: (...a: unknown[]) => deleteInboundMock(...a),
  getStatus: vi.fn().mockResolvedValue({ installed: true, version: "1.13.13", running: false, hasConfig: true }),
  logout: vi.fn(),
  UnauthorizedError: class extends Error {},
}));

beforeEach(() => {
  listInboundsMock.mockReset().mockResolvedValue([]);
  listTypesMock.mockReset().mockResolvedValue([
    { type: "vless-reality", label: "VLESS-Reality", network: "tcp", defaultPort: 8443 },
    { type: "hysteria2", label: "Hysteria2", network: "udp", defaultPort: 443 },
  ]);
  createInboundMock.mockReset().mockResolvedValue({ id: 1 });
  resetKeysMock.mockReset().mockResolvedValue({ id: 1 });
  deleteInboundMock.mockReset().mockResolvedValue(undefined);
});

describe("InboundsPage", () => {
  it("点新建入站打开 modal 并能提交（默认协议）", async () => {
    render(<InboundsPage />);
    await userEvent.click(await screen.findByRole("button", { name: /新建入站/ }));
    await userEvent.type(screen.getByLabelText(/标签/), "v9");
    await userEvent.click(screen.getByRole("button", { name: /创建/ }));
    await waitFor(() => expect(createInboundMock).toHaveBeenCalled());
    expect(createInboundMock.mock.calls[0][0]).toBe("vless-reality");
  });

  it("重置密钥需二次确认", async () => {
    listInboundsMock.mockResolvedValue([
      { id: 1, type: "vless-reality", tag: "v1", port: 8443, network: "tcp", publicInfo: { realityPublicKey: "PUB" } },
    ]);
    render(<InboundsPage />);
    await waitFor(() => expect(screen.getByText("v1")).toBeInTheDocument());
    await userEvent.click(screen.getByRole("button", { name: /重置密钥/ }));
    // confirm modal appears; resetKeys not called yet
    expect(resetKeysMock).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: /确认重置/ }));
    await waitFor(() => expect(resetKeysMock).toHaveBeenCalledWith(1));
  });
});
```

- [ ] **Step 2: 运行确认失败**

Run: `cd frontend && npm test -- app/inbounds`
Expected: FAIL。

- [ ] **Step 3: 整体替换 `page.tsx`**
```tsx
"use client";

import { useCallback, useEffect, useState } from "react";
import {
  listInbounds, listInboundTypes, createInbound, updateInbound, resetInboundKeys, deleteInbound,
  Inbound, InboundType,
} from "@/lib/api";
import { AppShell } from "@/components/app-shell";
import { Modal } from "@/components/modal";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from "@/components/ui/select";

type FormState = {
  id?: number;
  type: string;
  tag: string;
  port: string;
  handshake: string;
  handshakePort: string;
  sni: string;
  up: string;
  down: string;
};

const emptyForm: FormState = {
  type: "vless-reality", tag: "", port: "8443",
  handshake: "www.microsoft.com", handshakePort: "443",
  sni: "bing.com", up: "100", down: "100",
};

export default function InboundsPage() {
  const [inbounds, setInbounds] = useState<Inbound[]>([]);
  const [types, setTypes] = useState<InboundType[]>([]);
  const [form, setForm] = useState<FormState | null>(null); // open modal when non-null
  const [confirmReset, setConfirmReset] = useState<Inbound | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(() => {
    listInbounds().then(setInbounds).catch(() => setError("加载入站失败"));
  }, []);
  useEffect(() => {
    refresh();
    listInboundTypes().then(setTypes).catch(() => {});
  }, [refresh]);

  function openCreate() {
    setError("");
    setForm({ ...emptyForm });
  }
  function openEdit(ib: Inbound) {
    setError("");
    const pi = ib.publicInfo;
    setForm({
      id: ib.id, type: ib.type, tag: ib.tag, port: String(ib.port),
      handshake: String(pi.serverName ?? "www.microsoft.com"),
      handshakePort: "443",
      sni: String(pi.serverName ?? "bing.com"),
      up: String(pi.upMbps ?? "100"),
      down: String(pi.downMbps ?? "100"),
    });
  }

  function setType(t: string) {
    if (!form) return;
    const info = types.find((x) => x.type === t);
    setForm({ ...form, type: t, port: info ? String(info.defaultPort) : form.port });
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!form) return;
    setBusy(true);
    setError("");
    const params: Record<string, unknown> =
      form.type === "hysteria2"
        ? { serverName: form.sni, upMbps: Number(form.up), downMbps: Number(form.down) }
        : { handshake: form.handshake, handshakePort: Number(form.handshakePort) };
    try {
      if (form.id) await updateInbound(form.id, form.tag, Number(form.port), params);
      else await createInbound(form.type, form.tag, Number(form.port), params);
      setForm(null);
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存失败");
    } finally {
      setBusy(false);
    }
  }

  async function doReset() {
    if (!confirmReset) return;
    await resetInboundKeys(confirmReset.id);
    setConfirmReset(null);
    refresh();
  }
  async function onDelete(id: number) {
    await deleteInbound(id);
    refresh();
  }

  const isEdit = !!form?.id;

  return (
    <AppShell>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-normal tracking-tight">入站</h1>
        <Button className="rounded-full" onClick={openCreate}>新建入站</Button>
      </div>

      <div className="space-y-4">
        {inbounds.map((ib) => (
          <Card key={ib.id} className="rounded-lg">
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle className="text-base font-normal">
                  {ib.tag} <span className="text-muted-foreground">· {ib.type} · :{ib.port}/{ib.network}</span>
                </CardTitle>
                <div className="flex gap-2">
                  <Button variant="outline" className="rounded-full" onClick={() => openEdit(ib)}>编辑</Button>
                  <Button variant="outline" className="rounded-full" onClick={() => setConfirmReset(ib)}>重置密钥</Button>
                  <Button variant="outline" className="rounded-full" onClick={() => onDelete(ib.id)}>删除</Button>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <div className="divide-y divide-border">
                {Object.entries(ib.publicInfo).map(([k, v]) => (
                  <div key={k} className="flex justify-between gap-4 py-1">
                    <span className="font-mono text-xs text-muted-foreground uppercase">{k}</span>
                    <span className="truncate font-mono text-xs">{String(v)}</span>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        ))}
        {inbounds.length === 0 && <p className="text-sm text-muted-foreground">暂无入站，点右上角新建。</p>}
      </div>

      <Modal open={form !== null} onClose={() => setForm(null)} title={isEdit ? "编辑入站" : "新建入站"}>
        {form && (
          <form onSubmit={submit} className="space-y-4">
            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground">协议</Label>
              <Select value={form.type} onValueChange={setType} disabled={isEdit}>
                <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  {types.map((t) => (
                    <SelectItem key={t.type} value={t.type}>{t.label}</SelectItem>
                  ))}
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
            {form.type === "hysteria2" ? (
              <div className="flex gap-4">
                <div className="flex-1 space-y-2">
                  <Label htmlFor="sni" className="text-xs text-muted-foreground">SNI</Label>
                  <Input id="sni" value={form.sni} onChange={(e) => setForm({ ...form, sni: e.target.value })} />
                </div>
                <div className="w-24 space-y-2">
                  <Label htmlFor="up" className="text-xs text-muted-foreground">上行</Label>
                  <Input id="up" value={form.up} onChange={(e) => setForm({ ...form, up: e.target.value })} />
                </div>
                <div className="w-24 space-y-2">
                  <Label htmlFor="down" className="text-xs text-muted-foreground">下行</Label>
                  <Input id="down" value={form.down} onChange={(e) => setForm({ ...form, down: e.target.value })} />
                </div>
              </div>
            ) : (
              <div className="flex gap-4">
                <div className="flex-1 space-y-2">
                  <Label htmlFor="hs" className="text-xs text-muted-foreground">握手域名</Label>
                  <Input id="hs" value={form.handshake} onChange={(e) => setForm({ ...form, handshake: e.target.value })} />
                </div>
                <div className="w-28 space-y-2">
                  <Label htmlFor="hsp" className="text-xs text-muted-foreground">握手端口</Label>
                  <Input id="hsp" value={form.handshakePort} onChange={(e) => setForm({ ...form, handshakePort: e.target.value })} />
                </div>
              </div>
            )}
            {error && <p className="text-sm text-destructive">{error}</p>}
            <div className="flex justify-end gap-3 pt-2">
              <Button type="button" variant="outline" className="rounded-full" onClick={() => setForm(null)}>取消</Button>
              <Button type="submit" className="rounded-full" disabled={busy}>{isEdit ? "保存" : "创建"}</Button>
            </div>
          </form>
        )}
      </Modal>

      <Modal open={confirmReset !== null} onClose={() => setConfirmReset(null)} title="重置密钥">
        <p className="mb-4 text-sm text-muted-foreground">
          重置后会生成新的 reality 密钥 / 证书，**已分发的旧客户端将失效**。确定继续？
        </p>
        <div className="flex justify-end gap-3">
          <Button variant="outline" className="rounded-full" onClick={() => setConfirmReset(null)}>取消</Button>
          <Button className="rounded-full" onClick={doReset}>确认重置</Button>
        </div>
      </Modal>
    </AppShell>
  );
}
```

- [ ] **Step 4: 运行确认通过 + 构建**

Run: `cd frontend && npm test -- app/inbounds && npm run build`
Expected: app/inbounds 通过；构建成功。

- [ ] **Step 5: 提交**

```bash
git add frontend/app/inbounds
git commit -m "feat(frontend): inbounds via modal (create/edit, shadcn select, reset-keys confirm)"
```

---

## Task 6: 用户页 + 侧边栏「用户」

**Files:** Create `frontend/app/users/page.tsx`+test; Modify `frontend/components/app-shell.tsx`+test

- [ ] **Step 1: 改 app-shell 导航 + 测试**

`app-shell.test.tsx` 的 nav 测试体追加 `用户` 断言：
```tsx
  it("renders nav items 概览 入站 用户 配置", () => {
    render(<AppShell><div>x</div></AppShell>);
    expect(screen.getByRole("link", { name: /概览/ })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /入站/ })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /用户/ })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /配置/ })).toBeInTheDocument();
  });
```
`app-shell.tsx` 的 `NAV` 改为：
```tsx
const NAV = [
  { href: "/dashboard", label: "概览" },
  { href: "/inbounds", label: "入站" },
  { href: "/users", label: "用户" },
  { href: "/config", label: "配置" },
];
```

- [ ] **Step 2: 写 users 失败测试 `frontend/app/users/page.test.tsx`**
```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import UsersPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/users",
}));

const listUsersMock = vi.fn();
const listInboundsMock = vi.fn();
const createUserMock = vi.fn();
vi.mock("@/lib/api", () => ({
  listUsers: () => listUsersMock(),
  listInbounds: () => listInboundsMock(),
  createUser: (...a: unknown[]) => createUserMock(...a),
  updateUser: vi.fn(),
  resetUserCreds: vi.fn(),
  deleteUser: vi.fn(),
  getStatus: vi.fn().mockResolvedValue({ installed: true, version: "1.13.13", running: false, hasConfig: true }),
  logout: vi.fn(),
  UnauthorizedError: class extends Error {},
}));

beforeEach(() => {
  listUsersMock.mockReset().mockResolvedValue([]);
  listInboundsMock.mockReset().mockResolvedValue([
    { id: 1, type: "vless-reality", tag: "v1", port: 8443, network: "tcp", publicInfo: {} },
  ]);
  createUserMock.mockReset().mockResolvedValue({ id: 1 });
});

describe("UsersPage", () => {
  it("列表显示用户的 UUID 与明文密码", async () => {
    listUsersMock.mockResolvedValue([
      { id: 1, name: "alice", uuid: "the-uuid", password: "the-pass", inboundIds: [1], inboundTags: ["v1"] },
    ]);
    render(<UsersPage />);
    await waitFor(() => expect(screen.getByText("alice")).toBeInTheDocument());
    expect(screen.getByText("the-uuid")).toBeInTheDocument();
    expect(screen.getByText("the-pass")).toBeInTheDocument();
  });

  it("新建用户 modal 勾选入站并提交", async () => {
    render(<UsersPage />);
    await userEvent.click(await screen.findByRole("button", { name: /新建用户/ }));
    await userEvent.type(screen.getByLabelText(/名称/), "bob");
    await userEvent.click(screen.getByLabelText(/v1/));
    await userEvent.click(screen.getByRole("button", { name: /创建/ }));
    await waitFor(() => expect(createUserMock).toHaveBeenCalled());
    expect(createUserMock.mock.calls[0][0]).toBe("bob");
    expect(createUserMock.mock.calls[0][1]).toEqual([1]);
  });
});
```

- [ ] **Step 3: 运行确认失败**

Run: `cd frontend && npm test -- app/users components/app-shell`
Expected: FAIL（页面不存在 / 导航无用户）

- [ ] **Step 4: 创建 `frontend/app/users/page.tsx`**
```tsx
"use client";

import { useCallback, useEffect, useState } from "react";
import {
  listUsers, listInbounds, createUser, updateUser, resetUserCreds, deleteUser,
  User, Inbound,
} from "@/lib/api";
import { AppShell } from "@/components/app-shell";
import { Modal } from "@/components/modal";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent } from "@/components/ui/card";

type Form = { id?: number; name: string; inboundIds: number[] };

export default function UsersPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [inbounds, setInbounds] = useState<Inbound[]>([]);
  const [form, setForm] = useState<Form | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(() => {
    listUsers().then(setUsers).catch(() => setError("加载用户失败"));
  }, []);
  useEffect(() => {
    refresh();
    listInbounds().then(setInbounds).catch(() => {});
  }, [refresh]);

  function toggle(id: number) {
    if (!form) return;
    const has = form.inboundIds.includes(id);
    setForm({ ...form, inboundIds: has ? form.inboundIds.filter((x) => x !== id) : [...form.inboundIds, id] });
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!form) return;
    setBusy(true);
    setError("");
    try {
      if (form.id) await updateUser(form.id, form.name, form.inboundIds);
      else await createUser(form.name, form.inboundIds);
      setForm(null);
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存失败");
    } finally {
      setBusy(false);
    }
  }

  async function onReset(id: number) {
    await resetUserCreds(id);
    refresh();
  }
  async function onDelete(id: number) {
    await deleteUser(id);
    refresh();
  }

  return (
    <AppShell>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-normal tracking-tight">用户</h1>
        <Button className="rounded-full" onClick={() => { setError(""); setForm({ name: "", inboundIds: [] }); }}>
          新建用户
        </Button>
      </div>

      <Card className="rounded-lg">
        <CardContent className="pt-4">
          <div className="divide-y divide-border">
            <div className="grid grid-cols-[1fr_2fr_2fr_2fr_auto] gap-4 py-2 font-mono text-xs text-muted-foreground uppercase">
              <span>名称</span><span>UUID</span><span>密码</span><span>入站</span><span></span>
            </div>
            {users.map((u) => (
              <div key={u.id} className="grid grid-cols-[1fr_2fr_2fr_2fr_auto] items-center gap-4 py-3 text-sm">
                <span>{u.name}</span>
                <span className="truncate font-mono text-xs">{u.uuid}</span>
                <span className="truncate font-mono text-xs">{u.password}</span>
                <span className="truncate text-xs text-muted-foreground">{u.inboundTags.join(", ") || "—"}</span>
                <span className="flex gap-2">
                  <Button variant="outline" className="rounded-full" onClick={() => { setError(""); setForm({ id: u.id, name: u.name, inboundIds: u.inboundIds }); }}>编辑</Button>
                  <Button variant="outline" className="rounded-full" onClick={() => onReset(u.id)}>重置凭证</Button>
                  <Button variant="outline" className="rounded-full" onClick={() => onDelete(u.id)}>删除</Button>
                </span>
              </div>
            ))}
            {users.length === 0 && <p className="py-3 text-sm text-muted-foreground">暂无用户。</p>}
          </div>
        </CardContent>
      </Card>

      <Modal open={form !== null} onClose={() => setForm(null)} title={form?.id ? "编辑用户" : "新建用户"}>
        {form && (
          <form onSubmit={submit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="name" className="text-xs text-muted-foreground">名称</Label>
              <Input id="name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
            </div>
            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground">所属入站</Label>
              <div className="space-y-1">
                {inbounds.map((ib) => (
                  <label key={ib.id} className="flex items-center gap-2 text-sm">
                    <input
                      type="checkbox"
                      aria-label={ib.tag}
                      checked={form.inboundIds.includes(ib.id)}
                      onChange={() => toggle(ib.id)}
                    />
                    {ib.tag} <span className="text-muted-foreground">· {ib.type}</span>
                  </label>
                ))}
                {inbounds.length === 0 && <p className="text-xs text-muted-foreground">还没有入站</p>}
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
    </AppShell>
  );
}
```

- [ ] **Step 5: 运行确认通过 + 全量 + 构建**

Run: `cd frontend && npm test && npm run build`
Expected: 全 PASS；构建成功（路由含 /users）。

- [ ] **Step 6: 提交**

```bash
git add frontend/app/users frontend/components/app-shell.tsx frontend/components/app-shell.test.tsx
git commit -m "feat(frontend): global users page + 用户 nav"
```

---

## Task 7: 全量验证 + 截图

**Files:** 无

- [ ] **Step 1:** `cd backend && go test ./...` → 全 PASS。
- [ ] **Step 2:** `cd frontend && npm test && npm run build` → 全 PASS、构建成功。
- [ ] **Step 3:** `make build` 出单二进制，临时 DB/SINGBOX_DIR 起在 :8080，浏览器登录后：
  - `/inbounds`：点「新建入站」开 modal、协议 Select 切 vless/hy2 看字段变化；对默认入站点「编辑」（协议只读）、「重置密钥」走二次确认。
  - `/users`：新建用户勾选两个入站、看列表显示 UUID + 明文密码 + 入站标签；重置凭证。
  - 截图 `/inbounds`、`/users`；清理进程与临时文件。
- [ ] **Step 4:** 如有样式微调 `git add -A && git commit -m "chore(frontend): M5 polish" || echo "no changes"`

---

## Self-Review Notes

- **Spec 覆盖**：driver 三方法(T1)、User 全局 m2m 模型(T2)、generate 按协议取凭证(T2)、service 用户 CRUD+reset / 入站 update+reset-keys(T2)、handlers 拆用户+入站新端点(T2)、main 路由(T3)、shadcn select + Modal + api(T4)、入站 modal/编辑/重置确认(T5)、用户页+导航(T6)、验证(T7)。用户列表显 UUID+明文密码(T6 测试断言)、reset-keys 二次确认(T5)、卡片不显示用户(T5 卡片只渲染 publicInfo)、协议 select 用 shadcn(T4/T5)、type 不可改(T5 编辑时 Select disabled + 后端 UpdateInbound 不改 type)。
- **类型一致性**：`Driver` 新增 `CredentialKind/UpdateSettings/ResetSecrets`；`UserView{id,name,uuid,password,inboundIds,inboundTags}` ↔ 前端 `User`；`InboundView` 去 users ↔ 前端 `Inbound` 去 users；service `CreateUser(name,[]uint)/UpdateUser/ResetUserCreds/DeleteUser`、`UpdateInbound/ResetInboundKeys` 与 `UserController`/`InboundController`、handler body、前端 api 一致；端口/标签编辑校验排除自身。
- **执行注意**：T2 改 service 签名 + 删 M4 内联用户方法/路由依赖，main.go 暂编译失败属预期（T2 只验 ./internal/...，T3 修）。T2 又一次结构性改表（User 去 InboundID/Credential 加 UUID/Password/m2m），预发布期重建库。shadcn Select 是 base-ui 自定义组件，测试避免直接驱动其下拉（T5 测试只用默认协议提交 + 重置确认，协议切换由实机截图验证）。T4 前端中途 page 测试会暂失败，仅需 lib/api 绿。
```

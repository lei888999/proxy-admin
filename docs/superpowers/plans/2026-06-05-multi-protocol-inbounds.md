# M4: 多协议入站（driver 注册表 + Hysteria2）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把入站改造成多协议（协议注册表/driver），新增 Hysteria2，部署后默认 seed 一个 VLESS-Reality(TCP 8443) 和一个 Hysteria2(UDP 443) 入站。

**Architecture:** Driver 设计成**与 DB 模型解耦的纯逻辑**（输入 tag/port/settings/users，不依赖 GORM model），先独立建好测好；随后一次性把 `Inbound` 模型（type/network/settings JSON）、generate、service、handlers 切到注册表。证书自签由 Go 生成。

**Tech Stack:** Go 1.26 + Gin + GORM + crypto/x509(ECDSA) + golang.org/x/crypto/curve25519；Next.js 16 + TS + Tailwind + Vitest。

参考规范：`docs/superpowers/specs/2026-06-05-multi-protocol-inbounds-design.md`

---

## File Structure

后端 `internal/inbound/`：
- `cert.go`（新）— 自签 ECDSA 证书
- `driver.go`（新）— Driver 接口 + 注册表 + `Cred`/`TypeInfo`/`Get`/`Types`/`ErrUnknownType`
- `driver_vless.go`（新）— vless-reality driver（复用 `keygen.go`）
- `driver_hysteria2.go`（新）— hysteria2 driver
- `generate.go`（重写）— 走注册表
- `service.go`（重写）— CreateInbound(type,tag,port,params)、ListInboundViews、端口冲突按 (port,network)
- `seed.go`（新）— SeedDefaults
- `keygen.go`（沿用）
其它：
- `backend/internal/models/inbound.go`（重写）— type/network/settings + User.Credential
- `backend/internal/handlers/inbound.go`（改）— controller 新签名 + `/inbound-types`
- `backend/cmd/server/main.go`（改）— seed 接线
前端：`frontend/lib/api.ts`（改）、`frontend/app/inbounds/page.tsx`+`page.test.tsx`（改）。

---

## Task 1: 自签证书 cert.go

**Files:** Create `backend/internal/inbound/cert.go`, `backend/internal/inbound/cert_test.go`

- [ ] **Step 1: 写失败测试**

```go
package inbound

import (
	"crypto/x509"
	"encoding/pem"
	"testing"
)

func TestSelfSignedCert(t *testing.T) {
	certPEM, keyPEM, err := selfSignedCert("bing.com")
	if err != nil {
		t.Fatalf("selfSignedCert: %v", err)
	}
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		t.Fatal("cert not PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	found := false
	for _, n := range cert.DNSNames {
		if n == "bing.com" {
			found = true
		}
	}
	if !found {
		t.Fatalf("SAN missing bing.com: %v", cert.DNSNames)
	}
	if block, _ := pem.Decode([]byte(keyPEM)); block == nil {
		t.Fatal("key not PEM")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/inbound/ -run TestSelfSignedCert`
Expected: FAIL（selfSignedCert 未定义）

- [ ] **Step 3: 实现**

```go
package inbound

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"
)

// selfSignedCert returns a fresh self-signed ECDSA P-256 cert+key (PEM) for sni.
func selfSignedCert(sni string) (certPEM, keyPEM string, err error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", err
	}
	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: sni},
		DNSNames:     []string{sni},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		return "", "", err
	}
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return "", "", err
	}
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	return certPEM, keyPEM, nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd backend && go test ./internal/inbound/ -run TestSelfSignedCert`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/inbound/cert.go backend/internal/inbound/cert_test.go
git commit -m "feat(inbound): self-signed ECDSA cert generation"
```

---

## Task 2: Driver 接口 + 注册表 + 两个 driver

**Files:** Create `driver.go`, `driver_vless.go`, `driver_hysteria2.go`, `driver_test.go` (all in `backend/internal/inbound/`)

> Driver 与 DB 模型解耦：输入 `tag/port/settings/users`，输出 sing-box 入站 map。`keygen.go`（`KeyGen`/`NewKeyGen`）已存在。

- [ ] **Step 1: 写失败测试 `driver_test.go`**

```go
package inbound

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRegistryTypes(t *testing.T) {
	ts := Types()
	if len(ts) != 2 {
		t.Fatalf("types = %d, want 2", len(ts))
	}
	if ts[0].Type != "vless-reality" || ts[0].Network != "tcp" || ts[0].DefaultPort != 8443 {
		t.Fatalf("vless typeinfo: %+v", ts[0])
	}
	if ts[1].Type != "hysteria2" || ts[1].Network != "udp" || ts[1].DefaultPort != 443 {
		t.Fatalf("hy2 typeinfo: %+v", ts[1])
	}
}

func TestVlessDriver(t *testing.T) {
	d, ok := Get("vless-reality")
	if !ok {
		t.Fatal("vless driver not registered")
	}
	s, err := d.BuildSettings(map[string]any{"handshake": "example.com"})
	if err != nil {
		t.Fatalf("BuildSettings: %v", err)
	}
	in, err := d.BuildInbound("v1", 8443, s, []Cred{{Name: "a", Credential: "uuid-1"}})
	if err != nil {
		t.Fatalf("BuildInbound: %v", err)
	}
	if in["type"] != "vless" {
		t.Fatalf("type=%v", in["type"])
	}
	reality := in["tls"].(map[string]any)["reality"].(map[string]any)
	if reality["private_key"] == "" || reality["private_key"] == nil {
		t.Fatal("missing private_key")
	}
	users := in["users"].([]map[string]any)
	if users[0]["uuid"] != "uuid-1" {
		t.Fatalf("user uuid=%v", users[0]["uuid"])
	}
	pi, _ := d.PublicInfo(s)
	if pi["serverName"] != "example.com" {
		t.Fatalf("publicInfo serverName=%v", pi["serverName"])
	}
	if strings.Contains(toJSON(pi), "realityPrivateKey") {
		t.Fatal("publicInfo leaked private key")
	}
}

func TestHysteria2Driver(t *testing.T) {
	d, ok := Get("hysteria2")
	if !ok {
		t.Fatal("hy2 driver not registered")
	}
	s, err := d.BuildSettings(map[string]any{"serverName": "bing.com", "upMbps": float64(50), "downMbps": float64(200)})
	if err != nil {
		t.Fatalf("BuildSettings: %v", err)
	}
	in, err := d.BuildInbound("h1", 443, s, []Cred{{Name: "a", Credential: "pass1"}})
	if err != nil {
		t.Fatalf("BuildInbound: %v", err)
	}
	if in["type"] != "hysteria2" || in["up_mbps"] != 50 || in["down_mbps"] != 200 {
		t.Fatalf("hy2 inbound: %+v", in)
	}
	tls := in["tls"].(map[string]any)
	if tls["alpn"].([]string)[0] != "h3" {
		t.Fatalf("alpn=%v", tls["alpn"])
	}
	users := in["users"].([]map[string]any)
	if users[0]["password"] != "pass1" {
		t.Fatalf("user password=%v", users[0]["password"])
	}
	pi, _ := d.PublicInfo(s)
	if pi["insecure"] != true || pi["serverName"] != "bing.com" {
		t.Fatalf("publicInfo=%v", pi)
	}
	if strings.Contains(toJSON(pi), "keyPEM") {
		t.Fatal("publicInfo leaked key")
	}
}

func toJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/inbound/ -run 'TestRegistry|TestVlessDriver|TestHysteria2Driver'`
Expected: FAIL（未定义）

- [ ] **Step 3: 实现 `driver.go`**

```go
package inbound

import "errors"

var ErrUnknownType = errors.New("unknown inbound type")

// Cred is one user's credential (uuid for vless, password for hysteria2).
type Cred struct {
	Name       string
	Credential string
}

// Driver encapsulates one protocol. It is decoupled from the DB model.
type Driver interface {
	Type() string
	Label() string
	Network() string
	DefaultPort() uint16
	BuildSettings(params map[string]any) (string, error) // generate secrets + apply params/defaults -> settings JSON
	NewCredential() string
	BuildInbound(tag string, port uint16, settings string, users []Cred) (map[string]any, error)
	PublicInfo(settings string) (map[string]any, error)
}

type TypeInfo struct {
	Type        string `json:"type"`
	Label       string `json:"label"`
	Network     string `json:"network"`
	DefaultPort uint16 `json:"defaultPort"`
}

var registry = map[string]Driver{}

func register(d Driver) { registry[d.Type()] = d }

func Get(t string) (Driver, bool) { d, ok := registry[t]; return d, ok }

// Types returns registered drivers in a stable order (for the UI dropdown).
func Types() []TypeInfo {
	out := []TypeInfo{}
	for _, t := range []string{"vless-reality", "hysteria2"} {
		if d, ok := registry[t]; ok {
			out = append(out, TypeInfo{Type: d.Type(), Label: d.Label(), Network: d.Network(), DefaultPort: d.DefaultPort()})
		}
	}
	return out
}
```

- [ ] **Step 4: 实现 `driver_vless.go`**

```go
package inbound

import "encoding/json"

type vlessSettings struct {
	RealityPrivateKey string `json:"realityPrivateKey"`
	RealityPublicKey  string `json:"realityPublicKey"`
	ShortID           string `json:"shortId"`
	Handshake         string `json:"handshake"`
	HandshakePort     uint16 `json:"handshakePort"`
	ServerName        string `json:"serverName"`
	Flow              string `json:"flow"`
}

type vlessReality struct{ kg KeyGen }

func init() { register(vlessReality{kg: NewKeyGen()}) }

func (vlessReality) Type() string        { return "vless-reality" }
func (vlessReality) Label() string       { return "VLESS-Reality" }
func (vlessReality) Network() string     { return "tcp" }
func (vlessReality) DefaultPort() uint16 { return 8443 }

func (d vlessReality) BuildSettings(params map[string]any) (string, error) {
	priv, pub, err := d.kg.RealityKeypair()
	if err != nil {
		return "", err
	}
	s := vlessSettings{
		RealityPrivateKey: priv, RealityPublicKey: pub, ShortID: d.kg.ShortID(),
		Handshake: "www.microsoft.com", HandshakePort: 443, Flow: "xtls-rprx-vision",
	}
	if v, ok := params["handshake"].(string); ok && v != "" {
		s.Handshake = v
	}
	s.ServerName = s.Handshake
	b, err := json.Marshal(s)
	return string(b), err
}

func (d vlessReality) NewCredential() string { return d.kg.UUID() }

func (vlessReality) BuildInbound(tag string, port uint16, settings string, users []Cred) (map[string]any, error) {
	var s vlessSettings
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		return nil, err
	}
	us := []map[string]any{}
	for _, u := range users {
		us = append(us, map[string]any{"name": u.Name, "uuid": u.Credential, "flow": s.Flow})
	}
	return map[string]any{
		"type": "vless", "tag": tag, "listen": "::", "listen_port": port,
		"users": us,
		"tls": map[string]any{
			"enabled": true, "server_name": s.ServerName,
			"reality": map[string]any{
				"enabled":     true,
				"handshake":   map[string]any{"server": s.Handshake, "server_port": s.HandshakePort},
				"private_key": s.RealityPrivateKey,
				"short_id":    []string{s.ShortID},
			},
		},
	}, nil
}

func (vlessReality) PublicInfo(settings string) (map[string]any, error) {
	var s vlessSettings
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		return nil, err
	}
	return map[string]any{
		"realityPublicKey": s.RealityPublicKey, "shortId": s.ShortID,
		"serverName": s.ServerName, "flow": s.Flow,
	}, nil
}
```

- [ ] **Step 5: 实现 `driver_hysteria2.go`**

```go
package inbound

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
)

type hy2Settings struct {
	ServerName string `json:"serverName"`
	CertPEM    string `json:"certPEM"`
	KeyPEM     string `json:"keyPEM"`
	UpMbps     int    `json:"upMbps"`
	DownMbps   int    `json:"downMbps"`
}

type hysteria2 struct{}

func init() { register(hysteria2{}) }

func (hysteria2) Type() string        { return "hysteria2" }
func (hysteria2) Label() string       { return "Hysteria2" }
func (hysteria2) Network() string     { return "udp" }
func (hysteria2) DefaultPort() uint16 { return 443 }

func (hysteria2) BuildSettings(params map[string]any) (string, error) {
	sni := "bing.com"
	if v, ok := params["serverName"].(string); ok && v != "" {
		sni = v
	}
	cert, key, err := selfSignedCert(sni)
	if err != nil {
		return "", err
	}
	s := hy2Settings{ServerName: sni, CertPEM: cert, KeyPEM: key, UpMbps: 100, DownMbps: 100}
	if v, ok := toInt(params["upMbps"]); ok {
		s.UpMbps = v
	}
	if v, ok := toInt(params["downMbps"]); ok {
		s.DownMbps = v
	}
	b, err := json.Marshal(s)
	return string(b), err
}

func (hysteria2) NewCredential() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (hysteria2) BuildInbound(tag string, port uint16, settings string, users []Cred) (map[string]any, error) {
	var s hy2Settings
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		return nil, err
	}
	us := []map[string]any{}
	for _, u := range users {
		us = append(us, map[string]any{"name": u.Name, "password": u.Credential})
	}
	in := map[string]any{
		"type": "hysteria2", "tag": tag, "listen": "::", "listen_port": port,
		"users": us,
		"tls": map[string]any{
			"enabled": true, "server_name": s.ServerName, "alpn": []string{"h3"},
			"certificate": []string{s.CertPEM}, "key": []string{s.KeyPEM},
		},
	}
	if s.UpMbps > 0 {
		in["up_mbps"] = s.UpMbps
	}
	if s.DownMbps > 0 {
		in["down_mbps"] = s.DownMbps
	}
	return in, nil
}

func (hysteria2) PublicInfo(settings string) (map[string]any, error) {
	var s hy2Settings
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		return nil, err
	}
	return map[string]any{
		"serverName": s.ServerName, "upMbps": s.UpMbps, "downMbps": s.DownMbps, "insecure": true,
	}, nil
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	}
	return 0, false
}
```

- [ ] **Step 6: 运行确认通过 + vet**

Run: `cd backend && go test ./internal/inbound/ && go vet ./internal/inbound/`
Expected: 全 PASS（cert + driver；旧 generate/service 仍按旧 model，本任务未动它们）

> 注意：本任务不改 `models`、`generate.go`、`service.go`。它们仍用 M3 旧字段，包仍编译通过。

- [ ] **Step 7: 提交**

```bash
git add backend/internal/inbound/driver.go backend/internal/inbound/driver_vless.go backend/internal/inbound/driver_hysteria2.go backend/internal/inbound/driver_test.go
git commit -m "feat(inbound): protocol driver registry (vless-reality, hysteria2)"
```

---

## Task 3: 切换核心（model + generate + service + handlers）

> 这是耦合的核心切换：改 `Inbound`/`User` 模型 → generate 走注册表 → service 新 API → handlers 适配。一个任务内完成、结尾全后端绿。逐文件替换，最后统一跑测试。

**Files:**
- Rewrite: `backend/internal/models/inbound.go`
- Rewrite: `backend/internal/inbound/generate.go`, `backend/internal/inbound/generate_test.go`
- Rewrite: `backend/internal/inbound/service.go`, `backend/internal/inbound/service_test.go`
- Modify: `backend/internal/handlers/inbound.go`, `backend/internal/handlers/inbound_test.go`

- [ ] **Step 1: 重写 model** `backend/internal/models/inbound.go`

```go
package models

import "time"

type Inbound struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Tag       string    `gorm:"uniqueIndex;not null" json:"tag"`
	Type      string    `gorm:"not null" json:"type"`
	Network   string    `gorm:"not null" json:"network"`
	Port      uint16    `gorm:"not null" json:"port"`
	Settings  string    `gorm:"not null" json:"-"` // protocol JSON (contains secrets); never serialized
	Users     []User    `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type User struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	InboundID  uint      `gorm:"index;not null" json:"inboundId"`
	Name       string    `gorm:"not null" json:"name"`
	Credential string    `gorm:"not null" json:"credential"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
```

- [ ] **Step 2: 重写 `generate.go`**

```go
package inbound

import (
	"encoding/json"
	"fmt"

	"singbox-admin/internal/models"
)

// Generate builds a complete sing-box config from inbounds via the driver registry.
func Generate(inbounds []models.Inbound) (string, error) {
	ins := []map[string]any{}
	for _, in := range inbounds {
		d, ok := Get(in.Type)
		if !ok {
			return "", fmt.Errorf("%w: %s", ErrUnknownType, in.Type)
		}
		creds := make([]Cred, 0, len(in.Users))
		for _, u := range in.Users {
			creds = append(creds, Cred{Name: u.Name, Credential: u.Credential})
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

- [ ] **Step 3: 重写 `generate_test.go`**

```go
package inbound

import (
	"encoding/json"
	"testing"

	"singbox-admin/internal/models"
)

func TestGenerateMixedProtocols(t *testing.T) {
	vd, _ := Get("vless-reality")
	vs, _ := vd.BuildSettings(nil)
	hd, _ := Get("hysteria2")
	hs, _ := hd.BuildSettings(nil)
	ins := []models.Inbound{
		{Tag: "v1", Type: "vless-reality", Network: "tcp", Port: 8443, Settings: vs,
			Users: []models.User{{Name: "a", Credential: "uuid-1"}}},
		{Tag: "h1", Type: "hysteria2", Network: "udp", Port: 443, Settings: hs,
			Users: []models.User{{Name: "b", Credential: "pass-1"}}},
	}
	out, err := Generate(ins)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	arr := cfg["inbounds"].([]any)
	if len(arr) != 2 {
		t.Fatalf("inbounds=%d", len(arr))
	}
	if arr[0].(map[string]any)["type"] != "vless" || arr[1].(map[string]any)["type"] != "hysteria2" {
		t.Fatalf("types: %v / %v", arr[0], arr[1])
	}
}

func TestGenerateUnknownType(t *testing.T) {
	_, err := Generate([]models.Inbound{{Tag: "x", Type: "nope", Settings: "{}"}})
	if err == nil {
		t.Fatal("want error for unknown type")
	}
}
```

- [ ] **Step 4: 重写 `service.go`**

```go
package inbound

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"singbox-admin/internal/models"
)

var (
	ErrTagExists  = errors.New("tag exists")
	ErrNotFound   = errors.New("not found")
	ErrInvalidTag = errors.New("invalid tag")
	ErrPortInUse  = errors.New("port in use")
)

// ConfigWriter is satisfied by *singbox.Service (SaveConfig).
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

// InboundView is the API-facing shape: no secret settings, plus driver publicInfo.
type InboundView struct {
	ID         uint           `json:"id"`
	Type       string         `json:"type"`
	Tag        string         `json:"tag"`
	Port       uint16         `json:"port"`
	Network    string         `json:"network"`
	PublicInfo map[string]any `json:"publicInfo"`
	Users      []models.User  `json:"users"`
}

func (s *Service) listInbounds() ([]models.Inbound, error) {
	var ins []models.Inbound
	err := s.db.Preload("Users").Order("id").Find(&ins).Error
	return ins, err
}

func (s *Service) ListInboundViews() ([]InboundView, error) {
	ins, err := s.listInbounds()
	if err != nil {
		return nil, err
	}
	views := make([]InboundView, 0, len(ins))
	for _, in := range ins {
		v := InboundView{ID: in.ID, Type: in.Type, Tag: in.Tag, Port: in.Port, Network: in.Network, Users: in.Users}
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
	d, ok := Get(in.Type)
	if !ok {
		return models.User{}, ErrUnknownType
	}
	u := models.User{InboundID: inboundID, Name: name, Credential: d.NewCredential()}
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

- [ ] **Step 5: 重写 `service_test.go`**

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

func TestCreateInboundVless(t *testing.T) {
	s, w := newTestService(t)
	in, err := s.CreateInbound("vless-reality", "v1", 8443, map[string]any{"handshake": "example.com"})
	if err != nil {
		t.Fatalf("CreateInbound: %v", err)
	}
	if in.Type != "vless-reality" || in.Network != "tcp" || in.Settings == "" {
		t.Fatalf("inbound = %+v", in)
	}
	if w.calls == 0 || !strings.Contains(w.last, "\"vless\"") {
		t.Fatalf("config not regenerated: %q", w.last)
	}
}

func TestCreateInboundUnknownType(t *testing.T) {
	s, _ := newTestService(t)
	if _, err := s.CreateInbound("nope", "x", 1, nil); err != ErrUnknownType {
		t.Fatalf("err = %v, want ErrUnknownType", err)
	}
}

func TestCreateInboundWhitespaceTag(t *testing.T) {
	s, _ := newTestService(t)
	if _, err := s.CreateInbound("hysteria2", "   ", 443, nil); err != ErrInvalidTag {
		t.Fatalf("err = %v, want ErrInvalidTag", err)
	}
}

func TestPortConflictByNetwork(t *testing.T) {
	s, _ := newTestService(t)
	// vless tcp:443 and hy2 udp:443 can coexist (different network)
	if _, err := s.CreateInbound("vless-reality", "v", 443, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateInbound("hysteria2", "h", 443, nil); err != nil {
		t.Fatalf("udp 443 should coexist with tcp 443: %v", err)
	}
	// but two tcp on 443 conflict
	if _, err := s.CreateInbound("vless-reality", "v2", 443, nil); err != ErrPortInUse {
		t.Fatalf("err = %v, want ErrPortInUse", err)
	}
}

func TestCreateUserCredentialByType(t *testing.T) {
	s, _ := newTestService(t)
	hin, _ := s.CreateInbound("hysteria2", "h", 443, nil)
	u, err := s.CreateUser(hin.ID, "alice")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.Credential == "" {
		t.Fatal("empty credential")
	}
}

func TestListInboundViewsNoSettings(t *testing.T) {
	s, _ := newTestService(t)
	_, _ = s.CreateInbound("vless-reality", "v1", 8443, nil)
	views, err := s.ListInboundViews()
	if err != nil || len(views) != 1 {
		t.Fatalf("views err=%v n=%d", err, len(views))
	}
	if views[0].PublicInfo["realityPublicKey"] == nil {
		t.Fatal("missing publicInfo")
	}
	if strings.Contains(toJSON(views[0]), "realityPrivateKey") {
		t.Fatal("view leaked private key")
	}
}

func TestCascadeDelete(t *testing.T) {
	s, _ := newTestService(t)
	in, _ := s.CreateInbound("vless-reality", "v1", 8443, nil)
	_, _ = s.CreateUser(in.ID, "a")
	if err := s.DeleteInbound(in.ID); err != nil {
		t.Fatal(err)
	}
	us, _ := s.ListUsers(in.ID)
	if len(us) != 0 {
		t.Fatalf("users not cascade-deleted: %d", len(us))
	}
}
```

- [ ] **Step 6: 改 handlers** `backend/internal/handlers/inbound.go`

替换 `InboundController` 接口、`ListInbounds`、`CreateInbound`，新增 `ListTypes`。完整新文件：

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

func (h *InboundHandler) ListTypes(c *gin.Context) {
	c.JSON(http.StatusOK, h.ctrl.Types())
}

func (h *InboundHandler) ListInbounds(c *gin.Context) {
	views, err := h.ctrl.ListInboundViews()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, views)
}

type createInboundBody struct {
	Type   string         `json:"type" binding:"required"`
	Tag    string         `json:"tag" binding:"required"`
	Port   uint16         `json:"port" binding:"required"`
	Params map[string]any `json:"params"`
}

func (h *InboundHandler) CreateInbound(c *gin.Context) {
	var b createInboundBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type/tag/port required"})
		return
	}
	in, err := h.ctrl.CreateInbound(b.Type, b.Tag, b.Port, b.Params)
	if err != nil {
		switch {
		case errors.Is(err, inbound.ErrUnknownType), errors.Is(err, inbound.ErrInvalidTag):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, inbound.ErrTagExists), errors.Is(err, inbound.ErrPortInUse):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
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
		writeStartError(c, err)
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

- [ ] **Step 7: 改 handler 测试** `backend/internal/handlers/inbound_test.go`

完整替换为：

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
	views      []inbound.InboundView
	types      []inbound.TypeInfo
	createErr  error
	deleteErr  error
	users      []models.User
	createUErr error
	deleteUErr error
	regenErr   error
	lastType   string
	lastTag    string
	lastUName  string
}

func (f *fakeInbCtrl) ListInboundViews() ([]inbound.InboundView, error) { return f.views, nil }
func (f *fakeInbCtrl) Types() []inbound.TypeInfo                        { return f.types }
func (f *fakeInbCtrl) CreateInbound(typ, tag string, port uint16, params map[string]any) (models.Inbound, error) {
	f.lastType, f.lastTag = typ, tag
	return models.Inbound{ID: 1, Tag: tag, Type: typ, Port: port}, f.createErr
}
func (f *fakeInbCtrl) DeleteInbound(id uint) error              { return f.deleteErr }
func (f *fakeInbCtrl) ListUsers(id uint) ([]models.User, error) { return f.users, nil }
func (f *fakeInbCtrl) CreateUser(id uint, name string) (models.User, error) {
	f.lastUName = name
	return models.User{ID: 1, Name: name, Credential: "c"}, f.createUErr
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
	e.GET("/api/inbound-types", h.ListTypes)
	e.GET("/api/inbounds", h.ListInbounds)
	e.POST("/api/inbounds", h.CreateInbound)
	e.DELETE("/api/inbounds/:id", h.DeleteInbound)
	e.POST("/api/inbounds/:id/users", h.CreateUser)
	e.DELETE("/api/users/:id", h.DeleteUser)
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
	if w.Code != 200 || c.lastType != "hysteria2" || c.lastTag != "h1" {
		t.Fatalf("code=%d type=%q tag=%q", w.Code, c.lastType, c.lastTag)
	}
}

func TestCreateInboundUnknownType(t *testing.T) {
	w := inbReq(inbRouter(&fakeInbCtrl{createErr: inbound.ErrUnknownType}, &fakeRestarter{}), http.MethodPost, "/api/inbounds", `{"type":"x","tag":"t","port":1}`)
	if w.Code != 400 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestCreateInboundPortInUse(t *testing.T) {
	w := inbReq(inbRouter(&fakeInbCtrl{createErr: inbound.ErrPortInUse}, &fakeRestarter{}), http.MethodPost, "/api/inbounds", `{"type":"vless-reality","tag":"t","port":443}`)
	if w.Code != 409 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestCreateUserOK(t *testing.T) {
	c := &fakeInbCtrl{}
	w := inbReq(inbRouter(c, &fakeRestarter{}), http.MethodPost, "/api/inbounds/1/users", `{"name":"alice"}`)
	if w.Code != 200 || c.lastUName != "alice" {
		t.Fatalf("code=%d name=%q", w.Code, c.lastUName)
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

- [ ] **Step 8: 全后端编译 + vet + 测试**

Run: `cd backend && go build ./... && go vet ./... && go test ./...`
Expected: 全 PASS。（main.go 仍调用旧 `inbound.NewService(db, sbSvc, keygen)` 与旧路由 → **会编译失败**！本步预期 main.go 报错。）

> main.go 在 Task 4 修。若本步因 main.go 编译失败，属预期；先确保 `go test ./internal/...` 全绿：
> Run: `cd backend && go test ./internal/...`
> Expected: 全 PASS。

- [ ] **Step 9: 提交**

```bash
git add backend/internal/models/inbound.go backend/internal/inbound/generate.go backend/internal/inbound/generate_test.go backend/internal/inbound/service.go backend/internal/inbound/service_test.go backend/internal/handlers/inbound.go backend/internal/handlers/inbound_test.go
git commit -m "feat(inbound): switch model+generate+service+handlers to driver registry"
```

---

## Task 4: SeedDefaults + 接线 main

**Files:** Create `backend/internal/inbound/seed.go`, `backend/internal/inbound/seed_test.go`; Modify `backend/cmd/server/main.go`

- [ ] **Step 1: 写失败测试 `seed_test.go`**

```go
package inbound

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"singbox-admin/internal/models"
)

func TestSeedDefaultsCreatesTwo(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.AutoMigrate(&models.Inbound{}, &models.User{})
	s := NewService(db, &fakeWriter{})
	if err := SeedDefaults(s); err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}
	var n int64
	db.Model(&models.Inbound{}).Count(&n)
	if n != 2 {
		t.Fatalf("seeded %d, want 2", n)
	}
	// idempotent
	if err := SeedDefaults(s); err != nil {
		t.Fatal(err)
	}
	db.Model(&models.Inbound{}).Count(&n)
	if n != 2 {
		t.Fatalf("after re-seed %d, want 2", n)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/inbound/ -run TestSeedDefaults`
Expected: FAIL（SeedDefaults 未定义）

- [ ] **Step 3: 实现 `seed.go`**

```go
package inbound

import "singbox-admin/internal/models"

// SeedDefaults creates one vless-reality (tcp 8443) and one hysteria2 (udp 443)
// inbound when there are none yet. Idempotent.
func SeedDefaults(s *Service) error {
	var n int64
	if err := s.db.Model(&models.Inbound{}).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	if _, err := s.CreateInbound("vless-reality", "vless-reality", 8443, nil); err != nil {
		return err
	}
	if _, err := s.CreateInbound("hysteria2", "hysteria2", 443, nil); err != nil {
		return err
	}
	return nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd backend && go test ./internal/inbound/`
Expected: 全 PASS

- [ ] **Step 5: 改 main.go** — 替换 inbound 接线与路由：

`inbSvc` 构造去掉 keygen 参数，seed，注册 `/inbound-types`。把 main.go 中相关段替换为：
```go
	sbSvc := singbox.NewDefault(cfg.SingboxDir, cfg.SingboxBin)
	sbHandler := handlers.NewSingboxHandler(sbSvc)

	inbSvc := inbound.NewService(db, sbSvc)
	if err := inbound.SeedDefaults(inbSvc); err != nil {
		log.Fatalf("seed defaults: %v", err)
	}
	inbHandler := handlers.NewInboundHandler(inbSvc, sbSvc)
```
并在 `authed` 路由组加一行（与现有 inbounds 路由相邻）：
```go
		authed.GET("/inbound-types", inbHandler.ListTypes)
```

- [ ] **Step 6: 全后端编译 + 测试 + 冒烟**

Run: `cd backend && go build ./... && go vet ./... && go test ./...`
Expected: 全 PASS。

冒烟（默认 seed + 列表 + 类型）:
```bash
cd backend && JWT_SECRET=t DB_PATH=/tmp/m4.db SINGBOX_DIR=/tmp/m4sb go build -o /tmp/m4srv ./cmd/server && (/tmp/m4srv >/tmp/m4.log 2>&1 &)
for i in $(seq 1 40); do curl -s -o /dev/null localhost:8080/api/nope 2>/dev/null && break; sleep 0.25; done
curl -s -c /tmp/m4.cookie -X POST localhost:8080/api/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"mnice7082"}' >/dev/null
echo "types:"; curl -s -b /tmp/m4.cookie localhost:8080/api/inbound-types; echo
echo "seeded inbounds:"; curl -s -b /tmp/m4.cookie localhost:8080/api/inbounds | head -c 400; echo
echo -n "status: "; curl -s -b /tmp/m4.cookie localhost:8080/api/status; echo
pkill -f /tmp/m4srv; rm -f /tmp/m4.db /tmp/m4.cookie /tmp/m4srv; rm -rf /tmp/m4sb
```
Expected: types 含 vless-reality 与 hysteria2；inbounds 返回两条（含 `"network":"tcp"` 与 `"udp"`，含 publicInfo，无 settings/realityPrivateKey/keyPEM）；status `hasConfig:true`。

- [ ] **Step 7: 提交**

```bash
git add backend/internal/inbound/seed.go backend/internal/inbound/seed_test.go backend/cmd/server/main.go
git commit -m "feat(inbound): default-seed vless-reality + hysteria2 on first run"
```

---

## Task 5: 前端 API 客户端

**Files:** Modify `frontend/lib/api.ts`, `frontend/lib/api.test.ts`

- [ ] **Step 1: 追加失败测试**（在 `api.test.ts` 的 describe 内）

```ts
  it("listInboundTypes returns array", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify([{ type: "hysteria2", label: "Hysteria2", network: "udp", defaultPort: 443 }]), { status: 200 })));
    const ts = await listInboundTypes();
    expect(ts[0].type).toBe("hysteria2");
  });

  it("createInbound posts type/tag/port/params", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: 1 }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    await createInbound("hysteria2", "h1", 443, { upMbps: 50 });
    const [, init] = fetchMock.mock.calls[0];
    const body = JSON.parse(init.body);
    expect(body.type).toBe("hysteria2");
    expect(body.params.upMbps).toBe(50);
  });
```

并把 import 行加上新符号：
```ts
import { login, getStatus, UnauthorizedError, listInbounds, listInboundTypes, createInbound, applySingbox } from "./api";
```

- [ ] **Step 2: 运行确认失败**

Run: `cd frontend && npm test -- lib/api`
Expected: FAIL（listInboundTypes 未导出 / createInbound 签名变化）

- [ ] **Step 3: 实现** — 在 `frontend/lib/api.ts`：

(a) 替换 `Inbound`/`SingboxUser` 接口并新增 `InboundType`：
```ts
export interface InboundType {
  type: string;
  label: string;
  network: string;
  defaultPort: number;
}

export interface SingboxUser {
  id: number;
  inboundId: number;
  name: string;
  credential: string;
}

export interface Inbound {
  id: number;
  type: string;
  tag: string;
  port: number;
  network: string;
  publicInfo: Record<string, unknown>;
  users?: SingboxUser[];
}
```
(b) 替换 `createInbound` 并新增 `listInboundTypes`（其余 listInbounds/deleteInbound/listUsers/createUser/deleteUser/applySingbox 保持）：
```ts
export async function listInboundTypes(): Promise<InboundType[]> {
  const res = await request("/api/inbound-types");
  if (!res.ok) throw new Error("加载协议类型失败");
  return res.json();
}

export async function createInbound(
  type: string,
  tag: string,
  port: number,
  params: Record<string, unknown>,
): Promise<Inbound> {
  const res = await request("/api/inbounds", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ type, tag, port, params }),
  });
  if (!res.ok) {
    const b = await res.json().catch(() => ({}));
    throw new Error(b.error || "创建入站失败");
  }
  return res.json();
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd frontend && npm test -- lib/api`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add frontend/lib/api.ts frontend/lib/api.test.ts
git commit -m "feat(frontend): api client for inbound types + typed create"
```

---

## Task 6: 入站页（协议下拉 + type-aware 表单 + 按类型展示）

**Files:** Modify `frontend/app/inbounds/page.tsx`, `frontend/app/inbounds/page.test.tsx`

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
vi.mock("@/lib/api", () => ({
  listInbounds: () => listInboundsMock(),
  listInboundTypes: () => listTypesMock(),
  createInbound: (...a: unknown[]) => createInboundMock(...a),
  deleteInbound: vi.fn(),
  listUsers: vi.fn().mockResolvedValue([]),
  createUser: vi.fn(),
  deleteUser: vi.fn(),
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
  createInboundMock.mockReset().mockResolvedValue({ id: 1, type: "hysteria2", tag: "h1", port: 443, network: "udp", publicInfo: {}, users: [] });
});

describe("InboundsPage", () => {
  it("渲染协议下拉与默认 vless 字段", async () => {
    render(<InboundsPage />);
    await waitFor(() => expect(screen.getByLabelText(/协议/)).toBeInTheDocument());
    expect(screen.getByLabelText(/握手域名/)).toBeInTheDocument();
  });

  it("切到 hysteria2 显示限速字段并提交 params", async () => {
    render(<InboundsPage />);
    await waitFor(() => expect(screen.getByLabelText(/协议/)).toBeInTheDocument());
    await userEvent.selectOptions(screen.getByLabelText(/协议/), "hysteria2");
    expect(screen.getByLabelText(/上行/)).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText(/标签/), "h1");
    await userEvent.click(screen.getByRole("button", { name: /新建入站/ }));
    await waitFor(() => expect(createInboundMock).toHaveBeenCalled());
    expect(createInboundMock.mock.calls[0][0]).toBe("hysteria2");
  });

  it("按类型展示入站卡片 publicInfo", async () => {
    listInboundsMock.mockResolvedValue([
      { id: 1, type: "vless-reality", tag: "v1", port: 8443, network: "tcp", publicInfo: { realityPublicKey: "PUB", shortId: "ab", serverName: "x", flow: "f" }, users: [] },
    ]);
    render(<InboundsPage />);
    await waitFor(() => expect(screen.getByText("v1")).toBeInTheDocument());
    expect(screen.getByText("PUB")).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: 运行确认失败**

Run: `cd frontend && npm test -- app/inbounds`
Expected: FAIL（旧页面无协议下拉）

- [ ] **Step 3: 整体替换 `page.tsx`**

```tsx
"use client";

import { useCallback, useEffect, useState } from "react";
import {
  listInbounds, listInboundTypes, createInbound, deleteInbound,
  listUsers, createUser, deleteUser,
  Inbound, InboundType, SingboxUser,
} from "@/lib/api";
import { AppShell } from "@/components/app-shell";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function InboundsPage() {
  const [inbounds, setInbounds] = useState<Inbound[]>([]);
  const [types, setTypes] = useState<InboundType[]>([]);
  const [type, setType] = useState("vless-reality");
  const [tag, setTag] = useState("");
  const [port, setPort] = useState("8443");
  const [handshake, setHandshake] = useState("www.microsoft.com");
  const [sni, setSni] = useState("bing.com");
  const [up, setUp] = useState("100");
  const [down, setDown] = useState("100");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(() => {
    listInbounds().then(setInbounds).catch(() => setError("加载入站失败"));
  }, []);
  useEffect(() => {
    refresh();
    listInboundTypes().then((ts) => {
      setTypes(ts);
      if (ts[0]) setType(ts[0].type);
    }).catch(() => {});
  }, [refresh]);

  function onTypeChange(t: string) {
    setType(t);
    const info = types.find((x) => x.type === t);
    if (info) setPort(String(info.defaultPort));
  }

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    const params: Record<string, unknown> =
      type === "hysteria2"
        ? { serverName: sni, upMbps: Number(up), downMbps: Number(down) }
        : { handshake };
    try {
      await createInbound(type, tag, Number(port), params);
      setTag("");
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
          <CardTitle className="font-mono text-xs tracking-wider text-muted-foreground uppercase">新建入站</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={onCreate} className="flex flex-wrap items-end gap-4">
            <div className="space-y-2">
              <Label htmlFor="type" className="text-xs text-muted-foreground">协议</Label>
              <select
                id="type"
                value={type}
                onChange={(e) => onTypeChange(e.target.value)}
                className="h-10 rounded-lg border border-input bg-secondary px-2 text-sm text-foreground"
              >
                {types.map((t) => (
                  <option key={t.type} value={t.type}>{t.label}</option>
                ))}
              </select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="tag" className="text-xs text-muted-foreground">标签</Label>
              <Input id="tag" className="h-10 w-40" value={tag} onChange={(e) => setTag(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="port" className="text-xs text-muted-foreground">端口</Label>
              <Input id="port" className="h-10 w-24" value={port} onChange={(e) => setPort(e.target.value)} />
            </div>
            {type === "hysteria2" ? (
              <>
                <div className="space-y-2">
                  <Label htmlFor="sni" className="text-xs text-muted-foreground">SNI</Label>
                  <Input id="sni" className="h-10 w-40" value={sni} onChange={(e) => setSni(e.target.value)} />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="up" className="text-xs text-muted-foreground">上行 Mbps</Label>
                  <Input id="up" className="h-10 w-24" value={up} onChange={(e) => setUp(e.target.value)} />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="down" className="text-xs text-muted-foreground">下行 Mbps</Label>
                  <Input id="down" className="h-10 w-24" value={down} onChange={(e) => setDown(e.target.value)} />
                </div>
              </>
            ) : (
              <div className="space-y-2">
                <Label htmlFor="hs" className="text-xs text-muted-foreground">握手域名</Label>
                <Input id="hs" className="h-10 w-56" value={handshake} onChange={(e) => setHandshake(e.target.value)} />
              </div>
            )}
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
  const [users, setUsers] = useState<SingboxUser[]>(inbound.users ?? []);
  const [name, setName] = useState("");

  const reloadUsers = useCallback(() => {
    listUsers(inbound.id).then(setUsers).catch(() => {});
  }, [inbound.id]);

  async function onAdd(e: React.FormEvent) {
    e.preventDefault();
    if (!name) return;
    await createUser(inbound.id, name);
    setName("");
    reloadUsers();
  }
  async function onRemove(id: number) {
    await deleteUser(id);
    reloadUsers();
  }

  const credLabel = inbound.type === "hysteria2" ? "密码" : "UUID";

  const field = (k: string, v: unknown) => (
    <div className="flex justify-between gap-4 py-1">
      <span className="font-mono text-xs text-muted-foreground uppercase">{k}</span>
      <span className="truncate font-mono text-xs">{String(v)}</span>
    </div>
  );

  return (
    <Card className="rounded-lg">
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="text-base font-normal">
            {inbound.tag}{" "}
            <span className="text-muted-foreground">· {inbound.type} · :{inbound.port}/{inbound.network}</span>
          </CardTitle>
          <Button variant="outline" className="rounded-full" onClick={onDelete}>删除</Button>
        </div>
      </CardHeader>
      <CardContent>
        <div className="mb-4 border-b border-border pb-3">
          {Object.entries(inbound.publicInfo).map(([k, v]) => field(k, v))}
        </div>
        <p className="mb-2 font-mono text-xs text-muted-foreground uppercase">用户（{credLabel}）</p>
        <div className="divide-y divide-border">
          {users.map((u) => (
            <div key={u.id} className="flex items-center justify-between gap-4 py-2">
              <span className="text-sm">{u.name}</span>
              <span className="truncate font-mono text-xs text-muted-foreground">{u.credential}</span>
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

- [ ] **Step 4: 运行确认通过 + 构建**

Run: `cd frontend && npm test -- app/inbounds && npm test && npm run build`
Expected: app/inbounds 通过；全量通过；构建成功（路由含 /inbounds）。

- [ ] **Step 5: 提交**

```bash
git add frontend/app/inbounds
git commit -m "feat(frontend): multi-protocol inbounds page (type dropdown, per-type fields)"
```

---

## Task 7: 全量验证 + 截图

**Files:** 无

- [ ] **Step 1:** `cd backend && go test ./...` → 全 PASS。
- [ ] **Step 2:** `cd frontend && npm test && npm run build` → 全 PASS、构建成功。
- [ ] **Step 3:** `make build` 出单二进制，临时 DB/SINGBOX_DIR 起在 :8080，浏览器登录后：
  - 截图 `/inbounds`：确认默认 seed 的两个入站（vless-reality :8443/tcp、hysteria2 :443/udp）已存在；切协议下拉看到 hy2 的限速字段；新建一个 hy2 入站；给某入站加用户看凭证标签（UUID/密码）。
  - 清理进程与临时文件。
- [ ] **Step 4:** 如有样式微调：`git add -A && git commit -m "chore(frontend): M4 polish" || echo "no changes"`

---

## Self-Review Notes

- **Spec 覆盖**：cert(T1)、driver 注册表+2 driver(T2)、模型重构+generate+service+handlers(T3)、seed+接线+/inbound-types(T4)、前端 api(T5)、入站页 type-aware(T6)、验证(T7)。限速字段(T2 hy2 BuildSettings/BuildInbound + T6 表单)、端口冲突按 (port,network)(T3 service)、自签证书(T1)、默认 8443/443(T2 DefaultPort + T4 seed)、私钥/证书私钥不下发(model Settings json:"-" + InboundView publicInfo + driver PublicInfo 测试)。
- **类型一致性**：`Driver{Type,Label,Network,DefaultPort,BuildSettings,NewCredential,BuildInbound,PublicInfo}`、`Cred{Name,Credential}`、`TypeInfo{type,label,network,defaultPort}`、`InboundView{id,type,tag,port,network,publicInfo,users}`、`models.User.Credential`——后端与前端 TS（`InboundType`/`Inbound`/`SingboxUser`）camelCase 对齐。`Service.CreateInbound(typ,tag,port,params)` 与 `InboundController`、handler body、前端 `createInbound` 一致。`NewService(db, writer)` 去掉了 keygen 入参（driver 自带 keygen）——main.go(T4) 同步。
- **执行注意**：T3 改了 service.CreateInbound 签名与 NewService 入参，main.go 暂时编译失败属预期，T3 只验 `go test ./internal/...`，T4 修 main 后再全量。T3 删除了 M3 的 reality 专用列与 User.UUID（model 重写），预发布期需重建库；seed 仅空表跑。前端 `createInbound` 由 3 参变 4 参（type 在最前）。
```

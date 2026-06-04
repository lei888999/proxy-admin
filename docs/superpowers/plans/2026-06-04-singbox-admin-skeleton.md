# sing-box-admin 骨架 (M1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 搭建可运行的 sing-box-admin 全栈骨架，跑通「前端登录 → 带 cookie 调用受保护 API → 展示 sing-box 状态」最小闭环，并能打包成单个 CGO-free 二进制。

**Architecture:** monorepo（`backend/` Go + `frontend/` Next.js）。后端 Gin + GORM + 纯 Go SQLite（`glebarez/sqlite`，免 CGO），JWT 存 httpOnly cookie。前端 Next.js App Router + Tailwind + shadcn/ui，`output:'export'` 静态导出，由 Go `embed.FS` 嵌入单二进制。开发期 Next dev(:3000) 经 rewrites 代理到 Go(:8080)。

**Tech Stack:** Go 1.22+、Gin、GORM、glebarez/sqlite、golang-jwt/v5、x/crypto/bcrypt；Next.js 15 + TypeScript、Tailwind、shadcn/ui、Vitest + Testing Library。

参考设计文档：`docs/superpowers/specs/2026-06-04-singbox-admin-skeleton-design.md`

---

## File Structure

后端（`backend/`）：
- `cmd/server/main.go` — 入口，装配配置/DB/路由/中间件
- `internal/config/config.go` — 环境变量配置
- `internal/models/admin.go` — Admin 模型
- `internal/database/database.go` — GORM 初始化、AutoMigrate、首启建默认管理员
- `internal/auth/jwt.go` — JWT 签发/校验
- `internal/middleware/auth.go` — 从 cookie 读 token 的鉴权中间件
- `internal/handlers/auth.go` — login / logout
- `internal/service/singbox.go` — sing-box 状态探测（CommandRunner 接口可 mock）
- `internal/handlers/status.go` — status 接口
- `internal/web/embed.go` — 嵌入前端产物 + SPA fallback

前端（`frontend/`）：
- `next.config.ts`、`tailwind.config.ts`、`app/layout.tsx`、`app/globals.css`
- `lib/api.ts` — fetch 封装
- `app/login/page.tsx`、`app/dashboard/page.tsx`
- `components/ui/*` — shadcn/ui 组件

顶层：`Makefile`、`.gitignore`

---

## Task 1: 初始化 Go module 与配置包

**Files:**
- Create: `backend/go.mod`
- Create: `backend/internal/config/config.go`
- Test: `backend/internal/config/config_test.go`

- [ ] **Step 1: 初始化 module**

Run（在 `backend/` 下）:
```bash
cd backend && go mod init singbox-admin
```

- [ ] **Step 2: 写失败测试**

Create `backend/internal/config/config_test.go`:
```go
package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	os.Clearenv()
	c := Load()
	if c.Port != "8080" {
		t.Fatalf("Port = %q, want 8080", c.Port)
	}
	if c.DBPath != "sing-box-admin.db" {
		t.Fatalf("DBPath = %q", c.DBPath)
	}
	if c.DefaultAdminUser != "admin" || c.DefaultAdminPass != "mnice7082" {
		t.Fatalf("default admin = %q/%q", c.DefaultAdminUser, c.DefaultAdminPass)
	}
	if c.JWTSecret == "" {
		t.Fatal("JWTSecret should be auto-generated when unset")
	}
}

func TestLoadFromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("PORT", "9000")
	os.Setenv("JWT_SECRET", "fixed-secret")
	c := Load()
	if c.Port != "9000" {
		t.Fatalf("Port = %q, want 9000", c.Port)
	}
	if c.JWTSecret != "fixed-secret" {
		t.Fatalf("JWTSecret = %q", c.JWTSecret)
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `cd backend && go test ./internal/config/`
Expected: FAIL（`Load` 未定义 / 编译错误）

- [ ] **Step 4: 写最小实现**

Create `backend/internal/config/config.go`:
```go
package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
)

type Config struct {
	Port             string
	DBPath           string
	JWTSecret        string
	DefaultAdminUser string
	DefaultAdminPass string
}

func Load() *Config {
	c := &Config{
		Port:             getenv("PORT", "8080"),
		DBPath:           getenv("DB_PATH", "sing-box-admin.db"),
		JWTSecret:        os.Getenv("JWT_SECRET"),
		DefaultAdminUser: getenv("DEFAULT_ADMIN_USER", "admin"),
		DefaultAdminPass: getenv("DEFAULT_ADMIN_PASS", "mnice7082"),
	}
	if c.JWTSecret == "" {
		c.JWTSecret = randomHex(32)
		log.Println("WARN: JWT_SECRET not set, generated a random one (sessions reset on restart)")
	}
	return c
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd backend && go test ./internal/config/`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add backend/go.mod backend/internal/config
git commit -m "feat(backend): config package with env loading and JWT secret fallback"
```

---

## Task 2: Admin 模型与数据库初始化（含首启默认管理员）

**Files:**
- Create: `backend/internal/models/admin.go`
- Create: `backend/internal/database/database.go`
- Test: `backend/internal/database/database_test.go`

- [ ] **Step 1: 加依赖**

Run:
```bash
cd backend && go get gorm.io/gorm github.com/glebarez/sqlite golang.org/x/crypto/bcrypt
```

- [ ] **Step 2: 写 Admin 模型**

Create `backend/internal/models/admin.go`:
```go
package models

import "time"

type Admin struct {
	ID           uint   `gorm:"primaryKey"`
	Username     string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
```

- [ ] **Step 3: 写失败测试**

Create `backend/internal/database/database_test.go`:
```go
package database

import (
	"testing"

	"singbox-admin/internal/models"
	"golang.org/x/crypto/bcrypt"
)

func TestInitCreatesDefaultAdmin(t *testing.T) {
	db, err := Init(":memory:", "admin", "mnice7082")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	var a models.Admin
	if err := db.Where("username = ?", "admin").First(&a).Error; err != nil {
		t.Fatalf("default admin not found: %v", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte("mnice7082")) != nil {
		t.Fatal("default admin password hash mismatch")
	}
}

func TestInitIdempotent(t *testing.T) {
	db, _ := Init(":memory:", "admin", "mnice7082")
	// Re-running seed on a non-empty table must not create duplicates.
	if err := seedDefaultAdmin(db, "admin", "mnice7082"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var count int64
	db.Model(&models.Admin{}).Count(&count)
	if count != 1 {
		t.Fatalf("admin count = %d, want 1", count)
	}
}
```

- [ ] **Step 4: 运行测试确认失败**

Run: `cd backend && go test ./internal/database/`
Expected: FAIL（`Init`/`seedDefaultAdmin` 未定义）

- [ ] **Step 5: 写实现**

Create `backend/internal/database/database.go`:
```go
package database

import (
	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"singbox-admin/internal/models"
)

func Init(dbPath, defaultUser, defaultPass string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&models.Admin{}); err != nil {
		return nil, err
	}
	if err := seedDefaultAdmin(db, defaultUser, defaultPass); err != nil {
		return nil, err
	}
	return db, nil
}

func seedDefaultAdmin(db *gorm.DB, user, pass string) error {
	var count int64
	if err := db.Model(&models.Admin{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return db.Create(&models.Admin{Username: user, PasswordHash: string(hash)}).Error
}
```

- [ ] **Step 6: 运行测试确认通过**

Run: `cd backend && go test ./internal/database/`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add backend/internal/models backend/internal/database backend/go.mod backend/go.sum
git commit -m "feat(backend): SQLite init with default admin seeding"
```

---

## Task 3: JWT 签发与校验

**Files:**
- Create: `backend/internal/auth/jwt.go`
- Test: `backend/internal/auth/jwt_test.go`

- [ ] **Step 1: 加依赖**

Run:
```bash
cd backend && go get github.com/golang-jwt/jwt/v5
```

- [ ] **Step 2: 写失败测试**

Create `backend/internal/auth/jwt_test.go`:
```go
package auth

import (
	"testing"
	"time"
)

func TestGenerateAndParse(t *testing.T) {
	m := NewJWTManager("secret", 7*24*time.Hour)
	token, err := m.Generate("admin")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	username, err := m.Parse(token)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if username != "admin" {
		t.Fatalf("username = %q, want admin", username)
	}
}

func TestParseRejectsBadSignature(t *testing.T) {
	token, _ := NewJWTManager("secret", time.Hour).Generate("admin")
	if _, err := NewJWTManager("other", time.Hour).Parse(token); err == nil {
		t.Fatal("expected error for wrong signing key")
	}
}

func TestParseRejectsExpired(t *testing.T) {
	m := NewJWTManager("secret", -time.Hour) // already expired
	token, _ := m.Generate("admin")
	if _, err := m.Parse(token); err == nil {
		t.Fatal("expected error for expired token")
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `cd backend && go test ./internal/auth/`
Expected: FAIL（未定义）

- [ ] **Step 4: 写实现**

Create `backend/internal/auth/jwt.go`:
```go
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTManager struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTManager(secret string, ttl time.Duration) *JWTManager {
	return &JWTManager{secret: []byte(secret), ttl: ttl}
}

func (m *JWTManager) Generate(username string) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   username,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.ttl)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

func (m *JWTManager) Parse(tokenString string) (string, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return "", err
	}
	return claims.Subject, nil
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd backend && go test ./internal/auth/`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add backend/internal/auth backend/go.mod backend/go.sum
git commit -m "feat(backend): JWT generate/parse manager"
```

---

## Task 4: 鉴权中间件（从 cookie 读 token）

**Files:**
- Create: `backend/internal/middleware/auth.go`
- Test: `backend/internal/middleware/auth_test.go`

- [ ] **Step 1: 加依赖**

Run:
```bash
cd backend && go get github.com/gin-gonic/gin
```

- [ ] **Step 2: 写失败测试**

Create `backend/internal/middleware/auth_test.go`:
```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"singbox-admin/internal/auth"
)

func setup() (*gin.Engine, *auth.JWTManager) {
	gin.SetMode(gin.TestMode)
	jm := auth.NewJWTManager("secret", time.Hour)
	r := gin.New()
	r.GET("/protected", RequireAuth(jm), func(c *gin.Context) {
		c.String(http.StatusOK, c.GetString("username"))
	})
	return r, jm
}

func TestRejectsMissingCookie(t *testing.T) {
	r, _ := setup()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", w.Code)
	}
}

func TestAcceptsValidCookie(t *testing.T) {
	r, jm := setup()
	token, _ := jm.Generate("admin")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Body.String() != "admin" {
		t.Fatalf("code=%d body=%q", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `cd backend && go test ./internal/middleware/`
Expected: FAIL（`RequireAuth` 未定义）

- [ ] **Step 4: 写实现**

Create `backend/internal/middleware/auth.go`:
```go
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"singbox-admin/internal/auth"
)

func RequireAuth(jm *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie("token")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		username, err := jm.Parse(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Set("username", username)
		c.Next()
	}
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd backend && go test ./internal/middleware/`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add backend/internal/middleware backend/go.sum
git commit -m "feat(backend): cookie-based auth middleware"
```

---

## Task 5: 登录 / 登出 handlers

**Files:**
- Create: `backend/internal/handlers/auth.go`
- Test: `backend/internal/handlers/auth_test.go`

- [ ] **Step 1: 写失败测试**

Create `backend/internal/handlers/auth_test.go`:
```go
package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"singbox-admin/internal/auth"
	"singbox-admin/internal/database"
)

func newAuthRouter(t *testing.T) *gin.Engine {
	gin.SetMode(gin.TestMode)
	db, err := database.Init(":memory:", "admin", "mnice7082")
	if err != nil {
		t.Fatalf("db init: %v", err)
	}
	h := NewAuthHandler(db, auth.NewJWTManager("secret", time.Hour))
	r := gin.New()
	r.POST("/api/auth/login", h.Login)
	r.POST("/api/auth/logout", h.Logout)
	return r
}

func TestLoginSuccessSetsCookie(t *testing.T) {
	r := newAuthRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login",
		strings.NewReader(`{"username":"admin","password":"mnice7082"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("Set-Cookie"), "token=") {
		t.Fatalf("expected token cookie, got %q", w.Header().Get("Set-Cookie"))
	}
}

func TestLoginWrongPassword(t *testing.T) {
	r := newAuthRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login",
		strings.NewReader(`{"username":"admin","password":"wrong"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", w.Code)
	}
}

func TestLoginMissingFields(t *testing.T) {
	r := newAuthRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", w.Code)
	}
}

func TestLogoutClearsCookie(t *testing.T) {
	r := newAuthRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Header().Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("expected cookie cleared, got %q", w.Header().Get("Set-Cookie"))
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd backend && go test ./internal/handlers/`
Expected: FAIL（`NewAuthHandler` 未定义）

- [ ] **Step 3: 写实现**

Create `backend/internal/handlers/auth.go`:
```go
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"singbox-admin/internal/auth"
	"singbox-admin/internal/models"
)

const cookieMaxAge = 7 * 24 * 60 * 60 // 7 days, seconds

type AuthHandler struct {
	db *gorm.DB
	jm *auth.JWTManager
}

func NewAuthHandler(db *gorm.DB, jm *auth.JWTManager) *AuthHandler {
	return &AuthHandler{db: db, jm: jm}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password required"})
		return
	}
	var admin models.Admin
	if err := h.db.Where("username = ?", req.Username).First(&admin).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	token, err := h.jm.Generate(admin.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return
	}
	// SetCookie(name, value, maxAge, path, domain, secure, httpOnly)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("token", token, cookieMaxAge, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"username": admin.Username})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("token", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
```

> 注：`c.SetCookie` 用负 maxAge 时 Gin 输出 `Max-Age=0`，对应测试断言。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd backend && go test ./internal/handlers/`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/handlers
git commit -m "feat(backend): login/logout handlers with httpOnly cookie"
```

---

## Task 6: sing-box 状态服务（可 mock 命令执行）

**Files:**
- Create: `backend/internal/service/singbox.go`
- Test: `backend/internal/service/singbox_test.go`

- [ ] **Step 1: 写失败测试**

Create `backend/internal/service/singbox_test.go`:
```go
package service

import (
	"errors"
	"testing"
)

type fakeRunner struct {
	versionOut string
	versionErr error
	running    bool
}

func (f fakeRunner) Version() (string, error) { return f.versionOut, f.versionErr }
func (f fakeRunner) IsRunning() bool          { return f.running }

func TestStatusInstalledRunning(t *testing.T) {
	svc := NewSingboxService(fakeRunner{versionOut: "sing-box version 1.9.0", running: true})
	st := svc.Status()
	if !st.Installed || st.Version != "1.9.0" || !st.Running {
		t.Fatalf("status = %+v", st)
	}
}

func TestStatusNotInstalled(t *testing.T) {
	svc := NewSingboxService(fakeRunner{versionErr: errors.New("not found")})
	st := svc.Status()
	if st.Installed || st.Version != "" || st.Running {
		t.Fatalf("status = %+v", st)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd backend && go test ./internal/service/`
Expected: FAIL（未定义）

- [ ] **Step 3: 写实现**

Create `backend/internal/service/singbox.go`:
```go
package service

import (
	"os/exec"
	"regexp"
	"strings"
)

// Runner abstracts sing-box process/command interaction so it can be mocked.
type Runner interface {
	Version() (string, error)
	IsRunning() bool
}

type Status struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	Running   bool   `json:"running"`
}

type SingboxService struct {
	runner Runner
}

func NewSingboxService(r Runner) *SingboxService {
	return &SingboxService{runner: r}
}

var versionRe = regexp.MustCompile(`(\d+\.\d+\.\d+)`)

func (s *SingboxService) Status() Status {
	out, err := s.runner.Version()
	if err != nil {
		return Status{Installed: false}
	}
	version := ""
	if m := versionRe.FindString(out); m != "" {
		version = m
	}
	return Status{Installed: true, Version: version, Running: s.runner.IsRunning()}
}

// execRunner is the real implementation backed by the local sing-box binary.
type execRunner struct{}

func NewExecRunner() Runner { return execRunner{} }

func (execRunner) Version() (string, error) {
	out, err := exec.Command("sing-box", "version").CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (execRunner) IsRunning() bool {
	// pgrep returns exit code 0 only when a matching process exists.
	return exec.Command("pgrep", "-x", "sing-box").Run() == nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd backend && go test ./internal/service/`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/service
git commit -m "feat(backend): sing-box status service with mockable runner"
```

---

## Task 7: status handler

**Files:**
- Create: `backend/internal/handlers/status.go`
- Test: `backend/internal/handlers/status_test.go`

- [ ] **Step 1: 写失败测试**

Create `backend/internal/handlers/status_test.go`:
```go
package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"singbox-admin/internal/service"
)

type stubRunner struct{}

func (stubRunner) Version() (string, error) { return "sing-box version 1.9.0", nil }
func (stubRunner) IsRunning() bool          { return false }

func TestStatusEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewStatusHandler(service.NewSingboxService(stubRunner{}))
	r := gin.New()
	r.GET("/api/status", h.Get)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"installed":true`) || !strings.Contains(body, `"version":"1.9.0"`) {
		t.Fatalf("unexpected body: %s", body)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd backend && go test ./internal/handlers/ -run TestStatusEndpoint`
Expected: FAIL（`NewStatusHandler` 未定义）

- [ ] **Step 3: 写实现**

Create `backend/internal/handlers/status.go`:
```go
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"singbox-admin/internal/service"
)

type StatusHandler struct {
	svc *service.SingboxService
}

func NewStatusHandler(svc *service.SingboxService) *StatusHandler {
	return &StatusHandler{svc: svc}
}

func (h *StatusHandler) Get(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.Status())
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd backend && go test ./internal/handlers/`
Expected: PASS（全部 handler 测试）

- [ ] **Step 5: 提交**

```bash
git add backend/internal/handlers/status.go backend/internal/handlers/status_test.go
git commit -m "feat(backend): status endpoint"
```

---

## Task 8: 前端嵌入与 SPA fallback

**Files:**
- Create: `backend/internal/web/embed.go`
- Create: `backend/internal/web/dist/.gitkeep`
- Create: `backend/internal/web/dist/index.html`（占位，构建时被真实产物覆盖）
- Test: `backend/internal/web/embed_test.go`

- [ ] **Step 1: 建占位产物目录**

Run:
```bash
mkdir -p backend/internal/web/dist
printf '<!doctype html><title>placeholder</title>' > backend/internal/web/dist/index.html
touch backend/internal/web/dist/.gitkeep
```

- [ ] **Step 2: 写失败测试**

Create `backend/internal/web/embed_test.go`:
```go
package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestServesIndexForUnknownRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil) // client-side route
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (SPA fallback)", w.Code)
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `cd backend && go test ./internal/web/`
Expected: FAIL（`Register` 未定义）

- [ ] **Step 4: 写实现**

Create `backend/internal/web/embed.go`:
```go
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var distFS embed.FS

// Register mounts the embedded static export. Unknown non-API routes fall back
// to index.html so the client-side router can handle them.
func Register(r *gin.Engine) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		// Serve the static file if it exists; otherwise fall back to index.html.
		if _, err := fs.Stat(sub, strings.TrimPrefix(path, "/")); err != nil {
			c.Request.URL.Path = "/"
		}
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd backend && go test ./internal/web/`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add backend/internal/web
git commit -m "feat(backend): embed frontend with SPA fallback"
```

---

## Task 9: main.go 装配 + 后端 Makefile

**Files:**
- Create: `backend/cmd/server/main.go`
- Create: `backend/Makefile`

- [ ] **Step 1: 写 main.go**

Create `backend/cmd/server/main.go`:
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
	"singbox-admin/internal/middleware"
	"singbox-admin/internal/service"
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
	statusHandler := handlers.NewStatusHandler(service.NewSingboxService(service.NewExecRunner()))

	r := gin.Default()

	api := r.Group("/api")
	{
		api.POST("/auth/login", authHandler.Login)
		api.POST("/auth/logout", authHandler.Logout)
		api.GET("/status", middleware.RequireAuth(jm), statusHandler.Get)
	}

	web.Register(r) // static frontend + SPA fallback

	log.Printf("listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 2: 校验编译**

Run: `cd backend && go build ./... && go vet ./...`
Expected: 无输出（成功）

- [ ] **Step 3: 写后端 Makefile**

Create `backend/Makefile`:
```make
.PHONY: test build run

test:
	go test ./...

build:
	go build -o bin/sing-box-admin ./cmd/server

run:
	go run ./cmd/server
```

- [ ] **Step 4: 跑全部后端测试**

Run: `cd backend && go test ./...`
Expected: 全部 PASS

- [ ] **Step 5: 提交**

```bash
git add backend/cmd backend/Makefile
git commit -m "feat(backend): wire server entrypoint and Makefile"
```

---

## Task 10: 初始化前端（Next.js + Tailwind + shadcn）

**Files:**
- Create: `frontend/`（脚手架）
- Modify: `frontend/next.config.ts`
- Create: `frontend/.env.local` 不需要；rewrites 走相对路径

- [ ] **Step 1: 脚手架**

Run（在仓库根目录）:
```bash
npx create-next-app@latest frontend --typescript --tailwind --eslint --app --src-dir=false --import-alias "@/*" --no-turbopack --use-npm
```
（全部按提示默认；若交互卡住，逐项选：App Router=yes、src dir=no、import alias=@/*）

- [ ] **Step 2: 配置静态导出 + 开发代理**

Replace `frontend/next.config.ts` 内容为:
```ts
import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "export",
  images: { unoptimized: true },
  // Dev-only proxy so the browser sees same-origin /api (cookies + no CORS).
  // rewrites are ignored during `next build` with output:'export', which is fine
  // because production serves /api from the same Go binary.
  async rewrites() {
    return [{ source: "/api/:path*", destination: "http://localhost:8080/api/:path*" }];
  },
};

export default nextConfig;
```

- [ ] **Step 3: 初始化 shadcn/ui**

Run:
```bash
cd frontend && npx shadcn@latest init -d && npx shadcn@latest add button input card label
```
（`-d` 用默认配置；若版本提示，接受默认）

- [ ] **Step 4: 确认开发服务器可起**

Run: `cd frontend && npm run build`
Expected: 构建成功，产物在 `frontend/out/`

- [ ] **Step 5: 提交**

```bash
git add frontend
git commit -m "chore(frontend): scaffold Next.js + Tailwind + shadcn, configure static export"
```

---

## Task 11: 前端 API 封装 + 测试框架

**Files:**
- Create: `frontend/lib/api.ts`
- Create: `frontend/vitest.config.ts`
- Create: `frontend/vitest.setup.ts`
- Test: `frontend/lib/api.test.ts`
- Modify: `frontend/package.json`（加 test 脚本与依赖）

- [ ] **Step 1: 安装测试依赖**

Run:
```bash
cd frontend && npm install -D vitest @testing-library/react @testing-library/jest-dom @testing-library/user-event jsdom @vitejs/plugin-react
```

- [ ] **Step 2: 配置 Vitest**

Create `frontend/vitest.config.ts`:
```ts
import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "path";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    setupFiles: ["./vitest.setup.ts"],
    globals: true,
  },
  resolve: {
    alias: { "@": path.resolve(__dirname, ".") },
  },
});
```

Create `frontend/vitest.setup.ts`:
```ts
import "@testing-library/jest-dom/vitest";
```

在 `frontend/package.json` 的 `"scripts"` 中加入:
```json
"test": "vitest run"
```

- [ ] **Step 3: 写失败测试**

Create `frontend/lib/api.test.ts`:
```ts
import { describe, it, expect, vi, beforeEach } from "vitest";
import { login, getStatus, UnauthorizedError } from "./api";

beforeEach(() => {
  vi.restoreAllMocks();
});

describe("api", () => {
  it("login posts credentials with cookies included", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ username: "admin" }), { status: 200 })
    );
    vi.stubGlobal("fetch", fetchMock);

    const res = await login("admin", "mnice7082");
    expect(res.username).toBe("admin");
    const [, init] = fetchMock.mock.calls[0];
    expect(init.credentials).toBe("include");
    expect(init.method).toBe("POST");
  });

  it("getStatus throws UnauthorizedError on 401", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("", { status: 401 })));
    await expect(getStatus()).rejects.toBeInstanceOf(UnauthorizedError);
  });

  it("getStatus returns parsed status on 200", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ installed: true, version: "1.9.0", running: false }), {
          status: 200,
        })
      )
    );
    const st = await getStatus();
    expect(st.version).toBe("1.9.0");
  });
});
```

- [ ] **Step 4: 运行测试确认失败**

Run: `cd frontend && npm test`
Expected: FAIL（`./api` 模块不存在）

- [ ] **Step 5: 写实现**

Create `frontend/lib/api.ts`:
```ts
export class UnauthorizedError extends Error {}

export interface SingboxStatus {
  installed: boolean;
  version: string;
  running: boolean;
}

async function request(path: string, init?: RequestInit): Promise<Response> {
  const res = await fetch(path, { credentials: "include", ...init });
  if (res.status === 401) {
    throw new UnauthorizedError("unauthorized");
  }
  return res;
}

export async function login(username: string, password: string): Promise<{ username: string }> {
  const res = await request("/api/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });
  if (!res.ok) {
    throw new Error("login failed");
  }
  return res.json();
}

export async function logout(): Promise<void> {
  await request("/api/auth/logout", { method: "POST" });
}

export async function getStatus(): Promise<SingboxStatus> {
  const res = await request("/api/status");
  if (!res.ok) {
    throw new Error("failed to load status");
  }
  return res.json();
}
```

- [ ] **Step 6: 运行测试确认通过**

Run: `cd frontend && npm test`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add frontend/lib frontend/vitest.config.ts frontend/vitest.setup.ts frontend/package.json frontend/package-lock.json
git commit -m "feat(frontend): api client and vitest setup"
```

---

## Task 12: 登录页

**Files:**
- Create: `frontend/app/login/page.tsx`
- Test: `frontend/app/login/page.test.tsx`

- [ ] **Step 1: 写失败测试**

Create `frontend/app/login/page.test.tsx`:
```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import LoginPage from "./page";

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: pushMock }) }));
vi.mock("@/lib/api", () => ({ login: vi.fn().mockResolvedValue({ username: "admin" }) }));

beforeEach(() => {
  pushMock.mockClear();
});

describe("LoginPage", () => {
  it("renders username and password fields", () => {
    render(<LoginPage />);
    expect(screen.getByLabelText(/username/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
  });

  it("redirects to dashboard after successful login", async () => {
    render(<LoginPage />);
    await userEvent.type(screen.getByLabelText(/username/i), "admin");
    await userEvent.type(screen.getByLabelText(/password/i), "mnice7082");
    await userEvent.click(screen.getByRole("button", { name: /sign in/i }));
    expect(pushMock).toHaveBeenCalledWith("/dashboard");
  });
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd frontend && npm test -- app/login`
Expected: FAIL（`./page` 不存在）

- [ ] **Step 3: 写实现**

Create `frontend/app/login/page.tsx`:
```tsx
"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { login } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function LoginPage() {
  const router = useRouter();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      await login(username, password);
      router.push("/dashboard");
    } catch {
      setError("用户名或密码错误");
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-muted p-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>sing-box-admin 登录</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={onSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="username">Username</Label>
              <Input id="username" value={username} onChange={(e) => setUsername(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>
            {error && <p className="text-sm text-red-500">{error}</p>}
            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? "..." : "Sign in"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </main>
  );
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd frontend && npm test -- app/login`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add frontend/app/login
git commit -m "feat(frontend): login page"
```

---

## Task 13: 仪表盘页

**Files:**
- Create: `frontend/app/dashboard/page.tsx`
- Test: `frontend/app/dashboard/page.test.tsx`

- [ ] **Step 1: 写失败测试**

Create `frontend/app/dashboard/page.test.tsx`:
```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import DashboardPage from "./page";
import { UnauthorizedError } from "@/lib/api";

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: pushMock }) }));

const getStatusMock = vi.fn();
const logoutMock = vi.fn();
vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return { ...actual, getStatus: () => getStatusMock(), logout: () => logoutMock() };
});

beforeEach(() => {
  pushMock.mockClear();
  getStatusMock.mockReset();
  logoutMock.mockReset();
});

describe("DashboardPage", () => {
  it("shows sing-box status when authorized", async () => {
    getStatusMock.mockResolvedValue({ installed: true, version: "1.9.0", running: false });
    render(<DashboardPage />);
    await waitFor(() => expect(screen.getByText(/1\.9\.0/)).toBeInTheDocument());
  });

  it("redirects to login on 401", async () => {
    getStatusMock.mockRejectedValue(new UnauthorizedError());
    render(<DashboardPage />);
    await waitFor(() => expect(pushMock).toHaveBeenCalledWith("/login"));
  });
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd frontend && npm test -- app/dashboard`
Expected: FAIL（`./page` 不存在）

- [ ] **Step 3: 写实现**

Create `frontend/app/dashboard/page.tsx`:
```tsx
"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { getStatus, logout, UnauthorizedError, SingboxStatus } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function DashboardPage() {
  const router = useRouter();
  const [status, setStatus] = useState<SingboxStatus | null>(null);

  useEffect(() => {
    getStatus()
      .then(setStatus)
      .catch((err) => {
        if (err instanceof UnauthorizedError) {
          router.push("/login");
        }
      });
  }, [router]);

  async function onLogout() {
    await logout();
    router.push("/login");
  }

  return (
    <main className="min-h-screen bg-muted p-8">
      <div className="mx-auto max-w-2xl space-y-4">
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-semibold">Dashboard</h1>
          <Button variant="outline" onClick={onLogout}>
            Logout
          </Button>
        </div>
        <Card>
          <CardHeader>
            <CardTitle>sing-box 状态</CardTitle>
          </CardHeader>
          <CardContent>
            {status === null ? (
              <p>加载中…</p>
            ) : (
              <ul className="space-y-1">
                <li>已安装：{status.installed ? "是" : "否"}</li>
                <li>版本：{status.version || "—"}</li>
                <li>运行中：{status.running ? "是" : "否"}</li>
              </ul>
            )}
          </CardContent>
        </Card>
      </div>
    </main>
  );
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd frontend && npm test`
Expected: 全部 PASS

- [ ] **Step 5: 让根路径跳转到 dashboard**

Replace `frontend/app/page.tsx` 内容为:
```tsx
"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

export default function Home() {
  const router = useRouter();
  useEffect(() => {
    router.replace("/dashboard");
  }, [router]);
  return null;
}
```

- [ ] **Step 6: 提交**

```bash
git add frontend/app/dashboard frontend/app/page.tsx
git commit -m "feat(frontend): dashboard page with status + auth redirect"
```

---

## Task 14: 顶层 Makefile、.gitignore、构建集成

**Files:**
- Create: `Makefile`
- Create: `.gitignore`

- [ ] **Step 1: 写 .gitignore**

Create `.gitignore`:
```gitignore
# Go
backend/bin/
*.db

# Frontend
frontend/node_modules/
frontend/.next/
frontend/out/

# embedded build output (regenerated by `make build`)
backend/internal/web/dist/*
!backend/internal/web/dist/.gitkeep
!backend/internal/web/dist/index.html

# misc
.DS_Store
```

- [ ] **Step 2: 写顶层 Makefile**

Create `Makefile`:
```make
.PHONY: dev build run test test-backend test-frontend

# Run backend (:8080) and frontend dev (:3000) together.
dev:
	@echo "Starting backend on :8080 and frontend on :3000"
	@(cd backend && go run ./cmd/server) & \
	(cd frontend && npm run dev) ; \
	kill %1 2>/dev/null || true

# Build single binary: export frontend -> copy into embed dir -> go build.
build:
	cd frontend && npm install && npm run build
	rm -rf backend/internal/web/dist
	mkdir -p backend/internal/web/dist
	cp -r frontend/out/. backend/internal/web/dist/
	cd backend && go build -o bin/sing-box-admin ./cmd/server
	@echo "Built backend/bin/sing-box-admin"

run: build
	./backend/bin/sing-box-admin

test: test-backend test-frontend

test-backend:
	cd backend && go test ./...

test-frontend:
	cd frontend && npm test
```

- [ ] **Step 3: 端到端构建验证**

Run: `make build`
Expected: 生成 `backend/bin/sing-box-admin`，无报错。

- [ ] **Step 4: 运行单二进制冒烟测试**

Run（前台启动后另开终端，或用 curl 探活）:
```bash
JWT_SECRET=test ./backend/bin/sing-box-admin &
sleep 1
curl -s -i -X POST localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"mnice7082"}' | grep -i set-cookie
curl -s localhost:8080/ | head -c 50
kill %1
```
Expected: 看到 `Set-Cookie: token=...`，且根路径返回 HTML。

- [ ] **Step 5: 提交**

```bash
git add Makefile .gitignore
git commit -m "build: top-level Makefile, gitignore, single-binary build integration"
```

---

## Task 15: 生成 CLAUDE.md（init 命令的最终产物）

**Files:**
- Create: `CLAUDE.md`

- [ ] **Step 1: 据真实代码与命令写 CLAUDE.md**

Create `CLAUDE.md`，须以规定前缀开头，并涵盖：
- 项目概述（sing-box VPS 服务端管理面板；前端 Next.js / 后端 Go 单二进制）
- 常用命令：`make dev` / `make build` / `make test`；单测：`cd backend && go test ./internal/handlers/ -run TestLoginSuccessSetsCookie`、`cd frontend && npm test -- app/login`
- 架构大图：Gin + GORM + 纯 Go SQLite；JWT httpOnly cookie；前端静态导出经 `embed.FS` 嵌入；开发期 Next rewrites 代理到 :8080
- 关键约定：CGO-free（glebarez/sqlite）；`internal/web/dist` 为构建产物（被 gitignore，构建时由 `frontend/out` 填充）；sing-box 状态探测在 `internal/service`，命令执行经 `Runner` 接口可 mock
- 范围说明：M1 仅骨架+登录+状态；进程控制/配置生成等见 specs

内容须真实对应已实现代码，不杜撰未实现的命令或功能。

- [ ] **Step 2: 提交**

```bash
git add CLAUDE.md
git commit -m "docs: add CLAUDE.md for repository guidance"
```

---

## Self-Review Notes

- **Spec coverage:** monorepo 结构(Task1-14)、JWT+cookie 登录(Task3-5,11-12)、SQLite+默认管理员(Task2)、sing-box 状态(Task6-7,13)、单二进制嵌入(Task8,14)、Makefile(Task9,14)、测试策略(各 Task TDD)、CLAUDE.md(Task15)——均覆盖。
- **类型一致性:** `Runner` 接口（`Version()/IsRunning()`）在 Task6 定义，Task6/7 测试一致；`SingboxStatus` 字段 `installed/version/running` 前后端一致（Go json tag 小写 ↔ TS interface）；cookie 名 `token` 在 middleware/handler/前端 credentials 流程一致。
- **风险点提醒（执行时注意）:** create-next-app 与 shadcn 的交互式提示可能随版本变化；若非交互参数失效，按 Step 内说明手动选默认值即可。shadcn 生成的组件路径默认 `@/components/ui/*`，与 import 一致。
```

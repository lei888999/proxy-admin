# M2: sing-box 进程控制 + B 端后台 shell Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让面板能编辑 sing-box 原始 config.json、启动/停止本机 sing-box 进程，并把前端改造成深色 B 端后台 shell（全中文、大气登录页）。

**Architecture:** 新增 `internal/singbox` 包，拆为 `ConfigStore`（配置读写校验）、`ProcessManager`（子进程托管，PID 文件）、`Service`（编排+状态），OS 交互走可注入的 `Env` 接口便于单测。Gin 暴露 `/api/singbox/*` 与扩展 `/api/status`。前端为 Next App Router 静态导出的后台 shell（侧边栏+顶栏+内容区）。sing-box 随部署内置（docker 镜像/native 打包），面板不做安装。

**Tech Stack:** Go 1.26 + Gin + GORM；Next.js 16 + TS + Tailwind v4 + shadcn/ui + Vitest。

参考规范：`docs/superpowers/specs/2026-06-05-singbox-process-control-design.md`

---

## File Structure

后端：
- `backend/internal/config/config.go`（改）— 加 `SingboxDir`、`SingboxBin`
- `backend/internal/singbox/env.go`（新）— `Env` 接口 + 真实 `osEnv` 实现
- `backend/internal/singbox/errors.go`（新）— 哨兵错误 + `InvalidConfigError`
- `backend/internal/singbox/config_store.go`（新）— `ConfigStore`
- `backend/internal/singbox/process.go`（新）— `ProcessManager`
- `backend/internal/singbox/service.go`（新）— `Service`、`Status`、`NewDefault`
- `backend/internal/handlers/singbox.go`（新）— HTTP handlers + `SingboxController` 接口
- `backend/cmd/server/main.go`（改）— 路由接线，改用 `internal/singbox`
- 删除：`backend/internal/service/`、`backend/internal/handlers/status.go`、`status_test.go`

部署：
- `docker-compose.yml`（改）、`scripts/deploy-native.sh`（改）

前端：
- `frontend/lib/api.ts`（改）
- `frontend/components/app-shell.tsx`（新）
- `frontend/app/login/page.tsx`（改）、`login/page.test.tsx`（改）
- `frontend/app/dashboard/page.tsx`（改）、`dashboard/page.test.tsx`（改）
- `frontend/app/config/page.tsx`（新）、`config/page.test.tsx`（新）

---

## Task 1: 配置项 SingboxDir / SingboxBin

**Files:**
- Modify: `backend/internal/config/config.go`
- Test: `backend/internal/config/config_test.go`

- [ ] **Step 1: 加失败测试**（在 `config_test.go` 末尾追加）

```go
func TestSingboxDefaults(t *testing.T) {
	os.Clearenv()
	c := Load()
	if c.SingboxDir != "./singbox" {
		t.Fatalf("SingboxDir = %q, want ./singbox", c.SingboxDir)
	}
	if c.SingboxBin != "" {
		t.Fatalf("SingboxBin = %q, want empty", c.SingboxBin)
	}
}

func TestSingboxFromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("SINGBOX_DIR", "/data/singbox")
	os.Setenv("SINGBOX_BIN", "/usr/local/bin/sing-box")
	c := Load()
	if c.SingboxDir != "/data/singbox" || c.SingboxBin != "/usr/local/bin/sing-box" {
		t.Fatalf("got %q / %q", c.SingboxDir, c.SingboxBin)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/config/`
Expected: FAIL（`SingboxDir` 未定义）

- [ ] **Step 3: 实现** — 在 `Config` 结构体加两个字段，在 `Load()` 里赋值。

`config.go` 的 `Config` 结构体加：
```go
	SingboxDir       string
	SingboxBin       string
```
`Load()` 的 `c := &Config{...}` 里加：
```go
		SingboxDir:       getenv("SINGBOX_DIR", "./singbox"),
		SingboxBin:       os.Getenv("SINGBOX_BIN"),
```

- [ ] **Step 4: 运行确认通过**

Run: `cd backend && go test ./internal/config/`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/config
git commit -m "feat(backend): config SingboxDir/SingboxBin"
```

---

## Task 2: singbox 包 — Env 接口 + 错误 + osEnv

**Files:**
- Create: `backend/internal/singbox/env.go`
- Create: `backend/internal/singbox/errors.go`

> 本任务只建接口/错误与真实实现（无独立单测；后续任务用 fake Env 驱动）。结尾用 `go build` 校验。

- [ ] **Step 1: 写 errors.go**

```go
package singbox

import "errors"

var (
	ErrNotInstalled   = errors.New("sing-box not installed")
	ErrNoConfig       = errors.New("no config")
	ErrAlreadyRunning = errors.New("already running")
	ErrNotRunning     = errors.New("not running")
	ErrInvalidJSON    = errors.New("invalid json")
)

// InvalidConfigError carries the `sing-box check` output for a bad config.
type InvalidConfigError struct{ Output string }

func (e *InvalidConfigError) Error() string { return "invalid config" }
```

- [ ] **Step 2: 写 env.go**

```go
package singbox

import (
	"os"
	"os/exec"
	"syscall"
)

// Env abstracts every OS interaction so the service/process logic is testable.
type Env interface {
	LookPath(file string) (string, bool)
	FileExists(path string) bool
	RunVersion(bin string) (string, bool)         // `bin version`
	Check(bin, configPath string) (string, error) // `bin check -c`; err if exit != 0
	Spawn(logPath, bin, configPath string) (int, error)
	Alive(pid int) bool
	Pgrep(name string) bool
	Signal(pid int, sig syscall.Signal) error
}

type osEnv struct{}

// NewOSEnv returns the production Env backed by the real OS.
func NewOSEnv() Env { return osEnv{} }

func (osEnv) LookPath(file string) (string, bool) {
	p, err := exec.LookPath(file)
	return p, err == nil
}

func (osEnv) FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (osEnv) RunVersion(bin string) (string, bool) {
	out, err := exec.Command(bin, "version").CombinedOutput()
	if err != nil {
		return "", false
	}
	return string(out), true
}

func (osEnv) Check(bin, configPath string) (string, error) {
	out, err := exec.Command(bin, "check", "-c", configPath).CombinedOutput()
	return string(out), err
}

func (osEnv) Spawn(logPath, bin, configPath string) (int, error) {
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	cmd := exec.Command(bin, "run", "-c", configPath)
	cmd.Stdout = f
	cmd.Stderr = f
	// Detach from the panel's process group so the panel restarting/exiting
	// does not signal sing-box.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	return cmd.Process.Pid, nil
}

func (osEnv) Alive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

func (osEnv) Pgrep(name string) bool {
	return exec.Command("pgrep", "-x", name).Run() == nil
}

func (osEnv) Signal(pid int, sig syscall.Signal) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(sig)
}
```

- [ ] **Step 3: 校验编译**

Run: `cd backend && go build ./internal/singbox/`
Expected: 无输出（成功）

- [ ] **Step 4: 提交**

```bash
git add backend/internal/singbox/env.go backend/internal/singbox/errors.go
git commit -m "feat(singbox): Env interface, osEnv impl, sentinel errors"
```

---

## Task 3: ConfigStore

**Files:**
- Create: `backend/internal/singbox/config_store.go`
- Test: `backend/internal/singbox/config_store_test.go`

- [ ] **Step 1: 写失败测试**

```go
package singbox

import (
	"path/filepath"
	"testing"
)

func TestConfigStoreSaveAndGet(t *testing.T) {
	dir := t.TempDir()
	cs := NewConfigStore(filepath.Join(dir, "config.json"))
	if cs.Exists() {
		t.Fatal("should not exist initially")
	}
	if got, err := cs.Get(); err != nil || got != "" {
		t.Fatalf("empty Get = %q, %v", got, err)
	}
	if err := cs.Save(`{"log":{"level":"info"}}`); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !cs.Exists() {
		t.Fatal("should exist after save")
	}
	got, err := cs.Get()
	if err != nil || got != `{"log":{"level":"info"}}` {
		t.Fatalf("Get = %q, %v", got, err)
	}
}

func TestConfigStoreRejectsInvalidJSON(t *testing.T) {
	cs := NewConfigStore(filepath.Join(t.TempDir(), "config.json"))
	if err := cs.Save("{not json"); err != ErrInvalidJSON {
		t.Fatalf("err = %v, want ErrInvalidJSON", err)
	}
	if cs.Exists() {
		t.Fatal("invalid save must not create the file")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/singbox/ -run TestConfigStore`
Expected: FAIL（`NewConfigStore` 未定义）

- [ ] **Step 3: 实现**

```go
package singbox

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type ConfigStore struct{ path string }

func NewConfigStore(path string) *ConfigStore { return &ConfigStore{path: path} }

func (c *ConfigStore) Path() string  { return c.path }
func (c *ConfigStore) Exists() bool  { _, err := os.Stat(c.path); return err == nil }

func (c *ConfigStore) Get() (string, error) {
	b, err := os.ReadFile(c.path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (c *ConfigStore) Save(content string) error {
	if !json.Valid([]byte(content)) {
		return ErrInvalidJSON
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0755); err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0600); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd backend && go test ./internal/singbox/ -run TestConfigStore`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/singbox/config_store.go backend/internal/singbox/config_store_test.go
git commit -m "feat(singbox): ConfigStore with JSON-validated atomic save"
```

---

## Task 4: ProcessManager

**Files:**
- Create: `backend/internal/singbox/process.go`
- Test: `backend/internal/singbox/process_test.go`

- [ ] **Step 1: 写失败测试**（含 fakeEnv，供后续任务复用）

```go
package singbox

import (
	"path/filepath"
	"syscall"
	"testing"
)

// fakeEnv is a controllable Env for tests.
type fakeEnv struct {
	binPath      string // LookPath result ("" => not found)
	existing     map[string]bool
	versionOut   string
	versionOK    bool
	checkOut     string
	checkErr     error
	spawnPid     int
	spawnErr     error
	alive        map[int]bool
	pgrep        bool
	signals      []struct {
		pid int
		sig syscall.Signal
	}
}

func newFakeEnv() *fakeEnv {
	return &fakeEnv{existing: map[string]bool{}, alive: map[int]bool{}}
}
func (f *fakeEnv) LookPath(string) (string, bool) { return f.binPath, f.binPath != "" }
func (f *fakeEnv) FileExists(p string) bool        { return f.existing[p] }
func (f *fakeEnv) RunVersion(string) (string, bool) { return f.versionOut, f.versionOK }
func (f *fakeEnv) Check(string, string) (string, error) { return f.checkOut, f.checkErr }
func (f *fakeEnv) Spawn(string, string, string) (int, error) { return f.spawnPid, f.spawnErr }
func (f *fakeEnv) Alive(pid int) bool { return f.alive[pid] }
func (f *fakeEnv) Pgrep(string) bool  { return f.pgrep }
func (f *fakeEnv) Signal(pid int, sig syscall.Signal) error {
	f.signals = append(f.signals, struct {
		pid int
		sig syscall.Signal
	}{pid, sig})
	if sig == syscall.SIGTERM {
		f.alive[pid] = false // model graceful exit
	}
	return nil
}

func newPM(t *testing.T, env Env) *ProcessManager {
	dir := t.TempDir()
	return NewProcessManager(env, filepath.Join(dir, "sing-box.pid"), filepath.Join(dir, "sing-box.log"))
}

func TestProcessStartSpawnsAndWritesPid(t *testing.T) {
	env := newFakeEnv()
	env.spawnPid = 4242
	env.alive[4242] = true
	pm := newPM(t, env)
	if err := pm.Start("/bin/sing-box", "/cfg.json"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !pm.Running() {
		t.Fatal("should be running after start")
	}
}

func TestProcessStartRejectsBadConfig(t *testing.T) {
	env := newFakeEnv()
	env.checkErr = errAny()
	env.checkOut = "config error: bad inbound"
	pm := newPM(t, env)
	err := pm.Start("/bin/sing-box", "/cfg.json")
	var ice *InvalidConfigError
	if !asInvalidConfig(err, &ice) || ice.Output != "config error: bad inbound" {
		t.Fatalf("err = %v, want InvalidConfigError with output", err)
	}
}

func TestProcessStartRejectsWhenRunning(t *testing.T) {
	env := newFakeEnv()
	env.pgrep = true // already running externally
	pm := newPM(t, env)
	if err := pm.Start("/b", "/c"); err != ErrAlreadyRunning {
		t.Fatalf("err = %v, want ErrAlreadyRunning", err)
	}
}

func TestProcessStopSignalsAndClears(t *testing.T) {
	env := newFakeEnv()
	env.spawnPid = 99
	env.alive[99] = true
	pm := newPM(t, env)
	_ = pm.Start("/b", "/c")
	if err := pm.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if pm.Running() {
		t.Fatal("should not be running after stop")
	}
	if len(env.signals) == 0 || env.signals[0].sig != syscall.SIGTERM {
		t.Fatalf("expected SIGTERM, got %+v", env.signals)
	}
}

func TestProcessStopWhenNotRunning(t *testing.T) {
	pm := newPM(t, newFakeEnv())
	if err := pm.Stop(); err != ErrNotRunning {
		t.Fatalf("err = %v, want ErrNotRunning", err)
	}
}

// helpers
func errAny() error { return &simpleErr{"exit 1"} }

type simpleErr struct{ s string }

func (e *simpleErr) Error() string { return e.s }

func asInvalidConfig(err error, target **InvalidConfigError) bool {
	ice, ok := err.(*InvalidConfigError)
	if ok {
		*target = ice
	}
	return ok
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/singbox/ -run TestProcess`
Expected: FAIL（`NewProcessManager` 未定义）

- [ ] **Step 3: 实现**

```go
package singbox

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type ProcessManager struct {
	env     Env
	pidPath string
	logPath string
}

func NewProcessManager(env Env, pidPath, logPath string) *ProcessManager {
	return &ProcessManager{env: env, pidPath: pidPath, logPath: logPath}
}

func (p *ProcessManager) Running() bool {
	if pid, ok := p.readPid(); ok && p.env.Alive(pid) {
		return true
	}
	return p.env.Pgrep("sing-box")
}

func (p *ProcessManager) Start(bin, configPath string) error {
	if p.Running() {
		return ErrAlreadyRunning
	}
	if out, err := p.env.Check(bin, configPath); err != nil {
		return &InvalidConfigError{Output: strings.TrimSpace(out)}
	}
	pid, err := p.env.Spawn(p.logPath, bin, configPath)
	if err != nil {
		return err
	}
	return p.writePid(pid)
}

func (p *ProcessManager) Stop() error {
	pid, ok := p.readPid()
	if !ok || !p.env.Alive(pid) {
		_ = os.Remove(p.pidPath)
		return ErrNotRunning
	}
	_ = p.env.Signal(pid, syscall.SIGTERM)
	for i := 0; i < 30; i++ {
		if !p.env.Alive(pid) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if p.env.Alive(pid) {
		_ = p.env.Signal(pid, syscall.SIGKILL)
	}
	return os.Remove(p.pidPath)
}

func (p *ProcessManager) readPid() (int, bool) {
	b, err := os.ReadFile(p.pidPath)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0, false
	}
	return pid, true
}

func (p *ProcessManager) writePid(pid int) error {
	return os.WriteFile(p.pidPath, []byte(strconv.Itoa(pid)), 0644)
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd backend && go test ./internal/singbox/ -run TestProcess`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/singbox/process.go backend/internal/singbox/process_test.go
git commit -m "feat(singbox): ProcessManager with PID-file supervision"
```

---

## Task 5: Service（编排 + Status）

**Files:**
- Create: `backend/internal/singbox/service.go`
- Test: `backend/internal/singbox/service_test.go`

- [ ] **Step 1: 写失败测试**（复用 Task 4 的 `fakeEnv`）

```go
package singbox

import (
	"path/filepath"
	"testing"
)

func newService(t *testing.T, env *fakeEnv) *Service {
	dir := t.TempDir()
	return New(env, dir, "")
}

func TestStatusInstalledWithConfig(t *testing.T) {
	env := newFakeEnv()
	env.binPath = "/usr/bin/sing-box"
	env.versionOut, env.versionOK = "sing-box version 1.14.0", true
	svc := newService(t, env)
	if err := svc.SaveConfig(`{"log":{}}`); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	st := svc.Status()
	if !st.Installed || st.Version != "1.14.0" || !st.HasConfig {
		t.Fatalf("status = %+v", st)
	}
	if st.Running {
		t.Fatal("should not be running")
	}
}

func TestStartNotInstalled(t *testing.T) {
	env := newFakeEnv() // binPath empty, pgrep false
	svc := newService(t, env)
	if _, err := svc.Start(); err != ErrNotInstalled {
		t.Fatalf("err = %v, want ErrNotInstalled", err)
	}
}

func TestStartNoConfig(t *testing.T) {
	env := newFakeEnv()
	env.binPath = "/usr/bin/sing-box"
	svc := newService(t, env)
	if _, err := svc.Start(); err != ErrNoConfig {
		t.Fatalf("err = %v, want ErrNoConfig", err)
	}
}

func TestStartSuccessReturnsRunningStatus(t *testing.T) {
	env := newFakeEnv()
	env.binPath = "/usr/bin/sing-box"
	env.versionOut, env.versionOK = "sing-box version 1.14.0", true
	env.spawnPid = 7
	env.alive[7] = true
	svc := newService(t, env)
	_ = svc.SaveConfig(`{"log":{}}`)
	st, err := svc.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !st.Running {
		t.Fatalf("status not running: %+v", st)
	}
}

// sanity: managed bin path under dir is preferred and counts as installed
func TestResolvePrefersManagedBin(t *testing.T) {
	env := newFakeEnv()
	dir := t.TempDir()
	managed := filepath.Join(dir, "bin", "sing-box")
	env.existing[managed] = true
	svc := New(env, dir, "")
	if svc.Status().Installed != true {
		t.Fatal("managed bin should count as installed")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/singbox/ -run 'TestStatus|TestStart|TestResolve'`
Expected: FAIL（`New` 未定义）

- [ ] **Step 3: 实现**

```go
package singbox

import (
	"path/filepath"
	"regexp"
	"sync"
)

type Status struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	Running   bool   `json:"running"`
	HasConfig bool   `json:"hasConfig"`
}

type Service struct {
	env         Env
	dir         string
	binOverride string
	store       *ConfigStore
	pm          *ProcessManager
	mu          sync.Mutex
}

// New builds a Service from an injected Env (used by tests).
func New(env Env, dir, binOverride string) *Service {
	return &Service{
		env:         env,
		dir:         dir,
		binOverride: binOverride,
		store:       NewConfigStore(filepath.Join(dir, "config.json")),
		pm:          NewProcessManager(env, filepath.Join(dir, "sing-box.pid"), filepath.Join(dir, "sing-box.log")),
	}
}

// NewDefault wires the production OS-backed Env.
func NewDefault(dir, binOverride string) *Service { return New(NewOSEnv(), dir, binOverride) }

var versionRe = regexp.MustCompile(`(\d+\.\d+\.\d+)`)

func (s *Service) resolveBin() string {
	if s.binOverride != "" && s.env.FileExists(s.binOverride) {
		return s.binOverride
	}
	managed := filepath.Join(s.dir, "bin", "sing-box")
	if s.env.FileExists(managed) {
		return managed
	}
	if p, ok := s.env.LookPath("sing-box"); ok {
		return p
	}
	return ""
}

func (s *Service) statusLocked() Status {
	bin := s.resolveBin()
	st := Status{HasConfig: s.store.Exists(), Running: s.pm.Running()}
	if bin != "" {
		st.Installed = true
		if out, ok := s.env.RunVersion(bin); ok {
			st.Version = versionRe.FindString(out)
		}
	}
	return st
}

func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statusLocked()
}

func (s *Service) Start() (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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

func (s *Service) Stop() (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.pm.Stop()
	return s.statusLocked(), err
}

func (s *Service) GetConfig() (string, error)   { return s.store.Get() }
func (s *Service) SaveConfig(c string) error    { return s.store.Save(c) }
```

- [ ] **Step 4: 运行确认通过**

Run: `cd backend && go test ./internal/singbox/`
Expected: PASS（全部 singbox 测试）

- [ ] **Step 5: 提交**

```bash
git add backend/internal/singbox/service.go backend/internal/singbox/service_test.go
git commit -m "feat(singbox): Service orchestration + Status"
```

---

## Task 6: HTTP handlers

**Files:**
- Create: `backend/internal/handlers/singbox.go`
- Test: `backend/internal/handlers/singbox_test.go`

- [ ] **Step 1: 写失败测试**

```go
package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"singbox-admin/internal/singbox"
)

type fakeCtrl struct {
	status     singbox.Status
	startErr   error
	stopErr    error
	config     string
	saveErr    error
	savedWith  string
}

func (f *fakeCtrl) Status() singbox.Status              { return f.status }
func (f *fakeCtrl) Start() (singbox.Status, error)      { return f.status, f.startErr }
func (f *fakeCtrl) Stop() (singbox.Status, error)       { return f.status, f.stopErr }
func (f *fakeCtrl) GetConfig() (string, error)          { return f.config, nil }
func (f *fakeCtrl) SaveConfig(c string) error           { f.savedWith = c; return f.saveErr }

func newRouter(ctrl SingboxController) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewSingboxHandler(ctrl)
	r := gin.New()
	r.GET("/api/status", h.Status)
	r.POST("/api/singbox/start", h.Start)
	r.POST("/api/singbox/stop", h.Stop)
	r.GET("/api/singbox/config", h.GetConfig)
	r.PUT("/api/singbox/config", h.PutConfig)
	return r
}

func do(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	var rd *strings.Reader
	if body != "" {
		rd = strings.NewReader(body)
	} else {
		rd = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestStatusEndpoint(t *testing.T) {
	r := newRouter(&fakeCtrl{status: singbox.Status{Installed: true, Version: "1.14.0", HasConfig: true}})
	w := do(r, http.MethodGet, "/api/status", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"version":"1.14.0"`) || !strings.Contains(w.Body.String(), `"hasConfig":true`) {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestStartInvalidConfig(t *testing.T) {
	r := newRouter(&fakeCtrl{startErr: &singbox.InvalidConfigError{Output: "bad inbound"}})
	w := do(r, http.MethodPost, "/api/singbox/start", "")
	if w.Code != 400 || !strings.Contains(w.Body.String(), "bad inbound") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestStartAlreadyRunning(t *testing.T) {
	r := newRouter(&fakeCtrl{startErr: singbox.ErrAlreadyRunning})
	w := do(r, http.MethodPost, "/api/singbox/start", "")
	if w.Code != 409 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestStopNotRunning(t *testing.T) {
	r := newRouter(&fakeCtrl{stopErr: singbox.ErrNotRunning})
	w := do(r, http.MethodPost, "/api/singbox/stop", "")
	if w.Code != 409 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestPutConfigInvalidJSON(t *testing.T) {
	r := newRouter(&fakeCtrl{saveErr: singbox.ErrInvalidJSON})
	w := do(r, http.MethodPut, "/api/singbox/config", `{"content":"{bad"}`)
	if w.Code != 400 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestPutConfigOK(t *testing.T) {
	ctrl := &fakeCtrl{}
	r := newRouter(ctrl)
	w := do(r, http.MethodPut, "/api/singbox/config", `{"content":"{\"log\":{}}"}`)
	if w.Code != 200 || ctrl.savedWith != `{"log":{}}` {
		t.Fatalf("code=%d saved=%q", w.Code, ctrl.savedWith)
	}
}

func TestGetConfig(t *testing.T) {
	r := newRouter(&fakeCtrl{config: `{"log":{}}`})
	w := do(r, http.MethodGet, "/api/singbox/config", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"content"`) {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/handlers/ -run 'TestStatusEndpoint|TestStart|TestStop|TestPutConfig|TestGetConfig'`
Expected: FAIL（`NewSingboxHandler` 未定义）

- [ ] **Step 3: 实现**

```go
package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"singbox-admin/internal/singbox"
)

// SingboxController is the subset of *singbox.Service the handlers use.
type SingboxController interface {
	Status() singbox.Status
	Start() (singbox.Status, error)
	Stop() (singbox.Status, error)
	GetConfig() (string, error)
	SaveConfig(content string) error
}

type SingboxHandler struct{ ctrl SingboxController }

func NewSingboxHandler(c SingboxController) *SingboxHandler { return &SingboxHandler{ctrl: c} }

func (h *SingboxHandler) Status(c *gin.Context) {
	c.JSON(http.StatusOK, h.ctrl.Status())
}

func (h *SingboxHandler) Start(c *gin.Context) {
	st, err := h.ctrl.Start()
	if err != nil {
		writeStartError(c, err)
		return
	}
	c.JSON(http.StatusOK, st)
}

func (h *SingboxHandler) Stop(c *gin.Context) {
	st, err := h.ctrl.Stop()
	if err != nil {
		if errors.Is(err, singbox.ErrNotRunning) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, st)
}

func (h *SingboxHandler) GetConfig(c *gin.Context) {
	content, err := h.ctrl.GetConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"content": content})
}

type configBody struct {
	Content string `json:"content"`
}

func (h *SingboxHandler) PutConfig(c *gin.Context) {
	var body configBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content required"})
		return
	}
	if err := h.ctrl.SaveConfig(body.Content); err != nil {
		if errors.Is(err, singbox.ErrInvalidJSON) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func writeStartError(c *gin.Context, err error) {
	var ice *singbox.InvalidConfigError
	switch {
	case errors.As(err, &ice):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config", "detail": ice.Output})
	case errors.Is(err, singbox.ErrNotInstalled), errors.Is(err, singbox.ErrNoConfig):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, singbox.ErrAlreadyRunning):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd backend && go test ./internal/handlers/ -run 'TestStatusEndpoint|TestStart|TestStop|TestPutConfig|TestGetConfig'`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add backend/internal/handlers/singbox.go backend/internal/handlers/singbox_test.go
git commit -m "feat(handlers): sing-box status/start/stop/config endpoints"
```

---

## Task 7: 接线 main.go，移除旧 internal/service

**Files:**
- Modify: `backend/cmd/server/main.go`
- Delete: `backend/internal/service/singbox.go`, `backend/internal/service/singbox_test.go`
- Delete: `backend/internal/handlers/status.go`, `backend/internal/handlers/status_test.go`

> 旧 `status` handler/test 与 `internal/service` 被新 `internal/singbox` + 新 handler 取代。

- [ ] **Step 1: 删除旧文件**

```bash
git rm backend/internal/service/singbox.go backend/internal/service/singbox_test.go \
       backend/internal/handlers/status.go backend/internal/handlers/status_test.go
```

- [ ] **Step 2: 改 main.go**

把 `main.go` 中 `service` 相关行替换。完整新 `main.go`：

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
	sbHandler := handlers.NewSingboxHandler(singbox.NewDefault(cfg.SingboxDir, cfg.SingboxBin))

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
	}

	web.Register(r) // static frontend + SPA fallback

	log.Printf("listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 3: 校验编译 + 全后端测试**

Run: `cd backend && go build ./... && go vet ./... && go test ./...`
Expected: 编译通过，所有包测试 PASS（无 `internal/service` 残留引用）

- [ ] **Step 4: 单二进制冒烟（status 需鉴权）**

Run:
```bash
cd backend && JWT_SECRET=t DB_PATH=/tmp/m2.db SINGBOX_DIR=/tmp/m2sb go build -o /tmp/m2srv ./cmd/server && (/tmp/m2srv &) && sleep 1
curl -s -o /dev/null -w "status no-auth: %{http_code}\n" localhost:8080/api/status
curl -s -c /tmp/m2.cookie -X POST localhost:8080/api/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"mnice7082"}' >/dev/null
curl -s -b /tmp/m2.cookie localhost:8080/api/status; echo
pkill -f /tmp/m2srv; rm -f /tmp/m2.db /tmp/m2.cookie; rm -rf /tmp/m2sb
```
Expected: 第一行 401；第二个 curl 返回含 `"hasConfig":false`（无 sing-box 时 `installed` 视本机而定）。

- [ ] **Step 5: 提交**

```bash
git add backend
git commit -m "refactor(backend): wire internal/singbox, drop internal/service"
```

---

## Task 8: 部署改动（内置 sing-box 1.14.0 + SINGBOX_DIR）

**Files:**
- Modify: `docker-compose.yml`
- Modify: `scripts/deploy-native.sh`

- [ ] **Step 1: docker-compose.yml**

把 `SING_BOX_IMAGE` 默认改为钉版，并加 `SINGBOX_DIR`。`args` 段：
```yaml
        SING_BOX_IMAGE: ${SING_BOX_IMAGE:-ghcr.io/sagernet/sing-box:v1.14.0}
```
`environment` 段加一行：
```yaml
      SINGBOX_DIR: /data/singbox
```

- [ ] **Step 2: deploy-native.sh — 打包 sing-box 并设置 SINGBOX_DIR**

在 `scripts/deploy-native.sh` 顶部变量区（`SSH_PORT=...` 之后）加：
```bash
SINGBOX_VERSION="${SINGBOX_VERSION:-1.14.0}"
```
在「上传 binary」之前插入一段：下载对应架构 sing-box 并上传到 `$DEPLOY_PATH/data/singbox/bin/`：
```bash
echo "==> fetching sing-box ${SINGBOX_VERSION} (linux/${GOARCH})"
SB_TMP="$(mktemp -d)"
curl -fsSL "https://github.com/SagerNet/sing-box/releases/download/v${SINGBOX_VERSION}/sing-box-${SINGBOX_VERSION}-linux-${GOARCH}.tar.gz" \
  | tar -xz -C "$SB_TMP"
ssh -p "$SSH_PORT" "$DEPLOY_HOST" "mkdir -p '$DEPLOY_PATH/data/singbox/bin'"
scp -P "$SSH_PORT" "$SB_TMP/sing-box-${SINGBOX_VERSION}-linux-${GOARCH}/sing-box" \
  "$DEPLOY_HOST:$DEPLOY_PATH/data/singbox/bin/sing-box"
ssh -p "$SSH_PORT" "$DEPLOY_HOST" "chmod +x '$DEPLOY_PATH/data/singbox/bin/sing-box'"
rm -rf "$SB_TMP"
```
在写 systemd unit 的 heredoc 里，`Environment=DB_PATH=...` 之后加一行：
```
Environment=SINGBOX_DIR=$DEPLOY_PATH/data/singbox
```

- [ ] **Step 3: 语法校验**

Run: `bash -n scripts/deploy-native.sh && echo OK`
Expected: `OK`

- [ ] **Step 4: 提交**

```bash
git add docker-compose.yml scripts/deploy-native.sh
git commit -m "build: bundle sing-box 1.14.0 at deploy, set SINGBOX_DIR"
```

---

## Task 9: 前端 API 客户端

**Files:**
- Modify: `frontend/lib/api.ts`
- Modify: `frontend/lib/api.test.ts`

- [ ] **Step 1: 追加失败测试**（在 `api.test.ts` 的 `describe` 内）

```ts
  it("startSingbox posts and returns status", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ installed: true, version: "1.14.0", running: true, hasConfig: true }), { status: 200 })
      )
    );
    const st = await startSingbox();
    expect(st.running).toBe(true);
  });

  it("startSingbox throws with backend detail on 400", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: "invalid config", detail: "bad inbound" }), { status: 400 }))
    );
    await expect(startSingbox()).rejects.toThrow(/bad inbound/);
  });

  it("getConfig returns content string", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ content: "{}" }), { status: 200 })));
    expect(await getConfig()).toBe("{}");
  });

  it("saveConfig PUTs content", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}", { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    await saveConfig('{"log":{}}');
    const [, init] = fetchMock.mock.calls[0];
    expect(init.method).toBe("PUT");
    expect(JSON.parse(init.body).content).toBe('{"log":{}}');
  });
```

并在 `api.test.ts` 顶部 import 增补：
```ts
import { login, getStatus, UnauthorizedError, startSingbox, getConfig, saveConfig } from "./api";
```

- [ ] **Step 2: 运行确认失败**

Run: `cd frontend && npm test -- lib/api`
Expected: FAIL（`startSingbox` 未导出）

- [ ] **Step 3: 实现** — 在 `frontend/lib/api.ts` 修改 `SingboxStatus` 并追加函数。

把 `SingboxStatus` 接口改为：
```ts
export interface SingboxStatus {
  installed: boolean;
  version: string;
  running: boolean;
  hasConfig: boolean;
}
```
文件末尾追加：
```ts
async function statusAction(path: string): Promise<SingboxStatus> {
  const res = await request(path, { method: "POST" });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.detail || body.error || "操作失败");
  }
  return res.json();
}

export function startSingbox(): Promise<SingboxStatus> {
  return statusAction("/api/singbox/start");
}

export function stopSingbox(): Promise<SingboxStatus> {
  return statusAction("/api/singbox/stop");
}

export async function getConfig(): Promise<string> {
  const res = await request("/api/singbox/config");
  if (!res.ok) throw new Error("加载配置失败");
  const body = await res.json();
  return body.content as string;
}

export async function saveConfig(content: string): Promise<void> {
  const res = await request("/api/singbox/config", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ content }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || "保存失败");
  }
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd frontend && npm test -- lib/api`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add frontend/lib/api.ts frontend/lib/api.test.ts
git commit -m "feat(frontend): api client for singbox start/stop/config + hasConfig"
```

---

## Task 10: 后台 shell 组件（侧边栏+顶栏+鉴权）

**Files:**
- Create: `frontend/components/app-shell.tsx`
- Test: `frontend/components/app-shell.test.tsx`

> 实现期可调用 `frontend-design` 技能润色视觉；保持下方结构与文案（中文）与 data-testid 不变以过测试。

- [ ] **Step 1: 写失败测试**

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AppShell } from "./app-shell";

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
  usePathname: () => "/dashboard",
}));
const getStatusMock = vi.fn().mockResolvedValue({ installed: true, version: "1.14.0", running: false, hasConfig: false });
const logoutMock = vi.fn().mockResolvedValue(undefined);
vi.mock("@/lib/api", () => ({
  getStatus: () => getStatusMock(),
  logout: () => logoutMock(),
  UnauthorizedError: class extends Error {},
}));

beforeEach(() => {
  pushMock.mockClear();
  logoutMock.mockClear();
});

describe("AppShell", () => {
  it("renders nav items 概览 and 配置", () => {
    render(<AppShell><div>内容</div></AppShell>);
    expect(screen.getByRole("link", { name: /概览/ })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /配置/ })).toBeInTheDocument();
    expect(screen.getByText("内容")).toBeInTheDocument();
  });

  it("logout button calls logout and redirects", async () => {
    render(<AppShell><div /></AppShell>);
    await userEvent.click(screen.getByRole("button", { name: /退出登录/ }));
    expect(logoutMock).toHaveBeenCalled();
    expect(pushMock).toHaveBeenCalledWith("/login");
  });
});
```

- [ ] **Step 2: 运行确认失败**

Run: `cd frontend && npm test -- components/app-shell`
Expected: FAIL（模块不存在）

- [ ] **Step 3: 实现**

```tsx
"use client";

import { useEffect } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { getStatus, logout, UnauthorizedError } from "@/lib/api";

const NAV = [
  { href: "/dashboard", label: "概览" },
  { href: "/config", label: "配置" },
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
    <div className="flex min-h-screen bg-background text-foreground">
      <aside className="flex w-56 shrink-0 flex-col border-r border-border bg-card">
        <div className="px-5 py-5">
          <span className="font-mono text-sm tracking-[0.18em] text-foreground uppercase">
            sing-box admin
          </span>
        </div>
        <nav className="flex flex-col gap-1 px-3">
          {NAV.map((item) => {
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
        </nav>
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-14 items-center justify-between border-b border-border px-6">
          <span className="text-sm text-muted-foreground">sing-box 管理面板</span>
          <button
            onClick={onLogout}
            className="rounded-full border border-border px-3 py-1 text-sm text-foreground transition-colors hover:bg-secondary"
          >
            退出登录
          </button>
        </header>
        <main className="min-w-0 flex-1 px-6 py-8">
          <div className="mx-auto max-w-4xl">{children}</div>
        </main>
      </div>
    </div>
  );
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd frontend && npm test -- components/app-shell`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add frontend/components/app-shell.tsx frontend/components/app-shell.test.tsx
git commit -m "feat(frontend): B-end app shell (sidebar/topbar/auth gate)"
```

---

## Task 11: 仪表盘（概览：状态 + 启停，套 shell，中文）

**Files:**
- Modify: `frontend/app/dashboard/page.tsx`
- Modify: `frontend/app/dashboard/page.test.tsx`

- [ ] **Step 1: 改测试为中文 + 启停**（整体替换 `dashboard/page.test.tsx`）

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import DashboardPage from "./page";

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
  usePathname: () => "/dashboard",
}));

const getStatusMock = vi.fn();
const startMock = vi.fn();
const stopMock = vi.fn();
vi.mock("@/lib/api", () => ({
  getStatus: () => getStatusMock(),
  startSingbox: () => startMock(),
  stopSingbox: () => stopMock(),
  logout: vi.fn(),
  UnauthorizedError: class extends Error {},
}));

beforeEach(() => {
  pushMock.mockClear();
  getStatusMock.mockReset();
  startMock.mockReset();
  stopMock.mockReset();
});

describe("DashboardPage", () => {
  it("显示运行状态与版本", async () => {
    getStatusMock.mockResolvedValue({ installed: true, version: "1.14.0", running: true, hasConfig: true });
    render(<DashboardPage />);
    await waitFor(() => expect(screen.getByText(/运行中/)).toBeInTheDocument());
    expect(screen.getByText(/1\.14\.0/)).toBeInTheDocument();
  });

  it("点击启动调用 startSingbox", async () => {
    getStatusMock.mockResolvedValue({ installed: true, version: "1.14.0", running: false, hasConfig: true });
    startMock.mockResolvedValue({ installed: true, version: "1.14.0", running: true, hasConfig: true });
    render(<DashboardPage />);
    const btn = await screen.findByRole("button", { name: /启动/ });
    await userEvent.click(btn);
    expect(startMock).toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: 运行确认失败**

Run: `cd frontend && npm test -- app/dashboard`
Expected: FAIL（页面仍是旧英文/无启动按钮）

- [ ] **Step 3: 实现**（整体替换 `dashboard/page.tsx`）

```tsx
"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { getStatus, startSingbox, stopSingbox, UnauthorizedError, SingboxStatus } from "@/lib/api";
import { AppShell } from "@/components/app-shell";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function DashboardPage() {
  const router = useRouter();
  const [status, setStatus] = useState<SingboxStatus | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const refresh = useCallback(() => {
    getStatus()
      .then(setStatus)
      .catch((err) => {
        if (err instanceof UnauthorizedError) router.push("/login");
      });
  }, [router]);

  useEffect(refresh, [refresh]);

  async function run(action: () => Promise<SingboxStatus>) {
    setBusy(true);
    setError("");
    try {
      setStatus(await action());
    } catch (e) {
      setError(e instanceof Error ? e.message : "操作失败");
    } finally {
      setBusy(false);
    }
  }

  const row = (label: string, value: React.ReactNode) => (
    <div className="flex items-center justify-between py-3">
      <span className="font-mono text-xs tracking-wider text-muted-foreground uppercase">{label}</span>
      <span className="text-sm">{value}</span>
    </div>
  );

  return (
    <AppShell>
      <h1 className="mb-6 text-2xl font-normal tracking-tight">概览</h1>
      <Card className="rounded-lg">
        <CardHeader>
          <CardTitle className="font-mono text-xs tracking-wider text-muted-foreground uppercase">
            sing-box 状态
          </CardTitle>
        </CardHeader>
        <CardContent>
          {status === null ? (
            <p className="text-sm text-muted-foreground">加载中…</p>
          ) : (
            <>
              <div className="divide-y divide-border">
                {row("已安装", status.installed ? "是" : "否")}
                {row("版本", <span className="font-mono">{status.version || "—"}</span>)}
                {row("配置", status.hasConfig ? "已就绪" : "未配置")}
                {row(
                  "运行状态",
                  <span className="inline-flex items-center gap-2">
                    <span
                      className="size-1.5 rounded-full"
                      style={{ background: status.running ? "var(--sunset)" : "var(--muted-foreground)" }}
                    />
                    {status.running ? "运行中" : "已停止"}
                  </span>
                )}
              </div>
              {error && <p className="mt-4 text-sm text-destructive">{error}</p>}
              <div className="mt-6 flex gap-3">
                <Button
                  className="rounded-full"
                  disabled={busy || !status.installed || !status.hasConfig || status.running}
                  onClick={() => run(startSingbox)}
                >
                  启动
                </Button>
                <Button
                  variant="outline"
                  className="rounded-full"
                  disabled={busy || !status.running}
                  onClick={() => run(stopSingbox)}
                >
                  停止
                </Button>
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </AppShell>
  );
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd frontend && npm test -- app/dashboard`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add frontend/app/dashboard
git commit -m "feat(frontend): dashboard overview with start/stop (Chinese, in shell)"
```

---

## Task 12: 配置页（原始 JSON 编辑器）

**Files:**
- Create: `frontend/app/config/page.tsx`
- Test: `frontend/app/config/page.test.tsx`

- [ ] **Step 1: 写失败测试**

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import ConfigPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/config",
}));
const getConfigMock = vi.fn();
const saveConfigMock = vi.fn();
vi.mock("@/lib/api", () => ({
  getConfig: () => getConfigMock(),
  saveConfig: (c: string) => saveConfigMock(c),
  getStatus: vi.fn().mockResolvedValue({ installed: true, version: "1.14.0", running: false, hasConfig: true }),
  logout: vi.fn(),
  UnauthorizedError: class extends Error {},
}));

beforeEach(() => {
  getConfigMock.mockReset();
  saveConfigMock.mockReset();
});

describe("ConfigPage", () => {
  it("载入既有配置到文本框", async () => {
    getConfigMock.mockResolvedValue('{"log":{}}');
    render(<ConfigPage />);
    await waitFor(() => expect(screen.getByRole("textbox")).toHaveValue('{"log":{}}'));
  });

  it("保存调用 saveConfig", async () => {
    getConfigMock.mockResolvedValue("");
    saveConfigMock.mockResolvedValue(undefined);
    render(<ConfigPage />);
    const ta = await screen.findByRole("textbox");
    await userEvent.type(ta, '{{"log":{{}}}');
    await userEvent.click(screen.getByRole("button", { name: /保存配置/ }));
    await waitFor(() => expect(saveConfigMock).toHaveBeenCalled());
  });
});
```

- [ ] **Step 2: 运行确认失败**

Run: `cd frontend && npm test -- app/config`
Expected: FAIL（页面不存在）

- [ ] **Step 3: 实现**

```tsx
"use client";

import { useEffect, useState } from "react";
import { getConfig, saveConfig } from "@/lib/api";
import { AppShell } from "@/components/app-shell";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function ConfigPage() {
  const [content, setContent] = useState("");
  const [error, setError] = useState("");
  const [ok, setOk] = useState(false);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    getConfig().then(setContent).catch(() => setError("加载配置失败"));
  }, []);

  async function onSave() {
    setBusy(true);
    setError("");
    setOk(false);
    try {
      await saveConfig(content);
      setOk(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : "保存失败");
    } finally {
      setBusy(false);
    }
  }

  return (
    <AppShell>
      <h1 className="mb-6 text-2xl font-normal tracking-tight">配置</h1>
      <Card className="rounded-lg">
        <CardHeader>
          <CardTitle className="font-mono text-xs tracking-wider text-muted-foreground uppercase">
            config.json
          </CardTitle>
        </CardHeader>
        <CardContent>
          <textarea
            value={content}
            onChange={(e) => setContent(e.target.value)}
            spellCheck={false}
            className="h-96 w-full rounded-lg border border-input bg-secondary p-3 font-mono text-sm text-foreground outline-none focus-visible:border-ring"
            placeholder='{ "log": { "level": "info" } }'
          />
          {error && <p className="mt-3 text-sm text-destructive">{error}</p>}
          {ok && <p className="mt-3 text-sm text-muted-foreground">已保存</p>}
          <div className="mt-4">
            <Button className="rounded-full" disabled={busy} onClick={onSave}>
              保存配置
            </Button>
          </div>
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
git commit -m "feat(frontend): config.json editor page"
```

---

## Task 13: 登录页大气化 + 中文

**Files:**
- Modify: `frontend/app/login/page.tsx`
- Modify: `frontend/app/login/page.test.tsx`

> 实现期可用 `frontend-design` 技能润色；保持下列中文文案与可访问性（label 关联）以过测试。

- [ ] **Step 1: 改测试为中文**（整体替换 `login/page.test.tsx`）

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
  it("渲染用户名和密码输入", () => {
    render(<LoginPage />);
    expect(screen.getByLabelText(/用户名/)).toBeInTheDocument();
    expect(screen.getByLabelText(/密码/)).toBeInTheDocument();
  });

  it("登录成功后跳转仪表盘", async () => {
    render(<LoginPage />);
    await userEvent.type(screen.getByLabelText(/用户名/), "admin");
    await userEvent.type(screen.getByLabelText(/密码/), "mnice7082");
    await userEvent.click(screen.getByRole("button", { name: /登录/ }));
    expect(pushMock).toHaveBeenCalledWith("/dashboard");
  });
});
```

- [ ] **Step 2: 运行确认失败**

Run: `cd frontend && npm test -- app/login`
Expected: FAIL（旧页面英文 label）

- [ ] **Step 3: 实现**（整体替换 `login/page.tsx`，左右分屏、中文、大气）

```tsx
"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { login } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

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
    <main className="grid min-h-screen bg-background lg:grid-cols-2">
      {/* 品牌区 */}
      <section className="hidden flex-col justify-between border-r border-border p-12 lg:flex">
        <span className="font-mono text-xs tracking-[0.2em] text-muted-foreground uppercase">
          sing-box admin
        </span>
        <div>
          <h1 className="max-w-md text-5xl leading-tight font-normal tracking-[-0.03em] text-foreground">
            掌控你的 sing-box 代理
          </h1>
          <p className="mt-5 max-w-sm text-base leading-relaxed text-muted-foreground">
            一处管理服务端配置、启停与运行状态。
          </p>
        </div>
        <span className="font-mono text-xs tracking-wider text-muted-foreground">
          自建 · 单二进制 · 部署即用
        </span>
      </section>

      {/* 表单区 */}
      <section className="flex items-center justify-center p-8">
        <div className="w-full max-w-sm">
          <p className="mb-2 font-mono text-xs tracking-[0.18em] text-muted-foreground uppercase lg:hidden">
            sing-box admin
          </p>
          <h2 className="mb-1 text-2xl font-normal tracking-tight">登录</h2>
          <p className="mb-8 text-sm text-muted-foreground">使用管理员账号继续</p>
          <form onSubmit={onSubmit} className="space-y-5">
            <div className="space-y-2">
              <Label htmlFor="username" className="font-mono text-xs tracking-wider text-muted-foreground uppercase">
                用户名
              </Label>
              <Input id="username" className="h-11" value={username} onChange={(e) => setUsername(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="password" className="font-mono text-xs tracking-wider text-muted-foreground uppercase">
                密码
              </Label>
              <Input
                id="password"
                type="password"
                className="h-11"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
            <Button type="submit" className="h-11 w-full rounded-full" disabled={loading}>
              {loading ? "登录中…" : "登录"}
            </Button>
          </form>
        </div>
      </section>
    </main>
  );
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd frontend && npm test -- app/login`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add frontend/app/login
git commit -m "feat(frontend): grand split-screen login page (Chinese)"
```

---

## Task 14: 全量验证 + 截图

**Files:** 无（验证任务）

- [ ] **Step 1: 全后端测试**

Run: `cd backend && go test ./...`
Expected: 全 PASS

- [ ] **Step 2: 全前端测试 + 构建**

Run: `cd frontend && npm test && npm run build`
Expected: 测试全 PASS；构建成功，路由含 `/login` `/dashboard` `/config`

- [ ] **Step 3: 起静态服务并截图三页**（确认暗色 B 端视觉，非仅编译）

Run:
```bash
cd frontend && (python3 -m http.server 4173 -d out >/tmp/serve.log 2>&1 &) && sleep 1
```
用浏览器工具依次打开并截图：`http://localhost:4173/login.html`、`/dashboard.html`、`/config.html`，确认侧边栏+顶栏 shell、暗色、全中文。完成后 `pkill -f "http.server 4173"`。
（dashboard/config 无后端会停在"加载中…"或跳登录，属正常，主要看 shell 与登录页视觉。）

- [ ] **Step 4: 提交（若截图阶段做了样式微调）**

```bash
git add -A
git commit -m "chore(frontend): visual polish for M2 shell" || echo "no changes"
```

---

## Self-Review Notes

- **Spec 覆盖**：config 路径/二进制解析(Task1,5)、ConfigStore(Task3)、ProcessManager 启停/PID/检查/已运行/未运行(Task4)、Service 编排+Status.HasConfig(Task5)、5 端点+错误码映射(Task6)、main 接线去 service(Task7)、部署内置 1.14.0+SINGBOX_DIR(Task8)、前端 api(Task9)、B 端 shell(Task10)、概览启停(Task11)、配置编辑(Task12)、大气登录+全中文(Task13)、验证(Task14)。安装功能按规范已删除——无对应任务（正确）。
- **类型一致性**：`Status{Installed,Version,Running,HasConfig}` 前后端字段一致（Go json tag `hasConfig` ↔ TS `hasConfig`）；`SingboxController` 接口方法与 `*singbox.Service` 方法签名一致（`Start()/Stop() (Status,error)`、`GetConfig()(string,error)`、`SaveConfig(string)error`、`Status()Status`）；错误哨兵 `ErrNotInstalled/ErrNoConfig/ErrAlreadyRunning/ErrNotRunning/ErrInvalidJSON` 与 `*InvalidConfigError` 在 Task2 定义、Task4-6 使用一致。
- **占位符**：无 TBD/TODO；每个代码步骤含完整代码。
- **执行注意**：Task7 删除旧 `internal/service` 与 `status.go/_test.go` 后再编译；前端 shell/页面测试用中文断言，旧英文断言已在 Task11/13 同步替换。
```

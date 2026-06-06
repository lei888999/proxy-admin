package singbox

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestApplyConfigNotRunningOnlyPersists(t *testing.T) {
	env := newFakeEnv()
	env.binPath = "/usr/bin/sing-box"
	svc := newService(t, env)
	if err := svc.ApplyConfig(`{"log":{}}`); err != nil {
		t.Fatalf("ApplyConfig: %v", err)
	}
	if svc.Status().Running {
		t.Fatal("ApplyConfig must not start a stopped sing-box")
	}
	if env.spawnCount != 0 {
		t.Fatalf("spawnCount=%d, want 0 (no start when stopped)", env.spawnCount)
	}
	if got, _ := svc.GetConfig(); got == "" {
		t.Fatal("config was not persisted")
	}
}

func TestApplyConfigRunningValidRestarts(t *testing.T) {
	env := newFakeEnv()
	env.binPath = "/usr/bin/sing-box"
	env.spawnPid = 7
	env.alive[7] = true
	svc := newService(t, env)
	_ = svc.SaveConfig(`{"log":{}}`)
	if _, err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if env.spawnCount != 1 {
		t.Fatalf("after start spawnCount=%d, want 1", env.spawnCount)
	}
	if err := svc.ApplyConfig(`{"log":{"level":"info"}}`); err != nil {
		t.Fatalf("ApplyConfig: %v", err)
	}
	if env.spawnCount != 2 {
		t.Fatalf("spawnCount=%d, want 2 (restart applied the new config)", env.spawnCount)
	}
}

func TestApplyConfigRunningInvalidKeepsOldProcess(t *testing.T) {
	env := newFakeEnv()
	env.binPath = "/usr/bin/sing-box"
	env.spawnPid = 7
	env.alive[7] = true
	svc := newService(t, env)
	_ = svc.SaveConfig(`{"log":{}}`)
	if _, err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	env.checkErr = errors.New("config error")
	env.checkOut = "bad inbound"
	err := svc.ApplyConfig(`{"log":{}}`)
	var ice *InvalidConfigError
	if !errors.As(err, &ice) {
		t.Fatalf("err=%v, want *InvalidConfigError", err)
	}
	if env.spawnCount != 1 {
		t.Fatalf("spawnCount=%d, want 1 (must NOT restart on invalid config)", env.spawnCount)
	}
	if !svc.Status().Running {
		t.Fatal("a running sing-box must stay up when the new config is invalid")
	}
}

func newService(t *testing.T, env *fakeEnv) *Service {
	dir := t.TempDir()
	return New(env, dir, "")
}

func TestStatusInstalledWithConfig(t *testing.T) {
	env := newFakeEnv()
	env.binPath = "/usr/bin/sing-box"
	env.versionOut, env.versionOK = "sing-box version 1.13.13", true
	svc := newService(t, env)
	if err := svc.SaveConfig(`{"log":{}}`); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	st := svc.Status()
	if !st.Installed || st.Version != "1.13.13" || !st.HasConfig {
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
	env.versionOut, env.versionOK = "sing-box version 1.13.13", true
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

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

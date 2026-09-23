package singbox

import (
	"path/filepath"
	"testing"
	"time"
)

// A spawned child that exits must stop being reported as alive. Without the
// reaping goroutine in Spawn the child lingers as a zombie, its PID keeps
// answering signal 0, and the panel reports a dead sing-box as running forever —
// which also makes Start refuse with "already running" with no way out.
func TestOSEnvSpawnReapsExitedChild(t *testing.T) {
	env := NewOSEnv()
	logPath := filepath.Join(t.TempDir(), "child.log")
	// /bin/sh invoked with sing-box's argument shape fails and exits at once,
	// which is exactly the short-lived child we need.
	pid, err := env.Spawn(logPath, "/bin/sh", filepath.Join(t.TempDir(), "nonexistent.json"))
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !env.Alive(pid) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("Alive(%d) still true after the child exited (child was not reaped)", pid)
}

func TestOSEnvAliveRejectsBogusPid(t *testing.T) {
	env := NewOSEnv()
	if env.Alive(0) || env.Alive(-1) {
		t.Fatal("Alive must reject non-positive PIDs")
	}
}

package singbox

import (
	"path/filepath"
	"syscall"
	"testing"
)

// fakeEnv is a controllable Env for tests.
type fakeEnv struct {
	binPath    string // LookPath result ("" => not found)
	existing   map[string]bool
	versionOut string
	versionOK  bool
	checkOut   string
	checkErr   error
	spawnPid   int
	spawnErr   error
	alive      map[int]bool
	pgrep      bool
	signals    []struct {
		pid int
		sig syscall.Signal
	}
}

func newFakeEnv() *fakeEnv {
	return &fakeEnv{existing: map[string]bool{}, alive: map[int]bool{}}
}
func (f *fakeEnv) LookPath(string) (string, bool)       { return f.binPath, f.binPath != "" }
func (f *fakeEnv) FileExists(p string) bool             { return f.existing[p] }
func (f *fakeEnv) RunVersion(string) (string, bool)     { return f.versionOut, f.versionOK }
func (f *fakeEnv) Check(string, string) (string, error) { return f.checkOut, f.checkErr }
func (f *fakeEnv) Spawn(string, string, string) (int, error) {
	return f.spawnPid, f.spawnErr
}
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

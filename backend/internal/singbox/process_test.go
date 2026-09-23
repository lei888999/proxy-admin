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
	spawnCount int
	// spawnDies models a process that passes `check` but exits at once (a port
	// already bound), i.e. spawn succeeds yet the PID is never alive.
	spawnDies bool
	alive     map[int]bool
	cmdlines  map[int]string
	pgrep     bool
	signals   []struct {
		pid int
		sig syscall.Signal
	}
}

func newFakeEnv() *fakeEnv {
	return &fakeEnv{existing: map[string]bool{}, alive: map[int]bool{}, cmdlines: map[int]string{}}
}
func (f *fakeEnv) LookPath(string) (string, bool)       { return f.binPath, f.binPath != "" }
func (f *fakeEnv) FileExists(p string) bool             { return f.existing[p] }
func (f *fakeEnv) RunVersion(string) (string, bool)     { return f.versionOut, f.versionOK }
func (f *fakeEnv) Check(string, string) (string, error) { return f.checkOut, f.checkErr }
func (f *fakeEnv) Spawn(string, string, string) (int, error) {
	if f.spawnErr == nil {
		f.spawnCount++
		f.alive[f.spawnPid] = !f.spawnDies // model the newly started process being alive
	}
	return f.spawnPid, f.spawnErr
}
func (f *fakeEnv) Alive(pid int) bool { return f.alive[pid] }
func (f *fakeEnv) CommandLine(pid int) (string, bool) {
	s, ok := f.cmdlines[pid]
	return s, ok
}
func (f *fakeEnv) Pgrep(string) bool { return f.pgrep }
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

const testConfigPath = "/cfg.json"

func newPM(t *testing.T, env Env) *ProcessManager {
	dir := t.TempDir()
	return NewProcessManager(env, filepath.Join(dir, "sing-box.pid"), filepath.Join(dir, "sing-box.log"), testConfigPath)
}

func TestProcessStartSpawnsAndWritesPid(t *testing.T) {
	env := newFakeEnv()
	env.spawnPid = 4242
	env.alive[4242] = true
	pm := newPM(t, env)
	if err := pm.Start("/bin/sing-box", testConfigPath); err != nil {
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
	err := pm.Start("/bin/sing-box", testConfigPath)
	var ice *InvalidConfigError
	if !asInvalidConfig(err, &ice) || ice.Output != "config error: bad inbound" {
		t.Fatalf("err = %v, want InvalidConfigError with output", err)
	}
}

// An externally started sing-box must NOT block the panel from starting its own:
// the old pgrep fallback made Start return "already running" forever, with no way
// out of the UI.
func TestProcessStartIgnoresExternalProcess(t *testing.T) {
	env := newFakeEnv()
	env.pgrep = true // a sing-box the panel does not manage
	env.spawnPid = 31
	pm := newPM(t, env)
	if err := pm.Start("/b", testConfigPath); err != nil {
		t.Fatalf("Start with external process present: %v, want nil", err)
	}
}

func TestProcessStartRejectsWhenAlreadyStarted(t *testing.T) {
	env := newFakeEnv()
	env.spawnPid = 55
	pm := newPM(t, env)
	if err := pm.Start("/b", testConfigPath); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if err := pm.Start("/b", testConfigPath); err != ErrAlreadyRunning {
		t.Fatalf("err = %v, want ErrAlreadyRunning", err)
	}
}

// sing-box can pass `check` and still die on spawn (port already bound). Start
// must report that instead of leaving a pid file behind and claiming success.
func TestProcessStartReportsImmediateExit(t *testing.T) {
	env := newFakeEnv()
	env.spawnPid = 77
	env.spawnDies = true
	pm := newPM(t, env)
	err := pm.Start("/b", testConfigPath)
	var sfe *StartFailedError
	if !asStartFailed(err, &sfe) {
		t.Fatalf("err = %v, want *StartFailedError", err)
	}
	if pm.Running() {
		t.Fatal("must not report running after an immediate exit")
	}
}

func TestProcessStopSignalsAndClears(t *testing.T) {
	env := newFakeEnv()
	env.spawnPid = 99
	env.alive[99] = true
	pm := newPM(t, env)
	_ = pm.Start("/b", testConfigPath)
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

// ManagedRunning tracks only the panel-started (PID-file) process and must NOT
// report true for a sing-box merely detected on the host.
func TestProcessManagedRunning(t *testing.T) {
	env := newFakeEnv()
	pm := newPM(t, env)

	if pm.ManagedRunning() {
		t.Fatal("should not be managed-running before start")
	}

	// External sing-box only: not managed, and reported separately.
	env.pgrep = true
	if pm.ManagedRunning() {
		t.Fatal("external process must not count as managed-running")
	}
	if !pm.ExternalRunning() {
		t.Fatal("external process should be reported by ExternalRunning")
	}

	env.pgrep = false
	env.spawnPid = 4242
	if err := pm.Start("/bin/sing-box", testConfigPath); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !pm.ManagedRunning() {
		t.Fatal("should be managed-running after start")
	}
	if pm.ExternalRunning() {
		t.Fatal("our own process must not be reported as external")
	}
}

// A pid file left by a previous panel run points at a PID that may have been
// recycled. Without a command-line match it must not be treated as ours, and
// Stop must refuse rather than signal an unrelated process.
func TestProcessIgnoresRecycledPidFromPreviousRun(t *testing.T) {
	env := newFakeEnv()
	env.alive[1234] = true
	env.cmdlines[1234] = "/usr/bin/postgres -D /var/lib/pg"
	pm := newPM(t, env)
	if err := pm.writePid(1234); err != nil {
		t.Fatalf("writePid: %v", err)
	}
	if pm.ManagedRunning() {
		t.Fatal("an unrelated process must not count as our sing-box")
	}
	if err := pm.Stop(); err != ErrNotRunning {
		t.Fatalf("Stop = %v, want ErrNotRunning", err)
	}
	if len(env.signals) != 0 {
		t.Fatalf("must not signal an unrelated process, got %+v", env.signals)
	}
}

// The same pid file IS ours when the command line matches sing-box + our config.
func TestProcessAdoptsOwnPidAfterPanelRestart(t *testing.T) {
	env := newFakeEnv()
	env.alive[2345] = true
	env.cmdlines[2345] = "/usr/local/bin/sing-box run -c " + testConfigPath
	pm := newPM(t, env)
	if err := pm.writePid(2345); err != nil {
		t.Fatalf("writePid: %v", err)
	}
	if !pm.ManagedRunning() {
		t.Fatal("a matching sing-box from a previous panel run is ours")
	}
}

func TestProcessStopWhenNotRunning(t *testing.T) {
	pm := newPM(t, newFakeEnv())
	if err := pm.Stop(); err != ErrNotRunning {
		t.Fatalf("err = %v, want ErrNotRunning", err)
	}
}

// Stop must never reach a process the panel did not start.
func TestProcessStopLeavesExternalProcessAlone(t *testing.T) {
	env := newFakeEnv()
	env.pgrep = true // running externally, no PID file written by us
	pm := newPM(t, env)
	if err := pm.Stop(); err != ErrNotRunning {
		t.Fatalf("Stop = %v, want ErrNotRunning", err)
	}
	if !env.pgrep {
		t.Fatal("the external sing-box must still be running")
	}
	if len(env.signals) != 0 {
		t.Fatalf("must not signal an external process, got %+v", env.signals)
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

func asStartFailed(err error, target **StartFailedError) bool {
	sfe, ok := err.(*StartFailedError)
	if ok {
		*target = sfe
	}
	return ok
}

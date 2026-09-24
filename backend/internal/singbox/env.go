package singbox

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	// CommandLine returns pid's command line. Used to confirm a PID read from the
	// pid file really is our sing-box: PIDs are recycled, and a pid file left by
	// a previous panel process must not make an unrelated process look managed.
	CommandLine(pid int) (string, bool)
	// Pgrep reports whether ANY sing-box is running. READ-ONLY — it exists to
	// warn about a process the panel does not manage, never to adopt or kill one.
	Pgrep(name string) bool
	Signal(pid int, sig syscall.Signal) error
}

type osEnv struct {
	mu sync.Mutex
	// exited records children we spawned and have since reaped. A reaped child's
	// PID may be recycled, but until then signal 0 still succeeds for it, so this
	// is what keeps Alive from reporting a dead sing-box as running.
	exited map[int]bool
}

// NewOSEnv returns the production Env backed by the real OS.
func NewOSEnv() Env { return &osEnv{exited: map[int]bool{}} }

func (*osEnv) LookPath(file string) (string, bool) {
	p, err := exec.LookPath(file)
	return p, err == nil
}

func (*osEnv) FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (*osEnv) RunVersion(bin string) (string, bool) {
	out, err := exec.Command(bin, "version").CombinedOutput()
	if err != nil {
		return "", false
	}
	return string(out), true
}

func (*osEnv) Check(bin, configPath string) (string, error) {
	out, err := exec.Command(bin, "check", "-c", configPath).CombinedOutput()
	return string(out), err
}

func (e *osEnv) Spawn(logPath, bin, configPath string) (int, error) {
	// 0600: the log can carry connection detail, and on a shared VPS it should
	// not be world-readable.
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	cmd := exec.Command(bin, "run", "-c", configPath)
	// sing-box resolves relative paths such as experimental.cache_file.path from
	// its working directory. Keep those state files beside config.json so the
	// rule-set/fake-IP cache survives restarts and is covered by the persistent
	// singbox directory in both native and Docker deployments.
	cmd.Dir = filepath.Dir(configPath)
	cmd.Stdout = f
	cmd.Stderr = f
	// Detach from the panel's process group so the panel restarting/exiting
	// does not signal sing-box.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	e.mu.Lock()
	delete(e.exited, pid)
	e.mu.Unlock()
	// Reap the child. Without a Wait the exited process lingers as a zombie whose
	// PID still answers signal 0, so Alive — and therefore the panel's whole
	// "is it running" story — would report a crashed sing-box as healthy.
	go func() {
		_ = cmd.Wait()
		e.mu.Lock()
		e.exited[pid] = true
		e.mu.Unlock()
	}()
	return pid, nil
}

func (e *osEnv) Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	e.mu.Lock()
	exited := e.exited[pid]
	e.mu.Unlock()
	if exited {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

// CommandLine prefers /proc (Linux, and free of any external binary) and falls
// back to ps for platforms without it (macOS during local development).
func (*osEnv) CommandLine(pid int) (string, bool) {
	if b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline"); err == nil {
		s := strings.TrimSpace(string(bytes.ReplaceAll(bytes.TrimRight(b, "\x00"), []byte{0}, []byte{' '})))
		return s, s != ""
	}
	out, err := exec.Command("ps", "-o", "command=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return "", false
	}
	s := strings.TrimSpace(string(out))
	return s, s != ""
}

func (*osEnv) Pgrep(name string) bool {
	return exec.Command("pgrep", "-x", name).Run() == nil
}

func (*osEnv) Signal(pid int, sig syscall.Signal) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(sig)
}

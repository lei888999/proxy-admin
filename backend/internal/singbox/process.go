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
	if p.ManagedRunning() {
		return true
	}
	return p.env.Pgrep("sing-box")
}

// ManagedRunning reports whether the panel-started process (tracked by the PID
// file) is alive. Unlike Running it does NOT fall back to pgrep, so it excludes
// sing-box processes started outside the panel — used by the traffic poller,
// which only expects the panel's config (with clash_api on 9090) to be listening.
func (p *ProcessManager) ManagedRunning() bool {
	pid, ok := p.readPid()
	return ok && p.env.Alive(pid)
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
		// No live PID file, but a sing-box may be running that we adopted via
		// the pgrep fallback (started outside the panel). Stop it too.
		if p.env.Pgrep("sing-box") {
			return p.env.Pkill("sing-box")
		}
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

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

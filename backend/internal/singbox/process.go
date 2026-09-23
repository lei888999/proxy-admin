package singbox

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// settleChecks/settleDelay bound how long Start waits to see whether a freshly
// spawned sing-box stays up. `check` passing does not mean `run` will: a listen
// port already held by another process makes it exit within milliseconds.
const (
	settleChecks = 3
	settleDelay  = 100 * time.Millisecond
)

type ProcessManager struct {
	env        Env
	pidPath    string
	logPath    string
	configPath string

	mu sync.Mutex
	// ownPid is the PID this panel process started. A PID we started ourselves
	// needs no command-line verification; anything else does.
	ownPid int
}

func NewProcessManager(env Env, pidPath, logPath, configPath string) *ProcessManager {
	return &ProcessManager{env: env, pidPath: pidPath, logPath: logPath, configPath: configPath}
}

// Running reports whether the panel's own sing-box is up. It deliberately does
// NOT consider processes started outside the panel: adopting one meant the panel
// could neither start (it looked "already running") nor stop it without pkill
// taking down every sing-box on the host. Use ExternalRunning to report those.
func (p *ProcessManager) Running() bool { return p.ManagedRunning() }

// ManagedRunning reports whether the PID-file process is alive AND is really our
// sing-box. A pid file surviving a panel restart may point at a recycled PID, so
// any PID this process did not spawn is checked against its command line.
func (p *ProcessManager) ManagedRunning() bool {
	pid, ok := p.readPid()
	if !ok || !p.env.Alive(pid) {
		return false
	}
	p.mu.Lock()
	own := p.ownPid == pid
	p.mu.Unlock()
	if own {
		return true
	}
	cmdline, ok := p.env.CommandLine(pid)
	if !ok {
		return false
	}
	return strings.Contains(cmdline, "sing-box") && strings.Contains(cmdline, p.configPath)
}

// ExternalRunning reports a sing-box the panel does not manage. Reporting only —
// the panel never signals it, because under Docker host networking it may be an
// unrelated service and a pkill would take the host's proxying down.
func (p *ProcessManager) ExternalRunning() bool {
	if p.ManagedRunning() {
		return false
	}
	return p.env.Pgrep("sing-box")
}

func (p *ProcessManager) Start(bin, configPath string) error {
	if p.ManagedRunning() {
		return ErrAlreadyRunning
	}
	if out, err := p.env.Check(bin, configPath); err != nil {
		return &InvalidConfigError{Output: strings.TrimSpace(out)}
	}
	pid, err := p.env.Spawn(p.logPath, bin, configPath)
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.ownPid = pid
	p.mu.Unlock()
	if err := p.writePid(pid); err != nil {
		return err
	}
	if !p.settled(pid) {
		p.forget()
		return &StartFailedError{Output: p.logTail()}
	}
	return nil
}

// settled waits briefly to see whether a spawned process survives, so a start
// that dies on a port conflict is reported instead of looking successful.
func (p *ProcessManager) settled(pid int) bool {
	for i := 0; i < settleChecks; i++ {
		time.Sleep(settleDelay)
		if !p.env.Alive(pid) {
			return false
		}
	}
	return true
}

func (p *ProcessManager) Stop() error {
	// Gate on ManagedRunning so a stale pid file whose PID got recycled can never
	// make us signal an unrelated process.
	if !p.ManagedRunning() {
		p.forget()
		return ErrNotRunning
	}
	pid, _ := p.readPid()
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
	p.mu.Lock()
	p.ownPid = 0
	p.mu.Unlock()
	return os.Remove(p.pidPath)
}

// forget drops the pid file and ownership after a failed start or a process that
// is gone, so the next Start is not refused by a leftover record.
func (p *ProcessManager) forget() {
	_ = os.Remove(p.pidPath)
	p.mu.Lock()
	p.ownPid = 0
	p.mu.Unlock()
}

// logTail returns the end of sing-box's log, used to explain a start that died.
func (p *ProcessManager) logTail() string {
	b, err := os.ReadFile(p.logPath)
	if err != nil {
		return ""
	}
	const max = 2000
	if len(b) > max {
		b = b[len(b)-max:]
	}
	return strings.TrimSpace(string(b))
}

func (p *ProcessManager) readPid() (int, bool) {
	b, err := os.ReadFile(p.pidPath)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

func (p *ProcessManager) writePid(pid int) error {
	return os.WriteFile(p.pidPath, []byte(strconv.Itoa(pid)), 0600)
}

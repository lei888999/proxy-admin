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
	Pkill(name string) error
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

func (osEnv) Pkill(name string) error {
	return exec.Command("pkill", "-x", name).Run()
}

func (osEnv) Signal(pid int, sig syscall.Signal) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(sig)
}

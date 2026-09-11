package singbox

import (
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

type Status struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	Running   bool   `json:"running"`
	HasConfig bool   `json:"hasConfig"`
}

type Service struct {
	env         Env
	dir         string
	binOverride string
	store       *ConfigStore
	pm          *ProcessManager
	mu          sync.Mutex
}

// New builds a Service from an injected Env (used by tests).
func New(env Env, dir, binOverride string) *Service {
	return &Service{
		env:         env,
		dir:         dir,
		binOverride: binOverride,
		store:       NewConfigStore(filepath.Join(dir, "config.json")),
		pm:          NewProcessManager(env, filepath.Join(dir, "sing-box.pid"), filepath.Join(dir, "sing-box.log")),
	}
}

// NewDefault wires the production OS-backed Env.
func NewDefault(dir, binOverride string) *Service { return New(NewOSEnv(), dir, binOverride) }

var versionRe = regexp.MustCompile(`(\d+\.\d+\.\d+)`)

func (s *Service) resolveBin() string {
	if s.binOverride != "" && s.env.FileExists(s.binOverride) {
		return s.binOverride
	}
	managed := filepath.Join(s.dir, "bin", "sing-box")
	if s.env.FileExists(managed) {
		return managed
	}
	if p, ok := s.env.LookPath("sing-box"); ok {
		return p
	}
	return ""
}

func (s *Service) statusLocked() Status {
	bin := s.resolveBin()
	st := Status{HasConfig: s.store.Exists(), Running: s.pm.Running()}
	if bin != "" {
		st.Installed = true
		if out, ok := s.env.RunVersion(bin); ok {
			st.Version = versionRe.FindString(out)
		}
	}
	return st
}

func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statusLocked()
}

// ManagedRunning reports whether the panel-managed sing-box (started via Start)
// is running, ignoring processes started outside the panel. The traffic poller
// uses this so it only polls the Clash API when the panel's config (which
// enables clash_api) is the one actually running.
func (s *Service) ManagedRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pm.ManagedRunning()
}

func (s *Service) Start() (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bin := s.resolveBin()
	if bin == "" {
		return s.statusLocked(), ErrNotInstalled
	}
	if !s.store.Exists() {
		return s.statusLocked(), ErrNoConfig
	}
	if err := s.pm.Start(bin, s.store.Path()); err != nil {
		return s.statusLocked(), err
	}
	return s.statusLocked(), nil
}

func (s *Service) Stop() (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.pm.Stop()
	return s.statusLocked(), err
}

// Restart stops sing-box (ignoring "not running") and starts it again so a
// regenerated config takes effect.
func (s *Service) Restart() (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.pm.Stop(); err != nil && err != ErrNotRunning {
		return s.statusLocked(), err
	}
	bin := s.resolveBin()
	if bin == "" {
		return s.statusLocked(), ErrNotInstalled
	}
	if !s.store.Exists() {
		return s.statusLocked(), ErrNoConfig
	}
	if err := s.pm.Start(bin, s.store.Path()); err != nil {
		return s.statusLocked(), err
	}
	return s.statusLocked(), nil
}

func (s *Service) GetConfig() (string, error) { return s.store.Get() }
func (s *Service) SaveConfig(c string) error  { return s.store.Save(c) }

// ApplyConfig persists the config and, if sing-box is running, validates it and
// restarts to apply. Validation happens BEFORE stopping, so a bad config leaves
// the running process untouched (returns *InvalidConfigError). When sing-box is
// not running it is only persisted — the user starts it from the dashboard.
func (s *Service) ApplyConfig(content string) error {
	if err := s.store.Save(content); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.pm.Running() {
		return nil
	}
	bin := s.resolveBin()
	if bin == "" {
		return nil
	}
	if out, err := s.env.Check(bin, s.store.Path()); err != nil {
		return &InvalidConfigError{Output: strings.TrimSpace(out)}
	}
	if err := s.pm.Stop(); err != nil && err != ErrNotRunning {
		return err
	}
	return s.pm.Start(bin, s.store.Path())
}

package singbox

import (
	"path/filepath"
	"regexp"
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

func (s *Service) GetConfig() (string, error) { return s.store.Get() }
func (s *Service) SaveConfig(c string) error  { return s.store.Save(c) }

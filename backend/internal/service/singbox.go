package service

import (
	"os/exec"
	"regexp"
	"strings"
)

// Runner abstracts sing-box process/command interaction so it can be mocked.
type Runner interface {
	Version() (string, error)
	IsRunning() bool
}

type Status struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	Running   bool   `json:"running"`
}

type SingboxService struct {
	runner Runner
}

func NewSingboxService(r Runner) *SingboxService {
	return &SingboxService{runner: r}
}

var versionRe = regexp.MustCompile(`(\d+\.\d+\.\d+)`)

func (s *SingboxService) Status() Status {
	out, err := s.runner.Version()
	if err != nil {
		return Status{Installed: false}
	}
	version := ""
	if m := versionRe.FindString(out); m != "" {
		version = m
	}
	return Status{Installed: true, Version: version, Running: s.runner.IsRunning()}
}

// execRunner is the real implementation backed by the local sing-box binary.
type execRunner struct{}

func NewExecRunner() Runner { return execRunner{} }

func (execRunner) Version() (string, error) {
	out, err := exec.Command("sing-box", "version").CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (execRunner) IsRunning() bool {
	// pgrep returns exit code 0 only when a matching process exists.
	return exec.Command("pgrep", "-x", "sing-box").Run() == nil
}

package service

import (
	"errors"
	"testing"
)

type fakeRunner struct {
	versionOut string
	versionErr error
	running    bool
}

func (f fakeRunner) Version() (string, error) { return f.versionOut, f.versionErr }
func (f fakeRunner) IsRunning() bool          { return f.running }

func TestStatusInstalledRunning(t *testing.T) {
	svc := NewSingboxService(fakeRunner{versionOut: "sing-box version 1.9.0", running: true})
	st := svc.Status()
	if !st.Installed || st.Version != "1.9.0" || !st.Running {
		t.Fatalf("status = %+v", st)
	}
}

func TestStatusNotInstalled(t *testing.T) {
	svc := NewSingboxService(fakeRunner{versionErr: errors.New("not found")})
	st := svc.Status()
	if st.Installed || st.Version != "" || st.Running {
		t.Fatalf("status = %+v", st)
	}
}

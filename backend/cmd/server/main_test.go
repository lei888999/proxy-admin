package main

import (
	"errors"
	"testing"

	"singbox-admin/internal/singbox"
)

type fakeStartupSingbox struct {
	status     singbox.Status
	startErr   error
	startCalls int
}

func (f *fakeStartupSingbox) Status() singbox.Status { return f.status }
func (f *fakeStartupSingbox) Start() (singbox.Status, error) {
	f.startCalls++
	return f.status, f.startErr
}

func TestStartManagedSingboxStartsStoppedConfiguredProcess(t *testing.T) {
	f := &fakeStartupSingbox{status: singbox.Status{Installed: true, HasConfig: true, Running: false}}
	startManagedSingbox(f)
	if f.startCalls != 1 {
		t.Fatalf("Start calls = %d, want 1", f.startCalls)
	}
}

func TestStartManagedSingboxSkipsNoConfigOrAlreadyRunning(t *testing.T) {
	for _, status := range []singbox.Status{
		{Installed: false, HasConfig: true},
		{Installed: true, HasConfig: false},
		{Installed: true, HasConfig: true, Running: true},
	} {
		f := &fakeStartupSingbox{status: status}
		startManagedSingbox(f)
		if f.startCalls != 0 {
			t.Fatalf("status %+v: Start calls = %d, want 0", status, f.startCalls)
		}
	}
}

func TestStartManagedSingboxLogsFailureButReturns(t *testing.T) {
	f := &fakeStartupSingbox{
		status:   singbox.Status{Installed: true, HasConfig: true},
		startErr: errors.New("port in use"),
	}
	startManagedSingbox(f)
	if f.startCalls != 1 {
		t.Fatalf("Start calls = %d, want 1", f.startCalls)
	}
}

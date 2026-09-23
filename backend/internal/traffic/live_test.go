package traffic

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

type liveStreamFake struct {
	mu     sync.Mutex
	bodies []string
	errs   []error
	opens  int
}

func (f *liveStreamFake) OpenTraffic(context.Context) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	i := f.opens
	f.opens++
	if i < len(f.errs) && f.errs[i] != nil {
		return nil, f.errs[i]
	}
	if i >= len(f.bodies) {
		return io.NopCloser(strings.NewReader("")), nil
	}
	return io.NopCloser(strings.NewReader(f.bodies[i])), nil
}

func (f *liveStreamFake) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.opens
}

func TestLiveMonitorReadsLatestStreamSample(t *testing.T) {
	f := &liveStreamFake{bodies: []string{"{\"up\":10,\"down\":20}\n{\"up\":30,\"down\":40}\n"}}
	m := NewLiveMonitorWithStream(f, nil)
	m.retry = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.Run(ctx)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got := m.Snapshot(); got.Up == 30 && got.Down == 40 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("snapshot = %+v, want latest 30/40", m.Snapshot())
}

func TestLiveMonitorDoesNotOpenStreamWhilePanelStopped(t *testing.T) {
	f := &liveStreamFake{}
	m := NewLiveMonitorWithStream(f, func() bool { return false })
	m.retry = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	go m.Run(ctx)
	time.Sleep(20 * time.Millisecond)
	cancel()
	if got := f.count(); got != 0 {
		t.Fatalf("opened %d streams while stopped, want 0", got)
	}
}

func TestLiveMonitorPublishesZeroAfterOpenError(t *testing.T) {
	f := &liveStreamFake{errs: []error{errors.New("connection refused")}}
	m := NewLiveMonitorWithStream(f, nil)
	m.setLatest(Live{Up: 10, Down: 20})
	m.retry = time.Hour // one attempt is enough for this test
	ctx, cancel := context.WithCancel(context.Background())
	go m.Run(ctx)
	defer cancel()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got := m.Snapshot(); got == (Live{}) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("snapshot = %+v, want zero after stream error", m.Snapshot())
}

func TestReadStreamSkipsMalformedTicks(t *testing.T) {
	m := NewLiveMonitorWithStream(&liveStreamFake{}, nil)
	hadSample, err := m.readStream(context.Background(), strings.NewReader("invalid\n{\"up\":7,\"down\":8}\n"))
	if !hadSample {
		t.Fatal("expected a valid sample")
	}
	if !errors.Is(err, io.EOF) {
		t.Fatalf("err = %v, want EOF", err)
	}
	if got := m.Snapshot(); got != (Live{Up: 7, Down: 8}) {
		t.Fatalf("snapshot = %+v, want 7/8", got)
	}
}

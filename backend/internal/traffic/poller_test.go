package traffic

import (
	"context"
	"sync"
	"testing"
	"time"
)

type fakeConns struct {
	batches [][]Connection
	i       int
}

func (f *fakeConns) Connections(ctx context.Context) ([]Connection, error) {
	if f.i >= len(f.batches) {
		return nil, nil
	}
	b := f.batches[f.i]
	f.i++
	return b, nil
}

type memStore struct{ up, down map[uint]int64 }

func (m *memStore) AddTraffic(userID uint, up, down int64) error {
	m.up[userID] += up
	m.down[userID] += down
	return nil
}

func TestPollOnceAccumulatesDeltas(t *testing.T) {
	fc := &fakeConns{batches: [][]Connection{
		// tick 1: one connection for user 7 at 100/200 cumulative
		{{ID: "c1", User: "u7", Up: 100, Down: 200}},
		// tick 2: same connection grew to 150/200 -> delta 50/0
		{{ID: "c1", User: "u7", Up: 150, Down: 200}},
	}}
	st := &memStore{up: map[uint]int64{}, down: map[uint]int64{}}
	p := NewPoller(fc, st, 0, nil)

	if err := p.pollOnce(context.Background()); err != nil {
		t.Fatalf("pollOnce: %v", err)
	}
	if st.up[7] != 100 || st.down[7] != 200 {
		t.Fatalf("after tick1 up=%d down=%d, want 100/200", st.up[7], st.down[7])
	}
	if err := p.pollOnce(context.Background()); err != nil {
		t.Fatalf("pollOnce: %v", err)
	}
	if st.up[7] != 150 || st.down[7] != 200 {
		t.Fatalf("after tick2 up=%d down=%d, want 150/200 (only the delta is added)", st.up[7], st.down[7])
	}
}

func TestPollOnceNewConnectionAndPrune(t *testing.T) {
	fc := &fakeConns{batches: [][]Connection{
		{{ID: "c1", User: "u7", Up: 100, Down: 0}},
		// c1 closed, a fresh c2 appears at 30 cumulative -> full 30 counted (no prev)
		{{ID: "c2", User: "u7", Up: 30, Down: 0}},
	}}
	st := &memStore{up: map[uint]int64{}, down: map[uint]int64{}}
	p := NewPoller(fc, st, 0, nil)
	_ = p.pollOnce(context.Background())
	_ = p.pollOnce(context.Background())
	if st.up[7] != 130 {
		t.Fatalf("up=%d, want 130 (100 + new conn 30)", st.up[7])
	}
	if _, ok := p.lastSeen["c1"]; ok {
		t.Fatal("closed connection c1 should be pruned from lastSeen")
	}
}

type countingConns struct {
	mu    sync.Mutex
	calls int
}

func (c *countingConns) Connections(ctx context.Context) ([]Connection, error) {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()
	return nil, nil
}

func (c *countingConns) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func TestRunSkipsPollWhileSingboxDown(t *testing.T) {
	cc := &countingConns{}
	st := &memStore{up: map[uint]int64{}, down: map[uint]int64{}}
	p := NewPoller(cc, st, time.Millisecond, func() bool { return false })

	ctx, cancel := context.WithCancel(context.Background())
	go p.Run(ctx)
	time.Sleep(30 * time.Millisecond) // many ticks would have fired
	cancel()

	if n := cc.count(); n != 0 {
		t.Fatalf("client called %d times while sing-box is down, want 0", n)
	}
}

func TestParseUserID(t *testing.T) {
	if id, ok := parseUserID("u42"); !ok || id != 42 {
		t.Fatalf("parseUserID(u42)=%d %v", id, ok)
	}
	if _, ok := parseUserID(""); ok {
		t.Fatal("empty user should not parse")
	}
	if _, ok := parseUserID("admin"); ok {
		t.Fatal("non-u-prefixed user should not parse")
	}
	if _, ok := parseUserID("uX"); ok {
		t.Fatal("non-numeric suffix should not parse")
	}
}

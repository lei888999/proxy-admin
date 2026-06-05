package traffic

import (
	"context"
	"testing"
)

type fakeStats struct {
	batches [][]Stat
	i       int
}

func (f *fakeStats) QueryStats(ctx context.Context) ([]Stat, error) {
	if f.i >= len(f.batches) {
		return nil, nil
	}
	b := f.batches[f.i]
	f.i++
	return b, nil
}
func (f *fakeStats) Close() error { return nil }

type memStore struct{ up, down map[uint]int64 }

func (m *memStore) AddTraffic(userID uint, up, down int64) error {
	m.up[userID] += up
	m.down[userID] += down
	return nil
}

func TestPollOnceAccumulates(t *testing.T) {
	fs := &fakeStats{batches: [][]Stat{
		{{Name: "user>>>u7>>>traffic>>>uplink", Value: 100}, {Name: "user>>>u7>>>traffic>>>downlink", Value: 200}},
		{{Name: "user>>>u7>>>traffic>>>uplink", Value: 50}},
	}}
	st := &memStore{up: map[uint]int64{}, down: map[uint]int64{}}
	p := NewPoller(fs, st, 0)

	if err := p.pollOnce(context.Background()); err != nil {
		t.Fatalf("pollOnce: %v", err)
	}
	if st.up[7] != 100 || st.down[7] != 200 {
		t.Fatalf("after tick1 up=%d down=%d", st.up[7], st.down[7])
	}
	if err := p.pollOnce(context.Background()); err != nil {
		t.Fatalf("pollOnce: %v", err)
	}
	if st.up[7] != 150 || st.down[7] != 200 {
		t.Fatalf("after tick2 up=%d down=%d (deltas should accumulate)", st.up[7], st.down[7])
	}
}

func TestParseStatName(t *testing.T) {
	id, dir, ok := parseUserStat("user>>>u42>>>traffic>>>downlink")
	if !ok || id != 42 || dir != "downlink" {
		t.Fatalf("parse=%d %s %v", id, dir, ok)
	}
	if _, _, ok := parseUserStat("inbound>>>x>>>traffic>>>uplink"); ok {
		t.Fatal("non-user stat should not parse")
	}
}

package traffic

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"
)

// TrafficStore persists accumulated per-user traffic.
type TrafficStore interface {
	AddTraffic(userID uint, up, down int64) error
}

type counters struct{ up, down int64 }

// flushInterval bounds how long observed traffic remains in memory before it is
// made visible in the user list. Sampling is intentionally faster (1s in main)
// to reduce the blind window for short connections; flushing stays coarser so a
// busy server does not generate one SQLite update per user per second.
const flushInterval = 5 * time.Second

type Poller struct {
	client   ConnectionsClient
	store    TrafficStore
	interval time.Duration
	// running reports whether sing-box is up. When it isn't, the Clash API is
	// down and we skip the tick instead of logging connection-refused noise.
	// nil means "always poll" (used in tests).
	running func() bool
	// lastSeen tracks each live connection's cumulative bytes so we add only the
	// per-tick delta. Entries for closed connections are pruned each tick.
	lastSeen map[string]counters
	// pending batches deltas between SQLite flushes. Run owns this map, so it
	// needs no mutex; explicit pollOnce/flush calls in unit tests are sequential.
	pending map[uint]counters
}

func NewPoller(client ConnectionsClient, store TrafficStore, interval time.Duration, running func() bool) *Poller {
	return &Poller{
		client:   client,
		store:    store,
		interval: interval,
		running:  running,
		lastSeen: map[string]counters{},
		pending:  map[uint]counters{},
	}
}

// parseUserID turns a connection's metadata.user ("u<id>") into a user ID.
func parseUserID(user string) (uint, bool) {
	if !strings.HasPrefix(user, "u") {
		return 0, false
	}
	id, err := strconv.ParseUint(user[1:], 10, 64)
	if err != nil {
		return 0, false
	}
	return uint(id), true
}

// pollOnce samples the active connection table. This is necessarily an observed
// metric: Clash has no completed-connection event or per-user total counter, so
// a connection that opens and closes entirely between two samples is invisible.
// The one-second production interval materially reduces (but cannot eliminate)
// that blind window. The UI calls this "在线连接采样累计", not billable traffic.
func (p *Poller) pollOnce(ctx context.Context) error {
	conns, err := p.client.Connections(ctx)
	if err != nil {
		return err
	}
	acc := map[uint]*counters{}
	live := make(map[string]struct{}, len(conns))
	for _, cn := range conns {
		live[cn.ID] = struct{}{}
		uid, ok := parseUserID(cn.User)
		if !ok {
			continue
		}
		prev := p.lastSeen[cn.ID]
		du, dd := cn.Up-prev.up, cn.Down-prev.down
		// A negative delta means the connection ID was reused or counters reset;
		// treat the current value as the delta instead of subtracting.
		if du < 0 {
			du = cn.Up
		}
		if dd < 0 {
			dd = cn.Down
		}
		p.lastSeen[cn.ID] = counters{cn.Up, cn.Down}
		d := acc[uid]
		if d == nil {
			d = &counters{}
			acc[uid] = d
		}
		d.up += du
		d.down += dd
	}
	// Prune connections that have closed since the last tick.
	for id := range p.lastSeen {
		if _, ok := live[id]; !ok {
			delete(p.lastSeen, id)
		}
	}
	for uid, d := range acc {
		if d.up == 0 && d.down == 0 {
			continue
		}
		queued := p.pending[uid]
		queued.up += d.up
		queued.down += d.down
		p.pending[uid] = queued
	}
	return nil
}

// flush persists accumulated deltas. Remove each entry only after its database
// write succeeds, so a transient SQLite failure never turns into either lost
// traffic or a duplicate write for a user already persisted earlier in the loop.
func (p *Poller) flush() error {
	for uid, d := range p.pending {
		if err := p.store.AddTraffic(uid, d.up, d.down); err != nil {
			return err
		}
		delete(p.pending, uid)
	}
	return nil
}

// Run polls until ctx is cancelled. Errors are logged and skipped so a stopped
// sing-box doesn't kill the loop. Pending deltas are flushed on cancellation too
// (using a bounded background context isn't needed: TrafficStore has no context).
func (p *Poller) Run(ctx context.Context) {
	if p.interval <= 0 {
		p.interval = time.Second
	}
	pollTick := time.NewTicker(p.interval)
	flushTick := time.NewTicker(flushInterval)
	defer pollTick.Stop()
	defer flushTick.Stop()
	defer func() {
		if err := p.flush(); err != nil {
			log.Printf("traffic flush on shutdown: %v", err)
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-pollTick.C:
			// Skip while sing-box is down — its Clash API isn't listening, so a
			// poll would only produce connection-refused log spam.
			if p.running != nil && !p.running() {
				continue
			}
			if err := p.pollOnce(ctx); err != nil {
				log.Printf("traffic poll: %v", err)
			}
		case <-flushTick.C:
			if err := p.flush(); err != nil {
				log.Printf("traffic flush: %v", err)
			}
		}
	}
}

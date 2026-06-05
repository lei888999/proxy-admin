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

type Poller struct {
	client   ConnectionsClient
	store    TrafficStore
	interval time.Duration
	// lastSeen tracks each live connection's cumulative bytes so we add only the
	// per-tick delta. Entries for closed connections are pruned each tick.
	lastSeen map[string]counters
}

func NewPoller(client ConnectionsClient, store TrafficStore, interval time.Duration) *Poller {
	return &Poller{client: client, store: store, interval: interval, lastSeen: map[string]counters{}}
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
		if err := p.store.AddTraffic(uid, d.up, d.down); err != nil {
			return err
		}
	}
	return nil
}

// Run polls until ctx is cancelled. Errors are logged and skipped so a stopped
// sing-box doesn't kill the loop.
func (p *Poller) Run(ctx context.Context) {
	t := time.NewTicker(p.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := p.pollOnce(ctx); err != nil {
				log.Printf("traffic poll: %v", err)
			}
		}
	}
}

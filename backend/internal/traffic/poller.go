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

type Poller struct {
	client   StatsClient
	store    TrafficStore
	interval time.Duration
}

func NewPoller(client StatsClient, store TrafficStore, interval time.Duration) *Poller {
	return &Poller{client: client, store: store, interval: interval}
}

// parseUserStat parses "user>>>u<id>>>>traffic>>><uplink|downlink>".
func parseUserStat(name string) (uint, string, bool) {
	parts := strings.Split(name, ">>>")
	if len(parts) != 4 || parts[0] != "user" || parts[2] != "traffic" {
		return 0, "", false
	}
	if !strings.HasPrefix(parts[1], "u") {
		return 0, "", false
	}
	id, err := strconv.ParseUint(parts[1][1:], 10, 64)
	if err != nil {
		return 0, "", false
	}
	return uint(id), parts[3], true
}

func (p *Poller) pollOnce(ctx context.Context) error {
	stats, err := p.client.QueryStats(ctx)
	if err != nil {
		return err
	}
	type delta struct{ up, down int64 }
	acc := map[uint]*delta{}
	for _, s := range stats {
		id, dir, ok := parseUserStat(s.Name)
		if !ok {
			continue
		}
		d := acc[id]
		if d == nil {
			d = &delta{}
			acc[id] = d
		}
		if dir == "uplink" {
			d.up += s.Value
		} else if dir == "downlink" {
			d.down += s.Value
		}
	}
	for id, d := range acc {
		if err := p.store.AddTraffic(id, d.up, d.down); err != nil {
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

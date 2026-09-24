package diagnostics

import (
	"context"
	"net"
	"strconv"
	"time"

	"singbox-admin/internal/inbound"
)

// Probe is a reachability sample from the VPS to one configured upstream.
// It intentionally measures only TCP connect time; it does not claim that an
// authenticated proxy request or a client-to-VPS UDP path is healthy.
type Probe struct {
	Tag       string `json:"tag"`
	Server    string `json:"server"`
	Port      uint16 `json:"port"`
	Reachable bool   `json:"reachable"`
	LatencyMs int64  `json:"latencyMs,omitempty"`
	Error     string `json:"error,omitempty"`
}

func ProbeOutbounds(ctx context.Context, outbounds []inbound.OutboundView) []Probe {
	out := make([]Probe, 0, len(outbounds))
	if len(outbounds) == 0 {
		return out
	}
	type indexedProbe struct {
		index int
		probe Probe
	}
	results := make(chan indexedProbe, len(outbounds))
	for i, o := range outbounds {
		index := i
		o := o
		go func() {
			p := Probe{Tag: o.Tag, Server: o.Server, Port: o.Port}
			start := time.Now()
			dialCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", net.JoinHostPort(o.Server, strconv.Itoa(int(o.Port))))
			cancel()
			if err != nil {
				p.Error = err.Error()
			} else {
				p.Reachable = true
				p.LatencyMs = time.Since(start).Milliseconds()
				_ = conn.Close()
			}
			results <- indexedProbe{index: index, probe: p}
		}()
	}
	// Preserve the configured order so the dashboard does not reorder rows as
	// individual probes finish.
	ordered := make([]Probe, len(outbounds))
	for range outbounds {
		p := <-results
		ordered[p.index] = p.probe
	}
	out = append(out, ordered...)
	return out
}

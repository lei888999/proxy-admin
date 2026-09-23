package traffic

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Live is a one-tick global throughput snapshot in bytes/second.
type Live struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

// LiveStream opens the Clash API's newline-delimited /traffic stream. It is
// deliberately small so the monitor can be tested without a real HTTP server.
type LiveStream interface {
	OpenTraffic(context.Context) (io.ReadCloser, error)
}

type clashTrafficStream struct {
	addr   string
	secret string
	client *http.Client
}

func (c *clashTrafficStream) OpenTraffic(ctx context.Context) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+c.addr+"/traffic", nil)
	if err != nil {
		return nil, err
	}
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("traffic stream returned %s", resp.Status)
	}
	return resp.Body, nil
}

// LiveMonitor keeps exactly one Clash /traffic stream open and exposes its most
// recent value. Before this existed, every open browser tab made a fresh stream
// request every 3 seconds, held it open until the first sample arrived, then
// discarded it — N dashboards meant N concurrent one-second stream requests.
type LiveMonitor struct {
	stream  LiveStream
	running func() bool
	retry   time.Duration

	mu      sync.RWMutex
	latest  Live
	updated time.Time
	lastErr string
}

func NewLiveMonitor(addr, secret string, running func() bool) *LiveMonitor {
	return NewLiveMonitorWithStream(
		&clashTrafficStream{addr: addr, secret: secret, client: &http.Client{}},
		running,
	)
}

func NewLiveMonitorWithStream(stream LiveStream, running func() bool) *LiveMonitor {
	return &LiveMonitor{stream: stream, running: running, retry: time.Second}
}

// Snapshot is instant and allocation-free aside from the returned value; API
// handlers use it rather than opening another stream.
func (m *LiveMonitor) Snapshot() Live {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.latest
}

func (m *LiveMonitor) setLatest(live Live) {
	m.mu.Lock()
	m.latest = live
	m.updated = time.Now()
	m.lastErr = ""
	m.mu.Unlock()
}

func (m *LiveMonitor) setDown(err error) {
	m.mu.Lock()
	m.latest = Live{}
	if err != nil {
		m.lastErr = err.Error()
	} else {
		m.lastErr = ""
	}
	m.mu.Unlock()
}

// Run reconnects the one stream after an error. It returns on ctx cancellation.
// When panel-managed sing-box is down, it publishes zero and avoids repeated
// connection-refused logs/requests until it comes back.
func (m *LiveMonitor) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		if m.running != nil && !m.running() {
			m.setDown(nil)
			if !wait(ctx, m.retry) {
				return
			}
			continue
		}
		body, err := m.stream.OpenTraffic(ctx)
		if err != nil {
			m.setDown(err)
			if !wait(ctx, m.retry) {
				return
			}
			continue
		}
		hadSample, err := m.readStream(ctx, body)
		_ = body.Close()
		if ctx.Err() != nil {
			return
		}
		// EOF after valid samples is a normal reconnect: retain the last known
		// rate until the next stream provides a newer one. A stream that failed
		// before any sample (or a scanner/open error) is genuinely unavailable.
		if !hadSample || (err != nil && !errors.Is(err, io.EOF)) {
			m.setDown(err)
		}
		if !wait(ctx, m.retry) {
			return
		}
	}
}

func (m *LiveMonitor) readStream(ctx context.Context, body io.Reader) (bool, error) {
	sc := bufio.NewScanner(body)
	// Scanner's default 64 KiB token limit is overly tight for a proxy stream
	// controlled outside this process. A traffic tick is tiny, but keep a bounded
	// 1 MiB cap and fail/reconnect rather than panic on malformed input.
	sc.Buffer(make([]byte, 1024), 1<<20)
	hadSample := false
	for sc.Scan() {
		var l Live
		if err := json.Unmarshal(sc.Bytes(), &l); err == nil {
			hadSample = true
			m.setLatest(l)
		}
		if ctx.Err() != nil {
			return hadSample, ctx.Err()
		}
	}
	if err := sc.Err(); err != nil {
		return hadSample, err
	}
	return hadSample, io.EOF
}

func wait(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

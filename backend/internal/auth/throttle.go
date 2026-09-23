package auth

import (
	"sync"
	"time"
)

// Throttle rate-limits repeated login failures per key (client IP).
//
// It is deliberately checked BEFORE the bcrypt comparison: bcrypt is
// intentionally expensive (tens of milliseconds of CPU each), so an unthrottled
// login endpoint is not only a brute-force target but a cheap way to saturate
// the VPS's CPU and starve the proxying that shares it.
type Throttle struct {
	max      int           // failures tolerated inside window
	window   time.Duration // failures older than this are forgotten
	block    time.Duration // lockout once max is reached
	maxKeys  int           // cap so the map itself cannot be used to exhaust memory
	mu       sync.Mutex
	entries  map[string]*throttleEntry
	nowFn    func() time.Time // injectable clock for tests
	lastGC   time.Time
	gcPeriod time.Duration
}

type throttleEntry struct {
	failures    int
	lastFailure time.Time
	blockedThru time.Time
}

// NewThrottle builds a throttle allowing max failures per window, then blocking
// for block. Defaults used by the panel: 5 failures / 15 min, 15 min lockout.
func NewThrottle(max int, window, block time.Duration) *Throttle {
	return &Throttle{
		max:      max,
		window:   window,
		block:    block,
		maxKeys:  10000,
		entries:  map[string]*throttleEntry{},
		nowFn:    time.Now,
		gcPeriod: time.Minute,
	}
}

// Allow reports whether key may attempt a login now. When it may not, the second
// return value is how long until it may.
func (t *Throttle) Allow(key string) (time.Duration, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.nowFn()
	t.gcLocked(now)
	e := t.entries[key]
	if e == nil {
		return 0, true
	}
	if now.Before(e.blockedThru) {
		return e.blockedThru.Sub(now), false
	}
	return 0, true
}

// Fail records a failed attempt, starting a lockout once max is reached.
func (t *Throttle) Fail(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.nowFn()
	t.gcLocked(now)
	e := t.entries[key]
	if e == nil {
		if len(t.entries) >= t.maxKeys {
			// Full and nothing collectable: drop this record rather than grow
			// without bound. The attempt still fails, it just isn't counted.
			return
		}
		e = &throttleEntry{}
		t.entries[key] = e
	}
	// A long-quiet key starts over, so an honest user is not punished for a
	// mistyped password from hours ago.
	if !e.lastFailure.IsZero() && now.Sub(e.lastFailure) > t.window {
		e.failures = 0
	}
	e.failures++
	e.lastFailure = now
	if e.failures >= t.max {
		e.blockedThru = now.Add(t.block)
		e.failures = 0
	}
}

// Reset clears a key's history after a successful login.
func (t *Throttle) Reset(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, key)
}

// gcLocked drops entries that are neither blocked nor recently active.
func (t *Throttle) gcLocked(now time.Time) {
	if now.Sub(t.lastGC) < t.gcPeriod {
		return
	}
	t.lastGC = now
	for k, e := range t.entries {
		if now.Before(e.blockedThru) {
			continue
		}
		if now.Sub(e.lastFailure) > t.window {
			delete(t.entries, k)
		}
	}
}

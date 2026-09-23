package auth

import (
	"testing"
	"time"
)

func newTestThrottle() (*Throttle, *time.Time) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t := NewThrottle(3, 10*time.Minute, 5*time.Minute)
	t.nowFn = func() time.Time { return now }
	return t, &now
}

func TestThrottleAllowsUntilMaxFailures(t *testing.T) {
	th, _ := newTestThrottle()
	for i := 0; i < 2; i++ {
		th.Fail("1.2.3.4")
		if _, ok := th.Allow("1.2.3.4"); !ok {
			t.Fatalf("blocked after %d failures, want allowed", i+1)
		}
	}
	th.Fail("1.2.3.4")
	wait, ok := th.Allow("1.2.3.4")
	if ok {
		t.Fatal("should be blocked after reaching max failures")
	}
	if wait <= 0 || wait > 5*time.Minute {
		t.Fatalf("wait = %v, want within the block window", wait)
	}
}

func TestThrottleBlockExpires(t *testing.T) {
	th, now := newTestThrottle()
	for i := 0; i < 3; i++ {
		th.Fail("ip")
	}
	if _, ok := th.Allow("ip"); ok {
		t.Fatal("precondition: should be blocked")
	}
	*now = now.Add(6 * time.Minute)
	if _, ok := th.Allow("ip"); !ok {
		t.Fatal("block should have expired")
	}
}

func TestThrottleForgetsStaleFailures(t *testing.T) {
	th, now := newTestThrottle()
	th.Fail("ip")
	th.Fail("ip")
	*now = now.Add(11 * time.Minute) // older than the window
	th.Fail("ip")                    // counts as the first failure again
	if _, ok := th.Allow("ip"); !ok {
		t.Fatal("stale failures must not accumulate into a lockout")
	}
}

func TestThrottleResetOnSuccess(t *testing.T) {
	th, _ := newTestThrottle()
	th.Fail("ip")
	th.Fail("ip")
	th.Reset("ip")
	th.Fail("ip")
	if _, ok := th.Allow("ip"); !ok {
		t.Fatal("a successful login must clear the failure count")
	}
}

func TestThrottleIsPerKey(t *testing.T) {
	th, _ := newTestThrottle()
	for i := 0; i < 3; i++ {
		th.Fail("attacker")
	}
	if _, ok := th.Allow("innocent"); !ok {
		t.Fatal("one key's lockout must not affect another")
	}
}

func TestThrottleCapsTrackedKeys(t *testing.T) {
	th, _ := newTestThrottle()
	th.maxKeys = 2
	th.Fail("a")
	th.Fail("b")
	th.Fail("c") // dropped rather than growing the map
	if len(th.entries) > 2 {
		t.Fatalf("tracked %d keys, want at most 2", len(th.entries))
	}
}

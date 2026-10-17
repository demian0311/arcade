package session

import (
	"testing"
	"time"
)

func TestBlockedKeyWaitsForAFreshPress(t *testing.T) {
	var h Hold
	t0 := time.Now()
	if !h.Press(t0) || !h.Active(t0) {
		t.Fatal("first press should count")
	}
	h.Block()
	at := t0.Add(250 * time.Millisecond) // auto-repeat begins and keeps arriving every 25 ms
	for i := 0; i < 20; i++ {
		if h.Press(at) || h.Active(at) {
			t.Fatalf("held key must not act after a block (repeat %d)", i)
		}
		at = at.Add(25 * time.Millisecond)
	}
	if !h.Press(at.Add(400*time.Millisecond)) || !h.Active(at.Add(401*time.Millisecond)) {
		t.Fatal("re-press after a gap should count")
	}
}

func TestHoldBridgesRepeatDelayAndReleaseEnds(t *testing.T) {
	var h Hold
	t0 := time.Now()
	h.Press(t0)
	if h.Held() {
		t.Fatal("a fresh press is not a confirmed hold")
	}
	if !h.Active(t0.Add(200 * time.Millisecond)) {
		t.Fatal("a fresh press must stay held across the repeat delay")
	}
	if h.Active(t0.Add(DefaultRepeatDelay + RepeatSlack + time.Millisecond)) {
		t.Fatal("a lone tap must end after the bridge window")
	}
	at := t0
	for i := 0; i < 12; i++ { // the first repeat lands at 250 ms, then every 25 ms
		if i == 0 {
			at = at.Add(250 * time.Millisecond)
		} else {
			at = at.Add(25 * time.Millisecond)
		}
		h.Press(at)
		if !h.Active(at.Add(20*time.Millisecond)) || !h.Held() {
			t.Fatalf("repeat %d should keep the hold alive and confirmed", i)
		}
	}
	if h.RepeatDelay() != 250*time.Millisecond {
		t.Fatalf("learned repeat delay %v", h.RepeatDelay())
	}
	if h.Active(at.Add(HoldGap + time.Millisecond)) {
		t.Fatal("the hold should end once repeats stop")
	}
	h.Press(at.Add(500 * time.Millisecond))
	if !h.Active(at.Add(500*time.Millisecond + 250*time.Millisecond + RepeatSlack - time.Millisecond)) {
		t.Fatal("the next fresh press should use the learned delay")
	}
}

func TestThrottle(t *testing.T) {
	th := Throttle{Gap: 60 * time.Millisecond}
	t0 := time.Now()
	if !th.Allow(t0) || th.Allow(t0.Add(25*time.Millisecond)) || !th.Allow(t0.Add(60*time.Millisecond)) {
		t.Fatal("throttle let the wrong presses through")
	}
}

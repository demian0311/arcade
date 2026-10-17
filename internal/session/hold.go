// Package session holds what every game's Bubble Tea model needs: ticks
// that discard stale timers, the pause on focus loss, and telling a tapped
// key from a held one.
package session

import "time"

// Terminals send no key-up, so a held key is one press, a pause (the repeat
// delay, 250 ms on Hyprland's default), then auto-repeat presses 25 ms apart.
const (
	// HoldGap is how long a repeat press keeps the key counted as held; a
	// longer gap means it was released.
	HoldGap = 100 * time.Millisecond
	// A fresh press keeps the key held for the learned repeat delay plus
	// RepeatSlack, so a hold does not stutter before repeats start.
	RepeatSlack        = 60 * time.Millisecond
	DefaultRepeatDelay = 300 * time.Millisecond
	MaxRepeatDelay     = 700 * time.Millisecond
)

// Hold tracks one key and reports whether it is held right now. The zero
// value is ready to use.
type Hold struct {
	lastDown    time.Time
	lastFresh   bool          // the previous press began a hold
	held        bool          // auto-repeat has confirmed the key is held
	repeatDelay time.Duration // measured gap from a fresh press to its first repeat
	until       time.Time     // the key counts as held until this moment
	blocked     bool          // ignore the key until it is released and pressed again
}

// Press registers a press at the given time and reports whether it counts.
// A fresh press holds the key long enough to reach the first auto-repeat;
// each repeat extends it by HoldGap. A blocked key stays blocked until a
// fresh press.
func (h *Hold) Press(at time.Time) bool {
	if h.repeatDelay == 0 {
		h.repeatDelay = DefaultRepeatDelay
	}
	gap := at.Sub(h.lastDown)
	fresh := h.lastDown.IsZero() || gap >= h.repeatDelay+RepeatSlack ||
		(!h.lastFresh && gap >= HoldGap)
	if h.lastFresh && !fresh {
		h.repeatDelay = min(MaxRepeatDelay, max(HoldGap, gap))
	}
	h.lastDown, h.lastFresh = at, fresh
	if h.blocked && !fresh {
		return false
	}
	h.blocked = false
	h.held = !fresh
	window := HoldGap
	if fresh {
		window = h.repeatDelay + RepeatSlack
	}
	h.until = at.Add(window)
	return true
}

// Active reports whether the key counts as held at the given time.
func (h *Hold) Active(at time.Time) bool {
	return !h.blocked && at.Before(h.until)
}

// Held reports whether auto-repeat has confirmed a real hold, as opposed to
// the window a lone tap opens.
func (h *Hold) Held() bool { return h.held }

// Block ignores the key until it is released and pressed again, so a key
// still held from the previous piece does not act on the next one.
func (h *Hold) Block() { h.blocked, h.held = true, false }

// RepeatDelay is the learned gap from a fresh press to its first repeat.
func (h *Hold) RepeatDelay() time.Duration { return h.repeatDelay }

// Throttle lets an action through at most once per Gap, so auto-repeat
// arriving every 25 ms runs at a pace a player can stop.
type Throttle struct {
	Gap  time.Duration
	last time.Time
}

// Allow reports whether an action at the given time may run.
func (t *Throttle) Allow(at time.Time) bool {
	if at.Sub(t.last) < t.Gap {
		return false
	}
	t.last = at
	return true
}

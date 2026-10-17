package session

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Ticker hands out timers whose ids let a model discard ticks it no longer wants.
type Ticker struct{ id int }

// TickMsg is a Ticker's timer firing.
type TickMsg struct {
	ID int
	At time.Time
}

// Tick schedules a tick after d and makes every earlier tick stale.
func (t *Ticker) Tick(d time.Duration) tea.Cmd {
	t.id++
	id := t.id
	return tea.Tick(d, func(at time.Time) tea.Msg { return TickMsg{id, at} })
}

// Current reports whether msg is the tick most recently scheduled.
func (t *Ticker) Current(msg TickMsg) bool { return msg.ID == t.id }

// ID is the id the current tick carries; for tests.
func (t *Ticker) ID() int { return t.id }

const (
	CountdownFrom = 3
	CountdownStep = 700 * time.Millisecond
)

// Focus pauses a game when the terminal loses focus and, once focus
// returns, counts down and resumes it. A pause the player made stays paused.
type Focus struct {
	Blurred    bool // the terminal is unfocused; the view should fade
	Countdown  int  // steps left before an auto-paused game resumes, 0 when not counting
	autoPaused bool
	countID    int
}

// CountMsg steps the countdown.
type CountMsg struct{ ID int }

// Blur records a focus loss. running says whether play was in progress; the
// result says whether the caller should pause it.
func (f *Focus) Blur(running bool) bool {
	f.Blurred = true
	f.Countdown = 0
	f.autoPaused = running
	return running
}

// Regain records a focus return and starts the countdown if the blur paused play.
func (f *Focus) Regain() tea.Cmd {
	f.Blurred = false
	if f.autoPaused {
		f.Countdown = CountdownFrom
		return f.count()
	}
	return nil
}

func (f *Focus) count() tea.Cmd {
	f.countID++
	id := f.countID
	return tea.Tick(CountdownStep, func(time.Time) tea.Msg { return CountMsg{id} })
}

// Count handles a countdown step. resume says the caller should unpause now.
func (f *Focus) Count(msg CountMsg) (resume bool, cmd tea.Cmd) {
	if msg.ID != f.countID || f.Countdown == 0 {
		return false, nil
	}
	f.Countdown--
	if f.Countdown > 0 {
		return false, f.count()
	}
	return true, nil
}

// Resumed records that play is running again, however that came about.
func (f *Focus) Resumed() { f.autoPaused, f.Countdown = false, 0 }

// CountID is the id the next CountMsg must carry; for tests.
func (f *Focus) CountID() int { return f.countID }

package main

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/demian0311/arcade/internal/backdrop"
	"github.com/demian0311/arcade/internal/layout"
	"github.com/demian0311/arcade/internal/session"
)

type phase int

const (
	phaseMenu phase = iota
	phaseFalling
	phasePaused
	phaseClearing
	phaseDropping
	phaseLevelClear
	phaseGameOver
)

const (
	frameDelay = 33 * time.Millisecond  // falling-pill animation rate
	clearDelay = 200 * time.Millisecond // matched cells flash before they vanish
	dropDelay  = 50 * time.Millisecond  // one row of cascade
	// moveGap throttles horizontal auto-repeat, which arrives every 25 ms, to a pace a player can stop.
	moveGap  = 60 * time.Millisecond
	maxScale = 4
)

// bgMsg drives the backdrop animation on its own clock.
type bgMsg struct{ at time.Time }

type model struct {
	g             *Game
	phase         phase
	resume        phase // the phase a pause returns to
	focus         session.Focus
	down          session.Hold
	slideGate     session.Throttle
	ticker        session.Ticker
	level         int
	speed         Speed
	width, height int
	scale         int
	titleScale    int
	lastFrame     time.Time
	bg            *backdrop.Backdrop
	lastBg        time.Time
}

func newModel() model {
	m := model{speed: Med, bg: &backdrop.Backdrop{}, slideGate: session.Throttle{Gap: moveGap}}
	m.g = NewGame(m.level, m.speed, time.Now().UnixNano())
	return m
}

func (m model) Init() tea.Cmd { return bgTick() }

func bgTick() tea.Cmd {
	return tea.Tick(backdrop.Frame, func(t time.Time) tea.Msg { return bgMsg{t} })
}

func (m model) running() bool {
	return m.phase == phaseFalling || m.phase == phaseClearing || m.phase == phaseDropping
}

// pause stops play in any running phase and remembers where to resume.
func (m *model) pause() {
	if m.running() {
		m.resume, m.phase = m.phase, phasePaused
	}
}

// unpause returns to the phase pause left and restarts its clock.
func (m *model) unpause() tea.Cmd {
	m.phase = m.resume
	m.focus.Resumed()
	switch m.phase {
	case phaseClearing:
		return m.ticker.Tick(clearDelay)
	case phaseDropping:
		return m.ticker.Tick(dropDelay)
	}
	return m.frame()
}

// frame schedules the next pill animation frame and marks when the previous one was.
func (m *model) frame() tea.Cmd {
	m.lastFrame = time.Now()
	return m.ticker.Tick(frameDelay)
}

// fitScale picks the largest board scale whose framed bottle fits the
// terminal height, then the largest title scale whose side column fits the
// width, down to the plain-text title at 0. A title that leaves room to
// centre the bottle wins over a larger one that would push it aside. Both
// are 0 when nothing fits.
func fitScale(w, h int) (s, ts int) {
	for s = maxScale; s >= 1; s-- {
		if boardRows(s) > h {
			continue
		}
		for _, mirrored := range []bool{true, false} {
			for ts = layout.MaxTitleScale; ts >= 0; ts-- {
				need := layoutWidth(s, ts)
				if mirrored {
					need += colWidth(s, ts) + colPad
				}
				if need <= w && sideRows(s, ts) <= boardRows(s) {
					return s, ts
				}
			}
		}
	}
	return 0, 0
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.scale, m.titleScale = fitScale(msg.Width, msg.Height)
		m.bg.Resize(msg.Width, msg.Height)
		return m, nil
	case tea.BlurMsg:
		if m.focus.Blur(m.running()) {
			m.pause()
		}
		return m, nil
	case tea.FocusMsg:
		return m, m.focus.Regain()
	case session.CountMsg:
		resume, cmd := m.focus.Count(msg)
		if resume {
			return m, m.unpause()
		}
		return m, cmd
	case tea.KeyMsg:
		return m.key(msg.String())
	case bgMsg:
		if !m.lastBg.IsZero() {
			m.bg.Step(msg.at.Sub(m.lastBg))
		}
		m.lastBg = msg.at
		return m, bgTick()
	case session.TickMsg:
		if !m.ticker.Current(msg) {
			return m, nil
		}
		switch m.phase {
		case phaseFalling:
			dt := msg.At.Sub(m.lastFrame)
			m.lastFrame = msg.At
			m.g.Soft = m.down.Active(msg.At)
			m.g.Held = m.g.Soft && m.down.Held()
			if m.g.Advance(dt) {
				return m.lock()
			}
			return m, m.ticker.Tick(frameDelay)
		case phaseClearing:
			m.bg.Flare(hex[m.g.clearingColor()])
			m.g.Clear()
			m.phase = phaseDropping
			return m, m.ticker.Tick(dropDelay)
		case phaseDropping:
			if m.g.DropStep() {
				return m, m.ticker.Tick(dropDelay)
			}
			return m.resolve()
		}
	}
	return m, nil
}

func (m model) key(k string) (tea.Model, tea.Cmd) {
	if k == "ctrl+c" || k == "q" {
		return m, tea.Quit
	}
	switch m.phase {
	case phaseMenu:
		switch k {
		case "left", "h":
			m.level = max(0, m.level-1)
		case "right", "l":
			m.level = min(MaxLevel, m.level+1)
		case "up", "k":
			m.speed = min(Hi, m.speed+1)
		case "down", "j":
			m.speed = max(Low, m.speed-1)
		case "enter":
			return m.start(0)
		default:
			return m, nil
		}
		m.g = NewGame(m.level, m.speed, time.Now().UnixNano())
	case phaseFalling:
		switch k {
		case "left", "h":
			m.slide(-1, time.Now())
		case "right", "l":
			m.slide(1, time.Now())
		case "up", "k", "x":
			m.g.Rotate(true)
		case "z":
			m.g.Rotate(false)
		case "down", "j":
			m.pressDown(time.Now())
		case " ":
			for m.g.Fall() {
			}
			return m.lock()
		case "p":
			m.pause()
		}
	case phasePaused:
		// Any key resumes; quit was handled above.
		return m, m.unpause()
	case phaseLevelClear:
		if k == "enter" {
			m.level = min(MaxLevel, m.g.Level+1)
			return m.start(m.g.Score)
		}
	case phaseGameOver:
		if k == "enter" || k == "r" {
			m.phase = phaseMenu
			m.g = NewGame(m.level, m.speed, time.Now().UnixNano())
		}
	}
	return m, nil
}

func (m model) start(score int) (tea.Model, tea.Cmd) {
	m.g = NewGame(m.level, m.speed, time.Now().UnixNano())
	m.g.Score = score
	return m.resolve()
}

// lock writes the pill into the board; a down key still held waits for a fresh press.
func (m model) lock() (tea.Model, tea.Cmd) {
	m.g.Lock()
	m.down.Block()
	return m.resolve()
}

// slide moves the pill sideways, ignoring presses that arrive faster than moveGap.
func (m *model) slide(dx int, at time.Time) {
	if m.slideGate.Allow(at) {
		m.g.Move(dx)
	}
}

// pressDown registers a down press and starts the soft drop when it counts.
func (m *model) pressDown(at time.Time) {
	if m.down.Press(at) {
		m.g.Soft = true
	}
}

// resolve runs after a lock or a settled cascade: flash matches, or spawn, or end the round.
func (m model) resolve() (tea.Model, tea.Cmd) {
	if m.g.MarkMatches() > 0 {
		m.phase = phaseClearing
		return m, m.ticker.Tick(clearDelay)
	}
	if m.g.Viruses == 0 {
		m.phase = phaseLevelClear
		return m, nil
	}
	if !m.g.Spawn() {
		m.phase = phaseGameOver
		return m, nil
	}
	m.phase = phaseFalling
	return m, m.frame()
}

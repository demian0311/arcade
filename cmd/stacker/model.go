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
	phaseGameOver
)

const (
	frameDelay = 33 * time.Millisecond // animation rate
	// moveGap throttles horizontal auto-repeat, which arrives every 25 ms, to a pace a player can stop.
	moveGap  = 60 * time.Millisecond
	maxScale = 4
)

// bgMsg drives the backdrop animation on its own clock.
type bgMsg struct{ at time.Time }

type model struct {
	g             *Game
	fx            effects
	phase         phase
	resume        phase // the phase a pause returns to
	focus         session.Focus
	down          session.Hold
	slideGate     session.Throttle
	ticker        session.Ticker
	level         int
	width, height int
	scale         int
	titleScale    int
	lastFrame     time.Time
	bg            *backdrop.Backdrop
	lastBg        time.Time
}

func newModel() model {
	m := model{bg: &backdrop.Backdrop{}, slideGate: session.Throttle{Gap: moveGap}}
	m.g = NewGame(m.level, time.Now().UnixNano())
	return m
}

func (m model) Init() tea.Cmd { return bgTick() }

func bgTick() tea.Cmd {
	return tea.Tick(backdrop.Frame, func(t time.Time) tea.Msg { return bgMsg{t} })
}

func (m model) running() bool {
	return m.phase == phaseFalling || m.phase == phaseClearing
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
	if m.phase == phaseClearing {
		m.fx.clearAt = time.Now()
	}
	return m.frame()
}

// frame schedules the next animation frame and marks when the previous one was.
func (m *model) frame() tea.Cmd {
	m.lastFrame = time.Now()
	return m.ticker.Tick(frameDelay)
}

// fitScale picks the largest board scale whose framed well fits the
// terminal height, then the largest title scale whose side column fits the
// width, down to the plain-text title at 0. A title that leaves room to
// centre the well wins over a larger one that would push it aside. Both
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
			if msg.At.Sub(m.fx.clearAt) < clearDelay {
				return m, m.ticker.Tick(frameDelay)
			}
			m.g.Clear()
			m.fx.clearAt = time.Time{}
			return m.spawn()
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
		case "left", "h", "down", "j":
			m.level = max(0, m.level-1)
		case "right", "l", "up", "k":
			m.level = min(MaxLevel, m.level+1)
		case "enter":
			m.g = NewGame(m.level, time.Now().UnixNano())
			return m.spawn()
		default:
			return m, nil
		}
		m.g = NewGame(m.level, time.Now().UnixNano())
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
			return m.slam()
		case "p":
			m.pause()
		}
	case phasePaused:
		// Any key resumes; quit was handled above.
		return m, m.unpause()
	case phaseGameOver:
		if k == "enter" || k == "r" {
			m.phase = phaseMenu
			m.g = NewGame(m.level, time.Now().UnixNano())
		}
	}
	return m, nil
}

// slam drops the piece to its landing spot at once, leaving a trail over
// the band it crossed, and locks it.
func (m model) slam() (tea.Model, tea.Cmd) {
	p := *m.g.Piece
	x0, x1, y0 := BoardW, 0, BoardH
	p.cells(func(x, y int, _ Link) { x0, x1, y0 = min(x0, x), max(x1, x), min(y0, y) })
	for m.g.Fall() {
	}
	y1 := BoardH
	m.g.Piece.cells(func(_, y int, _ Link) { y1 = min(y1, y) })
	m.fx.slamX0, m.fx.slamX1, m.fx.slamY0, m.fx.slamY1 = x0, x1, max(0, y0), max(0, y1)
	m.fx.slamAt = time.Now()
	return m.lock()
}

// lock writes the piece into the well and starts its landing flash; a down
// key still held waits for a fresh press.
func (m model) lock() (tea.Model, tea.Cmd) {
	m.down.Block()
	m.fx.landed = m.fx.landed[:0]
	m.g.Piece.cells(func(x, y int, _ Link) { m.fx.landed = append(m.fx.landed, Point{x, y}) })
	m.fx.landedAt = time.Now()
	if !m.g.Lock() {
		m.phase = phaseGameOver
		return m, nil
	}
	if n := m.g.MarkFull(); n > 0 {
		m.phase = phaseClearing
		m.fx.clearAt = time.Now()
		sum := 0
		for _, p := range m.fx.landed {
			sum += p.X
		}
		m.fx.clearCX = float64(sum) / float64(len(m.fx.landed))
		m.bg.Flare([...]string{"", hex[m.g.Last], hex[Yellow], hex[Orange], "#ffffff"}[min(n, 4)])
		return m, m.frame()
	}
	return m.spawn()
}

// spawn starts the next piece, or ends the game when it has no room.
func (m model) spawn() (tea.Model, tea.Cmd) {
	if !m.g.Spawn() {
		m.phase = phaseGameOver
		return m, nil
	}
	m.phase = phaseFalling
	return m, m.frame()
}

// slide moves the piece sideways, ignoring presses that arrive faster than moveGap.
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

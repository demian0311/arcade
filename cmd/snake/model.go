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
	phaseRunning
	phasePaused
	phaseDying
	phaseGameOver
)

const (
	frameDelay = 33 * time.Millisecond  // animation rate
	dieDelay   = 400 * time.Millisecond // the snake shows white before the round ends
	maxScale   = 4
)

// bgMsg drives the backdrop animation on its own clock.
type bgMsg struct{ at time.Time }

type model struct {
	g             *Game
	phase         phase
	focus         session.Focus
	ticker        session.Ticker
	speed         Speed
	width, height int
	scale         int
	titleScale    int
	lastFrame     time.Time
	bg            *backdrop.Backdrop
	lastBg        time.Time
}

func newModel() model {
	m := model{speed: Med, bg: &backdrop.Backdrop{}}
	m.g = NewGame(m.speed, time.Now().UnixNano())
	return m
}

func (m model) Init() tea.Cmd { return bgTick() }

func bgTick() tea.Cmd {
	return tea.Tick(backdrop.Frame, func(t time.Time) tea.Msg { return bgMsg{t} })
}

func (m model) running() bool { return m.phase == phaseRunning }

// unpause restarts the clock.
func (m *model) unpause() tea.Cmd {
	m.phase = phaseRunning
	m.focus.Resumed()
	return m.frame()
}

// frame schedules the next animation frame and marks when the previous one was.
func (m *model) frame() tea.Cmd {
	m.lastFrame = time.Now()
	return m.ticker.Tick(frameDelay)
}

// fitScale picks the largest board scale whose framed field fits the
// terminal height, then the largest title scale whose side column fits the
// width, down to the plain-text title at 0. A title that leaves room to
// centre the field wins over a larger one that would push it aside. Both
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
					need += colWidth(ts) + colPad
				}
				if need <= w && sideRows(ts) <= boardRows(s) {
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
			m.phase = phasePaused
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
		case phaseRunning:
			dt := msg.At.Sub(m.lastFrame)
			m.lastFrame = msg.At
			eaten := m.g.Eaten
			if m.g.Advance(dt) {
				m.phase = phaseDying
				m.bg.Flare(hex[inkWhite])
				return m, m.ticker.Tick(dieDelay)
			}
			if m.g.Eaten > eaten {
				m.bg.Flare(hex[inkFood])
			}
			return m, m.ticker.Tick(frameDelay)
		case phaseDying:
			m.phase = phaseGameOver
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
		case "up", "k", "right", "l":
			m.speed = min(Hi, m.speed+1)
		case "down", "j", "left", "h":
			m.speed = max(Low, m.speed-1)
		case "enter":
			m.g = NewGame(m.speed, time.Now().UnixNano())
			return m, m.unpause()
		default:
			return m, nil
		}
		m.g = NewGame(m.speed, time.Now().UnixNano())
	case phaseRunning:
		switch k {
		case "left", "h":
			m.g.Turn(Left)
		case "right", "l":
			m.g.Turn(Right)
		case "up", "k":
			m.g.Turn(Up)
		case "down", "j":
			m.g.Turn(Down)
		case "p":
			m.phase = phasePaused
		}
	case phasePaused:
		if k == "p" {
			return m, m.unpause()
		}
	case phaseGameOver:
		if k == "enter" || k == "r" {
			m.phase = phaseMenu
			m.g = NewGame(m.speed, time.Now().UnixNano())
		}
	}
	return m, nil
}

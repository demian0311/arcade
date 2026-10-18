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
	phasePlaying
	phasePaused
	phaseWaveClear
	phaseGameOver
)

const (
	frameDelay = 16 * time.Millisecond // physics and animation rate
	maxScale   = 4
)

// bgMsg drives the backdrop animation on its own clock.
type bgMsg struct{ at time.Time }

type model struct {
	g             *Game
	phase         phase
	focus         session.Focus
	left, right   session.Hold
	ticker        session.Ticker
	width, height int
	scale         int
	titleScale    int
	lastFrame     time.Time
	bg            *backdrop.Backdrop
	lastBg        time.Time
}

func newModel() model {
	m := model{bg: &backdrop.Backdrop{}}
	m.g = NewGame(time.Now().UnixNano())
	return m
}

func (m model) Init() tea.Cmd { return bgTick() }

func bgTick() tea.Cmd {
	return tea.Tick(backdrop.Frame, func(t time.Time) tea.Msg { return bgMsg{t} })
}

func (m model) running() bool { return m.phase == phasePlaying }

// unpause restarts the clock.
func (m *model) unpause() tea.Cmd {
	m.phase = phasePlaying
	m.focus.Resumed()
	return m.frame()
}

// frame schedules the next physics frame and marks when the previous one was.
func (m *model) frame() tea.Cmd {
	m.lastFrame = time.Now()
	return m.ticker.Tick(frameDelay)
}

// fitScale picks the largest field scale whose frame fits the terminal
// height, then the largest title scale whose side column fits the width,
// down to the plain-text title at 0. A title that leaves room to centre the
// field wins over a larger one that would push it aside. Both are 0 when
// nothing fits.
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
		if !m.ticker.Current(msg) || m.phase != phasePlaying {
			return m, nil
		}
		dt := msg.At.Sub(m.lastFrame)
		m.lastFrame = msg.At
		return m.advance(dt, msg.At)
	}
	return m, nil
}

// advance runs one physics frame and reacts to what happened in it.
func (m model) advance(dt time.Duration, at time.Time) (tea.Model, tea.Cmd) {
	if m.left.Active(at) != m.right.Active(at) {
		dx := CannonSpeed * dt.Seconds()
		if m.left.Active(at) {
			dx = -dx
		}
		m.g.Move(dx)
	}
	m.g.Advance(dt)
	if m.g.Killed {
		m.bg.Flare(hex[kindInk[m.g.Flare]])
	}
	if m.g.Lost {
		m.bg.Flare(hex[inkCannon])
	}
	switch {
	case m.g.Over || m.g.Lives <= 0:
		m.phase = phaseGameOver
		return m, nil
	case m.g.Remaining() == 0:
		m.phase = phaseWaveClear
		return m, nil
	}
	return m, m.ticker.Tick(frameDelay)
}

func (m model) key(k string) (tea.Model, tea.Cmd) {
	if k == "ctrl+c" || k == "q" {
		return m, tea.Quit
	}
	switch m.phase {
	case phaseMenu:
		if k == "enter" {
			m.g = NewGame(time.Now().UnixNano())
			return m, m.unpause()
		}
	case phasePlaying:
		switch k {
		case "left", "h":
			m.left.Press(time.Now())
		case "right", "l":
			m.right.Press(time.Now())
		case " ":
			m.g.Fire()
		case "p":
			m.phase = phasePaused
		}
	case phasePaused:
		// Any key resumes; quit was handled above.
		return m, m.unpause()
	case phaseWaveClear:
		if k == "enter" {
			m.g.startWave(m.g.Wave + 1)
			return m, m.unpause()
		}
	case phaseGameOver:
		if k == "enter" || k == "r" {
			m.phase = phaseMenu
			m.g = NewGame(time.Now().UnixNano())
		}
	}
	return m, nil
}

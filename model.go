package main

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	frameDelay = 33 * time.Millisecond // falling-pill animation rate
	clearDelay = 350 * time.Millisecond
	dropDelay  = 90 * time.Millisecond
	// holdGap separates key auto-repeat from a real re-press. Terminals send no
	// key-up, so a down press less than this after the previous one counts as held.
	holdGap  = 100 * time.Millisecond
	maxScale = 4
)

// tickMsg drives the pill, the clear flash and the drop cascade; the id discards stale ticks.
type tickMsg struct {
	id int
	at time.Time
}

// bgMsg drives the backdrop animation on its own clock.
type bgMsg struct{ at time.Time }

type model struct {
	g             *Game
	phase         phase
	level         int
	speed         Speed
	width, height int
	scale         int
	titleScale    int
	tickID        int
	lastFrame     time.Time
	lastDown      time.Time
	softBlocked   bool // after a landing, soft drop waits for the down key to be released and pressed again
	bg            *backdrop
	lastBg        time.Time
}

func newModel() model {
	m := model{speed: Med, bg: &backdrop{}}
	m.g = NewGame(m.level, m.speed, time.Now().UnixNano())
	return m
}

func (m model) Init() tea.Cmd { return bgTick() }

func bgTick() tea.Cmd {
	return tea.Tick(bgFrame, func(t time.Time) tea.Msg { return bgMsg{t} })
}

func (m *model) tick(d time.Duration) tea.Cmd {
	m.tickID++
	id := m.tickID
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg{id, t} })
}

// frame schedules the next pill animation frame and marks when the previous one was.
func (m *model) frame() tea.Cmd {
	m.lastFrame = time.Now()
	return m.tick(frameDelay)
}

// fitScale picks the largest board scale, then the largest title scale, whose
// stacked layout fits the terminal. Both are 0 when nothing fits.
func fitScale(w, h int) (s, ts int) {
	for s = maxScale; s >= 1; s-- {
		if BoardW*2*s+2 > w-2 {
			continue
		}
		for ts = maxTitleScale; ts >= 1; ts-- {
			if layoutHeight(s, ts) <= h && titleWidth(ts) <= w-2 {
				return s, ts
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
		m.bg.resize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyMsg:
		return m.key(msg.String())
	case bgMsg:
		if !m.lastBg.IsZero() {
			m.bg.step(msg.at.Sub(m.lastBg))
		}
		m.lastBg = msg.at
		return m, bgTick()
	case tickMsg:
		if msg.id != m.tickID {
			return m, nil
		}
		switch m.phase {
		case phaseFalling:
			dt := msg.at.Sub(m.lastFrame)
			m.lastFrame = msg.at
			m.g.Soft = m.softActive(msg.at)
			if m.g.Advance(dt) {
				return m.lock()
			}
			return m, m.tick(frameDelay)
		case phaseClearing:
			m.bg.flare(m.g.clearingColor())
			m.g.Clear()
			m.phase = phaseDropping
			return m, m.tick(dropDelay)
		case phaseDropping:
			if m.g.DropStep() {
				return m, m.tick(dropDelay)
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
			m.g.Move(-1)
		case "right", "l":
			m.g.Move(1)
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
			m.phase = phasePaused
		}
	case phasePaused:
		if k == "p" {
			m.phase = phaseFalling
			return m, m.frame()
		}
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

func (m model) lock() (tea.Model, tea.Cmd) {
	m.g.Lock()
	m.softBlocked = true
	return m.resolve()
}

// pressDown registers a down press. Soft drop runs while presses keep arriving
// within holdGap of each other, and a key still held from the previous pill is ignored.
func (m *model) pressDown(at time.Time) {
	held := !m.lastDown.IsZero() && at.Sub(m.lastDown) < holdGap
	m.lastDown = at
	if m.softBlocked && held {
		return
	}
	m.softBlocked = false
	m.g.Soft = true
}

// softActive reports whether the down key counts as held right now.
func (m model) softActive(at time.Time) bool {
	return !m.softBlocked && !m.lastDown.IsZero() && at.Sub(m.lastDown) < holdGap
}

// resolve runs after a lock or a settled cascade: flash matches, or spawn, or end the round.
func (m model) resolve() (tea.Model, tea.Cmd) {
	if m.g.MarkMatches() > 0 {
		m.phase = phaseClearing
		return m, m.tick(clearDelay)
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

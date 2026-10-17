package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	sidebarW   = 24
	// holdGap separates key auto-repeat from a real re-press. Terminals send no
	// key-up, so a down press less than this after the previous one counts as held.
	holdGap  = 100 * time.Millisecond
	chromeW  = 6 // bottle border, gap to the sidebar, outer padding
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

// fitScale picks the largest cell scale whose board and sidebar fit the terminal.
func fitScale(w, h int) int {
	s := min((h-2)/BoardH, (w-chromeW-sidebarW)/(BoardW*2))
	return min(s, maxScale)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.scale = fitScale(msg.Width, msg.Height)
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

var (
	frameStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#8a8a8a"))
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ff6b6b"))
	labelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#8a8a8a"))
	valueStyle = lipgloss.NewStyle().Bold(true)
	noteStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffd75f"))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#6c6c6c"))
)

func (m model) View() string {
	if m.width == 0 {
		return ""
	}
	if m.scale < 1 {
		need := fmt.Sprintf("pilldoc needs %dx%d, have %dx%d", BoardW*2+chromeW+sidebarW, BoardH+2, m.width, m.height)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, need)
	}
	frame := frameStyle.Render(renderBoard(m.g, m.scale))
	content := lipgloss.NewStyle().Padding(0, 1).Render(lipgloss.JoinHorizontal(lipgloss.Top, frame, "  ", m.sidebar()))
	return m.compose(content)
}

// compose centres the game over the backdrop; the game block is opaque, the field shows around it.
func (m model) compose(content string) string {
	lines := strings.Split(content, "\n")
	cw, ch := lipgloss.Width(content), len(lines)
	x0, y0 := max(0, (m.width-cw)/2), max(0, (m.height-ch)/2)
	m.bg.resize(m.width, m.height)
	st := m.bg.style()
	var out strings.Builder
	for row := 0; row < m.height; row++ {
		if row >= y0 && row < y0+ch {
			left, right := m.bg.line(row, x0, x0+cw)
			out.WriteString(st.Render(left))
			out.WriteString(lines[row-y0])
			out.WriteString(st.Render(right))
		} else {
			left, _ := m.bg.line(row, m.width, m.width)
			out.WriteString(st.Render(left))
		}
		if row < m.height-1 {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

func (m model) sidebar() string {
	stat := func(label string, v any) string {
		return labelStyle.Render(fmt.Sprintf("%-6s", label)) + " " + valueStyle.Render(fmt.Sprint(v))
	}
	lines := []string{
		titleStyle.Render("P I L L D O C"),
		"",
		stat("LEVEL", m.g.Level),
		stat("SPEED", m.g.Speed),
		stat("SCORE", m.g.Score),
		stat("VIRUS", m.g.Viruses),
		"",
		labelStyle.Render("NEXT"),
		renderNext(m.g.Next, m.scale),
		"",
	}
	switch m.phase {
	case phaseMenu:
		lines = append(lines,
			dimStyle.Render("◀ ▶  level"),
			dimStyle.Render("▲ ▼  speed"),
			"",
			noteStyle.Render("enter  start"))
	case phaseFalling, phaseClearing, phaseDropping:
		lines = append(lines,
			dimStyle.Render("◀ ▶  move"),
			dimStyle.Render("▲ z  rotate"),
			dimStyle.Render("▼    drop"),
			dimStyle.Render("spc  slam"),
			dimStyle.Render("p    pause"))
	case phasePaused:
		lines = append(lines, noteStyle.Render("PAUSED"), dimStyle.Render("p    resume"))
	case phaseLevelClear:
		lines = append(lines, noteStyle.Render("LEVEL CLEAR"), dimStyle.Render("enter  next level"))
	case phaseGameOver:
		lines = append(lines, noteStyle.Render("GAME OVER"), dimStyle.Render("enter  menu"))
	}
	lines = append(lines, dimStyle.Render("q    quit"))
	return lipgloss.NewStyle().Width(sidebarW).Render(strings.Join(lines, "\n"))
}

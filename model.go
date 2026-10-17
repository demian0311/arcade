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
	clearDelay = 350 * time.Millisecond
	dropDelay  = 90 * time.Millisecond
	sidebarW   = 24
	maxScale   = 4
)

// tickMsg drives gravity, the clear flash and the drop cascade; the id discards stale ticks.
type tickMsg struct{ id int }

type model struct {
	g             *Game
	phase         phase
	level         int
	speed         Speed
	width, height int
	scale         int
	tickID        int
}

func newModel() model {
	m := model{speed: Med}
	m.g = NewGame(m.level, m.speed, time.Now().UnixNano())
	return m
}

func (m model) Init() tea.Cmd { return nil }

func (m *model) tick(d time.Duration) tea.Cmd {
	m.tickID++
	id := m.tickID
	return tea.Tick(d, func(time.Time) tea.Msg { return tickMsg{id} })
}

// fitScale picks the largest cell scale whose board and sidebar fit the terminal.
func fitScale(w, h int) int {
	s := min((h-2)/BoardH, (w-4-sidebarW)/(BoardW*2))
	return min(s, maxScale)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.scale = fitScale(msg.Width, msg.Height)
		return m, nil
	case tea.KeyMsg:
		return m.key(msg.String())
	case tickMsg:
		if msg.id != m.tickID {
			return m, nil
		}
		switch m.phase {
		case phaseFalling:
			if m.g.Fall() {
				return m, m.tick(m.g.Interval())
			}
			return m.lock()
		case phaseClearing:
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
			if !m.g.Fall() {
				return m.lock()
			}
			return m, m.tick(m.g.Interval())
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
			return m, m.tick(m.g.Interval())
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
	return m.resolve()
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
	return m, m.tick(m.g.Interval())
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
		need := fmt.Sprintf("pilldoc needs %dx%d, have %dx%d", BoardW*2+4+sidebarW, BoardH+2, m.width, m.height)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, need)
	}
	frame := frameStyle.Render(renderBoard(m.g, m.scale))
	content := lipgloss.JoinHorizontal(lipgloss.Top, frame, "  ", m.sidebar())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
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

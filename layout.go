package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	frameStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#8a8a8a"))
	labelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#8a8a8a"))
	valueStyle = lipgloss.NewStyle().Bold(true)
	noteStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffd75f"))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#6c6c6c"))
	padStyle   = lipgloss.NewStyle().Padding(0, 1)
	fadeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#585858"))
)

// A block is an opaque rectangle laid over the backdrop.
type block struct {
	x, y  int
	w     int
	lines []string
}

func newBlock(s string) block {
	lines := strings.Split(s, "\n")
	w := 0
	for _, l := range lines {
		w = max(w, lipgloss.Width(l))
	}
	for i, l := range lines {
		lines[i] = l + strings.Repeat(" ", w-lipgloss.Width(l))
	}
	return block{w: w, lines: lines}
}

func (b block) h() int { return len(b.lines) }

// glyphs is a 5x7 dot font for the title.
var glyphs = map[rune][7]string{
	'P': {"####.", "#...#", "#...#", "####.", "#....", "#....", "#...."},
	'I': {"#####", "..#..", "..#..", "..#..", "..#..", "..#..", "#####"},
	'L': {"#....", "#....", "#....", "#....", "#....", "#....", "#####"},
	'D': {"####.", "#...#", "#...#", "#...#", "#...#", "#...#", "####."},
	'O': {".###.", "#...#", "#...#", "#...#", "#...#", "#...#", ".###."},
	'C': {".###.", "#...#", "#....", "#....", "#....", "#...#", ".###."},
}

const (
	titleText = "PILLDOC"
	glyphW    = 5
	glyphH    = 7
	glyphGap  = 1

	maxTitleScale = 2
)

func titleDots(ts int) (w, h int) {
	w = (len(titleText)*(glyphW+glyphGap) - glyphGap) * ts
	return w, glyphH * ts
}

// Title scale 0 is plain text, for terminals too narrow for the dot font.
func titleWidth(ts int) int {
	if ts == 0 {
		return len(titleText)
	}
	w, _ := titleDots(ts)
	return (w + 1) / 2
}

func titleHeight(ts int) int {
	if ts == 0 {
		return 1
	}
	_, h := titleDots(ts)
	return (h + 3) / 4
}

// renderTitle draws PILLDOC in braille, each letter in a pill colour; at
// scale 0 it is bold text in the same colours.
func renderTitle(ts int) string {
	if ts == 0 {
		var b strings.Builder
		for i, r := range titleText {
			b.WriteString(inkStyles[ink(colors[i%3])].Bold(true).Render(string(r)))
		}
		return b.String()
	}

	c := &canvas{w: titleWidth(ts) * 2, h: titleHeight(ts) * 4}
	c.dots = make([]ink, c.w*c.h)
	for i, r := range titleText {
		g := glyphs[r]
		i0 := ink(colors[i%3])
		ox := i * (glyphW + glyphGap) * ts
		for gy := 0; gy < glyphH; gy++ {
			for gx := 0; gx < glyphW; gx++ {
				if g[gy][gx] != '#' {
					continue
				}
				for dy := 0; dy < ts; dy++ {
					for dx := 0; dx < ts; dx++ {
						c.set(ox+gx*ts+dx, gy*ts+dy, i0)
					}
				}
			}
		}
	}
	return c.String()
}

const (
	colPad = 1 // columns between a side column and the board frame

	// controlsW is the widest padded key hint ("enter start"), so a side
	// column never has to break a hint in two.
	controlsW = 13
	// maxControlLines is the most hints any phase shows, one per line.
	maxControlLines = 6
	// statsW is a padded stat with a seven-digit value.
	statsW = 15
)

// boardRows and boardCols are the framed board's size at scale s.
func boardRows(s int) int { return BoardH*s + 2 }
func boardCols(s int) int { return BoardW*2*s + 2 }

// nextW is the padded NEXT line at scale s.
func nextW(s int) int { return 8 + 4*s }

// colWidth is the width of each side column at board scale s and title scale
// ts. Both columns are the same width, which keeps the board centred.
func colWidth(s, ts int) int {
	return max(titleWidth(ts)+2, nextW(s), controlsW, statsW)
}

// layoutWidth is the columns the side-by-side layout needs.
func layoutWidth(s, ts int) int {
	return 2*(colWidth(s, ts)+colPad) + boardCols(s)
}

// sideRows is the most rows the left column can need: title, next pill and
// controls, each separated by a blank row.
func sideRows(s, ts int) int {
	return titleHeight(ts) + 1 + s + 1 + maxControlLines
}

func (m model) stats() string {
	stat := func(label string, v any) string {
		return labelStyle.Render(label) + " " + valueStyle.Render(fmt.Sprint(v))
	}
	return padStyle.Render(strings.Join([]string{
		stat("LEVEL", m.g.Level), stat("SPEED", m.g.Speed), stat("SCORE", m.g.Score), stat("VIRUS", m.g.Viruses),
	}, "\n"))
}

// controls is the key hint line for the current phase, wrapped to the side column.
func (m model) controls() string {
	var items []string
	note := ""
	switch m.phase {
	case phaseMenu:
		items = []string{"◀ ▶ level", "▲ ▼ speed", "enter start"}
	case phaseFalling, phaseClearing, phaseDropping:
		items = []string{"◀ ▶ move", "▲ z rotate", "▼ drop", "spc slam", "p pause"}
	case phasePaused:
		note, items = "PAUSED", []string{"p resume"}
		if m.countdown > 0 {
			note = fmt.Sprintf("RESUMING IN %d", m.countdown)
		}
	case phaseLevelClear:
		note, items = "LEVEL CLEAR", []string{"enter next"}
	case phaseGameOver:
		note, items = "GAME OVER", []string{"enter menu"}
	}
	items = append(items, "q quit")
	wrap := colWidth(m.scale, m.titleScale) - 2
	var lines []string
	line := ""
	if note != "" {
		line = noteStyle.Render(note)
	}
	for _, it := range items {
		it = dimStyle.Render(it)
		if line == "" {
			line = it
		} else if lipgloss.Width(line)+3+lipgloss.Width(it) > wrap {
			lines = append(lines, line)
			line = it
		} else {
			line += "   " + it
		}
	}
	lines = append(lines, line)
	return padStyle.Render(strings.Join(lines, "\n"))
}

func (m model) next() string {
	return padStyle.Render(lipgloss.JoinHorizontal(lipgloss.Center, labelStyle.Render("NEXT"), "  ", renderNext(m.g.Next, m.scale)))
}

// blocks centres the bottle and fills the height with it. The left column
// hugs the bottle with the title and next pill at the top and the controls at
// the bottom; the right column holds the stats. Blocks are ordered left to
// right for compose.
func (m model) blocks() []block {
	board := newBlock(frameStyle.Render(renderBoard(m.g, m.scale)))
	board.x = (m.width - board.w) / 2
	board.y = (m.height - board.h()) / 2

	title := newBlock(padStyle.Render(renderTitle(m.titleScale)))
	next := newBlock(m.next())
	ctl := newBlock(m.controls())
	stats := newBlock(m.stats())

	leftEdge := board.x - colPad
	title.x, title.y = leftEdge-title.w, board.y
	next.x, next.y = leftEdge-next.w, title.y+title.h()+1
	ctl.x, ctl.y = leftEdge-ctl.w, max(next.y+next.h()+1, board.y+board.h()-ctl.h())
	stats.x, stats.y = board.x+board.w+colPad, board.y
	return []block{title, next, ctl, board, stats}
}

func (m model) View() string {
	if m.width == 0 {
		return ""
	}
	if m.scale < 1 {
		need := fmt.Sprintf("pilldoc needs %dx%d, have %dx%d", layoutWidth(1, 0), boardRows(1), m.width, m.height)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, need)
	}
	v := m.compose(m.blocks())
	if m.blurred {
		return fade(v)
	}
	return v
}

// fade drops every colour from a rendered view and paints it one dim grey, so
// an unfocused terminal reads as asleep.
func fade(v string) string {
	lines := strings.Split(ansi.Strip(v), "\n")
	for i, l := range lines {
		lines[i] = fadeStyle.Render(l)
	}
	return strings.Join(lines, "\n")
}

// compose paints the backdrop and lays each block over it.
func (m model) compose(bs []block) string {
	m.bg.resize(m.width, m.height)
	st := m.bg.style()
	var out strings.Builder
	for row := 0; row < m.height; row++ {
		x := 0
		for _, b := range bs {
			if row < b.y || row >= b.y+b.h() {
				continue
			}
			left, _ := m.bg.line(row, b.x, b.x)
			out.WriteString(st.Render(string([]rune(left)[x:])))
			out.WriteString(b.lines[row-b.y])
			x = b.x + b.w
		}
		_, right := m.bg.line(row, x, x)
		out.WriteString(st.Render(right))
		if row < m.height-1 {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

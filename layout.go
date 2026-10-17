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
	colPad = 1 // columns between the side column and the board frame

	// statsW is a padded stat with a seven-digit value; the widest note
	// ("RESUMING IN 3", padded) is the same width.
	statsW = 15
	// statRows is one line per stat plus the note under a blank row.
	statRows = 4 + 2
)

// boardRows and boardCols are the framed board's size at scale s.
func boardRows(s int) int { return BoardH*s + 2 }
func boardCols(s int) int { return BoardW*2*s + 2 }

// nextW is the padded NEXT line at scale s.
func nextW(s int) int { return 8 + 4*s }

// colWidth is the width of the side column at board scale s and title scale ts.
func colWidth(s, ts int) int {
	return max(titleWidth(ts)+2, nextW(s), statsW)
}

// layoutWidth is the columns the side-by-side layout needs.
func layoutWidth(s, ts int) int {
	return colWidth(s, ts) + colPad + boardCols(s)
}

// sideRows is the rows the side column needs: title, next pill and stats,
// each separated by a blank row.
func sideRows(s, ts int) int {
	return titleHeight(ts) + 1 + s + 1 + statRows
}

// stats lists the counters and, under them, the note for the current phase.
func (m model) stats() string {
	stat := func(label string, v any) string {
		return labelStyle.Render(label) + " " + valueStyle.Render(fmt.Sprint(v))
	}
	lines := []string{
		stat("LEVEL", m.g.Level), stat("SPEED", m.g.Speed), stat("SCORE", m.g.Score), stat("VIRUS", m.g.Viruses),
	}
	if n := m.note(); n != "" {
		lines = append(lines, "", noteStyle.Render(n))
	}
	return padStyle.Render(strings.Join(lines, "\n"))
}

// note is the phase the player has to act on, or "" during play.
func (m model) note() string {
	switch m.phase {
	case phaseMenu:
		return "ENTER START"
	case phasePaused:
		if m.countdown > 0 {
			return fmt.Sprintf("RESUMING IN %d", m.countdown)
		}
		return "PAUSED"
	case phaseLevelClear:
		return "LEVEL CLEAR"
	case phaseGameOver:
		return "GAME OVER"
	}
	return ""
}

func (m model) next() string {
	return padStyle.Render(lipgloss.JoinHorizontal(lipgloss.Center, labelStyle.Render("NEXT"), "  ", renderNext(m.g.Next, m.scale)))
}

// blocks fills the height with the bottle and hugs its left side with one
// column: title, next pill, then the stats and note. The bottle is centred
// in the terminal when the column fits beside it; otherwise the column and
// bottle are centred together. Blocks are ordered left to right for compose.
func (m model) blocks() []block {
	board := newBlock(frameStyle.Render(renderBoard(m.g, m.scale)))
	colW := colWidth(m.scale, m.titleScale)
	board.x = max((m.width-board.w)/2, (m.width-colW-colPad-board.w)/2+colW+colPad)
	board.y = (m.height - board.h()) / 2

	title := newBlock(padStyle.Render(renderTitle(m.titleScale)))
	next := newBlock(m.next())
	stats := newBlock(m.stats())

	edge := board.x - colPad
	title.x, title.y = edge-title.w, board.y
	next.x, next.y = edge-next.w, title.y+title.h()+1
	stats.x, stats.y = edge-stats.w, next.y+next.h()+1
	return []block{title, next, stats, board}
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

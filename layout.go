package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	frameStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#8a8a8a"))
	labelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#8a8a8a"))
	valueStyle = lipgloss.NewStyle().Bold(true)
	noteStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffd75f"))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#6c6c6c"))
	padStyle   = lipgloss.NewStyle().Padding(0, 1)
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

func titleWidth(ts int) int  { w, _ := titleDots(ts); return (w + 1) / 2 }
func titleHeight(ts int) int { _, h := titleDots(ts); return (h + 3) / 4 }

// renderTitle draws PILLDOC in braille, each letter in a pill colour.
func renderTitle(ts int) string {

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

// layoutHeight is the rows the stacked layout needs at board scale s and title scale ts, with no gaps.
func layoutHeight(s, ts int) int {
	return titleHeight(ts) + s + BoardH*s + 2 + 2
}

func (m model) stats() string {
	stat := func(label string, v any) string {
		return labelStyle.Render(label) + " " + valueStyle.Render(fmt.Sprint(v))
	}
	return padStyle.Render(strings.Join([]string{
		stat("LEVEL", m.g.Level), stat("SPEED", m.g.Speed), stat("SCORE", m.g.Score), stat("VIRUS", m.g.Viruses),
	}, "   "))
}

// controls is the key hint line for the current phase, wrapped to the terminal width.
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
	case phaseLevelClear:
		note, items = "LEVEL CLEAR", []string{"enter next level"}
	case phaseGameOver:
		note, items = "GAME OVER", []string{"enter menu"}
	}
	items = append(items, "q quit")
	var lines []string
	line := ""
	if note != "" {
		line = noteStyle.Render(note)
	}
	for _, it := range items {
		it = dimStyle.Render(it)
		if line == "" {
			line = it
		} else if lipgloss.Width(line)+3+lipgloss.Width(it) > m.width-2 {
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

// blocks stacks title, next pill, bottle, stats and controls down the middle of
// the terminal, spreading spare rows into gaps that show the backdrop.
func (m model) blocks() []block {
	bs := []block{
		newBlock(padStyle.Render(renderTitle(m.titleScale))),
		newBlock(m.next()),
		newBlock(frameStyle.Render(renderBoard(m.g, m.scale))),
		newBlock(m.stats()),
		newBlock(m.controls()),
	}
	need := 0
	for _, b := range bs {
		need += b.h()
	}
	spare := max(0, m.height-need)
	gap := min(2, spare/(len(bs)-1))
	y := (m.height - need - gap*(len(bs)-1)) / 2
	for i := range bs {
		bs[i].x = max(0, (m.width-bs[i].w)/2)
		bs[i].y = y
		y += bs[i].h() + gap
	}
	return bs
}

func (m model) View() string {
	if m.width == 0 {
		return ""
	}
	if m.scale < 1 {
		need := fmt.Sprintf("pilldoc needs %dx%d, have %dx%d", titleWidth(1)+2, layoutHeight(1, 1), m.width, m.height)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, need)
	}
	return m.compose(m.blocks())
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

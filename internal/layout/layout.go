// Package layout lays opaque blocks over a backdrop and holds the styles and
// dot-font title the games share.
package layout

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	Frame = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#8a8a8a"))
	Label = lipgloss.NewStyle().Foreground(lipgloss.Color("#8a8a8a"))
	Value = lipgloss.NewStyle().Bold(true)
	Note  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffd75f"))
	Pad   = lipgloss.NewStyle().Padding(0, 1)
	fade  = lipgloss.NewStyle().Foreground(lipgloss.Color("#585858"))
)

// A Block is an opaque rectangle laid over the backdrop at (X,Y).
type Block struct {
	X, Y  int
	W     int
	Lines []string
}

// NewBlock wraps a rendered string, padding every line to the widest.
func NewBlock(s string) Block {
	lines := strings.Split(s, "\n")
	w := 0
	for _, l := range lines {
		w = max(w, lipgloss.Width(l))
	}
	for i, l := range lines {
		lines[i] = l + strings.Repeat(" ", w-lipgloss.Width(l))
	}
	return Block{W: w, Lines: lines}
}

// H is the block's height in rows.
func (b Block) H() int { return len(b.Lines) }

// A Background paints the rows around the blocks.
type Background interface {
	Style() lipgloss.Style
	Line(row, x0, x1 int) (left, right string)
}

// Compose paints a w x h view: the background with every block laid over it.
// Blocks that share a row must be ordered left to right and must not overlap.
func Compose(bg Background, w, h int, bs []Block) string {
	st := bg.Style()
	var out strings.Builder
	for row := 0; row < h; row++ {
		x := 0
		for _, b := range bs {
			if row < b.Y || row >= b.Y+b.H() {
				continue
			}
			left, _ := bg.Line(row, b.X, b.X)
			out.WriteString(st.Render(string([]rune(left)[x:])))
			out.WriteString(b.Lines[row-b.Y])
			x = b.X + b.W
		}
		_, right := bg.Line(row, x, x)
		out.WriteString(st.Render(right))
		if row < h-1 {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

// Fade drops every colour from a rendered view and paints it one dim grey,
// so an unfocused terminal reads as asleep.
func Fade(v string) string {
	lines := strings.Split(ansi.Strip(v), "\n")
	for i, l := range lines {
		lines[i] = fade.Render(l)
	}
	return strings.Join(lines, "\n")
}

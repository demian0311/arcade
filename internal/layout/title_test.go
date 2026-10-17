package layout

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/demian0311/arcade/internal/canvas"
)

func TestFontCoversTheAlphabet(t *testing.T) {
	for r := 'A'; r <= 'Z'; r++ {
		g, ok := glyphs[r]
		if !ok {
			t.Fatalf("no glyph for %c", r)
		}
		for i, row := range g {
			if len(row) != GlyphW || strings.Trim(row, "#.") != "" {
				t.Fatalf("%c row %d is %q", r, i, row)
			}
		}
	}
}

func TestTitleSize(t *testing.T) {
	pal := canvas.Palette{1: lipgloss.NewStyle()}
	for _, ts := range []int{0, 1, 2} {
		s := Title("TETRIS", ts, []canvas.Ink{1}, pal)
		lines := strings.Split(s, "\n")
		if len(lines) != TitleHeight("TETRIS", ts) {
			t.Fatalf("scale %d: %d lines", ts, len(lines))
		}
		if w := lipgloss.Width(lines[0]); w != TitleWidth("TETRIS", ts) {
			t.Fatalf("scale %d: width %d", ts, w)
		}
	}
}

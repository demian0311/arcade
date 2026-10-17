package canvas

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestPixelsRenderHalfBlocks(t *testing.T) {
	pal := Palette{1: lipgloss.NewStyle(), 2: lipgloss.NewStyle()}
	p := NewPixels(3, 2, pal)
	p.Set(0, 0, 1)
	p.Set(1, 1, 1)
	p.Set(2, 0, 1)
	p.Set(2, 1, 2)
	if got := p.String(); got != "▀▄▀" {
		t.Fatalf("got %q", got)
	}
	p = NewPixels(2, 4, pal)
	p.Tile(0, 0, 2, 4, 0, Corners{}, 1)
	if got := p.String(); strings.Count(got, "█") != 4 {
		t.Fatalf("got %q", got)
	}
}

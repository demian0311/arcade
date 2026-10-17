package canvas

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Pixels is a grid drawn with half-block characters. Each character holds
// two pixels, top and bottom, each in its own colour, so a pixel is about
// square and any two inks can meet inside a character. It is coarser than
// braille but comes in finer size steps.
type Pixels struct {
	W, H int
	px   []Ink
	pal  Palette
}

// NewPixels makes a blank w x h grid painted from pal; h must be even.
func NewPixels(w, h int, pal Palette) *Pixels {
	return &Pixels{W: w, H: h, px: make([]Ink, w*h), pal: pal}
}

// Set paints one pixel; pixels outside the grid are dropped.
func (p *Pixels) Set(x, y int, i Ink) {
	if x >= 0 && x < p.W && y >= 0 && y < p.H {
		p.px[y*p.W+x] = i
	}
}

// Tile paints a w x h rectangle with its top-left pixel at (ox,oy), cutting
// each rounded corner back by radius r.
func (p *Pixels) Tile(ox, oy, w, h, r int, round Corners, i Ink) {
	tile(p.Set, ox, oy, w, h, r, round, i)
}

// String renders the grid as styled half blocks, one terminal row per line.
func (p *Pixels) String() string {
	var out strings.Builder
	rows := p.H / 2
	for row := 0; row < rows; row++ {
		var run strings.Builder
		var runTop, runBot Ink
		flush := func() {
			if run.Len() == 0 {
				return
			}
			switch {
			case runTop == 0 && runBot == 0:
				out.WriteString(run.String())
			case runBot == 0:
				out.WriteString(p.pal[runTop].Render(run.String()))
			case runTop == 0:
				out.WriteString(p.pal[runBot].Render(run.String()))
			default:
				st := lipgloss.NewStyle().Foreground(p.pal[runTop].GetForeground()).Background(p.pal[runBot].GetForeground())
				out.WriteString(st.Render(run.String()))
			}
			run.Reset()
		}
		for col := 0; col < p.W; col++ {
			top, bot := p.px[(row*2)*p.W+col], p.px[(row*2+1)*p.W+col]
			if top != runTop || bot != runBot {
				flush()
				runTop, runBot = top, bot
			}
			switch {
			case top == 0 && bot == 0:
				run.WriteByte(' ')
			case bot == 0:
				run.WriteRune('▀')
			case top == 0:
				run.WriteRune('▄')
			case top == bot:
				run.WriteRune('█')
			default:
				run.WriteRune('▀') // foreground on top, background below
			}
		}
		flush()
		if row < rows-1 {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

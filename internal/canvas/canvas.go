// Package canvas is a braille dot grid. Every character covers 2x4 dots and
// carries one colour, so shapes that share a character share an ink.
package canvas

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// An Ink is what a dot is painted with. Zero is unpainted; the other values
// are the game's to define and map to styles through a Palette.
type Ink uint8

// A Palette gives each non-zero ink its style.
type Palette map[Ink]lipgloss.Style

// Canvas is W x H dots, both multiples of the character cell (2 wide, 4 tall).
type Canvas struct {
	W, H int
	dots []Ink
	pal  Palette
}

// New makes a blank canvas of w x h dots painted from pal.
func New(w, h int, pal Palette) *Canvas {
	return &Canvas{W: w, H: h, dots: make([]Ink, w*h), pal: pal}
}

// Set paints one dot; dots outside the canvas are dropped.
func (c *Canvas) Set(x, y int, i Ink) {
	if x >= 0 && x < c.W && y >= 0 && y < c.H {
		c.dots[y*c.W+x] = i
	}
}

// Corners says which corners of a Tile are rounded.
type Corners struct{ TL, TR, BL, BR bool }

// Tile paints a w x h rectangle with its top-left dot at (ox,oy), cutting
// each rounded corner back by radius r.
func (c *Canvas) Tile(ox, oy, w, h, r int, round Corners, i Ink) {
	tile(c.Set, ox, oy, w, h, r, round, i)
}

func tile(set func(x, y int, i Ink), ox, oy, w, h, r int, round Corners, i Ink) {
	cut := func(x, y int) bool { return x < r && y < r && (r-1-x)+(r-1-y) >= r-1 }
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			switch {
			case round.TL && cut(x, y),
				round.TR && cut(w-1-x, y),
				round.BL && cut(x, h-1-y),
				round.BR && cut(w-1-x, h-1-y):
				continue
			}
			set(ox+x, oy+y, i)
		}
	}
}

// A Grid is any surface a game can paint dots on: the braille Canvas or the
// half-block Pixels.
type Grid interface {
	Set(x, y int, i Ink)
	Tile(ox, oy, w, h, r int, round Corners, i Ink)
	String() string
}

// BrailleBits maps a dot at (dx,dy) within a character to its bit in the
// braille code point.
var BrailleBits = [2][4]rune{
	{0x01, 0x02, 0x04, 0x40},
	{0x08, 0x10, 0x20, 0x80},
}

// String renders the canvas as styled braille, one terminal row per line.
// A character with dots of more than one ink takes the last one scanned.
func (c *Canvas) String() string {
	var out strings.Builder
	cols, rows := c.W/2, c.H/4
	for row := 0; row < rows; row++ {
		var run strings.Builder
		var runInk Ink
		flush := func() {
			if run.Len() == 0 {
				return
			}
			if runInk == 0 {
				out.WriteString(run.String())
			} else {
				out.WriteString(c.pal[runInk].Render(run.String()))
			}
			run.Reset()
		}
		for col := 0; col < cols; col++ {
			var bits rune
			var i Ink
			for dx := 0; dx < 2; dx++ {
				for dy := 0; dy < 4; dy++ {
					d := c.dots[(row*4+dy)*c.W+col*2+dx]
					if d != 0 {
						bits |= BrailleBits[dx][dy]
						i = d
					}
				}
			}
			if i != runInk {
				flush()
				runInk = i
			}
			if bits == 0 {
				run.WriteByte(' ')
			} else {
				run.WriteRune(0x2800 + bits)
			}
		}
		flush()
		if row < rows-1 {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

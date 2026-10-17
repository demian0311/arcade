package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// An ink is what a dot is painted with: a colour, a flag for virus shading, or white.
type ink uint8

const (
	inkNone  ink = 0
	inkVirus ink = 8
	inkWhite ink = 16
)

func inkFor(c Cell) ink {
	if c.Clearing {
		return inkWhite
	}
	i := ink(c.Color)
	if c.Kind == Virus {
		i |= inkVirus
	}
	return i
}

var inkStyles = map[ink]lipgloss.Style{
	ink(Red):               lipgloss.NewStyle().Foreground(lipgloss.Color("#ff6b6b")),
	ink(Yellow):            lipgloss.NewStyle().Foreground(lipgloss.Color("#ffd75f")),
	ink(Blue):              lipgloss.NewStyle().Foreground(lipgloss.Color("#5fafff")),
	ink(Red) | inkVirus:    lipgloss.NewStyle().Foreground(lipgloss.Color("#c0392b")),
	ink(Yellow) | inkVirus: lipgloss.NewStyle().Foreground(lipgloss.Color("#c9a227")),
	ink(Blue) | inkVirus:   lipgloss.NewStyle().Foreground(lipgloss.Color("#2e7fc1")),
	inkWhite:               lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Bold(true),
}

// canvas is a dot grid; every braille character covers 2x4 dots and every board
// cell covers 4s x 4s dots at scale s, so cell edges fall on character edges.
type canvas struct {
	w, h int
	dots []ink
}

func newCanvas(cellsW, cellsH, s int) *canvas {
	w, h := cellsW*4*s, cellsH*4*s
	return &canvas{w: w, h: h, dots: make([]ink, w*h)}
}

func (c *canvas) set(x, y int, i ink) {
	if x >= 0 && x < c.w && y >= 0 && y < c.h {
		c.dots[y*c.w+x] = i
	}
}

// drawCell paints one board cell at (cx,cy) in board coordinates.
func (c *canvas) drawCell(cx, cy, s int, cell Cell) {
	c.drawAt(cx*4*s, cy*4*s, s, cell)
}

// drawAt paints a cell with its top-left corner at dot (ox,oy).
func (c *canvas) drawAt(ox, oy, s int, cell Cell) {
	if cell.Kind == Empty {
		return
	}
	i := inkFor(cell)
	n := 4 * s
	gap := 0
	if s >= 2 {
		gap = 1
	}
	w, h := n, n
	if cell.Link != Right {
		w -= gap
	}
	if cell.Link != Down {
		h -= gap
	}
	r := max(1, n/4)
	roundTL := cell.Link != Left && cell.Link != Up
	roundTR := cell.Link != Right && cell.Link != Up
	roundBL := cell.Link != Left && cell.Link != Down
	roundBR := cell.Link != Right && cell.Link != Down
	cut := func(x, y int) bool { return x < r && y < r && (r-1-x)+(r-1-y) >= r-1 }
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			switch {
			case roundTL && cut(x, y),
				roundTR && cut(w-1-x, y),
				roundBL && cut(x, h-1-y),
				roundBR && cut(w-1-x, h-1-y):
				continue
			}
			c.set(ox+x, oy+y, i)
		}
	}
	if cell.Kind == Virus {
		c.face(ox, oy, w, h)
	}
}

// face punches eyes and a mouth out of a filled virus body.
func (c *canvas) face(ox, oy, w, h int) {
	m := min(w, h)
	e := max(1, m/5)
	eyeY, mouthY := m/3, m*2/3
	ex1, ex2 := m/4, w-e-m/4
	mt := max(1, m/8)
	for dy := 0; dy < e; dy++ {
		for dx := 0; dx < e; dx++ {
			c.set(ox+ex1+dx, oy+eyeY+dy, inkNone)
			c.set(ox+ex2+dx, oy+eyeY+dy, inkNone)
		}
	}
	for dy := 0; dy < mt; dy++ {
		for x := m / 4; x <= w-1-m/4; x++ {
			c.set(ox+x, oy+mouthY+dy, inkNone)
		}
	}
}

var brailleBits = [2][4]rune{
	{0x01, 0x02, 0x04, 0x40},
	{0x08, 0x10, 0x20, 0x80},
}

// String renders the canvas as styled braille, one terminal row per line.
func (c *canvas) String() string {
	var out strings.Builder
	cols, rows := c.w/2, c.h/4
	for row := 0; row < rows; row++ {
		var run strings.Builder
		var runInk ink
		flush := func() {
			if run.Len() == 0 {
				return
			}
			if runInk == inkNone {
				out.WriteString(run.String())
			} else {
				out.WriteString(inkStyles[runInk].Render(run.String()))
			}
			run.Reset()
		}
		for col := 0; col < cols; col++ {
			var bits rune
			var i ink
			for dx := 0; dx < 2; dx++ {
				for dy := 0; dy < 4; dy++ {
					d := c.dots[(row*4+dy)*c.w+col*2+dx]
					if d != inkNone {
						bits |= brailleBits[dx][dy]
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

func renderBoard(g *Game, s int) string {
	c := newCanvas(BoardW, BoardH, s)
	for y := 0; y < BoardH; y++ {
		for x := 0; x < BoardW; x++ {
			c.drawCell(x, y, s, g.Board[y][x])
		}
	}
	if p := g.Pill; p != nil {
		n := 4 * s
		off := int(g.Prog * float64(n)) // sub-cell fall, in dots
		l0, l1 := p.links()
		x2, y2 := p.Second()
		c.drawAt(p.X*n, p.Y*n+off, s, Cell{Kind: Half, Color: p.C[0], Link: l0})
		c.drawAt(x2*n, y2*n+off, s, Cell{Kind: Half, Color: p.C[1], Link: l1})
	}
	return c.String()
}

func renderNext(p Pill, s int) string {
	c := newCanvas(2, 1, s)
	c.drawCell(0, 0, s, Cell{Kind: Half, Color: p.C[0], Link: Right})
	c.drawCell(1, 0, s, Cell{Kind: Half, Color: p.C[1], Link: Left})
	return c.String()
}

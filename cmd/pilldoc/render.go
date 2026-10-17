package main

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/demian0311/arcade/internal/canvas"
)

// Inks: a colour, a flag for virus shading, or white for a clearing flash.
const (
	inkVirus canvas.Ink = 8
	inkWhite canvas.Ink = 16
)

func inkFor(c Cell) canvas.Ink {
	if c.Clearing {
		return inkWhite
	}
	i := canvas.Ink(c.Color)
	if c.Kind == Virus {
		i |= inkVirus
	}
	return i
}

var hex = map[Color]string{Red: "#ff6b6b", Yellow: "#ffd75f", Blue: "#5fafff"}

var palette = canvas.Palette{
	canvas.Ink(Red):               lipgloss.NewStyle().Foreground(lipgloss.Color(hex[Red])),
	canvas.Ink(Yellow):            lipgloss.NewStyle().Foreground(lipgloss.Color(hex[Yellow])),
	canvas.Ink(Blue):              lipgloss.NewStyle().Foreground(lipgloss.Color(hex[Blue])),
	canvas.Ink(Red) | inkVirus:    lipgloss.NewStyle().Foreground(lipgloss.Color("#c0392b")),
	canvas.Ink(Yellow) | inkVirus: lipgloss.NewStyle().Foreground(lipgloss.Color("#c9a227")),
	canvas.Ink(Blue) | inkVirus:   lipgloss.NewStyle().Foreground(lipgloss.Color("#2e7fc1")),
	inkWhite:                      lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Bold(true),
}

// drawCell paints one board cell at (cx,cy) in board coordinates.
func drawCell(c canvas.Grid, g geom, cx, cy int, cell Cell) {
	drawAt(c, g, cx*g.n, cy*g.n, cell)
}

// drawAt paints a cell with its top-left corner at (ox,oy). The side that
// joins a partner half is square and butts against it; every other side is
// rounded and, when the cell is big enough, leaves a dot of air.
func drawAt(c canvas.Grid, g geom, ox, oy int, cell Cell) {
	if cell.Kind == Empty {
		return
	}
	n, gap := g.n, g.gap()
	w, h := n, n
	if cell.Link != Right {
		w -= gap
	}
	if cell.Link != Down {
		h -= gap
	}
	round := canvas.Corners{
		TL: cell.Link != Left && cell.Link != Up,
		TR: cell.Link != Right && cell.Link != Up,
		BL: cell.Link != Left && cell.Link != Down,
		BR: cell.Link != Right && cell.Link != Down,
	}
	c.Tile(ox, oy, w, h, g.radius(cell.Kind), round, inkFor(cell))
	if cell.Kind == Virus {
		face(c, ox, oy, w, h)
	}
}

// face punches eyes and a mouth out of a filled virus body.
func face(c canvas.Grid, ox, oy, w, h int) {
	m := min(w, h)
	if m < 4 {
		// Too small for a mouth: two eyes on the middle row.
		c.Set(ox, oy+m/2, 0)
		c.Set(ox+w-1, oy+m/2, 0)
		return
	}
	e := max(1, m/5)
	eyeY, mouthY := m/3, m*2/3
	ex1, ex2 := m/4, w-e-m/4
	mt := max(1, m/8)
	for dy := 0; dy < e; dy++ {
		for dx := 0; dx < e; dx++ {
			c.Set(ox+ex1+dx, oy+eyeY+dy, 0)
			c.Set(ox+ex2+dx, oy+eyeY+dy, 0)
		}
	}
	for dy := 0; dy < mt; dy++ {
		for x := m / 4; x <= w-1-m/4; x++ {
			c.Set(ox+x, oy+mouthY+dy, 0)
		}
	}
}

func renderBoard(g *Game, gm geom) string {
	c := gm.grid(BoardW, BoardH)
	for y := 0; y < BoardH; y++ {
		for x := 0; x < BoardW; x++ {
			drawCell(c, gm, x, y, g.Board[y][x])
		}
	}
	if p := g.Pill; p != nil {
		n := gm.n
		off := int(g.Prog * float64(n)) // sub-cell fall
		l0, l1 := p.links()
		x2, y2 := p.Second()
		drawAt(c, gm, p.X*n, p.Y*n+off, Cell{Kind: Half, Color: p.C[0], Link: l0})
		drawAt(c, gm, x2*n, y2*n+off, Cell{Kind: Half, Color: p.C[1], Link: l1})
	}
	return c.String()
}

func renderNext(p Pill, gm geom) string {
	c := gm.grid(2, 1)
	drawCell(c, gm, 0, 0, Cell{Kind: Half, Color: p.C[0], Link: Right})
	drawCell(c, gm, 1, 0, Cell{Kind: Half, Color: p.C[1], Link: Left})
	return c.String()
}

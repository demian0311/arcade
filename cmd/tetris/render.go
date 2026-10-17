package main

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/demian0311/arcade/internal/canvas"
)

// inkWhite is the clearing flash; every other ink is a Color.
const inkWhite canvas.Ink = 16

var hex = map[Color]string{
	Cyan: "#5fd7ff", Yellow: "#ffd75f", Purple: "#c792ea", Green: "#8ae234",
	Red: "#ff6b6b", Blue: "#5fafff", Orange: "#ffaf5f",
}

var palette = func() canvas.Palette {
	p := canvas.Palette{inkWhite: lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Bold(true)}
	for c, h := range hex {
		p[canvas.Ink(c)] = lipgloss.NewStyle().Foreground(lipgloss.Color(h))
	}
	return p
}()

func inkFor(c Cell) canvas.Ink {
	if c.Clearing {
		return inkWhite
	}
	return canvas.Ink(c.Color)
}

// Every board cell covers 4s x 4s dots at scale s, so cell edges fall on character edges.
func newCanvas(cellsW, cellsH, s int) *canvas.Canvas {
	return canvas.New(cellsW*4*s, cellsH*4*s, palette)
}

// drawAt paints a cell with its top-left corner at dot (ox,oy). A side joined
// to the same piece is square and butts against its neighbour; every other
// side is rounded and, from scale 2 up, leaves a dot of air.
func drawAt(c *canvas.Canvas, ox, oy, s int, cell Cell) {
	if cell.Color == None {
		return
	}
	n := 4 * s
	gap := 0
	if s >= 2 {
		gap = 1
	}
	w, h := n, n
	if cell.Links&LinkR == 0 {
		w -= gap
	}
	if cell.Links&LinkD == 0 {
		h -= gap
	}
	l := cell.Links
	round := canvas.Corners{
		TL: l&(LinkL|LinkU) == 0,
		TR: l&(LinkR|LinkU) == 0,
		BL: l&(LinkL|LinkD) == 0,
		BR: l&(LinkR|LinkD) == 0,
	}
	c.Tile(ox, oy, w, h, max(1, n/4), round, inkFor(cell))
}

func renderBoard(g *Game, s int) string {
	c := newCanvas(BoardW, BoardH, s)
	n := 4 * s
	for y := 0; y < BoardH; y++ {
		for x := 0; x < BoardW; x++ {
			drawAt(c, x*n, y*n, s, g.Board[y][x])
		}
	}
	if p := g.Piece; p != nil {
		off := int(g.Prog * float64(n)) // sub-cell fall, in dots
		p.cells(func(x, y int, l Link) {
			drawAt(c, x*n, y*n+off, s, Cell{Color: colorOf[p.Kind], Links: l})
		})
	}
	return c.String()
}

// renderNext draws a kind in its spawn state in a 4x2 cell box.
func renderNext(k Kind, s int) string {
	c := newCanvas(4, 2, s)
	p := Piece{Kind: k}
	p.Y = -p.top()
	n := 4 * s
	p.cells(func(x, y int, l Link) {
		drawAt(c, x*n, y*n, s, Cell{Color: colorOf[k], Links: l})
	})
	return c.String()
}

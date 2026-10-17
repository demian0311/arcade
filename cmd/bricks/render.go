package main

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/demian0311/arcade/internal/canvas"
)

var hex = map[Color]string{
	Red: "#ff6b6b", Orange: "#ffaf5f", Yellow: "#ffd75f", Green: "#8ae234",
	Blue: "#5fafff", Purple: "#c792ea", White: "#ffffff",
}

var palette = func() canvas.Palette {
	p := canvas.Palette{}
	for c, h := range hex {
		p[canvas.Ink(c)] = lipgloss.NewStyle().Foreground(lipgloss.Color(h))
	}
	p[canvas.Ink(White)] = p[canvas.Ink(White)].Bold(true)
	return p
}()

var capsuleColor = [...]Color{Wide: Blue, Slow: Purple}

// Every board cell covers 4s x 4s dots at scale s.
func newCanvas(cellsW, cellsH, s int) *canvas.Canvas {
	return canvas.New(cellsW*4*s, cellsH*4*s, palette)
}

func gapAt(s int) int {
	if s >= 2 {
		return 1
	}
	return 0
}

func renderBoard(g *Game, s int) string {
	c := newCanvas(BoardW, BoardH, s)
	n := 4 * s
	gap := gapAt(s)
	dots := func(v float64) int { return int(v * float64(n)) }

	// Bricks: sharp, with a dot of air between them from scale 2.
	for y := 0; y < BoardH; y++ {
		for x := 0; x < BoardW; x++ {
			if col := g.Bricks[y][x]; col != None {
				c.Tile(x*n, y*n, n-gap, n-gap, 0, canvas.Corners{}, canvas.Ink(col))
			}
		}
	}

	// Capsules: a pill two cells wide.
	all := canvas.Corners{TL: true, TR: true, BL: true, BR: true}
	for _, cp := range g.Capsules {
		c.Tile(dots(cp.X), dots(cp.Y), 2*n-gap, n-gap, max(1, n/4), all, canvas.Ink(capsuleColor[cp.Kind]))
	}

	// Paddle: a rounded bar on the bottom row.
	c.Tile(dots(g.paddleLeft()), (BoardH-1)*n, dots(g.PaddleW), n, max(1, n/4), all, canvas.Ink(White))

	// Ball: a 2s-dot square.
	c.Tile(dots(g.BallX-ballR), dots(g.BallY-ballR), 2*s, 2*s, 0, canvas.Corners{}, canvas.Ink(White))
	return c.String()
}

package main

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/demian0311/arcade/internal/canvas"
)

const (
	inkBody canvas.Ink = 1 + iota
	inkHead
	inkFood
	inkWhite
)

var hex = map[canvas.Ink]string{inkBody: "#5faf5f", inkHead: "#8ae234", inkFood: "#ff6b6b", inkWhite: "#ffffff"}

var palette = func() canvas.Palette {
	p := canvas.Palette{}
	for i, h := range hex {
		p[i] = lipgloss.NewStyle().Foreground(lipgloss.Color(h))
	}
	p[inkWhite] = p[inkWhite].Bold(true)
	return p
}()

// Every board cell covers 4s x 4s dots at scale s, so cell edges fall on character edges.
func newCanvas(cellsW, cellsH, s int) *canvas.Canvas {
	return canvas.New(cellsW*4*s, cellsH*4*s, palette)
}

// links says which sides of a cell join a neighbour, in Dir order.
type links [4]bool

func (l links) corners() canvas.Corners {
	return canvas.Corners{
		TL: !l[Left] && !l[Up],
		TR: !l[Right] && !l[Up],
		BL: !l[Left] && !l[Down],
		BR: !l[Right] && !l[Down],
	}
}

// segment paints one cell of the snake. inset trims the cell on side
// `from` by that many dots, which is how the tail retracts; extend grows it
// past side `to` by that many dots, which is how the head advances.
func segment(c *canvas.Canvas, p Point, s int, l links, i canvas.Ink, from Dir, inset int, to Dir, extend int) {
	n := 4 * s
	ox, oy, w, h := p.X*n, p.Y*n, n, n
	switch from {
	case Left:
		ox, w = ox+inset, w-inset
	case Right:
		w -= inset
	case Up:
		oy, h = oy+inset, h-inset
	case Down:
		h -= inset
	}
	switch to {
	case Left:
		ox, w = ox-extend, w+extend
	case Right:
		w += extend
	case Up:
		oy, h = oy-extend, h+extend
	case Down:
		h += extend
	}
	if w <= 0 || h <= 0 {
		return
	}
	c.Tile(ox, oy, w, h, max(1, n/4), l.corners(), i)
}

// drawHead paints the head as a snout: the head cell plus its advance into
// the next cell, with the leading corners rounded to a half circle and two
// eyes punched out just behind the nose.
func drawHead(c *canvas.Canvas, g *Game, s, off int, i canvas.Ink) {
	n := 4 * s
	p := g.Head()
	ox, oy, w, h := p.X*n, p.Y*n, n, n
	var round canvas.Corners
	switch g.Dir {
	case Right:
		w += off
		round = canvas.Corners{TR: true, BR: true}
	case Left:
		ox, w = ox-off, w+off
		round = canvas.Corners{TL: true, BL: true}
	case Down:
		h += off
		round = canvas.Corners{BL: true, BR: true}
	case Up:
		oy, h = oy-off, h+off
		round = canvas.Corners{TL: true, TR: true}
	}
	c.Tile(ox, oy, w, h, n/2, round, i)

	e := max(1, n/8) // eye size in dots
	back, side := n/2, n/4 // eyes sit in the solid part of the head, not the tapering nose
	var eyes [2]Point
	switch g.Dir {
	case Right:
		eyes = [2]Point{{ox + w - back - e, oy + side}, {ox + w - back - e, oy + h - side - e}}
	case Left:
		eyes = [2]Point{{ox + back, oy + side}, {ox + back, oy + h - side - e}}
	case Down:
		eyes = [2]Point{{ox + side, oy + h - back - e}, {ox + w - side - e, oy + h - back - e}}
	case Up:
		eyes = [2]Point{{ox + side, oy + back}, {ox + w - side - e, oy + back}}
	}
	for _, eye := range eyes {
		for dy := 0; dy < e; dy++ {
			for dx := 0; dx < e; dx++ {
				c.Set(eye.X+dx, eye.Y+dy, 0)
			}
		}
	}
}

func dirTo(a, b Point) Dir {
	switch {
	case b.X > a.X:
		return Right
	case b.X < a.X:
		return Left
	case b.Y > a.Y:
		return Down
	}
	return Up
}

func renderBoard(g *Game, s int) string {
	c := newCanvas(BoardW, BoardH, s)
	n := 4 * s
	off := int(g.Prog * float64(n))
	body, head := inkBody, inkHead
	if g.Dead {
		body, head = inkWhite, inkWhite
	}
	last := len(g.Body) - 1
	for i := last; i >= 0; i-- {
		p := g.Body[i]
		var l links
		if i > 0 {
			l[dirTo(p, g.Body[i-1])] = true
		}
		if i < last {
			l[dirTo(p, g.Body[i+1])] = true
		}
		if i == 0 {
			drawHead(c, g, s, off, head)
			continue
		}
		from, inset := Right, 0
		if i == last && !g.Growing() && !g.Dead {
			from, inset = dirTo(p, g.Body[i-1]).opposite(), off
		}
		segment(c, p, s, l, body, from, inset, Right, 0)
	}
	segment(c, g.Food, s, links{}, inkFood, Right, 0, Right, 0)
	return c.String()
}

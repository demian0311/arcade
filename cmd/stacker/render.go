package main

import (
	"math"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/demian0311/arcade/internal/canvas"
)

// Inks: a Color, or a Color | inkBright for the landing flash's second
// step, or one of the named effect inks.
const (
	inkBright canvas.Ink = 8
	inkWhite  canvas.Ink = 16
	inkGhost  canvas.Ink = 17
	inkEmber  canvas.Ink = 18 // the last step of a burning row
)

var hex = map[Color]string{
	Cyan: "#5fd7ff", Yellow: "#ffd75f", Purple: "#c792ea", Green: "#8ae234",
	Red: "#ff6b6b", Blue: "#5fafff", Orange: "#ffaf5f",
}

var brightHex = map[Color]string{
	Cyan: "#afffff", Yellow: "#ffffaf", Purple: "#e6c8ff", Green: "#c8ff87",
	Red: "#ffafaf", Blue: "#afd7ff", Orange: "#ffd7af",
}

var palette = func() canvas.Palette {
	p := canvas.Palette{
		inkWhite: lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Bold(true),
		inkGhost: lipgloss.NewStyle().Foreground(lipgloss.Color("#6c6c6c")),
		inkEmber: lipgloss.NewStyle().Foreground(lipgloss.Color("#875f00")),
	}
	for c, h := range hex {
		p[canvas.Ink(c)] = lipgloss.NewStyle().Foreground(lipgloss.Color(h))
	}
	for c, h := range brightHex {
		p[canvas.Ink(c)|inkBright] = lipgloss.NewStyle().Foreground(lipgloss.Color(h))
	}
	return p
}()

// Effect timings.
const (
	landFlash  = 300 * time.Millisecond // a landed piece goes white, then bright, then settles
	landWhite  = 90 * time.Millisecond
	slamTrail  = 160 * time.Millisecond // the band a slam crossed fades
	clearDelay = 260 * time.Millisecond // a full row shatters outward from the landed piece
	sparksPer  = 3                      // sparks thrown by each shattered cell
)

// effects is what the renderer animates on top of the game state. Ages are
// measured from the moments the model records.
type effects struct {
	landed   []Point   // cells of the piece that locked most recently
	landedAt time.Time // zero when nothing has landed yet
	slamX0   int       // columns and rows the last slam crossed, inclusive
	slamX1   int
	slamY0   int
	slamY1   int
	slamAt   time.Time
	clearAt  time.Time // when the current clear flash began
	clearCX  float64   // column the shatter spreads from: the landed piece's centre
}

// popAt is the moment, in 0..1 of clearDelay, when a clearing cell at
// column x bursts: the wave starts at cx and reaches the far wall by 0.6.
func popAt(x int, cx float64) float64 {
	far := max(cx, float64(BoardW-1)-cx)
	return 0.05 + 0.55*math.Abs(float64(x)-cx)/max(far, 1)
}

// noise is a cheap deterministic hash in 0..1, so sparks need no storage.
func noise(a, b, c int) float64 {
	h := uint32(a*73856093) ^ uint32(b*19349663) ^ uint32(c*83492791)
	h ^= h >> 13
	h *= 0x5bd1e995
	h ^= h >> 15
	return float64(h&0xffff) / 0xffff
}

// sparks draws the debris of a cell that burst at time t0 (seconds) as seen
// at time t: a few dots thrown up and out, falling back under gravity.
func sparks(c *canvas.Canvas, x, y, n int, color Color, t0, t float64) {
	if t <= t0 {
		return
	}
	age := t - t0
	ox, oy := float64(x*n)+float64(n)/2, float64(y*n)+float64(n)/2
	for i := 0; i < sparksPer; i++ {
		vx := (noise(x, y, i*3) - 0.5) * 14 * float64(n)    // dots per second, either way
		vy := -(0.6 + noise(x, y, i*3+1)) * 10 * float64(n) // upwards
		px := ox + vx*age
		py := oy + vy*age + 0.5*40*float64(n)*age*age
		ink := canvas.Ink(color) | inkBright
		if age < 0.08 {
			ink = inkWhite
		}
		for dy := 0; dy < max(1, n/8); dy++ {
			for dx := 0; dx < max(1, n/8); dx++ {
				c.Set(int(px)+dx, int(py)+dy, ink)
			}
		}
	}
}

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

// drawAt paints a cell with its top-left corner at dot (ox,oy). Corners are
// sharp. A side joined to the same piece butts against its neighbour, so a
// piece is one solid shape; every other side leaves a dot of air from scale
// 2 up, so neighbouring pieces stay distinct.
func drawAt(c *canvas.Canvas, ox, oy, s int, cell Cell) {
	drawTileInk(c, ox, oy, s, cell, inkFor(cell))
}

// drawTileInk is drawAt in a chosen ink.
func drawTileInk(c *canvas.Canvas, ox, oy, s int, cell Cell, i canvas.Ink) {
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
	c.Tile(ox, oy, w, h, 0, canvas.Corners{}, i)
}

// drawOutline paints a cell as a one-dot ring in ink i, with no wall on a
// side joined to the same piece, so a piece outlines as one shape.
func drawOutline(c *canvas.Canvas, ox, oy, s int, cell Cell, i canvas.Ink) {
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
	c.Tile(ox, oy, w, h, 0, canvas.Corners{}, i)
	x0, x1, y0, y1 := 1, w-1, 1, h-1
	if l&LinkL != 0 {
		x0 = 0
	}
	if l&LinkR != 0 {
		x1 = w
	}
	if l&LinkU != 0 {
		y0 = 0
	}
	if l&LinkD != 0 {
		y1 = h
	}
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			c.Set(ox+x, oy+y, 0)
		}
	}
}

// completing says which rows would be full if p locked where it is.
func (g *Game) completing(p Piece) (full [BoardH]bool) {
	var board Board = g.Board
	p.cells(func(x, y int, _ Link) {
		if y >= 0 && y < BoardH {
			board[y][x].Color = colorOf[p.Kind]
		}
	})
	for y := range board {
		full[y] = true
		for x := range board[y] {
			if board[y][x].Color == None {
				full[y] = false
				break
			}
		}
	}
	return full
}

// landing is where the falling piece would come to rest.
func (g *Game) landing() Piece {
	p := *g.Piece
	for {
		q := p
		q.Y++
		if !g.fits(q) {
			return p
		}
		p = q
	}
}

func renderBoard(g *Game, s int, fx effects, now time.Time) string {
	c := newCanvas(BoardW, BoardH, s)
	n := 4 * s

	// The slam trail: a grey band over the columns the piece crossed, its
	// top sinking towards the landing spot as it fades.
	if age := now.Sub(fx.slamAt); !fx.slamAt.IsZero() && age < slamTrail && fx.slamY1 > fx.slamY0 {
		t := float64(age) / float64(slamTrail)
		top := fx.slamY0*n + int(t*float64((fx.slamY1-fx.slamY0)*n))
		for y := top; y < fx.slamY1*n; y++ { // checkerboard, so the band reads as haze
			for x := fx.slamX0 * n; x < (fx.slamX1+1)*n; x++ {
				if (x+y)%2 == 0 {
					c.Set(x, y, inkGhost)
				}
			}
		}
	}

	// Cells of the last landed piece flash white, then bright, then settle.
	var landInk canvas.Ink
	if age := now.Sub(fx.landedAt); !fx.landedAt.IsZero() && age < landFlash {
		landInk = inkWhite
		if age >= landWhite {
			landInk = inkBright
		}
	}
	landed := map[Point]bool{}
	if landInk != 0 {
		for _, p := range fx.landed {
			landed[p] = true
		}
	}

	// A clearing row burns through white, yellow, orange and ember while its
	// cells vanish from the centre outwards.
	clearT := 1.0
	if !fx.clearAt.IsZero() {
		clearT = min(1, float64(now.Sub(fx.clearAt))/float64(clearDelay))
	}

	// Rows the piece would complete where it is heading glow in their bright tints.
	var ghost Piece
	var would [BoardH]bool
	if g.Piece != nil {
		ghost = g.landing()
		would = g.completing(ghost)
	}

	for y := 0; y < BoardH; y++ {
		for x := 0; x < BoardW; x++ {
			cell := g.Board[y][x]
			if cell.Color == None {
				continue
			}
			ink := inkFor(cell)
			switch {
			case cell.Clearing:
				pop := popAt(x, fx.clearCX)
				if clearT >= pop {
					sparks(c, x, y, n, cell.Color, pop*clearDelay.Seconds(), clearT*clearDelay.Seconds())
					continue
				}
				ink = canvas.Ink(cell.Color) | inkBright
				if clearT >= pop-0.08 {
					ink = inkWhite // the instant before it bursts
				}
			case landed[Point{x, y}]:
				ink = landInk
				if landInk == inkBright {
					ink = canvas.Ink(cell.Color) | inkBright
				}
			case would[y]:
				ink = canvas.Ink(cell.Color) | inkBright
			}
			drawTileInk(c, x*n, y*n, s, cell, ink)
		}
	}
	if p := g.Piece; p != nil {
		if ghost.Y > p.Y {
			ghost.cells(func(x, y int, l Link) {
				ink := inkGhost
				if y >= 0 && y < BoardH && would[y] {
					ink = inkWhite // the ghost's cells in a row it would complete
				}
				drawOutline(c, x*n, y*n, s, Cell{Color: colorOf[p.Kind], Links: l}, ink)
			})
		}
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

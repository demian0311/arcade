package main

import (
	"math/rand"
	"time"
)

const (
	BoardW     = 8
	BoardH     = 16
	MaxViruses = 84
	MaxLevel   = 20
)

type Color uint8

const (
	None Color = iota
	Red
	Yellow
	Blue
)

var colors = [3]Color{Red, Yellow, Blue}

type Kind uint8

const (
	Empty Kind = iota
	Virus
	Half // one half of a pill
)

// Dir points from a pill half to its partner.
type Dir uint8

const (
	NoDir Dir = iota
	Left
	Right
	Up
	Down
)

func (d Dir) Delta() (int, int) {
	switch d {
	case Left:
		return -1, 0
	case Right:
		return 1, 0
	case Up:
		return 0, -1
	case Down:
		return 0, 1
	}
	return 0, 0
}

type Cell struct {
	Kind     Kind
	Color    Color
	Link     Dir
	Clearing bool
}

type Board [BoardH][BoardW]Cell

func inBounds(x, y int) bool { return x >= 0 && x < BoardW && y >= 0 && y < BoardH }

// free reports whether a pill half may occupy (x,y). Rows above the bottle are free.
func (b *Board) free(x, y int) bool {
	if x < 0 || x >= BoardW || y >= BoardH {
		return false
	}
	if y < 0 {
		return true
	}
	return b[y][x].Kind == Empty
}

type Speed int

const (
	Low Speed = iota
	Med
	Hi
)

func (s Speed) String() string { return [...]string{"LOW", "MED", "HI"}[s] }

// Pill is the falling capsule. C[0] sits at (X,Y); C[1] sits to the right, or above when vertical.
type Pill struct {
	X, Y     int
	Vertical bool
	C        [2]Color
}

func (p Pill) Second() (int, int) {
	if p.Vertical {
		return p.X, p.Y - 1
	}
	return p.X + 1, p.Y
}

func (p Pill) links() (Dir, Dir) {
	if p.Vertical {
		return Up, Down
	}
	return Right, Left
}

type Game struct {
	Board   Board
	Level   int
	Speed   Speed
	Score   int
	Viruses int
	Pill    *Pill
	Next    Pill
	Combo   int // viruses cleared by the current pill, drives score doubling
	rng     *rand.Rand

	Prog       float64       // how far the pill has fallen into the next row, 0 to 1
	Rest       time.Duration // how long the pill has sat on something
	Soft       bool          // soft drop, set by the down key, cleared when the pill lands
	Held       bool          // the down key is a confirmed hold, so soft drop runs at full rate
	lockResets int           // moves and turns that have restarted the lock delay for this pill
}

const (
	// softInterval is the row period of a confirmed held soft drop. Until the
	// hold is confirmed a press falls at gravity/tapSoftDivisor, so a tap moves
	// a row or two instead of the whole bottle.
	softInterval   = 45 * time.Millisecond
	tapSoftDivisor = 3
	// lockDelay is how long a pill may sit on something before it locks when it
	// landed under gravity; a move or turn restarts it, up to maxLockResets times
	// per pill. A pill driven down by soft drop locks the moment it lands.
	lockDelay     = 350 * time.Millisecond
	maxLockResets = 8
)

func NewGame(level int, speed Speed, seed int64) *Game {
	level = max(0, min(level, MaxLevel))
	g := &Game{Level: level, Speed: speed, rng: rand.New(rand.NewSource(seed))}
	g.placeViruses()
	g.Next = g.randomPill()
	return g
}

func virusCount(level int) int { return min((level+1)*4, MaxViruses) }

// virusTopRow is the highest row viruses may occupy, as in the NES original.
func virusTopRow(level int) int {
	switch {
	case level >= 19:
		return BoardH - 13
	case level >= 17:
		return BoardH - 12
	case level >= 15:
		return BoardH - 11
	}
	return BoardH - 10
}

func (g *Game) placeViruses() {
	count := virusCount(g.Level)
	top := virusTopRow(g.Level)
	for i := 0; i < count; i++ {
		c := colors[i%3]
		for try := 0; try < 2000; try++ {
			x := g.rng.Intn(BoardW)
			y := top + g.rng.Intn(BoardH-top)
			if g.Board[y][x].Kind != Empty {
				continue
			}
			if g.runLength(x, y, c, 1, 0) >= 3 || g.runLength(x, y, c, 0, 1) >= 3 {
				continue
			}
			g.Board[y][x] = Cell{Kind: Virus, Color: c}
			g.Viruses++
			break
		}
	}
}

// runLength is the length of the run of colour c through (x,y) along (dx,dy), if (x,y) held c.
func (g *Game) runLength(x, y int, c Color, dx, dy int) int {
	n := 1
	for _, sign := range [2]int{1, -1} {
		for i := 1; ; i++ {
			cx, cy := x+dx*i*sign, y+dy*i*sign
			if !inBounds(cx, cy) || g.Board[cy][cx].Kind == Empty || g.Board[cy][cx].Color != c {
				break
			}
			n++
		}
	}
	return n
}

func (g *Game) randomPill() Pill {
	return Pill{C: [2]Color{colors[g.rng.Intn(3)], colors[g.rng.Intn(3)]}}
}

// Spawn puts the next pill at the bottle neck. It returns false when the neck is blocked.
func (g *Game) Spawn() bool {
	p := g.Next
	p.X, p.Y, p.Vertical = BoardW/2-1, 0, false
	g.Next = g.randomPill()
	g.Combo = 0
	g.Pill = nil
	if !g.fits(p) {
		return false
	}
	g.Pill = &p
	g.Prog, g.Rest, g.Soft, g.lockResets = 0, 0, false, 0
	return true
}

// Advance moves time forward for the falling pill. It returns true when the pill has landed.
func (g *Game) Advance(dt time.Duration) bool {
	if g.Pill == nil {
		return false
	}
	iv := g.Interval()
	if g.Soft {
		iv = max(softInterval, iv/tapSoftDivisor)
		if g.Held {
			iv = softInterval
		}
	}
	if !g.canFall() {
		g.Prog = 0
		if g.Soft {
			return true
		}
		g.Rest += dt
		return g.Rest >= lockDelay
	}
	g.Rest = 0
	g.Prog += float64(dt) / float64(iv)
	for g.Prog >= 1 {
		if !g.Fall() {
			g.Prog = 0
			break
		}
		g.Prog--
	}
	if !g.canFall() {
		g.Prog = 0
	}
	return false
}

func (g *Game) canFall() bool {
	p := *g.Pill
	p.Y++
	return g.fits(p)
}

func (g *Game) fits(p Pill) bool {
	x2, y2 := p.Second()
	return g.Board.free(p.X, p.Y) && g.Board.free(x2, y2)
}

// Move slides the pill sideways. A pill drawn more than half way into the next
// row is treated as being there, so it can tuck under an overhang it is visually
// already below; failing that it moves in its logical row and snaps flush.
func (g *Game) Move(dx int) bool {
	if g.Pill == nil {
		return false
	}
	rows := []int{0}
	if g.Prog >= 0.5 {
		rows = []int{1, 0}
	}
	for _, dy := range rows {
		p := *g.Pill
		p.X += dx
		p.Y += dy
		if !g.fits(p) {
			continue
		}
		*g.Pill = p
		if dy == 1 {
			g.Prog = 0
		}
		g.settle()
		return true
	}
	return false
}

// settle snaps a pill that now hangs over a block flush with the row it is in,
// and restarts the lock delay so the player can keep adjusting it.
func (g *Game) settle() {
	if !g.canFall() {
		g.Prog = 0
	}
	if g.lockResets < maxLockResets {
		g.Rest = 0
		g.lockResets++
	}
}

// Fall moves the pill down one row. It returns false when the pill rests on something.
func (g *Game) Fall() bool {
	if g.Pill == nil {
		return false
	}
	p := *g.Pill
	p.Y++
	if !g.fits(p) {
		return false
	}
	*g.Pill = p
	return true
}

// Rotate turns the pill a quarter turn, with a one-column kick off the right wall.
func (g *Game) Rotate(cw bool) bool {
	if g.Pill == nil {
		return false
	}
	p := *g.Pill
	if p.Vertical {
		p.Vertical = false
		if !cw {
			p.C[0], p.C[1] = p.C[1], p.C[0]
		}
	} else {
		p.Vertical = true
		if cw {
			p.C[0], p.C[1] = p.C[1], p.C[0]
		}
	}
	if g.fits(p) {
		*g.Pill = p
		g.settle()
		return true
	}
	if !p.Vertical {
		p.X--
		if g.fits(p) {
			*g.Pill = p
			g.settle()
			return true
		}
	}
	return false
}

// Lock writes the falling pill into the board.
func (g *Game) Lock() {
	p := g.Pill
	if p == nil {
		return
	}
	l0, l1 := p.links()
	x2, y2 := p.Second()
	if !inBounds(x2, y2) {
		l0 = NoDir
	}
	g.set(p.X, p.Y, Cell{Kind: Half, Color: p.C[0], Link: l0})
	g.set(x2, y2, Cell{Kind: Half, Color: p.C[1], Link: l1})
	g.Pill = nil
	g.Prog, g.Rest, g.Soft, g.Held = 0, 0, false, false
}

func (g *Game) set(x, y int, c Cell) {
	if inBounds(x, y) {
		g.Board[y][x] = c
	}
}

// MarkMatches flags every run of four or more same-coloured cells. It returns the number flagged.
func (g *Game) MarkMatches() int {
	marked := 0
	mark := func(x, y, dx, dy, n int) {
		for i := 0; i < n; i++ {
			c := &g.Board[y+dy*i][x+dx*i]
			if !c.Clearing {
				c.Clearing = true
				marked++
			}
		}
	}
	for y := 0; y < BoardH; y++ {
		for x := 0; x < BoardW; {
			c := g.Board[y][x]
			n := 1
			if c.Kind != Empty {
				for x+n < BoardW && g.Board[y][x+n].Kind != Empty && g.Board[y][x+n].Color == c.Color {
					n++
				}
				if n >= 4 {
					mark(x, y, 1, 0, n)
				}
			}
			x += n
		}
	}
	for x := 0; x < BoardW; x++ {
		for y := 0; y < BoardH; {
			c := g.Board[y][x]
			n := 1
			if c.Kind != Empty {
				for y+n < BoardH && g.Board[y+n][x].Kind != Empty && g.Board[y+n][x].Color == c.Color {
					n++
				}
				if n >= 4 {
					mark(x, y, 0, 1, n)
				}
			}
			y += n
		}
	}
	return marked
}

// clearingColor is the colour of the first flagged cell, for the backdrop flare.
func (g *Game) clearingColor() Color {
	for y := range g.Board {
		for x := range g.Board[y] {
			if g.Board[y][x].Clearing {
				return g.Board[y][x].Color
			}
		}
	}
	return None
}

// Clear removes flagged cells, orphans their partners and scores the viruses.
func (g *Game) Clear() int {
	viruses := 0
	for y := range g.Board {
		for x := range g.Board[y] {
			c := g.Board[y][x]
			if !c.Clearing {
				continue
			}
			if c.Kind == Virus {
				viruses++
			}
			if dx, dy := c.Link.Delta(); c.Link != NoDir && inBounds(x+dx, y+dy) {
				g.Board[y+dy][x+dx].Link = NoDir
			}
			g.Board[y][x] = Cell{}
		}
	}
	g.Viruses -= viruses
	base := [...]int{100, 200, 300}[g.Speed]
	for i := 0; i < viruses; i++ {
		g.Score += base << min(g.Combo, 6)
		g.Combo++
	}
	return viruses
}

// DropStep lets every unsupported pill half or pair fall one row. Viruses never move.
func (g *Game) DropStep() bool {
	moved := false
	for y := BoardH - 2; y >= 0; y-- {
		for x := 0; x < BoardW; x++ {
			c := g.Board[y][x]
			if c.Kind != Half {
				continue
			}
			switch c.Link {
			case NoDir:
				if g.Board[y+1][x].Kind == Empty {
					g.Board[y+1][x], g.Board[y][x] = c, Cell{}
					moved = true
				}
			case Right:
				if x+1 < BoardW && g.Board[y+1][x].Kind == Empty && g.Board[y+1][x+1].Kind == Empty {
					g.Board[y+1][x], g.Board[y+1][x+1] = c, g.Board[y][x+1]
					g.Board[y][x], g.Board[y][x+1] = Cell{}, Cell{}
					moved = true
				}
			case Up:
				if y-1 >= 0 && g.Board[y+1][x].Kind == Empty {
					g.Board[y+1][x], g.Board[y][x], g.Board[y-1][x] = c, g.Board[y-1][x], Cell{}
					moved = true
				}
			}
		}
	}
	return moved
}

// Interval is the gravity period for the current speed and level.
func (g *Game) Interval() time.Duration {
	base := [...]int{900, 600, 350}[g.Speed]
	return time.Duration(max(base-g.Level*25, 120)) * time.Millisecond
}

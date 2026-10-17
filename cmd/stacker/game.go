package main

import (
	"math/rand"
	"time"
)

const (
	BoardW   = 10
	BoardH   = 20
	MaxLevel = 19
)

type Color uint8

const (
	None Color = iota
	Cyan
	Yellow
	Purple
	Green
	Red
	Blue
	Orange
)

// A Link bit says a cell is joined to the neighbour on that side, so the
// two are drawn as one piece.
type Link uint8

const (
	LinkL Link = 1 << iota
	LinkR
	LinkU
	LinkD
)

type Cell struct {
	Color    Color // None is empty
	Links    Link
	Clearing bool
}

type Board [BoardH][BoardW]Cell

type Kind uint8

const (
	I Kind = iota
	O
	T
	S
	Z
	J
	L
	kinds
)

var colorOf = [kinds]Color{Cyan, Yellow, Purple, Green, Red, Blue, Orange}

// shapes holds the four rotation states of each kind, in a box up to 4x4.
var shapes = [kinds][4][]string{
	I: {{"....", "####", "....", "...."}, {"..#.", "..#.", "..#.", "..#."}, {"....", "....", "####", "...."}, {".#..", ".#..", ".#..", ".#.."}},
	O: {{".##.", ".##."}, {".##.", ".##."}, {".##.", ".##."}, {".##.", ".##."}},
	T: {{".#.", "###", "..."}, {".#.", ".##", ".#."}, {"...", "###", ".#."}, {".#.", "##.", ".#."}},
	S: {{".##", "##.", "..."}, {".#.", ".##", "..#"}, {"...", ".##", "##."}, {"#..", "##.", ".#."}},
	Z: {{"##.", ".##", "..."}, {"..#", ".##", ".#."}, {"...", "##.", ".##"}, {".#.", "##.", "#.."}},
	J: {{"#..", "###", "..."}, {".##", ".#.", ".#."}, {"...", "###", "..#"}, {".#.", ".#.", "##."}},
	L: {{"..#", "###", "..."}, {".#.", ".#.", ".##"}, {"...", "###", "#.."}, {"##.", ".#.", ".#."}},
}

// kicks are the SRS wall kicks tried in order when a rotation does not fit,
// indexed by the state rotated from, then 0 for clockwise and 1 for
// counter-clockwise. Offsets are (dx, dy) with y up, as the tables are published.
var kicksJLSTZ = [4][2][5][2]int{
	{{{0, 0}, {-1, 0}, {-1, 1}, {0, -2}, {-1, -2}}, {{0, 0}, {1, 0}, {1, 1}, {0, -2}, {1, -2}}},
	{{{0, 0}, {1, 0}, {1, -1}, {0, 2}, {1, 2}}, {{0, 0}, {1, 0}, {1, -1}, {0, 2}, {1, 2}}},
	{{{0, 0}, {1, 0}, {1, 1}, {0, -2}, {1, -2}}, {{0, 0}, {-1, 0}, {-1, 1}, {0, -2}, {-1, -2}}},
	{{{0, 0}, {-1, 0}, {-1, -1}, {0, 2}, {-1, 2}}, {{0, 0}, {-1, 0}, {-1, -1}, {0, 2}, {-1, 2}}},
}

var kicksI = [4][2][5][2]int{
	{{{0, 0}, {-2, 0}, {1, 0}, {-2, -1}, {1, 2}}, {{0, 0}, {-1, 0}, {2, 0}, {-1, 2}, {2, -1}}},
	{{{0, 0}, {-1, 0}, {2, 0}, {-1, 2}, {2, -1}}, {{0, 0}, {2, 0}, {-1, 0}, {2, 1}, {-1, -2}}},
	{{{0, 0}, {2, 0}, {-1, 0}, {2, 1}, {-1, -2}}, {{0, 0}, {1, 0}, {-2, 0}, {1, -2}, {-2, 1}}},
	{{{0, 0}, {1, 0}, {-2, 0}, {1, -2}, {-2, 1}}, {{0, 0}, {-2, 0}, {1, 0}, {-2, -1}, {1, 2}}},
}

// A Piece is a kind in a rotation state with its box's top-left at (X,Y).
type Piece struct {
	Kind Kind
	Rot  int
	X, Y int
}

func (p Piece) shape() []string { return shapes[p.Kind][p.Rot] }

// cells calls f for every filled cell of the piece in board coordinates,
// with the links to its neighbours in the same piece.
func (p Piece) cells(f func(x, y int, l Link)) {
	sh := p.shape()
	at := func(x, y int) bool { return y >= 0 && y < len(sh) && x >= 0 && x < len(sh[y]) && sh[y][x] == '#' }
	for y := range sh {
		for x := range sh[y] {
			if !at(x, y) {
				continue
			}
			var l Link
			if at(x-1, y) {
				l |= LinkL
			}
			if at(x+1, y) {
				l |= LinkR
			}
			if at(x, y-1) {
				l |= LinkU
			}
			if at(x, y+1) {
				l |= LinkD
			}
			f(p.X+x, p.Y+y, l)
		}
	}
}

// top is the first row of the box with a filled cell.
func (p Piece) top() int {
	for y, row := range p.shape() {
		for _, c := range row {
			if c == '#' {
				return y
			}
		}
	}
	return 0
}

type Game struct {
	Board Board
	Piece *Piece
	Next  Kind
	Level int
	Lines int
	Score int
	Last  Color // colour of the piece that locked most recently

	Prog       float64       // how far the piece has fallen into the next row, 0 to 1
	Rest       time.Duration // how long the piece has sat on something
	Soft       bool          // soft drop, set by the down key, cleared when the piece lands
	Held       bool          // the down key is a confirmed hold, so soft drop runs at full rate
	lockResets int

	start int
	rng   *rand.Rand
	bag   []Kind
}

const (
	softInterval   = 25 * time.Millisecond
	tapSoftDivisor = 6
	lockDelay      = 350 * time.Millisecond
	softLockDelay  = 150 * time.Millisecond
	maxLockResets  = 8
)

func NewGame(level int, seed int64) *Game {
	g := &Game{Level: level, start: level, rng: rand.New(rand.NewSource(seed))}
	g.Next = g.draw()
	return g
}

// draw takes the next kind from a seven-bag, so no kind waits more than twelve pieces.
func (g *Game) draw() Kind {
	if len(g.bag) == 0 {
		for k := Kind(0); k < kinds; k++ {
			g.bag = append(g.bag, k)
		}
		g.rng.Shuffle(len(g.bag), func(i, j int) { g.bag[i], g.bag[j] = g.bag[j], g.bag[i] })
	}
	k := g.bag[0]
	g.bag = g.bag[1:]
	return k
}

// Spawn puts the next piece at the top, centred. It returns false when it does not fit.
func (g *Game) Spawn() bool {
	p := Piece{Kind: g.Next}
	p.X = (BoardW - len(p.shape()[0])) / 2
	p.Y = -p.top()
	g.Next = g.draw()
	g.Piece = &p
	g.Prog, g.Rest, g.Soft, g.Held, g.lockResets = 0, 0, false, false, 0
	return g.fits(p)
}

func (g *Game) fits(p Piece) bool {
	ok := true
	p.cells(func(x, y int, _ Link) {
		if x < 0 || x >= BoardW || y >= BoardH || (y >= 0 && g.Board[y][x].Color != None) {
			ok = false
		}
	})
	return ok
}

func (g *Game) canFall() bool {
	p := *g.Piece
	p.Y++
	return g.fits(p)
}

// Advance moves time forward for the falling piece. It returns true when the piece has landed.
func (g *Game) Advance(dt time.Duration) bool {
	if g.Piece == nil {
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
		g.Rest += dt
		if g.Soft {
			return g.Rest >= softLockDelay
		}
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

// Fall moves the piece down a row; it returns false when it is resting.
func (g *Game) Fall() bool {
	if g.Piece == nil || !g.canFall() {
		return false
	}
	g.Piece.Y++
	return true
}

// settle restarts the lock delay after a move or turn, a limited number of times per piece.
func (g *Game) settle() {
	if g.canFall() {
		g.Prog = 0
		return
	}
	if g.lockResets < maxLockResets {
		g.Rest = 0
		g.lockResets++
	}
}

func (g *Game) Move(dx int) bool {
	if g.Piece == nil {
		return false
	}
	p := *g.Piece
	p.X += dx
	if !g.fits(p) {
		return false
	}
	*g.Piece = p
	g.settle()
	return true
}

// Rotate turns the piece, trying the SRS kicks in order.
func (g *Game) Rotate(cw bool) bool {
	if g.Piece == nil {
		return false
	}
	from := g.Piece.Rot
	dir := 0 // index 0 is clockwise, 1 counter-clockwise
	if !cw {
		dir = 1
	}
	table := kicksJLSTZ
	if g.Piece.Kind == I {
		table = kicksI
	}
	for _, k := range table[from][dir] {
		p := *g.Piece
		p.Rot = (from + 1 + 2*dir) % 4 // +1 clockwise, +3 counter-clockwise
		p.X += k[0]
		p.Y -= k[1]
		if g.fits(p) {
			*g.Piece = p
			g.settle()
			return true
		}
	}
	return false
}

// Lock writes the piece into the board. It returns false when any cell is
// above the board, which ends the game.
func (g *Game) Lock() bool {
	p := g.Piece
	if p == nil {
		return true
	}
	inside := true
	p.cells(func(x, y int, l Link) {
		if y < 0 {
			inside = false
			return
		}
		g.Board[y][x] = Cell{Color: colorOf[p.Kind], Links: l}
	})
	g.Last = colorOf[p.Kind]
	g.Piece = nil
	g.Prog, g.Rest, g.Soft, g.Held = 0, 0, false, false
	return inside
}

// MarkFull flags every full row for the clear flash and returns how many there are.
func (g *Game) MarkFull() int {
	n := 0
	for y := range g.Board {
		full := true
		for x := range g.Board[y] {
			if g.Board[y][x].Color == None {
				full = false
				break
			}
		}
		if full {
			n++
			for x := range g.Board[y] {
				g.Board[y][x].Clearing = true
			}
		}
	}
	return n
}

var lineScore = [...]int{0, 40, 100, 300, 1200}

// Clear removes the flagged rows, drops everything above them, and scores.
func (g *Game) Clear() int {
	// Links into a removed row are cut first, so pieces split cleanly.
	for y := range g.Board {
		if !g.Board[y][0].Clearing {
			continue
		}
		for x := range g.Board[y] {
			if y > 0 {
				g.Board[y-1][x].Links &^= LinkD
			}
			if y < BoardH-1 {
				g.Board[y+1][x].Links &^= LinkU
			}
		}
	}
	var kept Board
	w := BoardH - 1
	n := 0
	for y := BoardH - 1; y >= 0; y-- {
		if g.Board[y][0].Clearing {
			n++
			continue
		}
		kept[w] = g.Board[y]
		w--
	}
	g.Board = kept
	g.Lines += n
	g.Score += lineScore[min(n, 4)] * (g.Level + 1)
	g.Level = min(MaxLevel, g.start+g.Lines/10)
	return n
}

// Interval is the gravity period for the current level.
func (g *Game) Interval() time.Duration {
	return time.Duration(max(800-g.Level*55, 80)) * time.Millisecond
}

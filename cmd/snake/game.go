package main

import (
	"math/rand"
	"time"
)

const (
	BoardW = 16
	BoardH = 13
	// startLen is the snake's length at the start of a round.
	startLen = 3
)

type Speed int

const (
	Low Speed = iota
	Med
	Hi
)

func (s Speed) String() string { return [...]string{"LOW", "MED", "HI"}[s] }

// A Dir is a heading; Delta is its unit step.
type Dir uint8

const (
	Right Dir = iota
	Down
	Left
	Up
)

var deltas = [...][2]int{{1, 0}, {0, 1}, {-1, 0}, {0, -1}}

func (d Dir) Delta() (int, int) { return deltas[d][0], deltas[d][1] }

func (d Dir) opposite() Dir { return (d + 2) % 4 }

type Point struct{ X, Y int }

// Game is one round. Body runs head first; Prog is how far the head has
// moved into the next cell, so the snake can be drawn between cells.
type Game struct {
	Body    []Point
	Dir     Dir   // the heading the head is moving in now
	turns   []Dir // headings queued by the player, applied one per step
	Food    Point
	Speed   Speed
	Score   int
	Eaten   int
	Dead    bool
	Prog    float64
	growing int // steps the tail stays put after a meal
	rng     *rand.Rand
}

// food is the score for a meal at each speed.
var foodScore = [...]int{10, 20, 30}

// A meal grows the snake by growBy cells.
const growBy = 2

func NewGame(speed Speed, seed int64) *Game {
	g := &Game{Speed: speed, Dir: Right, rng: rand.New(rand.NewSource(seed))}
	x, y := BoardW/2, BoardH/2
	for i := 0; i < startLen; i++ {
		g.Body = append(g.Body, Point{x - i, y})
	}
	g.placeFood()
	return g
}

func (g *Game) Head() Point { return g.Body[0] }

// Next is the cell the head is moving into.
func (g *Game) Next() Point {
	dx, dy := g.Dir.Delta()
	return Point{g.Head().X + dx, g.Head().Y + dy}
}

func inBounds(p Point) bool { return p.X >= 0 && p.X < BoardW && p.Y >= 0 && p.Y < BoardH }

func (g *Game) onBody(p Point) bool {
	for _, b := range g.Body {
		if b == p {
			return true
		}
	}
	return false
}

func (g *Game) placeFood() {
	free := make([]Point, 0, BoardW*BoardH-len(g.Body))
	for y := 0; y < BoardH; y++ {
		for x := 0; x < BoardW; x++ {
			if p := (Point{x, y}); !g.onBody(p) {
				free = append(free, p)
			}
		}
	}
	if len(free) == 0 {
		return
	}
	g.Food = free[g.rng.Intn(len(free))]
}

// Turn queues a heading. A reversal or a repeat of the last queued heading is ignored.
func (g *Game) Turn(d Dir) {
	last := g.Dir
	if n := len(g.turns); n > 0 {
		last = g.turns[n-1]
	}
	if d == last || d == last.opposite() || len(g.turns) >= 2 {
		return
	}
	g.turns = append(g.turns, d)
}

// Interval is the time per cell at the current speed, quickening with each meal.
func (g *Game) Interval() time.Duration {
	base := [...]int{200, 140, 95}[g.Speed]
	return time.Duration(max(base-g.Eaten*3, 50)) * time.Millisecond
}

// Advance moves time forward. It returns true when the snake has just died.
func (g *Game) Advance(dt time.Duration) bool {
	if g.Dead {
		return false
	}
	g.Prog += float64(dt) / float64(g.Interval())
	for g.Prog >= 1 {
		g.Prog--
		if g.step() {
			g.Prog = 0
			return true
		}
	}
	return false
}

// step moves the head one cell. It returns true when the move kills the snake.
func (g *Game) step() bool {
	if len(g.turns) > 0 {
		g.Dir, g.turns = g.turns[0], g.turns[1:]
	}
	next := g.Next()
	// The tail cell is free to enter unless the tail is staying put this step.
	tail := g.Body[len(g.Body)-1]
	if !inBounds(next) || (g.onBody(next) && (next != tail || g.growing > 0)) {
		g.Dead = true
		return true
	}
	g.Body = append([]Point{next}, g.Body...)
	if next == g.Food {
		g.Eaten++
		g.Score += foodScore[g.Speed]
		g.growing += growBy
		g.placeFood()
	}
	if g.growing > 0 {
		g.growing--
	} else {
		g.Body = g.Body[:len(g.Body)-1]
	}
	return false
}

// Growing reports whether the tail stays put this step, so it is drawn whole.
func (g *Game) Growing() bool { return g.growing > 0 }

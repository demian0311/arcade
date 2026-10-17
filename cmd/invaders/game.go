package main

import (
	"math/rand"
	"time"
)

// The field is 28 x 16 cells of 4 dots, so everything here is in dots at
// scale 1: FieldW x FieldH. The renderer multiplies by the scale.
const (
	FieldW = 112
	FieldH = 64

	Cols = 8 // aliens per row
	Rows = 5 // rows of aliens
	// Formation spacing in dots.
	ColGap = 11
	RowGap = 8
	// Formation start, in dots; each wave starts a row lower.
	StartX = 4
	StartY = 4

	AlienW, AlienH   = 8, 6
	CannonW, CannonH = 8, 4
	ShieldW, ShieldH = 12, 8
	ShieldY          = 46 // top of the shields; the formation reaching it ends the game
	CannonY          = FieldH - CannonH - 2
	ShotLen          = 3
	ShipW, ShipH     = 8, 4

	StepMax   = 600 * time.Millisecond // between formation steps with a full formation
	StepMin   = 60 * time.Millisecond  // with one alien left
	StepDX    = 2                      // dots sideways per step
	StepDown  = 4                      // dots down at an edge
	Lives     = 3
	Respawn   = time.Second // the cannon is hidden this long after a hit
	BombEvery = 800 * time.Millisecond
	MaxBombs  = 3
	ShipMin   = 20 * time.Second
	ShipMax   = 30 * time.Second
	ShipScore = 100

	CannonSpeed = 40.0 // dots per second
	ShotSpeed   = 80.0
	BombSpeed   = 30.0
	ShipSpeed   = 20.0
)

// Kind is an alien type by row: top, middle, bottom.
type Kind uint8

const (
	Top Kind = iota
	Mid
	Bottom
)

var kindScore = [...]int{Top: 30, Mid: 20, Bottom: 10}

func kindOf(row int) Kind {
	switch row {
	case 0:
		return Top
	case 1, 2:
		return Mid
	}
	return Bottom
}

// Rect is a dot rectangle: [X, X+W) x [Y, Y+H).
type Rect struct{ X, Y, W, H int }

func (r Rect) overlaps(o Rect) bool {
	return r.X < o.X+o.W && o.X < r.X+r.W && r.Y < o.Y+o.H && o.Y < r.Y+r.H
}

// Shot is a 1 x ShotLen line moving up (player) or down (bomb).
type Shot struct {
	X, Y float64
	Down bool
}

func (s Shot) rect() Rect { return Rect{int(s.X), int(s.Y), 1, ShotLen} }

// Shield is a bitmap that erodes where shots land.
type Shield struct {
	X, Y int
	Dots [ShieldH][ShieldW]bool
}

var shieldShape = [ShieldH]string{
	"..########..",
	".##########.",
	"############",
	"############",
	"############",
	"####....####",
	"###......###",
	"###......###",
}

func newShield(x, y int) Shield {
	s := Shield{X: x, Y: y}
	for r := range shieldShape {
		for c, ch := range shieldShape[r] {
			s.Dots[r][c] = ch == '#'
		}
	}
	return s
}

// hit reports whether any dot of r touches the shield and, if so, clears a
// 3 x 3 area around the first dot touched.
func (s *Shield) hit(r Rect) bool {
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			lx, ly := x-s.X, y-s.Y
			if lx < 0 || lx >= ShieldW || ly < 0 || ly >= ShieldH || !s.Dots[ly][lx] {
				continue
			}
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if ex, ey := lx+dx, ly+dy; ex >= 0 && ex < ShieldW && ey >= 0 && ey < ShieldH {
						s.Dots[ey][ex] = false
					}
				}
			}
			return true
		}
	}
	return false
}

type Game struct {
	Wave   int
	Score  int
	Lives  int
	Alive  [Rows][Cols]bool
	FX, FY int           // formation top-left, in dots
	Dir    int           // +1 right, -1 left
	Frame  int           // animation frame, swapped on each step
	Step   time.Duration // time since the last formation step

	CannonX   float64
	Hidden    time.Duration // time left until the cannon respawns; 0 when it is on the field
	Shot      *Shot         // the one player shot in the air
	Bombs     []Shot
	Shields   [4]Shield
	Ship      *float64 // the mystery ship's X when crossing
	shipIn    time.Duration
	bombClock time.Duration
	rng       *rand.Rand

	Flare  Kind // last alien killed, for the backdrop
	Killed bool // an alien died this tick
	Lost   bool // the cannon was hit this tick
	Over   bool // the formation reached the shields
}

func NewGame(seed int64) *Game {
	g := &Game{Lives: Lives, rng: rand.New(rand.NewSource(seed))}
	g.startWave(1)
	return g
}

// startWave resets the formation for wave n, one row lower each time.
func (g *Game) startWave(n int) {
	g.Wave = n
	for r := range g.Alive {
		for c := range g.Alive[r] {
			g.Alive[r][c] = true
		}
	}
	g.FX, g.FY, g.Dir, g.Frame, g.Step = StartX, StartY+(n-1)*RowGap/2, 1, 0, 0
	g.CannonX = float64(FieldW-CannonW) / 2
	g.Hidden, g.Shot, g.Bombs, g.Ship = 0, nil, nil, nil
	for i := range g.Shields {
		g.Shields[i] = newShield(10+i*24, ShieldY)
	}
	g.shipIn = g.shipDelay()
	g.Over = false
}

func (g *Game) shipDelay() time.Duration {
	return ShipMin + time.Duration(g.rng.Int63n(int64(ShipMax-ShipMin)))
}

// Remaining counts live aliens.
func (g *Game) Remaining() int {
	n := 0
	for r := range g.Alive {
		for c := range g.Alive[r] {
			if g.Alive[r][c] {
				n++
			}
		}
	}
	return n
}

// StepInterval shrinks from StepMax with a full formation to StepMin with
// one alien left, and tightens a little with each wave.
func (g *Game) StepInterval() time.Duration {
	n := max(1, g.Remaining())
	iv := StepMin + time.Duration(float64(StepMax-StepMin)*float64(n-1)/float64(Rows*Cols-1))
	iv -= time.Duration(g.Wave-1) * 40 * time.Millisecond
	return max(StepMin, iv)
}

func (g *Game) alienRect(r, c int) Rect {
	return Rect{g.FX + c*ColGap, g.FY + r*RowGap, AlienW, AlienH}
}

// StepFormation moves the formation one step: sideways, or down and
// reversed at an edge. It returns true when the aliens reach the shields.
func (g *Game) StepFormation() bool {
	g.Frame ^= 1
	minX, maxX := FieldW, -1
	for r := range g.Alive {
		for c := range g.Alive[r] {
			if g.Alive[r][c] {
				a := g.alienRect(r, c)
				minX, maxX = min(minX, a.X), max(maxX, a.X+a.W)
			}
		}
	}
	if maxX < 0 {
		return false
	}
	if nx := minX + g.Dir*StepDX; nx < 0 || maxX+g.Dir*StepDX > FieldW {
		g.FY += StepDown
		g.Dir = -g.Dir
	} else {
		g.FX += g.Dir * StepDX
	}
	for r := range g.Alive {
		for c := range g.Alive[r] {
			if g.Alive[r][c] && g.alienRect(r, c).Y+AlienH > ShieldY {
				g.Over = true
				return true
			}
		}
	}
	return false
}

func (g *Game) cannonRect() Rect { return Rect{int(g.CannonX), CannonY, CannonW, CannonH} }

// Fire launches the player's shot if none is in the air.
func (g *Game) Fire() bool {
	if g.Shot != nil || g.Hidden > 0 {
		return false
	}
	g.Shot = &Shot{X: g.CannonX + CannonW/2, Y: CannonY - ShotLen}
	return true
}

// Move slides the cannon by dx dots, kept on the field.
func (g *Game) Move(dx float64) {
	if g.Hidden > 0 {
		return
	}
	g.CannonX = max(0, min(float64(FieldW-CannonW), g.CannonX+dx))
}

// Advance runs dt of play. Killed, Lost and Over report what happened.
func (g *Game) Advance(dt time.Duration) {
	g.Killed, g.Lost = false, false
	sec := dt.Seconds()

	g.Step += dt
	for g.Step >= g.StepInterval() {
		g.Step -= g.StepInterval()
		if g.StepFormation() {
			return
		}
	}

	if g.Hidden > 0 {
		g.Hidden = max(0, g.Hidden-dt)
	}

	if g.Shot != nil {
		g.Shot.Y -= ShotSpeed * sec
		if g.Shot.Y < -ShotLen {
			g.Shot = nil
		} else if g.hitShield(g.Shot.rect()) {
			g.Shot = nil
		} else if g.Ship != nil && g.Shot.rect().overlaps(Rect{int(*g.Ship), 0, ShipW, ShipH}) {
			g.Score += ShipScore
			g.Ship, g.Shot = nil, nil
		} else {
			r := g.Shot.rect()
		kill:
			for row := Rows - 1; row >= 0; row-- {
				for c := range g.Alive[row] {
					if g.Alive[row][c] && r.overlaps(g.alienRect(row, c)) {
						g.Alive[row][c] = false
						g.Score += kindScore[kindOf(row)]
						g.Flare, g.Killed = kindOf(row), true
						g.Shot = nil
						break kill
					}
				}
			}
		}
	}

	g.bombClock += dt
	if len(g.Bombs) < MaxBombs && g.rng.Float64() < sec/BombEvery.Seconds() {
		if c := g.randomBottomColumn(); c >= 0 {
			a := g.alienRect(g.bottomRow(c), c)
			g.Bombs = append(g.Bombs, Shot{X: float64(a.X + AlienW/2), Y: float64(a.Y + AlienH), Down: true})
		}
	}
	kept := g.Bombs[:0]
	for _, b := range g.Bombs {
		b.Y += BombSpeed * sec
		r := b.rect()
		switch {
		case b.Y > FieldH:
		case g.hitShield(r):
		case g.Hidden == 0 && r.overlaps(g.cannonRect()):
			g.Lives--
			g.Lost = true
			g.Hidden = Respawn
			g.CannonX = float64(FieldW-CannonW) / 2
		default:
			kept = append(kept, b)
		}
	}
	g.Bombs = kept

	if g.Ship != nil {
		*g.Ship += ShipSpeed * sec
		if *g.Ship > FieldW {
			g.Ship = nil
			g.shipIn = g.shipDelay()
		}
	} else {
		g.shipIn -= dt
		if g.shipIn <= 0 {
			x := float64(-ShipW)
			g.Ship = &x
		}
	}
}

func (g *Game) hitShield(r Rect) bool {
	for i := range g.Shields {
		if g.Shields[i].hit(r) {
			return true
		}
	}
	return false
}

// bottomRow is the lowest live row in column c, or -1.
func (g *Game) bottomRow(c int) int {
	for r := Rows - 1; r >= 0; r-- {
		if g.Alive[r][c] {
			return r
		}
	}
	return -1
}

func (g *Game) randomBottomColumn() int {
	var cols []int
	for c := 0; c < Cols; c++ {
		if g.bottomRow(c) >= 0 {
			cols = append(cols, c)
		}
	}
	if len(cols) == 0 {
		return -1
	}
	return cols[g.rng.Intn(len(cols))]
}

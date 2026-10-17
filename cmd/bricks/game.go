package main

import (
	"math"
	"math/rand"
	"time"
)

// The board is BoardW x BoardH cells; positions and speeds are in cells,
// which at scale 1 is four dots per cell.
const (
	BoardW = 24
	BoardH = 16

	brickRows   = 5   // rows 1..brickRows hold bricks
	paddleW     = 4.0 // cells
	paddleWide  = 6.0
	paddleSpeed = 22.0 // cells per second
	ballR       = 0.25 // half the ball's side: a 2x2-dot square at scale 1
	baseSpeed   = 11.0 // cells per second at level 1
	capsuleFall = 5.0  // cells per second
	capsuleOdds = 8    // one brick in capsuleOdds drops a capsule
	wideFor     = 20.0 // seconds
	slowFor     = 15.0
	slowFactor  = 0.7
	maxBounce   = 60.0 // degrees from vertical off the paddle's edge
	startLives  = 3
)

type Color uint8

const (
	None Color = iota
	Red
	Orange
	Yellow
	Green
	Blue
	Purple
	White
)

var rowColor = [brickRows]Color{Red, Orange, Yellow, Green, Blue}
var rowScore = [brickRows]int{50, 40, 30, 20, 10}

type Kind uint8

const (
	Wide Kind = iota
	Slow
)

// A Capsule is a dropped power-up, two cells wide and one tall, falling.
type Capsule struct {
	X, Y float64 // top-left corner
	Kind Kind
}

// events is what happened during one Advance, for the model to react to.
type events struct {
	Hit     Color // a brick of this colour was broken, or None
	Lost    bool  // the ball fell out
	Cleared bool  // the last brick went
	Caught  bool  // a capsule was caught
}

type Game struct {
	Bricks   [BoardH][BoardW]Color
	PaddleX  float64 // centre
	PaddleW  float64
	BallX    float64 // centre
	BallY    float64
	VX, VY   float64
	Stuck    bool // the ball rides the paddle until launched
	Capsules []Capsule
	Level    int
	Score    int
	Lives    int
	Left     int // bricks remaining
	Dir      int // paddle motion this frame: -1, 0, 1

	t         float64 // seconds played
	wideUntil float64
	slowUntil float64
	rng       *rand.Rand
}

func NewGame(level int, seed int64) *Game {
	g := &Game{Level: level, Lives: startLives, rng: rand.New(rand.NewSource(seed))}
	g.reset()
	for y := 1; y <= brickRows; y++ {
		for x := 0; x < BoardW; x++ {
			g.Bricks[y][x] = rowColor[y-1]
			g.Left++
		}
	}
	return g
}

// NextLevel keeps the score and lives and refills the wall.
func (g *Game) NextLevel() {
	n := NewGame(g.Level+1, g.rng.Int63())
	n.Score, n.Lives = g.Score, g.Lives
	*g = *n
}

// reset puts the paddle in the middle with the ball resting on it.
func (g *Game) reset() {
	g.PaddleX, g.PaddleW = BoardW/2, paddleW
	g.Stuck = true
	g.VX, g.VY = 0, 0
	g.Capsules = nil
	g.wideUntil, g.slowUntil = 0, 0
	g.restBall()
}

func (g *Game) restBall() {
	g.BallX, g.BallY = g.PaddleX, BoardH-1-ballR
}

// speed is the ball's speed now, in cells per second.
func (g *Game) speed() float64 {
	v := baseSpeed * math.Pow(1.1, float64(g.Level-1))
	if g.t < g.slowUntil {
		v *= slowFactor
	}
	return v
}

// Launch sends the ball up from the paddle at a slight random angle.
func (g *Game) Launch() {
	if !g.Stuck {
		return
	}
	g.Stuck = false
	a := (10 + g.rng.Float64()*15) * math.Pi / 180
	if g.rng.Intn(2) == 0 {
		a = -a
	}
	g.setVelocity(a)
}

// setVelocity points the ball a radians from straight up, at the current speed.
func (g *Game) setVelocity(a float64) {
	v := g.speed()
	g.VX, g.VY = v*math.Sin(a), -v*math.Cos(a)
}

func (g *Game) paddleLeft() float64  { return g.PaddleX - g.PaddleW/2 }
func (g *Game) paddleRight() float64 { return g.PaddleX + g.PaddleW/2 }

// Advance moves time forward by dt and reports what happened.
func (g *Game) Advance(dt time.Duration) (ev events) {
	s := dt.Seconds()
	g.t += s
	if g.t >= g.wideUntil {
		g.PaddleW = paddleW
	}

	// Paddle.
	g.PaddleX += float64(g.Dir) * paddleSpeed * s
	g.PaddleX = max(g.PaddleW/2, min(BoardW-g.PaddleW/2, g.PaddleX))
	if g.Stuck {
		g.restBall()
	}

	// Capsules.
	kept := g.Capsules[:0]
	for _, c := range g.Capsules {
		c.Y += capsuleFall * s
		switch {
		case c.Y+1 >= BoardH-1 && c.X+2 > g.paddleLeft() && c.X < g.paddleRight():
			ev.Caught = true
			g.apply(c.Kind)
		case c.Y > BoardH:
		default:
			kept = append(kept, c)
		}
	}
	g.Capsules = kept

	// Ball, in steps short enough that it cannot pass through a brick.
	if g.Stuck {
		return ev
	}
	v := g.speed()
	if cur := math.Hypot(g.VX, g.VY); cur > 0 && math.Abs(cur-v) > 1e-6 {
		g.VX, g.VY = g.VX/cur*v, g.VY/cur*v
	}
	steps := max(1, int(math.Ceil(v*s/ballR)))
	for i := 0; i < steps && !ev.Lost; i++ {
		g.step(s/float64(steps), &ev)
	}
	return ev
}

func (g *Game) apply(k Kind) {
	switch k {
	case Wide:
		g.PaddleW = paddleWide
		g.wideUntil = g.t + wideFor
	case Slow:
		g.slowUntil = g.t + slowFor
	}
}

// step moves the ball for s seconds and resolves one collision.
func (g *Game) step(s float64, ev *events) {
	g.BallX += g.VX * s
	g.BallY += g.VY * s

	// Walls and ceiling.
	if g.BallX-ballR < 0 {
		g.BallX, g.VX = ballR, math.Abs(g.VX)
	} else if g.BallX+ballR > BoardW {
		g.BallX, g.VX = BoardW-ballR, -math.Abs(g.VX)
	}
	if g.BallY-ballR < 0 {
		g.BallY, g.VY = ballR, math.Abs(g.VY)
	}

	// Floor.
	if g.BallY-ballR > BoardH {
		g.Lives--
		ev.Lost = true
		g.reset()
		return
	}

	// Paddle: the ball's bottom crosses the paddle's top while falling.
	top := float64(BoardH - 1)
	if g.VY > 0 && g.BallY+ballR >= top && g.BallY-ballR < top+1 &&
		g.BallX+ballR > g.paddleLeft() && g.BallX-ballR < g.paddleRight() {
		g.BallY = top - ballR
		g.bounce(g.BallX)
		return
	}

	// Bricks: reflect on the axis of least penetration.
	x0, x1 := int(math.Floor(g.BallX-ballR)), int(math.Floor(g.BallX+ballR))
	y0, y1 := int(math.Floor(g.BallY-ballR)), int(math.Floor(g.BallY+ballR))
	for cy := max(0, y0); cy <= min(BoardH-1, y1); cy++ {
		for cx := max(0, x0); cx <= min(BoardW-1, x1); cx++ {
			c := g.Bricks[cy][cx]
			if c == None {
				continue
			}
			px := min(g.BallX+ballR-float64(cx), float64(cx+1)-(g.BallX-ballR))
			py := min(g.BallY+ballR-float64(cy), float64(cy+1)-(g.BallY-ballR))
			if px < py {
				if g.BallX < float64(cx)+0.5 {
					g.BallX -= px
				} else {
					g.BallX += px
				}
				g.VX = -g.VX
			} else {
				if g.BallY < float64(cy)+0.5 {
					g.BallY -= py
				} else {
					g.BallY += py
				}
				g.VY = -g.VY
			}
			g.Bricks[cy][cx] = None
			g.Left--
			g.Score += rowScore[cy-1]
			ev.Hit = c
			if g.rng.Intn(capsuleOdds) == 0 {
				g.Capsules = append(g.Capsules, Capsule{X: float64(cx) - 0.5, Y: float64(cy), Kind: Kind(g.rng.Intn(2))})
			}
			if g.Left == 0 {
				ev.Cleared = true
				g.Stuck = true
				g.VX, g.VY = 0, 0
				g.restBall()
			}
			return
		}
	}
}

// bounce sends the ball up off the paddle, steeper the nearer the edge it hit.
func (g *Game) bounce(x float64) {
	off := (x - g.PaddleX) / (g.PaddleW / 2)
	off = max(-1, min(1, off))
	g.setVelocity(off * maxBounce * math.Pi / 180)
}

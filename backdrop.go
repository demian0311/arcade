package main

import (
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// backdrop is the animated braille field behind the bottle: a slow plasma,
// dithered into dots so density carries the brightness, in one dim tint that
// drifts through the spectrum and flares in the colour of whatever just cleared.
type backdrop struct {
	cols, rows int
	t          float64
	lines      []string
	pulse      float64
	pulseColor Color
}

const (
	bgFrame     = 80 * time.Millisecond
	pulseDecay  = 1.6 // per second
	bgMaxDense  = 0.55
	bgHueCycleS = 90.0 // seconds for one trip round the hue wheel
)

// bayer8 is the 8x8 ordered-dither matrix, indexed [y][x] in absolute dot space,
// so the pattern never repeats within a character and density has 64 steps.
var bayer8 = func() (m [8][8]float64) {
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			v := 0
			for bit := 0; bit < 3; bit++ {
				xb, yb := (x>>bit)&1, (y>>bit)&1
				v |= (xb^yb)<<(2*(2-bit)+1) | yb<<(2*(2-bit))
			}
			m[y][x] = float64(v)
		}
	}
	return
}()

func (b *backdrop) resize(cols, rows int) {
	if cols == b.cols && rows == b.rows {
		return
	}
	b.cols, b.rows = cols, rows
	b.redraw()
}

func (b *backdrop) step(dt time.Duration) {
	b.t += dt.Seconds()
	b.pulse = max(0, b.pulse-dt.Seconds()*pulseDecay)
	b.redraw()
}

func (b *backdrop) flare(c Color) {
	b.pulse, b.pulseColor = 1, c
}

// field is the plasma value at dot (x,y), in 0..1.
func (b *backdrop) field(x, y float64) float64 {
	cx, cy := float64(b.cols), float64(b.rows)*2
	dx, dy := x-cx, y-cy
	v := math.Sin(x*0.055+b.t*0.5) +
		math.Sin(y*0.045-b.t*0.35) +
		math.Sin((x*0.03+y*0.04)+b.t*0.25) +
		math.Sin(math.Sqrt(dx*dx+dy*dy)*0.08-b.t*0.9)
	return (v + 4) / 8
}

func (b *backdrop) redraw() {
	if b.cols <= 0 || b.rows <= 0 {
		b.lines = nil
		return
	}
	lines := make([]string, b.rows)
	var sb strings.Builder
	for row := 0; row < b.rows; row++ {
		sb.Reset()
		for col := 0; col < b.cols; col++ {
			p := b.field(float64(col*2+1), float64(row*4+2))
			dense := p * p * bgMaxDense
			var bits rune
			for dx := 0; dx < 2; dx++ {
				for dy := 0; dy < 4; dy++ {
					if dense > (bayer8[(row*4+dy)%8][(col*2+dx)%8]+0.5)/64 {
						bits |= brailleBits[dx][dy]
					}
				}
			}
			if bits == 0 {
				sb.WriteByte(' ')
			} else {
				sb.WriteRune(0x2800 + bits)
			}
		}
		lines[row] = sb.String()
	}
	b.lines = lines
}

var pulseHex = map[Color]string{Red: "#ff6b6b", Yellow: "#ffd75f", Blue: "#5fafff"}

func (b *backdrop) style() lipgloss.Style {
	hue := math.Mod(b.t/bgHueCycleS*360, 360)
	base := colorful.Hsl(hue, 0.45, 0.28)
	if b.pulse > 0 {
		if flare, err := colorful.Hex(pulseHex[b.pulseColor]); err == nil {
			base = base.BlendLuv(flare, b.pulse*b.pulse)
		}
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(base.Hex()))
}

// line returns row r of the field, split around an opaque span [x0,x1) that the game occupies.
func (b *backdrop) line(r, x0, x1 int) (left, right string) {
	if r < 0 || r >= len(b.lines) {
		return "", ""
	}
	rs := []rune(b.lines[r])
	x0 = max(0, min(x0, len(rs)))
	x1 = max(x0, min(x1, len(rs)))
	return string(rs[:x0]), string(rs[x1:])
}

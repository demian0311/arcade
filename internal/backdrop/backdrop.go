// Package backdrop is the animated braille field behind a game: a slow
// plasma, dithered into dots so density carries the brightness, in one dim
// tint that drifts through the spectrum and flares in a colour on demand.
package backdrop

import (
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/demian0311/arcade/internal/canvas"
)

// Backdrop fills the terminal; the zero value is ready to use.
type Backdrop struct {
	cols, rows int
	t          float64
	lines      []string
	pulse      float64
	pulseHex   string
}

const (
	// Frame is how often the field should be stepped.
	Frame       = 80 * time.Millisecond
	pulseDecay  = 1.6 // per second
	maxDense    = 0.55
	hueCycleSec = 90.0 // seconds for one trip round the hue wheel
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

// Resize fits the field to a terminal of cols x rows characters.
func (b *Backdrop) Resize(cols, rows int) {
	if cols == b.cols && rows == b.rows {
		return
	}
	b.cols, b.rows = cols, rows
	b.redraw()
}

// Step advances the animation by dt.
func (b *Backdrop) Step(dt time.Duration) {
	b.t += dt.Seconds()
	b.pulse = max(0, b.pulse-dt.Seconds()*pulseDecay)
	b.redraw()
}

// Flare tints the field towards hex, fading back over about a second.
func (b *Backdrop) Flare(hex string) {
	b.pulse, b.pulseHex = 1, hex
}

// field is the plasma value at dot (x,y), in 0..1.
func (b *Backdrop) field(x, y float64) float64 {
	cx, cy := float64(b.cols), float64(b.rows)*2
	dx, dy := x-cx, y-cy
	v := math.Sin(x*0.055+b.t*0.5) +
		math.Sin(y*0.045-b.t*0.35) +
		math.Sin((x*0.03+y*0.04)+b.t*0.25) +
		math.Sin(math.Sqrt(dx*dx+dy*dy)*0.08-b.t*0.9)
	return (v + 4) / 8
}

func (b *Backdrop) redraw() {
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
			dense := p * p * maxDense
			var bits rune
			for dx := 0; dx < 2; dx++ {
				for dy := 0; dy < 4; dy++ {
					if dense > (bayer8[(row*4+dy)%8][(col*2+dx)%8]+0.5)/64 {
						bits |= canvas.BrailleBits[dx][dy]
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

// Style is the field's current tint.
func (b *Backdrop) Style() lipgloss.Style {
	hue := math.Mod(b.t/hueCycleSec*360, 360)
	base := colorful.Hsl(hue, 0.45, 0.28)
	if b.pulse > 0 {
		if flare, err := colorful.Hex(b.pulseHex); err == nil {
			base = base.BlendLuv(flare, b.pulse*b.pulse)
		}
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(base.Hex()))
}

// Line returns row r of the field, split around an opaque span [x0,x1) that the game occupies.
func (b *Backdrop) Line(r, x0, x1 int) (left, right string) {
	if r < 0 || r >= len(b.lines) {
		return "", ""
	}
	rs := []rune(b.lines[r])
	x0 = max(0, min(x0, len(rs)))
	x1 = max(x0, min(x1, len(rs)))
	return string(rs[:x0]), string(rs[x1:])
}

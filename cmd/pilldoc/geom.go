package main

import (
	"github.com/demian0311/arcade/internal/canvas"
)

// A geom is how the bottle is drawn: on the braille canvas, where a cell is
// n = 4s dots square (2s characters by s rows), or on half-block pixels,
// where a cell is n pixels square (n characters by n/2 rows). Braille is
// finer; pixels come in more sizes, so between two braille steps the pixel
// bottle is the bigger one.
type geom struct {
	pixel bool
	n     int // dots or pixels per cell; 0 means nothing fits
}

// geoms are the candidate sizes, smallest first.
var geoms = func() (gs []geom) {
	for n := 2; n <= 16; n++ {
		if n%4 == 0 {
			gs = append(gs, geom{n: n})
		}
		gs = append(gs, geom{pixel: true, n: n})
	}
	return
}()

// cellRows is a cell's height in terminal rows.
func (g geom) cellRows() float64 {
	if g.pixel {
		return float64(g.n) / 2
	}
	return float64(g.n) / 4
}

// rows and cols are the character size of a cells-wide by cells-tall grid.
func (g geom) rows(cells int) int {
	if g.pixel {
		return cells * g.n / 2
	}
	return cells * g.n / 4
}

func (g geom) cols(cells int) int {
	if g.pixel {
		return cells * g.n
	}
	return cells * g.n / 2
}

// grid makes a blank surface for a cells-wide by cells-tall area.
func (g geom) grid(cellsW, cellsH int) canvas.Grid {
	if g.pixel {
		return canvas.NewPixels(cellsW*g.n, cellsH*g.n, palette)
	}
	return canvas.New(cellsW*g.n, cellsH*g.n, palette)
}

// gap is the air a cell leaves on an unjoined side, so neighbours stay distinct.
func (g geom) gap() int {
	if g.n >= 6 {
		return 1
	}
	return 0
}

// radius is how far a rounded corner is cut back. A virus in a tiny pixel
// cell stays square, since a cut plus its eyes would leave only a bar.
func (g geom) radius(kind Kind) int {
	if g.pixel && g.n < 4 && kind == Virus {
		return 0
	}
	return max(1, g.n/4)
}

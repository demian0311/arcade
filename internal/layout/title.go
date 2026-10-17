package layout

import (
	"strings"

	"github.com/demian0311/arcade/internal/canvas"
)

// glyphs is a 5x7 dot font for titles.
var glyphs = map[rune][GlyphH]string{
	'A': {".###.", "#...#", "#...#", "#####", "#...#", "#...#", "#...#"},
	'B': {"####.", "#...#", "#...#", "####.", "#...#", "#...#", "####."},
	'C': {".###.", "#...#", "#....", "#....", "#....", "#...#", ".###."},
	'D': {"####.", "#...#", "#...#", "#...#", "#...#", "#...#", "####."},
	'E': {"#####", "#....", "#....", "####.", "#....", "#....", "#####"},
	'F': {"#####", "#....", "#....", "####.", "#....", "#....", "#...."},
	'G': {".###.", "#...#", "#....", "#.###", "#...#", "#...#", ".###."},
	'H': {"#...#", "#...#", "#...#", "#####", "#...#", "#...#", "#...#"},
	'I': {"#####", "..#..", "..#..", "..#..", "..#..", "..#..", "#####"},
	'J': {"..###", "...#.", "...#.", "...#.", "...#.", "#..#.", ".##.."},
	'K': {"#...#", "#..#.", "#.#..", "##...", "#.#..", "#..#.", "#...#"},
	'L': {"#....", "#....", "#....", "#....", "#....", "#....", "#####"},
	'M': {"#...#", "##.##", "#.#.#", "#.#.#", "#...#", "#...#", "#...#"},
	'N': {"#...#", "##..#", "#.#.#", "#..##", "#...#", "#...#", "#...#"},
	'O': {".###.", "#...#", "#...#", "#...#", "#...#", "#...#", ".###."},
	'P': {"####.", "#...#", "#...#", "####.", "#....", "#....", "#...."},
	'Q': {".###.", "#...#", "#...#", "#...#", "#.#.#", "#..#.", ".##.#"},
	'R': {"####.", "#...#", "#...#", "####.", "#.#..", "#..#.", "#...#"},
	'S': {".####", "#....", "#....", ".###.", "....#", "....#", "####."},
	'T': {"#####", "..#..", "..#..", "..#..", "..#..", "..#..", "..#.."},
	'U': {"#...#", "#...#", "#...#", "#...#", "#...#", "#...#", ".###."},
	'V': {"#...#", "#...#", "#...#", "#...#", "#...#", ".#.#.", "..#.."},
	'W': {"#...#", "#...#", "#...#", "#.#.#", "#.#.#", "##.##", "#...#"},
	'X': {"#...#", "#...#", ".#.#.", "..#..", ".#.#.", "#...#", "#...#"},
	'Y': {"#...#", "#...#", ".#.#.", "..#..", "..#..", "..#..", "..#.."},
	'Z': {"#####", "....#", "...#.", "..#..", ".#...", "#....", "#####"},
}

const (
	GlyphW   = 5
	GlyphH   = 7
	GlyphGap = 1

	// MaxTitleScale is the largest dot multiplier Title is asked for.
	MaxTitleScale = 2
)

func titleDots(text string, ts int) (w, h int) {
	w = (len(text)*(GlyphW+GlyphGap) - GlyphGap) * ts
	return w, GlyphH * ts
}

// TitleWidth and TitleHeight are the character size of Title. Scale 0 is
// plain text, for terminals too narrow for the dot font.
func TitleWidth(text string, ts int) int {
	if ts == 0 {
		return len(text)
	}
	w, _ := titleDots(text, ts)
	return (w + 1) / 2
}

func TitleHeight(text string, ts int) int {
	if ts == 0 {
		return 1
	}
	_, h := titleDots(text, ts)
	return (h + 3) / 4
}

// Title draws text in the dot font at scale ts, each letter in the next ink
// from inks; at scale 0 it is bold text in the same colours.
func Title(text string, ts int, inks []canvas.Ink, pal canvas.Palette) string {
	if ts == 0 {
		var b strings.Builder
		for i, r := range text {
			b.WriteString(pal[inks[i%len(inks)]].Bold(true).Render(string(r)))
		}
		return b.String()
	}
	c := canvas.New(TitleWidth(text, ts)*2, TitleHeight(text, ts)*4, pal)
	for i, r := range text {
		g := glyphs[r]
		ink := inks[i%len(inks)]
		ox := i * (GlyphW + GlyphGap) * ts
		for gy := 0; gy < GlyphH; gy++ {
			for gx := 0; gx < GlyphW; gx++ {
				if g[gy][gx] != '#' {
					continue
				}
				for dy := 0; dy < ts; dy++ {
					for dx := 0; dx < ts; dx++ {
						c.Set(ox+gx*ts+dx, gy*ts+dy, ink)
					}
				}
			}
		}
	}
	return c.String()
}

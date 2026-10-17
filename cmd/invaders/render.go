package main

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/demian0311/arcade/internal/canvas"
)

const (
	inkTop canvas.Ink = 1 + iota
	inkMid
	inkBottom
	inkCannon
	inkShield
	inkBomb
	inkShip
)

var hex = map[canvas.Ink]string{
	inkTop: "#c792ea", inkMid: "#5fd7ff", inkBottom: "#8ae234",
	inkCannon: "#ffffff", inkShield: "#6c6c6c", inkBomb: "#ffaf5f", inkShip: "#ff6b6b",
}

var kindInk = [...]canvas.Ink{Top: inkTop, Mid: inkMid, Bottom: inkBottom}

var palette = func() canvas.Palette {
	p := canvas.Palette{}
	for i, h := range hex {
		p[i] = lipgloss.NewStyle().Foreground(lipgloss.Color(h))
	}
	p[inkCannon] = p[inkCannon].Bold(true)
	return p
}()

// Sprites are dot bitmaps at scale 1; each dot becomes an s x s block.
var alienFrames = [3][2][AlienH]string{
	Top: {
		{"...##...", "..####..", ".######.", "##.##.##", "########", ".#.##.#."},
		{"...##...", "..####..", ".######.", "##.##.##", "########", "#......#"},
	},
	Mid: {
		{"..#..#..", "#.####.#", "########", "##.##.##", "########", ".#....#."},
		{"..#..#..", "..####..", ".######.", "##.##.##", "########", "#.#..#.#"},
	},
	Bottom: {
		{".######.", "########", "###..###", "########", "..#..#..", ".#.##.#."},
		{".######.", "########", "###..###", "########", ".#.##.#.", "#......#"},
	},
}

var cannonShape = [CannonH]string{
	"...##...",
	".######.",
	"########",
	"########",
}

var shipShape = [ShipH]string{
	"..####..",
	".######.",
	"########",
	".#.##.#.",
}

// blit paints bitmap rows at dot (ox,oy), scaled by s.
func blit(c *canvas.Canvas, rows []string, ox, oy, s int, i canvas.Ink) {
	for y, row := range rows {
		for x, ch := range row {
			if ch == '#' {
				c.Tile((ox+x)*s, (oy+y)*s, s, s, 0, canvas.Corners{}, i)
			}
		}
	}
}

func renderBoard(g *Game, s int) string {
	c := canvas.New(FieldW*s, FieldH*s, palette)
	for r := range g.Alive {
		for col := range g.Alive[r] {
			if !g.Alive[r][col] {
				continue
			}
			a := g.alienRect(r, col)
			k := kindOf(r)
			blit(c, alienFrames[k][g.Frame][:], a.X, a.Y, s, kindInk[k])
		}
	}
	for i := range g.Shields {
		sh := &g.Shields[i]
		for y := range sh.Dots {
			for x := range sh.Dots[y] {
				if sh.Dots[y][x] {
					c.Tile((sh.X+x)*s, (sh.Y+y)*s, s, s, 0, canvas.Corners{}, inkShield)
				}
			}
		}
	}
	if g.Hidden == 0 {
		blit(c, cannonShape[:], int(g.CannonX), CannonY, s, inkCannon)
	}
	if g.Shot != nil {
		r := g.Shot.rect()
		c.Tile(r.X*s, r.Y*s, s, ShotLen*s, 0, canvas.Corners{}, inkCannon)
	}
	for _, b := range g.Bombs {
		r := b.rect()
		c.Tile(r.X*s, r.Y*s, s, ShotLen*s, 0, canvas.Corners{}, inkBomb)
	}
	if g.Ship != nil {
		blit(c, shipShape[:], int(*g.Ship), 0, s, inkShip)
	}
	return c.String()
}

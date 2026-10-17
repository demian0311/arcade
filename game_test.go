package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func emptyGame() *Game {
	g := NewGame(0, Med, 1)
	g.Board = Board{}
	g.Viruses = 0
	return g
}

func TestVirusPlacementHasNoTriples(t *testing.T) {
	for level := 0; level <= MaxLevel; level++ {
		g := NewGame(level, Med, int64(level))
		if g.Viruses != virusCount(level) {
			t.Fatalf("level %d: placed %d viruses, want %d", level, g.Viruses, virusCount(level))
		}
		if g.MarkMatches() != 0 {
			t.Fatalf("level %d: fresh board already has a match", level)
		}
		for y := 0; y < virusTopRow(level); y++ {
			for x := 0; x < BoardW; x++ {
				if g.Board[y][x].Kind != Empty {
					t.Fatalf("level %d: virus above row %d", level, virusTopRow(level))
				}
			}
		}
	}
}

func TestRotateSwapsColours(t *testing.T) {
	g := emptyGame()
	g.Pill = &Pill{X: 3, Y: 5, C: [2]Color{Red, Blue}}
	g.Rotate(true) // clockwise: left half goes on top
	if !g.Pill.Vertical || g.Pill.C != [2]Color{Blue, Red} {
		t.Fatalf("after cw: %+v", *g.Pill)
	}
	g.Rotate(true) // back to horizontal, top half goes right
	if g.Pill.Vertical || g.Pill.C != [2]Color{Blue, Red} {
		t.Fatalf("after second cw: %+v", *g.Pill)
	}
	g.Rotate(false)
	g.Rotate(false)
	if g.Pill.Vertical || g.Pill.C != [2]Color{Red, Blue} {
		t.Fatalf("two ccw should undo two cw: %+v", *g.Pill)
	}
}

func TestRotateKicksOffRightWall(t *testing.T) {
	g := emptyGame()
	g.Pill = &Pill{X: BoardW - 1, Y: 5, Vertical: true, C: [2]Color{Red, Blue}}
	if !g.Rotate(true) || g.Pill.X != BoardW-2 || g.Pill.Vertical {
		t.Fatalf("expected kick to column %d, got %+v", BoardW-2, *g.Pill)
	}
}

func TestMatchClearsAndOrphansPartner(t *testing.T) {
	g := emptyGame()
	y := BoardH - 1
	for x := 0; x < 3; x++ {
		g.Board[y][x] = Cell{Kind: Virus, Color: Red}
	}
	g.Viruses = 3
	g.Board[y][3] = Cell{Kind: Half, Color: Red, Link: Up}
	g.Board[y-1][3] = Cell{Kind: Half, Color: Blue, Link: Down}
	if n := g.MarkMatches(); n != 4 {
		t.Fatalf("marked %d, want 4", n)
	}
	if v := g.Clear(); v != 3 || g.Viruses != 0 {
		t.Fatalf("cleared %d viruses, left %d", v, g.Viruses)
	}
	if g.Score != 200+400+800 {
		t.Fatalf("score %d", g.Score)
	}
	if c := g.Board[y-1][3]; c.Kind != Half || c.Link != NoDir {
		t.Fatalf("partner not orphaned: %+v", c)
	}
	if !g.DropStep() || g.Board[y][3].Kind != Half || g.DropStep() {
		t.Fatal("orphan should fall exactly one row")
	}
}

func TestDropStepMovesPairsTogether(t *testing.T) {
	g := emptyGame()
	g.Board[0][2] = Cell{Kind: Half, Color: Red, Link: Right}
	g.Board[0][3] = Cell{Kind: Half, Color: Blue, Link: Left}
	g.Board[BoardH-1][3] = Cell{Kind: Virus, Color: Yellow}
	steps := 0
	for g.DropStep() {
		steps++
	}
	if steps != BoardH-2 {
		t.Fatalf("pair fell %d rows, want %d", steps, BoardH-2)
	}
	if g.Board[BoardH-2][2].Link != Right || g.Board[BoardH-2][3].Link != Left {
		t.Fatalf("pair split: %+v %+v", g.Board[BoardH-2][2], g.Board[BoardH-2][3])
	}
}

func TestViewFitsTerminal(t *testing.T) {
	for _, sz := range [][2]int{{44, 18}, {80, 24}, {120, 40}, {200, 70}} {
		m := newModel()
		mm, _ := m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		m = mm.(model)
		if m.scale < 1 {
			t.Fatalf("%v: scale %d", sz, m.scale)
		}
		v := m.View()
		if rows := strings.Count(v, "\n") + 1; rows > sz[1] {
			t.Fatalf("%v: view has %d rows", sz, rows)
		}
		if testing.Verbose() && sz[0] == 80 {
			t.Logf("\n%s", v)
		}
	}
}

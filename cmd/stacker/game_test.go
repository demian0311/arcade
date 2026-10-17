package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/demian0311/arcade/internal/session"
)

func TestEveryShapeHasFourCells(t *testing.T) {
	for k := Kind(0); k < kinds; k++ {
		for rot := 0; rot < 4; rot++ {
			n := 0
			Piece{Kind: k, Rot: rot}.cells(func(_, _ int, _ Link) { n++ })
			if n != 4 {
				t.Fatalf("kind %d rot %d has %d cells", k, rot, n)
			}
		}
	}
}

func TestLinksJoinThePiece(t *testing.T) {
	var links []Link
	Piece{Kind: O}.cells(func(_, _ int, l Link) { links = append(links, l) })
	want := []Link{LinkR | LinkD, LinkL | LinkD, LinkR | LinkU, LinkL | LinkU}
	for i := range want {
		if links[i] != want[i] {
			t.Fatalf("O links %v, want %v", links, want)
		}
	}
}

func TestRotationKicksOffTheWall(t *testing.T) {
	g := NewGame(0, 1)
	g.Piece = &Piece{Kind: T, Rot: 1, X: -1, Y: 5} // vertical T hugging the left wall
	if !g.fits(*g.Piece) {
		t.Fatal("setup piece does not fit")
	}
	if !g.Rotate(true) || g.Piece.Rot != 2 || g.Piece.X != 0 {
		t.Fatalf("rotation should kick right: rot %d x %d", g.Piece.Rot, g.Piece.X)
	}
}

func TestClearDropsRowsCutsLinksAndScores(t *testing.T) {
	g := NewGame(0, 1)
	for x := 0; x < BoardW; x++ {
		g.Board[BoardH-1][x] = Cell{Color: Red, Links: LinkU}
		g.Board[BoardH-2][x] = Cell{Color: Red, Links: LinkD}
	}
	g.Board[BoardH-2][0].Color = None // row above is not full
	if n := g.MarkFull(); n != 1 {
		t.Fatalf("marked %d rows", n)
	}
	if n := g.Clear(); n != 1 || g.Lines != 1 || g.Score != 40 {
		t.Fatalf("cleared %d, lines %d, score %d", n, g.Lines, g.Score)
	}
	if g.Board[BoardH-1][1].Color != Red || g.Board[BoardH-1][1].Links&LinkD != 0 {
		t.Fatalf("row above should drop with its downward link cut: %+v", g.Board[BoardH-1][1])
	}
	if g.Board[BoardH-2][1].Color != None {
		t.Fatal("the dropped row left a copy behind")
	}
}

func TestSoftDropLocksAfterGrace(t *testing.T) {
	g := NewGame(0, 1)
	g.Spawn()
	g.Soft, g.Held = true, true
	for !g.Advance(softInterval) {
	}
	if g.Rest < softLockDelay {
		t.Fatalf("locked early, rest %v", g.Rest)
	}
	if !g.Lock() || g.Piece != nil {
		t.Fatal("lock should succeed inside the well")
	}
}

func TestLockAboveTheWellEndsTheGame(t *testing.T) {
	g := NewGame(0, 1)
	g.Piece = &Piece{Kind: I, Rot: 1, X: 3, Y: -2}
	if g.Lock() {
		t.Fatal("a piece sticking out of the top should end the game")
	}
}

func TestViewFitsTerminal(t *testing.T) {
	for _, sz := range [][2]int{{layoutWidth(1, 0), boardRows(1)}, {56, 29}, {80, 24}, {120, 44}, {200, 90}} {
		m := newModel()
		mm, _ := m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		m = mm.(model)
		if m.scale < 1 {
			t.Fatalf("%v: scale %d", sz, m.scale)
		}
		mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = mm.(model)
		v := m.View()
		if rows := strings.Count(v, "\n") + 1; rows != sz[1] {
			t.Fatalf("%v: view has %d rows", sz, rows)
		}
		for i, l := range strings.Split(v, "\n") {
			if w := lipgloss.Width(l); w != sz[0] {
				t.Fatalf("%v: row %d is %d wide", sz, i, w)
			}
		}
	}
}

func TestBlurPausesAndFocusResumes(t *testing.T) {
	m := newModel()
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mm.(model)
	mm, _ = m.Update(tea.BlurMsg{})
	m = mm.(model)
	if m.phase != phasePaused {
		t.Fatalf("blur should pause, phase %d", m.phase)
	}
	mm, _ = m.Update(tea.FocusMsg{})
	m = mm.(model)
	if m.phase != phasePaused || m.focus.Countdown == 0 {
		t.Fatal("focus should start the countdown")
	}
}

func TestGhostRestsOnThePile(t *testing.T) {
	g := NewGame(0, 1)
	g.Piece = &Piece{Kind: O, X: 0, Y: 0}
	g.Board[BoardH-1][1] = Cell{Color: Red}
	if l := g.landing(); l.Y != BoardH-3 {
		t.Fatalf("ghost landed at y %d", l.Y)
	}
}

func TestClearWaitsForTheBurnThenCollapses(t *testing.T) {
	m := newModel()
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mm.(model)
	for x := 0; x < BoardW; x++ {
		m.g.Board[BoardH-1][x] = Cell{Color: Red}
	}
	m.g.Board[BoardH-1][0].Color = None
	m.g.Piece = &Piece{Kind: I, Rot: 1, X: -2, Y: BoardH - 4} // vertical bar filling column 0
	mm, _ = m.lock()
	m = mm.(model)
	if m.phase != phaseClearing || m.g.Lines != 0 {
		t.Fatalf("lock onto a full row should start clearing: phase %d lines %d", m.phase, m.g.Lines)
	}
	start := m.fx.clearAt
	mm, _ = m.Update(session.TickMsg{ID: m.ticker.ID(), At: start.Add(clearDelay / 2)})
	m = mm.(model)
	if m.phase != phaseClearing {
		t.Fatal("the row collapsed before the burn finished")
	}
	mm, _ = m.Update(session.TickMsg{ID: m.ticker.ID(), At: start.Add(clearDelay)})
	m = mm.(model)
	if m.phase != phaseFalling || m.g.Lines != 1 {
		t.Fatalf("after the burn: phase %d lines %d", m.phase, m.g.Lines)
	}
}

func TestSlamRecordsTheBandItCrossed(t *testing.T) {
	m := newModel()
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mm.(model)
	m.g.Piece = &Piece{Kind: O, X: 4, Y: 0}
	mm, _ = m.slam()
	m = mm.(model)
	if m.fx.slamX0 != 5 || m.fx.slamX1 != 6 || m.fx.slamY0 != 0 || m.fx.slamY1 != BoardH-2 {
		t.Fatalf("slam band %d..%d x %d..%d", m.fx.slamX0, m.fx.slamX1, m.fx.slamY0, m.fx.slamY1)
	}
	if len(m.fx.landed) != 4 || m.fx.landedAt.IsZero() {
		t.Fatal("the landing flash was not started")
	}
}

func TestCompletingRowsFollowTheLanding(t *testing.T) {
	g := NewGame(0, 1)
	for x := 1; x < BoardW; x++ {
		g.Board[BoardH-1][x] = Cell{Color: Red}
		g.Board[BoardH-3][x] = Cell{Color: Red}
	}
	g.Piece = &Piece{Kind: I, Rot: 1, X: -2, Y: 0} // vertical bar over column 0
	full := g.completing(g.landing())
	if !full[BoardH-1] || !full[BoardH-3] || full[BoardH-2] {
		t.Fatalf("completing rows %v", full)
	}
	g.Piece.X = 3
	if full := g.completing(g.landing()); full[BoardH-1] {
		t.Fatal("a bar away from the gap completes nothing")
	}
}

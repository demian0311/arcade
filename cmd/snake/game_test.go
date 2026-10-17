package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestStepMovesAndKeepsLength(t *testing.T) {
	g := NewGame(Med, 1)
	g.Food = Point{0, 0}
	head := g.Head()
	if g.step() || g.Head() != (Point{head.X + 1, head.Y}) || len(g.Body) != startLen {
		t.Fatalf("after a step: head %v len %d", g.Head(), len(g.Body))
	}
}

func TestEatingGrowsAndScores(t *testing.T) {
	g := NewGame(Med, 1)
	g.Food = g.Next()
	g.step()
	for i := 0; i < growBy; i++ {
		g.Food = Point{0, 0}
		g.step()
	}
	if len(g.Body) != startLen+growBy || g.Score != foodScore[Med] || g.Eaten != 1 {
		t.Fatalf("len %d score %d eaten %d", len(g.Body), g.Score, g.Eaten)
	}
}

func TestWallAndSelfKill(t *testing.T) {
	g := NewGame(Med, 1)
	g.Food = Point{0, 0}
	for !g.step() {
	}
	if !g.Dead || inBounds(g.Next()) {
		t.Fatal("running right should die at the wall")
	}
	g = NewGame(Med, 1)
	g.Food = Point{0, 0}
	g.Body = []Point{{5, 5}, {4, 5}, {4, 6}, {5, 6}, {6, 6}}
	g.Dir = Right
	g.Turn(Down)
	if !g.step() {
		t.Fatal("turning into the body should die")
	}
}

func TestTurnRejectsReversalAndQueuesTwo(t *testing.T) {
	g := NewGame(Med, 1)
	g.Turn(Left)
	if len(g.turns) != 0 {
		t.Fatal("a reversal should be ignored")
	}
	g.Turn(Up)
	g.Turn(Left)
	g.Turn(Down)
	if len(g.turns) != 2 || g.turns[0] != Up || g.turns[1] != Left {
		t.Fatalf("queued %v", g.turns)
	}
}

func TestTailCellIsFreeToEnter(t *testing.T) {
	g := NewGame(Med, 1)
	g.Food = Point{0, 0}
	g.Body = []Point{{5, 5}, {4, 5}, {4, 6}, {5, 6}}
	g.Dir = Down // into the tail cell, which moves away this step
	if g.step() {
		t.Fatal("moving into the vacating tail should be allowed")
	}
}

func TestAdvanceStepsOncePerInterval(t *testing.T) {
	g := NewGame(Med, 1)
	g.Food = Point{0, 0}
	head := g.Head()
	g.Advance(g.Interval() / 2)
	if g.Head() != head || g.Prog < 0.49 {
		t.Fatalf("half an interval moved the head or lost progress: %v %.2f", g.Head(), g.Prog)
	}
	g.Advance(g.Interval() / 2)
	if g.Head().X != head.X+1 {
		t.Fatalf("one interval should step once, head %v", g.Head())
	}
}

func TestViewFitsTerminal(t *testing.T) {
	for _, sz := range [][2]int{{layoutWidth(1, 0), boardRows(1)}, {88, 29}, {80, 24}, {120, 40}, {200, 70}} {
		m := newModel()
		mm, _ := m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		m = mm.(model)
		if m.scale < 1 {
			t.Fatalf("%v: scale %d", sz, m.scale)
		}
		mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = mm.(model)
		m.g.Prog = 0.5
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

package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestFormationReversesAndDropsAtTheEdge(t *testing.T) {
	g := NewGame(1)
	fy := g.FY
	steps := 0
	for g.Dir == 1 {
		g.StepFormation()
		steps++
		if steps > 200 {
			t.Fatal("formation never reached the right edge")
		}
	}
	if g.FY != fy+StepDown {
		t.Fatalf("formation should drop %d at the edge, dropped %d", StepDown, g.FY-fy)
	}
	right := g.FX + (Cols-1)*ColGap + AlienW
	if right > FieldW {
		t.Fatalf("formation overran the field: right edge %d", right)
	}
}

func TestShotKillsAnAlienAndScoresByType(t *testing.T) {
	g := NewGame(1)
	g.Bombs = nil
	a := g.alienRect(Rows-1, 0) // a bottom-row alien
	g.Shot = &Shot{X: float64(a.X + AlienW/2), Y: float64(a.Y + AlienH)}
	g.Advance(10 * time.Millisecond)
	if g.Alive[Rows-1][0] || g.Score != kindScore[Bottom] || !g.Killed || g.Flare != Bottom {
		t.Fatalf("alive %v score %d killed %v flare %d", g.Alive[Rows-1][0], g.Score, g.Killed, g.Flare)
	}
	if g.Shot != nil {
		t.Fatal("the shot should be spent")
	}
	g.Alive = [Rows][Cols]bool{}
	g.Alive[0][3] = true
	a = g.alienRect(0, 3)
	g.Shot = &Shot{X: float64(a.X + AlienW/2), Y: float64(a.Y + AlienH)}
	g.Advance(10 * time.Millisecond)
	if g.Score != kindScore[Bottom]+kindScore[Top] {
		t.Fatalf("a top alien should score %d, total %d", kindScore[Top], g.Score)
	}
}

func TestShieldErodesWhereHit(t *testing.T) {
	s := newShield(10, ShieldY)
	before := 0
	for _, row := range s.Dots {
		for _, d := range row {
			if d {
				before++
			}
		}
	}
	if !s.hit(Rect{10 + ShieldW/2, ShieldY + 2, 1, ShotLen}) {
		t.Fatal("a shot into the shield should hit")
	}
	after := 0
	for _, row := range s.Dots {
		for _, d := range row {
			if d {
				after++
			}
		}
	}
	if before-after != 9 {
		t.Fatalf("a hit should clear 9 dots, cleared %d", before-after)
	}
	if s.hit(Rect{0, 0, 1, 1}) {
		t.Fatal("a shot away from the shield should miss")
	}
}

func TestStepIntervalShrinksAsAliensDie(t *testing.T) {
	g := NewGame(1)
	full := g.StepInterval()
	if full != StepMax {
		t.Fatalf("full formation interval %v", full)
	}
	for r := range g.Alive {
		for c := range g.Alive[r] {
			g.Alive[r][c] = false
		}
	}
	g.Alive[0][0] = true
	if last := g.StepInterval(); last != StepMin {
		t.Fatalf("last alien interval %v", last)
	}
	g.Alive[0][1] = true
	if two := g.StepInterval(); two <= StepMin || two >= full {
		t.Fatalf("two aliens interval %v", two)
	}
}

func TestViewFitsTerminal(t *testing.T) {
	for _, sz := range [][2]int{{layoutWidth(1, 0), boardRows(1)}, {88, 29}, {120, 40}, {200, 70}} {
		m := newModel()
		mm, _ := m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		m = mm.(model)
		if m.scale < 1 {
			t.Fatalf("%v: scale %d", sz, m.scale)
		}
		mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = mm.(model)
		x := 30.0
		m.g.Ship = &x
		m.g.Shot = &Shot{X: 50, Y: 30}
		m.g.Bombs = []Shot{{X: 20, Y: 20, Down: true}}
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

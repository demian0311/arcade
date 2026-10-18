package main

import (
	"math"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func clearGame() *Game {
	g := NewGame(1, 1)
	g.Bricks = [BoardH][BoardW]Color{}
	g.Left = 0
	g.Stuck = false
	return g
}

func TestBallReflectsOffTheWall(t *testing.T) {
	g := clearGame()
	g.BallX, g.BallY = 0.5, 8
	g.VX, g.VY = -g.speed(), 0
	g.Advance(100 * time.Millisecond)
	if g.VX <= 0 || g.BallX < ballR {
		t.Fatalf("after the wall: x %.2f vx %.2f", g.BallX, g.VX)
	}
}

func TestBrickHitRemovesItAndScores(t *testing.T) {
	g := clearGame()
	g.Bricks[3][10], g.Bricks[1][0] = Yellow, Red
	g.Left = 2
	g.BallX, g.BallY = 10.5, 4.6
	g.VX, g.VY = 0, -g.speed()
	ev := g.Advance(50 * time.Millisecond)
	if ev.Hit != Yellow || g.Bricks[3][10] != None || g.Score != rowScore[2] || g.VY <= 0 || ev.Cleared {
		t.Fatalf("hit %d brick %d score %d vy %.2f cleared %v", ev.Hit, g.Bricks[3][10], g.Score, g.VY, ev.Cleared)
	}
	g.BallX, g.BallY = 0.5, 2.6
	g.VX, g.VY = 0, -g.speed()
	ev = g.Advance(50 * time.Millisecond)
	if !ev.Cleared || !g.Stuck || g.Left != 0 {
		t.Fatal("the last brick should clear the level and rest the ball")
	}
}

func TestPaddleBounceAngleFollowsHitPosition(t *testing.T) {
	angle := func(x float64) float64 {
		g := clearGame()
		g.PaddleX = 12
		g.BallX, g.BallY = x, BoardH-1-ballR-0.1
		g.VX, g.VY = 0, g.speed()
		g.Advance(30 * time.Millisecond)
		return math.Atan2(g.VX, -g.VY)
	}
	centre, edge := angle(12), angle(12+paddleW/2-0.1)
	if math.Abs(centre) > 0.01 || edge < 0.5 {
		t.Fatalf("centre %.2f edge %.2f", centre, edge)
	}
}

func TestLostBallCostsALife(t *testing.T) {
	g := clearGame()
	g.BallX, g.BallY = 2, BoardH-0.5 // off the paddle, almost out
	g.VX, g.VY = 0, g.speed()
	ev := g.Advance(100 * time.Millisecond)
	if !ev.Lost || g.Lives != startLives-1 || !g.Stuck {
		t.Fatalf("lost %v lives %d stuck %v", ev.Lost, g.Lives, g.Stuck)
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
		m.g.Launch()
		m.g.Advance(300 * time.Millisecond)
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

func TestAnyKeyResumesAPause(t *testing.T) {
	m := newModel()
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mm.(model)
	want := m.phase
	mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	m = mm.(model)
	if m.phase != phasePaused {
		t.Fatalf("p should pause, phase %d", m.phase)
	}
	mm, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = mm.(model)
	if m.phase != want {
		t.Fatalf("space should resume to phase %d, got %d", want, m.phase)
	}
	if !m.running() {
		t.Fatal("resumed game is not running")
	}
}

package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

func TestAdvanceFallsThenLocksAfterRest(t *testing.T) {
	g := emptyGame()
	g.Pill = &Pill{X: 0, Y: BoardH - 3, C: [2]Color{Red, Blue}}
	iv := g.Interval()
	if g.Advance(iv/2) || g.Pill.Y != BoardH-3 || g.Prog < 0.49 {
		t.Fatalf("half an interval: y=%d prog=%.2f", g.Pill.Y, g.Prog)
	}
	if g.Advance(iv/2) || g.Pill.Y != BoardH-2 {
		t.Fatalf("one interval should fall one row, y=%d", g.Pill.Y)
	}
	if g.Advance(iv) || g.Pill.Y != BoardH-1 || g.Prog != 0 {
		t.Fatalf("on the floor: y=%d prog=%.2f", g.Pill.Y, g.Prog)
	}
	if g.Advance(lockDelay / 2) {
		t.Fatal("locked before the lock delay")
	}
	if !g.Move(1) || g.Rest != 0 {
		t.Fatalf("a move while resting should restart the lock delay, rest=%v", g.Rest)
	}
	if g.Advance(lockDelay / 2) {
		t.Fatal("locked before the restarted delay ran out")
	}
	if !g.Advance(lockDelay / 2) {
		t.Fatal("did not lock after the lock delay")
	}
}

func TestSoftDropResetsOnLock(t *testing.T) {
	g := emptyGame()
	g.Pill = &Pill{X: 0, Y: 0, C: [2]Color{Red, Blue}}
	g.Soft = true
	for !g.Advance(softInterval) {
	}
	if g.Pill.Y != BoardH-1 {
		t.Fatalf("soft drop stopped at y=%d", g.Pill.Y)
	}
	if g.Rest < softLockDelay || g.Rest >= lockDelay {
		t.Fatalf("a soft-dropped pill should lock after softLockDelay, rest=%v", g.Rest)
	}
	g.Lock()
	if g.Soft || g.Prog != 0 {
		t.Fatalf("soft=%v prog=%.2f after lock", g.Soft, g.Prog)
	}
}

func TestHeldDownDoesNotCarryToNextPill(t *testing.T) {
	m := newModel()
	m.g = emptyGame()
	m.g.Board[BoardH-1][7] = Cell{Kind: Virus, Color: Yellow} // keeps the level alive
	m.g.Viruses = 1
	m.g.Pill = &Pill{X: 0, Y: 0, C: [2]Color{Red, Blue}}
	t0 := time.Now()
	m.pressDown(t0)
	if !m.g.Soft {
		t.Fatal("first press should start the soft drop")
	}
	for !m.g.Advance(softInterval) {
	}
	mm, _ := m.lock()
	m = mm.(model)
	if m.g.Pill == nil || m.g.Soft {
		t.Fatalf("expected a fresh slow pill, soft=%v", m.g.Soft)
	}
	at := t0.Add(250 * time.Millisecond) // auto-repeat begins and keeps arriving every 25 ms
	for i := 0; i < 20; i++ {
		m.pressDown(at)
		if m.softActive(at) {
			t.Fatalf("held key must not soft-drop the new pill (repeat %d)", i)
		}
		at = at.Add(25 * time.Millisecond)
	}
	m.pressDown(at.Add(400 * time.Millisecond)) // release, then press again
	if !m.softActive(at.Add(401 * time.Millisecond)) {
		t.Fatal("re-press after a gap should soft-drop")
	}
}

func TestHoldBridgesRepeatDelayAndReleaseEnds(t *testing.T) {
	m := newModel()
	t0 := time.Now()
	m.pressDown(t0)
	if !m.softActive(t0.Add(200 * time.Millisecond)) {
		t.Fatal("a fresh press must stay soft across the repeat delay")
	}
	if m.softActive(t0.Add(defaultRepeatDelay + repeatSlack + time.Millisecond)) {
		t.Fatal("a lone tap must end after the bridge window")
	}
	at := t0
	for i := 0; i < 12; i++ { // the first repeat lands at 250 ms, then every 25 ms
		if i == 0 {
			at = at.Add(250 * time.Millisecond)
		} else {
			at = at.Add(25 * time.Millisecond)
		}
		m.pressDown(at)
		if !m.softActive(at.Add(20 * time.Millisecond)) {
			t.Fatalf("repeat %d should keep soft alive", i)
		}
	}
	if m.repeatDelay != 250*time.Millisecond {
		t.Fatalf("learned repeat delay %v", m.repeatDelay)
	}
	if m.softActive(at.Add(holdGap + time.Millisecond)) {
		t.Fatal("soft should end once repeats stop")
	}
	m.pressDown(at.Add(500 * time.Millisecond))
	if !m.softActive(at.Add(500*time.Millisecond + 250*time.Millisecond + repeatSlack - time.Millisecond)) {
		t.Fatal("the next fresh press should use the learned delay")
	}
}

func TestBackdropCoversTerminal(t *testing.T) {
	b := &backdrop{}
	b.resize(60, 20)
	b.step(time.Second)
	if len(b.lines) != 20 {
		t.Fatalf("%d lines", len(b.lines))
	}
	dots := 0
	for _, l := range b.lines {
		if n := len([]rune(l)); n != 60 {
			t.Fatalf("line width %d", n)
		}
		for _, r := range l {
			if r != ' ' {
				dots++
			}
		}
	}
	if dots == 0 {
		t.Fatal("backdrop is blank")
	}
	b.flare(Red)
	if b.style().GetForeground() == (&backdrop{t: b.t}).style().GetForeground() {
		t.Fatal("flare did not change the tint")
	}
}

func TestViewFitsTerminal(t *testing.T) {
	for _, sz := range [][2]int{{56, 18}, {56, 29}, {66, 18}, {80, 24}, {120, 40}, {200, 70}} {
		m := newModel()
		mm, _ := m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		m = mm.(model)
		if m.scale < 1 {
			t.Fatalf("%v: scale %d", sz, m.scale)
		}
		v := m.View()
		if rows := strings.Count(v, "\n") + 1; rows != sz[1] {
			t.Fatalf("%v: view has %d rows", sz, rows)
		}
		for i, l := range strings.Split(v, "\n") {
			if w := lipgloss.Width(l); w != sz[0] {
				t.Fatalf("%v: row %d is %d wide", sz, i, w)
			}
		}
		if testing.Verbose() && sz[0] == 120 {
			t.Logf("\n%s", v)
		}
	}
}

func TestBlurPausesAndFocusKeepsPaused(t *testing.T) {
	m := newModel()
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = mm.(model)
	for _, ph := range []phase{phaseFalling, phaseClearing, phaseDropping} {
		m.phase = ph
		mm, _ = m.Update(tea.BlurMsg{})
		m = mm.(model)
		if m.phase != phasePaused || !m.blurred {
			t.Fatalf("from %d: phase %d blurred %v after blur", ph, m.phase, m.blurred)
		}
		if strings.Count(m.View(), "\n")+1 != 40 {
			t.Fatal("faded view lost rows")
		}
		mm, _ = m.Update(tea.FocusMsg{})
		m = mm.(model)
		if m.phase != phasePaused || m.blurred {
			t.Fatalf("focus should unfade but stay paused, phase %d blurred %v", m.phase, m.blurred)
		}
		mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
		m = mm.(model)
		if m.phase != ph || cmd == nil {
			t.Fatalf("p should resume phase %d with a tick, got %d cmd %v", ph, m.phase, cmd != nil)
		}
	}
}

func TestFocusReturnCountsDownThenResumes(t *testing.T) {
	m := newModel()
	m.phase = phaseFalling
	mm, _ := m.Update(tea.BlurMsg{})
	m = mm.(model)
	mm, cmd := m.Update(tea.FocusMsg{})
	m = mm.(model)
	if m.phase != phasePaused || m.countdown != countdownFrom || cmd == nil {
		t.Fatalf("focus should start the countdown: phase %d countdown %d", m.phase, m.countdown)
	}
	for i := countdownFrom; i > 0; i-- {
		mm, _ = m.Update(countMsg{m.countID})
		m = mm.(model)
	}
	if m.phase != phaseFalling || m.countdown != 0 || m.autoPaused {
		t.Fatalf("countdown should resume play: phase %d countdown %d auto %v", m.phase, m.countdown, m.autoPaused)
	}

	// A manual pause is not undone by focus.
	mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	m = mm.(model)
	mm, _ = m.Update(tea.BlurMsg{})
	m = mm.(model)
	mm, _ = m.Update(tea.FocusMsg{})
	m = mm.(model)
	if m.phase != phasePaused || m.countdown != 0 {
		t.Fatalf("manual pause should stay paused on focus: phase %d countdown %d", m.phase, m.countdown)
	}

	// A blur during the countdown cancels it.
	m.phase = phaseFalling
	mm, _ = m.Update(tea.BlurMsg{})
	m = mm.(model)
	mm, _ = m.Update(tea.FocusMsg{})
	m = mm.(model)
	id := m.countID
	mm, _ = m.Update(tea.BlurMsg{})
	m = mm.(model)
	mm, _ = m.Update(countMsg{id})
	m = mm.(model)
	if m.phase != phasePaused {
		t.Fatal("a stale countdown step resumed a blurred game")
	}
}

func TestTapSoftDropIsSlowerThanHold(t *testing.T) {
	g := emptyGame()
	g.Pill = &Pill{X: 0, Y: 0, C: [2]Color{Red, Blue}}
	g.Soft = true
	g.Advance(defaultRepeatDelay + repeatSlack)
	if g.Pill.Y > 3 {
		t.Fatalf("a tap window fell %d rows", g.Pill.Y)
	}
	g.Held = true
	y := g.Pill.Y
	g.Advance(softInterval)
	if g.Pill.Y != y+1 {
		t.Fatalf("a held soft drop should fall one row per softInterval, y %d -> %d", y, g.Pill.Y)
	}
}

func TestRepeatPressConfirmsHold(t *testing.T) {
	m := newModel()
	m.phase = phaseFalling
	at := time.Now()
	m.pressDown(at)
	if m.softHeld {
		t.Fatal("a fresh press is not a hold")
	}
	m.pressDown(at.Add(defaultRepeatDelay))
	if !m.softHeld {
		t.Fatal("a repeat press should confirm the hold")
	}
}

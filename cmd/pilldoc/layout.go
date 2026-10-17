package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/demian0311/arcade/internal/canvas"
	"github.com/demian0311/arcade/internal/layout"
)

const (
	titleText = "PILLDOC"
	colPad    = 1 // columns between the side column and the board frame

	// statsW is a padded stat with a seven-digit value; the widest note
	// ("RESUMING IN 3", padded) is the same width.
	statsW = 15
	// statRows is one line per stat plus the note under a blank row.
	statRows = 4 + 2
)

var titleInks = []canvas.Ink{canvas.Ink(Red), canvas.Ink(Yellow), canvas.Ink(Blue)}

func titleWidth(ts int) int  { return layout.TitleWidth(titleText, ts) }
func titleHeight(ts int) int { return layout.TitleHeight(titleText, ts) }

// boardRows and boardCols are the framed board's size.
func boardRows(g geom) int { return g.rows(BoardH) + 2 }
func boardCols(g geom) int { return g.cols(BoardW) + 2 }

// nextW is the padded NEXT line: label, two spaces, a two-cell pill.
func nextW(g geom) int { return 8 + g.cols(2) }

// colWidth is the width of the side column at board geometry g and title scale ts.
func colWidth(g geom, ts int) int {
	return max(titleWidth(ts)+2, nextW(g), statsW)
}

// layoutWidth is the columns the side-by-side layout needs.
func layoutWidth(g geom, ts int) int {
	return colWidth(g, ts) + colPad + boardCols(g)
}

// sideRows is the rows the side column needs: title, next pill and stats,
// each separated by a blank row.
func sideRows(g geom, ts int) int {
	return titleHeight(ts) + 1 + g.rows(1) + 1 + statRows
}

// stats lists the counters and, under them, the note for the current phase.
func (m model) stats() string {
	stat := func(label string, v any) string {
		return layout.Label.Render(label) + " " + layout.Value.Render(fmt.Sprint(v))
	}
	lines := []string{
		stat("LEVEL", m.g.Level), stat("SPEED", m.g.Speed), stat("SCORE", m.g.Score), stat("VIRUS", m.g.Viruses),
	}
	if n := m.note(); n != "" {
		lines = append(lines, "", layout.Note.Render(n))
	}
	return layout.Pad.Render(strings.Join(lines, "\n"))
}

// note is the phase the player has to act on, or "" during play.
func (m model) note() string {
	switch m.phase {
	case phaseMenu:
		return "ENTER START"
	case phasePaused:
		if m.focus.Countdown > 0 {
			return fmt.Sprintf("RESUMING IN %d", m.focus.Countdown)
		}
		return "PAUSED"
	case phaseLevelClear:
		return "LEVEL CLEAR"
	case phaseGameOver:
		return "GAME OVER"
	}
	return ""
}

func (m model) next() string {
	return layout.Pad.Render(lipgloss.JoinHorizontal(lipgloss.Center, layout.Label.Render("NEXT"), "  ", renderNext(m.g.Next, m.geom)))
}

// blocks fills the height with the bottle and hugs its left side with one
// column: title, next pill, then the stats and note. The bottle is centred
// in the terminal when the column fits beside it; otherwise the column and
// bottle are centred together. Blocks are ordered left to right for Compose.
func (m model) blocks() []layout.Block {
	board := layout.NewBlock(layout.Frame.Render(renderBoard(m.g, m.geom)))
	colW := colWidth(m.geom, m.titleScale)
	board.X = (m.width - board.W) / 2
	if board.X < colW+colPad {
		board.X = (m.width-colW-colPad-board.W)/2 + colW + colPad
	}
	board.Y = (m.height - board.H()) / 2

	title := layout.NewBlock(layout.Pad.Render(layout.Title(titleText, m.titleScale, titleInks, palette)))
	next := layout.NewBlock(m.next())
	stats := layout.NewBlock(m.stats())

	edge := board.X - colPad
	title.X, title.Y = edge-title.W, board.Y
	next.X, next.Y = edge-next.W, title.Y+title.H()+1
	stats.X, stats.Y = edge-stats.W, next.Y+next.H()+1
	return []layout.Block{title, next, stats, board}
}

func (m model) View() string {
	if m.width == 0 {
		return ""
	}
	if m.geom.n == 0 {
		least := geoms[0]
		need := fmt.Sprintf("pilldoc needs %dx%d, have %dx%d", layoutWidth(least, 0), boardRows(least), m.width, m.height)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, need)
	}
	v := layout.Compose(m.bg, m.width, m.height, m.blocks())
	if m.focus.Blurred {
		return layout.Fade(v)
	}
	return v
}

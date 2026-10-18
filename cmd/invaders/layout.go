package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/demian0311/arcade/internal/canvas"
	"github.com/demian0311/arcade/internal/layout"
)

const (
	titleText = "INVADERS"
	colPad    = 1 // columns between the side column and the field frame

	// statsW is a padded stat with a seven-digit value; the widest note
	// ("RESUMING IN 3", padded) is the same width.
	statsW = 15
	// statRows is one line per stat plus the note under a blank row.
	statRows = 3 + 2
)

var titleInks = []canvas.Ink{inkTop, inkMid, inkBottom}

func titleWidth(ts int) int  { return layout.TitleWidth(titleText, ts) }
func titleHeight(ts int) int { return layout.TitleHeight(titleText, ts) }

// boardRows and boardCols are the framed field's size at scale s.
func boardRows(s int) int { return FieldH/4*s + 2 }
func boardCols(s int) int { return FieldW/2*s + 2 }

// colWidth is the width of the side column at title scale ts.
func colWidth(ts int) int { return max(titleWidth(ts)+2, statsW) }

// layoutWidth is the columns the side-by-side layout needs.
func layoutWidth(s, ts int) int { return colWidth(ts) + colPad + boardCols(s) }

// sideRows is the rows the side column needs: title and stats, separated by a blank row.
func sideRows(ts int) int { return titleHeight(ts) + 1 + statRows }

// stats lists the counters and, under them, the note for the current phase.
func (m model) stats() string {
	stat := func(label string, v any) string {
		return layout.Label.Render(label) + " " + layout.Value.Render(fmt.Sprint(v))
	}
	lines := []string{stat("WAVE", m.g.Wave), stat("SCORE", m.g.Score), stat("LIVES", m.g.Lives)}
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
		return "SPACE RESUME"
	case phaseWaveClear:
		return "WAVE CLEAR"
	case phaseGameOver:
		return "GAME OVER"
	}
	return ""
}

// blocks fills the height with the field and hugs its left side with one
// column: title, then the stats and note. The field is centred in the
// terminal when the column fits beside it; otherwise the column and field
// are centred together. Blocks are ordered left to right for Compose.
func (m model) blocks() []layout.Block {
	board := layout.NewBlock(layout.Frame.Render(renderBoard(m.g, m.scale)))
	colW := colWidth(m.titleScale)
	board.X = (m.width - board.W) / 2
	if board.X < colW+colPad {
		board.X = (m.width-colW-colPad-board.W)/2 + colW + colPad
	}
	board.Y = (m.height - board.H()) / 2

	title := layout.NewBlock(layout.Pad.Render(layout.Title(titleText, m.titleScale, titleInks, palette)))
	stats := layout.NewBlock(m.stats())

	edge := board.X - colPad
	title.X, title.Y = edge-title.W, board.Y
	stats.X, stats.Y = edge-stats.W, title.Y+title.H()+1
	return []layout.Block{title, stats, board}
}

func (m model) View() string {
	if m.width == 0 {
		return ""
	}
	if m.scale < 1 {
		need := fmt.Sprintf("invaders needs %dx%d, have %dx%d", layoutWidth(1, 0), boardRows(1), m.width, m.height)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, need)
	}
	v := layout.Compose(m.bg, m.width, m.height, m.blocks())
	if m.focus.Blurred {
		return layout.Fade(v)
	}
	return v
}

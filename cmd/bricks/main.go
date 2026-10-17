// bricks is a terminal Breakout built on Bubble Tea, drawn in braille.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if _, err := tea.NewProgram(newModel(), tea.WithAltScreen(), tea.WithReportFocus()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

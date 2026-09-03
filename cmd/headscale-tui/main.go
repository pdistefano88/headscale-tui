package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/pdistefano88/headscale-tui/internal/ui"
)

func main() {
	program := tea.NewProgram(ui.NewApp())
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

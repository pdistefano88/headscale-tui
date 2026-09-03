package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

var items = []string{"Users", "Nodes", "API Keys"}

// Menu manages navigation from the application's main screen.
type Menu struct {
	cursor int
}

func NewMenu() Menu {
	return Menu{}
}

func (m Menu) Update(message tea.Msg) (Menu, tea.Cmd) {
	if message, ok := message.(tea.KeyPressMsg); ok {
		switch message.String() {
		case "ctrl+c", "q":
			return m, func() tea.Msg { return quitMsg{} }
		case "enter":
			if items[m.cursor] == "Nodes" {
				return m, func() tea.Msg { return openNodesMsg{} }
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(items)-1 {
				m.cursor++
			}
		}
	}

	return m, nil
}

func (m Menu) View() string {
	lines := []string{"Headscale TUI", ""}
	for index, item := range items {
		prefix := "  "
		if index == m.cursor {
			prefix = "> "
		}
		lines = append(lines, prefix+item)
	}

	lines = append(lines, "", "up/down or j/k: navigate  enter: select  q: quit")
	return strings.Join(lines, "\n")
}

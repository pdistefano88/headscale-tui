package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestMenuNavigation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		keys   []string
		cursor int
	}{
		{name: "moves down", keys: []string{"down"}, cursor: 1},
		{name: "moves with vim keys", keys: []string{"j", "j", "k"}, cursor: 1},
		{name: "stops at first item", keys: []string{"up", "k"}, cursor: 0},
		{name: "stops at final item", keys: []string{"down", "down", "down"}, cursor: 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			menu := NewMenu()
			for _, key := range test.keys {
				model, _ := menu.Update(tea.KeyPressMsg{Text: key})
				menu = model.(Menu)
			}

			if menu.cursor != test.cursor {
				t.Fatalf("cursor = %d, want %d", menu.cursor, test.cursor)
			}
		})
	}
}

func TestMenuRendersSelection(t *testing.T) {
	t.Parallel()

	menu := NewMenu()
	model, _ := menu.Update(tea.KeyPressMsg{Text: "j"})
	menu = model.(Menu)

	view := menu.View()
	if !view.AltScreen {
		t.Fatal("menu did not request the alternate screen")
	}

	if got := view.Content; !strings.Contains(got, "> Nodes") {
		t.Fatalf("menu did not render selected node item: %q", got)
	}
}

func TestMenuLoadsNodes(t *testing.T) {
	t.Parallel()

	menu := NewMenu()
	menu.loadNodes = func() tea.Cmd {
		return func() tea.Msg {
			return nodesLoadedMsg{nodes: []node{{ID: 1, Name: "alice-laptop", Online: true}}}
		}
	}

	model, command := menu.Update(tea.KeyPressMsg{Text: "j"})
	menu = model.(Menu)
	model, command = menu.Update(tea.KeyPressMsg{Text: "enter"})
	menu = model.(Menu)
	if command == nil {
		t.Fatal("selecting Nodes did not start a load command")
	}
	if menu.screen != nodesScreen || !menu.loading {
		t.Fatalf("selecting Nodes did not show its loading screen: %+v", menu)
	}

	model, _ = menu.Update(command())
	menu = model.(Menu)
	if menu.loading || len(menu.nodes) != 1 {
		t.Fatalf("node result was not applied: %+v", menu)
	}
	if got := menu.View().Content; !strings.Contains(got, "alice-laptop") {
		t.Fatalf("node list was not rendered: %q", got)
	}
}

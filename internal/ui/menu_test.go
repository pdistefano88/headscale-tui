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
		{name: "stops at final item", keys: []string{"down", "down", "down", "down"}, cursor: 3},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			menu := NewMenu()
			for _, key := range test.keys {
				menu, _ = menu.Update(tea.KeyPressMsg{Text: key})
			}

			if got := menu.list.Index(); got != test.cursor {
				t.Fatalf("selected index = %d, want %d", got, test.cursor)
			}
		})
	}
}

func TestMenuRendersSelection(t *testing.T) {
	t.Parallel()

	menu := NewMenu()
	menu, _ = menu.Update(tea.KeyPressMsg{Text: "j"})

	if got := menu.View(); !strings.Contains(got, "Nodes") {
		t.Fatalf("menu did not render selected node item: %q", got)
	}
	if got := menu.View(); !strings.Contains(got, "API Keys") {
		t.Fatalf("menu did not render every item: %q", got)
	}
	if got := menu.View(); !strings.Contains(got, "Preauth Keys") {
		t.Fatalf("menu did not render preauth keys: %q", got)
	}
}

func TestMenuDoesNotQuitOnV(t *testing.T) {
	t.Parallel()

	menu := NewMenu()
	_, command := menu.Update(tea.KeyPressMsg{Text: "v"})
	if command != nil {
		t.Fatal("v unexpectedly returned a command")
	}
}

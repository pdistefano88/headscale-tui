package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestNodesGroupByOwner(t *testing.T) {
	t.Parallel()

	aliceNode := Node{ID: 2, Name: "alice-laptop", Online: true}
	aliceNode.User.Name = "alice"
	bobNode := Node{ID: 3, Name: "bob-phone", Online: false}
	bobNode.User.Name = "bob"
	taggedNode := Node{ID: 1, Name: "alice-server", Tags: []string{"tag:server"}, Online: true}
	taggedNode.User.Name = "tagged-devices"

	nodes := NewNodes()
	nodes.nodes = []Node{taggedNode, bobNode, aliceNode}

	view := nodes.View()
	for _, text := range []string{
		"alice\n  ID",
		"bob\n  ID",
		"Tagged devices\n  ID",
		"tag:server",
		"\x1b[32m●\x1b[0m",
		"\x1b[31m●\x1b[0m",
	} {
		if !strings.Contains(view, text) {
			t.Fatalf("node grouping is missing %q: %q", text, view)
		}
	}
	if strings.Index(view, "alice\n  ID") > strings.Index(view, "bob\n  ID") ||
		strings.Index(view, "bob\n  ID") > strings.Index(view, "Tagged devices\n  ID") {
		t.Fatalf("node groups are not ordered by owner followed by tagged devices: %q", view)
	}
}

func TestNodeDeleteConfirmation(t *testing.T) {
	t.Parallel()

	aliceNode := Node{ID: 2, Name: "alice-laptop"}
	aliceNode.User.Name = "alice"
	bobNode := Node{ID: 3, Name: "bob-phone"}
	bobNode.User.Name = "bob"
	taggedNode := Node{ID: 1, Name: "alice-server"}
	taggedNode.User.Name = "tagged-devices"

	var deletedID uint64
	nodes := NewNodes()
	nodes.nodes = []Node{taggedNode, bobNode, aliceNode}
	nodes.deleteNode = func(id uint64) tea.Cmd {
		return func() tea.Msg {
			deletedID = id
			return nodeDeletedMsg{}
		}
	}

	nodes, _ = nodes.Update(tea.KeyPressMsg{Text: "j"})
	nodes, command := nodes.Update(tea.KeyPressMsg{Text: "d"})
	if command != nil || !nodes.confirming {
		t.Fatalf("delete did not show a confirmation: %+v", nodes)
	}
	if got := nodes.View(); !strings.Contains(got, `Delete node "bob-phone" (ID 3)?`) {
		t.Fatalf("confirmation did not name the selected node: %q", got)
	}

	nodes, command = nodes.Update(tea.KeyPressMsg{Text: "y"})
	if command == nil || nodes.confirming || !nodes.deleting {
		t.Fatalf("confirmation did not start deletion: %+v", nodes)
	}
	if _, ok := command().(nodeDeletedMsg); !ok || deletedID != 3 {
		t.Fatalf("delete command targeted node %d, want 3", deletedID)
	}
}

func TestNodeTableRowsHaveEqualDisplayWidths(t *testing.T) {
	t.Parallel()

	rows := strings.Split(nodeSection("alice", []Node{
		{ID: 1, Name: "online", Online: true},
		{ID: 2, Name: "offline", Online: false},
	}, 1), "\n")

	for _, row := range rows[1:] {
		if got, want := ansi.StringWidth(row), 102; got != want {
			t.Fatalf("row width = %d, want %d: %q", got, want, row)
		}
	}
}

package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNodesTableOrdersByOwner(t *testing.T) {
	t.Parallel()

	aliceNode := Node{ID: 2, Name: "alice-laptop", Online: true}
	aliceNode.User.Name = "alice"
	bobNode := Node{ID: 3, Name: "bob-phone", Online: false}
	bobNode.User.Name = "bob"
	taggedNode := Node{ID: 1, Name: "alice-server", Tags: []string{"tag:server"}, Online: true}
	taggedNode.User.Name = "tagged-devices"

	nodes := NewNodes()
	nodes.nodes = []Node{taggedNode, bobNode, aliceNode}
	nodes.setTableRows()

	view := nodes.View()
	for _, text := range []string{
		"ID",
		"NAME",
		"alice\n",
		"\n\nbob\n",
		"\n\nTagged devices\n",
		"tag:server",
		"\x1b[32m● online\x1b[0m",
		"\x1b[31m● offline\x1b[0m",
	} {
		if !strings.Contains(view, text) {
			t.Fatalf("node table is missing %q: %q", text, view)
		}
	}
	if strings.Index(view, "alice-laptop") > strings.Index(view, "bob-phone") ||
		strings.Index(view, "bob-phone") > strings.Index(view, "alice-server") {
		t.Fatalf("node rows are not ordered by owner followed by tagged devices: %q", view)
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
	nodes.setTableRows()
	nodes.deleteNode = func(id uint64) tea.Cmd {
		return func() tea.Msg {
			deletedID = id
			return nodeDeletedMsg{}
		}
	}

	nodes, _ = nodes.Update(tea.KeyPressMsg{Code: tea.KeyTab})
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

func TestNodesTableUsesSelectedRow(t *testing.T) {
	t.Parallel()

	nodes := NewNodes()
	nodes.nodes = []Node{
		{ID: 1, Name: "online", Online: true},
		{ID: 2, Name: "offline", Online: false},
	}
	nodes.setTableRows()
	nodes, _ = nodes.Update(tea.KeyPressMsg{Text: "j"})

	if got := nodes.tables[0].table.SelectedRow()[1]; got != "offline" {
		t.Fatalf("selected row name = %q, want offline", got)
	}
	if node, ok := nodes.selectedNode(); !ok || node.ID != 2 {
		t.Fatalf("selected node = %+v, %t; want node 2", node, ok)
	}
}

func TestNodesTabCyclesOwnerGroups(t *testing.T) {
	t.Parallel()

	aliceNode := Node{ID: 1, Name: "alice-laptop"}
	aliceNode.User.Name = "alice"
	bobNode := Node{ID: 2, Name: "bob-phone"}
	bobNode.User.Name = "bob"

	nodes := NewNodes()
	nodes.nodes = []Node{aliceNode, bobNode}
	nodes.setTableRows()
	nodes, _ = nodes.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})

	if nodes.activeTable != 1 {
		t.Fatalf("active table = %d, want 1", nodes.activeTable)
	}
	if node, ok := nodes.selectedNode(); !ok || node.ID != 2 {
		t.Fatalf("selected node = %+v, %t; want node 2", node, ok)
	}
}

func TestNodesNavigationCrossesOwnerGroups(t *testing.T) {
	t.Parallel()

	aliceNode := Node{ID: 1, Name: "alice-laptop"}
	aliceNode.User.Name = "alice"
	bobNode := Node{ID: 2, Name: "bob-phone"}
	bobNode.User.Name = "bob"

	nodes := NewNodes()
	nodes.nodes = []Node{aliceNode, bobNode}
	nodes.setTableRows()
	nodes, _ = nodes.Update(tea.KeyPressMsg{Text: "j"})

	if nodes.activeTable != 1 {
		t.Fatalf("active table = %d, want 1", nodes.activeTable)
	}
	if node, ok := nodes.selectedNode(); !ok || node.ID != 2 {
		t.Fatalf("selected node = %+v, %t; want node 2", node, ok)
	}
}

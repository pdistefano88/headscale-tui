package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

type Node struct {
	ID          uint64   `json:"id"`
	Name        string   `json:"name"`
	IPAddresses []string `json:"ip_addresses"`
	Tags        []string `json:"tags"`
	Online      bool     `json:"online"`
	User        struct {
		Name string `json:"name"`
	} `json:"user"`
}

type nodesLoadedMsg struct {
	nodes []Node
	err   error
}

type nodeDeletedMsg struct {
	err error
}

type nodeGroup struct {
	heading string
	nodes   []Node
}

var nodeColumnWidths = [...]int{5, 23, 16, 6, 38}

// Nodes manages listing and deleting Headscale nodes.
type Nodes struct {
	cursor       int
	loading      bool
	deleting     bool
	confirming   bool
	nodeToDelete Node
	nodes        []Node
	err          error
	loadNodes    func() tea.Cmd
	deleteNode   func(uint64) tea.Cmd
}

func NewNodes() Nodes {
	return Nodes{loadNodes: listNodes, deleteNode: deleteNode}
}

func (m Nodes) Load() (Nodes, tea.Cmd) {
	m.loading = true
	m.err = nil
	return m, m.loadNodes()
}

func (m Nodes) Update(message tea.Msg) (Nodes, tea.Cmd) {
	switch message := message.(type) {
	case nodesLoadedMsg:
		m.loading = false
		m.nodes = message.nodes
		m.err = message.err
		m.clampCursor()
	case nodeDeletedMsg:
		m.deleting = false
		if message.err != nil {
			m.err = message.err
			return m, nil
		}

		m.loading = true
		return m, m.loadNodes()
	case tea.KeyPressMsg:
		if m.confirming {
			switch message.String() {
			case "y":
				m.confirming = false
				m.deleting = true
				m.err = nil
				return m, m.deleteNode(m.nodeToDelete.ID)
			case "n", "esc", "b", "q":
				m.confirming = false
			}

			return m, nil
		}

		switch message.String() {
		case "ctrl+c", "q":
			return m, func() tea.Msg { return quitMsg{} }
		case "esc", "b":
			return m, func() tea.Msg { return backToMenuMsg{} }
		case "r":
			if !m.loading && !m.deleting {
				m.loading = true
				return m, m.loadNodes()
			}
		case "d":
			if !m.loading && !m.deleting {
				if node, ok := m.selectedNode(); ok {
					m.confirming = true
					m.nodeToDelete = node
				}
			}
		case "up", "k":
			if !m.loading && !m.deleting && m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if !m.loading && !m.deleting && m.cursor < m.nodeCount()-1 {
				m.cursor++
			}
		}
	}

	return m, nil
}

func (m Nodes) View() string {
	lines := []string{"Nodes", ""}

	switch {
	case m.confirming:
		lines = append(lines,
			fmt.Sprintf("Delete node %q (ID %d)?", safeText(m.nodeToDelete.Name), m.nodeToDelete.ID),
			"This action cannot be undone.",
			"",
			"y: delete  n: cancel",
		)
		return strings.Join(lines, "\n")
	case m.deleting:
		lines = append(lines, "Deleting node "+safeText(m.nodeToDelete.Name)+"...")
	case m.loading:
		lines = append(lines, "Loading nodes...")
	case m.err != nil:
		lines = append(lines, "Error: "+safeText(m.err.Error()))
	case len(m.nodes) == 0:
		lines = append(lines, "No nodes found.")
	default:
		var sections []string
		selectedNode, selected := m.selectedNode()
		for _, group := range m.nodeGroups() {
			selectedID := uint64(0)
			if selected {
				selectedID = selectedNode.ID
			}
			sections = append(sections, nodeSection(group.heading, group.nodes, selectedID))
		}
		lines = append(lines, strings.Join(sections, "\n\n"))
	}

	lines = append(lines, "", "up/down or j/k: select  d: delete  r: refresh  b: back  q: quit")
	return strings.Join(lines, "\n")
}

func (m Nodes) nodeGroups() []nodeGroup {
	owners := make(map[string][]Node)
	var taggedNodes []Node
	for _, node := range m.nodes {
		if node.User.Name == "tagged-devices" {
			taggedNodes = append(taggedNodes, node)
			continue
		}

		owners[node.User.Name] = append(owners[node.User.Name], node)
	}

	ownerNames := make([]string, 0, len(owners))
	for owner := range owners {
		ownerNames = append(ownerNames, owner)
	}
	sort.Strings(ownerNames)

	groups := make([]nodeGroup, 0, len(ownerNames)+1)
	for _, owner := range ownerNames {
		groups = append(groups, nodeGroup{heading: owner, nodes: owners[owner]})
	}
	if len(taggedNodes) > 0 {
		groups = append(groups, nodeGroup{heading: "Tagged devices", nodes: taggedNodes})
	}

	return groups
}

func (m Nodes) nodeCount() int {
	count := 0
	for _, group := range m.nodeGroups() {
		count += len(group.nodes)
	}
	return count
}

func (m Nodes) selectedNode() (Node, bool) {
	index := 0
	for _, group := range m.nodeGroups() {
		for _, node := range group.nodes {
			if index == m.cursor {
				return node, true
			}
			index++
		}
	}

	return Node{}, false
}

func (m *Nodes) clampCursor() {
	if m.cursor >= m.nodeCount() {
		m.cursor = 0
	}
}

func nodeSection(owner string, nodes []Node, selectedID uint64) string {
	lines := []string{
		safeText(owner),
		"  " + nodeTableRow([]string{"ID", "NAME", "TAGS", "STATUS", "ADDRESSES"}),
		"  " + nodeTableRule(),
	}
	for _, node := range nodes {
		status := "\x1b[31m●\x1b[0m"
		if node.Online {
			status = "\x1b[32m●\x1b[0m"
		}

		prefix := "  "
		if node.ID == selectedID {
			prefix = "> "
		}

		lines = append(lines, fmt.Sprintf(
			"%s%s",
			prefix,
			nodeTableRow([]string{
				fmt.Sprint(node.ID),
				truncate(safeText(node.Name), nodeColumnWidths[1]),
				truncate(safeText(strings.Join(node.Tags, ", ")), nodeColumnWidths[2]),
				status,
				truncate(safeText(strings.Join(node.IPAddresses, ", ")), nodeColumnWidths[4]),
			}),
		))
	}

	return strings.Join(lines, "\n")
}

func nodeTableRow(values []string) string {
	cells := make([]string, len(values))
	for index, value := range values {
		cells[index] = padCell(value, nodeColumnWidths[index])
	}

	return strings.Join(cells, " | ")
}

func nodeTableRule() string {
	cells := make([]string, len(nodeColumnWidths))
	for index, width := range nodeColumnWidths {
		cells[index] = strings.Repeat("-", width)
	}

	return strings.Join(cells, "-+-")
}

func padCell(value string, width int) string {
	padding := width - ansi.StringWidth(value)
	if padding <= 0 {
		return value
	}

	return value + strings.Repeat(" ", padding)
}

func deleteNode(id uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := exec.CommandContext(ctx, "headscale", "nodes", "delete", "--identifier", fmt.Sprint(id), "--force").Run()
		if err != nil {
			return nodeDeletedMsg{err: fmt.Errorf("deleting node %d: %w", id, err)}
		}

		return nodeDeletedMsg{}
	}
}

func listNodes() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		output, err := exec.CommandContext(ctx, "headscale", "nodes", "list", "--output", "json").Output()
		if err != nil {
			return nodesLoadedMsg{err: fmt.Errorf("running headscale nodes list: %w", err)}
		}

		var nodes []Node
		if err := json.Unmarshal(output, &nodes); err != nil {
			return nodesLoadedMsg{err: fmt.Errorf("decoding node list: %w", err)}
		}

		return nodesLoadedMsg{nodes: nodes}
	}
}

func safeText(value string) string {
	return strings.Map(func(character rune) rune {
		if unicode.IsControl(character) {
			return -1
		}
		return character
	}, value)
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}

	return value[:limit-3] + "..."
}

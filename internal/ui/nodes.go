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

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

type nodeTable struct {
	heading string
	nodes   []Node
	table   table.Model
}

const (
	nodeTableHeight = 8
	nodeTableWidth  = 101
)

var nodeColumns = []table.Column{
	{Title: "ID", Width: 5},
	{Title: "NAME", Width: 23},
	{Title: "TAGS", Width: 16},
	{Title: "STATUS", Width: 9},
	{Title: "ADDRESSES", Width: 38},
}

// Nodes manages listing and deleting Headscale nodes.
type Nodes struct {
	loading      bool
	deleting     bool
	confirming   bool
	nodeToDelete Node
	nodes        []Node
	tables       []nodeTable
	activeTable  int
	err          error
	loadNodes    func() tea.Cmd
	deleteNode   func(uint64) tea.Cmd
}

func NewNodes() Nodes {
	return Nodes{
		loadNodes:  listNodes,
		deleteNode: deleteNode,
	}
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
		m.setTableRows()
	case nodeDeletedMsg:
		m.deleting = false
		if message.err != nil {
			m.err = message.err
			return m, nil
		}

		m.loading = true
		return m, m.loadNodes()
	case tea.WindowSizeMsg:
		for index := range m.tables {
			m.tables[index].table.SetWidth(max(nodeTableWidth, message.Width))
		}
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
		case "tab":
			m.selectTable(1)
			m.syncTableStyles()
			return m, nil
		case "shift+tab":
			m.selectTable(-1)
			m.syncTableStyles()
			return m, nil
		case "up", "k":
			m.moveUp()
			return m, nil
		case "down", "j":
			m.moveDown()
			return m, nil
		}

		if !m.loading && !m.deleting && len(m.tables) > 0 {
			var command tea.Cmd
			m.tables[m.activeTable].table, command = m.tables[m.activeTable].table.Update(message)
			return m, command
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
		sections := make([]string, 0, len(m.tables))
		for _, nodeTable := range m.tables {
			sections = append(sections, safeText(nodeTable.heading)+"\n"+nodeTable.table.View())
		}
		lines = append(lines, strings.Join(sections, "\n\n"))
	}

	lines = append(lines, "", "up/down or j/k: select  tab: switch owner  d: delete  r: refresh  b: back  q: quit")
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

func (m Nodes) selectedNode() (Node, bool) {
	if m.activeTable < 0 || m.activeTable >= len(m.tables) {
		return Node{}, false
	}

	group := m.tables[m.activeTable]
	index := group.table.Cursor()
	if index >= 0 && index < len(group.nodes) {
		return group.nodes[index], true
	}

	return Node{}, false
}

func (m *Nodes) setTableRows() {
	groups := m.nodeGroups()
	m.tables = make([]nodeTable, 0, len(groups))
	m.activeTable = 0
	for index, group := range groups {
		rows := make([]table.Row, 0, len(group.nodes))
		for _, node := range group.nodes {
			status := "\x1b[31m● offline\x1b[0m"
			if node.Online {
				status = "\x1b[32m● online\x1b[0m"
			}

			rows = append(rows, table.Row{
				fmt.Sprint(node.ID),
				safeText(node.Name),
				safeText(strings.Join(node.Tags, ", ")),
				status,
				safeText(strings.Join(node.IPAddresses, ", ")),
			})
		}

		m.tables = append(m.tables, nodeTable{
			heading: group.heading,
			nodes:   group.nodes,
			table: table.New(
				table.WithColumns(nodeColumns),
				table.WithRows(rows),
				table.WithFocused(index == 0),
				table.WithHeight(min(nodeTableHeight, len(rows)+1)),
				table.WithWidth(nodeTableWidth),
			),
		})
	}
	m.syncTableStyles()
}

func (m *Nodes) selectTable(direction int) {
	if len(m.tables) < 2 {
		return
	}

	m.tables[m.activeTable].table.Blur()
	m.activeTable = (m.activeTable + direction + len(m.tables)) % len(m.tables)
	m.tables[m.activeTable].table.Focus()
}

func (m *Nodes) moveUp() {
	if len(m.tables) == 0 {
		return
	}

	current := &m.tables[m.activeTable].table
	if current.Cursor() == 0 && m.activeTable > 0 {
		m.selectTable(-1)
		m.tables[m.activeTable].table.GotoBottom()
	} else {
		current.MoveUp(1)
	}

	m.syncTableStyles()
}

func (m *Nodes) moveDown() {
	if len(m.tables) == 0 {
		return
	}

	current := &m.tables[m.activeTable]
	if current.table.Cursor() == len(current.nodes)-1 && m.activeTable < len(m.tables)-1 {
		m.selectTable(1)
		m.tables[m.activeTable].table.GotoTop()
	} else {
		current.table.MoveDown(1)
	}

	m.syncTableStyles()
}

func (m *Nodes) syncTableStyles() {
	for tableIndex := range m.tables {
		styles := table.DefaultStyles()
		if tableIndex != m.activeTable {
			styles.Selected = lipgloss.NewStyle()
		}
		m.tables[tableIndex].table.SetStyles(styles)
	}
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

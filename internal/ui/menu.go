package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
)

var items = []string{"Users", "Nodes", "API Keys"}

const (
	menuScreen = iota
	nodesScreen
)

type node struct {
	ID          uint64   `json:"id"`
	Name        string   `json:"name"`
	IPAddresses []string `json:"ip_addresses"`
	Online      bool     `json:"online"`
	User        struct {
		Name string `json:"name"`
	} `json:"user"`
}

type nodesLoadedMsg struct {
	nodes []node
	err   error
}

// Menu is the initial application screen. Headscale commands run outside the
// update loop and return their result as Bubble Tea messages.
type Menu struct {
	cursor    int
	width     int
	height    int
	screen    int
	loading   bool
	nodes     []node
	err       error
	loadNodes func() tea.Cmd
}

func NewMenu() Menu {
	return Menu{loadNodes: listNodes}
}

func (m Menu) Init() tea.Cmd {
	return nil
}

func (m Menu) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width = message.Width
		m.height = message.Height
	case nodesLoadedMsg:
		m.loading = false
		m.nodes = message.nodes
		m.err = message.err
	case tea.KeyPressMsg:
		switch message.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc", "b":
			if m.screen == nodesScreen {
				m.screen = menuScreen
			}
		case "r":
			if m.screen == nodesScreen && !m.loading {
				m.loading = true
				return m, m.loadNodes()
			}
		case "enter":
			if m.screen == menuScreen && items[m.cursor] == "Nodes" {
				m.screen = nodesScreen
				m.loading = true
				m.err = nil
				return m, m.loadNodes()
			}
		case "up", "k":
			if m.screen == menuScreen && m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.screen == menuScreen && m.cursor < len(items)-1 {
				m.cursor++
			}
		}
	}

	return m, nil
}

func (m Menu) View() tea.View {
	content := m.menuView()
	if m.screen == nodesScreen {
		content = m.nodesView()
	}

	if m.width > 0 {
		content = center(content, m.width)
	}

	view := tea.NewView(content)
	view.AltScreen = true

	return view
}

func (m Menu) menuView() string {
	lines := []string{
		"Headscale TUI",
		"",
	}

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

func (m Menu) nodesView() string {
	lines := []string{"Nodes", ""}

	switch {
	case m.loading:
		lines = append(lines, "Loading nodes...")
	case m.err != nil:
		lines = append(lines, "Unable to list nodes: "+safeText(m.err.Error()))
	case len(m.nodes) == 0:
		lines = append(lines, "No nodes found.")
	default:
		lines = append(lines, "ID    NAME                    USER          STATUS   ADDRESSES")
		for _, node := range m.nodes {
			status := "offline"
			if node.Online {
				status = "online"
			}

			lines = append(lines, fmt.Sprintf(
				"%-5d %-23s %-13s %-8s %s",
				node.ID,
				truncate(safeText(node.Name), 23),
				truncate(safeText(node.User.Name), 13),
				status,
				truncate(safeText(strings.Join(node.IPAddresses, ", ")), 38),
			))
		}
	}

	lines = append(lines, "", "r: refresh  b: back  q: quit")
	return strings.Join(lines, "\n")
}

func listNodes() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		output, err := exec.CommandContext(ctx, "headscale", "nodes", "list", "--output", "json").Output()
		if err != nil {
			return nodesLoadedMsg{err: fmt.Errorf("running headscale nodes list: %w", err)}
		}

		var nodes []node
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

func center(content string, width int) string {
	lines := strings.Split(content, "\n")
	for index, line := range lines {
		padding := (width - len(line)) / 2
		if padding > 0 {
			lines[index] = strings.Repeat(" ", padding) + line
		}
	}

	return strings.Join(lines, "\n")
}

package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	menuScreen = iota
	nodesScreen
)

type openNodesMsg struct{}
type backToMenuMsg struct{}
type quitMsg struct{}

// App owns screen routing and shared presentation state.
type App struct {
	width  int
	screen int
	menu   Menu
	nodes  Nodes
}

func NewApp() App {
	return App{menu: NewMenu(), nodes: NewNodes()}
}

func (a App) Init() tea.Cmd {
	return nil
}

func (a App) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		a.width = message.Width
	case openNodesMsg:
		a.screen = nodesScreen
		var command tea.Cmd
		a.nodes, command = a.nodes.Load()
		return a, command
	case backToMenuMsg:
		a.screen = menuScreen
		return a, nil
	case quitMsg:
		return a, tea.Quit
	}

	if a.screen == nodesScreen {
		var command tea.Cmd
		a.nodes, command = a.nodes.Update(message)
		return a, command
	}

	var command tea.Cmd
	a.menu, command = a.menu.Update(message)
	return a, command
}

func (a App) View() tea.View {
	content := a.menu.View()
	if a.screen == nodesScreen {
		content = a.nodes.View()
	}

	if a.width > 0 {
		content = center(content, a.width)
	}

	view := tea.NewView(content)
	view.AltScreen = true
	return view
}

func center(content string, width int) string {
	lines := strings.Split(content, "\n")
	contentWidth := 0
	for _, line := range lines {
		contentWidth = max(contentWidth, ansi.StringWidth(line))
	}

	padding := (width - contentWidth) / 2
	if padding <= 0 {
		return content
	}

	for index, line := range lines {
		lines[index] = strings.Repeat(" ", padding) + line
	}

	return strings.Join(lines, "\n")
}

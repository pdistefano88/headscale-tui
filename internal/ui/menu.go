package ui

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

type menuItem struct {
	title       string
	description string
}

func (i menuItem) Title() string       { return i.title }
func (i menuItem) Description() string { return i.description }
func (i menuItem) FilterValue() string { return i.title }

var items = []list.Item{
	menuItem{title: "Users", description: "Manage Headscale users"},
	menuItem{title: "Nodes", description: "List and manage devices"},
	menuItem{title: "API Keys", description: "Manage API keys"},
}

// Menu manages navigation from the application's main screen.
type Menu struct {
	list list.Model
}

func NewMenu() Menu {
	delegate := list.NewDefaultDelegate()
	menu := list.New(items, delegate, 40, len(items)*(delegate.Height()+delegate.Spacing())+4)
	menu.Title = "Headscale TUI"
	menu.SetShowFilter(false)
	menu.SetShowPagination(false)
	menu.SetShowStatusBar(false)
	menu.DisableQuitKeybindings()

	return Menu{list: menu}
}

func (m Menu) Update(message tea.Msg) (Menu, tea.Cmd) {
	if message, ok := message.(tea.WindowSizeMsg); ok {
		m.list.SetSize(min(40, message.Width), message.Height)
	}

	if message, ok := message.(tea.KeyPressMsg); ok {
		switch message.String() {
		case "ctrl+c", "q":
			return m, func() tea.Msg { return quitMsg{} }
		case "enter":
			item, ok := m.list.SelectedItem().(menuItem)
			if ok {
				switch item.title {
				case "Users":
					return m, func() tea.Msg { return openUsersMsg{} }
				case "Nodes":
					return m, func() tea.Msg { return openNodesMsg{} }
				}
			}
		}
	}

	var command tea.Cmd
	m.list, command = m.list.Update(message)
	return m, command
}

func (m Menu) View() string {
	return m.list.View()
}

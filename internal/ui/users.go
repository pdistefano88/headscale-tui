package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type User struct {
	ID    uint64 `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type usersLoadedMsg struct {
	users []User
	err   error
}

type userDeletedMsg struct {
	err      error
	hasNodes bool
}

type userDeleteCheckedMsg struct {
	hasNodes bool
	err      error
}

type userDeleteAlertExpiredMsg struct {
	generation uint
}

type userCreatedMsg struct {
	err error
}

var userColumns = []table.Column{
	{Title: "ID", Width: 5},
	{Title: "NAME", Width: 30},
	{Title: "EMAIL", Width: 30},
}

var userDeleteAlertStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("9")).
	Padding(0, 1)

const userDeleteAlertContent = "Cannot delete user\n\nThis user still owns one or more nodes.\nDelete or transfer their nodes before deleting the user."

// Users manages listing, creating, and deleting Headscale users.
type Users struct {
	loading                   bool
	creating                  bool
	creatingUser              bool
	checkingDelete            bool
	checkingAfterConfirmation bool
	deleting                  bool
	confirming                bool
	alerting                  bool
	alertGeneration           uint
	userToDelete              User
	users                     []User
	table                     table.Model
	nameInput                 textinput.Model
	emailInput                textinput.Model
	err                       error
	loadUsers                 func() tea.Cmd
	createUser                func(string, string) tea.Cmd
	checkUserNodes            func(string) tea.Cmd
	deleteUser                func(uint64) tea.Cmd
}

func NewUsers() Users {
	nameInput := textinput.New()
	nameInput.Prompt = "User name: "
	nameInput.Placeholder = "alice"
	nameInput.CharLimit = 255
	nameInput.SetWidth(30)
	emailInput := textinput.New()
	emailInput.Prompt = "Email (optional): "
	emailInput.Placeholder = "alice@example.com"
	emailInput.CharLimit = 255
	emailInput.SetWidth(30)

	return Users{
		table: table.New(
			table.WithColumns(userColumns),
			table.WithFocused(true),
			table.WithHeight(8),
			table.WithWidth(69),
		),
		nameInput:      nameInput,
		emailInput:     emailInput,
		loadUsers:      listUsers,
		createUser:     createUser,
		checkUserNodes: checkUserNodes,
		deleteUser:     deleteUser,
	}
}

func (m Users) Load() (Users, tea.Cmd) {
	m.loading = true
	m.err = nil
	return m, m.loadUsers()
}

func (m Users) Update(message tea.Msg) (Users, tea.Cmd) {
	switch message := message.(type) {
	case usersLoadedMsg:
		m.loading = false
		m.users = message.users
		m.err = message.err
		m.setTableRows()
		return m, nil
	case userDeletedMsg:
		m.deleting = false
		if message.hasNodes {
			return m, m.showUserDeleteAlert()
		}
		if message.err != nil {
			m.err = message.err
			return m, nil
		}
		return m.Load()
	case userDeleteCheckedMsg:
		m.checkingDelete = false
		if message.err != nil {
			m.checkingAfterConfirmation = false
			m.err = message.err
			return m, nil
		}
		if message.hasNodes {
			m.checkingAfterConfirmation = false
			return m, m.showUserDeleteAlert()
		}
		if m.checkingAfterConfirmation {
			m.checkingAfterConfirmation = false
			m.deleting = true
			return m, m.deleteUser(m.userToDelete.ID)
		}
		m.confirming = true
		return m, nil
	case userDeleteAlertExpiredMsg:
		if m.alerting && message.generation == m.alertGeneration {
			m.alerting = false
		}
		return m, nil
	case userCreatedMsg:
		m.creatingUser = false
		if message.err != nil {
			m.err = message.err
			return m, nil
		}
		return m.Load()
	case tea.WindowSizeMsg:
		m.table.SetWidth(max(69, message.Width))
	}

	if message, ok := message.(tea.KeyPressMsg); ok {
		if m.confirming {
			switch message.String() {
			case "y":
				m.confirming = false
				m.checkingDelete = true
				m.checkingAfterConfirmation = true
				m.err = nil
				return m, m.checkUserNodes(m.userToDelete.Name)
			case "n", "esc", "b", "q":
				m.confirming = false
			}
			return m, nil
		}

		if m.creating {
			switch message.String() {
			case "esc":
				m.creating = false
				m.nameInput.Blur()
				m.nameInput.Reset()
				m.emailInput.Blur()
				m.emailInput.Reset()
				m.err = nil
				return m, nil
			case "tab", "shift+tab":
				if m.nameInput.Focused() {
					m.nameInput.Blur()
					return m, m.emailInput.Focus()
				}
				m.emailInput.Blur()
				return m, m.nameInput.Focus()
			case "enter":
				name := strings.TrimSpace(m.nameInput.Value())
				if name == "" {
					m.err = fmt.Errorf("user name is required")
					return m, nil
				}
				if m.hasUser(name) {
					m.err = fmt.Errorf("user %q already exists", name)
					return m, nil
				}

				m.creating = false
				m.creatingUser = true
				m.nameInput.Blur()
				m.emailInput.Blur()
				m.err = nil
				return m, m.createUser(name, strings.TrimSpace(m.emailInput.Value()))
			}

			var command tea.Cmd
			if m.emailInput.Focused() {
				m.emailInput, command = m.emailInput.Update(message)
			} else {
				m.nameInput, command = m.nameInput.Update(message)
			}
			m.err = nil
			return m, command
		}

		switch message.String() {
		case "ctrl+c", "q":
			return m, func() tea.Msg { return quitMsg{} }
		case "esc", "b":
			return m, func() tea.Msg { return backToMenuMsg{} }
		case "r":
			if !m.loading && !m.creatingUser && !m.checkingDelete && !m.deleting {
				return m.Load()
			}
		case "n":
			if !m.loading && !m.creatingUser && !m.checkingDelete && !m.deleting {
				m.creating = true
				m.err = nil
				m.nameInput.Reset()
				m.emailInput.Reset()
				return m, m.nameInput.Focus()
			}
		case "d":
			if !m.loading && !m.creatingUser && !m.checkingDelete && !m.deleting {
				if user, ok := m.selectedUser(); ok {
					m.userToDelete = user
					m.checkingDelete = true
					m.checkingAfterConfirmation = false
					m.err = nil
					return m, m.checkUserNodes(user.Name)
				}
			}
		}
	}

	if !m.loading && !m.creatingUser && !m.checkingDelete && !m.deleting && !m.confirming && !m.creating {
		var command tea.Cmd
		m.table, command = m.table.Update(message)
		return m, command
	}

	return m, nil
}

func (m Users) View() string {
	content := m.contentView()
	if !m.alerting {
		return content
	}

	return floatingAlert(content, userDeleteAlertStyle.Render(userDeleteAlertContent))
}

func (m Users) contentView() string {
	lines := []string{"Users", ""}

	switch {
	case m.confirming:
		lines = append(
			lines,
			fmt.Sprintf("Delete user %q (ID %d)?", safeText(m.userToDelete.Name), m.userToDelete.ID),
			"This action cannot be undone.",
			"",
			"y: delete  n: cancel",
		)
		return strings.Join(lines, "\n")
	case m.checkingDelete:
		lines = append(lines, "Checking whether user can be deleted...")
	case m.creating:
		lines = append(lines, "Create a user", m.nameInput.View(), m.emailInput.View())
		if m.err != nil {
			lines = append(lines, "Error: "+safeText(m.err.Error()))
		}
		lines = append(lines, "", "tab: next field  enter: create  esc: cancel")
		return strings.Join(lines, "\n")
	case m.creatingUser:
		lines = append(lines, "Creating user...")
	case m.deleting:
		lines = append(lines, "Deleting user "+safeText(m.userToDelete.Name)+"...")
	case m.loading:
		lines = append(lines, "Loading users...")
	case m.err != nil:
		lines = append(lines, "Error: "+safeText(m.err.Error()))
	case len(m.users) == 0:
		lines = append(lines, "No users found.")
	default:
		lines = append(lines, m.table.View())
	}

	lines = append(lines, "", "up/down or j/k: select  n: new user  d: delete  r: refresh  b: back  q: quit")
	return strings.Join(lines, "\n")
}

func (m *Users) showUserDeleteAlert() tea.Cmd {
	m.alerting = true
	m.alertGeneration++
	generation := m.alertGeneration
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		return userDeleteAlertExpiredMsg{generation: generation}
	})
}

func floatingAlert(content, alert string) string {
	width := max(lipgloss.Width(content), lipgloss.Width(alert))
	height := max(lipgloss.Height(content), lipgloss.Height(alert))
	background := lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, content)
	alertLayer := lipgloss.NewLayer(alert).
		X((width - lipgloss.Width(alert)) / 2).
		Y(height - lipgloss.Height(alert)).
		Z(1)
	return lipgloss.NewCompositor(lipgloss.NewLayer(background), alertLayer).Render()
}

func (m Users) selectedUser() (User, bool) {
	index := m.table.Cursor()
	if index >= 0 && index < len(m.users) {
		return m.users[index], true
	}
	return User{}, false
}

func (m Users) hasUser(name string) bool {
	for _, user := range m.users {
		if user.Name == name {
			return true
		}
	}
	return false
}

func (m *Users) setTableRows() {
	rows := make([]table.Row, 0, len(m.users))
	for _, user := range m.users {
		rows = append(rows, table.Row{fmt.Sprint(user.ID), safeText(user.Name), safeText(user.Email)})
	}
	m.table.SetRows(rows)
	m.table.SetHeight(min(8, len(rows)+1))
}

func listUsers() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		output, err := exec.CommandContext(ctx, "headscale", "users", "list", "--output", "json").Output()
		if err != nil {
			return usersLoadedMsg{err: fmt.Errorf("running headscale users list: %w", err)}
		}

		var users []User
		if err := json.Unmarshal(output, &users); err != nil {
			return usersLoadedMsg{err: fmt.Errorf("decoding user list: %w", err)}
		}

		return usersLoadedMsg{users: users}
	}
}

func createUser(name, email string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		arguments := []string{"users", "create", name}
		if email != "" {
			arguments = append(arguments, "--email", email)
		}
		err := exec.CommandContext(ctx, "headscale", arguments...).Run()
		if err != nil {
			return userCreatedMsg{err: fmt.Errorf("creating user %q: %w", name, err)}
		}

		return userCreatedMsg{}
	}
}

func checkUserNodes(name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		output, err := exec.CommandContext(ctx, "headscale", "nodes", "list", "--user", name, "--output", "json").Output()
		if err != nil {
			return userDeleteCheckedMsg{err: fmt.Errorf("listing nodes for user %q: %w", name, err)}
		}

		var nodes []Node
		if err := json.Unmarshal(output, &nodes); err != nil {
			return userDeleteCheckedMsg{err: fmt.Errorf("decoding nodes for user %q: %w", name, err)}
		}

		return userDeleteCheckedMsg{hasNodes: len(nodes) > 0}
	}
}

func deleteUser(id uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		output, err := exec.CommandContext(ctx, "headscale", "users", "destroy", "--identifier", fmt.Sprint(id), "--force").CombinedOutput()
		if err != nil {
			detail := strings.TrimSpace(string(output))
			if strings.Contains(detail, "user not empty") {
				return userDeletedMsg{hasNodes: true}
			}
			if detail != "" {
				return userDeletedMsg{err: fmt.Errorf("deleting user %d: %s", id, detail)}
			}
			return userDeletedMsg{err: fmt.Errorf("deleting user %d: %w", id, err)}
		}

		return userDeletedMsg{}
	}
}

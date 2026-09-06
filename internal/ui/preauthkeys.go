package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type apiTimestamp struct {
	Seconds int64 `json:"seconds"`
}

type PreAuthKey struct {
	ID         uint64        `json:"id"`
	Key        string        `json:"key"`
	Reusable   bool          `json:"reusable"`
	Ephemeral  bool          `json:"ephemeral"`
	Expiration *apiTimestamp `json:"expiration"`
	CreatedAt  *apiTimestamp `json:"created_at"`
	User       User          `json:"user"`
}

type preAuthKeysLoadedMsg struct {
	keys  []PreAuthKey
	users []User
	err   error
}

type preAuthKeyCreatedMsg struct {
	key string
	err error
}

type preAuthKeyDeletedMsg struct {
	err error
}

type preAuthKeyGroup struct {
	heading string
	keys    []PreAuthKey
}

type preAuthKeyTable struct {
	heading string
	keys    []PreAuthKey
	table   table.Model
}

const (
	preAuthKeyTableHeight = 8
	preAuthKeyTableWidth  = 94
)

var preAuthKeyColumns = []table.Column{
	{Title: "ID", Width: 5},
	{Title: "KEY", Width: 34},
	{Title: "REUSABLE", Width: 10},
	{Title: "EPHEMERAL", Width: 11},
	{Title: "EXPIRATION", Width: 20},
}

// PreAuthKeys manages listing, creating, and deleting Headscale preauth keys.
type PreAuthKeys struct {
	loading           bool
	creating          bool
	creatingKey       bool
	showingCreatedKey bool
	deleting          bool
	confirming        bool
	keyToDelete       PreAuthKey
	keys              []PreAuthKey
	users             []User
	tables            []preAuthKeyTable
	activeTable       int
	userInput         textinput.Model
	durationInput     textinput.Model
	formField         int
	reusable          bool
	ephemeral         bool
	createdKey        string
	err               error
	loadPreAuthKeys   func() tea.Cmd
	createPreAuthKey  func(uint64, string, bool, bool) tea.Cmd
	deletePreAuthKey  func(uint64) tea.Cmd
}

func NewPreAuthKeys() PreAuthKeys {
	userInput := textinput.New()
	userInput.Prompt = "User: "
	userInput.Placeholder = "alice"
	userInput.CharLimit = 255
	userInput.SetWidth(30)
	durationInput := textinput.New()
	durationInput.Prompt = "Expiration duration: "
	durationInput.Placeholder = "1h"
	durationInput.CharLimit = 255
	durationInput.SetWidth(30)

	return PreAuthKeys{
		userInput:        userInput,
		durationInput:    durationInput,
		loadPreAuthKeys:  listPreAuthKeys,
		createPreAuthKey: createPreAuthKey,
		deletePreAuthKey: deletePreAuthKey,
	}
}

func (m PreAuthKeys) Load() (PreAuthKeys, tea.Cmd) {
	m.loading = true
	m.err = nil
	return m, m.loadPreAuthKeys()
}

func (m PreAuthKeys) Update(message tea.Msg) (PreAuthKeys, tea.Cmd) {
	switch message := message.(type) {
	case preAuthKeysLoadedMsg:
		m.loading = false
		m.keys = message.keys
		m.users = message.users
		m.err = message.err
		m.setTableRows()
		return m, nil
	case preAuthKeyCreatedMsg:
		m.creatingKey = false
		if message.err != nil {
			m.err = message.err
			return m, nil
		}
		m.createdKey = message.key
		m.showingCreatedKey = true
		return m, nil
	case preAuthKeyDeletedMsg:
		m.deleting = false
		if message.err != nil {
			m.err = message.err
			return m, nil
		}
		return m.Load()
	case tea.WindowSizeMsg:
		for index := range m.tables {
			m.tables[index].table.SetWidth(max(preAuthKeyTableWidth, message.Width))
		}
	}

	keyPress, ok := message.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	if m.confirming {
		switch keyPress.String() {
		case "y":
			m.confirming = false
			m.deleting = true
			m.err = nil
			return m, m.deletePreAuthKey(m.keyToDelete.ID)
		case "n", "esc", "b", "q":
			m.confirming = false
		}
		return m, nil
	}
	if m.showingCreatedKey {
		switch keyPress.String() {
		case "enter":
			m.showingCreatedKey = false
			m.createdKey = ""
			return m.Load()
		case "ctrl+c", "q":
			return m, func() tea.Msg { return quitMsg{} }
		}
		return m, nil
	}

	if m.creating {
		switch keyPress.String() {
		case "esc":
			m.creating = false
			m.userInput.Blur()
			m.durationInput.Blur()
			m.err = nil
			return m, nil
		case "tab", "shift+tab":
			direction := 1
			if keyPress.String() == "shift+tab" {
				direction = -1
			}
			return m, m.focusFormField(m.formField + direction)
		case "space", " ":
			switch m.formField {
			case 2:
				m.reusable = !m.reusable
			case 3:
				m.ephemeral = !m.ephemeral
			}
			return m, nil
		case "enter":
			user, ok := m.userNamed(strings.TrimSpace(m.userInput.Value()))
			if !ok {
				m.err = fmt.Errorf("an existing user is required")
				return m, nil
			}
			m.creating = false
			m.creatingKey = true
			m.userInput.Blur()
			m.durationInput.Blur()
			m.err = nil
			return m, m.createPreAuthKey(user.ID, strings.TrimSpace(m.durationInput.Value()), m.reusable, m.ephemeral)
		}

		var command tea.Cmd
		if m.formField == 1 {
			m.durationInput, command = m.durationInput.Update(keyPress)
		} else if m.formField == 0 {
			m.userInput, command = m.userInput.Update(keyPress)
		}
		m.err = nil
		return m, command
	}

	switch keyPress.String() {
	case "ctrl+c", "q":
		return m, func() tea.Msg { return quitMsg{} }
	case "esc", "b":
		return m, func() tea.Msg { return backToMenuMsg{} }
	case "r":
		if !m.loading && !m.creatingKey && !m.deleting {
			return m.Load()
		}
	case "n":
		if !m.loading && !m.creatingKey && !m.deleting {
			m.creating = true
			m.userInput.Reset()
			m.durationInput.SetValue("1h")
			m.formField = 0
			m.reusable = false
			m.ephemeral = false
			m.err = nil
			return m, m.userInput.Focus()
		}
	case "d":
		if !m.loading && !m.creatingKey && !m.deleting {
			if key, ok := m.selectedPreAuthKey(); ok {
				m.keyToDelete = key
				m.confirming = true
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

	if !m.loading && !m.creatingKey && !m.deleting && len(m.tables) > 0 {
		var command tea.Cmd
		m.tables[m.activeTable].table, command = m.tables[m.activeTable].table.Update(keyPress)
		return m, command
	}

	return m, nil
}

func (m PreAuthKeys) View() string {
	lines := []string{"Preauth Keys", ""}

	switch {
	case m.confirming:
		lines = append(
			lines,
			fmt.Sprintf("Destroy preauth key %d for %q?", m.keyToDelete.ID, safeText(m.keyToDelete.User.Name)),
			"This action cannot be undone.",
			"",
			"y: destroy  n: cancel",
		)
		return strings.Join(lines, "\n")
	case m.showingCreatedKey:
		lines = append(
			lines,
			"Preauth key created",
			"",
			"Copy this key now. It cannot be shown again.",
			safeText(m.createdKey),
			"",
			"enter: return to preauth keys  q: quit",
		)
		return strings.Join(lines, "\n")
	case m.creating:
		lines = append(
			lines,
			"Create a preauth key",
			m.userInput.View(),
			m.durationInput.View(),
			m.checkboxView("Reusable", m.reusable, 2),
			m.checkboxView("Ephemeral", m.ephemeral, 3),
		)
		if len(m.users) > 0 {
			lines = append(lines, "Available users: "+strings.Join(m.userNames(), ", "))
		}
		if m.err != nil {
			lines = append(lines, "Error: "+safeText(m.err.Error()))
		}
		lines = append(lines, "", "tab: next field  space: toggle  enter: create  esc: cancel")
		return strings.Join(lines, "\n")
	case m.creatingKey:
		lines = append(lines, "Creating preauth key...")
	case m.deleting:
		lines = append(lines, fmt.Sprintf("Destroying preauth key %d...", m.keyToDelete.ID))
	case m.loading:
		lines = append(lines, "Loading preauth keys...")
	case m.err != nil:
		lines = append(lines, "Error: "+safeText(m.err.Error()))
	case len(m.keys) == 0:
		lines = append(lines, "No preauth keys found.")
	default:
		sections := make([]string, 0, len(m.tables))
		for _, keyTable := range m.tables {
			sections = append(sections, safeText(keyTable.heading)+"\n"+keyTable.table.View())
		}
		lines = append(lines, strings.Join(sections, "\n\n"))
	}

	lines = append(lines, "", "up/down or j/k: select  tab: switch user  n: new preauth key  d: destroy  r: refresh  b: back  q: quit")
	return strings.Join(lines, "\n")
}

func (m PreAuthKeys) preAuthKeyGroups() []preAuthKeyGroup {
	owners := make(map[string][]PreAuthKey)
	for _, key := range m.keys {
		owners[key.User.Name] = append(owners[key.User.Name], key)
	}

	ownerNames := make([]string, 0, len(owners))
	for owner := range owners {
		ownerNames = append(ownerNames, owner)
	}
	sort.Strings(ownerNames)

	groups := make([]preAuthKeyGroup, 0, len(ownerNames))
	for _, owner := range ownerNames {
		groups = append(groups, preAuthKeyGroup{heading: owner, keys: owners[owner]})
	}
	return groups
}

func (m PreAuthKeys) selectedPreAuthKey() (PreAuthKey, bool) {
	if m.activeTable < 0 || m.activeTable >= len(m.tables) {
		return PreAuthKey{}, false
	}

	group := m.tables[m.activeTable]
	index := group.table.Cursor()
	if index >= 0 && index < len(group.keys) {
		return group.keys[index], true
	}
	return PreAuthKey{}, false
}

func (m PreAuthKeys) userNamed(name string) (User, bool) {
	for _, user := range m.users {
		if user.Name == name {
			return user, true
		}
	}
	return User{}, false
}

func (m PreAuthKeys) userNames() []string {
	names := make([]string, 0, len(m.users))
	for _, user := range m.users {
		names = append(names, safeText(user.Name))
	}
	sort.Strings(names)
	return names
}

func (m *PreAuthKeys) focusFormField(field int) tea.Cmd {
	const fieldCount = 4
	m.formField = (field + fieldCount) % fieldCount
	m.userInput.Blur()
	m.durationInput.Blur()
	switch m.formField {
	case 0:
		return m.userInput.Focus()
	case 1:
		return m.durationInput.Focus()
	default:
		return nil
	}
}

func (m PreAuthKeys) checkboxView(label string, checked bool, field int) string {
	box := "[ ]"
	if checked {
		box = "[x]"
	}
	prefix := "  "
	if m.formField == field {
		prefix = "> "
	}
	return prefix + box + " " + label
}

func (m *PreAuthKeys) setTableRows() {
	groups := m.preAuthKeyGroups()
	m.tables = make([]preAuthKeyTable, 0, len(groups))
	m.activeTable = 0
	for index, group := range groups {
		rows := make([]table.Row, 0, len(group.keys))
		for _, key := range group.keys {
			rows = append(rows, table.Row{
				fmt.Sprint(key.ID),
				safeText(key.Key),
				fmt.Sprint(key.Reusable),
				fmt.Sprint(key.Ephemeral),
				formatPreAuthKeyTimestamp(key.Expiration),
			})
		}

		m.tables = append(m.tables, preAuthKeyTable{
			heading: group.heading,
			keys:    group.keys,
			table: table.New(
				table.WithColumns(preAuthKeyColumns),
				table.WithRows(rows),
				table.WithFocused(index == 0),
				table.WithHeight(min(preAuthKeyTableHeight, len(rows)+1)),
				table.WithWidth(preAuthKeyTableWidth),
			),
		})
	}
	m.syncTableStyles()
}

func (m *PreAuthKeys) selectTable(direction int) {
	if len(m.tables) < 2 {
		return
	}
	m.tables[m.activeTable].table.Blur()
	m.activeTable = (m.activeTable + direction + len(m.tables)) % len(m.tables)
	m.tables[m.activeTable].table.Focus()
}

func (m *PreAuthKeys) moveUp() {
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

func (m *PreAuthKeys) moveDown() {
	if len(m.tables) == 0 {
		return
	}
	current := &m.tables[m.activeTable]
	if current.table.Cursor() == len(current.keys)-1 && m.activeTable < len(m.tables)-1 {
		m.selectTable(1)
		m.tables[m.activeTable].table.GotoTop()
	} else {
		current.table.MoveDown(1)
	}
	m.syncTableStyles()
}

func (m *PreAuthKeys) syncTableStyles() {
	for tableIndex := range m.tables {
		styles := table.DefaultStyles()
		if tableIndex != m.activeTable {
			styles.Selected = lipgloss.NewStyle()
		}
		m.tables[tableIndex].table.SetStyles(styles)
	}
}

func formatPreAuthKeyTimestamp(timestamp *apiTimestamp) string {
	if timestamp == nil || timestamp.Seconds == 0 {
		return "never"
	}
	return time.Unix(timestamp.Seconds, 0).Local().Format("2006-01-02 15:04 MST")
}

func listPreAuthKeys() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		keyOutput, err := exec.CommandContext(ctx, "headscale", "preauthkeys", "list", "--output", "json").CombinedOutput()
		if err != nil {
			return preAuthKeysLoadedMsg{err: headscaleCommandError("listing preauth keys", err, keyOutput)}
		}
		var keys []PreAuthKey
		if err := json.Unmarshal(keyOutput, &keys); err != nil {
			return preAuthKeysLoadedMsg{err: fmt.Errorf("decoding preauth key list: %w", err)}
		}

		userOutput, err := exec.CommandContext(ctx, "headscale", "users", "list", "--output", "json").CombinedOutput()
		if err != nil {
			return preAuthKeysLoadedMsg{err: headscaleCommandError("listing users", err, userOutput)}
		}
		var users []User
		if err := json.Unmarshal(userOutput, &users); err != nil {
			return preAuthKeysLoadedMsg{err: fmt.Errorf("decoding user list: %w", err)}
		}

		return preAuthKeysLoadedMsg{keys: keys, users: users}
	}
}

func createPreAuthKey(userID uint64, duration string, reusable, ephemeral bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		arguments := []string{"preauthkeys", "create", "--user", fmt.Sprint(userID)}
		if duration != "" {
			arguments = append(arguments, "--expiration", duration)
		}
		if reusable {
			arguments = append(arguments, "--reusable")
		}
		if ephemeral {
			arguments = append(arguments, "--ephemeral")
		}
		arguments = append(arguments, "--output", "json")
		output, err := exec.CommandContext(ctx, "headscale", arguments...).CombinedOutput()
		if err != nil {
			return preAuthKeyCreatedMsg{err: headscaleCommandError("creating preauth key", err, output)}
		}
		var key PreAuthKey
		if err := json.Unmarshal(output, &key); err != nil {
			return preAuthKeyCreatedMsg{err: fmt.Errorf("decoding created preauth key: %w", err)}
		}
		return preAuthKeyCreatedMsg{key: key.Key}
	}
}

func deletePreAuthKey(id uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		output, err := exec.CommandContext(ctx, "headscale", "preauthkeys", "delete", "--id", fmt.Sprint(id), "--force", "--output", "json").CombinedOutput()
		if err != nil {
			return preAuthKeyDeletedMsg{err: headscaleCommandError("destroying preauth key", err, output)}
		}
		return preAuthKeyDeletedMsg{}
	}
}

func headscaleCommandError(action string, err error, output []byte) error {
	var response struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(output, &response) == nil && response.Error != "" {
		return fmt.Errorf("%s: %s", action, response.Error)
	}
	if detail := strings.TrimSpace(string(output)); detail != "" {
		return fmt.Errorf("%s: %s", action, safeText(detail))
	}
	return fmt.Errorf("%s: %w", action, err)
}

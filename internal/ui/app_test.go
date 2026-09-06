package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestAppLoadsNodes(t *testing.T) {
	t.Parallel()

	app := NewApp()
	app.nodes.loadNodes = func() tea.Cmd {
		return func() tea.Msg {
			return nodesLoadedMsg{nodes: []Node{{ID: 1, Name: "alice-laptop", Online: true}}}
		}
	}

	model, _ := app.Update(tea.KeyPressMsg{Text: "j"})
	app = model.(App)
	model, command := app.Update(tea.KeyPressMsg{Text: "enter"})
	app = model.(App)
	if command == nil {
		t.Fatal("selecting Nodes did not request navigation")
	}

	model, command = app.Update(command())
	app = model.(App)
	if command == nil || app.screen != nodesScreen || !app.nodes.loading {
		t.Fatalf("opening Nodes did not start loading: %+v", app)
	}

	model, _ = app.Update(command())
	app = model.(App)
	if app.nodes.loading || len(app.nodes.nodes) != 1 {
		t.Fatalf("node result was not applied: %+v", app.nodes)
	}
	if got := app.View(); !got.AltScreen || !strings.Contains(got.Content, "alice-laptop") {
		t.Fatalf("node list was not rendered in the alternate screen: %+v", got)
	}
}

func TestAppLoadsUsers(t *testing.T) {
	t.Parallel()

	app := NewApp()
	app.users.loadUsers = func() tea.Cmd {
		return func() tea.Msg { return usersLoadedMsg{users: []User{{ID: 1, Name: "alice"}}} }
	}

	model, command := app.Update(tea.KeyPressMsg{Text: "enter"})
	app = model.(App)
	if command == nil {
		t.Fatal("selecting Users did not request navigation")
	}

	model, command = app.Update(command())
	app = model.(App)
	if command == nil || app.screen != usersScreen || !app.users.loading {
		t.Fatalf("opening Users did not start loading: %+v", app)
	}

	model, _ = app.Update(command())
	app = model.(App)
	if app.users.loading || len(app.users.users) != 1 {
		t.Fatalf("user result was not applied: %+v", app.users)
	}
	if got := app.View(); !got.AltScreen || !strings.Contains(got.Content, "alice") {
		t.Fatalf("user list was not rendered in the alternate screen: %+v", got)
	}
}

func TestAppLoadsPreAuthKeys(t *testing.T) {
	t.Parallel()

	app := NewApp()
	app.preAuthKeys.loadPreAuthKeys = func() tea.Cmd {
		return func() tea.Msg {
			key := PreAuthKey{ID: 1, Key: "hskey-auth-***"}
			key.User.Name = "alice"
			return preAuthKeysLoadedMsg{keys: []PreAuthKey{key}, users: []User{{ID: 1, Name: "alice"}}}
		}
	}

	model, _ := app.Update(tea.KeyPressMsg{Text: "j"})
	app = model.(App)
	model, _ = app.Update(tea.KeyPressMsg{Text: "j"})
	app = model.(App)
	model, command := app.Update(tea.KeyPressMsg{Text: "enter"})
	app = model.(App)
	if command == nil {
		t.Fatal("selecting Preauth Keys did not request navigation")
	}

	model, command = app.Update(command())
	app = model.(App)
	if command == nil || app.screen != preAuthKeysScreen || !app.preAuthKeys.loading {
		t.Fatalf("opening Preauth Keys did not start loading: %+v", app)
	}

	model, _ = app.Update(command())
	app = model.(App)
	if app.preAuthKeys.loading || len(app.preAuthKeys.keys) != 1 {
		t.Fatalf("preauth key result was not applied: %+v", app.preAuthKeys)
	}
	if got := app.View(); !got.AltScreen || !strings.Contains(got.Content, "hskey-auth-***") {
		t.Fatalf("preauth key list was not rendered in the alternate screen: %+v", got)
	}
}

func TestAppReturnsToMenu(t *testing.T) {
	t.Parallel()

	app := NewApp()
	app.screen = nodesScreen
	model, command := app.Update(tea.KeyPressMsg{Text: "b"})
	app = model.(App)
	if command == nil || app.screen != nodesScreen {
		t.Fatalf("back did not request menu navigation: %+v", app)
	}

	model, _ = app.Update(command())
	app = model.(App)
	if app.screen != menuScreen {
		t.Fatalf("back did not return to the menu: %+v", app)
	}
}

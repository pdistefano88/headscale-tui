package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestUsersTableUsesSelectedRow(t *testing.T) {
	t.Parallel()

	users := NewUsers()
	users.users = []User{{ID: 1, Name: "alice"}, {ID: 2, Name: "bob", Email: "bob@example.test"}}
	users.setTableRows()
	users, _ = users.Update(tea.KeyPressMsg{Text: "j"})

	if got := users.table.SelectedRow()[1]; got != "bob" {
		t.Fatalf("selected row name = %q, want bob", got)
	}
	if user, ok := users.selectedUser(); !ok || user.ID != 2 {
		t.Fatalf("selected user = %+v, %t; want user 2", user, ok)
	}
}

func TestUsersCreateRejectsDuplicateName(t *testing.T) {
	t.Parallel()

	users := NewUsers()
	users.users = []User{{ID: 1, Name: "alice"}}
	users.setTableRows()
	users, _ = users.Update(tea.KeyPressMsg{Text: "n"})
	users.nameInput.SetValue("alice")
	users, command := users.Update(tea.KeyPressMsg{Text: "enter"})

	if command != nil || !users.creating {
		t.Fatalf("duplicate name unexpectedly started creation: %+v", users)
	}
	if got := users.View(); !strings.Contains(got, `user "alice" already exists`) {
		t.Fatalf("duplicate name error was not rendered: %q", got)
	}
}

func TestUsersCreateAndDelete(t *testing.T) {
	t.Parallel()

	var createdName string
	var createdEmail string
	var deletedID uint64
	var nodeChecks int
	users := NewUsers()
	users.users = []User{{ID: 1, Name: "alice"}, {ID: 2, Name: "bob"}}
	users.setTableRows()
	users.createUser = func(name, email string) tea.Cmd {
		return func() tea.Msg {
			createdName = name
			createdEmail = email
			return userCreatedMsg{}
		}
	}
	users.checkUserNodes = func(string) tea.Cmd {
		return func() tea.Msg {
			nodeChecks++
			return userDeleteCheckedMsg{}
		}
	}
	users.deleteUser = func(id uint64) tea.Cmd {
		return func() tea.Msg {
			deletedID = id
			return userDeletedMsg{}
		}
	}
	users.loadUsers = func() tea.Cmd {
		return func() tea.Msg { return usersLoadedMsg{users: users.users} }
	}

	users, _ = users.Update(tea.KeyPressMsg{Text: "n"})
	users.nameInput.SetValue("carol")
	users, _ = users.Update(tea.KeyPressMsg{Text: "tab"})
	if !users.emailInput.Focused() {
		t.Fatal("tab did not focus the email field")
	}
	users.emailInput.SetValue("carol@example.test")
	users, command := users.Update(tea.KeyPressMsg{Text: "enter"})
	if command == nil || !users.creatingUser {
		t.Fatalf("creation did not start: %+v", users)
	}
	users, command = users.Update(command())
	if createdName != "carol" || createdEmail != "carol@example.test" || command == nil || !users.loading {
		t.Fatalf("creation did not reload users: name=%q email=%q users=%+v", createdName, createdEmail, users)
	}
	users, _ = users.Update(command())

	users, _ = users.Update(tea.KeyPressMsg{Text: "j"})
	users, command = users.Update(tea.KeyPressMsg{Text: "d"})
	if command == nil || !users.checkingDelete {
		t.Fatalf("delete did not check the user before confirmation: %+v", users)
	}
	users, command = users.Update(command())
	if command != nil || nodeChecks != 1 || !users.confirming {
		t.Fatalf("delete did not show a confirmation after checking: %+v", users)
	}
	if got := users.View(); !strings.Contains(got, `Delete user "bob" (ID 2)?`) {
		t.Fatalf("confirmation did not name the selected user: %q", got)
	}

	users, command = users.Update(tea.KeyPressMsg{Text: "y"})
	if command == nil || users.confirming || !users.checkingDelete || !users.checkingAfterConfirmation {
		t.Fatalf("confirmation did not recheck the user: %+v", users)
	}
	users, command = users.Update(command())
	if command == nil || nodeChecks != 2 || !users.deleting {
		t.Fatalf("confirmation did not start deletion after rechecking: %+v", users)
	}
	if _, ok := command().(userDeletedMsg); !ok || deletedID != 2 {
		t.Fatalf("delete command targeted user %d, want 2", deletedID)
	}
}

func TestUsersDeleteShowsAlertWhenUserHasNodes(t *testing.T) {
	t.Parallel()

	for _, afterConfirmation := range []bool{false, true} {
		users := NewUsers()
		users.checkingDelete = true
		users.checkingAfterConfirmation = afterConfirmation
		users, command := users.Update(userDeleteCheckedMsg{hasNodes: true})

		if command == nil || users.confirming || !users.alerting {
			t.Fatalf("non-empty user did not show an alert: %+v", users)
		}
		if got := users.View(); !strings.Contains(got, "This user still owns one or more nodes.") {
			t.Fatalf("non-empty user alert was not rendered: %q", got)
		}

		firstGeneration := users.alertGeneration
		if command := users.showUserDeleteAlert(); command == nil {
			t.Fatal("new alert did not schedule expiration")
		}
		users, _ = users.Update(userDeleteAlertExpiredMsg{generation: firstGeneration})
		if !users.alerting {
			t.Fatal("stale alert expiration dismissed the newer alert")
		}
		users, _ = users.Update(userDeleteAlertExpiredMsg{generation: users.alertGeneration})
		if users.alerting {
			t.Fatal("matching alert expiration did not dismiss the alert")
		}
	}
}

func TestUsersAlertOverlaysTableWithoutBlockingInput(t *testing.T) {
	t.Parallel()

	users := NewUsers()
	users.users = []User{
		{ID: 1, Name: "alice"},
		{ID: 2, Name: "bob"},
		{ID: 3, Name: "carol"},
		{ID: 4, Name: "dave"},
		{ID: 5, Name: "eve"},
		{ID: 6, Name: "frank"},
		{ID: 7, Name: "grace"},
		{ID: 8, Name: "heidi"},
	}
	users.setTableRows()
	users.alerting = true

	if got := users.View(); !strings.Contains(got, "Cannot delete user") || !strings.Contains(got, "q: quit") {
		t.Fatalf("alert did not overlay the user view: %q", got)
	}
	users, _ = users.Update(tea.KeyPressMsg{Text: "j"})
	if got := users.table.Cursor(); got != 1 {
		t.Fatalf("alert blocked table navigation; cursor = %d, want 1", got)
	}
}

package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestPreAuthKeysTablesGroupByUser(t *testing.T) {
	t.Parallel()

	aliceKey := PreAuthKey{ID: 2, Key: "hskey-auth-alice-***", Reusable: true}
	aliceKey.User.Name = "alice"
	bobKey := PreAuthKey{ID: 1, Key: "hskey-auth-bob-***", Ephemeral: true}
	bobKey.User.Name = "bob"

	keys := NewPreAuthKeys()
	keys.keys = []PreAuthKey{bobKey, aliceKey}
	keys.setTableRows()

	view := keys.View()
	for _, text := range []string{"ID", "REUSABLE", "EPHEMERAL", "alice\n", "\n\nbob\n", "hskey-auth-alice-***", "hskey-auth-bob-***"} {
		if !strings.Contains(view, text) {
			t.Fatalf("preauth key table is missing %q: %q", text, view)
		}
	}
	if strings.Index(view, "hskey-auth-alice-***") > strings.Index(view, "hskey-auth-bob-***") {
		t.Fatalf("preauth keys are not ordered by user: %q", view)
	}
}

func TestPreAuthKeyCreateUsesSelectedUserAndDuration(t *testing.T) {
	t.Parallel()

	var createdUserID uint64
	var createdDuration string
	var createdReusable bool
	var createdEphemeral bool
	keys := NewPreAuthKeys()
	keys.users = []User{{ID: 2, Name: "alice"}}
	keys.createPreAuthKey = func(userID uint64, duration string, reusable, ephemeral bool) tea.Cmd {
		return func() tea.Msg {
			createdUserID = userID
			createdDuration = duration
			createdReusable = reusable
			createdEphemeral = ephemeral
			return preAuthKeyCreatedMsg{key: "hskey-auth-secret"}
		}
	}
	keys.loadPreAuthKeys = func() tea.Cmd {
		return func() tea.Msg {
			key := PreAuthKey{ID: 4, Key: "hskey-auth-***"}
			key.User.Name = "alice"
			return preAuthKeysLoadedMsg{keys: []PreAuthKey{key}, users: keys.users}
		}
	}

	keys, _ = keys.Update(tea.KeyPressMsg{Text: "n"})
	if !keys.creating || !keys.userInput.Focused() || keys.durationInput.Value() != "1h" {
		t.Fatalf("new preauth key did not open the form with Headscale's default duration: %+v", keys)
	}
	keys.userInput.SetValue("alice")
	keys.durationInput.SetValue("90d")
	keys, _ = keys.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	keys, _ = keys.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	keys, _ = keys.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	keys, _ = keys.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	keys, _ = keys.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	keys, command := keys.Update(tea.KeyPressMsg{Text: "enter"})
	if command == nil || !keys.creatingKey {
		t.Fatalf("preauth key creation did not start: %+v", keys)
	}

	keys, command = keys.Update(command())
	if command != nil || createdUserID != 2 || keys.createdKey != "hskey-auth-secret" || !keys.showingCreatedKey {
		t.Fatalf("preauth key creation did not show the created key: user=%d keys=%+v", createdUserID, keys)
	}
	if createdDuration != "90d" {
		t.Fatalf("creation duration = %q, want 90d", createdDuration)
	}
	if !createdReusable || !createdEphemeral {
		t.Fatalf("creation flags = reusable:%t ephemeral:%t, want both true", createdReusable, createdEphemeral)
	}
	if got := keys.View(); !strings.Contains(got, "Copy this key now.") || !strings.Contains(got, "hskey-auth-secret") {
		t.Fatalf("created preauth key was not displayed: %q", got)
	}

	keys, command = keys.Update(tea.KeyPressMsg{Text: "enter"})
	if command == nil || keys.showingCreatedKey || keys.createdKey != "" || !keys.loading {
		t.Fatalf("acknowledging created key did not clear and reload: %+v", keys)
	}
	keys, _ = keys.Update(command())
	if got := keys.View(); strings.Contains(got, "hskey-auth-secret") {
		t.Fatalf("created preauth key remained visible after returning to the list: %q", got)
	}
}

func TestPreAuthKeyCreateRejectsUnknownUser(t *testing.T) {
	t.Parallel()

	keys := NewPreAuthKeys()
	keys.users = []User{{ID: 1, Name: "alice"}}
	keys, _ = keys.Update(tea.KeyPressMsg{Text: "n"})
	keys.userInput.SetValue("bob")
	keys, command := keys.Update(tea.KeyPressMsg{Text: "enter"})

	if command != nil || !keys.creating {
		t.Fatalf("unknown user unexpectedly started creation: %+v", keys)
	}
	if got := keys.View(); !strings.Contains(got, "an existing user is required") {
		t.Fatalf("unknown user error was not rendered: %q", got)
	}
}

func TestPreAuthKeyDestroyConfirmation(t *testing.T) {
	t.Parallel()

	var deletedID uint64
	key := PreAuthKey{ID: 3, Key: "hskey-auth-***"}
	key.User.Name = "alice"
	keys := NewPreAuthKeys()
	keys.keys = []PreAuthKey{key}
	keys.setTableRows()
	keys.deletePreAuthKey = func(id uint64) tea.Cmd {
		return func() tea.Msg {
			deletedID = id
			return preAuthKeyDeletedMsg{}
		}
	}

	keys, command := keys.Update(tea.KeyPressMsg{Text: "d"})
	if command != nil || !keys.confirming {
		t.Fatalf("destroy did not show confirmation: %+v", keys)
	}
	if got := keys.View(); !strings.Contains(got, `Destroy preauth key 3 for "alice"?`) {
		t.Fatalf("confirmation did not name the selected key: %q", got)
	}

	keys, command = keys.Update(tea.KeyPressMsg{Text: "y"})
	if command == nil || keys.confirming || !keys.deleting {
		t.Fatalf("confirmation did not start destruction: %+v", keys)
	}
	if _, ok := command().(preAuthKeyDeletedMsg); !ok || deletedID != 3 {
		t.Fatalf("destroy command targeted key %d, want 3", deletedID)
	}
}

func TestHeadscaleCommandErrorDecodesJSON(t *testing.T) {
	t.Parallel()

	err := headscaleCommandError("creating preauth key", errors.New("exit status 1"), []byte(`{"error":"parsing duration: unknown unit"}`))
	if got, want := err.Error(), "creating preauth key: parsing duration: unknown unit"; got != want {
		t.Fatalf("parsed command error = %q, want %q", got, want)
	}
}

func TestPreAuthKeyTimestampIncludesTimezone(t *testing.T) {
	t.Parallel()

	timestamp := &apiTimestamp{Seconds: 1788646082}
	want := time.Unix(timestamp.Seconds, 0).Local().Format("2006-01-02 15:04 MST")
	if got := formatPreAuthKeyTimestamp(timestamp); got != want {
		t.Fatalf("formatted timestamp = %q, want %q", got, want)
	}
}

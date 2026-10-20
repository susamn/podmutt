package ui

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestGlobalSeekKeysAreGuardedAndIndependentOfNavigation(t *testing.T) {
	var commands []string
	m := resize(New(fixture()), 140, 44).WithActions(func(command string) tea.Cmd {
		return func() tea.Msg { commands = append(commands, command); return nil }
	})
	key := func(value string) {
		t.Helper()
		next, cmd := m.Update(keyMsg(value))
		m = next.(Model)
		if cmd != nil {
			cmd()
		}
	}
	key(",")
	key(".")
	if len(commands) != 0 {
		t.Fatal("disabled seek dispatched")
	}
	m = m.WithPlayback(m.data.Queue, Player{Status: "paused", HasCurrent: true, CanSeek: true})
	for _, panel := range []Panel{Podcasts, Episodes, Queue, Details, PlayerPanel} {
		m = m.WithFocus(panel)
		key(",")
		key(".")
		if m.Focus() != panel {
			t.Fatal("seek changed focus")
		}
	}
	if len(commands) != 10 || !reflect.DeepEqual(commands[:2], []string{"seek-backward", "seek-forward"}) {
		t.Fatal(commands)
	}
	key("?")
	key(",")
	key(".")
	key("esc")
	key("e")
	key(",")
	key(".")
	key("esc")
	m = m.WithDialog(&Dialog{Title: "Confirm"})
	key(",")
	key(".")
	m = m.WithDialog(nil).WithFocus(Episodes)
	key("/")
	key(",")
	key(".")
	if m.filters[Episodes] != ",." || len(commands) != 10 {
		t.Fatal("seek stole input or escaped overlay")
	}
	key("esc")
	m = m.WithFocus(Episodes)
	key("l")
	if m.Focus() != Episodes || m.colOffset[Episodes] != 4 {
		t.Fatal("l did not scroll column offset on episodes")
	}
	key("h")
	if m.Focus() != Episodes || m.colOffset[Episodes] != 0 {
		t.Fatal("h did not scroll column offset back")
	}
	m = m.WithFocus(PlayerPanel)
	key("tab")
	if m.Focus() != Podcasts {
		t.Fatal("Tab cannot leave Player")
	}
}

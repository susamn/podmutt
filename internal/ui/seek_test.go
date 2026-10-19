package ui

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestPlayerSeekKeysAreScopedAndGuarded(t *testing.T) {
	var commands []string
	m := resize(New(fixture()), 140, 44).WithFocus(PlayerPanel).WithActions(func(command string) tea.Cmd {
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
	key("h")
	key("l")
	if len(commands) != 0 || m.Focus() != PlayerPanel {
		t.Fatal("disabled seek dispatched or moved focus")
	}
	m = m.WithPlayback(m.data.Queue, Player{Status: "paused", HasCurrent: true, CanSeek: true})
	key("h")
	key("l")
	if !reflect.DeepEqual(commands, []string{"seek-backward", "seek-forward"}) {
		t.Fatal(commands)
	}
	key("?")
	key("h")
	key("l")
	key("esc")
	m = m.WithDialog(&Dialog{Title: "Confirm"})
	key("h")
	key("l")
	m = m.WithDialog(nil)
	if len(commands) != 2 {
		t.Fatal("busy seek dispatched")
	}
	m = m.WithFocus(Episodes)
	key("/")
	key("h")
	key("l")
	if m.filters[Episodes] != "hl" || len(commands) != 2 {
		t.Fatal("seek stole input")
	}
	key("esc")
	// , and . seek backward and forward
	key(",")
	key(".")
	if !reflect.DeepEqual(commands, []string{"seek-backward", "seek-forward", "seek-backward", "seek-forward"}) {
		t.Fatalf("unexpected seek commands: %v", commands)
	}

	m = m.WithFocus(Episodes)
	key("l")
	if m.Focus() != Episodes || m.colOffset[Episodes] != 4 {
		t.Fatal("l did not scroll column offset on episodes")
	}
	key("tab")
	if m.Focus() != Queue {
		t.Fatal("tab did not move focus to Queue")
	}
	m = m.WithFocus(PlayerPanel)
	key("tab")
	if m.Focus() != Podcasts {
		t.Fatal("Tab cannot leave Player")
	}
}

package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"podmutt/internal/ui"
)

func TestUnifiedPopupDesignUserJourneyPreservesActions(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		updateLibrary(&m, tea.WindowSizeMsg{Width: 100, Height: 28})
		finishLibrary(&m, libraryKey(&m, 'a'))
		state := syncPlayer(t, &m)
		if len(state.Queue) != 1 {
			t.Fatal("fixture enqueue failed", state)
		}
		id := fmt.Sprint(state.Queue[0].ID)

		// Episode actions uses the same padded section/footer design as Chapters.
		m.Model = m.Model.WithFocus(ui.Queue).Select(ui.Queue, id)
		libraryKey(&m, 'm')
		actions := ansi.Strip(m.View().Content)
		for _, want := range []string{"Episode Actions", "DETAILS", "Mark as favorite", "Enter apply · Esc cancel"} {
			if !strings.Contains(actions, want) {
				t.Fatalf("actions popup missing %q:\n%s", want, actions)
			}
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEsc})
		if m.confirming != nil || m.Model.Busy() {
			t.Fatal("Esc did not close actions popup")
		}

		// Delete confirmation retains its exact decision behavior and recovery keys.
		m.Model = m.Model.WithFocus(ui.Queue).Select(ui.Queue, id)
		libraryKey(&m, 'd')
		remove := ansi.Strip(m.View().Content)
		for _, want := range []string{"Remove episode?", "DETAILS", "Progress/history kept.", "y remove · n/Esc cancel"} {
			if !strings.Contains(remove, want) {
				t.Fatalf("remove popup missing %q:\n%s", want, remove)
			}
		}
		finishLibrary(&m, libraryKey(&m, 'n'))
		if len(syncPlayer(t, &m).Queue) != 1 {
			t.Fatal("cancel removed queue item")
		}

		// Settings and Help use named sections and preserve key ownership.
		libraryKey(&m, 'e')
		settings := ansi.Strip(m.View().Content)
		for _, want := range []string{"Settings", "APPEARANCE", "CONFIGURATION", "Esc close"} {
			if !strings.Contains(settings, want) {
				t.Fatalf("settings missing %q:\n%s", want, settings)
			}
		}
		// C remains owned by Settings; it must not open Chapters.
		libraryKey(&m, 'C')
		if m.Model.ChapterSelection() != -1 || !m.Model.SettingsOpen() {
			t.Fatal("Settings lost C ownership")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEsc})
		libraryKey(&m, '?')
		help := ansi.Strip(m.View().Content)
		for _, want := range []string{"Keys", "NAVIGATION", "PLAYBACK", "LIBRARY", "Esc close"} {
			if !strings.Contains(help, want) {
				t.Fatalf("help missing %q:\n%s", want, help)
			}
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEsc})

		// Minimum terminal keeps destructive recovery keys reachable.
		updateLibrary(&m, tea.WindowSizeMsg{Width: 30, Height: 12})
		m.Model = m.Model.WithFocus(ui.Queue).Select(ui.Queue, id)
		libraryKey(&m, 'd')
		compact := ansi.Strip(m.View().Content)
		if !strings.Contains(compact, "Esc cancel") || len(strings.Split(compact, "\n")) != 12 {
			t.Fatal("compact confirmation lost recovery/footer or resized frame", compact)
		}
		finishLibrary(&m, libraryKey(&m, 'n'))
	})
}

package app

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/player"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

func TestQueueCurrentRowNavigationPromotionAndCompletion(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		first, _ := m.selectedEpisode()
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace}))
		syncPlayer(t, &m)
		if rows := m.Model.Visible(ui.Queue); len(rows) != 1 || !strings.Contains(rows[0].Title, "Alpha show") || !strings.Contains(rows[0].Title, "[playing]") {
			t.Fatal("current not visible", rows)
		}
		m.Model = m.Model.WithFocus(ui.PlayerPanel)
		for range 2 {
			next, _ := m.Model.Update(tea.KeyPressMsg{Code: tea.KeyRight})
			m.Model = next.(ui.Model)
		}
		if _, cmd := m.Model.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
			t.Fatal("current-only stack enables Next")
		}
		m.Model = m.Model.WithFocus(ui.Episodes)
		libraryKey(&m, 'j')
		second, _ := m.selectedEpisode()
		if second.ID == first.ID {
			t.Fatal("episode navigation failed")
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace}))
		syncPlayer(t, &m)
		if m.state.Current.ID != second.ID || len(m.Model.Visible(ui.Queue)) != 2 {
			t.Fatal("interrupted episode missing")
		}
		m.Model = m.Model.WithFocus(ui.Queue).Select(ui.Queue, fmt.Sprint(second.ID))
		finishLibrary(&m, libraryKey(&m, 'J'))
		if !strings.Contains(m.notice, "stays at the front") {
			t.Fatal(m.notice)
		}
		libraryKey(&m, 'd')
		if m.confirming != nil {
			t.Fatal("current removal confirmation")
		}
		libraryKey(&m, 'j')
		selected, _ := m.Model.Selected(ui.Queue)
		if selected.ID != fmt.Sprint(first.ID) {
			t.Fatal("queue j failed", selected)
		}
		libraryKey(&m, 'k')
		libraryKey(&m, 'j')
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		syncPlayer(t, &m)
		if m.state.Current.ID != first.ID || m.Model.Focus() != ui.Queue || len(m.state.Queue) != 1 || m.state.Queue[0].ID != second.ID {
			t.Fatal("Enter failed to promote", m.state)
		}
		libraryKey(&m, 'D')
		finishLibrary(&m, libraryKey(&m, 'y'))
		syncPlayer(t, &m)
		if len(m.Model.Visible(ui.Queue)) != 1 {
			t.Fatal("pending clear removed current")
		}
		id, _ := p.EntryID(s.ctx)
		p.events <- player.Event{Name: "end-file", Reason: "eof", PlaylistEntryID: &id}
		until := time.Now().Add(time.Second)
		for !syncPlayer(t, &m).Current.Completed && time.Now().Before(until) {
			time.Sleep(time.Millisecond)
		}
		if len(m.Model.Visible(ui.Queue)) != 0 {
			t.Fatal("completed item still in stack")
		}
		history, err := s.store.Playback(s.ctx, first.ID)
		if err != nil || !history.Completed {
			t.Fatal("completion history missing", history, err)
		}
	})
}

func TestStackRowFormattingAndCurrentContext(t *testing.T) {
	row := stackItem(store.Episode{PositionSeconds: 62, DurationSeconds: 120}, "queued")
	if !strings.Contains(row.Title, "Unknown show") || !strings.Contains(row.Title, "1:02/2:00") {
		t.Fatal(row)
	}
}

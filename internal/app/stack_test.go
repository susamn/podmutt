package app

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/playback"
	"podmutt/internal/player"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

func TestUpNextExcludesCurrentAndPreservesInterruptedPlayback(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		first, _ := m.selectedEpisode()
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		syncPlayer(t, &m)
		if rows := m.Model.Visible(ui.Queue); len(rows) != 0 {
			t.Fatal("current leaked into Up next", rows)
		}
		if !strings.Contains(m.View().Content, "Now playing") || !strings.Contains(m.View().Content, first.Title) {
			t.Fatal("current audio is not visible separately")
		}
		m.Model = m.Model.WithFocus(ui.PlayerPanel)
		for range 2 {
			next, _ := m.Model.Update(tea.KeyPressMsg{Code: tea.KeyRight})
			m.Model = next.(ui.Model)
		}
		if _, cmd := m.Model.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
			t.Fatal("current-only playback enables Next")
		}
		m.Model = m.Model.WithFocus(ui.Episodes)
		libraryKey(&m, 'j')
		second, _ := m.selectedEpisode()
		if second.ID == first.ID {
			t.Fatal("episode navigation failed")
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		syncPlayer(t, &m)
		rows := m.Model.Visible(ui.Queue)
		if m.state.Current.ID != second.ID || len(rows) != 1 || rows[0].ID != fmt.Sprint(first.ID) || rows[0].Index != 1 {
			t.Fatal("interrupted playback not preserved in Up next", m.state, rows)
		}
		m.Model = m.Model.WithFocus(ui.Queue)
		finishLibrary(&m, libraryKey(&m, 'J'))
		finishLibrary(&m, libraryKey(&m, 'K'))
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		syncPlayer(t, &m)
		if m.state.Current.ID != first.ID || m.Model.Focus() != ui.Queue || len(m.state.Queue) != 1 || m.state.Queue[0].ID != second.ID {
			t.Fatal("Enter failed to promote pending episode", m.state)
		}
		libraryKey(&m, 'D')
		finishLibrary(&m, libraryKey(&m, 'y'))
		syncPlayer(t, &m)
		if len(m.Model.Visible(ui.Queue)) != 0 || m.state.Current.ID != first.ID || m.state.Status != playback.Playing {
			t.Fatal("clear affected current audio", m.state)
		}
		for _, key := range []rune{'J', 'K', 'd', 'D', 'i'} {
			finishLibrary(&m, libraryKey(&m, key))
			if m.Model.Focus() != ui.Queue || m.confirming != nil {
				t.Fatal("empty Up next targeted current", string(key))
			}
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if state := syncPlayer(t, &m); state.Current.ID != first.ID || state.Status != playback.Playing {
			t.Fatal("empty Up next Enter changed current", state)
		}
		id, _ := p.EntryID(s.ctx)
		p.events <- player.Event{Name: "end-file", Reason: "eof", PlaylistEntryID: &id}
		until := time.Now().Add(time.Second)
		for !syncPlayer(t, &m).Current.Completed && time.Now().Before(until) {
			time.Sleep(time.Millisecond)
		}
		if len(m.Model.Visible(ui.Queue)) != 0 {
			t.Fatal("completion created pending work")
		}
		history, err := s.store.Playback(s.ctx, first.ID)
		if err != nil || !history.Completed {
			t.Fatal("completion history missing", history, err)
		}
	})
}

func TestQueueRowFormatting(t *testing.T) {
	row := queueItem(store.Episode{PositionSeconds: 62, DurationSeconds: 120})
	if !strings.Contains(row.Title, "Unknown show") || !strings.Contains(row.Title, "1:02/2:00") {
		t.Fatal(row)
	}
	if row.Remains != "48%" {
		t.Fatalf("expected remains 48%%, got %q", row.Remains)
	}
}

package app

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/ui"
)

func TestNextChangesWithLiveQueuePublications(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		receive := func() {
			t.Helper()
			select {
			case state := <-s.playback.Updates():
				updateLibrary(&m, playbackUpdate{state, true})
			case <-time.After(time.Second):
				t.Fatal("actor update stalled")
			}
		}
		receive()
		m.Model = m.Model.WithFocus(ui.PlayerPanel)
		for range 2 {
			updated, _ := m.Model.Update(tea.KeyPressMsg{Code: tea.KeyRight})
			m.Model = updated.(ui.Model)
		}
		enabled := func(want bool) {
			t.Helper()
			_, cmd := m.Model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			if (cmd != nil) != want {
				t.Fatalf("Next enabled=%v, want %v", cmd != nil, want)
			}
		}
		enabled(false)
		episode, ok := m.selectedEpisode()
		if !ok {
			t.Fatal("no selected episode")
		}
		for range 3 {
			if err := s.playback.Enqueue(s.ctx, episode.ID); err != nil {
				t.Fatal(err)
			}
			receive()
			enabled(true)
			if len(m.Model.Visible(ui.Queue)) != 1 {
				t.Fatal("queue not updated")
			}
			if err := s.playback.SetQueue(s.ctx, nil); err != nil {
				t.Fatal(err)
			}
			receive()
			enabled(false)
		}
	})
}

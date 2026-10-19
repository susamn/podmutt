package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/playback"
	"podmutt/internal/ui"
)

func TestCompletedHistoryDoesNotHideStillActiveCurrentAudio(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		current := syncPlayer(t, &m).Current.ID
		if err := s.playback.MarkCompleted(s.ctx, current, true); err != nil {
			t.Fatal(err)
		}
		for _, want := range []playback.Status{playback.Playing, playback.Paused} {
			state := syncPlayer(t, &m)
			rows := m.Model.Visible(ui.Queue)
			if state.Status != want || !state.Current.Completed || len(rows) != 1 || !rows[0].Current {
				t.Fatal("history marking hid current audio", state, rows)
			}
			m.Model = m.Model.WithFocus(ui.Queue)
			if libraryKey(&m, 'd') != nil || m.queueEditing {
				t.Fatal("current became removable")
			}
			if err := s.playback.Pause(s.ctx); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.playback.Stop(s.ctx); err != nil {
			t.Fatal(err)
		}
		if state := syncPlayer(t, &m); state.Status != playback.Stopped || len(m.Model.Visible(ui.Queue)) != 0 {
			t.Fatal("finished current is still pending", state)
		}
	})
}

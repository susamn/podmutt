package app

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/ui"
)

func TestActivationAcrossShowsAlignsScopeButProgressDoesNot(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		first := m.rows[0]
		otherShow := m.snapshot.Feeds[1].ID
		if err := s.store.SetSubscribed(s.ctx, otherShow, true); err != nil {
			t.Fatal(err)
		}
		finishLibrary(&m, m.reload())
		other, err := s.store.Episodes(s.ctx, otherShow)
		if err != nil {
			t.Fatal(err)
		}
		openLibraryDestination(&m, fmt.Sprint(first.FeedID))
		m.Model = m.Model.WithFocus(ui.Episodes).Select(ui.Episodes, fmt.Sprint(first.ID))
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		syncPlayer(t, &m)
		if err := s.playback.Enqueue(s.ctx, other[0].ID); err != nil {
			t.Fatal(err)
		}
		syncPlayer(t, &m)
		// A queue-only publication updates badges, not the current browsing scope.
		if m.libraryFeedID() != first.FeedID {
			t.Fatal("enqueue changed scope")
		}
		if err := s.playback.Next(s.ctx); err != nil {
			t.Fatal(err)
		}
		syncPlayer(t, &m)
		if m.libraryFeedID() != otherShow {
			t.Fatal("new current is hidden behind old show's scope", m.libraryID)
		}
		for _, panel := range []ui.Panel{ui.Podcasts, ui.Episodes, ui.Queue} {
			want := fmt.Sprint(other[0].ID)
			if panel == ui.Podcasts {
				want = fmt.Sprint(otherShow)
			}
			if selected, _ := m.Model.Selected(panel); selected.ID != want {
				t.Fatal("activation not aligned", panel, selected)
			}
		}
	})
}

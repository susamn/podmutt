package app

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/player"
	"podmutt/internal/ui"
)

func TestNotStartedMovesToContinueOnLiveProgressAndSurvivesStaleRead(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		cached := m.snapshot
		first := m.rows[0]
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		syncPlayer(t, &m)
		entry, _ := p.EntryID(s.ctx)
		p.events <- player.Event{Name: "start-file", PlaylistEntryID: &entry}
		p.events <- player.Event{Name: "property-change", Property: "time-pos", Data: []byte("42")}
		deadline := time.Now().Add(time.Second)
		for syncPlayer(t, &m).Current.PositionSeconds != 42 {
			if time.Now().After(deadline) {
				t.Fatal("progress was not received")
			}
			time.Sleep(time.Millisecond)
		}
		if len(m.rows) != 1 || m.rows[0].ID == first.ID {
			t.Fatal("started episode remained in Not started", m.rows)
		}
		updateLibrary(&m, libraryLoaded{version: m.version, snapshot: cached})
		if len(m.rows) != 1 || m.rows[0].ID == first.ID {
			t.Fatal("cached read reintroduced current into Not started")
		}
		openLibraryDestination(&m, continueDestination)
		if len(m.rows) != 1 || m.rows[0].ID != first.ID || m.rows[0].PositionSeconds != 42 {
			t.Fatal("live progress missing from Continue", m.rows)
		}
		openLibraryDestination(&m, allDestination)
		if len(m.rows) != 2 {
			t.Fatal("All subscribed episodes lost started history")
		}
		m.Model = m.Model.WithFocus(ui.Episodes)
		finishLibrary(&m, libraryKey(&m, 'c'))
		syncPlayer(t, &m)
		openLibraryDestination(&m, newDestination)
		if len(m.rows) != 1 || m.rows[0].ID == first.ID {
			t.Fatal("completed history entered Not started")
		}
		openLibraryDestination(&m, continueDestination)
		if len(m.rows) != 0 {
			t.Fatal("completed history remained in Continue")
		}
	})
}

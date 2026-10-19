package app

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/player"
)

func TestAllRetainsCurrentProgressAcrossCachedRead(t *testing.T) {
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
				t.Fatal("progress missing")
			}
			time.Sleep(time.Millisecond)
		}
		updateLibrary(&m, libraryLoaded{version: m.version, snapshot: cached})
		if len(m.rows) != 2 || m.rows[0].ID != first.ID || m.rows[0].PositionSeconds != 42 {
			t.Fatal("progress moved current out of All or stale cache won", m.rows)
		}
		finishLibrary(&m, libraryKey(&m, 'c'))
		if len(m.rows) != 2 || !m.rows[0].Completed {
			t.Fatal("completion hid All history", m.rows)
		}
	})
}
